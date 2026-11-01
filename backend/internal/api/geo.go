package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/expvintl/scopex/internal/model"
)

// geoSettings 返回地理库状态与当前配置，供设置页展示与编辑。
func (s *Server) geoSettings(c *gin.Context) {
	stored, found, err := s.store.GetGeoSettings(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取地理库设置失败")
		return
	}
	city, asn := s.geo.Sources()
	if !found {
		stored = model.GeoSettings{
			Enabled:      s.geo.Enabled(),
			AutoDownload: s.cfg.Geo.AutoDownload,
		}
	}
	if stored.CityURL == "" {
		stored.CityURL = city
	}
	if stored.ASNURL == "" {
		stored.ASNURL = asn
	}
	ok(c, gin.H{"status": s.geo.Status(), "settings": stored})
}

// saveGeoSettings 保存自定义下载源并即时生效。
func (s *Server) saveGeoSettings(c *gin.Context) {
	var req model.GeoSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.CityURL = strings.TrimSpace(req.CityURL)
	req.ASNURL = strings.TrimSpace(req.ASNURL)
	if req.CityURL == "" || req.ASNURL == "" {
		fail(c, http.StatusBadRequest, "两个下载地址都不能为空")
		return
	}
	for _, u := range []string{req.CityURL, req.ASNURL} {
		parsed, err := url.Parse(u)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			fail(c, http.StatusBadRequest, "下载地址必须是 http(s) 链接："+u)
			return
		}
	}
	if err := s.store.SaveGeoSettings(c.Request.Context(), req); err != nil {
		fail(c, http.StatusInternalServerError, "保存地理库设置失败")
		return
	}
	s.geo.Configure(req.Enabled, req.AutoDownload, req.CityURL, req.ASNURL)
	if req.Enabled && !s.geo.Status().Ready {
		go s.refreshGeoAsync()
	}
	ok(c, gin.H{"status": s.geo.Status(), "settings": req})
}

// refreshGeo 立即拉取最新库；下载在后台进行，通过状态接口轮询进度。
func (s *Server) refreshGeo(c *gin.Context) {
	if !s.geo.Enabled() {
		fail(c, http.StatusBadRequest, "地理库未启用")
		return
	}
	if s.geo.Status().Downloading {
		ok(c, gin.H{"started": false, "status": s.geo.Status()})
		return
	}
	go s.refreshGeoAsync()
	ok(c, gin.H{"started": true, "status": s.geo.Status()})
}

func (s *Server) refreshGeoAsync() {
	if err := s.geo.Refresh(context.Background()); err != nil {
		s.log.Warn("地理库更新失败", "err", err)
	}
}
