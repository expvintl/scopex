package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/expvintl/scopex/internal/i18n"
	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/query"
	"github.com/expvintl/scopex/internal/scanner"
	"github.com/expvintl/scopex/internal/util"
)

var macPattern = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)

func (s *Server) overview(c *gin.Context) {
	ov, err := s.store.BuildOverview(c.Request.Context())
	if err != nil {
		failf(c, http.StatusInternalServerError, "统计失败：%s", i18n.Message(lang(c), err))
		return
	}
	ok(c, ov)
}

func (s *Server) tools(c *gin.Context) {
	tools := s.registry.Statuses()
	for i := range tools {
		tools[i].Purpose = i18n.T(lang(c), tools[i].Purpose)
		tools[i].Reason = i18n.T(lang(c), tools[i].Reason)
	}
	ok(c, gin.H{
		"tools": tools,
		"limits": gin.H{
			"maxTargets": s.cfg.Scanner.MaxTargets,
			"maxRate":    s.cfg.Scanner.MaxRate,
			"denyCIDRs":  s.engine.DenyCIDRs(),
		},
		"geo": s.geo.Status(),
	})
}

// scanSettings 返回持久化的扫描参数，以及本机网卡与默认路由供前端自动探测。
func (s *Server) scanSettings(c *gin.Context) {
	st, err := s.store.GetScanSettings(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取扫描设置失败")
		return
	}
	ok(c, gin.H{
		"settings":     st,
		"interfaces":   scanner.ListInterfaces(),
		"defaultRoute": scanner.DefaultRoute(),
	})
}

func (s *Server) saveScanSettings(c *gin.Context) {
	var req model.ScanSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.ScanInterface = strings.TrimSpace(req.ScanInterface)
	req.MasscanInterface = strings.TrimSpace(req.MasscanInterface)
	req.MasscanRouterMAC = strings.TrimSpace(req.MasscanRouterMAC)
	req.MasscanExtraArgs = strings.TrimSpace(req.MasscanExtraArgs)

	if req.ScanInterface != "" && !scanner.InterfaceExists(req.ScanInterface) {
		failf(c, http.StatusBadRequest, "网卡不存在：%s", req.ScanInterface)
		return
	}
	if req.MasscanInterface != "" && !scanner.InterfaceExists(req.MasscanInterface) {
		failf(c, http.StatusBadRequest, "masscan 网卡不存在：%s", req.MasscanInterface)
		return
	}
	if req.MasscanRouterMAC != "" && !macPattern.MatchString(req.MasscanRouterMAC) {
		fail(c, http.StatusBadRequest, "网关 MAC 格式应为 aa:bb:cc:dd:ee:ff")
		return
	}
	if len(req.DenyCIDRs) > 128 {
		fail(c, http.StatusBadRequest, "禁止扫描网段不能超过 128 条")
		return
	}
	for _, item := range req.DenyCIDRs {
		if _, err := util.ParseTarget(strings.TrimSpace(item)); err != nil {
			failf(c, http.StatusBadRequest, "无效的地址段：%s", item)
			return
		}
	}
	req.DefaultPorts = strings.ReplaceAll(strings.TrimSpace(req.DefaultPorts), " ", "")
	if req.DefaultPorts != "" {
		if len(req.DefaultPorts) > 500 {
			fail(c, http.StatusBadRequest, "默认端口不能超过 500 字符")
			return
		}
		if _, err := scanner.ParsePorts(req.DefaultPorts); err != nil {
			failf(c, http.StatusBadRequest, "默认端口格式错误：%s", i18n.Message(lang(c), err))
			return
		}
	}
	if len(req.MasscanExtraArgs) > 500 {
		fail(c, http.StatusBadRequest, "masscan 额外参数不能超过 500 字符")
		return
	}
	if err := s.store.SaveScanSettings(c.Request.Context(), req); err != nil {
		failf(c, http.StatusInternalServerError, "保存扫描设置失败：%s", i18n.Message(lang(c), err))
		return
	}
	if err := s.engine.SetDenyCIDRs(req.DenyCIDRs); err != nil {
		failf(c, http.StatusBadRequest, "禁止扫描网段无效：%s", i18n.Message(lang(c), err))
		return
	}
	s.registry.ApplySettings(req)
	ok(c, req)
}

func (s *Server) querySyntax(c *gin.Context) {
	docs := make([]query.FieldDoc, len(query.FieldDocs))
	copy(docs, query.FieldDocs)
	for i := range docs {
		docs[i].Desc = i18n.T(lang(c), docs[i].Desc)
		docs[i].Example = i18n.T(lang(c), docs[i].Example)
	}
	ok(c, docs)
}

func (s *Server) listUsers(c *gin.Context) {
	users, err := s.store.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取用户失败")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	ok(c, users)
}

func (s *Server) createUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 {
		fail(c, http.StatusBadRequest, "用户名至少 3 个字符")
		return
	}
	if len(req.Password) < 8 {
		fail(c, http.StatusBadRequest, "密码至少 8 位")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	if req.Role != "admin" && req.Role != "viewer" {
		fail(c, http.StatusBadRequest, "角色只能是 admin 或 viewer")
		return
	}
	u, err := s.store.CreateUser(c.Request.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		failf(c, http.StatusBadRequest, "创建用户失败：%s", i18n.Message(lang(c), err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": u})
}
