import { ChevronLeft, ChevronRight, Eye, Plus, Radar, RefreshCw, Trash2, XCircle } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'


import { Empty, Loading, PageHeader, StatusPill, duration, formatTime, timeAgo } from '@/components/shared'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import type { Task, TaskLog } from '@/lib/types'
import { t } from '@/lib/i18n'

const FILTERS = [
  { value: '', label: '全部' },
  { value: 'running', label: '运行中' },
  { value: 'pending', label: '排队中' },
  { value: 'done', label: '已完成' },
  { value: 'failed', label: '失败' },
]

const PHASES: Record<string, string> = {
  pending: '排队中',
  discovery: '端口发现',
  fingerprint: '指纹识别',
  service: '服务识别',
  done: '已完成',
  finalized: '收尾',
}

const PAGE_SIZE = 10

function isActive(task: Task) {
  return task.status === 'pending' || task.status === 'running'
}

function TaskCard({
  task,
  onOpen,
  onCancel,
  onDelete,
}: {
  task: Task
  onOpen: () => void
  onCancel: () => void
  onDelete: () => void
}) {
  const active = isActive(task)
  const percent = Math.min(100, Math.round(task.progress || 0))
  const targetList = task.targets ?? []
  const targets = targetList.slice(0, 3).join('、')

  return (
    <Card className="transition-colors hover:border-primary/30">
      <CardHeader className="gap-2 pb-3">
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="min-w-0 truncate text-sm">{task.name}</CardTitle>
          <StatusPill status={task.status} />
          <Badge variant="outline" className="font-mono text-[10px]">
            {task.tool}
          </Badge>
          {task.deep && (
            <Badge variant="secondary" className="text-[10px]">
              {t('深度识别')}
            </Badge>
          )}
          <div className="ml-auto flex items-center gap-1">
            <Button variant="ghost" size="sm" onClick={onOpen}>
              <Eye />
              <span className="hidden sm:inline">{t("详情")}</span>
            </Button>
            {active && (
              <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={onCancel}>
                <XCircle />
                <span className="hidden sm:inline">{t("取消")}</span>
              </Button>
            )}
            {!active && (
              <Button
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground hover:text-destructive"
                onClick={onDelete}
                aria-label={t("删除任务")}
              >
                <Trash2 />
              </Button>
            )}
          </div>
        </div>
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span className="min-w-0 truncate">
            {t('目标')} {targets || '—'}
            {targetList.length > 3 && t(' 等 {n} 项', { n: targetList.length })}
          </span>
          <span className="break-all font-mono">{t('端口 {ports}', { ports: task.ports })}</span>
        </div>
      </CardHeader>

      <CardContent className="space-y-3">
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t(PHASES[task.phase] ?? (task.phase || '准备中'))}</span>
            <span className="font-mono">{percent}%</span>
          </div>
          <Progress value={percent} />
        </div>

        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {[
            { label: t('主机'), value: task.stats?.hostsScanned ?? 0 },
            { label: t('开放端口'), value: task.stats?.openPorts ?? 0 },
            { label: t('新增'), value: task.stats?.newAssets ?? 0 },
            { label: t('更新'), value: task.stats?.updated ?? 0 },
          ].map((item) => (
            <div key={t(item.label)} className="rounded-md bg-muted/50 px-2 py-1.5">
              <p className="font-mono text-sm font-medium tabular-nums">{item.value}</p>
              <p className="text-[10px] text-muted-foreground">{t(item.label)}</p>
            </div>
          ))}
        </div>

        <div className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
          <span>{t('创建 {at}', { at: timeAgo(task.createdAt) })}</span>
          <span>{t('耗时 {d}', { d: duration(task.startedAt, task.finishedAt) })}</span>
          <span>{t('发起人 {user}', { user: task.createdBy || '—' })}</span>
          {task.error && <span className="text-destructive">{task.error}</span>}
        </div>
      </CardContent>
    </Card>
  )
}

function TaskDetail({ id, onClose }: { id: string; onClose: () => void }) {
  const [task, setTask] = useState<Task | null>(null)
  const [logs, setLogs] = useState<TaskLog[]>([])
  const [error, setError] = useState('')
  const lastRef = useRef(0)
  const boxRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    let alive = true
    let timer = 0
    // 任务是否仍处于活动状态：只有活动中的任务需要持续轮询。
    let active = true
    const tick = async () => {
      try {
        const res = await api.task(id)
        if (!alive) return
        setTask(res.data.task)
        setError('')
        const lr = await api.taskLogs(id, lastRef.current)
        if (!alive) return
        // 后端在没有新日志时会返回空数组；再兜一层，避免 null 让整个轮询中断。
        const fresh = lr.data.logs ?? []
        if (fresh.length) {
          lastRef.current = lr.data.last
          setLogs((prev) => [...prev, ...fresh].slice(-500))
        }
        // 排队中的任务同样需要继续轮询，否则详情页会一直停在“排队中”。
        active = res.data.running || res.data.task?.status === 'pending'
        if (active) timer = window.setTimeout(() => void tick(), 1800)
      } catch (err) {
        if (!alive) return
        setError(err instanceof Error ? err.message : t('加载任务失败'))
        // 单次请求失败（网络抖动、服务重启）不该让详情页永久停在错误上。
        if (active) timer = window.setTimeout(() => void tick(), 3000)
      }
    }
    void tick()
    return () => {
      alive = false
      window.clearTimeout(timer)
    }
  }, [id])

  useEffect(() => {
    const box = boxRef.current
    if (box) box.scrollTop = box.scrollHeight
  }, [logs])

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex flex-wrap items-center gap-2 pr-6">
            <span className="truncate">{task?.name ?? t('任务详情')}</span>
            {task && <StatusPill status={task.status} />}
          </DialogTitle>
          <DialogDescription className="font-mono text-[11px]">
            {task
              ? `${(task.targets ?? []).join('、')} · ${t('端口 {ports}', { ports: task.ports })} · ${task.tool}`
              : t('加载中…')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {!task && !error && <Loading label={t("加载任务")} />}
          {error && <p className="text-xs text-destructive">{error}</p>}

          {task && (
            <>
              <div className="space-y-1.5">
                <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                  <span>{t(PHASES[task.phase] ?? (task.phase || '排队'))}</span>
                  <span className="font-mono">{Math.round(task.progress || 0)}%</span>
                </div>
                <Progress value={Math.min(100, task.progress || 0)} />
              </div>

              <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
                {[
                  { label: t('主机'), value: task.stats?.hostsScanned ?? 0 },
                  { label: t('开放端口'), value: task.stats?.openPorts ?? 0 },
                  { label: t('新增'), value: task.stats?.newAssets ?? 0 },
                  { label: t('更新'), value: task.stats?.updated ?? 0 },
                  { label: t('错误'), value: task.stats?.errors ?? 0 },
                ].map((item) => (
                  <div key={t(item.label)} className="rounded-md border border-border px-2 py-1.5">
                    <p className="font-mono text-sm tabular-nums">{item.value}</p>
                    <p className="text-[10px] text-muted-foreground">{t(item.label)}</p>
                  </div>
                ))}
              </div>

              <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
                <span>{t('创建 {at}', { at: formatTime(task.createdAt) })}</span>
                <span>{t('开始 {at}', { at: formatTime(task.startedAt) })}</span>
                <span>{t('结束 {at}', { at: formatTime(task.finishedAt) })}</span>
                <span>{t('耗时 {d}', { d: duration(task.startedAt, task.finishedAt) })}</span>
              </div>

              {task.error && (
                <div className="rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-xs text-destructive">
                  {task.error}
                </div>
              )}
            </>
          )}

          <div className="space-y-1.5">
            <p className="text-[11px] font-medium text-muted-foreground">{t("运行日志")}</p>
            <div
              ref={boxRef}
              className="h-56 overflow-y-auto rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed"
            >
              {logs.length === 0 && <p className="text-muted-foreground">{t("暂无日志")}</p>}
              {logs.map((log) => (
                <p key={`${log.id}-${log.seq}`} className="flex gap-2">
                  <span className="shrink-0 text-muted-foreground/60">{log.at.slice(11, 19)}</span>
                  <span className={log.level === 'error' ? 'text-destructive' : log.level === 'warn' ? 'text-warn' : ''}>
                    {t(log.msg)}
                  </span>
                </p>
              ))}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t('关闭')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// 后端只认 "80,443,1-1024" 形式的表达式，预设直接落成端口串，选中后仍可手动微调。
const PORT_PRESETS = [
  {
    value: 'common',
    label: '常见服务端口',
    ports: '21,22,23,25,53,80,110,135,139,143,443,445,465,587,993,995,1433,1521,2049,3306,3389,5432,5900,5985,6379,8080,8443,9000,9090,9200,11211,27017',
  },
  { value: 'web', label: 'Web / 中间件', ports: '80,81,443,3000,5000,5601,7001,8000,8008,8009,8080,8081,8082,8088,8090,8180,8443,8848,8888,9000,9001,9090,9200,9300,9443,10000' },
  { value: 'internal', label: '远程与内网', ports: '22,23,135,139,389,445,636,1433,3306,3389,5432,5900,5985,5986,6379,11211,27017' },
  { value: 'ai', label: 'AI 与容器', ports: '2375,2376,5000,6443,7860,8000,8080,8081,8188,8888,9090,10250,11434,11435' },
  {
    value: 'top100',
    label: 'Nmap Top 100',
    ports: '7,9,13,21,22,23,25,26,37,53,79,80,81,88,106,110,111,113,119,135,139,143,144,179,199,389,427,443,444,445,465,513,514,515,543,544,548,554,587,631,646,873,990,993,995,1025,1026,1027,1028,1029,1110,1433,1720,1723,1755,1900,2000,2001,2049,2121,2717,3000,3128,3306,3389,3986,4899,5000,5009,5051,5060,5101,5190,5357,5432,5631,5666,5800,5900,6000,6001,6646,7070,8000,8008,8009,8080,8081,8443,8888,9100,9999,10000,32768,49152,49153,49154,49155,49156,49157',
  },
  { value: 'all', label: '全端口（1-65535）', ports: '1-65535' },
  { value: 'custom', label: '自定义', ports: '' },
]

// countPorts 只做展示用的端口计数，语法错误时按 0 处理，真正校验交给后端。
function countPorts(spec: string): number {
  let total = 0
  for (const part of spec.split(',')) {
    const [from, to] = part.trim().split('-')
    const start = Number(from)
    const end = to === undefined ? start : Number(to)
    if (!Number.isInteger(start) || !Number.isInteger(end) || start < 1 || end > 65535 || start > end) continue
    total += end - start + 1
  }
  return total
}

function CreateTask({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState('')
  const [targets, setTargets] = useState('')
  const [ports, setPorts] = useState('')
  const [preset, setPreset] = useState('custom')
  const [rate, setRate] = useState('1000')
  const [tool, setTool] = useState('auto')
  const [deep, setDeep] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let alive = true
    api
      .scanSettings()
      .then((res) => {
        if (!alive) return
        const spec = res.data.settings.defaultPorts?.trim() || '80,443,8080,8443'
        setPorts(spec)
        setPreset(PORT_PRESETS.find((item) => item.ports === spec)?.value ?? 'custom')
      })
      .catch(() => {
        if (alive) setPorts((prev) => prev || '80,443,8080,8443')
      })
    return () => {
      alive = false
    }
  }, [])

  const applyPreset = (value: string) => {
    const item = PORT_PRESETS.find((entry) => entry.value === value)
    if (!item) return
    setPreset(value)
    if (item.ports) setPorts(item.ports)
  }

  const portCount = countPorts(ports)

  const submit = async () => {
    const list = targets
      .split(/[\s,;]+/)
      .map((item) => item.trim())
      .filter(Boolean)
    if (!list.length) {
      setError(t('请至少填写一个扫描目标'))
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.createTask({
        name: name.trim() || undefined,
        targets: list,
        ports: ports.trim() || undefined,
        rate: Number(rate) || undefined,
        tool,
        deep,
      })
      onCreated()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('创建任务失败'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("新建扫描任务")}</DialogTitle>
          <DialogDescription>{t("填写目标与端口，任务将在后台排队执行")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="task-name">{t("任务名称")}</Label>
            <Input id="task-name" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("可选，默认按时间生成")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="task-targets">{t("扫描目标")}</Label>
            <Textarea
              id="task-targets"
              rows={3}
              value={targets}
              onChange={(e) => setTargets(e.target.value)}
              placeholder={t("192.168.6.0/24 或 10.0.0.1-10.0.0.50，空格/换行分隔")}
              className="font-mono text-xs"
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-baseline justify-between gap-2">
              <Label htmlFor="task-ports">{t('端口')}</Label>
              <span className="text-[11px] text-muted-foreground">
                {portCount > 0 ? t('共 {n} 个端口', { n: portCount.toLocaleString() }) : t('端口格式：80,443,1000-2000')}
              </span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {PORT_PRESETS.map((item) => (
                <button
                  key={item.value}
                  type="button"
                  onClick={() => applyPreset(item.value)}
                  className={cn(
                    'rounded-full border px-2.5 py-1 text-[11px] transition-colors',
                    preset === item.value
                      ? 'border-primary/40 bg-primary/10 font-medium text-primary'
                      : 'border-border text-muted-foreground hover:bg-muted',
                  )}
                >
                  {t(item.label)}
                </button>
              ))}
            </div>
            <Input
              id="task-ports"
              value={ports}
              onChange={(e) => {
                setPorts(e.target.value)
                setPreset('custom')
              }}
              placeholder={t('端口格式：80,443,1000-2000')}
              className="font-mono text-xs"
            />
            {preset === 'all' && (
              <p className="text-[11px] text-warn">{t('全端口扫描较慢，按 1000 pps 估算单个目标需数分钟')}</p>
            )}
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="task-rate">{t("速率（pps）")}</Label>
              <Input id="task-rate" value={rate} onChange={(e) => setRate(e.target.value)} className="font-mono text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label>{t("扫描引擎")}</Label>
              <Select value={tool} onValueChange={setTool}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">{t("自动选择（推荐）")}</SelectItem>
                  <SelectItem value="native">{t("原生 TCP")}</SelectItem>
                  <SelectItem value="masscan">Masscan</SelectItem>
                  <SelectItem value="nmap">Nmap</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <label className="flex items-start gap-2.5 rounded-md border border-border p-2.5">
            <input
              type="checkbox"
              checked={deep}
              onChange={(e) => setDeep(e.target.checked)}
              className="mt-0.5 size-4 accent-primary"
            />
            <span className="space-y-0.5">
              <span className="block text-xs font-medium">{t("深度服务识别")}</span>
              <span className="block text-[11px] text-muted-foreground">
                {t('对开放端口调用 nmap -sV（较慢，但服务与版本更准确）')}
              </span>
            </span>
          </label>

          {error && (
            <p className="rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-xs text-destructive">
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            {t('取消')}
          </Button>
          <Button onClick={submit} disabled={busy}>
            {busy ? t('创建中…') : t('开始扫描')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function Tasks() {
  const [items, setItems] = useState<Task[]>([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [detailId, setDetailId] = useState<string | null>(null)
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const [confirming, setConfirming] = useState<{ task: Task; kind: 'cancel' | 'delete' } | null>(null)
  const [busy, setBusy] = useState(false)
  const [clearOpen, setClearOpen] = useState(false)
  const [notice, setNotice] = useState('')

  const load = useCallback(async () => {
    setError('')
    try {
      const res = await api.tasks({ status: status || undefined, page, size: PAGE_SIZE })
      setItems(res.data)
      setTotal(res.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('加载任务失败'))
    } finally {
      setLoading(false)
    }
  }, [status, page])

  useEffect(() => {
    setLoading(true)
    void load()
  }, [load])

  useEffect(() => {
    if (!items.some(isActive)) return
    const timer = window.setTimeout(() => void load(), 4000)
    return () => window.clearTimeout(timer)
  }, [items, load])

  const runAction = async () => {
    if (!confirming) return
    setBusy(true)
    try {
      if (confirming.kind === 'cancel') await api.cancelTask(confirming.task.id)
      else await api.deleteTask(confirming.task.id)
      setConfirming(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('操作失败'))
    } finally {
      setBusy(false)
    }
  }

  const runClear = async () => {
    setBusy(true)
    setNotice('')
    try {
      const res = await api.clearTasks()
      setClearOpen(false)
      setNotice(t('已清空 {n} 个已结束的任务', { n: res.data.deleted }))
      setPage(1)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('清空任务失败'))
    } finally {
      setBusy(false)
    }
  }

  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className="space-y-5">
      <PageHeader title={t("扫描任务")} description={t("创建并跟踪端口发现、指纹识别与服务探测任务")}>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCw className={loading ? 'animate-spin' : undefined} />
          {t('刷新')}
        </Button>
        {isAdmin && (
          <Button variant="outline" size="sm" onClick={() => setClearOpen(true)}>
            <Trash2 />
            {t('清空已结束')}
          </Button>
        )}
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus />
          {t('新建任务')}
        </Button>
      </PageHeader>

      <div className="flex flex-wrap items-center gap-1.5">
        {FILTERS.map((item) => (
          <Button
            key={item.value}
            variant={status === item.value ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => {
              setStatus(item.value)
              setPage(1)
            }}
          >
            {t(item.label)}
          </Button>
        ))}
        <span className="ml-auto text-xs text-muted-foreground">{t('共 {n} 个任务', { n: total })}</span>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      {notice && (
        <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">{notice}</div>
      )}

      {loading && items.length === 0 && (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-40 w-full rounded-xl" />
          ))}
        </div>
      )}

      {!loading && items.length === 0 && (
        <Card>
          <Empty title={t("暂无扫描任务")} hint={t("点击右上角「新建任务」开始一次资产测绘")} icon={<Radar className="size-5" />} />
        </Card>
      )}

      <div className="space-y-3">
        {items.map((task) => (
          <TaskCard
            key={task.id}
            task={task}
            onOpen={() => setDetailId(task.id)}
            onCancel={() => setConfirming({ task, kind: 'cancel' })}
            onDelete={() => setConfirming({ task, kind: 'delete' })}
          />
        ))}
      </div>

      {pages > 1 && (
        <div className="flex items-center justify-center gap-3">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            <ChevronLeft />
            {t('上一页')}
          </Button>
          <span className="text-xs text-muted-foreground">
            {page} / {pages}
          </span>
          <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => setPage((p) => p + 1)}>
            {t('下一页')}
            <ChevronRight />
          </Button>
        </div>
      )}

      {createOpen && <CreateTask onClose={() => setCreateOpen(false)} onCreated={() => void load()} />}
      {detailId && <TaskDetail id={detailId} onClose={() => setDetailId(null)} />}

      <Dialog open={clearOpen} onOpenChange={setClearOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("清空已结束的任务")}</DialogTitle>
            <DialogDescription>
              {t('会删除全部已完成、失败、已取消与已中断的任务记录及其日志；运行中与排队中的任务会保留。此操作不可撤销。')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setClearOpen(false)} disabled={busy}>
              {t('取消')}
            </Button>
            <Button variant="destructive" onClick={runClear} disabled={busy}>
              {busy ? t('清理中…') : t('确认清空')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!confirming} onOpenChange={(open) => !open && setConfirming(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{confirming?.kind === 'cancel' ? t('取消扫描任务') : t('删除扫描任务')}</DialogTitle>
            <DialogDescription>
              {confirming?.kind === 'cancel'
                ? t('任务会立即停止，已发现的资产将保留。')
                : t('任务记录与运行日志会被删除，已发现的资产不受影响。')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(null)} disabled={busy}>
              {t('返回')}
            </Button>
            <Button variant="destructive" onClick={runAction} disabled={busy}>
              {busy ? t('处理中…') : t('确认')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
