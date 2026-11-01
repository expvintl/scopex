package model

import "strings"

// 本文件提供面向统计口径的归类函数。
// 返回的是稳定的英文键（如 "linux"、"asia"、"ai"），由前端做本地化展示；
// 这样后端聚合结果与界面语言解耦，也能被 API 使用方直接消费。

// OSFamily 把操作系统的各种写法归并成统计用的家族键。
//
// 识别来源很杂：SSH banner、HTTP Server 头、SMB 原生 OS 串、ComfyUI/Docker
// 这类服务自报的运行环境，写法从 "Ubuntu 22.04.3 LTS" 到 "posix" 都有，
// 统计时只关心它属于哪个家族。
func OSFamily(os string) string {
	v := strings.ToLower(strings.TrimSpace(os))
	if v == "" {
		return ""
	}
	switch {
	case strings.Contains(v, "windows"):
		return "windows"
	case strings.Contains(v, "android"):
		return "android"
	case strings.Contains(v, "ios"), strings.Contains(v, "iphone"), strings.Contains(v, "darwin"), strings.Contains(v, "mac os"), strings.Contains(v, "macos"):
		return "macos"
	case strings.Contains(v, "linux"),
		strings.Contains(v, "ubuntu"), strings.Contains(v, "debian"), strings.Contains(v, "centos"),
		strings.Contains(v, "red hat"), strings.Contains(v, "rhel"), strings.Contains(v, "fedora"),
		strings.Contains(v, "alpine"), strings.Contains(v, "suse"), strings.Contains(v, "arch"),
		strings.Contains(v, "openwrt"), strings.Contains(v, "dd-wrt"), strings.Contains(v, "routeros"),
		strings.Contains(v, "pfsense"), strings.Contains(v, "synology"), strings.Contains(v, "qnap"),
		strings.Contains(v, "gentoo"), strings.Contains(v, "raspbian"), strings.Contains(v, "openEuler"),
		strings.Contains(v, "kylin"), strings.Contains(v, "uos"), strings.Contains(v, "harmony"):
		return "linux"
	case strings.Contains(v, "freebsd"), strings.Contains(v, "openbsd"), strings.Contains(v, "netbsd"), strings.Contains(v, "unix"), strings.Contains(v, "solaris"), strings.Contains(v, "aix"):
		return "unix"
	case strings.Contains(v, "ios-xe"), strings.Contains(v, "ios xe"), strings.Contains(v, "junos"), strings.Contains(v, "fortios"), strings.Contains(v, "pan-os"), strings.Contains(v, "cisco"):
		return "network"
	}
	return "other"
}

// OSFamilyOf 在 host.os 为空时用 CPE 与 banner 线索兜底推断操作系统家族。
//
// 很多资产探测不到 os 字段（HTTP 服务自报的运行环境有限），但 nmap 的
// cpe:/o: 条目、SSH/HTTP banner 里的发行版字样同样是可靠证据；汇总页需要
// 尽量填满这一块，否则「系统分布」在只扫 Web 的场景下永远是空的。
func OSFamilyOf(h *Host) string {
	if h == nil {
		return ""
	}
	if fam := OSFamily(h.OS); fam != "" && fam != "other" {
		return fam
	}
	for _, c := range h.CPE {
		v := strings.ToLower(c)
		if !strings.HasPrefix(v, "cpe:/o:") {
			continue
		}
		if fam := OSFamily(strings.ReplaceAll(v, ":", " ")); fam != "" && fam != "other" {
			return fam
		}
	}
	// banner / Server 头里可能是任意文本，只匹配足够明确的发行版关键字，避免 "arch"/"unix" 误判。
	text := strings.ToLower(h.Banner + " " + h.Service.ExtraInfo)
	for key, val := range h.Headers {
		if strings.EqualFold(key, "server") || strings.EqualFold(key, "x-powered-by") {
			text += " " + strings.ToLower(val)
		}
	}
	for _, token := range []string{"windows", "ubuntu", "debian", "centos", "red hat", "rhel",
		"fedora", "alpine", "opensuse", "freebsd", "openbsd", "netbsd", "darwin", "macos"} {
		if strings.Contains(text, token) {
			return OSFamily(token)
		}
	}
	return OSFamily(h.OS)
}

// appCategories 把应用/产品指纹归入统计品类。
// 键统一小写，值取自 AppCategory 的键集合；未命中返回 "other"。
var appCategories = map[string]string{
	// AI 推理与模型服务
	"ollama": "ai", "open webui": "ai", "vllm": "ai", "lm studio": "ai", "localai": "ai",
	"text generation webui": "ai", "comfyui": "ai", "stable diffusion webui": "ai",
	"jupyter": "ai", "dify": "ai", "librechat": "ai", "fastgpt": "ai", "anythingllm": "ai",
	"flowise": "ai", "langfuse": "ai", "llama.cpp": "ai", "xinference": "ai", "vllm openai": "ai",
	"mlflow": "ai", "triton": "ai", "tensorrt": "ai",
	"milvus": "ai", "qdrant": "ai", "weaviate": "ai", "chroma": "ai", "pgvector": "ai",

	// Web 服务器与运行时
	"nginx": "web", "apache": "web", "httpd": "web", "openresty": "web", "tengine": "web",
	"iis": "web", "caddy": "web", "tomcat": "web", "jetty": "web", "node.js": "web",
	"express": "web", "gunicorn": "web", "uvicorn": "web", "werkzeug": "web", "php": "web",

	// 应用与框架
	"wordpress": "app", "thinkphp": "app", "spring boot": "app", "spring": "app", "laravel": "app",
	"django": "app", "flask": "app", "rails": "app", "discuz": "app", "dedecms": "app",
	"joomla": "app", "drupal": "app", "shiro": "app", "宝塔面板": "app", "1panel": "app",
	"gitlab": "app", "github enterprise": "app", "gitea": "app", "nextcloud": "app",
	"jellyfin": "app", "plex": "app", "emby": "app", "wiki.js": "app", "outline": "app",

	// 运维与监控
	"grafana": "ops", "kibana": "ops", "prometheus": "ops", "alertmanager": "ops", "zabbix": "ops",
	"jenkins": "ops", "portainer": "ops", "rancher": "ops", "argocd": "ops", "harbor": "ops",
	"nacos": "ops", "consul": "ops", "etcd": "ops", "kubernetes": "ops", "kubernetes dashboard": "ops",
	"docker": "ops", "cockpit": "ops", "rundeck": "ops", "sonarqube": "ops", "n8n": "ops",
	"airflow": "ops", "jaeger": "ops", "zipkin": "ops", "skywalking": "ops", "nexus": "ops",
	"superset": "ops", "metabase": "ops", "consul by hashicorp": "ops",

	// 数据库与缓存
	"mysql": "db", "mariadb": "db", "postgresql": "db", "mongodb": "db", "redis": "db",
	"memcached": "db", "elasticsearch": "db", "clickhouse": "db", "influxdb": "db",
	"couchdb": "db", "cassandra": "db", "oracle": "db", "sql server": "db", "neo4j": "db",
	"rabbitmq": "queue", "kafka": "queue", "activemq": "queue", "rocketmq": "queue", "nats": "queue",

	// 文件与共享
	"smb": "storage", "samba": "storage", "nfs": "storage", "ftp": "storage", "vsftpd": "storage",
	"minio": "storage", "rsync": "storage", "webdav": "storage", "synology dsm": "storage",
	"qnap": "storage", "nextcloud office": "storage",

	// 远程管理与目录
	"ssh": "remote", "openssh": "remote", "rdp": "remote", "vnc": "remote", "telnet": "remote",
	"winrm": "remote", "teamviewer": "remote", "active directory": "directory", "ldap": "directory",
	"openldap": "directory", "kerberos": "directory",

	// 邮件
	"smtp": "mail", "postfix": "mail", "exim": "mail", "exchange": "mail", "dovecot": "mail",
	"zimbra": "mail", "roundcube": "mail",

	// 安全设备与 IoT
	"hikvision": "iot", "dahua": "iot", "uniview": "iot", "axis": "iot", "mikrotik": "iot",
	"fortinet": "iot", "palo alto": "iot", "sophos": "iot", "greenbone": "security",
	"nessus": "security", "openvas": "security",
}

// AppCategory 返回应用指纹所属的统计品类键。
func AppCategory(app string) string {
	if c, ok := appCategories[strings.ToLower(strings.TrimSpace(app))]; ok {
		return c
	}
	return "other"
}

// CategoryKeys 是品类键的固定顺序，便于前端保持图例顺序稳定。
var CategoryKeys = []string{"web", "app", "ai", "db", "queue", "storage", "remote", "directory", "mail", "ops", "security", "iot", "other"}

// continentByCode 依据联合国地理方案把 ISO 3166-1 两位国家码归入大洲。
// 这里只做统计展示，个别跨洲国家（TR、RU、KZ 等）沿用通行划分。
var continentByCode = map[string]string{}

func init() {
	groups := map[string]string{
		"asia":          "AF AM AZ BH BD BT BN KH CN CY GE HK IN ID IR IQ IL JP JO KZ KP KR KW KG LA LB MO MY MV MN MM NP OM PK PS PH QA SA SG LK SY TW TJ TH TL TR TM AE UZ VN YE",
		"europe":        "AL AD AT BY BE BA BG HR CZ DK EE FI FR DE GR HU IS IE IT XK LV LI LT LU MT MD MC ME NL MK NO PL PT RO RU SM RS SK SI ES SE CH UA GB VA GG JE IM AX FO GI",
		"africa":        "DZ AO BJ BW BF BI CV CM CF TD KM CG CD CI DJ EG GQ ER SZ ET GA GM GH GN GW KE LS LR LY MG MW ML MR MU MA MZ NA NE NG RW ST SN SC SL SO ZA SS SD TZ TG TN UG ZM ZW EH RE YT SH",
		"north_america": "AG BS BB BZ CA CR CU DM DO SV GD GT HT HN JM MX NI PA KN LC VC TT US PR BM GL AW CW SX BQ KY TC VG VI AI MS GU MP AS",
		"south_america": "AR BO BR CL CO EC GY PY PE SR UY VE FK GF",
		"oceania":       "AU NZ FJ PG SB VU WS TO TV KI NR FM MH PW NC PF CK NU NF PN TL",
		"antarctica":    "AQ BV GS HM TF",
	}
	for continent, codes := range groups {
		for _, code := range strings.Fields(codes) {
			continentByCode[code] = continent
		}
	}
}

// Continent 返回国家码所属的大洲键，未知或非法返回空串。
func Continent(code string) string {
	return continentByCode[strings.ToUpper(strings.TrimSpace(code))]
}

// serviceCategories 把协议层的 service.name 归入统计大类。
// 这里的键与 AppCategory 共用一套展示标签（cat.*），便于前端复用配色。
var serviceCategories = map[string]string{
	"http": "web", "https": "web", "http-proxy": "web", "https-alt": "web", "http-alt": "web",
	"ssl/http": "web", "http-proxy-alt": "web",

	"ssh": "remote", "rdp": "remote", "vnc": "remote", "telnet": "remote", "ms-wbt-server": "remote",
	"wsman": "remote", "wsmans": "remote", "winrm": "remote", "exec": "remote", "login": "remote",
	"shell": "remote", "socks": "remote", "mumble": "remote",

	"mysql": "db", "postgresql": "db", "mongodb": "db", "redis": "db", "memcached": "db",
	"elasticsearch": "db", "couchdb": "db", "ms-sql-s": "db", "oracle": "db", "clickhouse": "db",
	"influxdb": "db", "qdrant": "db",

	"ftp": "storage", "sftp": "storage", "smb": "storage", "netbios-ssn": "storage",
	"microsoft-ds": "storage", "nfs": "storage", "rsync": "storage", "webdav": "storage",

	"smtp": "mail", "smtps": "mail", "submission": "mail", "pop3": "mail", "pop3s": "mail",
	"imap": "mail", "imaps": "mail",

	"ldap": "directory", "ldaps": "directory", "kerberos": "directory", "rpcbind": "directory",

	"amqp": "queue", "amqps": "queue", "mqtt": "queue", "kafka": "queue",

	"docker": "ops", "kubernetes": "ops", "etcd": "ops", "consul": "ops", "prometheus": "ops",
	"rabbitmq": "ops", "kibana": "ops", "domain": "ops",
}

// ServiceCategory 返回协议服务所属的统计大类键。
func ServiceCategory(name string) string {
	if c, ok := serviceCategories[strings.ToLower(strings.TrimSpace(name))]; ok {
		return c
	}
	return "other"
}
