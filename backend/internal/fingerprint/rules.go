package fingerprint

import (
	"regexp"
	"strings"
)

// 指纹规则覆盖两个维度：HTTP 响应特征与协议 banner。
// 规则只做字符串命中，不做任何主动探测，避免误报与侵入。

type matchRule struct {
	App     string
	Target  string // header | body | title | cookie
	Key     string // Target=header 时的头名（小写），为空表示扫描所有头
	Pattern *regexp.Regexp
}

func mustRE(p string) *regexp.Regexp { return regexp.MustCompile("(?i)" + p) }

var httpRules = []matchRule{
	{App: "WordPress", Target: "body", Pattern: mustRE(`/wp-(content|includes)/`)},
	{App: "WordPress", Target: "header", Key: "x-pingback", Pattern: mustRE(`xmlrpc\.php`)},
	{App: "ThinkPHP", Target: "body", Pattern: mustRE(`thinkphp|/thinkphp_`)},
	{App: "ThinkPHP", Target: "header", Key: "x-powered-by", Pattern: mustRE(`thinkphp`)},
	{App: "Shiro", Target: "header", Key: "set-cookie", Pattern: mustRE(`rememberMe=deleteMe`)},
	{App: "Spring Boot", Target: "body", Pattern: mustRE(`Whitelabel Error Page`)},
	{App: "Spring", Target: "header", Key: "x-application-context", Pattern: mustRE(`.`)},
	{App: "Laravel", Target: "cookie", Pattern: mustRE(`laravel_session`)},
	{App: "Jenkins", Target: "header", Key: "x-jenkins", Pattern: mustRE(`.`)},
	{App: "Jenkins", Target: "title", Pattern: mustRE(`Dashboard \[Jenkins\]`)},
	{App: "GitLab", Target: "body", Pattern: mustRE(`gon\.gitlab_url|content="GitLab"`)},
	{App: "GitHub Enterprise", Target: "body", Pattern: mustRE(`github-enterprise`)},
	{App: "Grafana", Target: "body", Pattern: mustRE(`public/build/grafana|grafana-app`)},
	{App: "Kibana", Target: "body", Pattern: mustRE(`kbn-injected-metadata`)},
	{App: "Elasticsearch", Target: "body", Pattern: mustRE(`"You Know, for Search"`)},
	{App: "MinIO", Target: "body", Pattern: mustRE(`<title>MinIO`)},
	{App: "MinIO", Target: "header", Key: "x-minio-deployment-id", Pattern: mustRE(`.`)},
	{App: "Portainer", Target: "body", Pattern: mustRE(`ng-app="portainer"|portainer\.js`)},
	{App: "Nacos", Target: "body", Pattern: mustRE(`/nacos/|nacos\.js`)},
	{App: "Zabbix", Target: "title", Pattern: mustRE(`Zabbix`)},
	{App: "Prometheus", Target: "title", Pattern: mustRE(`Prometheus`)},
	{App: "Alertmanager", Target: "title", Pattern: mustRE(`Alertmanager`)},
	{App: "RabbitMQ", Target: "title", Pattern: mustRE(`RabbitMQ Management`)},
	{App: "phpMyAdmin", Target: "body", Pattern: mustRE(`phpMyAdmin|pma_username`)},
	{App: "Apache Tomcat", Target: "body", Pattern: mustRE(`Apache Tomcat`)},
	{App: "Apache Tomcat", Target: "title", Pattern: mustRE(`Tomcat`)},
	{App: "WebLogic", Target: "body", Pattern: mustRE(`WebLogic`)},
	{App: "JBoss", Target: "body", Pattern: mustRE(`JBoss`)},
	{App: "Discuz", Target: "body", Pattern: mustRE(`Discuz!|discuz_uid`)},
	{App: "DedeCMS", Target: "body", Pattern: mustRE(`DedeCMS|dedecms`)},
	{App: "Joomla", Target: "body", Pattern: mustRE(`/media/system/js/|option=com_`)},
	{App: "Drupal", Target: "header", Key: "x-generator", Pattern: mustRE(`Drupal`)},
	{App: "Kubernetes Dashboard", Target: "title", Pattern: mustRE(`Kubernetes Dashboard`)},
	{App: "宝塔面板", Target: "body", Pattern: mustRE(`BT-Panel|bt\.cn/`)},
	{App: "宝塔面板", Target: "title", Pattern: mustRE(`宝塔`)},
	{App: "Synology DSM", Target: "body", Pattern: mustRE(`Synology|DSM`)},
	{App: "Hikvision", Target: "body", Pattern: mustRE(`Hikvision|DNVRS`)},
	{App: "Dahua", Target: "body", Pattern: mustRE(`Dahua|dahuaTechnology`)},
	{App: "泛微 e-cology", Target: "body", Pattern: mustRE(`/wui/index\.html|ecology`)},
	{App: "致远 OA", Target: "body", Pattern: mustRE(`seeyon`)},
	{App: "通达 OA", Target: "title", Pattern: mustRE(`通达OA`)},
	{App: "用友 NC", Target: "body", Pattern: mustRE(`/nc/|用友`)},
	{App: "Apache APISIX", Target: "title", Pattern: mustRE(`APISIX`)},
	{App: "Next.js", Target: "body", Pattern: mustRE(`__NEXT_DATA__`)},
	{App: "Vue", Target: "body", Pattern: mustRE(`id="app"|__VUE__|vue\.js`)},
	{App: "React", Target: "body", Pattern: mustRE(`data-reactroot|_next/static`)},
	// AI 推理与模型服务：首页通常只是一个前端壳，产品名往往写在标题或静态资源里。
	{App: "Ollama", Target: "body", Pattern: mustRE(`Ollama is running`)},
	{App: "Open WebUI", Target: "title", Pattern: mustRE(`Open WebUI`)},
	{App: "Open WebUI", Target: "body", Pattern: mustRE(`/static/ollama\.png|open-webui`)},
	{App: "ComfyUI", Target: "title", Pattern: mustRE(`ComfyUI`)},
	{App: "ComfyUI", Target: "body", Pattern: mustRE(`comfyui|/scripts/app\.js`)},
	{App: "Stable Diffusion WebUI", Target: "title", Pattern: mustRE(`Stable Diffusion`)},
	{App: "Stable Diffusion WebUI", Target: "body", Pattern: mustRE(`gradio_config|sd-webui|stable-diffusion-webui`)},
	{App: "Text Generation WebUI", Target: "title", Pattern: mustRE(`Text generation web UI`)},
	{App: "Jupyter", Target: "title", Pattern: mustRE(`Jupyter`)},
	{App: "Jupyter", Target: "body", Pattern: mustRE(`jupyter_notebook|jupyterlab`)},
	{App: "Dify", Target: "body", Pattern: mustRE(`dify\.ai|__dify`)},
	{App: "LibreChat", Target: "title", Pattern: mustRE(`LibreChat`)},
	{App: "AnythingLLM", Target: "body", Pattern: mustRE(`anythingllm`)},
	{App: "Flowise", Target: "body", Pattern: mustRE(`flowise`)},
	{App: "FastGPT", Target: "body", Pattern: mustRE(`fastgpt`)},
	{App: "n8n", Target: "body", Pattern: mustRE(`n8n-editor-ui|n8n\.io`)},
	{App: "LM Studio", Target: "body", Pattern: mustRE(`lm-studio|lmstudio`)},
	{App: "LocalAI", Target: "body", Pattern: mustRE(`localai`)},
	{App: "Milvus", Target: "body", Pattern: mustRE(`milvus`)},
	{App: "Weaviate", Target: "body", Pattern: mustRE(`weaviate`)},
	{App: "Langfuse", Target: "body", Pattern: mustRE(`langfuse`)},
	// 常见运维/数据面板，扫描里出现频率高且暴露面大。
	{App: "Superset", Target: "title", Pattern: mustRE(`Superset`)},
	{App: "Metabase", Target: "title", Pattern: mustRE(`Metabase`)},
	{App: "Airflow", Target: "title", Pattern: mustRE(`Airflow`)},
	{App: "SonarQube", Target: "body", Pattern: mustRE(`sonarqube`)},
	{App: "Harbor", Target: "body", Pattern: mustRE(`harbor-core|harbor\.yml`)},
	{App: "ArgoCD", Target: "title", Pattern: mustRE(`Argo CD`)},
	{App: "Consul", Target: "title", Pattern: mustRE(`Consul by HashiCorp`)},
	{App: "etcd", Target: "body", Pattern: mustRE(`etcdserver|etcdcluster`)},
	{App: "ClickHouse", Target: "body", Pattern: mustRE(`ClickHouse`)},
	{App: "InfluxDB", Target: "body", Pattern: mustRE(`influxdb`)},
	{App: "Cockpit", Target: "title", Pattern: mustRE(`Cockpit`)},
	{App: "Jellyfin", Target: "body", Pattern: mustRE(`jellyfin`)},
	{App: "Plex", Target: "header", Key: "x-plex-version", Pattern: mustRE(`.`)},
}

var serverApps = map[string]string{
	"nginx":          "nginx",
	"openresty":      "OpenResty",
	"apache":         "Apache",
	"httpd":          "Apache",
	"microsoft-iis":  "Microsoft-IIS",
	"iis":            "Microsoft-IIS",
	"cloudflare":     "Cloudflare",
	"caddy":          "Caddy",
	"litespeed":      "LiteSpeed",
	"jetty":          "Jetty",
	"werkzeug":       "Werkzeug",
	"tornado":        "Tornado",
	"gunicorn":       "Gunicorn",
	"uvicorn":        "Uvicorn",
	"kestrel":        "Kestrel",
	"tomcat":         "Apache Tomcat",
	"mikrotik":       "RouterOS",
	"routeros":       "RouterOS",
	"awselb":         "AWS ELB",
	"envoy":          "Envoy",
	"traefik":        "Traefik",
	"nginx-ui":       "nginx-ui",
	"prometheus":     "Prometheus",
	"warp":           "Deno",
	"deno":           "Deno",
	"go-http-client": "",
	"ollama":         "Ollama",
	"jupyter":        "Jupyter",
	"hypercorn":      "Hypercorn",
	"waitress":       "Waitress",
	"clickhouse":     "ClickHouse",
	"influxdb":       "InfluxDB",
	"vllm":           "vLLM",
	"mlflow":         "MLflow",
}

// normalizeServer 把 Server 头折叠成产品名与版本。
func normalizeServer(v string) (app, version string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", ""
	}
	token := v
	if i := strings.IndexAny(token, " ;("); i > 0 {
		token = token[:i]
	}
	name := strings.ToLower(token)
	if i := strings.Index(name, "/"); i > 0 {
		version = token[i+1:]
		name = name[:i]
	}
	return serverApps[name], version
}
