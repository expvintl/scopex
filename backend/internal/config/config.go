package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/expvintl/scopex/internal/logging"
)

// Config 是服务端的全局配置，全部来自环境变量（见 README）。
type Config struct {
	Listen    string
	MongoURI  string
	MongoDB   string
	JWTSecret string
	TokenTTL  time.Duration
	AdminUser string
	AdminPass string
	WebDir    string
	DevMode   bool
	MaxExport int

	Scanner ScannerConfig
	Geo     GeoConfig
	Log     LogConfig
}

// LogConfig 控制屏幕与日志文件的详细程度。
type LogConfig struct {
	// File 为日志文件路径，空表示只写屏幕。
	File         string
	Level        slog.Level // 文件级别，默认 info
	ConsoleLevel slog.Level // 屏幕级别，默认 warn
	MaxMB        int
	Backups      int
}

type ScannerConfig struct {
	MasscanPath     string
	NmapPath        string
	MaxRate         int
	MaxConcurrency  int
	MaxTargets      int
	DiscoverTimeout time.Duration
	ServiceTimeout  time.Duration
	ProbeTimeout    time.Duration
	DenyCIDRs       []string
}

type GeoConfig struct {
	Enabled      bool
	Dir          string
	AutoDownload bool
	CityURL      string
	ASNURL       string
	Timeout      time.Duration
}

func Load() Config {
	c := Config{
		Listen:    env("SCOPEX_LISTEN", ":8088"),
		MongoURI:  env("SCOPEX_MONGO_URI", "mongodb://127.0.0.1:27017"),
		MongoDB:   env("SCOPEX_MONGO_DB", "scopex"),
		JWTSecret: env("SCOPEX_JWT_SECRET", ""),
		TokenTTL:  envDuration("SCOPEX_TOKEN_TTL", 24*time.Hour),
		AdminUser: env("SCOPEX_ADMIN_USER", "admin"),
		AdminPass: env("SCOPEX_ADMIN_PASS", ""),
		WebDir:    env("SCOPEX_WEB_DIR", ""),
		DevMode:   envBool("SCOPEX_DEV", false),
		MaxExport: envInt("SCOPEX_MAX_EXPORT", 50000),
		Scanner: ScannerConfig{
			MasscanPath:     env("SCOPEX_MASSCAN", "masscan"),
			NmapPath:        env("SCOPEX_NMAP", "nmap"),
			MaxRate:         envInt("SCOPEX_MAX_RATE", 20000),
			MaxConcurrency:  envInt("SCOPEX_MAX_CONCURRENCY", 512),
			MaxTargets:      envInt("SCOPEX_MAX_TARGETS", 65536),
			DiscoverTimeout: envDuration("SCOPEX_DISCOVER_TIMEOUT", 30*time.Minute),
			ServiceTimeout:  envDuration("SCOPEX_SERVICE_TIMEOUT", 45*time.Minute),
			ProbeTimeout:    envDuration("SCOPEX_PROBE_TIMEOUT", 10*time.Second),
			DenyCIDRs:       splitList(env("SCOPEX_DENY_CIDRS", "224.0.0.0/4,240.0.0.0/4")),
		},
		Log: LogConfig{
			File:         logFile(env("SCOPEX_LOG_FILE", "data/logs/scopex.log")),
			Level:        logging.ParseLevel(env("SCOPEX_LOG_LEVEL", "info"), slog.LevelInfo),
			ConsoleLevel: logging.ParseLevel(env("SCOPEX_LOG_CONSOLE_LEVEL", "warn"), slog.LevelWarn),
			MaxMB:        envInt("SCOPEX_LOG_MAX_MB", 20),
			Backups:      envInt("SCOPEX_LOG_BACKUPS", 3),
		},
		Geo: GeoConfig{
			Enabled:      envBool("SCOPEX_GEO_ENABLED", true),
			Dir:          env("SCOPEX_GEO_DIR", "data/geoip"),
			AutoDownload: envBool("SCOPEX_GEO_AUTO_DOWNLOAD", true),
			CityURL:      env("SCOPEX_GEO_CITY_URL", "https://download.db-ip.com/free/dbip-city-lite-{month}.mmdb.gz"),
			ASNURL:       env("SCOPEX_GEO_ASN_URL", "https://download.db-ip.com/free/dbip-asn-lite-{month}.mmdb.gz"),
			Timeout:      envDuration("SCOPEX_GEO_TIMEOUT", 6*time.Second),
		},
	}
	// JWTSecret 留空表示“未通过环境变量指定”：启动时会生成随机密钥并保存到数据库。
	// 这里不再提供固定默认值，否则任何人都能用仓库里的常量伪造管理员令牌。
	return c
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// logFile 解析日志路径，“off/none/-” 表示关闭文件日志。
func logFile(v string) string {
	switch strings.ToLower(v) {
	case "off", "none", "-", "stdout":
		return ""
	}
	return v
}
