package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/expvintl/scopex/internal/model"
)

// newAITarget 把 httptest 服务包装成探测目标，避免测试依赖固定端口。
func newAITarget(t *testing.T, server string) *aiTarget {
	t.Helper()
	u, err := url.Parse(server)
	if err != nil {
		t.Fatalf("解析测试服务地址失败: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("测试服务端口非法: %v", err)
	}
	return &aiTarget{
		p:      NewProber(2 * time.Second),
		ctx:    context.Background(),
		scheme: "http",
		ip:     u.Hostname(),
		port:   port,
	}
}

// serve 起一个按路径返回固定响应的服务。
func serve(t *testing.T, routes map[string]string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.RequestURI()]
		if !ok {
			body, ok = routes[r.URL.Path]
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func probeByProduct(t *testing.T, product string) *aiProbe {
	t.Helper()
	for _, ep := range aiProbes {
		if ep.product == product {
			return ep
		}
	}
	t.Fatalf("未找到产品探测定义: %s", product)
	return nil
}

func TestAIOllamaProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/api/version": `{"version":"0.3.14"}`,
		"/api/tags":    `{"models":[{"name":"llama3:8b","details":{"parameter_size":"8B"}},{"name":"qwen2.5:7b"}]}`,
	})
	info := probeByProduct(t, "Ollama").run(newAITarget(t, addr))
	if info == nil {
		t.Fatal("未识别出 Ollama")
	}
	if info.Product != "Ollama" || info.Version != "0.3.14" {
		t.Fatalf("产品/版本不符: %+v", info)
	}
	if len(info.Models) != 2 || info.Models[0] != "llama3:8b (8B)" {
		t.Fatalf("模型清单不符: %v", info.Models)
	}
	if info.Kind != "llm" {
		t.Fatalf("品类应为 llm，实际 %q", info.Kind)
	}
}

func TestAIComfyUIProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/system_stats": `{"system":{"os":"posix","python_version":"3.11.9","comfyui_version":"0.3.36"},
			"devices":[{"name":"cuda:0 NVIDIA GeForce RTX 4090","type":"cuda","vram_total":25757220864}]}`,
	})
	info := probeByProduct(t, "ComfyUI").run(newAITarget(t, addr))
	if info == nil {
		t.Fatal("未识别出 ComfyUI")
	}
	if info.Version != "0.3.36" || info.OS != "Linux" {
		t.Fatalf("版本/系统不符: %+v", info)
	}
	if info.Kind != "image" {
		t.Fatalf("品类应为 image，实际 %q", info.Kind)
	}
	if want := "显存 24GB"; !strings.Contains(info.System, want) {
		t.Fatalf("系统信息应包含 %q，实际 %q", want, info.System)
	}
	if !strings.Contains(info.System, "RTX 4090") {
		t.Fatalf("系统信息应包含显卡型号，实际 %q", info.System)
	}
}

func TestAIStableDiffusionProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/sdapi/v1/options": `{"sd_model_checkpoint":"v1-5-pruned-emaonly.safetensors [6ce0161689]","sd_vae":"vae-ft-mse"}`,
	})
	info := probeByProduct(t, "Stable Diffusion WebUI").run(newAITarget(t, addr))
	if info == nil {
		t.Fatal("未识别出 Stable Diffusion WebUI")
	}
	if len(info.Models) != 1 || !strings.Contains(info.Models[0], "v1-5-pruned") {
		t.Fatalf("模型清单不符: %v", info.Models)
	}
}

func TestAIOpenAICompatibleProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/v1/models":   `{"object":"list","data":[{"id":"Qwen/Qwen2.5-7B-Instruct","object":"model","owned_by":"vllm"},{"id":"meta-llama/Llama-3-8B","object":"model","owned_by":"vllm"}]}`,
		"/version":     `{"version":"0.6.3"}`,
		"/api/version": `{"version":"0.6.3"}`,
	})
	info := probeByProduct(t, "OpenAI 兼容 API").run(newAITarget(t, addr))
	if info == nil {
		t.Fatal("未识别出 OpenAI 兼容端点")
	}
	if info.Product != "vLLM" {
		t.Fatalf("应根据 owned_by 判定为 vLLM，实际 %q", info.Product)
	}
	if info.Version != "0.6.3" {
		t.Fatalf("版本不符: %q", info.Version)
	}
	if len(info.Models) != 2 {
		t.Fatalf("模型数量不符: %v", info.Models)
	}
}

func TestAIDockerProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/version": `{"Version":"24.0.7","ApiVersion":"1.43","Os":"linux","Arch":"amd64","KernelVersion":"5.15.0-91-generic"}`,
	})
	info := probeByProduct(t, "Docker Engine").run(newAITarget(t, addr))
	if info == nil {
		t.Fatal("未识别出 Docker Engine")
	}
	if info.Version != "24.0.7" || info.Kind != "infra" {
		t.Fatalf("结论不符: %+v", info)
	}
	if !strings.Contains(info.System, "linux/amd64") || !strings.Contains(info.Note, "未鉴权") {
		t.Fatalf("补充信息不符: %+v", info)
	}
}

func TestAIClickHouseProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/?query=SELECT%20version()": "23.8.2.7\n",
	})
	info := probeByProduct(t, "ClickHouse").run(newAITarget(t, addr))
	if info == nil || info.Version != "23.8.2.7" {
		t.Fatalf("未正确识别 ClickHouse: %+v", info)
	}
}

func TestAIClickHouseRejectsNonVersion(t *testing.T) {
	addr := serve(t, map[string]string{
		"/?query=SELECT%20version()": "<html>hello</html>",
	})
	if info := probeByProduct(t, "ClickHouse").run(newAITarget(t, addr)); info != nil {
		t.Fatalf("不应识别出 ClickHouse: %+v", info)
	}
}

func TestAIQdrantProbe(t *testing.T) {
	addr := serve(t, map[string]string{
		"/collections": `{"result":{"collections":[{"name":"docs"}]},"status":"ok","time":0.0001}`,
	})
	info := probeByProduct(t, "Qdrant").run(newAITarget(t, addr))
	if info == nil || info.Product != "Qdrant" {
		t.Fatalf("未识别出 Qdrant: %+v", info)
	}
	if len(info.Models) != 1 || info.Models[0] != "docs" {
		t.Fatalf("集合清单不符: %v", info.Models)
	}
}

// 普通 JSON 接口不应被误判成任何产品。
func TestAINoFalsePositive(t *testing.T) {
	addr := serve(t, map[string]string{
		"/api/version": `{"name":"demo","built":"2024"}`,
		"/v1/models":   `{"items":[]}`,
		"/version":     `{"Version":"1.0"}`,
	})
	target := newAITarget(t, addr)
	for _, ep := range aiProbes {
		if info := ep.run(target); info != nil && ep.product != "OpenAI 兼容 API" {
			t.Fatalf("普通接口被误判为 %s: %+v", ep.product, info)
		}
		target.used = 0
	}
}

// 门槛判断：端口命中、特征命中才探测，二者都没有就不发请求。
func TestAIProbeApplies(t *testing.T) {
	ollama := probeByProduct(t, "Ollama")
	if !ollama.applies(11434, &Result{}) {
		t.Fatal("11434 端口应触发 Ollama 探测")
	}
	if ollama.applies(12345, &Result{}) {
		t.Fatal("无关端口不应触发 Ollama 探测")
	}
	byMarker := &Result{Title: "Ollama Web", Headers: map[string]string{}}
	if !ollama.applies(5000, byMarker) {
		t.Fatal("标题含 Ollama 时应触发探测")
	}
}

// applyAI 应把结论写进 service、apps 与 scripts。
func TestApplyAIWritesResult(t *testing.T) {
	res := &Result{Service: model.Service{Name: "http"}, Headers: map[string]string{}}
	applyAI(res, &aiInfo{
		Product: "Ollama",
		Version: "0.3.14",
		Kind:    "llm",
		Models:  []string{"llama3:8b", "qwen2.5:7b"},
		System:  "Linux, Python 3.11",
		OS:      "Linux",
		Note:    "API 未鉴权",
	})
	if res.Service.Product != "Ollama" || res.Service.Version != "0.3.14" {
		t.Fatalf("service 未更新: %+v", res.Service)
	}
	if res.OS != "Linux" {
		t.Fatalf("操作系统未更新: %q", res.OS)
	}
	if len(res.Apps) == 0 || res.Apps[0] != "Ollama" {
		t.Fatalf("apps 未更新: %v", res.Apps)
	}
	if res.AI == nil || res.AI.Product != "Ollama" || res.AI.Kind != "llm" || res.AI.Note != "API 未鉴权" {
		t.Fatalf("ai 结论未写入: %+v", res.AI)
	}
	if got := strings.Join(res.AI.Models, ", "); got != "llama3:8b, qwen2.5:7b" {
		t.Fatalf("模型清单未写入: %q", got)
	}
	if !strings.Contains(res.Service.ExtraInfo, "2 个模型") {
		t.Fatalf("extra_info 未汇总: %q", res.Service.ExtraInfo)
	}
	if deviceFromApps(res.Apps) != "ai-server" {
		t.Fatalf("AI 节点应归为 ai-server，实际 %q", deviceFromApps(res.Apps))
	}
}
