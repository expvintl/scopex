package fingerprint

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
)

// 本文件实现「主动 API 指纹」：很多服务在首页给不出任何线索，但未鉴权的
// 版本/状态接口会直接吐出产品名、版本、运行环境甚至模型清单。
// 这类接口比猜 banner 精确得多，也是资产清点里最有价值的证据。
//
// 代价是额外的 HTTP 请求，所以每个探测都带严格的前置条件（端口或首页特征），
// 并且单端口请求总量有上限（aiMaxRequests），避免对目标造成放大流量。

const aiMaxRequests = 4

// aiInfo 是一次主动指纹探测的结论。
type aiInfo struct {
	Product string
	Version string
	Kind    string // llm / image / platform / infra / db / ops，用于统计归类
	Models  []string
	System  string // 运行环境、硬件等补充信息
	OS      string
	Note    string // 例如「接口需要认证」这类判断依据
}

// summary 把结论压成一行，写入 service.extra_info 供列表页直接展示。
func (i *aiInfo) summary() string {
	parts := make([]string, 0, 3)
	if len(i.Models) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个模型", len(i.Models)))
	}
	if i.System != "" {
		parts = append(parts, i.System)
	}
	if i.Note != "" {
		parts = append(parts, i.Note)
	}
	return strings.Join(parts, "; ")
}

type apiResp struct {
	status int
	header map[string]string
	body   []byte
}

// aiTarget 承接一次探测的连接参数，并限制额外请求的数量。
type aiTarget struct {
	p      *Prober
	ctx    context.Context
	scheme string
	ip     string
	port   int
	used   int
}

func (t *aiTarget) client() *http.Client {
	return &http.Client{
		Timeout: t.p.Timeout,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// get 发起一次 GET；失败或超出请求预算时返回 nil。
func (t *aiTarget) get(path string) *apiResp {
	if t.used >= aiMaxRequests {
		return nil
	}
	t.used++
	u := fmt.Sprintf("%s://%s%s", t.scheme, net.JoinHostPort(t.ip, strconv.Itoa(t.port)), path)
	req, err := http.NewRequestWithContext(t.ctx, "GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", t.p.UserAgent)
	req.Header.Set("Accept", "application/json,*/*")
	resp, err := t.client().Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	out := &apiResp{status: resp.StatusCode, header: map[string]string{}, body: body}
	for name, values := range resp.Header {
		out.header[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return out
}

func (r *apiResp) ok() bool { return r != nil && r.status >= 200 && r.status < 300 }

// JSON 解析辅助：接口返回结构深浅不一，用路径式取值能少写很多 struct。

func jsonObj(body []byte) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil
	}
	return m
}

func jsonWalk(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = obj[k]
		if !ok {
			return nil
		}
	}
	return cur
}

func jsonStr(m map[string]any, keys ...string) string {
	v := jsonWalk(m, keys...)
	switch typed := v.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}
	return ""
}

func jsonList(m map[string]any, keys ...string) []any {
	list, _ := jsonWalk(m, keys...).([]any)
	return list
}

// aiProbe 描述一个可识别的产品及其探测入口。
type aiProbe struct {
	product string
	ports   []int    // 端口命中即视为值得探测
	markers []string // 首页标题/正文/头部出现这些词时探测（小写）
	run     func(t *aiTarget) *aiInfo
}

// applies 判断当前端口是否值得为该产品多发一次请求。
func (ep *aiProbe) applies(port int, res *Result) bool {
	for _, p := range ep.ports {
		if p == port {
			return true
		}
	}
	if len(ep.markers) == 0 {
		return false
	}
	hay := strings.ToLower(res.Title + "\n" + res.Banner + "\n" + res.HeaderText)
	for _, marker := range ep.markers {
		if strings.Contains(hay, marker) {
			return true
		}
	}
	return false
}

var aiProbes = []*aiProbe{
	{
		// Ollama 默认无鉴权，/api/version 与 /api/tags 直接给出模型清单。
		product: "Ollama",
		ports:   []int{11434},
		markers: []string{"ollama"},
		run: func(t *aiTarget) *aiInfo {
			ver := t.get("/api/version")
			if !ver.ok() {
				return nil
			}
			m := jsonObj(ver.body)
			version := jsonStr(m, "version")
			if version == "" {
				return nil
			}
			info := &aiInfo{Product: "Ollama", Version: version, Kind: "llm"}
			if tags := t.get("/api/tags"); tags.ok() {
				for _, item := range jsonList(jsonObj(tags.body), "models") {
					model, _ := item.(map[string]any)
					name := jsonStr(model, "name")
					if name == "" {
						continue
					}
					if size := jsonStr(model, "details", "parameter_size"); size != "" {
						name += " (" + size + ")"
					}
					if len(info.Models) < 20 {
						info.Models = append(info.Models, name)
					}
				}
			}
			return info
		},
	},
	{
		// ComfyUI 的 /system_stats 会泄露操作系统、Python 版本与显卡型号/显存。
		product: "ComfyUI",
		ports:   []int{8188, 8189},
		markers: []string{"comfyui"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/system_stats")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			if m == nil || (jsonWalk(m, "system") == nil && jsonWalk(m, "devices") == nil) {
				return nil
			}
			info := &aiInfo{Product: "ComfyUI", Version: jsonStr(m, "system", "comfyui_version"), Kind: "image"}
			if v := jsonStr(m, "system", "os"); v == "nt" {
				info.OS = "Windows"
			} else if v == "posix" {
				info.OS = "Linux"
			}
			sys := []string{}
			if os := jsonStr(m, "system", "os"); os != "" {
				sys = append(sys, os)
			}
			if py := jsonStr(m, "system", "python_version"); py != "" {
				sys = append(sys, "Python "+py)
			}
			if devs := jsonList(m, "devices"); len(devs) > 0 {
				if dev, ok := devs[0].(map[string]any); ok {
					if name := jsonStr(dev, "name"); name != "" {
						sys = append(sys, name)
					}
					if vram, ok := jsonWalk(dev, "vram_total").(float64); ok && vram > 0 {
						sys = append(sys, fmt.Sprintf("显存 %.0fGB", vram/(1024*1024*1024)))
					}
				}
			}
			info.System = strings.Join(sys, ", ")
			return info
		},
	},
	{
		// Stable Diffusion WebUI / Forge 共用 /sdapi。
		product: "Stable Diffusion WebUI",
		ports:   []int{7860, 7861},
		markers: []string{"stable-diffusion-webui", "sdapi"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/sdapi/v1/options")
			if r == nil {
				return nil
			}
			if r.status == 401 || r.status == 403 {
				return &aiInfo{Product: "Stable Diffusion WebUI", Kind: "image", Note: "API 需要认证"}
			}
			m := jsonObj(r.body)
			info := &aiInfo{Product: "Stable Diffusion WebUI", Kind: "image"}
			if ckpt := jsonStr(m, "sd_model_checkpoint"); ckpt != "" {
				info.Models = append(info.Models, ckpt)
			}
			if info.Models == nil && m == nil {
				return nil
			}
			if vae := jsonStr(m, "sd_vae"); vae != "" && vae != "Automatic" {
				info.System = "VAE " + vae
			}
			return info
		},
	},
	{
		// text-generation-webui 的 /api/v1/model 直接回模型文件名。
		product: "Text Generation WebUI",
		markers: []string{"text generation webui", "text-generation-webui"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/api/v1/model")
			if !r.ok() {
				return nil
			}
			model := jsonStr(jsonObj(r.body), "result")
			if model == "" {
				return nil
			}
			return &aiInfo{Product: "Text Generation WebUI", Kind: "llm", Models: []string{model}}
		},
	},
	{
		// llama.cpp server 的 /props 带完整模型路径与上下文长度。
		product: "llama.cpp",
		ports:   []int{8080, 8081},
		markers: []string{"llama.cpp", "llamacpp"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/props")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			if jsonWalk(m, "default_generation_settings") == nil && jsonWalk(m, "model_path") == nil {
				return nil
			}
			info := &aiInfo{Product: "llama.cpp", Kind: "llm"}
			if path := jsonStr(m, "model_path"); path != "" {
				info.Models = append(info.Models, path)
			}
			if n := jsonStr(m, "default_generation_settings", "n_ctx"); n != "" {
				info.System = "ctx " + n
			}
			return info
		},
	},
	{
		// OpenAI 兼容端点：vLLM / LM Studio / LocalAI / TGI 等都会列出模型。
		product: "OpenAI 兼容 API",
		ports:   []int{1234, 5001, 3001, 4000, 8000, 9000},
		markers: []string{"vllm", "lm studio", "lmstudio", "localai", "text-generation-inference", "sglang", "openai"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/v1/models")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			items := jsonList(m, "data")
			if m == nil || items == nil {
				return nil
			}
			info := &aiInfo{Kind: "llm"}
			owners := map[string]bool{}
			for _, item := range items {
				model, _ := item.(map[string]any)
				id := jsonStr(model, "id")
				if id == "" {
					continue
				}
				if len(info.Models) < 20 {
					info.Models = append(info.Models, id)
				}
				if owner := jsonStr(model, "owned_by"); owner != "" {
					owners[owner] = true
				}
			}
			// 归属字段是最可靠的厂商标识，其次才回落到通用名称。
			product := ""
			for owner := range owners {
				switch strings.ToLower(owner) {
				case "vllm":
					product = "vLLM"
				case "localai", "local-ai":
					product = "LocalAI"
				case "lmstudio", "lm-studio":
					product = "LM Studio"
				case "llamacpp", "llama.cpp", "ggerganov":
					product = "llama.cpp"
				}
				if product != "" {
					break
				}
			}
			if product == "" {
				product = "OpenAI 兼容 API"
				if t.port == 1234 {
					product = "LM Studio"
				} else if t.port == 8000 {
					product = "vLLM"
				}
			}
			if product == "LM Studio" || product == "vLLM" {
				info.Note = "根据默认端口推断"
			}
			info.Product = product
			if ver := t.get("/version"); ver.ok() {
				info.Version = jsonStr(jsonObj(ver.body), "version")
			}
			return info
		},
	},
	{
		// Open WebUI 的 /api/version 无需鉴权。
		product: "Open WebUI",
		markers: []string{"open webui", "open-webui"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/api/version")
			if !r.ok() {
				return nil
			}
			version := jsonStr(jsonObj(r.body), "version")
			if version == "" {
				return nil
			}
			return &aiInfo{Product: "Open WebUI", Version: version, Kind: "platform"}
		},
	},
	{
		// Jupyter /api 返回内核版本；开启 token 时回落成 403 也是明确指纹。
		product: "Jupyter",
		ports:   []int{8888, 8889},
		markers: []string{"jupyter"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/api")
			if r == nil {
				return nil
			}
			if r.status == 401 || r.status == 403 {
				if strings.Contains(strings.ToLower(r.header["server"]), "tornado") || strings.Contains(strings.ToLower(r.header["content-type"]), "json") {
					return &aiInfo{Product: "Jupyter", Kind: "platform", Note: "接口需要认证"}
				}
				return nil
			}
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "version")
			if version == "" || jsonWalk(m, "revision") == nil {
				return nil
			}
			return &aiInfo{Product: "Jupyter", Version: version, Kind: "platform"}
		},
	},
	{
		// Docker Engine API：未鉴权暴露等于主机完全失控，属于高危发现。
		product: "Docker Engine",
		ports:   []int{2375, 2376},
		markers: []string{"docker"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/version")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "Version")
			if version == "" || jsonStr(m, "ApiVersion") == "" {
				return nil
			}
			sys := []string{}
			if os := jsonStr(m, "Os"); os != "" {
				arch := jsonStr(m, "Arch")
				sys = append(sys, strings.TrimSpace(os+"/"+arch))
			}
			if kernel := jsonStr(m, "KernelVersion"); kernel != "" {
				sys = append(sys, "kernel "+kernel)
			}
			return &aiInfo{Product: "Docker Engine", Version: version, Kind: "infra", System: strings.Join(sys, ", "), Note: "API 未鉴权"}
		},
	},
	{
		product: "Kubernetes",
		ports:   []int{6443},
		markers: []string{"kubernetes"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/version")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "gitVersion")
			if version == "" || jsonStr(m, "major") == "" {
				return nil
			}
			sys := ""
			if goVersion := jsonStr(m, "goVersion"); goVersion != "" {
				sys = goVersion
			}
			return &aiInfo{Product: "Kubernetes", Version: version, Kind: "infra", System: sys}
		},
	},
	{
		product: "etcd",
		ports:   []int{2379, 2380},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/version")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "etcdserver")
			if version == "" {
				return nil
			}
			sys := []string{}
			if cluster := jsonStr(m, "etcdcluster"); cluster != "" {
				sys = append(sys, "cluster "+cluster)
			}
			return &aiInfo{Product: "etcd", Version: version, Kind: "infra", System: strings.Join(sys, ", ")}
		},
	},
	{
		product: "ClickHouse",
		ports:   []int{8123},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/?query=SELECT%20version()")
			if !r.ok() {
				return nil
			}
			version := strings.TrimSpace(string(r.body))
			if version == "" || strings.Count(version, ".") < 2 || len(version) > 32 {
				return nil
			}
			for _, ch := range version {
				if (ch < '0' || ch > '9') && ch != '.' {
					return nil
				}
			}
			return &aiInfo{Product: "ClickHouse", Version: version, Kind: "db"}
		},
	},
	{
		product: "InfluxDB",
		ports:   []int{8086},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/health")
			if r == nil {
				return nil
			}
			m := jsonObj(r.body)
			if !strings.EqualFold(jsonStr(m, "name"), "influxdb") {
				return nil
			}
			return &aiInfo{Product: "InfluxDB", Version: jsonStr(m, "version"), Kind: "db", Note: jsonStr(m, "status")}
		},
	},
	{
		product: "HashiCorp Consul",
		ports:   []int{8500},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/v1/agent/self")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "Config", "Version")
			if version == "" {
				return nil
			}
			sys := []string{}
			if dc := jsonStr(m, "Config", "Datacenter"); dc != "" {
				sys = append(sys, "datacenter "+dc)
			}
			if node := jsonStr(m, "Config", "NodeName"); node != "" {
				sys = append(sys, "node "+node)
			}
			return &aiInfo{Product: "HashiCorp Consul", Version: version, Kind: "infra", System: strings.Join(sys, ", ")}
		},
	},
	{
		product: "RabbitMQ",
		ports:   []int{15672, 15671},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/api/overview")
			if r == nil {
				return nil
			}
			if r.status == 401 {
				return &aiInfo{Product: "RabbitMQ", Kind: "queue", Note: "管理接口需要认证"}
			}
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "rabbitmq_version")
			if version == "" {
				return nil
			}
			sys := []string{}
			if erlang := jsonStr(m, "erlang_version"); erlang != "" {
				sys = append(sys, "Erlang "+erlang)
			}
			if cluster := jsonStr(m, "cluster_name"); cluster != "" {
				sys = append(sys, cluster)
			}
			return &aiInfo{Product: "RabbitMQ", Version: version, Kind: "queue", System: strings.Join(sys, ", ")}
		},
	},
	{
		product: "Grafana",
		ports:   []int{3000},
		markers: []string{"grafana"},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/api/health")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			version := jsonStr(m, "version")
			if version == "" || jsonStr(m, "database") == "" {
				return nil
			}
			return &aiInfo{Product: "Grafana", Version: version, Kind: "ops", Note: "database " + jsonStr(m, "database")}
		},
	},
	{
		product: "Qdrant",
		ports:   []int{6333},
		run: func(t *aiTarget) *aiInfo {
			r := t.get("/collections")
			if !r.ok() {
				return nil
			}
			m := jsonObj(r.body)
			if jsonStr(m, "status") != "ok" || jsonWalk(m, "result", "collections") == nil {
				return nil
			}
			info := &aiInfo{Product: "Qdrant", Kind: "ai"}
			for _, item := range jsonList(m, "result", "collections") {
				col, _ := item.(map[string]any)
				if name := jsonStr(col, "name"); name != "" && len(info.Models) < 20 {
					info.Models = append(info.Models, name)
				}
			}
			return info
		},
	},
}

// probeAI 在 HTTP 探测之后追加主动指纹；只有端口或首页特征命中时才发请求。
func (p *Prober) probeAI(ctx context.Context, ip string, port int, scheme string, res *Result) *aiInfo {
	target := &aiTarget{p: p, ctx: ctx, scheme: scheme, ip: ip, port: port}
	for _, ep := range aiProbes {
		if target.used >= aiMaxRequests {
			break
		}
		if !ep.applies(port, res) {
			continue
		}
		if info := ep.run(target); info != nil {
			if info.Product == "" {
				info.Product = ep.product
			}
			return info
		}
	}
	return nil
}

// applyAI 把主动指纹结论并回探测结果。
func applyAI(res *Result, info *aiInfo) {
	if info.Version != "" {
		res.Service.Product = info.Product
		res.Service.Version = info.Version
	} else if res.Service.Product == "" {
		res.Service.Product = info.Product
	}
	extra := info.summary()
	if extra != "" {
		if res.Service.ExtraInfo != "" {
			extra = res.Service.ExtraInfo + " | " + extra
		}
		res.Service.ExtraInfo = util.Truncate(extra, 200)
	}
	res.Apps = appendUnique(res.Apps, info.Product)
	if info.OS != "" && res.OS == "" {
		res.OS = info.OS
	}
	models := make([]string, 0, len(info.Models))
	for _, name := range info.Models {
		models = append(models, util.Truncate(name, 120))
	}
	res.AI = &model.AIInfo{
		Product: util.Truncate(info.Product, 80),
		Version: util.Truncate(info.Version, 60),
		Kind:    info.Kind,
		Models:  models,
		System:  util.Truncate(info.System, 300),
		Note:    util.Truncate(info.Note, 200),
	}
}
