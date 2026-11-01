package query

import (
	"net"
	"regexp"
	"github.com/expvintl/scopex/internal/i18n"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"github.com/expvintl/scopex/internal/util"
)

// FieldDoc 描述可搜索字段，供前端语法提示使用。
type FieldDoc struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Desc    string `json:"desc"`
	Example string `json:"example"`
}

var FieldDocs = []FieldDoc{
	{"app", "string", "应用/组件指纹", `app:nginx`},
	{"service", "string", "服务名称", `service:http`},
	{"port", "int", "端口号，支持范围", `port:[80 TO 8080]`},
	{"ip", "ip", "IP 地址或网段", `ip:192.168.1.0/24`},
	{"title", "text", "HTTP 页面标题", `title:"管理后台"`},
	{"banner", "text", "原始响应内容", `banner:thinkphp`},
	{"server", "text", "HTTP 响应头", `server:nginx`},
	{"header", "text", "HTTP 响应头", `header:"X-Powered-By"`},
	{"country", "string", "国家代码或名称", `country:CN`},
	{"city", "string", "城市", `city:北京`},
	{"province", "string", "省份", `province:广东`},
	{"isp", "string", "运营商", `isp:电信`},
	{"asn", "int/string", "AS 号或组织", `asn:4134`},
	{"os", "string", "操作系统", `os:linux`},
	{"device", "string", "设备类型", `device:router`},
	{"hostname", "string", "主机名", `hostname:example.com`},
	{"domain", "string", "域名", `domain:example.com`},
	{"cert", "text", "证书主题或签发者", `cert:example.com`},
	{"favicon", "int/hash", "favicon 的 mmh3 值或 md5", `favicon:708578229`},
	{"tag", "string", "资产标签", `tag:核心`},
	{"protocol", "string", "协议 tcp/udp", `protocol:tcp`},
	{"status", "int", "HTTP 状态码", `status:200`},
	{"last_seen", "time", "最近发现时间", `last_seen:7d`},
	{"first_seen", "time", "首次发现时间", `first_seen:2024-01-01`},
}

var textFields = map[string]string{
	"title":   "title",
	"banner":  "banner",
	"body":    "banner",
	"header":  "header_text",
	"headers": "header_text",
	"server":  "header_text",
	"resp":    "header_text",
}

var intFields = map[string]string{
	"port":   "port",
	"status": "status_code",
	"code":   "status_code",
}

var arrayTextFields = map[string]string{
	"hostname": "hostnames",
	"domain":   "domains",
	"tag":      "tags",
	"tags":     "tags",
	"cpe":      "cpe",
}

var containsFields = map[string]string{
	"os":       "os",
	"device":   "device",
	"city":     "geo.city",
	"province": "geo.province",
	"region":   "geo.province",
	"isp":      "geo.isp",
	"version":  "service.version",
}

var timeFields = map[string]string{
	"last_seen":  "last_seen",
	"first_seen": "first_seen",
	"time":       "last_seen",
	"updated":    "last_seen",
}

func CompileString(input string) (bson.M, error) {
	n, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return Compile(n)
}

func Compile(n Node) (bson.M, error) {
	if n == nil {
		return bson.M{}, nil
	}
	return compileNode(n, time.Now())
}

func compileNode(n Node, now time.Time) (bson.M, error) {
	switch t := n.(type) {
	case AndNode:
		l, err := compileNode(t.Left, now)
		if err != nil {
			return nil, err
		}
		r, err := compileNode(t.Right, now)
		if err != nil {
			return nil, err
		}
		return bson.M{"$and": []bson.M{l, r}}, nil
	case OrNode:
		l, err := compileNode(t.Left, now)
		if err != nil {
			return nil, err
		}
		r, err := compileNode(t.Right, now)
		if err != nil {
			return nil, err
		}
		return bson.M{"$or": []bson.M{l, r}}, nil
	case NotNode:
		child, err := compileNode(t.Child, now)
		if err != nil {
			return nil, err
		}
		return bson.M{"$nor": []bson.M{child}}, nil
	case TextNode:
		return compileFreeText(t.Value)
	case FieldNode:
		return compileField(t, now)
	}
	return nil, i18n.Errorf("不支持的查询节点")
}

func regexCI(v string) primitive.Regex {
	return primitive.Regex{Pattern: regexp.QuoteMeta(v), Options: "i"}
}

func compileFreeText(v string) (bson.M, error) {
	if v == "" {
		return bson.M{}, nil
	}
	if r, err := util.ParseTarget(v); err == nil && !strings.Contains(v, "-") {
		return bson.M{"ipnum": bson.M{"$gte": r.Start, "$lte": r.End}}, nil
	}
	re := regexCI(v)
	return bson.M{"$or": []bson.M{
		{"title": re},
		{"banner": re},
		{"header_text": re},
		{"service.product": re},
		{"apps": re},
		{"service.name": re},
	}}, nil
}

func compileField(f FieldNode, now time.Time) (bson.M, error) {
	op := f.Op
	val := strings.TrimSpace(f.Value)
	if f.Range != nil {
		return compileRange(f)
	}
	if val == "" {
		return nil, i18n.Errorf("字段 %s 缺少查询值", f.Field)
	}

	if path, ok := textFields[f.Field]; ok {
		return compileTextOp(path, op, val)
	}
	if path, ok := intFields[f.Field]; ok {
		return compileIntOp(path, op, val)
	}
	if path, ok := arrayTextFields[f.Field]; ok {
		return compileTextOp(path, op, val)
	}
	if path, ok := containsFields[f.Field]; ok {
		return compileTextOp(path, op, val)
	}
	if path, ok := timeFields[f.Field]; ok {
		return compileTimeOp(path, op, val, now)
	}

	switch f.Field {
	case "app", "product", "component":
		return compileAppOp(op, val)
	case "ip", "host", "addr":
		return compileIPOp(op, val)
	case "net", "cidr":
		return compileIPOp(":", val)
	case "service", "services":
		return compileTextOp("service.name", op, val)
	case "country", "country_code":
		return compileCountryOp(op, val)
	case "asn":
		return compileASNOp(op, val)
	case "org", "organization":
		return compileTextOp("asn.org", op, val)
	case "cert", "tls":
		return compileCertOp(op, val)
	case "favicon":
		return compileFaviconOp(op, val)
	case "protocol", "proto":
		return compileTextOp("protocol", op, val)
	case "private":
		b := val == "true" || val == "1"
		if op == "!=" {
			b = !b
		}
		return bson.M{"geo.private": b}, nil
	}
	return nil, i18n.Errorf("未知字段 %q，可输入 help 查看支持的字段", f.Field)
}

func compileTextOp(path, op, val string) (bson.M, error) {
	re := regexCI(val)
	switch op {
	case ":", "=", "~":
		return bson.M{path: re}, nil
	case "!=":
		return bson.M{path: bson.M{"$not": re}}, nil
	}
	return nil, i18n.Errorf("字段不支持 %s 运算", op)
}

func compileAppOp(op, val string) (bson.M, error) {
	re := regexCI(val)
	cond := bson.M{"$or": []bson.M{
		{"apps": re},
		{"service.product": re},
		{"service.name": re},
	}}
	switch op {
	case ":", "=", "~":
		return cond, nil
	case "!=":
		return bson.M{"$nor": []bson.M{cond}}, nil
	}
	return nil, i18n.Errorf("app 字段不支持 %s 运算", op)
}

func compileIntOp(path, op, val string) (bson.M, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return nil, i18n.Errorf("%s 需要数字，收到 %q", path, val)
	}
	switch op {
	case ":", "=":
		return bson.M{path: n}, nil
	case "!=":
		return bson.M{path: bson.M{"$ne": n}}, nil
	case ">":
		return bson.M{path: bson.M{"$gt": n}}, nil
	case "<":
		return bson.M{path: bson.M{"$lt": n}}, nil
	case ">=":
		return bson.M{path: bson.M{"$gte": n}}, nil
	case "<=":
		return bson.M{path: bson.M{"$lte": n}}, nil
	}
	return nil, i18n.Errorf("字段不支持 %s 运算", op)
}

func compileIPOp(op, val string) (bson.M, error) {
	r, err := util.ParseTarget(val)
	if err != nil {
		return nil, err
	}
	cond := bson.M{"ipnum": bson.M{"$gte": r.Start, "$lte": r.End}}
	switch op {
	case ":", "=":
		return cond, nil
	case "!=":
		return bson.M{"$nor": []bson.M{cond}}, nil
	}
	return nil, i18n.Errorf("ip 字段不支持 %s 运算", op)
}

func compileCountryOp(op, val string) (bson.M, error) {
	upper := strings.ToUpper(val)
	conds := []bson.M{{"geo.country_code": upper}}
	if len(val) > 2 {
		conds = []bson.M{{"geo.country": regexCI(val)}, {"geo.country_code": upper}}
	}
	cond := bson.M{"$or": conds}
	switch op {
	case ":", "=", "~":
		return cond, nil
	case "!=":
		return bson.M{"$nor": conds}, nil
	}
	return nil, i18n.Errorf("country 字段不支持 %s 运算", op)
}

func compileASNOp(op, val string) (bson.M, error) {
	if n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(val), "AS")); err == nil {
		return compileIntOp("asn.number", op, strconv.Itoa(n))
	}
	return compileTextOp("asn.org", op, val)
}

func compileCertOp(op, val string) (bson.M, error) {
	re := regexCI(val)
	cond := bson.M{"$or": []bson.M{
		{"tls.subject": re},
		{"tls.issuer": re},
		{"tls.sans": re},
		{"tls.common_name": re},
	}}
	switch op {
	case ":", "=", "~":
		return cond, nil
	case "!=":
		return bson.M{"$nor": []bson.M{cond}}, nil
	}
	return nil, i18n.Errorf("cert 字段不支持 %s 运算", op)
}

func compileFaviconOp(op, val string) (bson.M, error) {
	if n, err := strconv.ParseInt(val, 10, 32); err == nil {
		return compileIntOp("favicon.mmh3", op, strconv.FormatInt(n, 10))
	}
	clean := strings.ToLower(strings.TrimSpace(val))
	if len(clean) != 32 {
		return nil, i18n.Errorf("favicon 需为 mmh3 数字或 32 位 md5")
	}
	if op == "!=" {
		return bson.M{"favicon.md5": bson.M{"$ne": clean}}, nil
	}
	return bson.M{"favicon.md5": clean}, nil
}

func compileTimeOp(path, op, val string, now time.Time) (bson.M, error) {
	t, err := parseTimeValue(val, now)
	if err != nil {
		return nil, err
	}
	switch op {
	case ":", "=":
		return bson.M{path: bson.M{"$gte": t}}, nil
	case "!=":
		return bson.M{path: bson.M{"$lt": t}}, nil
	case ">":
		return bson.M{path: bson.M{"$gt": t}}, nil
	case "<":
		return bson.M{path: bson.M{"$lt": t}}, nil
	case ">=":
		return bson.M{path: bson.M{"$gte": t}}, nil
	case "<=":
		return bson.M{path: bson.M{"$lte": t}}, nil
	}
	return nil, i18n.Errorf("时间字段不支持 %s 运算", op)
}

func parseTimeValue(v string, now time.Time) (time.Time, error) {
	lower := strings.ToLower(strings.TrimSpace(v))
	if strings.HasSuffix(lower, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(lower, "d")); err == nil {
			return now.AddDate(0, 0, -n), nil
		}
	}
	if d, err := time.ParseDuration(lower); err == nil {
		return now.Add(-d), nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, i18n.Errorf("无法识别的时间 %q，示例：7d、24h、2024-01-01", v)
}

func compileRange(f FieldNode) (bson.M, error) {
	path := ""
	isIP := false
	switch {
	case f.Field == "port":
		path = "port"
	case f.Field == "status" || f.Field == "code":
		path = "status_code"
	case f.Field == "ip" || f.Field == "net" || f.Field == "cidr" || f.Field == "host":
		path = "ipnum"
		isIP = true
	case f.Field == "asn":
		path = "asn.number"
	case f.Field == "last_seen" || f.Field == "first_seen" || f.Field == "time":
		path = timeFields[f.Field]
	default:
		return nil, i18n.Errorf("字段 %s 不支持范围查询", f.Field)
	}

	if isIP {
		cond := bson.M{}
		if f.Range.HasFrom {
			r, err := util.ParseTarget(f.Range.From)
			if err != nil {
				return nil, err
			}
			cond["$gte"] = r.Start
		}
		if f.Range.HasTo {
			r, err := util.ParseTarget(f.Range.To)
			if err != nil {
				return nil, err
			}
			cond["$lte"] = r.End
		}
		return bson.M{path: cond}, nil
	}

	cond := bson.M{}
	if f.Range.HasFrom {
		if path == "last_seen" || path == "first_seen" {
			t, err := parseTimeValue(f.Range.From, time.Now())
			if err != nil {
				return nil, err
			}
			cond["$gte"] = t
		} else {
			n, err := strconv.Atoi(f.Range.From)
			if err != nil {
				return nil, i18n.Errorf("%s 范围边界需要数字", f.Field)
			}
			cond["$gte"] = n
		}
	}
	if f.Range.HasTo {
		if path == "last_seen" || path == "first_seen" {
			t, err := parseTimeValue(f.Range.To, time.Now())
			if err != nil {
				return nil, err
			}
			cond["$lte"] = t
		} else {
			n, err := strconv.Atoi(f.Range.To)
			if err != nil {
				return nil, i18n.Errorf("%s 范围边界需要数字", f.Field)
			}
			cond["$lte"] = n
		}
	}
	return bson.M{path: cond}, nil
}

// ParseIPLike 报告字符串是否是一个 IP 或 CIDR，用于搜索结果分类。
func ParseIPLike(s string) (net.IP, bool) {
	if ip := net.ParseIP(s); ip != nil {
		return ip, true
	}
	if _, n, err := net.ParseCIDR(s); err == nil {
		return n.IP, true
	}
	return nil, false
}
