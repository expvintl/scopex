package fingerprint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sshDistroPatterns = []struct {
	needle string
	label  string
}{
	{"ubuntu", "Ubuntu"},
	{"debian", "Debian"},
	{"raspbian", "Raspbian"},
	{"alpine", "Alpine Linux"},
	{"centos", "CentOS"},
	{"red hat", "Red Hat Enterprise Linux"},
	{"suse", "SUSE Linux"},
	{"amazon", "Amazon Linux"},
	{"freebsd", "FreeBSD"},
	{"openbsd", "OpenBSD"},
	{"netbsd", "NetBSD"},
	{"openwrt", "OpenWrt"},
	{"dd-wrt", "DD-WRT"},
	{"synology", "Synology DSM"},
	{"pfsense", "pfSense"},
	{"mikrotik", "RouterOS"},
}

var windowsBuildPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

// osFromSSHSoftware 从 SSH 软件串（如 OpenSSH_8.9p1 Ubuntu-3ubuntu0.13）解析发行版。
func osFromSSHSoftware(software string) string {
	lower := strings.ToLower(software)
	for _, item := range sshDistroPatterns {
		if strings.Contains(lower, item.needle) {
			return item.label
		}
	}
	switch {
	case strings.Contains(lower, "el7"):
		return "CentOS 7 / RHEL 7"
	case strings.Contains(lower, "el8"):
		return "CentOS 8 / RHEL 8"
	case strings.Contains(lower, "el9"):
		return "CentOS 9 / RHEL 9"
	}
	if strings.Contains(lower, "dropbear") {
		return "嵌入式 Linux"
	}
	return ""
}

// osFromServerHeader 从 HTTP Server 响应头推断操作系统。
func osFromServerHeader(server string) string {
	lower := strings.ToLower(server)
	switch {
	case strings.Contains(lower, "microsoft-iis/10"):
		return "Windows Server 2016+"
	case strings.Contains(lower, "microsoft-iis/8.5"):
		return "Windows Server 2012 R2"
	case strings.Contains(lower, "microsoft-iis/8.0"):
		return "Windows Server 2012"
	case strings.Contains(lower, "microsoft-iis/7.5"):
		return "Windows Server 2008 R2"
	case strings.Contains(lower, "microsoft-iis"):
		return "Windows Server"
	case strings.Contains(lower, "win64"), strings.Contains(lower, "win32"):
		return "Windows"
	case strings.Contains(lower, "ubuntu"):
		return "Ubuntu"
	case strings.Contains(lower, "debian"):
		return "Debian"
	case strings.Contains(lower, "centos"):
		return "CentOS"
	case strings.Contains(lower, "freebsd"):
		return "FreeBSD"
	}
	return ""
}

// osFromWindowsVersion 把 Windows 版本号（10.0.19045）映射为发行版名称。
func osFromWindowsVersion(version string) string {
	match := windowsBuildPattern.FindStringSubmatch(strings.TrimSpace(version))
	if len(match) < 4 {
		return ""
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	build, _ := strconv.Atoi(match[3])
	return windowsBuildName(major, minor, build)
}

func windowsBuildName(major, minor, build int) string {
	switch {
	case major == 10 && build >= 22000:
		return "Windows 11 / Server 2022+"
	case major == 10 && build >= 20348:
		return "Windows Server 2022"
	case major == 10 && build >= 19041:
		return "Windows 10 2004+ / Server 2019+"
	case major == 10 && build >= 17763:
		return "Windows 10 1809 / Server 2019"
	case major == 10 && build >= 14393:
		return "Windows 10 1607 / Server 2016"
	case major == 10:
		return "Windows 10"
	case major == 6 && minor == 3:
		return "Windows 8.1 / Server 2012 R2"
	case major == 6 && minor == 2:
		return "Windows 8 / Server 2012"
	case major == 6 && minor == 1:
		return "Windows 7 / Server 2008 R2"
	case major == 6 && minor == 0:
		return "Windows Vista / Server 2008"
	case major == 5 && minor == 2:
		return "Windows Server 2003"
	case major == 5 && minor == 1:
		return "Windows XP"
	}
	if major > 0 {
		return fmt.Sprintf("Windows %d.%d.%d", major, minor, build)
	}
	return ""
}
