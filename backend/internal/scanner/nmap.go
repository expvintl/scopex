package scanner

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
)

// Nmap 负责服务识别（-sV），并在需要时补充主机名与操作系统线索。
type Nmap struct {
	Path string

	mu    sync.RWMutex
	iface string
}

// SetInterface 指定发起扫描的网卡，空字符串表示由系统路由决定。
func (n *Nmap) SetInterface(name string) {
	n.mu.Lock()
	n.iface = name
	n.mu.Unlock()
}

func (n *Nmap) Interface() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.iface
}

type NmapResult struct {
	IP        string
	Port      int
	Protocol  string
	Service   model.Service
	CPE       []string
	Hostnames []string
	Scripts   map[string]string
}

func (n *Nmap) Available() (string, error) {
	path, err := exec.LookPath(n.Path)
	if err != nil {
		return "", fmt.Errorf("未找到 nmap 可执行文件")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := exec.CommandContext(ctx, path, "--version").Output(); err != nil {
		return "", fmt.Errorf("nmap 无法执行: %v", err)
	}
	return path, nil
}

type nmapRun struct {
	Hosts []nmapHost `xml:"host"`
}

type nmapHost struct {
	Status    nmapStatus     `xml:"status"`
	Addresses []nmapAddress  `xml:"address"`
	Hostnames []nmapHostname `xml:"hostnames>hostname"`
	Ports     []nmapPort     `xml:"ports>port"`
}

type nmapStatus struct {
	State string `xml:"state,attr"`
}

type nmapAddress struct {
	Addr string `xml:"addr,attr"`
	Type string `xml:"addrtype,attr"`
}

type nmapHostname struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type nmapPort struct {
	Protocol string `xml:"protocol,attr"`
	PortID   int    `xml:"portid,attr"`
	State    struct {
		State string `xml:"state,attr"`
	} `xml:"state"`
	Service struct {
		Name       string   `xml:"name,attr"`
		Product    string   `xml:"product,attr"`
		Version    string   `xml:"version,attr"`
		ExtraInfo  string   `xml:"extrainfo,attr"`
		OSType     string   `xml:"ostype,attr"`
		DeviceType string   `xml:"devicetype,attr"`
		Method     string   `xml:"method,attr"`
		Conf       int      `xml:"conf,attr"`
		CPEs       []string `xml:"cpe"`
	} `xml:"service"`
	Scripts []struct {
		ID     string `xml:"id,attr"`
		Output string `xml:"output,attr"`
	} `xml:"script"`
}

// ScanServices 对给定的目标与端口执行一次 nmap -sV，结果通过回调返回。
func (n *Nmap) ScanServices(ctx context.Context, targets []string, ports []int, timeout time.Duration,
	onResult func(NmapResult)) error {
	if len(targets) == 0 || len(ports) == 0 {
		return nil
	}
	path, err := exec.LookPath(n.Path)
	if err != nil {
		return fmt.Errorf("未找到 nmap")
	}
	args := []string{"-Pn", "-sV", "-T4", "--version-intensity", "5", "--max-retries", "2", "--host-timeout", "5m"}
	if iface := n.Interface(); iface != "" {
		args = append(args, "-e", iface)
	}
	if os.Geteuid() == 0 {
		args = append(args, "-sS")
	} else {
		args = append(args, "-sT")
	}
	args = append(args, "-p", PortsToSpec(ports), "-oX", "-")
	args = append(args, targets...)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("nmap 执行失败: %s", util.Truncate(msg, 300))
	}
	return parseNmapXML(out, onResult)
}

func parseNmapXML(raw []byte, onResult func(NmapResult)) error {
	decoder := xml.NewDecoder(bytes.NewReader(sanitizeXML(raw)))
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var run nmapRun
	if err := decoder.Decode(&run); err != nil {
		return fmt.Errorf("解析 nmap 输出失败: %w", err)
	}
	for _, host := range run.Hosts {
		if host.Status.State != "up" {
			continue
		}
		ip := ""
		for _, a := range host.Addresses {
			if a.Type == "ipv4" || a.Type == "ipv6" {
				ip = a.Addr
				break
			}
		}
		if ip == "" {
			continue
		}
		var hostnames []string
		for _, hn := range host.Hostnames {
			if hn.Name != "" {
				hostnames = append(hostnames, hn.Name)
			}
		}
		for _, p := range host.Ports {
			if p.State.State != "open" {
				continue
			}
			svc := model.Service{
				Name:       p.Service.Name,
				Product:    p.Service.Product,
				Version:    p.Service.Version,
				ExtraInfo:  p.Service.ExtraInfo,
				OSType:     p.Service.OSType,
				DeviceType: p.Service.DeviceType,
				Method:     "nmap",
				Confidence: p.Service.Conf,
			}
			res := NmapResult{
				IP:        ip,
				Port:      p.PortID,
				Protocol:  p.Protocol,
				Service:   svc,
				CPE:       p.Service.CPEs,
				Hostnames: hostnames,
			}
			if len(p.Scripts) > 0 {
				res.Scripts = map[string]string{}
				for _, s := range p.Scripts {
					res.Scripts[s.ID] = util.Truncate(s.Output, 2000)
				}
			}
			onResult(res)
		}
	}
	return nil
}

// sanitizeXML 替换非法 UTF-8 字节，避免 nmap 输出中的二进制 banner 让 XML 解析失败。
func sanitizeXML(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); {
		r, size := decodeRune(raw[i:])
		if r == '\uFFFD' && size == 1 {
			i++
			continue
		}
		out = append(out, raw[i:i+size]...)
		i += size
	}
	return out
}

func decodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 0
	}
	c := b[0]
	switch {
	case c < 0x80:
		return rune(c), 1
	case c&0xE0 == 0xC0 && len(b) >= 2 && b[1]&0xC0 == 0x80:
		return rune(c&0x1F)<<6 | rune(b[1]&0x3F), 2
	case c&0xF0 == 0xE0 && len(b) >= 3 && b[1]&0xC0 == 0x80 && b[2]&0xC0 == 0x80:
		return rune(c&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F), 3
	case c&0xF8 == 0xF0 && len(b) >= 4 && b[1]&0xC0 == 0x80 && b[2]&0xC0 == 0x80 && b[3]&0xC0 == 0x80:
		return rune(c&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F), 4
	}
	return 0xFFFD, 1
}
