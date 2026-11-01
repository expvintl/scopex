package scanner

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// InterfaceInfo 描述一块可用于发起扫描的网卡。
type InterfaceInfo struct {
	Name       string   `json:"name"`
	MAC        string   `json:"mac,omitempty"`
	IPv4       string   `json:"ipv4,omitempty"`
	Addresses  []string `json:"addresses"`
	Up         bool     `json:"up"`
	Loopback   bool     `json:"loopback"`
	Default    bool     `json:"default"`
	Gateway    string   `json:"gateway,omitempty"`
	GatewayMAC string   `json:"gatewayMac,omitempty"`
}

// RouteInfo 是系统默认路由的摘要，供前端「自动探测」使用。
type RouteInfo struct {
	Interface  string `json:"interface"`
	Gateway    string `json:"gateway,omitempty"`
	GatewayMAC string `json:"gatewayMac,omitempty"`
}

// ListInterfaces 枚举本机网卡，并标注默认路由与网关 MAC（读取 ARP 邻居表）。
func ListInterfaces() []InterfaceInfo {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	routes := defaultRoutes()
	primary, primaryGW := detectRoute()
	if primaryGW != "" {
		ensureNeighbor(primaryGW)
		time.Sleep(30 * time.Millisecond)
	}
	arp := readARP()

	out := make([]InterfaceInfo, 0, len(ifaces))
	for _, it := range ifaces {
		info := InterfaceInfo{
			Name:      it.Name,
			MAC:       it.HardwareAddr.String(),
			Up:        it.Flags&net.FlagUp != 0,
			Loopback:  it.Flags&net.FlagLoopback != 0,
			Addresses: []string{},
		}
		if addrs, err := it.Addrs(); err == nil {
			for _, a := range addrs {
				info.Addresses = append(info.Addresses, a.String())
				if info.IPv4 == "" {
					if ipnet, ok := a.(*net.IPNet); ok {
						if ip4 := ipnet.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
							info.IPv4 = ip4.String()
						}
					}
				}
			}
		}
		if it.Name == primary {
			info.Default = true
			info.Gateway = routes[it.Name]
			if info.Gateway == "" {
				info.Gateway = primaryGW
			}
			info.GatewayMAC = arp[info.Gateway]
		}
		out = append(out, info)
	}
	return out
}

// DefaultRoute 返回默认出口网卡与网关信息，链路未就绪时返回零值。
func DefaultRoute() RouteInfo {
	name, gw := detectRoute()
	if name == "" {
		return RouteInfo{}
	}
	if gw != "" {
		ensureNeighbor(gw)
		time.Sleep(30 * time.Millisecond)
	}
	return RouteInfo{Interface: name, Gateway: gw, GatewayMAC: readARP()[gw]}
}

// InterfaceIPv4 返回指定网卡的首选 IPv4，名称不存在或无地址时返回 nil。
func InterfaceIPv4(name string) net.IP {
	if name == "" {
		return nil
	}
	it, err := net.InterfaceByName(name)
	if err != nil {
		return nil
	}
	addrs, err := it.Addrs()
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ip4 := ipnet.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
				return ip4
			}
		}
	}
	return nil
}

// InterfaceExists 判断网卡名是否存在于当前主机。
func InterfaceExists(name string) bool {
	if name == "" {
		return false
	}
	_, err := net.InterfaceByName(name)
	return err == nil
}

// detectRoute 依次尝试默认路由表、出口地址回推、首个可用网卡。
func detectRoute() (name, gateway string) {
	for iface, gw := range defaultRoutes() {
		return iface, gw
	}
	if ip := outwardIP(); ip != nil {
		if iface := interfaceOfIP(ip); iface != "" {
			return iface, guessGateway(iface)
		}
	}
	for _, it := range interfaces() {
		if it.Loopback || !it.Up || it.IPv4 == "" {
			continue
		}
		return it.Name, guessGateway(it.Name)
	}
	return "", ""
}

func interfaces() []InterfaceInfo {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]InterfaceInfo, 0, len(ifaces))
	for _, it := range ifaces {
		info := InterfaceInfo{
			Name:     it.Name,
			MAC:      it.HardwareAddr.String(),
			Up:       it.Flags&net.FlagUp != 0,
			Loopback: it.Flags&net.FlagLoopback != 0,
		}
		if addrs, err := it.Addrs(); err == nil {
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && info.IPv4 == "" {
					if ip4 := ipnet.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
						info.IPv4 = ip4.String()
					}
				}
			}
		}
		out = append(out, info)
	}
	return out
}

// outwardIP 通过一次不产生流量的 UDP 连接拿到出口源地址。
func outwardIP() net.IP {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 300*time.Millisecond)
	if err != nil {
		return nil
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.To4()
	}
	return nil
}

func interfaceOfIP(ip net.IP) string {
	for _, it := range interfaces() {
		if it.IPv4 == ip.String() {
			return it.Name
		}
	}
	return ""
}

// guessGateway 在缺少默认路由时按网卡子网推断网关（通常为 .1）。
func guessGateway(name string) string {
	it, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}
	addrs, err := it.Addrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipnet.IP.To4()
		if ip4 == nil || ip4.IsLinkLocalUnicast() {
			continue
		}
		ones, bits := ipnet.Mask.Size()
		if bits != 32 || ones < 24 {
			return ""
		}
		gw := make(net.IP, net.IPv4len)
		copy(gw, ip4.Mask(ipnet.Mask))
		gw[3]++
		return gw.String()
	}
	return ""
}

// defaultRoutes 解析 /proc/net/route，返回 网卡名 → 网关 IPv4。
func defaultRoutes() map[string]string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return out
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		if gw, err := hexToIPv4(fields[2]); err == nil && gw != "0.0.0.0" {
			out[fields[0]] = gw
		}
	}
	return out
}

// readARP 解析 /proc/net/arp，返回 IPv4 → MAC。
func readARP() map[string]string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return out
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 || len(fields[3]) != 17 || fields[3] == "00:00:00:00:00:00" {
			continue
		}
		out[fields[0]] = strings.ToLower(fields[3])
	}
	return out
}

// ensureNeighbor 向网关发一个空的 UDP 包，促使内核完成 ARP 解析。
func ensureNeighbor(ip string) {
	if ip == "" {
		return
	}
	conn, err := net.DialTimeout("udp", net.JoinHostPort(ip, "9"), 300*time.Millisecond)
	if err != nil {
		return
	}
	_, _ = conn.Write([]byte{0})
	_ = conn.Close()
}

func hexToIPv4(h string) (string, error) {
	if len(h) != 8 {
		return "", fmt.Errorf("无效的网关地址 %q", h)
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return "", err
	}
	return net.IPv4(b[3], b[2], b[1], b[0]).String(), nil
}
