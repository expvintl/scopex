package fingerprint

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/expvintl/scopex/internal/model"
)

const winrmIdentifyBody = `<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:wsmid="http://schemas.dmtf.org/wbem/wsman/identity/1/wsmanidentity.xsd"><s:Header/><s:Body><wsmid:Identify/></s:Body></s:Envelope>`

var (
	winrmVersionPattern = regexp.MustCompile(`(?:OS|ProductVersion)[:=]\s*([0-9]+\.[0-9]+\.[0-9]+)`)
	winrmProductBody    = regexp.MustCompile(`<wsmid:ProductVersion>([^<]{0,200})</wsmid:ProductVersion>`)
)

// probeWinRM 用 WS-Management Identify 请求识别 WinRM 服务与其系统版本。
func (p *Prober) probeWinRM(ctx context.Context, ip string, port int) *Result {
	useTLS := port == 5986
	conn, err := p.dialProbe(ctx, ip, port)
	if err != nil {
		return nil
	}
	defer conn.Close()

	if useTLS {
		tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, ServerName: ip})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil
		}
		conn = tlsConn
	}

	host := ip
	if port != 5985 && port != 5986 {
		host = net.JoinHostPort(ip, strconv.Itoa(port))
	}
	request := fmt.Sprintf(
		"POST /wsman HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nContent-Type: application/soap+xml;charset=UTF-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		host, p.UserAgent, len(winrmIdentifyBody), winrmIdentifyBody,
	)
	if _, err := conn.Write([]byte(request)); err != nil {
		return nil
	}

	resp, err := http.ReadResponse(bufio.NewReaderSize(conn, 32*1024), nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	server := resp.Header.Get("Server")
	auth := resp.Header.Get("Www-Authenticate")
	text := string(body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return nil
	}
	if !strings.Contains(strings.ToLower(server), "microsoft-httpapi") &&
		!strings.Contains(strings.ToLower(text), "wsmid") &&
		!strings.Contains(strings.ToLower(auth), "negotiate") {
		return nil
	}

	res := &Result{Headers: map[string]string{}, StatusCode: resp.StatusCode}
	if server != "" {
		res.Headers["server"] = server
	}
	if auth != "" {
		res.Headers["www-authenticate"] = auth
	}

	version := ""
	if group := winrmVersionPattern.FindStringSubmatch(text); len(group) > 1 {
		version = group[1]
	}
	res.Service = model.Service{
		Name:       "winrm",
		Product:    "Microsoft WinRM",
		Version:    version,
		Method:     "winrm",
		Confidence: 92,
	}
	if version != "" {
		res.OS = osFromWindowsVersion(version)
	}
	if group := winrmProductBody.FindStringSubmatch(text); len(group) > 1 {
		res.Service.ExtraInfo = strings.TrimSpace(group[1])
	}
	if server != "" {
		res.HeaderText = "Server: " + server
	}
	res.Banner = strings.TrimSpace("WinRM " + version + " " + res.Service.ExtraInfo)
	res.Apps = []string{"winrm"}
	res.CPE = []string{"cpe:/o:microsoft:windows"}
	return res
}
