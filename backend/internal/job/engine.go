package job

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/fingerprint"
	"github.com/expvintl/scopex/internal/geo"
	"github.com/expvintl/scopex/internal/i18n"
	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/scanner"
	"github.com/expvintl/scopex/internal/store"
	"github.com/expvintl/scopex/internal/util"
)

const (
	probeWorkers   = 24
	maxCollected   = 50000
	deepScanLimit  = 5000
	reportInterval = 1500 * time.Millisecond
	refreshWorkers = 8
)

// Engine 负责任务排队、扫描编排、进度与日志落库。
type Engine struct {
	cfg      config.Config
	store    *store.Store
	registry *scanner.Registry
	geo      *geo.Resolver
	prober   *fingerprint.Prober

	denyMu  sync.RWMutex
	deny    []util.IPRange
	denyRaw []string
	log     *slog.Logger

	queue chan primitive.ObjectID

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	wg      sync.WaitGroup
}

func NewEngine(cfg config.Config, st *store.Store, reg *scanner.Registry, g *geo.Resolver, logger *slog.Logger) *Engine {
	return &Engine{
		cfg:      cfg,
		store:    st,
		registry: reg,
		geo:      g,
		prober:   fingerprint.NewProber(cfg.Scanner.ProbeTimeout),
		log:      logger,
		queue:    make(chan primitive.ObjectID, 128),
		cancels:  make(map[string]context.CancelFunc),
	}
}

func (e *Engine) Start(ctx context.Context, workers int) {
	if workers <= 0 {
		workers = 2
	}
	for i := 0; i < workers; i++ {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case id, ok := <-e.queue:
					if !ok {
						return
					}
					e.run(ctx, id)
				}
			}
		}()
	}
}

func (e *Engine) Stop() {
	close(e.queue)
	e.wg.Wait()
}

func (e *Engine) Submit(id primitive.ObjectID) bool {
	select {
	case e.queue <- id:
		return true
	default:
		return false
	}
}

func (e *Engine) Cancel(id primitive.ObjectID) bool {
	e.mu.Lock()
	cancel, ok := e.cancels[id.Hex()]
	e.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (e *Engine) Running(id primitive.ObjectID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.cancels[id.Hex()]
	return ok
}

type runStats struct {
	openTotal     atomic.Int64
	probeDone     atomic.Int64
	created       atomic.Int64
	updated       atomic.Int64
	failures      atomic.Int64
	discPct       atomic.Int64
	discoveryDone atomic.Bool
	ips           sync.Map
}

func (s *runStats) snapshot() model.TaskStats {
	hosts := 0
	s.ips.Range(func(_, _ any) bool { hosts++; return true })
	return model.TaskStats{
		HostsScanned: hosts,
		OpenPorts:    int(s.openTotal.Load()),
		NewAssets:    int(s.created.Load()),
		Updated:      int(s.updated.Load()),
		Errors:       int(s.failures.Load()),
	}
}

func (e *Engine) run(parent context.Context, id primitive.ObjectID) {
	task, err := e.store.GetTask(parent, id)
	if err != nil {
		e.log.Warn("加载任务失败", "task", id.Hex(), "err", err)
		return
	}
	if task.Status == model.TaskCanceled || task.Status == model.TaskDone {
		return
	}

	ctx, cancel := context.WithCancel(parent)
	e.mu.Lock()
	e.cancels[id.Hex()] = cancel
	e.mu.Unlock()
	defer func() {
		cancel()
		e.mu.Lock()
		delete(e.cancels, id.Hex())
		e.mu.Unlock()
	}()

	taskLang := i18n.Lang(task.Lang)
	if taskLang == "" {
		taskLang = i18n.ZH
	}

	started := time.Now()
	e.update(id, bson.M{
		"status":     model.TaskRunning,
		"phase":      "discovery",
		"progress":   0.0,
		"started_at": started,
		"error":      "",
	})
	e.logf(id, taskLang, "info", "任务「%s」开始：%d 个目标，端口 %s", task.Name, len(task.Targets), task.Ports)

	ranges, total, err := util.ParseTargets(task.Targets)
	if err != nil {
		e.fail(id, taskLang, "解析目标失败: %s", i18n.Message(taskLang, err))
		return
	}
	if total == 0 {
		e.fail(id, taskLang, "没有可扫描的目标地址")
		return
	}
	if total > uint64(e.cfg.Scanner.MaxTargets) {
		e.fail(id, taskLang, "目标地址数 %d 超过上限 %d", total, e.cfg.Scanner.MaxTargets)
		return
	}
	ports, err := scanner.ParsePorts(task.Ports)
	if err != nil {
		e.fail(id, taskLang, "解析端口失败: %s", i18n.Message(taskLang, err))
		return
	}
	if err := e.checkDenied(ranges); err != nil {
		e.fail(id, taskLang, "%s", i18n.Message(taskLang, err))
		return
	}

	if settings, err := e.store.GetScanSettings(ctx); err == nil {
		e.registry.ApplySettings(settings)
		if settings.ScanInterface != "" {
			e.logf(id, taskLang, "info", "出口网卡：%s", settings.ScanInterface)
		}
	}

	tool, warn := e.registry.ResolveTool(task.Tool, total)
	if warn != "" {
		e.logf(id, taskLang, "warn", "%s", warn)
	}
	e.update(id, bson.M{"total_hosts": total, "tool": tool})
	e.logf(id, taskLang, "info", "目标 %d 个地址 × %d 个端口，发现引擎：%s", total, len(ports), tool)

	st := &runStats{}
	openCh := make(chan scanner.OpenPort, 4096)

	var collectMu sync.Mutex
	var collected []scanner.OpenPort
	var collectedN atomic.Int64

	send := func(op scanner.OpenPort) {
		st.openTotal.Add(1)
		if collectedN.Add(1) <= maxCollected {
			collectMu.Lock()
			collected = append(collected, op)
			collectMu.Unlock()
		}
		select {
		case openCh <- op:
		case <-ctx.Done():
		}
	}

	var dErr error
	discDone := make(chan struct{})
	go func() {
		defer close(discDone)
		defer close(openCh)
		switch tool {
		case "masscan":
			_, dErr = e.registry.Masscan.Scan(ctx, ranges, ports, task.Rate, send)
		case "nmap":
			_, dErr = e.registry.Nmap.ScanOpenPorts(ctx, ranges, ports, task.Rate, e.cfg.Scanner.ServiceTimeout, send, nil)
		default:
			_, dErr = e.registry.Native.Scan(ctx, ranges, ports, task.Rate, send, func(done, all int64) {
				if all > 0 {
					st.discPct.Store(done * 100 / all)
				}
			})
		}
	}()

	rctx, rcancel := context.WithCancel(ctx)
	reportDone := make(chan struct{})
	go func() {
		defer close(reportDone)
		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()
		for {
			select {
			case <-rctx.Done():
				return
			case <-ticker.C:
				pct, phase := progressOf(st)
				e.update(id, bson.M{"progress": pct, "phase": phase, "stats": st.snapshot()})
			}
		}
	}()

	var pwg sync.WaitGroup
	for i := 0; i < probeWorkers; i++ {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			for op := range openCh {
				e.inspect(ctx, task, op, st)
			}
		}()
	}
	pwg.Wait()
	<-discDone
	st.discoveryDone.Store(true)
	rcancel()
	<-reportDone

	if task.Deep && len(collected) > 0 && len(collected) <= deepScanLimit {
		if _, err := e.registry.Nmap.Available(); err == nil {
			e.update(id, bson.M{"phase": "service", "progress": 96.0})
			e.logf(id, taskLang, "info", "深度识别：nmap 扫描 %d 个开放端口", len(collected))
			e.deepScan(ctx, id, taskLang, collected, st)
		} else {
			e.logf(id, taskLang, "warn", "深度识别已跳过：nmap 不可用")
		}
	}

	status := model.TaskDone
	errMsg := ""
	if ctx.Err() != nil {
		status = model.TaskCanceled
	}
	if dErr != nil && !errors.Is(dErr, context.Canceled) && ctx.Err() == nil {
		status = model.TaskFailed
		errMsg = dErr.Error()
	}
	stats := st.snapshot()
	set := bson.M{
		"status":      status,
		"stats":       stats,
		"finished_at": time.Now(),
		"error":       errMsg,
	}
	if status != model.TaskCanceled {
		set["progress"] = 100.0
		set["phase"] = "done"
	}
	e.update(id, set)

	level := "info"
	if status == model.TaskFailed {
		level = "error"
	}
	e.logf(id, taskLang, level, "任务结束：状态 %s，开放端口 %d，新增 %d，更新 %d，错误 %d（耗时 %s）",
		status, stats.OpenPorts, stats.NewAssets, stats.Updated, stats.Errors,
		time.Since(started).Round(time.Second))
}

func (e *Engine) inspect(ctx context.Context, task *model.Task, op scanner.OpenPort, st *runStats) {
	defer st.probeDone.Add(1)
	if ctx.Err() != nil {
		return
	}
	st.ips.Store(op.IP, struct{}{})
	res := e.prober.Probe(ctx, op.IP, op.Port, "")
	if res == nil {
		st.failures.Add(1)
		return
	}
	loc, asn := e.geo.Lookup(ctx, op.IP)
	host := buildHost("task:"+task.ID.Hex(), op, res, loc, asn)
	created, err := e.store.UpsertHost(ctx, host)
	if err != nil {
		if ctx.Err() == nil {
			st.failures.Add(1)
			e.log.Debug("写入资产失败", "ip", op.IP, "port", op.Port, "err", err)
		}
		return
	}
	if created {
		st.created.Add(1)
	} else {
		st.updated.Add(1)
	}
}

func (e *Engine) deepScan(ctx context.Context, id primitive.ObjectID, l i18n.Lang, ports []scanner.OpenPort, st *runStats) {
	ips := make([]string, 0, 64)
	seen := make(map[string]struct{})
	portSet := make(map[int]struct{})
	for _, p := range ports {
		if _, ok := seen[p.IP]; !ok && len(ips) < 256 {
			seen[p.IP] = struct{}{}
			ips = append(ips, p.IP)
		}
		portSet[p.Port] = struct{}{}
	}
	if len(ips) > 64 {
		ips = ips[:64]
	}
	list := make([]int, 0, len(portSet))
	for p := range portSet {
		list = append(list, p)
	}
	timeout := e.cfg.Scanner.ServiceTimeout
	err := e.registry.Nmap.ScanServices(ctx, ips, list, timeout, func(r scanner.NmapResult) {
		if r.Service.Name == "" {
			return
		}
		if err := e.store.ApplyNmap(ctx, r.IP, r.Port, r.Protocol, r.Service, r.CPE, r.Scripts); err != nil {
			st.failures.Add(1)
		}
	})
	if err != nil && ctx.Err() == nil {
		e.logf(id, l, "warn", "深度识别出现错误: %v", i18n.Message(l, err))
	}
}

// SetDenyCIDRs 更新禁止扫描的地址段，解析失败时不生效。
func (e *Engine) SetDenyCIDRs(cidrs []string) error {
	raw := make([]string, 0, len(cidrs))
	for _, item := range cidrs {
		if v := strings.TrimSpace(item); v != "" {
			raw = append(raw, v)
		}
	}
	var ranges []util.IPRange
	if len(raw) > 0 {
		parsed, _, err := util.ParseTargets(raw)
		if err != nil {
			return err
		}
		ranges = parsed
	}
	e.denyMu.Lock()
	e.deny, e.denyRaw = ranges, raw
	e.denyMu.Unlock()
	return nil
}

// DenyCIDRs 返回当前生效的禁止扫描地址段。
func (e *Engine) DenyCIDRs() []string {
	e.denyMu.RLock()
	defer e.denyMu.RUnlock()
	return append([]string{}, e.denyRaw...)
}

func (e *Engine) checkDenied(ranges []util.IPRange) error {
	e.denyMu.RLock()
	deny := e.deny
	e.denyMu.RUnlock()
	if len(deny) == 0 {
		return nil
	}
	for _, r := range ranges {
		for _, d := range deny {
			if r.Start <= d.End && d.Start <= r.End {
				return i18n.Errorf("目标段 %s 命中禁止扫描的地址段 %s", r, d)
			}
		}
	}
	return nil
}

func (e *Engine) fail(id primitive.ObjectID, l i18n.Lang, format string, args ...any) {
	msg := i18n.T(l, format, args...)
	e.update(id, bson.M{
		"status":      model.TaskFailed,
		"error":       msg,
		"finished_at": time.Now(),
		"phase":       "failed",
	})
	e.logf(id, l, "error", "%s", msg)
}

func (e *Engine) update(id primitive.ObjectID, set bson.M) {
	set["updated_at"] = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.store.UpdateTask(ctx, id, set); err != nil {
		e.log.Warn("更新任务状态失败", "task", id.Hex(), "err", err)
	}
}

func (e *Engine) logf(id primitive.ObjectID, l i18n.Lang, level, format string, args ...any) {
	msg := i18n.T(l, format, args...)
	attrs := []any{"task", id.Hex(), "level", level}
	if level == "error" {
		e.log.Error(msg, attrs...)
	} else {
		e.log.Info(msg, attrs...)
	}
	if id == primitive.NilObjectID {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	seq, err := e.store.NextLogSeq(ctx, id, 1)
	if err != nil {
		return
	}
	if err := e.store.AppendLogs(ctx, id, []model.TaskLog{{Seq: seq, Level: level, Msg: msg, At: time.Now()}}); err != nil {
		e.log.Debug("写入任务日志失败", "task", id.Hex(), "err", err)
	}
}

func progressOf(st *runStats) (float64, string) {
	if !st.discoveryDone.Load() {
		return float64(st.discPct.Load()) * 0.5, "discovery"
	}
	open := st.openTotal.Load()
	if open == 0 {
		return 95.0, "fingerprint"
	}
	pct := 50.0 + 45.0*float64(st.probeDone.Load())/float64(open)
	if pct > 95 {
		pct = 95
	}
	return pct, "fingerprint"
}

// hostBanner 丢弃 HTML 响应体，避免把整页源码写入 banner 字段。
func hostBanner(banner string) string {
	trimmed := strings.TrimSpace(banner)
	if trimmed == "" || strings.HasPrefix(trimmed, "<") {
		return ""
	}
	return util.Truncate(trimmed, 4000)
}

// ErrNoAssets 表示该 IP 下没有已测绘的端口记录。
var ErrNoAssets = errors.New("该 IP 下没有已测绘的资产")

// RefreshIP 重新探测该 IP 下已记录的端口，并刷新归属地与指纹信息。
func (e *Engine) RefreshIP(ctx context.Context, ip string) (int, error) {
	hosts, err := e.store.HostsByIP(ctx, ip, 500)
	if err != nil {
		return 0, err
	}
	if len(hosts) == 0 {
		return 0, ErrNoAssets
	}

	var (
		wg        sync.WaitGroup
		refreshed atomic.Int32
	)
	sem := make(chan struct{}, refreshWorkers)
	for _, h := range hosts {
		port := h.Port
		proto := h.Protocol
		if proto == "" {
			proto = "tcp"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			res := e.prober.Probe(ctx, ip, port, "")
			if res == nil {
				return
			}
			loc, asn := e.geo.Lookup(ctx, ip)
			op := scanner.OpenPort{IP: ip, Port: port, Proto: proto}
			if _, err := e.store.UpsertHost(ctx, buildHost("refresh", op, res, loc, asn)); err != nil {
				e.log.Debug("刷新资产失败", "ip", ip, "port", port, "err", err)
				return
			}
			refreshed.Add(1)
		}()
	}
	wg.Wait()
	return int(refreshed.Load()), ctx.Err()
}

func buildHost(source string, op scanner.OpenPort, res *fingerprint.Result, g model.Geo, asn *model.ASN) *model.Host {
	now := time.Now()
	ip := net.ParseIP(op.IP)
	h := &model.Host{
		IP:         op.IP,
		Port:       op.Port,
		Protocol:   op.Proto,
		Service:    res.Service,
		Banner:     hostBanner(res.Banner),
		Title:      res.Title,
		Headers:    res.Headers,
		HeaderText: util.Truncate(res.HeaderText, 8000),
		StatusCode: res.StatusCode,
		Favicon:    res.Favicon,
		TLS:        res.TLS,
		Apps:       dedupe(res.Apps),
		CPE:        dedupe(res.CPE),
		Device:     res.Device,
		OS:         hostOS(res),
		Scripts:    res.Scripts,
		AI:         res.AI,
		Hostnames:  dedupe(res.Hostnames),
		Domains:    dedupe(res.Domains),
		Geo:        g,
		ASN:        asn,
		Source:     source,
		FirstSeen:  now,
		LastSeen:   now,
	}
	if ip != nil {
		h.IPNum = util.IPToUint32(ip)
	}
	if len(res.FaviconImg) > 0 && len(res.FaviconImg) <= 512*1024 {
		h.FaviconImg = res.FaviconImg
	}
	if h.Headers == nil {
		h.Headers = map[string]string{}
	}
	if h.Hostnames == nil {
		h.Hostnames = []string{}
	}
	if h.Domains == nil {
		h.Domains = []string{}
	}
	return h
}

// hostOS 汇总探针结论，优先使用协议协商得到的系统信息。
func hostOS(res *fingerprint.Result) string {
	if res.OS != "" {
		return util.Truncate(res.OS, 80)
	}
	return util.Truncate(res.Service.OSType, 80)
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
