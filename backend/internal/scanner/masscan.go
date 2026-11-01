package scanner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/expvintl/scopex/internal/util"
)

// Masscan 负责大范围端口测绘（需要 root / CAP_NET_RAW）。
type Masscan struct {
	Path string

	mu   sync.RWMutex
	opts MasscanOptions
}

// MasscanOptions 是控制台可调整的 masscan 参数。
type MasscanOptions struct {
	Interface string
	RouterMAC string
	ExtraArgs string
}

func (m *Masscan) SetOptions(o MasscanOptions) {
	m.mu.Lock()
	m.opts = o
	m.mu.Unlock()
}

func (m *Masscan) Options() MasscanOptions {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.opts
}

// buildArgs 组装 masscan 命令行；自定义参数放在默认参数之后，可覆盖默认行为。
func (m *Masscan) buildArgs(targets []string, ports []int, rate int) []string {
	o := m.Options()
	args := make([]string, 0, 16+len(targets))
	if o.Interface != "" {
		args = append(args, "--interface", o.Interface)
	}
	if o.RouterMAC != "" {
		args = append(args, "--router-mac", o.RouterMAC)
	}
	args = append(args, targets...)
	args = append(args,
		"-p"+PortsToSpec(ports),
		"--rate", strconv.Itoa(rate),
		"--wait", "2",
		"--open-only",
	)
	args = append(args, extraArgs(o.ExtraArgs)...)
	return append(args, "-oJ", "-")
}

// extraArgs 拆分自定义参数，剔除会破坏 JSON 输出或与表单字段冲突的选项。
func extraArgs(raw string) []string {
	fields := strings.Fields(raw)
	out := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "--interface" || f == "--router-mac" || strings.HasPrefix(f, "-o") || strings.HasPrefix(f, "--output-") {
			if i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
				i++
			}
			continue
		}
		out = append(out, f)
	}
	return out
}

func (m *Masscan) Available() (string, error) {
	path, err := exec.LookPath(m.Path)
	if err != nil {
		return "", fmt.Errorf("未找到 masscan 可执行文件")
	}
	if os.Geteuid() != 0 {
		return path, fmt.Errorf("masscan 需要 root 或 CAP_NET_RAW 权限")
	}
	return path, nil
}

func masscanPrivilege() string {
	if os.Geteuid() == 0 {
		return ""
	}
	return "需要 root 或 CAP_NET_RAW 权限，当前进程无 raw socket 权限"
}

func (m *Masscan) Scan(ctx context.Context, ranges []util.IPRange, ports []int, rate int,
	emit func(OpenPort)) (ScanStats, error) {
	path, err := exec.LookPath(m.Path)
	if err != nil {
		return ScanStats{}, fmt.Errorf("未找到 masscan")
	}
	if os.Geteuid() != 0 {
		return ScanStats{}, fmt.Errorf("masscan 需要 root 权限")
	}
	if rate <= 0 {
		rate = 1000
	}
	var targets []string
	for _, r := range ranges {
		targets = append(targets, RangeToCIDRs(r.Start, r.End)...)
	}
	if o := m.Options(); o.Interface != "" {
		if _, err := net.InterfaceByName(o.Interface); err != nil {
			return ScanStats{}, fmt.Errorf("网卡 %s 不存在", o.Interface)
		}
	}
	args := m.buildArgs(targets, ports, rate)
	cmd := exec.CommandContext(ctx, path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ScanStats{}, err
	}
	if err := cmd.Start(); err != nil {
		return ScanStats{}, fmt.Errorf("启动 masscan 失败: %w", err)
	}

	stats := ScanStats{}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		line = strings.TrimSuffix(strings.TrimRight(line, ","), ",")
		var record struct {
			IP    string `json:"ip"`
			Ports []struct {
				Port   int    `json:"port"`
				Proto  string `json:"proto"`
				Status string `json:"status"`
			} `json:"ports"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		for _, p := range record.Ports {
			if p.Status != "open" {
				continue
			}
			stats.Open++
			emit(OpenPort{IP: record.IP, Port: p.Port, Proto: p.Proto})
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return stats, err
	}
	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stats, fmt.Errorf("masscan 执行失败: %s", util.Truncate(msg, 300))
	}
	return stats, ctx.Err()
}

// RangeToCIDRs 把闭区间转换为覆盖它的最小 CIDR 列表。
func RangeToCIDRs(start, end uint32) []string {
	var out []string
	s, e := uint64(start), uint64(end)
	for s <= e {
		var bits uint
		for bits < 32 {
			size := uint64(1) << (bits + 1)
			if s%size != 0 || s+size-1 > e {
				break
			}
			bits++
		}
		size := uint64(1) << bits
		out = append(out, fmt.Sprintf("%s/%d", util.Uint32ToIP(uint32(s)).String(), 32-bits))
		s += size
	}
	return out
}

// ScanOpenPorts 用 nmap 完成端口发现（较慢但无需 root），结果带服务信息。
func (n *Nmap) ScanOpenPorts(ctx context.Context, ranges []util.IPRange, ports []int, rate int, timeout time.Duration,
	emit func(OpenPort), onService func(NmapResult)) (ScanStats, error) {
	var targets []string
	for _, r := range ranges {
		targets = append(targets, RangeToCIDRs(r.Start, r.End)...)
	}
	stats := ScanStats{}
	batches := ChunkStrings(targets, 32)
	portBatches := ChunkInts(ports, 200)
	for _, tb := range batches {
		for _, pb := range portBatches {
			if ctx.Err() != nil {
				return stats, ctx.Err()
			}
			err := n.ScanServices(ctx, tb, pb, timeout, func(r NmapResult) {
				stats.Open++
				emit(OpenPort{IP: r.IP, Port: r.Port, Proto: r.Protocol})
				if onService != nil {
					onService(r)
				}
			})
			if err != nil {
				return stats, err
			}
		}
		stats.Probed += int64(len(tb))
	}
	return stats, nil
}
