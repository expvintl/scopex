package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/expvintl/scopex/internal/i18n"
	"github.com/expvintl/scopex/internal/job"
	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/query"
	"github.com/expvintl/scopex/internal/store"
	"github.com/expvintl/scopex/internal/util"
)

func compileFilter(raw string) (bson.M, error) {
	return query.CompileString(strings.TrimSpace(raw))
}

// normalizeHost 保证 JSON 中集合字段始终为数组，避免前端处理 null。
func normalizeHost(h *model.Host) {
	if h.Apps == nil {
		h.Apps = []string{}
	}
	if h.CPE == nil {
		h.CPE = []string{}
	}
	if h.Hostnames == nil {
		h.Hostnames = []string{}
	}
	if h.Domains == nil {
		h.Domains = []string{}
	}
	if h.Tags == nil {
		h.Tags = []string{}
	}
	if h.Headers == nil {
		h.Headers = map[string]string{}
	}
	if h.TLS != nil && h.TLS.SANs == nil {
		h.TLS.SANs = []string{}
	}
}

func (s *Server) searchHosts(c *gin.Context) {
	filter, err := compileFilter(c.Query("q"))
	if err != nil {
		failf(c, http.StatusBadRequest, "查询语法错误: %s", i18n.Message(lang(c), err))
		return
	}
	page, size := pagination(c, 20, 200)
	if c.Query("group") == "ip" {
		groups, total, err := s.store.SearchIPGroups(c.Request.Context(), store.HostQuery{
			Filter:  filter,
			SortKey: c.DefaultQuery("sort", "last_seen"),
			Desc:    c.DefaultQuery("desc", "true") != "false",
			Page:    page,
			Size:    size,
		}, 12)
		if err != nil {
			failf(c, http.StatusInternalServerError, "查询失败: %s", i18n.Message(lang(c), err))
			return
		}
		for i := range groups {
			normalizeIPGroup(&groups[i])
		}
		okList(c, groups, total, page, size)
		return
	}
	items, total, err := s.store.SearchHosts(c.Request.Context(), store.HostQuery{
		Filter:  filter,
		SortKey: c.DefaultQuery("sort", "last_seen"),
		Desc:    c.DefaultQuery("desc", "true") != "false",
		Page:    page,
		Size:    size,
	})
	if err != nil {
		failf(c, http.StatusInternalServerError, "查询失败: %s", i18n.Message(lang(c), err))
		return
	}
	if items == nil {
		items = []model.Host{}
	}
	for i := range items {
		normalizeHost(&items[i])
	}
	okList(c, items, total, page, size)
}

func normalizeIPGroup(g *store.IPGroup) {
	if g.Hostnames == nil {
		g.Hostnames = []string{}
	}
	if g.Domains == nil {
		g.Domains = []string{}
	}
	if g.Ports == nil {
		g.Ports = []store.PortBrief{}
	}
	for i := range g.Ports {
		if g.Ports[i].Apps == nil {
			g.Ports[i].Apps = []string{}
		}
	}
}

// ipHosts 返回某个 IP 下的全部端口资产。
func (s *Server) ipHosts(c *gin.Context) {
	ip := c.Param("ip")
	if net.ParseIP(ip) == nil {
		fail(c, http.StatusBadRequest, "IP 格式不正确")
		return
	}
	items, err := s.store.HostsByIP(c.Request.Context(), ip, 500)
	if err != nil {
		failf(c, http.StatusInternalServerError, "查询失败: %s", i18n.Message(lang(c), err))
		return
	}
	if len(items) == 0 {
		fail(c, http.StatusNotFound, "该 IP 下没有资产")
		return
	}
	for i := range items {
		normalizeHost(&items[i])
	}
	okList(c, items, int64(len(items)), 1, len(items))
}

// refreshIP 重新探测该 IP 下已记录的端口，并刷新归属地信息。
func (s *Server) refreshIP(c *gin.Context) {
	ip := c.Param("ip")
	if net.ParseIP(ip) == nil {
		fail(c, http.StatusBadRequest, "IP 格式不正确")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	n, err := s.engine.RefreshIP(ctx, ip)
	if errors.Is(err, job.ErrNoAssets) {
		fail(c, http.StatusNotFound, err.Error())
		return
	}
	if err != nil && n == 0 {
		fail(c, http.StatusGatewayTimeout, "刷新超时，请稍后重试")
		return
	}
	ok(c, gin.H{"ip": ip, "refreshed": n, "partial": err != nil})
}

func (s *Server) hostFacets(c *gin.Context) {
	filter, err := compileFilter(c.Query("q"))
	if err != nil {
		failf(c, http.StatusBadRequest, "查询语法错误: %s", i18n.Message(lang(c), err))
		return
	}
	facets, err := s.store.HostFacets(c.Request.Context(), filter, 15)
	if err != nil {
		failf(c, http.StatusInternalServerError, "聚合失败: %s", i18n.Message(lang(c), err))
		return
	}
	ok(c, facets)
}

// deleteHost 由管理员删除单条资产。
func (s *Server) deleteHost(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的资产 ID")
		return
	}
	if err := s.store.DeleteHost(c.Request.Context(), id); err != nil {
		if store.IsNotFound(err) {
			fail(c, http.StatusNotFound, "资产不存在")
			return
		}
		failf(c, http.StatusInternalServerError, "删除资产失败：%s", i18n.Message(lang(c), err))
		return
	}
	ok(c, gin.H{"deleted": id.Hex()})
}

// deleteHostsByIP 按 IP、CIDR 或通配符批量删除资产。
func (s *Server) deleteHostsByIP(c *gin.Context) {
	ctx := c.Request.Context()

	if ip := strings.TrimSpace(c.Query("ip")); ip != "" {
		if net.ParseIP(ip) == nil {
			fail(c, http.StatusBadRequest, "无效的 IP 地址")
			return
		}
		n, err := s.store.DeleteHostsByIP(ctx, ip)
		if err != nil {
			failf(c, http.StatusInternalServerError, "删除资产失败：%s", i18n.Message(lang(c), err))
			return
		}
		if n == 0 {
			fail(c, http.StatusNotFound, "该 IP 下没有资产")
			return
		}
		ok(c, gin.H{"mode": "ip", "value": ip, "deleted": n})
		return
	}

	if cidr := strings.TrimSpace(c.Query("cidr")); cidr != "" {
		filter, err := cidrFilter(cidr)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		n, err := s.store.DeleteHostsByFilter(ctx, filter)
		if err != nil {
			failf(c, http.StatusInternalServerError, "删除资产失败：%s", i18n.Message(lang(c), err))
			return
		}
		ok(c, gin.H{"mode": "cidr", "value": cidr, "deleted": n})
		return
	}

	if pattern := strings.TrimSpace(c.Query("pattern")); pattern != "" {
		filter, err := wildcardFilter(pattern)
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		n, err := s.store.DeleteHostsByFilter(ctx, filter)
		if err != nil {
			failf(c, http.StatusInternalServerError, "删除资产失败：%s", i18n.Message(lang(c), err))
			return
		}
		ok(c, gin.H{"mode": "pattern", "value": pattern, "deleted": n})
		return
	}

	fail(c, http.StatusBadRequest, "需要提供 ip、cidr 或 pattern 参数")
}

// cidrFilter 把 CIDR 转成 ipnum 区间查询条件。
func cidrFilter(cidr string) (bson.M, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil || ipnet.IP.To4() == nil {
		return nil, errors.New("无效的 CIDR：" + cidr)
	}
	start := uint64(util.IPToUint32(ipnet.IP.To4()))
	ones, bits := ipnet.Mask.Size()
	size := uint64(1) << uint(bits-ones)
	return bson.M{"ipnum": bson.M{"$gte": uint32(start), "$lte": uint32(start + size - 1)}}, nil
}

// wildcardFilter 把 192.168.*.* 这类通配符转成 IP 正则条件。
func wildcardFilter(pattern string) (bson.M, error) {
	if len(pattern) > 32 {
		return nil, errors.New("匹配模式过长")
	}
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch {
		case r == '*':
			b.WriteString(".*")
		case r == '?':
			b.WriteString(".")
		case r >= '0' && r <= '9', r == '.':
			b.WriteString(string(r))
		default:
			return nil, errors.New("通配符只支持数字、点和 * ?：" + pattern)
		}
	}
	b.WriteString("$")
	return bson.M{"ip": primitive.Regex{Pattern: b.String()}}, nil
}

func (s *Server) getHost(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的资产 ID")
		return
	}
	host, err := s.store.GetHost(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "资产不存在")
		return
	}
	normalizeHost(host)
	ok(c, host)
}

func (s *Server) hostFavicon(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的资产 ID")
		return
	}
	img, err := s.store.GetFavicon(c.Request.Context(), id)
	if err != nil || len(img) == 0 {
		fail(c, http.StatusNotFound, "该资产没有图标")
		return
	}
	c.Header("Cache-Control", "public, max-age=604800")
	c.Data(http.StatusOK, imageType(img), img)
}

func imageType(b []byte) string {
	switch {
	case len(b) > 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8:
		return "image/jpeg"
	case len(b) > 4 && string(b[:4]) == "GIF8":
		return "image/gif"
	case len(b) > 4 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0:
		return "image/x-icon"
	case bytes.HasPrefix(bytes.TrimSpace(b), []byte("<svg")) || bytes.HasPrefix(bytes.TrimSpace(b), []byte("<?xml")):
		return "image/svg+xml"
	}
	return "application/octet-stream"
}

func (s *Server) exportHosts(c *gin.Context) {
	filter, err := compileFilter(c.Query("q"))
	if err != nil {
		failf(c, http.StatusBadRequest, "查询语法错误: %s", i18n.Message(lang(c), err))
		return
	}
	items, _, err := s.store.SearchHosts(c.Request.Context(), store.HostQuery{
		Filter:  filter,
		SortKey: c.DefaultQuery("sort", "last_seen"),
		Desc:    true,
		Page:    1,
		Size:    s.cfg.MaxExport,
	})
	if err != nil {
		failf(c, http.StatusInternalServerError, "导出失败: %s", i18n.Message(lang(c), err))
		return
	}
	if items == nil {
		items = []model.Host{}
	}
	if c.DefaultQuery("format", "csv") == "json" {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="scopex-%s.json"`, time.Now().Format("20060102-150405")))
		c.JSON(http.StatusOK, items)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="scopex-%s.csv"`, time.Now().Format("20060102-150405")))
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"ip", "port", "protocol", "service", "product", "version", "status", "title", "apps", "cpe",
		"hostnames", "domains", "country", "province", "city", "isp", "asn", "os", "device", "favicon_mmh3", "last_seen"})
	for _, h := range items {
		asn := ""
		if h.ASN != nil {
			asn = "AS" + strconv.Itoa(h.ASN.Number) + " " + h.ASN.Org
		}
		mmh3 := ""
		if h.Favicon != nil {
			mmh3 = strconv.FormatInt(int64(h.Favicon.MMH3), 10)
		}
		w.Write([]string{
			h.IP, strconv.Itoa(h.Port), h.Protocol, h.Service.Name, h.Service.Product, h.Service.Version,
			strconv.Itoa(h.StatusCode), h.Title, strings.Join(h.Apps, "|"), strings.Join(h.CPE, "|"),
			strings.Join(h.Hostnames, "|"), strings.Join(h.Domains, "|"), h.Geo.Country, h.Geo.Province,
			h.Geo.City, h.Geo.ISP, asn, h.OS, h.Device, mmh3, h.LastSeen.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
}
