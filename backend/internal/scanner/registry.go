package scanner

import (
	"os/exec"
	"strings"
	"time"

	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/model"
)

type ToolStatus struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Purpose   string `json:"purpose"`
}

type Registry struct {
	Config  config.ScannerConfig
	Native  *Native
	Nmap    *Nmap
	Masscan *Masscan
}

func NewRegistry(cfg config.ScannerConfig) *Registry {
	return &Registry{
		Config:  cfg,
		Native:  NewNative(cfg.MaxConcurrency, 1500*time.Millisecond),
		Nmap:    &Nmap{Path: cfg.NmapPath},
		Masscan: &Masscan{Path: cfg.MasscanPath},
	}
}

func (r *Registry) Statuses() []ToolStatus {
	var out []ToolStatus
	if path, err := exec.LookPath("nmap"); err == nil {
		version := ""
		if out2, err := exec.Command(path, "--version").Output(); err == nil {
			version = strings.SplitN(strings.TrimSpace(string(out2)), "\n", 2)[0]
		}
		out = append(out, ToolStatus{Name: "nmap", Path: path, Available: true, Version: version, Purpose: "服务/版本识别"})
	} else {
		out = append(out, ToolStatus{Name: "nmap", Path: r.Nmap.Path, Available: false, Reason: "未安装或不在 PATH 中", Purpose: "服务/版本识别"})
	}

	if path, err := exec.LookPath("masscan"); err == nil {
		if msg := masscanPrivilege(); msg != "" {
			out = append(out, ToolStatus{Name: "masscan", Path: path, Available: false, Reason: msg, Purpose: "大范围端口测绘"})
		} else {
			out = append(out, ToolStatus{Name: "masscan", Path: path, Available: true, Version: "1.3.x", Purpose: "大范围端口测绘"})
		}
	} else {
		out = append(out, ToolStatus{Name: "masscan", Path: r.Masscan.Path, Available: false, Reason: "未安装或不在 PATH 中", Purpose: "大范围端口测绘"})
	}

	out = append(out, ToolStatus{
		Name:      "native",
		Path:      "builtin",
		Available: true,
		Version:   "go-tcp",
		Purpose:   "内置 TCP 扫描（无需 root）",
	})
	return out
}

func (r *Registry) NmapAvailable() bool {
	_, err := r.Nmap.Available()
	return err == nil
}

// ResolveTool 根据请求与可用性选择端口发现引擎。
func (r *Registry) ResolveTool(requested string, targetCount uint64) (string, string) {
	if requested == "" || requested == "auto" {
		if r.Masscan != nil {
			if _, err := r.Masscan.Available(); err == nil && targetCount >= 256 {
				return "masscan", ""
			}
		}
		return "native", ""
	}
	if requested == "masscan" {
		if _, err := r.Masscan.Available(); err != nil {
			return "native", "masscan 不可用（" + err.Error() + "），已回退到内置扫描器"
		}
		return "masscan", ""
	}
	return "native", ""
}

func (r *Registry) ScanTimeout() time.Duration { return r.Config.DiscoverTimeout }

func (r *Registry) ProbeTimeout() time.Duration { return r.Config.ProbeTimeout }

// ApplySettings 把控制台保存的参数应用到各扫描引擎。
func (r *Registry) ApplySettings(st model.ScanSettings) {
	r.Masscan.SetOptions(MasscanOptions{
		Interface: st.MasscanInterface,
		RouterMAC: st.MasscanRouterMAC,
		ExtraArgs: st.MasscanExtraArgs,
	})
	r.Native.SetLocalIP(InterfaceIPv4(st.ScanInterface))
	r.Nmap.SetInterface(st.ScanInterface)
}
