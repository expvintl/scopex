import { Bot, Building2, Globe2, Layers, PieChart, RefreshCw, Server, ShieldCheck } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

import { Empty, Loading, PageHeader, flagOf, formatTime } from '@/components/shared'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { api } from '@/lib/api'
import { t } from '@/lib/i18n'
import type { InsightSlice, Insights } from '@/lib/types'

// 环形图配色：挑中间明度的色相，深色/浅色主题下都能分辨。
const PALETTE = ['#0ea5e9', '#22c55e', '#f59e0b', '#a855f7', '#ef4444', '#14b8a6', '#6366f1', '#eab308', '#ec4899', '#84cc16', '#06b6d4', '#f97316']

const CONTINENT_LABEL: Record<string, string> = {
  asia: '亚洲',
  europe: '欧洲',
  north_america: '北美洲',
  south_america: '南美洲',
  africa: '非洲',
  oceania: '大洋洲',
  antarctica: '南极洲',
  unknown: '未知地区',
}

const CATEGORY_LABEL: Record<string, string> = {
  web: 'Web 服务',
  app: 'Web 应用',
  ai: 'AI 推理',
  db: '数据库/缓存',
  queue: '消息队列',
  storage: '文件共享',
  remote: '远程管理',
  directory: '目录服务',
  mail: '邮件服务',
  ops: '运维/容器',
  security: '安全设备',
  iot: 'IoT/网络设备',
  other: '未识别',
}

const OS_LABEL: Record<string, string> = {
  windows: 'Windows',
  linux: 'Linux',
  unix: 'Unix/BSD',
  macos: 'macOS',
  android: 'Android',
  network: '网络设备系统',
  other: '其他系统',
}

const DEVICE_LABEL: Record<string, string> = {
  camera: '摄像头',
  router: '路由器',
  nas: 'NAS 存储',
  'server-panel': '服务器面板',
  'ai-server': 'AI 节点',
}

function labelOf(map: Record<string, string>, key: string): string {
  return t(map[key] ?? key)
}

/** Donut 是纯 SVG 环形图：不引入图表库，避免为一页统计增加依赖体积。 */
function Donut({
  title,
  description,
  items,
  nameOf,
}: {
  title: string
  description: string
  items: InsightSlice[]
  nameOf: (s: InsightSlice) => string
}) {
  // 超过 6 类时并成「其他」，否则环形图会碎成看不出比例的细条。
  const { data, sum } = useMemo(() => {
    const head = items.slice(0, 6)
    const rest = items.slice(6).reduce((n, s) => n + s.count, 0)
    const merged = rest > 0 ? [...head, { key: '__rest', label: '其他', count: rest }] : head
    return { data: merged, sum: merged.reduce((n, s) => n + s.count, 0) }
  }, [items])

  const R = 60
  const C = 2 * Math.PI * R
  let acc = 0
  const arcs = data.map((s, i) => {
    const len = sum > 0 ? (s.count / sum) * C : 0
    const arc = { key: s.key, len, offset: acc, color: PALETTE[i % PALETTE.length], item: s }
    acc += len
    return arc
  })

  return (
    <Card className="gap-0">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
        <CardDescription className="text-[11px]">{description}</CardDescription>
      </CardHeader>
      <CardContent>
        {sum === 0 ? (
          <p className="py-10 text-center text-xs text-muted-foreground">{t('暂无数据')}</p>
        ) : (
          <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-start">
            <div className="relative shrink-0">
              <svg viewBox="0 0 160 160" className="size-36" role="img" aria-label={title}>
                <circle cx="80" cy="80" r={R} fill="none" stroke="var(--muted)" strokeWidth="16" />
                {arcs.map((arc) => (
                  <circle
                    key={arc.key}
                    cx="80"
                    cy="80"
                    r={R}
                    fill="none"
                    stroke={arc.color}
                    strokeWidth="16"
                    strokeDasharray={`${Math.max(arc.len - 1.2, 0.4)} ${C}`}
                    strokeDashoffset={-arc.offset}
                    transform="rotate(-90 80 80)"
                  />
                ))}
              </svg>
              <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                <span className="text-lg font-semibold tabular-nums">{sum.toLocaleString()}</span>
                <span className="text-[10px] text-muted-foreground">{t('资产')}</span>
              </div>
            </div>
            <ul className="w-full space-y-1.5">
              {arcs.map((arc) => (
                <li key={arc.key} className="flex items-center gap-2 text-xs">
                  <span className="size-2 shrink-0 rounded-full" style={{ background: arc.color }} />
                  <span className="min-w-0 flex-1 truncate">
                    {arc.item.key === '__rest' ? t('其他') : nameOf(arc.item)}
                  </span>
                  <span className="tabular-nums text-muted-foreground">{arc.item.count.toLocaleString()}</span>
                  <span className="w-10 text-right text-[11px] tabular-nums text-muted-foreground">
                    {((arc.item.count / sum) * 100).toFixed(1)}%
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

/** BarList 用于排行：保留条形对比，同时给出可跳转的检索入口。 */
function BarList({
  title,
  description,
  items,
  nameOf,
  href,
  flag,
  empty = '暂无数据',
}: {
  title: string
  description: string
  items: InsightSlice[]
  nameOf: (s: InsightSlice) => string
  href?: (s: InsightSlice) => string
  flag?: boolean
  empty?: string
}) {
  const max = Math.max(1, ...items.map((i) => i.count))
  return (
    <Card className="gap-0">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
        <CardDescription className="text-[11px]">{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2.5">
        {items.length === 0 && <p className="py-4 text-center text-xs text-muted-foreground">{t(empty)}</p>}
        {items.map((item) => {
          const to = href?.(item)
          return (
            <div key={item.key} className="space-y-1">
              <div className="flex items-baseline justify-between gap-2 text-xs">
                {to ? (
                  <Link to={to} className="truncate font-mono hover:text-primary hover:underline" title={t('在资产搜索中查看')}>
                    {flag && flagOf(item.key) && <span className="mr-1 font-sans">{flagOf(item.key)}</span>}
                    {nameOf(item)}
                  </Link>
                ) : (
                  <span className="truncate font-mono">
                    {flag && flagOf(item.key) && <span className="mr-1 font-sans">{flagOf(item.key)}</span>}
                    {nameOf(item)}
                  </span>
                )}
                <span className="tabular-nums text-muted-foreground">{item.count.toLocaleString()}</span>
              </div>
              <div className="h-1 overflow-hidden rounded-full bg-muted">
                <div className="h-full rounded-full bg-primary/70" style={{ width: `${(item.count / max) * 100}%` }} />
              </div>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}

function StatCard({ label, value, hint, icon }: { label: string; value: number; hint: string; icon: React.ReactNode }) {
  return (
    <Card className="gap-0 py-4">
      <CardContent className="flex items-start justify-between px-4">
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-2xl font-semibold tabular-nums tracking-tight">{value.toLocaleString()}</p>
          <p className="text-[11px] text-muted-foreground">{hint}</p>
        </div>
        <div className="flex size-8 items-center justify-center rounded-md bg-primary/10 text-primary">{icon}</div>
      </CardContent>
    </Card>
  )
}

export default function InsightsPage() {
  const [data, setData] = useState<Insights | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setBusy(true)
    try {
      setData((await api.insights()).data)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('加载失败'))
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  if (!data) {
    return error ? <Empty title={t('汇总加载失败')} hint={error} /> : <Loading label={t('正在汇总全球分布')} />
  }

  const countryName = (s: InsightSlice) => s.key || s.label || '—'
  const asnName = (s: InsightSlice) => s.label || s.key

  return (
    <div className="space-y-5">
      <PageHeader title={t('全球数据汇总')} description={t('最后更新于 {at}', { at: formatTime(data.generatedAt) })}>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={busy}>
          <RefreshCw className={busy ? 'animate-spin' : undefined} />
          {t('刷新')}
        </Button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <StatCard label={t('资产总数')} value={data.total} hint={t('端口级记录')} icon={<Layers className="size-4" />} />
        <StatCard label={t('独立 IP')} value={data.uniqueIPs} hint={t('内网 {a} · 公网 {b}', { a: data.private, b: data.public })} icon={<Server className="size-4" />} />
        <StatCard label={t('国家/地区')} value={data.countryNum} hint={t('按归属地统计')} icon={<Globe2 className="size-4" />} />
        <StatCard label={t('ASN 组织')} value={data.asnNum} hint={t('按自治域统计')} icon={<Building2 className="size-4" />} />
        <StatCard label={t('AI 资产')} value={data.aiAssets} hint={t('识别出 AI 推理服务')} icon={<Bot className="size-4" />} />
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <Donut title={t('大洲分布')} description={t('按国家码折算大洲')} items={data.continents} nameOf={(s) => labelOf(CONTINENT_LABEL, s.key)} />
        <Donut title={t('服务类型')} description={t('按协议服务归类')} items={data.services} nameOf={(s) => labelOf(CATEGORY_LABEL, s.key)} />
        <Donut title={t('操作系统')} description={t('从 banner、响应头与服务自报信息推断')} items={data.osFamilies} nameOf={(s) => labelOf(OS_LABEL, s.key)} />
        <Donut title={t('应用品类')} description={t('命中的应用指纹归类')} items={data.appCategories} nameOf={(s) => labelOf(CATEGORY_LABEL, s.key)} />
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        <BarList
          title={t('国家地区排行')}
          description={t('按资产数量排序')}
          items={data.countries.slice(0, 10)}
          nameOf={(s) => (s.key === 'LAN' ? t('内网') : countryName(s))}
          href={(s) => `/search?q=${encodeURIComponent(`country:${s.key}`)}`}
          flag
        />
        <BarList
          title={t('热门端口')}
          description={t('开放端口分布')}
          items={data.ports.slice(0, 10)}
          nameOf={(s) => s.label || s.key}
          href={(s) => `/search?q=${encodeURIComponent(`port:${s.key}`)}`}
        />
        <BarList
          title={t('应用排行')}
          description={t('命中的应用组件')}
          items={data.apps.slice(0, 10)}
          nameOf={(s) => s.label || s.key}
          href={(s) => `/search?q=${encodeURIComponent(`app:${s.key}`)}`}
        />
        <BarList
          title={t('AI 服务分布')}
          description={t('主动探测识别出的 AI 推理服务')}
          items={data.aiApps.slice(0, 10)}
          nameOf={(s) => s.label || s.key}
          href={(s) => `/search?q=${encodeURIComponent(`app:${s.key}`)}`}
          empty="还没有识别到 AI 服务"
        />
        <BarList
          title={t('设备类型')}
          description={t('从指纹推断的设备角色')}
          items={data.devices.slice(0, 10)}
          nameOf={(s) => labelOf(DEVICE_LABEL, s.key)}
          href={(s) => `/search?q=${encodeURIComponent(`device:${s.key}`)}`}
        />
        <BarList
          title={t('组织与网络')}
          description={t('按 ASN 归属统计')}
          items={data.asns.slice(0, 10)}
          nameOf={asnName}
        />
        <BarList
          title={t('证书签发者')}
          description={t('TLS 证书的 CA 分布')}
          items={data.tlsIssuers.slice(0, 10)}
          nameOf={(s) => s.label || s.key}
        />
        <Card className="gap-0">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-sm">
              <ShieldCheck className="size-4 text-primary" />
              {t('暴露面提示')}
            </CardTitle>
            <CardDescription className="text-[11px]">{t('根据当前指纹给出的巡检建议')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-xs text-muted-foreground">
            <p>{t('AI 推理服务通常缺少认证，请确认 Ollama、ComfyUI、vLLM 等是否应暴露在公网。')}</p>
            <p>{t('未鉴权的 Docker API、etcd、ClickHouse 可直接导致主机或数据失守，建议优先收敛。')}</p>
            <Link to="/search?q=service:docker" className="inline-flex items-center gap-1 text-primary hover:underline">
              <PieChart className="size-3.5" />
              {t('查看容器类资产')}
            </Link>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
