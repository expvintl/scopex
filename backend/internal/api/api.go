package api

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/geo"
	"github.com/expvintl/scopex/internal/job"
	"github.com/expvintl/scopex/internal/scanner"
	"github.com/expvintl/scopex/internal/store"
	"github.com/expvintl/scopex/internal/version"
)

type Server struct {
	cfg      config.Config
	store    *store.Store
	engine   *job.Engine
	registry *scanner.Registry
	geo      *geo.Resolver
	log      *slog.Logger
	web      fs.FS
}

func New(cfg config.Config, st *store.Store, engine *job.Engine, reg *scanner.Registry,
	g *geo.Resolver, logger *slog.Logger, web fs.FS) *Server {
	return &Server{cfg: cfg, store: st, engine: engine, registry: reg, geo: g, log: logger, web: web}
}

func (s *Server) Router() *gin.Engine {
	if s.cfg.DevMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.MaxMultipartMemory = 8 << 20
	r.Use(gin.Recovery(), s.accessLog())
	if s.cfg.DevMode {
		r.Use(devCORS())
	}

	api := r.Group("/api")
	api.GET("/health", s.health)
	api.POST("/auth/login", s.login)
	api.GET("/meta/query-syntax", s.querySyntax)

	auth := api.Group("", s.requireAuth())
	{
		// 自助接口：任何登录用户都能改自己的密码与 API Key。
		auth.GET("/auth/me", s.me)
		auth.POST("/auth/password", s.changePassword)
		auth.POST("/auth/apikey", s.rotateAPIKey)

		// 只读接口：viewer 可访问。
		auth.GET("/hosts", s.searchHosts)
		auth.GET("/hosts/facets", s.hostFacets)
		auth.GET("/hosts/export", s.exportHosts)
		auth.GET("/ips/:ip", s.ipHosts)
		auth.GET("/hosts/:id", s.getHost)
		auth.GET("/hosts/:id/favicon", s.hostFavicon)
		auth.GET("/tasks", s.listTasks)
		auth.GET("/tasks/:id", s.getTask)
		auth.GET("/tasks/:id/logs", s.taskLogs)
		auth.GET("/stats/overview", s.overview)
		auth.GET("/system/tools", s.tools)
		auth.GET("/system/scan-settings", s.scanSettings)
		auth.GET("/system/geo", s.geoSettings)

		// 会发起探测或改动数据的接口一律要求管理员，viewer 是只读角色。
		auth.POST("/ips/:ip/refresh", s.requireAdmin(), s.refreshIP)
		auth.DELETE("/hosts/:id", s.requireAdmin(), s.deleteHost)

		auth.POST("/tasks", s.requireAdmin(), s.createTask)
		auth.POST("/tasks/:id/cancel", s.requireAdmin(), s.cancelTask)
		auth.DELETE("/tasks", s.requireAdmin(), s.deleteFinishedTasks)
		auth.DELETE("/tasks/:id", s.requireAdmin(), s.deleteTask)
		auth.DELETE("/hosts", s.requireAdmin(), s.deleteHostsByIP)

		auth.PUT("/system/scan-settings", s.requireAdmin(), s.saveScanSettings)
		auth.PUT("/system/geo", s.requireAdmin(), s.saveGeoSettings)
		auth.POST("/system/geo/refresh", s.requireAdmin(), s.refreshGeo)

		users := auth.Group("/users", s.requireAdmin())
		users.GET("", s.listUsers)
		users.POST("", s.createUser)
	}

	r.NoRoute(s.serveWeb)
	return r
}

func (s *Server) health(c *gin.Context) {
	ok(c, gin.H{
		"status":  "ok",
		"version": version.Version,
		"time":    time.Now(),
		"tools":   s.registry.Statuses(),
	})
}

func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			return
		}
		s.log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

func devCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) serveWeb(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		fail(c, http.StatusNotFound, "接口不存在")
		return
	}
	if s.web == nil {
		fail(c, http.StatusNotFound, "前端资源未构建")
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/")
	if name != "" {
		if f, err := s.web.Open(name); err == nil {
			defer f.Close()
			if st, err := f.Stat(); err == nil && !st.IsDir() {
				if rs, ok := f.(io.ReadSeeker); ok {
					http.ServeContent(c.Writer, c.Request, name, st.ModTime(), rs)
					return
				}
			}
		}
	}
	data, err := fs.ReadFile(s.web, "index.html")
	if err != nil {
		fail(c, http.StatusNotFound, "前端资源未构建")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}
