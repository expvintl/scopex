package geo

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
)

const (
	cityFile = "dbip-city-lite.mmdb"
	asnFile  = "dbip-asn-lite.mmdb"
	// 库文件超过该时长后认为过期，启动时尝试拉取新月度数据。
	maxAge = 35 * 24 * time.Hour
)

// Status 描述本地地理库的可用状态。
type Status struct {
	Enabled     bool      `json:"enabled"`
	Ready       bool      `json:"ready"`
	CityDate    time.Time `json:"cityDate"`
	ASNReady    bool      `json:"asnReady"`
	Downloading bool      `json:"downloading"`
	LastError   string    `json:"lastError,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Dir         string    `json:"dir"`
}

// Resolver 用本地 mmdb 库解析 IP 归属地，不依赖任何在线 API。
type Resolver struct {
	cfg    config.GeoConfig
	client *http.Client
	log    *slog.Logger

	mu     sync.RWMutex
	city   *maxminddb.Reader
	asn    *maxminddb.Reader
	status Status
}

func New(cfg config.GeoConfig, log *slog.Logger) *Resolver {
	if log == nil {
		log = slog.Default()
	}
	return &Resolver{
		cfg:    cfg,
		client: &http.Client{Timeout: 5 * time.Minute},
		log:    log,
		status: Status{Enabled: cfg.Enabled, Dir: cfg.Dir},
	}
}

// Start 载入已有库文件；缺失或过期时按配置在后台拉取。
func (r *Resolver) Start(ctx context.Context) {
	if !r.cfg.Enabled {
		return
	}
	loaded := r.load()
	if r.checkExpired() && r.cfg.AutoDownload {
		go r.Refresh(ctx)
	}
	if !loaded && !r.cfg.AutoDownload {
		r.log.Warn("地理库未就绪且未开启自动下载", "dir", r.cfg.Dir)
	}
}

func (r *Resolver) Enabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg.Enabled
}

// Configure 应用控制台保存的配置；启用状态发生变化时由调用方决定是否触发刷新。
func (r *Resolver) Configure(enabled, autoDownload bool, cityURL, asnURL string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := r.cfg.Enabled != enabled
	r.cfg.Enabled = enabled
	r.cfg.AutoDownload = autoDownload
	if cityURL != "" {
		r.cfg.CityURL = cityURL
	}
	if asnURL != "" {
		r.cfg.ASNURL = asnURL
	}
	r.status.Enabled = enabled
	return changed
}

// Sources 返回当前使用的两个下载地址。
func (r *Resolver) Sources() (city, asn string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg.CityURL, r.cfg.ASNURL
}

func (r *Resolver) Status() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// Lookup 返回 IP 的地理位置与 ASN，私网地址直接标记为内网。
func (r *Resolver) Lookup(_ context.Context, ipStr string) (model.Geo, *model.ASN) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return model.Geo{}, nil
	}
	if util.IsPrivateIP(ip) {
		return model.Geo{Private: true, Country: "内网", CountryCode: "LAN"}, nil
	}
	r.mu.RLock()
	city, asn := r.city, r.asn
	r.mu.RUnlock()
	if city == nil {
		return model.Geo{}, nil
	}

	var rec cityRecord
	if err := city.Lookup(ip, &rec); err != nil {
		return model.Geo{}, nil
	}
	g := model.Geo{
		CountryCode: rec.Country.ISOCode,
		Country:     pickName(rec.Country.Names),
		City:        pickName(rec.City.Names),
		Latitude:    rec.Location.Latitude,
		Longitude:   rec.Location.Longitude,
	}
	if len(rec.Subdivisions) > 0 {
		g.Province = pickName(rec.Subdivisions[0].Names)
	}
	var geoASN *model.ASN
	if asn != nil {
		var a asnRecord
		if err := asn.Lookup(ip, &a); err == nil && a.Number > 0 {
			geoASN = &model.ASN{Number: int(a.Number), Org: a.Org}
			g.ISP = a.Org
		}
	}
	return g, geoASN
}

// Refresh 下载最新月度库，下载失败时保留旧库。
func (r *Resolver) Refresh(ctx context.Context) error {
	r.mu.RLock()
	enabled, cityURL, asnURL := r.cfg.Enabled, r.cfg.CityURL, r.cfg.ASNURL
	r.mu.RUnlock()
	if !enabled {
		return errors.New("地理库未启用")
	}
	r.mu.Lock()
	if r.status.Downloading {
		r.mu.Unlock()
		return nil
	}
	r.status.Downloading = true
	r.status.LastError = ""
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.status.Downloading = false
		r.mu.Unlock()
	}()

	var errs []string
	for _, target := range []struct {
		pattern string
		file    string
		asn     bool
	}{
		{cityURL, cityFile, false},
		{asnURL, asnFile, true},
	} {
		if err := r.fetchMonth(ctx, target.pattern, target.file); err != nil {
			errs = append(errs, err.Error())
		}
	}
	r.load()
	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		r.mu.Lock()
		r.status.LastError = msg
		r.mu.Unlock()
		return errors.New(msg)
	}
	return nil
}

func (r *Resolver) fetchMonth(ctx context.Context, pattern, name string) error {
	if pattern == "" {
		return nil
	}
	now := time.Now()
	months := []string{now.Format("2006-01"), now.AddDate(0, -1, 0).Format("2006-01")}
	var lastErr error
	for _, month := range months {
		url := strings.ReplaceAll(pattern, "{month}", month)
		if err := r.download(ctx, url, name); err != nil {
			lastErr = err
			continue
		}
		r.log.Info("地理库已更新", "file", name, "month", month)
		return nil
	}
	if lastErr != nil {
		r.log.Warn("地理库下载失败", "file", name, "err", lastErr)
	}
	return lastErr
}

func (r *Resolver) download(ctx context.Context, url, name string) error {
	if err := os.MkdirAll(r.cfg.Dir, 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}

	var src io.Reader = resp.Body
	if strings.HasSuffix(url, ".gz") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gz.Close()
		src = gz
	}

	tmp := filepath.Join(r.cfg.Dir, name+".tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(r.cfg.Dir, name))
}

// load 打开目录下的库文件，返回是否至少加载到城市库。
func (r *Resolver) load() bool {
	cityPath := filepath.Join(r.cfg.Dir, cityFile)
	asnPath := filepath.Join(r.cfg.Dir, asnFile)

	var city, asn *maxminddb.Reader
	if f, err := os.Stat(cityPath); err == nil && f.Size() > 0 {
		if reader, err := maxminddb.Open(cityPath); err == nil {
			city = reader
		} else {
			r.log.Warn("地理库打开失败", "file", cityPath, "err", err)
		}
	}
	if f, err := os.Stat(asnPath); err == nil && f.Size() > 0 {
		if reader, err := maxminddb.Open(asnPath); err == nil {
			asn = reader
		}
	}

	r.mu.Lock()
	oldCity, oldASN := r.city, r.asn
	r.city, r.asn = city, asn
	if city != nil {
		if epoch := city.Metadata.BuildEpoch; epoch > 0 {
			r.status.CityDate = time.Unix(int64(epoch), 0)
		} else if info, err := os.Stat(cityPath); err == nil {
			r.status.CityDate = info.ModTime()
		}
	}
	r.status.Ready = city != nil
	r.status.ASNReady = asn != nil
	r.status.UpdatedAt = time.Now()
	r.mu.Unlock()

	if oldCity != nil {
		oldCity.Close()
	}
	if oldASN != nil {
		oldASN.Close()
	}
	return city != nil
}

func (r *Resolver) checkExpired() bool {
	info, err := os.Stat(filepath.Join(r.cfg.Dir, cityFile))
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > maxAge
}

type cityRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

type asnRecord struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

func pickName(names map[string]string) string {
	if len(names) == 0 {
		return ""
	}
	for _, key := range []string{"zh-CN", "zh", "en"} {
		if v, ok := names[key]; ok && v != "" {
			return v
		}
	}
	for _, v := range names {
		return v
	}
	return ""
}
