package fingerprint

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type Result struct {
	Service    model.Service
	Banner     string
	Title      string
	Headers    map[string]string
	HeaderText string
	StatusCode int
	Favicon    *model.Favicon
	FaviconImg []byte
	TLS        *model.TLSInfo
	Apps       []string
	CPE        []string
	Device     string
	OS         string
	Hostnames  []string
	Domains    []string
	Scripts    map[string]string
	AI         *model.AIInfo
}

type Prober struct {
	Timeout   time.Duration
	UserAgent string
	MaxBody   int
}

func NewProber(timeout time.Duration) *Prober {
	return &Prober{
		Timeout:   timeout,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36",
		MaxBody:   512 * 1024,
	}
}

// Probe 对一个端口做协议无关的识别：优先 HTTP(S)，失败后回落到 banner 抓取。
func (p *Prober) Probe(ctx context.Context, ip string, port int, hint string) *Result {
	useTLS := isTLSPort(port) || strings.Contains(hint, "ssl") || strings.Contains(hint, "https")

	if isMinecraftPort(port) || strings.EqualFold(hint, "minecraft") {
		if r := p.probeMinecraft(ctx, ip, port); r != nil {
			return r
		}
	}

	if port == 445 || hintIn(hint, "smb", "microsoft-ds", "netbios-ssn") {
		if r := p.probeSMB(ctx, ip, port); r != nil {
			return r
		}
	}
	if port == 5985 || port == 5986 || hintIn(hint, "winrm", "wsman", "wsmans") {
		if r := p.probeWinRM(ctx, ip, port); r != nil {
			return r
		}
	}
	if port == 64738 || hintIn(hint, "mumble") {
		if r := p.probeMumble(ctx, ip, port); r != nil {
			return r
		}
	}

	if isPassivePort(port) {
		if r := p.probePassive(ctx, ip, port); r != nil {
			return r
		}
	}
	if r, err := p.probeHTTP(ctx, ip, port, useTLS); err == nil && r != nil {
		return r
	}
	if useTLS {
		if r, err := p.probeHTTP(ctx, ip, port, false); err == nil && r != nil {
			return r
		}
	} else {
		if r, err := p.probeHTTP(ctx, ip, port, true); err == nil && r != nil {
			return r
		}
	}
	if r := p.probePassive(ctx, ip, port); r != nil {
		return r
	}
	return &Result{Service: model.Service{Name: guessService(port), Method: "guess"}}
}

func (p *Prober) probeHTTP(ctx context.Context, ip string, port int, useTLS bool) (*Result, error) {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	dialer := &net.Dialer{Timeout: p.Timeout}
	var conn net.Conn
	var err error
	if useTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         ip,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(p.Timeout))

	host := ip
	if !isDefaultPort(port, useTLS) {
		host = addr
	}
	reqText := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Language: zh-CN,zh;q=0.9,en;q=0.8\r\nConnection: close\r\n\r\n", host, p.UserAgent)
	if _, err := conn.Write([]byte(reqText)); err != nil {
		return nil, err
	}

	res := &Result{Headers: map[string]string{}}
	if tlsConn, ok := conn.(*tls.Conn); ok {
		state := tlsConn.ConnectionState()
		res.TLS = buildTLSInfo(state)
	}

	br := bufio.NewReaderSize(conn, 16*1024)
	head, err := br.Peek(5)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(head, []byte("HTTP/")) {
		data, _ := io.ReadAll(io.LimitReader(br, 8192))
		return p.analyzeBanner(port, data), nil
	}

	reqObj := &http.Request{Method: "GET", URL: &url.URL{Scheme: scheme, Host: addr, Path: "/"}}
	resp, err := http.ReadResponse(br, reqObj)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		key := strings.ToLower(name)
		res.Headers[key] = strings.Join(values, ", ")
	}
	res.StatusCode = resp.StatusCode

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, int64(p.MaxBody)))
	body := raw
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		if zr, err := gzip.NewReader(bytes.NewReader(raw)); err == nil {
			if dec, err := io.ReadAll(io.LimitReader(zr, int64(p.MaxBody))); err == nil {
				body = dec
			}
			zr.Close()
		}
	}
	text := decodeBody(body, resp.Header.Get("Content-Type"))
	res.Title = extractTitle(text)
	res.Banner = util.Truncate(text, 2000)
	res.HeaderText = flattenHeaders(res.Headers)
	res.Hostnames = append(res.Hostnames, hostnamesFromTLS(res.TLS)...)

	svc := model.Service{Name: "http", Method: "http", Confidence: 10}
	if useTLS {
		svc.Name = "https"
	}
	if server := res.Headers["server"]; server != "" {
		svc.Product, svc.Version = normalizeServer(server)
		res.OS = osFromServerHeader(server)
		svc.ExtraInfo = util.Truncate(server, 120)
	}
	if svc.Product == "" {
		if powered := res.Headers["x-powered-by"]; powered != "" {
			svc.Product = util.Truncate(strings.Split(powered, ",")[0], 60)
			svc.ExtraInfo = powered
		}
	}
	if res.Title != "" {
		svc.Banner = util.Truncate(res.Title, 120)
	}
	res.Service = svc
	res.Apps = p.matchHTTP(res.Headers, text, res.Title, svc.Product)

	if info := p.probeAI(ctx, ip, port, scheme, res); info != nil {
		applyAI(res, info)
	}

	if fav := p.fetchFavicon(ctx, scheme, ip, port, text); fav != nil {
		res.Favicon = fav.Favicon
		res.FaviconImg = fav.Data
	}
	res.Device = deviceFromApps(res.Apps)
	return res, nil
}

func (p *Prober) probePassive(ctx context.Context, ip string, port int) *Result {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(p.Timeout))

	if port == 6379 || port == 6380 {
		_, _ = conn.Write([]byte("PING\r\n"))
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return nil
	}
	res := p.analyzeBanner(port, buf[:n])
	if res.Service.Name == "" {
		return nil
	}
	return res
}

func (p *Prober) analyzeBanner(port int, data []byte) *Result {
	res := &Result{Headers: map[string]string{}}
	text := strings.ToValidUTF8(string(data), "")
	if i := strings.IndexAny(text, "\r\n"); i > 0 {
		text = text[:i]
	}
	res.Banner = util.Truncate(strings.TrimSpace(text), 2000)
	svc := model.Service{Name: guessService(port), Method: "banner", Banner: util.Truncate(res.Banner, 160)}

	lower := strings.ToLower(text)
	switch {
	case bytes.HasPrefix(data, []byte("AMQP")):
		svc.Name = "amqp"
		svc.Product = "AMQP"
		if len(data) >= 8 {
			svc.Version = fmt.Sprintf("%d.%d.%d", data[5], data[6], data[7])
		}
		if bytes.Contains(data, []byte("RabbitMQ")) {
			svc.Product = "RabbitMQ"
			if v := rabbitMQVersion(data); v != "" {
				svc.Version = v
			}
		}
	case strings.HasPrefix(text, "@RSYNCD:"):
		svc.Name = "rsync"
		svc.Product = "rsync"
		if idx := strings.Index(text, ":"); idx >= 0 {
			svc.Version = strings.TrimSpace(text[idx+1:])
		}
	case strings.HasPrefix(text, "SSH-"):
		svc.Name = "ssh"
		parts := strings.Fields(text)
		software := strings.TrimPrefix(parts[0], "SSH-2.0-")
		software = strings.TrimPrefix(software, "SSH-1.99-")
		if idx := strings.LastIndex(software, "_"); idx > 0 {
			svc.Product = software[:idx]
			svc.Version = software[idx+1:]
		} else {
			svc.Product = software
		}
		svc.ExtraInfo = util.Truncate(parts[0], 120)
		res.OS = osFromSSHSoftware(software)
	case strings.HasPrefix(text, "220") && (port == 21 || strings.Contains(lower, "ftp")):
		svc.Name = "ftp"
		svc.Product = "FTP"
	case strings.HasPrefix(text, "220") && (port == 25 || port == 587):
		svc.Name = "smtp"
	case strings.HasPrefix(text, "+PONG"):
		svc.Name = "redis"
		svc.Product = "Redis"
	case port == 6379 || port == 6380:
		svc.Name = "redis"
		svc.Product = "Redis"
	case strings.HasPrefix(text, "RFB "):
		svc.Name = "vnc"
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			svc.Product = "VNC"
			svc.Version = parts[1]
		}
	case strings.Contains(lower, "mysql_native_password") || (port == 3306 && len(data) > 10):
		svc.Name = "mysql"
		svc.Product = "MySQL"
		if v := mysqlVersion(data); v != "" {
			svc.Version = v
		}
	case strings.Contains(lower, "mongodb") || port == 27017:
		svc.Name = "mongodb"
		svc.Product = "MongoDB"
	case strings.HasPrefix(text, "HTTP/") || strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html"):
		svc.Name = "http"
	case port == 23:
		svc.Name = "telnet"
	case port == 5432:
		svc.Name = "postgresql"
		svc.Product = "PostgreSQL"
	case port == 5900:
		svc.Name = "vnc"
	case port == 3389:
		svc.Name = "rdp"
	case port == 11211:
		svc.Name = "memcached"
	case port == 9200:
		svc.Name = "http"
		svc.Product = "Elasticsearch"
	}

	res.Service = svc
	if svc.Name != "" && svc.Name != "unknown" {
		res.Apps = appendUnique(res.Apps, svc.Product)
	}
	return res
}

func mysqlVersion(data []byte) string {
	if len(data) < 6 {
		return ""
	}
	end := bytes.IndexByte(data[5:], 0)
	if end <= 0 {
		return ""
	}
	return util.Truncate(string(data[5:5+end]), 40)
}

type faviconResult struct {
	Favicon *model.Favicon
	Data    []byte
}

func (p *Prober) fetchFavicon(ctx context.Context, scheme, ip string, port int, body string) *faviconResult {
	if port != 80 && port != 443 && len(body) == 0 {
		return nil
	}
	candidates := faviconCandidates(body)
	client := &http.Client{
		Timeout: p.Timeout,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	for _, cand := range candidates {
		u := cand
		if !strings.HasPrefix(u, "http") {
			base := &url.URL{Scheme: scheme, Host: net.JoinHostPort(ip, strconv.Itoa(port))}
			ref, err := url.Parse(cand)
			if err != nil {
				continue
			}
			u = base.ResolveReference(ref).String()
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", p.UserAgent)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 200*1024))
		resp.Body.Close()
		if resp.StatusCode != 200 || len(data) == 0 || !looksLikeImage(data) {
			continue
		}
		sum := md5.Sum(data)
		return &faviconResult{
			Favicon: &model.Favicon{
				MD5:  hex.EncodeToString(sum[:]),
				MMH3: FaviconHash(data),
				URL:  util.Truncate(u, 200),
				Size: len(data),
			},
			Data: data,
		}
	}
	return nil
}

var (
	iconLinkRE = regexp.MustCompile(`(?is)<link[^>]+rel=["']?[^"'>]*icon[^"'>]*["']?[^>]*href=["']([^"']+)["']`)
	hrefIconRE = regexp.MustCompile(`(?is)<link[^>]+href=["']([^"']+)["'][^>]*rel=["']?[^"'>]*icon`)
)

func faviconCandidates(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, re := range []*regexp.Regexp{iconLinkRE, hrefIconRE} {
		for _, m := range re.FindAllStringSubmatch(body, 3) {
			href := strings.TrimSpace(html.UnescapeString(m[1]))
			if href == "" || strings.HasPrefix(href, "data:") || seen[href] {
				continue
			}
			seen[href] = true
			out = append(out, href)
		}
	}
	out = append(out, "/favicon.ico")
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func looksLikeImage(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	switch {
	case bytes.HasPrefix(data, []byte{0x00, 0x00, 0x01, 0x00}),
		bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}),
		bytes.HasPrefix(data, []byte{'G', 'I', 'F', '8'}),
		bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}),
		bytes.HasPrefix(data, []byte("BM")),
		bytes.HasPrefix(data, []byte("<svg")),
		bytes.HasPrefix(data, []byte("<?xml")):
		return true
	}
	return false
}

var titleRE = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
var generatorRE = regexp.MustCompile(`(?is)<meta[^>]+name=["']generator["'][^>]+content=["']([^"']+)["']`)

func extractTitle(body string) string {
	if body == "" {
		return ""
	}
	if m := titleRE.FindStringSubmatch(body); len(m) > 1 {
		t := strings.Join(strings.Fields(html.UnescapeString(m[1])), " ")
		return util.Truncate(t, 300)
	}
	return ""
}

func decodeBody(body []byte, contentType string) string {
	if len(body) == 0 {
		return ""
	}
	charset := ""
	lower := strings.ToLower(contentType)
	if i := strings.Index(lower, "charset="); i >= 0 {
		charset = strings.TrimSpace(strings.Trim(lower[i+8:], `"' ;`))
	}
	if charset == "" {
		head := string(body)
		if len(head) > 2048 {
			head = head[:2048]
		}
		if m := regexp.MustCompile(`(?i)<meta[^>]+charset=["']?([\w-]+)`).FindStringSubmatch(head); len(m) > 1 {
			charset = strings.ToLower(m[1])
		}
	}
	switch charset {
	case "gbk", "gb2312", "gb18030", "big5":
		if decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(body); err == nil {
			return string(decoded)
		}
	}
	return strings.ToValidUTF8(string(body), "")
}

func (p *Prober) matchHTTP(headers map[string]string, body, title, product string) []string {
	var apps []string
	if product != "" {
		apps = appendUnique(apps, product)
	}
	if m := generatorRE.FindStringSubmatch(body); len(m) > 1 {
		gen := strings.TrimSpace(html.UnescapeString(m[1]))
		if gen != "" {
			apps = appendUnique(apps, util.Truncate(gen, 60))
		}
	}
	for _, rule := range httpRules {
		var hay string
		switch rule.Target {
		case "header":
			if rule.Key != "" {
				hay = headers[rule.Key]
			} else {
				hay = flattenHeaders(headers)
			}
		case "cookie":
			hay = headers["set-cookie"]
		case "title":
			hay = title
		default:
			hay = body
		}
		if hay != "" && rule.Pattern.MatchString(hay) {
			apps = appendUnique(apps, rule.App)
		}
	}
	return apps
}

func buildTLSInfo(state tls.ConnectionState) *model.TLSInfo {
	info := &model.TLSInfo{
		Version: tlsVersionName(state.Version),
		Cipher:  tls.CipherSuiteName(state.CipherSuite),
	}
	if len(state.PeerCertificates) == 0 {
		return info
	}
	cert := state.PeerCertificates[0]
	info.Subject = cert.Subject.String()
	info.Issuer = cert.Issuer.String()
	info.CommonName = cert.Subject.CommonName
	info.SANs = cert.DNSNames
	if cert.SerialNumber != nil {
		info.Serial = cert.SerialNumber.Text(16)
	}
	info.NotBefore = cert.NotBefore
	info.NotAfter = cert.NotAfter
	return info
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	}
	return ""
}

func hostnamesFromTLS(info *model.TLSInfo) []string {
	if info == nil {
		return nil
	}
	var out []string
	if info.CommonName != "" && net.ParseIP(info.CommonName) == nil {
		out = append(out, info.CommonName)
	}
	for _, san := range info.SANs {
		if net.ParseIP(san) == nil {
			out = append(out, san)
		}
	}
	return out
}

func flattenHeaders(headers map[string]string) string {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(headers[k])
		sb.WriteByte('\n')
	}
	return util.Truncate(sb.String(), 4000)
}

func appendUnique(list []string, items ...string) []string {
	seen := make(map[string]bool, len(list))
	for _, v := range list {
		seen[strings.ToLower(v)] = true
	}
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" || seen[strings.ToLower(it)] {
			continue
		}
		seen[strings.ToLower(it)] = true
		list = append(list, it)
	}
	return list
}

func deviceFromApps(apps []string) string {
	for _, a := range apps {
		switch a {
		case "Hikvision", "Dahua":
			return "camera"
		case "RouterOS":
			return "router"
		case "Synology DSM":
			return "nas"
		case "宝塔面板":
			return "server-panel"
		case "Ollama", "ComfyUI", "Stable Diffusion WebUI", "vLLM", "LM Studio",
			"OpenAI 兼容 API", "Text Generation WebUI", "Qdrant":
			return "ai-server"
		}
	}
	return ""
}

func isTLSPort(port int) bool {
	switch port {
	case 443, 8443, 9443, 4443, 10443, 6443, 2376, 465, 993, 995, 5061, 8899:
		return true
	}
	return false
}

func isDefaultPort(port int, tls bool) bool {
	if tls {
		return port == 443
	}
	return port == 80
}

func isMinecraftPort(port int) bool {
	switch port {
	case 25565, 25566, 25567, 25568:
		return true
	}
	return false
}

func isPassivePort(port int) bool {
	switch port {
	case 21, 22, 23, 25, 110, 143, 465, 587, 873, 993, 995, 3306, 5432, 5671, 5672, 5900, 6379, 6380, 11211, 27017, 3389:
		return true
	}
	return false
}

var portServices = map[int]string{
	21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "domain", 80: "http", 110: "pop3",
	111: "rpcbind", 135: "msrpc", 139: "netbios-ssn", 143: "imap", 161: "snmp", 389: "ldap",
	443: "https", 445: "microsoft-ds", 465: "smtps", 512: "exec", 513: "login", 514: "shell",
	587: "submission", 636: "ldaps", 873: "rsync", 993: "imaps", 995: "pop3s", 1080: "socks",
	1433: "ms-sql-s", 1521: "oracle", 1723: "pptp", 2049: "nfs", 2375: "docker", 2376: "docker",
	3306: "mysql", 3389: "ms-wbt-server", 5432: "postgresql", 5672: "amqp", 5900: "vnc",
	5984: "couchdb", 6379: "redis", 6443: "kubernetes", 8080: "http-proxy", 8081: "http",
	8443: "https-alt", 8888: "http", 9000: "http", 9090: "http", 9200: "elasticsearch",
	9300: "elasticsearch", 11211: "memcached", 27017: "mongodb", 5000: "http", 8000: "http",
	10000: "http", 15672: "rabbitmq", 5601: "kibana", 3000: "http",
	5985: "wsman", 5986: "wsmans", 64738: "mumble",
	8086: "influxdb", 8123: "clickhouse", 8500: "consul", 11434: "ollama",
	8188: "comfyui", 7860: "http", 2379: "etcd", 6333: "qdrant",
}

func guessService(port int) string {
	if s, ok := portServices[port]; ok {
		return s
	}
	return "unknown"
}

// hintIn 判断调用方给出的服务名提示是否命中候选值。
func hintIn(hint string, names ...string) bool {
	value := strings.ToLower(strings.TrimSpace(hint))
	if value == "" {
		return false
	}
	for _, name := range names {
		if value == name {
			return true
		}
	}
	return false
}

// rabbitMQVersion 从 AMQP Connection.Start 字段表中提取 RabbitMQ 版本号。
func rabbitMQVersion(data []byte) string {
	index := bytes.Index(data, []byte("RabbitMQ"))
	if index < 0 {
		return ""
	}
	rest := data[index+len("RabbitMQ"):]
	for offset := 0; offset+4 < len(rest) && offset < 256; offset++ {
		if rest[offset] != 'S' {
			continue
		}
		size := int(binary.BigEndian.Uint32(rest[offset+1:]))
		start := offset + 5
		if size <= 0 || size > 32 || start+size > len(rest) {
			continue
		}
		value := string(rest[start : start+size])
		if value[0] >= '0' && value[0] <= '9' && strings.Count(value, ".") >= 1 {
			return value
		}
	}
	return ""
}
