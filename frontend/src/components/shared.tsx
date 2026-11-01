import { Check, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Copy, Inbox, Loader2 } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { getLang, t } from '@/lib/i18n'

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn('size-4 animate-spin', className)} />
}

export function Loading({ label = t('加载中') }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-16 text-sm text-muted-foreground">
      <Spinner />
      {label}
    </div>
  )
}

export function Empty({ title, hint, icon }: { title: string; hint?: string; icon?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
      <div className="flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
        {icon ?? <Inbox className="size-5" />}
      </div>
      <p className="text-sm font-medium">{title}</p>
      {hint && <p className="max-w-sm text-xs leading-relaxed text-muted-foreground">{hint}</p>}
    </div>
  )
}

export function CopyButton({ value, label }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
    } catch {
      const ta = document.createElement('textarea')
      ta.value = value
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      ta.remove()
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 1400)
  }

  return (
    <Button
      type="button"
      variant="ghost"
      size={label ? 'sm' : 'icon-sm'}
      onClick={() => void copy()}
      title={t("复制")}
      className="text-muted-foreground"
    >
      {copied ? <Check className="text-ok" /> : <Copy />}
      {label ? <span>{copied ? t('已复制') : label}</span> : null}
    </Button>
  )
}

type StatusTone = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'

const STATUS: Record<string, { label: string; tone: StatusTone }> = {
  pending: { label: '排队中', tone: 'secondary' },
  running: { label: '运行中', tone: 'default' },
  done: { label: '已完成', tone: 'success' },
  failed: { label: '失败', tone: 'destructive' },
  canceled: { label: '已取消', tone: 'outline' },
  interrupted: { label: '已中断', tone: 'warning' },
}

export function StatusPill({ status }: { status: string }) {
  const meta = STATUS[status] ?? { label: status || '未知', tone: 'outline' as StatusTone }
  return (
    <Badge variant={meta.tone} className="gap-1.5">
      {status === 'running' && <span className="size-1.5 animate-pulse rounded-full bg-primary" />}
      {t(meta.label)}
    </Badge>
  )
}

export function PageHeader({ title, description, children }: { title: string; description?: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div className="min-w-0 space-y-1">
        <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">{title}</h1>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  )
}

export function formatTime(iso?: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function timeAgo(iso?: string): string {
  if (!iso) return '—'
  const stamp = new Date(iso).getTime()
  if (Number.isNaN(stamp)) return '—'
  const diff = Date.now() - stamp
  const mins = Math.round(Math.abs(diff) / 60000)
  const future = diff < 0
  if (mins < 1) return t('刚刚')
  if (mins < 60) {
    return future ? t('{n} 分钟后', { n: mins }) : t('{n} 分钟前', { n: mins })
  }
  const hours = Math.round(mins / 60)
  if (hours < 24) {
    return future ? t('{n} 小时后', { n: hours }) : t('{n} 小时前', { n: hours })
  }
  const days = Math.round(hours / 24)
  if (days < 30) {
    return future ? t('{n} 天后', { n: days }) : t('{n} 天前', { n: days })
  }
  return formatTime(iso)
}

export function duration(from?: string, to?: string): string {
  if (!from) return '—'
  const a = new Date(from).getTime()
  const b = to ? new Date(to).getTime() : Date.now()
  if (Number.isNaN(a) || Number.isNaN(b)) return '—'
  const sec = Math.max(0, Math.round((b - a) / 1000))
  if (sec < 60) return t('{n} 秒', { n: sec })
  const min = Math.floor(sec / 60)
  if (min < 60) return t('{m} 分 {s} 秒', { m: min, s: sec % 60 })
  return t('{h} 时 {m} 分', { h: Math.floor(min / 60), m: min % 60 })
}

// countryLabel 优先用 ISO 码走 Intl 本地化，缺失时回退后端写入的国家名。
export function countryLabel(geo?: { countryCode?: string; country?: string }): string {
  const code = geo?.countryCode?.toUpperCase()
  if (!code) return geo?.country || '—'
  if (code === 'LAN') return t('内网')
  try {
    const name = new Intl.DisplayNames([getLang()], { type: 'region' }).of(code)
    if (name) return name
  } catch {
    // 运行环境缺少 Intl.DisplayNames 时退回原始值
  }
  return geo?.country || code
}

// flagOf 把 ISO 3166-1 两位国家码转成国旗 emoji，非法或本地区码返回空串。
export function flagOf(code?: string): string {
  const cc = (code ?? '').trim().toUpperCase()
  if (!/^[A-Z]{2}$/.test(cc)) return ''
  return String.fromCodePoint(...[...cc].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65))
}

// pageItems 生成带首尾与省略号的页码序列，span 控制当前页左右展开的数量。
function pageItems(current: number, total: number, span: number): (number | 'gap')[] {
  const width = span*2 + 1
  if (total <= width + 2) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  const picked = new Set<number>([1, total])
  for (let page = current - span; page <= current + span; page += 1) {
    if (page >= 1 && page <= total) picked.add(page)
  }
  const list = [...picked].sort((a, b) => a - b)
  const result: (number | 'gap')[] = []
  let prev = 0
  for (const page of list) {
    if (prev && page - prev > 1) result.push('gap')
    result.push(page)
    prev = page
  }
  return result
}

function useWidePager(): boolean {
  const [wide, setWide] = useState(() => window.matchMedia('(min-width: 640px)').matches)
  useEffect(() => {
    const media = window.matchMedia('(min-width: 640px)')
    const sync = () => setWide(media.matches)
    media.addEventListener('change', sync)
    return () => media.removeEventListener('change', sync)
  }, [])
  return wide
}

export function Pager({
  page,
  pages,
  onChange,
}: {
  page: number
  pages: number
  onChange: (page: number) => void
}) {
  const wide = useWidePager()
  if (pages <= 1) return null

  const items = pageItems(page, pages, wide ? 5 : 1)
  const arrow = (label: string, disabled: boolean, delta: number, icon: ReactNode) => (
    <Button
      variant="outline"
      size="icon-sm"
      className="shrink-0"
      disabled={disabled}
      onClick={() => onChange(delta)}
      aria-label={label}
      title={label}
    >
      {icon}
    </Button>
  )

  return (
    <div className="flex w-full items-center gap-1 text-xs">
      {arrow(t('第一页'), page <= 1, 1, <ChevronsLeft />)}
      {arrow(t('上一页'), page <= 1, page - 1, <ChevronLeft />)}
      <div className="flex flex-1 items-center gap-1">
        {items.map((item, index) =>
          item === 'gap' ? (
            <span key={`gap-${index}`} className="w-4 shrink-0 text-center text-muted-foreground">
              …
            </span>
          ) : (
            <Button
              key={item}
              variant={item === page ? 'default' : 'ghost'}
              className="h-8 min-w-0 flex-1 px-1 tabular-nums"
              aria-label={t('第 {n} 页', { n: item })}
              aria-current={item === page ? 'page' : undefined}
              onClick={() => onChange(item)}
            >
              {item}
            </Button>
          ),
        )}
      </div>
      {arrow(t('下一页'), page >= pages, page + 1, <ChevronRight />)}
      {arrow(t('最后一页'), page >= pages, pages, <ChevronsRight />)}
    </div>
  )
}
