import {
  Download,
  Filter,
  Globe2,
  HelpCircle,
  RefreshCw,
  Search as SearchIcon,
  ShieldAlert,
  Trash2,
  X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { useSearchParams } from 'react-router-dom'

import { CopyButton, Empty, Loading, PageHeader, Pager, Spinner, countryLabel, flagOf, timeAgo } from '@/components/shared'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { Facet, Facets, FieldDoc, Host, IPGroup, PortBrief } from '@/lib/types'
import { t } from '@/lib/i18n'

const PAGE_SIZE = 20

const SORTS = [
  { key: 'last_seen', label: '最近发现' },
  { key: 'first_seen', label: '首次发现' },
  { key: 'port_count', label: '端口数' },
]

export default function Search() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const page = Math.max(1, Number(params.get('page') ?? 1))
  const sort = params.get('sort') ?? 'last_seen'

  const [input, setInput] = useState(q)
  const [items, setItems] = useState<IPGroup[]>([])
  const [total, setTotal] = useState(0)
  const [facets, setFacets] = useState<Facets | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [showSyntax, setShowSyntax] = useState(false)
  const [showFacets, setShowFacets] = useState(false)
  const [docs, setDocs] = useState<FieldDoc[]>([])
  const [bulkOpen, setBulkOpen] = useState(false)
  const [bulkMode, setBulkMode] = useState<'pattern' | 'cidr'>('pattern')
  const [bulkValue, setBulkValue] = useState('')
  const [bulkError, setBulkError] = useState('')
  const [bulkLoading, setBulkLoading] = useState(false)
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  useEffect(() => {
    setInput(q)
  }, [q])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [pageData, facetData] = await Promise.all([
        api.ipGroups({ q, page, size: PAGE_SIZE, sort }),
        api.facets(q),
      ])
      setItems(pageData.data ?? [])
      setTotal(pageData.total ?? 0)
      setFacets(facetData.data)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('查询失败'))
    } finally {
      setLoading(false)
    }
  }, [q, page, sort])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (!showSyntax || docs.length) return
    void api.querySyntax().then((res) => setDocs(res.data)).catch(() => setDocs([]))
  }, [showSyntax, docs.length])

  const update = (next: { q?: string; page?: number; sort?: string }) => {
    const merged = new URLSearchParams(params)
    if (next.q !== undefined) {
      if (next.q) merged.set('q', next.q)
      else merged.delete('q')
      merged.delete('page')
    }
    if (next.page !== undefined) {
      if (next.page > 1) merged.set('page', String(next.page))
      else merged.delete('page')
    }
    if (next.sort !== undefined) merged.set('sort', next.sort)
    setParams(merged, { replace: true })
  }

  const submit = (event: FormEvent) => {
    event.preventDefault()
    update({ q: input.trim() })
  }

  const addFilter = (expr: string) => {
    const next = q ? `${q} && ${expr}` : expr
    update({ q: next })
    setShowFacets(false)
  }

  const runBulk = async () => {
    const value = bulkValue.trim()
    if (!value) {
      setBulkError(t('请输入匹配模式'))
      return
    }
    setBulkLoading(true)
    setBulkError('')
    try {
      const res = await api.deleteHostsBulk(bulkMode === 'cidr' ? { cidr: value } : { pattern: value })
      setBulkOpen(false)
      setBulkValue('')
      setNotice(t('已删除 {n} 条匹配 {value} 的资产', { n: res.data.deleted, value: res.data.value }))
      await load()
    } catch (err) {
      setBulkError(err instanceof Error ? err.message : t('批量删除失败'))
    } finally {
      setBulkLoading(false)
    }
  }

  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className="space-y-4">
      <PageHeader title={t("资产搜索")} description={t("使用统一检索语法定位已测绘的资产")}>
        <Button variant="outline" size="sm" onClick={() => setShowSyntax(true)}>
          <HelpCircle />
          {t('语法帮助')}
        </Button>
        {isAdmin && (
          <Button variant="outline" size="sm" onClick={() => setBulkOpen(true)}>
            <Trash2 />
            {t('批量删除')}
          </Button>
        )}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" disabled={total === 0}>
              <Download />
              {t('导出')}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem asChild>
              <a href={api.exportUrl(q, 'csv')} download>
                CSV（Excel 兼容）
              </a>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <a href={api.exportUrl(q, 'json')} download>
                JSON
              </a>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </PageHeader>

      <form onSubmit={submit} className="flex gap-2">
        <div className="relative flex-1">
          <SearchIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t('app:nginx && country:CN  或  ip:192.168.6.0/24')}
            className="h-10 pl-9 font-mono text-sm"
            aria-label={t("查询语句")}
          />
          {input && (
            <button
              type="button"
              onClick={() => {
                setInput('')
                update({ q: '' })
              }}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-muted-foreground hover:bg-accent"
              aria-label={t("清空")}
            >
              <X className="size-3.5" />
            </button>
          )}
        </div>
        <Button type="submit" size="lg" className="h-10">
          {t('搜索')}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="size-10 lg:hidden"
          onClick={() => setShowFacets((v) => !v)}
          aria-label={t("筛选")}
        >
          <Filter />
        </Button>
      </form>

      {notice && (
        <div className="flex items-center justify-between gap-3 rounded-md border border-ok/30 bg-ok/10 px-3 py-2 text-xs text-ok">
          <span>{notice}</span>
          <button onClick={() => setNotice('')} aria-label={t("关闭提示")}>
            <X className="size-3.5" />
          </button>
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
        <aside className={cn('space-y-3 lg:block', showFacets ? 'block' : 'hidden')}>
          {facets && (
            <>
              <FacetGroup title={t("端口")} items={facets.ports} prefix="port" onPick={addFilter} />
              <FacetGroup title={t("服务")} items={facets.services} prefix="service" onPick={addFilter} />
              <FacetGroup title={t("国家 / 地区")} items={facets.countries} prefix="country" onPick={addFilter} flag />
              <FacetGroup title={t("应用指纹")} items={facets.apps} prefix="app" onPick={addFilter} />
            </>
          )}
          {!facets?.ports?.length && !facets?.apps?.length && (
            <Card className="py-4">
              <CardContent className="px-4 text-xs text-muted-foreground">{t("暂无可用于筛选的聚合结果")}</CardContent>
            </Card>
          )}
        </aside>

        <section className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-muted-foreground">
              {loading ? t('检索中…') : t('共 {n} 个 IP', { n: total.toLocaleString() })}
              {q && <span className="ml-2 font-mono text-[11px] text-muted-foreground/80">{q}</span>}
            </p>
            <div className="flex flex-wrap gap-1">
              {SORTS.map((item) => (
                <Button
                  key={item.key}
                  variant={sort === item.key ? 'secondary' : 'ghost'}
                  size="sm"
                  onClick={() => update({ sort: item.key })}
                >
                  {t(item.label)}
                </Button>
              ))}
            </div>
          </div>

          <Card className="gap-0 overflow-hidden py-0">
            {loading && <Loading label={t("正在检索资产")} />}

            {!loading && error && <Empty title={t("查询失败")} hint={error} icon={<ShieldAlert className="size-5" />} />}

            {!loading && !error && items.length === 0 && (
              <Empty
                title={t("没有匹配的资产")}
                hint={q ? t('试试调整查询条件，或先创建一次扫描任务。') : t('还没有资产数据，去「扫描任务」发起第一次测绘。')}
                icon={<SearchIcon className="size-5" />}
              />
            )}

            {!loading && !error && items.length > 0 && (
              <div className="divide-y divide-border">
                {items.map((group) => (
                  <IPRow key={group.ip} group={group} onClick={() => setSelected(group.ip)} />
                ))}
              </div>
            )}
          </Card>

          <Pager page={page} pages={pages} onChange={(next) => update({ page: next })} />
        </section>
      </div>

      {selected && (
        <IPSheet
          ip={selected}
          onClose={() => setSelected(null)}
          onChanged={(message) => {
            setNotice(message)
            void load()
          }}
        />
      )}

      <Dialog open={bulkOpen} onOpenChange={setBulkOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("批量删除资产")}</DialogTitle>
            <DialogDescription>
              {t('按通配符或 CIDR 匹配 IP，命中的资产（含全部端口）会被永久删除，不可撤销。')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="flex gap-2">
              <Button
                type="button"
                variant={bulkMode === 'pattern' ? 'secondary' : 'outline'}
                size="sm"
                onClick={() => setBulkMode('pattern')}
              >
                {t('通配符')}
              </Button>
              <Button
                type="button"
                variant={bulkMode === 'cidr' ? 'secondary' : 'outline'}
                size="sm"
                onClick={() => setBulkMode('cidr')}
              >
                CIDR
              </Button>
            </div>
            <Input
              value={bulkValue}
              onChange={(e) => setBulkValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  void runBulk()
                }
              }}
              placeholder={bulkMode === 'cidr' ? '192.168.6.0/24' : '192.168.*.*'}
              className="font-mono text-xs"
            />
            <p className="text-[11px] text-muted-foreground">
              {bulkMode === 'cidr'
                ? t('按网段删除，例如 192.168.6.0/24、10.0.0.0/8')
                : t('支持 * 匹配任意字符、? 匹配单个字符，例如 192.168.*.*')}
            </p>
            {bulkError && <p className="text-xs text-destructive">{bulkError}</p>}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBulkOpen(false)} disabled={bulkLoading}>
              {t('取消')}
            </Button>
            <Button variant="destructive" onClick={runBulk} disabled={bulkLoading}>
              {bulkLoading ? t('删除中…') : t('确认删除')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showSyntax} onOpenChange={setShowSyntax}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("查询语法")}</DialogTitle>
            <DialogDescription>{t('字段查询可组合使用 &&、||、! 与括号，默认按 AND 处理空格。')}</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <div className="divide-y divide-border rounded-md border border-border">
              {docs.map((doc) => (
                <div key={doc.name} className="grid gap-1 px-3 py-2 sm:grid-cols-[130px_minmax(0,1fr)] sm:gap-3">
                  <code className="font-mono text-xs text-primary">{doc.name}</code>
                  <div className="space-y-0.5">
                    <p className="text-xs text-foreground/90">{doc.desc}</p>
                    <p className="font-mono text-[11px] text-muted-foreground">{doc.example}</p>
                  </div>
                </div>
              ))}
              {!docs.length && <p className="px-3 py-6 text-center text-xs text-muted-foreground">{t("加载中…")}</p>}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function PortChip({ p }: { p: PortBrief }) {
  const label = p.service || p.protocol || 'tcp'
  return (
    <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] leading-4">
      <span className="font-medium text-foreground">{p.port}</span>
      <span className="text-muted-foreground">{label}</span>
    </span>
  )
}

function IPRow({ group, onClick }: { group: IPGroup; onClick: () => void }) {
  const ports = [...(group.ports ?? [])].sort((a, b) => a.port - b.port)
  const shown = ports.slice(0, 8)
  const rest = (group.portCount ?? ports.length) - shown.length
  const names = [...(group.hostnames ?? []), ...(group.domains ?? [])]

  return (
    <button
      onClick={onClick}
      className="grid w-full grid-cols-1 items-start gap-2 px-4 py-3 text-left transition hover:bg-muted/50 md:grid-cols-[minmax(150px,0.8fr)_minmax(0,2.8fr)_minmax(84px,0.45fr)_78px] md:items-center md:gap-3"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-1.5">
          <span className="font-mono text-sm font-medium">{group.ip}</span>
          <Badge variant="secondary" className="font-mono">
            {group.portCount === 1 ? t('1 端口') : t('{n} 端口', { n: group.portCount })}
          </Badge>
        </div>
        {names.length > 0 && (
          <p className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">{names.slice(0, 2).join(' · ')}</p>
        )}
      </div>

      <div className="flex min-w-0 flex-wrap items-center gap-1">
        {shown.map((p) => (
          <PortChip key={p.port} p={p} />
        ))}
        {rest > 0 && (
          <span className="rounded-md border border-dashed border-border px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
            +{rest}
          </span>
        )}
      </div>

      <div className="flex items-center justify-between gap-2 md:contents">
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          {group.geo?.private ? (
            <Badge variant="secondary">{t("内网")}</Badge>
          ) : (
            <span className="truncate">
              {flagOf(group.geo?.countryCode) && <span className="mr-1">{flagOf(group.geo?.countryCode)}</span>}
              {countryLabel(group.geo)}
            </span>
          )}
        </div>
        <div className="text-[11px] text-muted-foreground md:text-right">{timeAgo(group.lastSeen)}</div>
      </div>
    </button>
  )
}

function FacetGroup({
  title,
  items,
  prefix,
  onPick,
  flag,
}: {
  title: string
  items: Facet[]
  prefix: string
  onPick: (expr: string) => void
  flag?: boolean
}) {
  if (!items?.length) return null
  const sorted = [...items].sort((a, b) => {
    const na = Number(a.value)
    const nb = Number(b.value)
    if (!Number.isNaN(na) && !Number.isNaN(nb)) return na - nb
    return b.count - a.count
  })
  return (
    <Card className="gap-0 py-3">
      <CardHeader className="px-3 pb-2">
        <CardTitle className="text-xs font-medium text-muted-foreground">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-0.5 px-2">
        {sorted.map((item) => {
          const label = prefix === 'country' && item.value === 'LAN' ? t('内网') : item.value
          return (
          <button
            key={item.value}
            onClick={() => onPick(`${prefix}:"${label}"`)}
            className="flex w-full items-center justify-between gap-2 rounded px-1.5 py-1 text-left text-xs transition hover:bg-accent"
          >
            <span className="truncate font-mono">
              {flag && flagOf(item.value) && <span className="mr-1 font-sans">{flagOf(item.value)}</span>}
              {label}
            </span>
            <span className="tabular-nums text-muted-foreground">{item.count}</span>
          </button>
          )
        })}
      </CardContent>
    </Card>
  )
}

function sourceLabel(source?: string): string {
  if (!source) return '—'
  if (source === 'refresh') return t('手动刷新')
  if (source.startsWith('task:')) return t('扫描任务')
  return source
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="space-y-2 py-3">
      <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

function Field({ label, mono, children }: { label: string; mono?: boolean; children: ReactNode }) {
  return (
    <div className="flex gap-3 text-xs leading-relaxed">
      <span className="w-20 shrink-0 text-muted-foreground">{label}</span>
      <span className={cn('min-w-0 break-all', mono && 'font-mono')}>{children || '—'}</span>
    </div>
  )
}

function IPSheet({ ip, onClose, onChanged }: { ip: string; onClose: () => void; onChanged: (message: string) => void }) {
  const { user } = useAuth()
  const [hosts, setHosts] = useState<Host[] | null>(null)
  const [activeId, setActiveId] = useState<string>('')
  const [error, setError] = useState('')
  const [confirm, setConfirm] = useState<'one' | 'ip' | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState('')
  const [reload, setReload] = useState(0)

  useEffect(() => {
    let alive = true
    setHosts(null)
    setError('')
    setActiveId('')
    api
      .ipHosts(ip)
      .then((res) => {
        if (!alive) return
        setHosts(res.data)
        setActiveId(res.data[0]?.id ?? '')
      })
      .catch((err) => alive && setError(err instanceof Error ? err.message : t('加载失败')))
    return () => {
      alive = false
    }
  }, [ip, reload])

  const host = hosts?.find((item) => item.id === activeId) ?? null
  const isAdmin = user?.role === 'admin'

  const [refreshing, setRefreshing] = useState(false)

  const runRefresh = async () => {
    setRefreshing(true)
    setActionError('')
    try {
      const res = await api.refreshIP(ip)
      onChanged(t('已刷新 {ip} 的 {n} 个端口', { ip, n: res.data.refreshed }))
      setReload((n) => n + 1)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t('刷新失败'))
    } finally {
      setRefreshing(false)
    }
  }

  const runDelete = async () => {
    setBusy(true)
    setActionError('')
    try {
      if (confirm === 'one' && host) {
        await api.deleteHost(host.id)
        const rest = (hosts ?? []).filter((item) => item.id !== host.id)
        onChanged(t('已删除资产 {asset}', { asset: `${ip}:${host.port}` }))
        if (rest.length === 0) {
          onClose()
        } else {
          setHosts(rest)
          setActiveId(rest[0].id)
        }
      } else if (confirm === 'ip') {
        const result = await api.deleteHostsByIP(ip)
        onChanged(t('已删除 {ip} 下的 {n} 条资产', { ip, n: result.data.deleted }))
        onClose()
      }
      setConfirm(null)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t('删除失败'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <Dialog open onOpenChange={(open) => !open && onClose()}>
        <DialogContent
          showCloseButton={false}
          className="left-auto right-0 top-0 h-dvh max-h-dvh w-full max-w-xl translate-x-0 translate-y-0 gap-0 rounded-none border-y-0 border-l border-r-0 p-0"
        >
          <DialogTitle className="sr-only">{t("IP 资产详情")}</DialogTitle>
          <div className="flex items-start justify-between gap-3 border-b border-border px-5 py-3.5">
            <div className="min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="font-mono text-sm font-medium">{ip}</span>
                {hosts && (
                  <Badge variant="secondary" className="font-mono">
                    {hosts.length === 1 ? t('1 端口') : t('{n} 端口', { n: hosts.length })}
                  </Badge>
                )}
              </div>
              {host?.title && <p className="mt-1 truncate text-xs text-muted-foreground">{host.title}</p>}
            </div>
            <div className="flex items-center gap-0.5">
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={runRefresh}
                disabled={refreshing}
                aria-label={t("刷新此 IP 的端口与归属地")}
                title={t("刷新此 IP 的端口与归属地")}
              >
                <RefreshCw className={refreshing ? 'animate-spin' : ''} />
              </Button>
              <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label={t("关闭")}>
                <X />
              </Button>
            </div>
          </div>

          {hosts && hosts.length > 0 && (
            <div className="flex gap-1 overflow-x-auto border-b border-border px-5 py-2">
              {hosts.map((item) => (
                <button
                  key={item.id}
                  onClick={() => setActiveId(item.id)}
                  className={cn(
                    'shrink-0 rounded-md border px-2 py-1 font-mono text-[11px] leading-4 transition',
                    item.id === activeId
                      ? 'border-primary/50 bg-primary/10 text-primary'
                      : 'border-border text-muted-foreground hover:bg-muted',
                  )}
                >
                  {item.port} <span className="opacity-70">{item.service?.name}</span>
                </button>
              ))}
            </div>
          )}

          <div className="min-h-0 flex-1 divide-y divide-border overflow-y-auto px-5">
            {error && <p className="py-10 text-center text-xs text-destructive">{error}</p>}
            {!hosts && !error && <Loading label={t("加载 IP 资产")} />}
            {host && <PortDetail host={host} />}
          </div>

          {host && isAdmin && (
            <div className="flex gap-2 border-t border-border px-5 py-3">
              <Button variant="outline" size="sm" className="flex-1" onClick={() => setConfirm('one')}>
                <Trash2 />
                {t('删除此端口')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="flex-1 text-destructive hover:text-destructive"
                onClick={() => setConfirm('ip')}
              >
                <Globe2 />
                {t('删除该 IP 全部资产')}
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={confirm !== null} onOpenChange={(open) => !open && setConfirm(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{confirm === 'ip' ? t('删除该 IP 的全部资产') : t('删除资产')}</DialogTitle>
            <DialogDescription>
              {confirm === 'ip' ? (
                <>
                  {t('将删除')} <span className="font-mono text-foreground">{ip}</span>{' '}
                  {t('下的全部 {n} 条端口资产记录，操作不可撤销。', { n: hosts?.length ?? 0 })}
                </>
              ) : (
                <>
                  {t('将删除')} <span className="font-mono text-foreground">{ip}:{host?.port}</span> {t('的资产记录，该操作不可撤销。')}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          {actionError && <p className="text-xs text-destructive">{actionError}</p>}
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirm(null)} disabled={busy}>
              {t('取消')}
            </Button>
            <Button variant="destructive" onClick={() => void runDelete()} disabled={busy}>
              {busy ? <Spinner className="size-4" /> : <Trash2 />}
              {t('确认删除')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

function PortDetail({ host }: { host: Host }) {
  const headerEntries = useMemo(() => Object.entries(host.headers ?? {}), [host])

  return (
    <>
      <Section title={t("服务识别")}>
        <div className="flex flex-wrap items-center gap-1.5">
          {host.service.name && <Badge variant="success">{host.service.name}</Badge>}
          {host.service.product && (
            <Badge variant="outline">
              {host.service.product}
              {host.service.version ? ` ${host.service.version}` : ''}
            </Badge>
          )}
          {host.statusCode > 0 && <Badge variant="warning">{host.statusCode}</Badge>}
          {(host.apps ?? []).map((app) => (
            <Badge key={app} variant="info">
              {app}
            </Badge>
          ))}
          {host.device && <Badge>{host.device}</Badge>}
          {host.os && <Badge variant="secondary">{host.os}</Badge>}
        </div>
        {host.service.extraInfo && <p className="mt-2 text-xs text-muted-foreground">{host.service.extraInfo}</p>}
      </Section>

      {((host.hostnames ?? []).length > 0 || (host.domains ?? []).length > 0) && (
        <Section title={t("域名 / 主机名")}>
          <div className="flex flex-wrap gap-1.5">
            {[...(host.hostnames ?? []), ...(host.domains ?? [])].map((name) => (
              <Badge key={name} variant="outline" className="font-mono">
                {name}
              </Badge>
            ))}
          </div>
        </Section>
      )}

      <Section title={t("地理位置")}>
        <Field label={t("国家/地区")}>
          {host.geo?.private ? (
            <Badge variant="secondary">{t("内网")}</Badge>
          ) : (
            <span>
              {flagOf(host.geo?.countryCode) && <span className="mr-1">{flagOf(host.geo?.countryCode)}</span>}
              {[countryLabel(host.geo), host.geo?.province, host.geo?.city].filter(Boolean).join(' · ')}
            </span>
          )}
        </Field>
        {host.geo?.isp && <Field label="ISP">{host.geo.isp}</Field>}
        {host.asn && (
          <Field label="ASN" mono>
            AS{host.asn.number} {host.asn.org}
          </Field>
        )}
      </Section>

      {host.favicon && (
        <Section title="Favicon">
          <div className="flex items-center gap-3">
            <img
              src={api.faviconUrl(host.id)}
              alt="favicon"
              className="size-8 rounded border border-border bg-secondary object-contain p-0.5"
            />
            <div className="min-w-0 flex-1 space-y-1">
              <div className="flex items-center gap-1 font-mono text-xs">
                mmh3 {host.favicon.mmh3}
                <CopyButton value={String(host.favicon.mmh3)} />
              </div>
              <p className="truncate font-mono text-[10px] text-muted-foreground">{host.favicon.md5}</p>
            </div>
          </div>
        </Section>
      )}

      {host.tls && (
        <Section title={t("TLS 证书")}>
          <Field label={t("主体")}>{host.tls.subject}</Field>
          <Field label={t("颁发者")}>{host.tls.issuer}</Field>
          <Field label={t("通用名")} mono>
            {host.tls.commonName}
          </Field>
          <Field label={t("有效期")}>
            {host.tls.notBefore ? new Date(host.tls.notBefore).toLocaleDateString('zh-CN') : '—'} →{' '}
            {host.tls.notAfter ? new Date(host.tls.notAfter).toLocaleDateString('zh-CN') : '—'}
          </Field>
          {(host.tls.sans ?? []).length > 0 && (
            <Field label="SAN" mono>
              {(host.tls.sans ?? []).join(', ')}
            </Field>
          )}
        </Section>
      )}

      {host.banner && (
        <Section title="Banner">
          <pre className="max-h-48 overflow-auto rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[10.5px] leading-relaxed">
            {host.banner}
          </pre>
        </Section>
      )}

      {headerEntries.length > 0 && (
        <Section title={t("响应头")}>
          <div className="space-y-1 rounded-md border border-border bg-muted/40 p-2.5">
            {headerEntries.map(([key, value]) => (
              <div key={key} className="flex gap-2 font-mono text-[10.5px] leading-relaxed">
                <span className="shrink-0 text-primary/80">{key}:</span>
                <span className="break-all text-muted-foreground">{value}</span>
              </div>
            ))}
          </div>
        </Section>
      )}

      {host.scripts && Object.keys(host.scripts).length > 0 && (
        <Section title={t("Nmap 脚本")}>
          <div className="space-y-1.5">
            {Object.entries(host.scripts).map(([key, value]) => (
              <div key={key}>
                <p className="font-mono text-[10px] text-muted-foreground">{key}</p>
                <pre className="mt-0.5 whitespace-pre-wrap rounded border border-border bg-muted/40 p-2 font-mono text-[10px]">
                  {value}
                </pre>
              </div>
            ))}
          </div>
        </Section>
      )}

      {(host.cpe ?? []).length > 0 && (
        <Section title="CPE">
          <div className="flex flex-wrap gap-1.5">
            {host.cpe.map((item) => (
              <Badge key={item} variant="outline" className="font-mono">
                {item}
              </Badge>
            ))}
          </div>
        </Section>
      )}

      <Section title={t("时间")}>
        <Field label={t("首次发现")}>{timeAgo(host.firstSeen)}</Field>
        <Field label={t("最近发现")}>{timeAgo(host.lastSeen)}</Field>
        {host.source && (
          <Field label={t("来源")}>{sourceLabel(host.source)}</Field>
        )}
      </Section>
    </>
  )
}
