import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

export type Lang = 'zh' | 'en'
export type LangPref = 'auto' | Lang

const STORAGE_KEY = 'scopex.lang'

/** 以中文原文为键的英文字典，缺失时回退中文。 */
const en: Record<string, string> = {
  '24h 新增': 'New (24h)',
  '50+ Web 指纹、TLS 证书、favicon 哈希与协议 banner': '50+ web fingerprints, TLS certificates, favicon hashes and protocol banners',
  '192.168.0.0/16 或 10.0.0.1-10.0.0.255': '192.168.0.0/16 or 10.0.0.1-10.0.0.255',
  '192.168.6.0/24 或 10.0.0.1-10.0.0.50，空格/换行分隔': '192.168.6.0/24 or 10.0.0.1-10.0.0.50, separated by spaces or newlines',
  '80,443,8080,8443 或 1-1024': '80,443,8080,8443 or 1-1024',
  'ASN 库': 'ASN database',
  'ASN 库下载地址': 'ASN database URL',
  'IP 资产详情': 'Asset details',
  'Masscan 额外参数': 'Masscan extra arguments',
  'Nmap 脚本': 'Nmap scripts',
  'TLS 证书': 'TLS certificate',
  '上一页': 'Previous page',
  '下一页': 'Next page',
  '两次输入的新密码不一致': 'The two passwords do not match',
  '主体': 'Subject',
  '主机': 'Hosts',
  '仅管理员可修改扫描配置。': 'Only administrators can change scan settings.',
  '仅管理员可管理用户。': 'Only administrators can manage users.',
  '任务会立即停止，已发现的资产将保留。': 'The task stops immediately; discovered assets are kept.',
  '任务名称': 'Task name',
  '任务状态分布': 'Task status',
  '任务记录与运行日志会被删除，已发现的资产不受影响。': 'Task records and logs are deleted; discovered assets are kept.',
  '任务详情': 'Task details',
  '使用管理台账号进入 SCOPEX': 'Sign in to SCOPEX',
  '使用统一检索语法定位已测绘的资产': 'Locate mapped assets with a unified query syntax',
  '保存中…': 'Saving…',
  '保存失败': 'Save failed',
  '保存配置': 'Save settings',
  '修改密码': 'Change password',
  '修改密码失败': 'Failed to change password',
  '全部': 'All',
  '关闭': 'Close',
  '关闭导航': 'Close navigation',
  '关闭提示': 'Dismiss',
  '内网': 'Private network',
  '准备中': 'Preparing',
  '切换主题': 'Switch theme',
  '刚刚': 'just now',
  '创建中…': 'Creating…',
  '创建任务失败': 'Failed to create task',
  '创建并跟踪端口发现、指纹识别与服务探测任务': 'Create and track port discovery, fingerprinting and service detection',
  '创建时间': 'Created',
  '创建用户失败': 'Failed to create user',
  '删除中…': 'Deleting…',
  '删除任务': 'Delete task',
  '删除失败': 'Delete failed',
  '删除扫描任务': 'Delete scan task',
  '删除该 IP 的全部资产': 'Delete all assets of this IP',
  '删除资产': 'Delete asset',
  '刷新失败': 'Refresh failed',
  '刷新此 IP 的端口与归属地': 'Re-scan ports and geo for this IP',
  '加载 IP 资产': 'Load IP assets',
  '加载中': 'Loading',
  '加载中…': 'Loading…',
  '加载任务': 'Load task',
  '加载任务失败': 'Failed to load task',
  '加载地理库配置失败': 'Failed to load geo database settings',
  '加载失败': 'Load failed',
  '加载扫描配置失败': 'Failed to load scan settings',
  '加载用户': 'Load user',
  '加载用户失败': 'Failed to load user',
  '单任务目标上限': 'Targets per task',
  '原生 TCP': 'Native TCP',
  '原生 TCP 探测、Nmap 服务识别、Masscan 高速发现': 'Native TCP probing, Nmap service detection, Masscan discovery',
  '原生指纹识别': 'Native fingerprinting',
  '取消': 'Cancel',
  '取消扫描任务': 'Cancel scan task',
  '只读': 'Read-only',
  '只读用户': 'Read-only user',
  '可选，默认按时间生成': 'Optional, defaults to a timestamp',
  '命中的应用组件': 'Matched apps',
  '响应头': 'Response headers',
  '国家 / 地区': 'Country / Region',
  '国家地区': 'Country',
  '地址段格式不正确，例如 192.168.0.0/16 或 10.0.0.1-10.0.0.255': 'Invalid range, e.g. 192.168.0.0/16 or 10.0.0.1-10.0.0.255',
  '地理位置': 'Location',
  '地理信息': 'Geo',
  '地理库配置已保存': 'Geo settings saved',
  '地理库配置已保存，正在后台下载': 'Saved, downloading in background',
  '城市库': 'City database',
  '城市库下载地址': 'City database URL',
  '域名 / 主机名': 'Domains / Hostnames',
  '填写目标与端口，任务将在后台排队执行': 'Enter targets and ports; the task will be queued',
  '处理中…': 'Working…',
  '复制': 'Copy',
  '外观': 'Appearance',
  '多引擎扫描': 'Multi-engine scanning',
  '失败': 'Failed',
  '字段查询可组合使用 &&、||、! 与括号，默认按 AND 处理空格。': 'Combine fields with &&, ||, ! and parentheses; spaces are treated as AND.',
  '密码': 'Password',
  '密码已更新': 'Password updated',
  '密码已更新，其他设备已退出登录': 'Password updated. Other devices have been signed out.',
  '已中断': 'Interrupted',
  '已加载': 'Loaded',
  '已取消': 'Canceled',
  '已复制': 'Copied',
  '已完成': 'Completed',
  '已就绪': 'Ready',
  '已开始后台更新，下载完成后自动生效': 'Update started; it takes effect once the download finishes',
  '应用指纹': 'Apps',
  '开始扫描': 'Start scan',
  '开放端口': 'Open ports',
  '开放端口分布': 'Open ports',
  '当前不做限制，任意地址段都可以提交扫描': 'No restrictions; any range can be scanned',
  '当前密码': 'Current password',
  '当前登录身份与权限': 'Current identity and permissions',
  '态势总览': 'Overview',
  '总览加载失败': 'Failed to load overview',
  '手动刷新': 'Refresh',
  '打开导航': 'Open navigation',
  '扫描任务': 'Scan tasks',
  '扫描引擎': 'Scanner',
  '扫描目标': 'Targets',
  '扫描网络与限制': 'Scanning network & limits',
  '扫描配置已保存，新任务生效': 'Scan settings saved; applies to new tasks',
  '批量删除失败': 'Bulk delete failed',
  '批量删除资产': 'Bulk delete assets',
  '指纹识别': 'Fingerprinting',
  '按地理归属': 'By location',
  '按网段删除，例如 192.168.6.0/24、10.0.0.0/8': 'Delete by CIDR, e.g. 192.168.6.0/24, 10.0.0.0/8',
  '排队': 'Queued',
  '排队中': 'Queued',
  '控制台': 'Console',
  '操作失败': 'Operation failed',
  '支持 * 匹配任意字符、? 匹配单个字符，例如 192.168.*.*': 'Use * for any characters and ? for one character, e.g. 192.168.*.*',
  '收尾': 'Finalizing',
  '新增': 'New',
  '新增资产趋势': 'New assets trend',
  '新密码': 'New password',
  '新密码至少 8 位': 'New password must be at least 8 characters',
  '新建扫描任务': 'New scan task',
  '新建用户': 'New user',
  '时间': 'Time',
  '暂无可用于筛选的聚合结果': 'No facets available',
  '暂无扫描任务': 'No scan tasks',
  '暂无数据': 'No data',
  '暂无日志': 'No logs',
  '更新': 'Updated',
  '更新中': 'Updating',
  '更新失败': 'Update failed',
  '最后一页': 'Last page',
  '最近发现': 'Recently seen',
  '最近登录': 'Last login',
  '有效期': 'Validity',
  '服务': 'Service',
  '服务分布': 'Services',
  '服务端强制约束': 'Server-side limits',
  '服务识别': 'Service detection',
  '未加载': 'Not loaded',
  '未就绪': 'Not ready',
  '未生成': 'Not generated',
  '未知': 'Unknown',
  '未获取到网关 MAC，可先 ping 一次网关再刷新': 'Gateway MAC not found; ping the gateway once and refresh',
  '本地库已就绪': 'Local database ready',
  '本机可用的探测能力': 'Detection capabilities on this host',
  '来源': 'Source',
  '查询失败': 'Query failed',
  '查询语句': 'Query',
  '查询语法': 'Query syntax',
  '检测中': 'Checking',
  '检索中…': 'Searching…',
  '正在检索资产': 'Searching assets',
  '正在汇总资产数据': 'Aggregating assets',
  '没有匹配的资产': 'No matching assets',
  '深度服务识别': 'Deep service detection',
  '清理中…': 'Clearing…',
  '清空': 'Clear',
  '清空任务失败': 'Failed to clear tasks',
  '清空已结束的任务': 'Clear finished tasks',
  '渲染异常:': 'Render error:',
  '点击右上角「新建任务」开始一次资产测绘': 'Click "New task" to start mapping',
  '热门端口': 'Top ports',
  '独立 IP': 'Unique IPs',
  '用于脚本与外部系统调用，请求头 X-API-Key': 'For scripts and external systems via the X-API-Key header',
  '用户名': 'Username',
  '用户名至少 3 位，密码至少 8 位': 'Username ≥ 3 chars, password ≥ 8 chars',
  '用户管理': 'Users',
  '登录失败': 'Sign-in failed',
  '登录控制台': 'Sign in',
  '确认': 'Confirm',
  '确认删除': 'Delete',
  '确认新密码': 'Confirm new password',
  '确认清空': 'Clear',
  '禁止扫描网段': 'Blocked ranges',
  '端口': 'Port',
  '端口发现': 'Port discovery',
  '端口数': 'Ports',
  '第一页': 'First page',
  '筛选': 'Filters',
  '管理员': 'Administrator',
  '管理员可创建只读或管理账号': 'Administrators can create read-only or admin accounts',
  '系统设置': 'Settings',
  '统一检索语法': 'Unified query syntax',
  '网关 MAC（Masscan）': 'Gateway MAC (Masscan)',
  '网卡': 'Interface',
  '网络空间资产测绘引擎': 'Cyber asset mapping engine',
  '自动探测': 'Auto-detect',
  '自动探测（默认路由）': 'Auto (default route)',
  '自动选择（推荐）': 'Automatic (recommended)',
  '角色': 'Role',
  '识别到的服务': 'Detected service',
  '试试调整查询条件，或先创建一次扫描任务。': 'Adjust the query, or run a scan first.',
  '详情': 'Details',
  '请至少填写一个扫描目标': 'Add at least one target',
  '请输入匹配模式': 'Enter a pattern',
  '账户与密钥': 'Account & keys',
  '账户信息': 'Account',
  '账户安全、扫描引擎与用户管理': 'Account security, scanners and users',
  '资产增长趋势': 'Asset growth',
  '资产总数': 'Total assets',
  '资产搜索': 'Asset search',
  '运行中': 'Running',
  '运行日志': 'Logs',
  '运行限制': 'Limits',
  '还没有扫描任务，先创建一次任务来发现资产。': 'No tasks yet — create one to discover assets.',
  '还没有资产数据，去「扫描任务」发起第一次测绘。': 'No assets yet — start a scan to begin mapping.',
  '通用名': 'Common name',
  '速率上限（pps）': 'Max rate (pps)',
  '速率（pps）': 'Rate (pps)',
  '重置失败': 'Reset failed',
  '错误': 'Error',
  '页面渲染出错': 'Page render error',
  '颁发者': 'Issuer',
  '首次发现': 'First seen',
  '默认扫描端口': 'Default ports',
  '目标': 'Targets',
  '跳过': 'Skip',
  '{n} 分钟前': '{n} min ago',
  '{n} 分钟后': 'in {n} min',
  '{n} 小时前': '{n} h ago',
  '{n} 小时后': 'in {n} h',
  '{n} 天前': '{n} d ago',
  '{n} 天后': 'in {n} d',
  '{n} 秒': '{n}s',
  '{m} 分 {s} 秒': '{m}m {s}s',
  '{h} 时 {m} 分': '{h}h {m}m',

  ' · 网关 {gw}': ' · gateway {gw}',
  ' · 默认': ' · default',
  ' 等 {n} 项': ' and {n} more',
  'app:nginx && country:CN  或  ip:192.168.6.0/24': 'app:nginx && country:CN  or  ip:192.168.6.0/24',
  'useAuth 必须在 AuthProvider 内使用': 'useAuth must be used inside an AuthProvider',
  '不可用': 'Unavailable',
  '亮色': 'Light',
  '以任务化的方式发现、识别并持续追踪网络资产，用一套统一的检索语法完成检索与研判。': 'Discover, fingerprint and track network assets as tasks, then search and triage them with one query syntax.',
  '任务由后台工作池调度，可在「扫描任务」页查看进度与实时日志。': 'Tasks run in a background worker pool; follow progress and live logs on the Scans page.',
  '会删除全部已完成、失败、已取消与已中断的任务记录及其日志；运行中与排队中的任务会保留。此操作不可撤销。': 'Deletes every finished, failed, canceled and interrupted task with its logs. Running and queued tasks are kept. This cannot be undone.',
  '使用探测值': 'Use detected value',
  '保存密码': 'Save password',
  '共 {n} 个 IP': '{n} IPs',
  '共 {n} 个任务': '{n} tasks',
  '内网 {a} · 公网 {b}': 'Private {a} · public {b}',
  '创建': 'Created',
  '创建 {at}': 'Created {at}',
  '删除此端口': 'Delete this port',
  '删除该 IP 全部资产': 'Delete all assets of this IP',
  '刷新': 'Refresh',
  '原样追加到 masscan 命令行；扫描目标、端口、速率与输出格式由系统管理，无需填写。': 'Appended verbatim to the masscan command line. Targets, ports, rate and output format are managed by SCOPEX.',
  '发起人 {user}': 'By {user}',
  '可用': 'Available',
  '启用本地地理库': 'Enable local GeoIP library',
  '国家/地区': 'Country/Region',
  '地理信息库': 'GeoIP library',
  '对开放端口调用 nmap -sV（较慢，但服务与版本更准确）': 'Run nmap -sV against open ports (slower, but more accurate service and version detection)',
  '导出': 'Export',
  '将删除': 'Delete',
  '已删除 {ip} 下的 {n} 条资产': 'Deleted {n} assets under {ip}',
  '已删除 {n} 条匹配 {value} 的资产': 'Deleted {n} assets matching {value}',
  '已删除资产 {asset}': 'Deleted asset {asset}',
  '已刷新 {ip} 的 {n} 个端口': 'Refreshed {n} ports of {ip}',
  '已清空 {n} 个已结束的任务': 'Cleared {n} finished tasks',
  '库过期时自动更新': 'Auto-update when outdated',
  '开始 {at}': 'Started {at}',
  '当前默认路由：': 'Default route:',
  '批量删除': 'Bulk delete',
  '指定探测流量走哪块网卡（Masscan 另需网关 MAC），并维护禁止扫描的地址段': 'Choose the interface for probe traffic (Masscan also needs the gateway MAC) and maintain the blocked ranges.',
  '按通配符或 CIDR 匹配 IP，命中的资产（含全部端口）会被永久删除，不可撤销。': 'Match IPs by wildcard or CIDR; matching assets (all ports) are deleted permanently.',
  '按首次发现时间统计近 {n} 天': 'First seen over the last {n} days',
  '搜索': 'Search',
  '操作不可撤销。': 'This cannot be undone.',
  '支持 IP、CIDR 与 a-b 区间，命中其中任一条的任务会在端口发现前被拒绝；清空即不限制。': 'Supports IP, CIDR and a-b ranges. A task matching any entry is rejected before discovery; leave empty for no restriction.',
  '新 Key 已生成，请立即复制保存；旧 Key 已失效。': 'A new key was generated. Copy it now; the old key is invalid.',
  '新建任务': 'New task',
  '新建任务时端口留空即使用这里的值；支持逗号分隔与区间（如 1-1024）。留空则回退为 80,443,8080,8443。': 'Leave the ports empty when creating a task to use this value (comma separated with ranges such as 1-1024). Empty falls back to 80,443,8080,8443.',
  '新建扫描': 'New scan',
  '暗色': 'Dark',
  '最后更新于 {at}': 'Updated {at}',
  '本地部署 · 数据不出域': 'Self-hosted · data stays in your network',
  '查看全部': 'View all',
  '深度识别': 'Deep scan',
  '添加': 'Add',
  '清空已结束': 'Clear finished',
  '下的全部 {n} 条端口资产记录，操作不可撤销。': ' — all {n} port records will be deleted. This cannot be undone.',
  '的资产记录，该操作不可撤销。': ' record. This cannot be undone.',
  '移除 {item}': 'Remove {item}',
  '立即更新': 'Update now',
  '端口 {ports}': 'Ports {ports}',
  '{n} 端口': '{n} ports',
  '服务/版本识别': 'Service/version detection',
  '大范围端口测绘': 'High-speed wide-range discovery',
  '内置 TCP 扫描（无需 root）': 'Built-in TCP scan (no root required)',
  '需要 root 或 CAP_NET_RAW 权限，当前进程无 raw socket 权限': 'Requires root or CAP_NET_RAW; this process cannot open a raw socket',

  '1 端口': '1 port',
  '第 {n} 页': 'Page {n}',
  '结束 {at}': 'Finished {at}',
  '耗时 {d}': 'Took {d}',
  '自动拉取公开的 DB-IP Lite（mmdb）离线库，扫描时本地解析国家、省市与 ASN，不依赖任何在线查询接口': 'Download the public DB-IP Lite (mmdb) databases and resolve country, region and ASN locally — no online API required.',
  '自动探测：': 'Detected:',
  '语法帮助': 'Query syntax',
  '账户与设置': 'Account & settings',
  '跟随系统': 'System',
  '运行中 {n}': '{n} running',
  '近 24h +{n}': '+{n} in 24h',
  '近 7 天 +{n}': '+{n} in 7d',
  '返回': 'Back',
  '进入控制台': 'Enter console',
  '退出登录': 'Sign out',
  '通配符': 'Wildcard',
  '重新加载': 'Reload',
  '重新生成': 'Regenerate',
  '首次启动可通过环境变量指定管理员密码：': 'Set the admin password on first start with',

  '自动检测': 'Auto',
  '语言': 'Language',
  '切换语言': 'Switch language',
  '请求失败（{status}）': 'Request failed ({status})',
  '令牌无效': 'Invalid token',
  '接口不存在': 'Endpoint not found',
  '参数不完整': 'Missing parameters',
  '不支持的扫描引擎': 'Unsupported scan engine',
  '密码至少 8 位': 'Password must be at least 8 characters',
  '单个任务最多 32 个目标段': 'A task can contain at most 32 targets',
  '角色只能是 admin 或 viewer': 'Role must be admin or viewer',
  '该资产没有图标': 'This asset has no favicon',
  '该 IP 下没有资产': 'No assets under this IP',
  '该 IP 下没有已测绘的资产': 'No mapped assets under this IP',
  '没有可扫描的目标地址': 'No scannable target',
  '禁止扫描网段不能超过 128 条': 'At most 128 blocked ranges',
  '默认端口不能超过 500 字符': 'Default ports cannot exceed 500 characters',
  '两个下载地址都不能为空': 'Both database URLs are required',
  '地理库未启用': 'GeoIP library is disabled',
  '地理库已更新': 'GeoIP library updated',
  '地理库下载失败': 'Failed to download the GeoIP library',
  '地理库更新失败': 'Failed to update the GeoIP library',
  '地理库打开失败': 'Failed to open the GeoIP library',
  '地理库未就绪且未开启自动下载': 'GeoIP library is not ready and auto download is off',
  '聚合失败': 'Aggregation failed',
  '保存地理库设置失败': 'Failed to save GeoIP settings',
  '读取地理库设置失败': 'Failed to read GeoIP settings',
  '读取扫描设置失败': 'Failed to read scan settings',
  '读取用户失败': 'Failed to load users',
  '读取日志失败': 'Failed to load logs',
  '查询任务失败': 'Failed to query tasks',
  '更新任务状态失败': 'Failed to update task status',
  '加密失败': 'Encryption failed',
  '签发令牌失败': 'Failed to issue token',
  '非法的签名算法': 'Unexpected signing method',
  '其他': 'Other',
  'AI 服务识别': 'AI service',
  '产品': 'Product',
  '类型': 'Kind',
  '模型': 'Models',
  '运行环境': 'Runtime',
  '备注': 'Notes',
  '大语言模型推理': 'LLM inference',
  '图像生成': 'Image generation',
  'ML 平台': 'ML platform',
  'AI 基础设施': 'AI infrastructure',
  '向量/时序数据库': 'Vector/time-series DB',
  '容器编排': 'Container orchestration',
  '全球汇总': 'Global insights',
  '全球数据汇总': 'Global insights',
  '汇总加载失败': 'Failed to load insights',
  '正在汇总全球分布': 'Aggregating global distribution',
  '资产': 'Assets',
  '端口级记录': 'Port-level records',
  '按归属地统计': 'By geographic origin',
  'ASN 组织': 'ASN organizations',
  '按自治域统计': 'By autonomous system',
  'AI 资产': 'AI assets',
  '识别出 AI 推理服务': 'Detected AI inference services',
  '大洲分布': 'Continents',
  '按国家码折算大洲': 'Continents derived from country codes',
  '服务类型': 'Service types',
  '按协议服务归类': 'Grouped by protocol service',
  '操作系统': 'Operating systems',
  '从 banner、响应头与服务自报信息推断': 'Inferred from banners, headers and self-reported info',
  '应用品类': 'Application categories',
  '命中的应用指纹归类': 'Grouped by matched app fingerprints',
  '国家地区排行': 'Top countries',
  '按资产数量排序': 'Ranked by asset count',
  '应用排行': 'Top applications',
  'AI 服务分布': 'AI services',
  '主动探测识别出的 AI 推理服务': 'AI inference services identified by active probing',
  '还没有识别到 AI 服务': 'No AI services identified yet',
  '设备类型': 'Device types',
  '从指纹推断的设备角色': 'Device roles inferred from fingerprints',
  '组织与网络': 'Organizations and networks',
  '按 ASN 归属统计': 'Grouped by ASN',
  '证书签发者': 'Certificate issuers',
  'TLS 证书的 CA 分布': 'CA distribution of TLS certificates',
  '暴露面提示': 'Exposure notes',
  '根据当前指纹给出的巡检建议': 'Review suggestions based on current fingerprints',
  'AI 推理服务通常缺少认证，请确认 Ollama、ComfyUI、vLLM 等是否应暴露在公网。': 'AI inference services usually lack authentication — decide whether Ollama, ComfyUI, vLLM and the like really belong on the public internet.',
  '未鉴权的 Docker API、etcd、ClickHouse 可直接导致主机或数据失守，建议优先收敛。': 'Unauthenticated Docker API, etcd or ClickHouse endpoints can hand over a host or its data — close them first.',
  '查看容器类资产': 'View container assets',
  '在资产搜索中查看': 'View in asset search',
  '未知地区': 'Unknown region',
  '亚洲': 'Asia',
  '欧洲': 'Europe',
  '北美洲': 'North America',
  '南美洲': 'South America',
  '非洲': 'Africa',
  '大洋洲': 'Oceania',
  '南极洲': 'Antarctica',
  'Web 服务': 'Web services',
  'Web 应用': 'Web apps',
  'AI 推理': 'AI inference',
  '数据库/缓存': 'Databases / caches',
  '消息队列': 'Message queues',
  '文件共享': 'File sharing',
  '远程管理': 'Remote access',
  '目录服务': 'Directory services',
  '邮件服务': 'Mail services',
  '运维/容器': 'Ops / containers',
  '安全设备': 'Security appliances',
  'IoT/网络设备': 'IoT / network devices',
  '未识别': 'Unidentified',
  '其他系统': 'Other OS',
  '网络设备系统': 'Network devices',
  '摄像头': 'Camera',
  '路由器': 'Router',
  'NAS 存储': 'NAS storage',
  '服务器面板': 'Server panel',
  'AI 节点': 'AI node',
  '指纹脚本输出': 'Fingerprint script output',
  '常见服务端口': 'Common ports',
  'Web / 中间件': 'Web / middleware',
  '远程与内网': 'Remote / internal',
  'AI 与容器': 'AI / containers',
  '全端口（1-65535）': 'All ports (1-65535)',
  '自定义': 'Custom',
  '共 {n} 个端口': '{n} ports',
  '端口格式：80,443,1000-2000': 'Format: 80,443,1000-2000',
  '全端口扫描较慢，按 1000 pps 估算单个目标需数分钟': 'Full port scans are slow: several minutes per target at 1000 pps',
}

function detect(): Lang {
  if (typeof navigator === 'undefined') {
    return 'en'
  }
  const list = navigator.languages?.length ? navigator.languages : [navigator.language]
  for (const item of list) {
    if (item?.toLowerCase().startsWith('zh')) {
      return 'zh'
    }
  }
  return 'en'


}

let currentLang: Lang = detect()

export function getLang(): Lang {
  return currentLang

}

export function translate(text: string, lang: Lang): string {
  if (lang === 'zh') {
    return text
  }
  return en[text] ?? text
}

export function t(text: string, vars?: Record<string, string | number>): string {
  return applyVars(translate(text, currentLang), vars)
}

export function tf(text: string, lang: Lang = currentLang): string {
  return translate(text, lang)
}

export function setCurrentLang(lang: Lang) {
  currentLang = lang
}

export interface I18nValue {
  lang: Lang
  pref: LangPref
  setPref: (next: LangPref) => void
  t: (text: string, vars?: Record<string, string | number>) => string
}

const fallback: I18nValue = {
  lang: currentLang,
  pref: 'auto',
  setPref: () => undefined,
  t: (text, vars) => applyVars(translate(text, currentLang), vars),
}

const I18nContext = createContext<I18nValue>(fallback)

function readPref(): LangPref {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'zh' || stored === 'en' || stored === 'auto') {
      return stored
    }
  } catch {
    // localStorage 不可用时退回自动检测
  }
  return 'auto'
}

function applyVars(text: string, vars?: Record<string, string | number>): string {
  if (!vars) {
    return text
  }
  return text.replace(/\{(\w+)\}/g, (match, key: string) => {
    const value = vars[key]
    return value === undefined ? match : String(value)
  })
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [pref, setPrefState] = useState<LangPref>(readPref)
  const [lang, setLang] = useState<Lang>(() => (pref === 'auto' ? detect() : pref))

  useEffect(() => {
    const next = pref === 'auto' ? detect() : pref
    setLang(next)
    currentLang = next
    document.documentElement.lang = next === 'zh' ? 'zh-CN' : 'en'
    if (pref === 'auto') {
      return
    }
    try {
      localStorage.setItem(STORAGE_KEY, pref)
    } catch {
      // 忽略持久化失败
    }
  }, [pref])

  useEffect(() => {
    if (pref !== 'auto' || typeof window === 'undefined') {
      return
    }
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    void media
    const onChange = () => {
      const next = detect()
      currentLang = next
      setLang(next)
    }
    window.addEventListener('languagechange', onChange)
    return () => window.removeEventListener('languagechange', onChange)
  }, [pref])

  const setPref = useCallback((next: LangPref) => setPrefState(next), [])

  const value = useMemo<I18nValue>(
    () => ({
      lang,
      pref,
      setPref,
      t: (text, vars) => applyVars(translate(text, lang), vars),
    }),
    [lang, pref, setPref],
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nValue {
  return useContext(I18nContext)
}

/** 语言偏好当前落库值，供请求头使用。 */
export function langHeader(): string {
  return currentLang === 'zh' ? 'zh-CN,zh;q=0.9,en;q=0.5' : 'en-US,en;q=0.9,zh;q=0.5'
}
