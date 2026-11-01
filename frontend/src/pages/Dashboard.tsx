import { Activity, Globe2, Layers, Radar, RefreshCw, Server, TrendingUp } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

import { Empty, Loading, PageHeader, StatusPill, flagOf, formatTime } from '@/components/shared'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { api } from '@/lib/api'
import type { Facet, Overview } from '@/lib/types'
import { t } from '@/lib/i18n'

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

function TrendChart({ data }: { data: { date: string; count: number }[] }) {
  const W = 760
  const H = 170
  const P = 14
  const max = Math.max(1, ...data.map((d) => d.count))
  const step = data.length > 1 ? (W - P * 2) / (data.length - 1) : 0
  const xOf = (i: number) => P + i * step
  const yOf = (v: number) => H - P - (v / max) * (H - P * 2)
  const line = data.map((d, i) => `${i === 0 ? 'M' : 'L'}${xOf(i).toFixed(1)},${yOf(d.count).toFixed(1)}`).join(' ')
  const area = `${line} L${xOf(data.length - 1).toFixed(1)},${H - P} L${xOf(0).toFixed(1)},${H - P} Z`
  const peak = data.reduce((best, d, i) => (d.count > data[best].count ? i : best), 0)

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="h-44 w-full" preserveAspectRatio="none" role="img" aria-label={t("新增资产趋势")}>
      <defs>
        <linearGradient id="trendArea" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--primary)" stopOpacity="0.28" />
          <stop offset="100%" stopColor="var(--primary)" stopOpacity="0" />
        </linearGradient>
      </defs>
      {[0.25, 0.5, 0.75].map((r) => (
        <line key={r} x1={P} x2={W - P} y1={P + r * (H - P * 2)} y2={P + r * (H - P * 2)} stroke="var(--border)" strokeDasharray="3 5" />
      ))}
      <path d={area} fill="url(#trendArea)" />
      <path d={line} fill="none" stroke="var(--primary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      {data.length > 0 && (
        <>
          <circle cx={xOf(peak)} cy={yOf(data[peak].count)} r="3.5" fill="var(--primary)" />
          <text x={Math.min(Math.max(xOf(peak), 30), W - 30)} y={Math.max(yOf(data[peak].count) - 10, 12)} textAnchor="middle" className="fill-muted-foreground text-[10px]">
            {data[peak].count}
          </text>
        </>
      )}
    </svg>
  )
}

function TopList({
  title,
  description,
  items,
  to,
  flag,
}: {
  title: string
  description: string
  items: Facet[]
  to: string
  flag?: boolean
}) {
  const max = Math.max(1, ...items.map((i) => i.count))
  return (
    <Card className="gap-0">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="text-sm">{title}</CardTitle>
          <Link to={to} className="text-[11px] text-muted-foreground transition hover:text-primary">
            {t('查看全部')}
          </Link>
        </div>
        <CardDescription className="text-[11px]">{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2.5">
        {items.length === 0 && <p className="py-4 text-center text-xs text-muted-foreground">{t("暂无数据")}</p>}
        {items.map((item) => (
          <div key={item.value} className="space-y-1">
            <div className="flex items-baseline justify-between gap-2 text-xs">
              <span className="truncate font-mono">
                {flag && flagOf(item.value) && <span className="mr-1 font-sans">{flagOf(item.value)}</span>}
                {flag && item.value === 'LAN' ? t('内网') : item.value}
              </span>
              <span className="tabular-nums text-muted-foreground">{item.count}</span>
            </div>
            <div className="h-1 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary/70" style={{ width: `${(item.count / max) * 100}%` }} />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export default function Dashboard() {
  const [data, setData] = useState<Overview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setBusy(true)
    try {
      setData((await api.overview()).data)
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

  const tasks = useMemo(() => Object.entries(data?.taskCounts ?? {}), [data])

  if (!data) {
    return error ? <Empty title={t("总览加载失败")} hint={error} /> : <Loading label={t("正在汇总资产数据")} />
  }

  return (
    <div className="space-y-5">
      <PageHeader title={t("态势总览")} description={t('最后更新于 {at}', { at: formatTime(data.generatedAt) })}>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={busy}>
          <RefreshCw className={busy ? 'animate-spin' : undefined} />
          {t('刷新')}
        </Button>
        <Button size="sm" asChild>
          <Link to="/tasks">
            <Radar />
            {t('新建扫描')}
          </Link>
        </Button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <StatCard label={t("资产总数")} value={data.totalAssets} hint={t('近 24h +{n}', { n: data.new24h })} icon={<Layers className="size-4" />} />
        <StatCard label={t("独立 IP")} value={data.uniqueIPs} hint={t('内网 {a} · 公网 {b}', { a: data.privateHosts, b: data.publicHosts })} icon={<Globe2 className="size-4" />} />
        <StatCard label={t("24h 新增")} value={data.new24h} hint={t('近 7 天 +{n}', { n: data.new7d })} icon={<TrendingUp className="size-4" />} />
        <StatCard label={t("扫描任务")} value={tasks.reduce((sum, [, n]) => sum + n, 0)} hint={t('运行中 {n}', { n: data.taskCounts.running ?? 0 })} icon={<Server className="size-4" />} />
      </div>

      <Card className="gap-0">
        <CardHeader>
          <div className="flex items-center justify-between gap-2">
            <div className="space-y-1">
              <CardTitle className="text-sm">{t("资产增长趋势")}</CardTitle>
              <CardDescription className="text-[11px]">{t('按首次发现时间统计近 {n} 天', { n: data.trend.length })}</CardDescription>
            </div>
            <Activity className="size-4 text-muted-foreground" />
          </div>
        </CardHeader>
        <CardContent className="pt-3">
          <TrendChart data={data.trend} />
          <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
            <span>{data.trend[0]?.date}</span>
            <span>{data.trend[data.trend.length - 1]?.date}</span>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <TopList title={t("热门端口")} description={t("开放端口分布")} items={data.topPorts} to="/search" />
        <TopList title={t("服务分布")} description={t("识别到的服务")} items={data.topServices} to="/search?q=service:*" />
        <TopList title={t("国家地区")} description={t("按地理归属")} items={data.topCountries} to="/search" flag />
        <TopList title={t("应用指纹")} description={t("命中的应用组件")} items={data.topApps} to="/search" />
      </div>

      <Card className="gap-0">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm">{t("任务状态分布")}</CardTitle>
        </CardHeader>
        <CardContent>
          {tasks.length === 0 ? (
            <p className="py-3 text-xs text-muted-foreground">{t("还没有扫描任务，先创建一次任务来发现资产。")}</p>
          ) : (
            <>
              <div className="flex flex-wrap gap-2">
                {tasks.map(([status, count]) => (
                  <div key={status} className="flex items-center gap-2 rounded-md border border-border px-2.5 py-1.5">
                    <StatusPill status={status} />
                    <span className="text-sm font-medium tabular-nums">{count}</span>
                  </div>
                ))}
              </div>
              <Separator className="my-4" />
              <p className="text-[11px] text-muted-foreground">
                {t('任务由后台工作池调度，可在「扫描任务」页查看进度与实时日志。')}
              </p>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
