package fingerprint

import "testing"

func TestOSFromSSHSoftware(t *testing.T) {
	cases := map[string]string{
		"OpenSSH_8.9p1 Ubuntu-3ubuntu0.13":         "Ubuntu",
		"OpenSSH_9.2p1 Debian-2+deb12u2":           "Debian",
		"OpenSSH_7.4":                              "",
		"dropbear_2025.89":                         "嵌入式 Linux",
		"OpenSSH_8.0 FreeBSD-openssh-portable-8.0": "FreeBSD",
		"OpenSSH_8.7 Raspbian-1":                   "Raspbian",
	}
	for in, want := range cases {
		if got := osFromSSHSoftware(in); got != want {
			t.Errorf("osFromSSHSoftware(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOSFromServerHeader(t *testing.T) {
	cases := map[string]string{
		"Microsoft-IIS/10.0":     "Windows Server 2016+",
		"Microsoft-IIS/8.5":      "Windows Server 2012 R2",
		"nginx/1.24.0 (Ubuntu)":  "Ubuntu",
		"Apache/2.4.62 (Debian)": "Debian",
		"Caddy":                  "",
	}
	for in, want := range cases {
		if got := osFromServerHeader(in); got != want {
			t.Errorf("osFromServerHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOSFromWindowsVersion(t *testing.T) {
	cases := map[string]string{
		"10.0.19045": "Windows 10 2004+ / Server 2019+",
		"10.0.20348": "Windows Server 2022",
		"6.3.9600":   "Windows 8.1 / Server 2012 R2",
		"6.1.7601":   "Windows 7 / Server 2008 R2",
		"5.1.2600":   "Windows XP",
		"":           "",
	}
	for in, want := range cases {
		if got := osFromWindowsVersion(in); got != want {
			t.Errorf("osFromWindowsVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
