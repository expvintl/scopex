package model

import (
	"strings"
	"testing"
)

func TestOSFamily(t *testing.T) {
	cases := map[string]string{
		"Ubuntu 22.04.3 LTS":       "linux",
		"Debian GNU/Linux 12":      "linux",
		"OpenWrt 23.05":            "linux",
		"Windows Server 2019":      "windows",
		"FreeBSD 13.2-RELEASE":     "unix",
		"Darwin Kernel Version 23": "macos",
		"Android 13":               "android",
		"something-weird":          "other",
		"":                         "",
	}
	for in, want := range cases {
		if got := OSFamily(in); got != want {
			t.Errorf("OSFamily(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestAppCategory(t *testing.T) {
	cases := map[string]string{
		"Ollama":        "ai",
		"ComfyUI":       "ai",
		"vLLM":          "ai",
		"nginx":         "web",
		"WordPress":     "app",
		"Grafana":       "ops",
		"MySQL":         "db",
		"RabbitMQ":      "queue",
		"MinIO":         "storage",
		"Jenkins":       "ops",
		"Unknown-Thing": "other",
	}
	for in, want := range cases {
		if got := AppCategory(in); got != want {
			t.Errorf("AppCategory(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestServiceCategory(t *testing.T) {
	cases := map[string]string{
		"http":    "web",
		"https":   "web",
		"ssh":     "remote",
		"mysql":   "db",
		"smb":     "storage",
		"smtp":    "mail",
		"ldap":    "directory",
		"amqp":    "queue",
		"docker":  "ops",
		"unknown": "other",
	}
	for in, want := range cases {
		if got := ServiceCategory(in); got != want {
			t.Errorf("ServiceCategory(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestContinent(t *testing.T) {
	cases := map[string]string{
		"CN": "asia",
		"DE": "europe",
		"US": "north_america",
		"BR": "south_america",
		"ZA": "africa",
		"AU": "oceania",
		"":   "",
		"XX": "",
	}
	for in, want := range cases {
		if got := Continent(in); got != want {
			t.Errorf("Continent(%q) = %q，期望 %q", in, got, want)
		}
	}
	if got := Continent("cn"); got != "asia" {
		t.Errorf("国家码应大小写不敏感，实际 %q", got)
	}
}

// 国家码表必须自洽：两位大写字母，且不能重复出现在两个大洲里。
func TestContinentTableWellFormed(t *testing.T) {
	for code := range continentByCode {
		if len(code) != 2 || strings.ToUpper(code) != code {
			t.Errorf("非法国家码: %q", code)
		}
	}
	if len(continentByCode) < 200 {
		t.Errorf("国家码覆盖过少: %d", len(continentByCode))
	}
}

func TestOSFamilyOfFallback(t *testing.T) {
	// os 字段优先，能被归族就不再走兜底。
	if got := OSFamilyOf(&Host{OS: "Ubuntu 22.04.3 LTS"}); got != "linux" {
		t.Fatalf("os 字段应优先: %q", got)
	}
	// 无 os 时用 cpe:/o: 条目。
	if got := OSFamilyOf(&Host{CPE: []string{"cpe:/a:nginx:nginx:1.25.3", "cpe:/o:linux:linux_kernel:5.15"}}); got != "linux" {
		t.Fatalf("cpe 兜底失败: %q", got)
	}
	// 无 os / 无 os 类 cpe 时用 Server 头。
	h := &Host{Headers: map[string]string{"Server": "Werkzeug/3.0.1 Python/3.11.2 Ubuntu-22.04"}}
	if got := OSFamilyOf(h); got != "linux" {
		t.Fatalf("Server 头兜底失败: %q", got)
	}
	// 线索不足时保持为空，不臆造。
	if got := OSFamilyOf(&Host{Banner: "hello world"}); got != "" {
		t.Fatalf("无证据时不应归类: %q", got)
	}
	if got := OSFamilyOf(nil); got != "" {
		t.Fatalf("nil 应返回空: %q", got)
	}
}
