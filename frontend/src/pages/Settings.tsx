import { CheckCircle2, KeyRound, Plus, RefreshCw, Save, ShieldAlert, UserPlus, X, XCircle } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { CopyButton, Loading, PageHeader, formatTime } from '@/components/shared'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import type { GeoLibraryStatus, ScanSettingsPayload, ToolInfo, User } from '@/lib/types'
import { t } from '@/lib/i18n'

function Notice({ tone, message }: { tone: 'ok' | 'error'; message: string }) {
  if (!message) return null
  return (
    <p
      className={
        tone === 'ok'
          ? 'rounded-md border border-ok/30 bg-ok/8 px-3 py-2 text-xs text-ok'
          : 'rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-xs text-destructive'
      }
    >
      {message}
    </p>
  )
}

function AccountPanel() {
  const { user, refresh } = useAuth()
  const [key, setKey] = useState(user?.apiKey ?? '')
  const [newKey, setNewKey] = useState('')
  const [rotating, setRotating] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ tone: 'ok' | 'error'; message: string }>({ tone: 'ok', message: '' })

  useEffect(() => {
    setKey(user?.apiKey ?? '')
  }, [user?.apiKey])

  const rotate = async () => {
    setRotating(true)
    setNotice({ tone: 'ok', message: '' })
    try {
      const res = await api.rotateAPIKey()
      setNewKey(res.data.apiKey)
      setKey(res.data.apiKey)
      await refresh()
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('重置失败') })
    } finally {
      setRotating(false)
    }
  }

  const savePassword = async () => {
    if (password !== confirm) {
      setNotice({ tone: 'error', message: t('两次输入的新密码不一致') })
      return
    }
    setBusy(true)
    setNotice({ tone: 'ok', message: '' })
    try {
      await api.changePassword(oldPassword, password)
      setOldPassword('')
      setPassword('')
      setConfirm('')
      setNotice({ tone: 'ok', message: t('密码已更新') })
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('修改密码失败') })
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid gap-5 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("账户信息")}</CardTitle>
          <CardDescription>{t("当前登录身份与权限")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("用户名")}</span>
            <span className="font-medium">{user?.username ?? '—'}</span>
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("角色")}</span>
            <Badge variant={user?.role === 'admin' ? 'default' : 'secondary'}>
              {user?.role === 'admin' ? t('管理员') : t('只读用户')}
            </Badge>
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("创建时间")}</span>
            <span className="text-xs">{formatTime(user?.createdAt)}</span>
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("最近登录")}</span>
            <span className="text-xs">{formatTime(user?.lastLoginAt)}</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">API Key</CardTitle>
          <CardDescription>{t("用于脚本与外部系统调用，请求头 X-API-Key")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2 rounded-md border border-border bg-muted/40 px-3 py-2">
            <code className="min-w-0 flex-1 truncate font-mono text-xs">{key || t('未生成')}</code>
            {key && <CopyButton value={key} />}
          </div>

          {newKey && (
            <p className="rounded-md border border-warn/30 bg-warn/8 px-3 py-2 text-[11px] text-warn">
              {t('新 Key 已生成，请立即复制保存；旧 Key 已失效。')}
            </p>
          )}

          <Button variant="outline" size="sm" onClick={rotate} disabled={rotating}>
            <RefreshCw className={rotating ? 'animate-spin' : undefined} />
            {t('重新生成')}
          </Button>
        </CardContent>
      </Card>

      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle className="text-base">{t("修改密码")}</CardTitle>
          <CardDescription>{t("新密码至少 8 位")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="space-y-1.5">
              <Label htmlFor="old-password">{t("当前密码")}</Label>
              <Input
                id="old-password"
                type="password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
                autoComplete="current-password"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new-password">{t("新密码")}</Label>
              <Input
                id="new-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="new-password"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="confirm-password">{t("确认新密码")}</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                autoComplete="new-password"
              />
            </div>
          </div>
          <Notice {...notice} />
          <Button size="sm" onClick={savePassword} disabled={busy || !oldPassword || password.length < 8}>
            <KeyRound />
            {t('保存密码')}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}

function ScannerPanel() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const [tools, setTools] = useState<ToolInfo | null>(null)
  const [payload, setPayload] = useState<ScanSettingsPayload | null>(null)
  const [iface, setIface] = useState('auto')
  const [routerMac, setRouterMac] = useState('')
  const [extraArgs, setExtraArgs] = useState('')
  const [deny, setDeny] = useState<string[]>([])
  const [defaultPorts, setDefaultPorts] = useState('')
  const [denyInput, setDenyInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ tone: 'ok' | 'error'; message: string }>({ tone: 'ok', message: '' })

  const load = useCallback(async () => {
    try {
      const [toolRes, scanRes] = await Promise.all([api.tools(), api.scanSettings()])
      setTools(toolRes.data)
      setPayload(scanRes.data)
      setIface(scanRes.data.settings.scanInterface || 'auto')
      setRouterMac(scanRes.data.settings.masscanRouterMac || '')
      setExtraArgs(scanRes.data.settings.masscanExtraArgs || '')
      setDefaultPorts(scanRes.data.settings.defaultPorts || '')
      setDeny(scanRes.data.settings.denyCidrs ?? [])
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('加载扫描配置失败') })
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const save = async () => {
    setBusy(true)
    setNotice({ tone: 'ok', message: '' })
    const value = iface === 'auto' ? '' : iface
    try {
      await api.saveScanSettings({
        scanInterface: value,
        masscanInterface: value,
        masscanRouterMac: routerMac.trim(),
        masscanExtraArgs: extraArgs.trim(),
        defaultPorts: defaultPorts.trim(),
        denyCidrs: deny,
      })
      setNotice({ tone: 'ok', message: t('扫描配置已保存，新任务生效') })
      await load()
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('保存失败') })
    } finally {
      setBusy(false)
    }
  }

  const addDeny = () => {
    const value = denyInput.trim()
    if (!value) return
    if (!/^[0-9a-fA-F:.\/-]+$/.test(value)) {
      setNotice({ tone: 'error', message: t('地址段格式不正确，例如 192.168.0.0/16 或 10.0.0.1-10.0.0.255') })
      return
    }
    setNotice({ tone: 'ok', message: '' })
    setDeny((prev) => (prev.includes(value) ? prev : [...prev, value]))
    setDenyInput('')
  }

  const detectedMac = payload?.defaultRoute.gatewayMac ?? ''

  return (
    <div className="grid gap-5 lg:grid-cols-2">
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle className="text-base">{t("扫描网络与限制")}</CardTitle>
          <CardDescription>
            {t('指定探测流量走哪块网卡（Masscan 另需网关 MAC），并维护禁止扫描的地址段')}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>{t("网卡")}</Label>
              <Select value={iface} onValueChange={setIface} disabled={!isAdmin}>
                <SelectTrigger>
                  <SelectValue placeholder={t("自动探测")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">{t("自动探测（默认路由）")}</SelectItem>
                  {(payload?.interfaces ?? []).map((item) => (
                    <SelectItem key={item.name} value={item.name}>
                      {item.name}
                      {item.default ? t(' · 默认') : ''}
                      {item.ipv4 ? ` · ${item.ipv4}` : ''}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {payload && (
                <p className="text-[11px] text-muted-foreground">
                  {t('当前默认路由：')} {payload.defaultRoute.interface || t('未知')}
                  {payload.defaultRoute.gateway ? t(' · 网关 {gw}', { gw: payload.defaultRoute.gateway }) : ''}
                </p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="router-mac">{t("网关 MAC（Masscan）")}</Label>
              <div className="flex gap-2">
                <Input
                  id="router-mac"
                  value={routerMac}
                  onChange={(e) => setRouterMac(e.target.value)}
                  placeholder={detectedMac || 'aa:bb:cc:dd:ee:ff'}
                  className="font-mono text-xs"
                  disabled={!isAdmin}
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="shrink-0"
                  disabled={!isAdmin || !detectedMac}
                  onClick={() => setRouterMac(detectedMac)}
                >
                  {t('使用探测值')}
                </Button>
              </div>
              <p className="text-[11px] text-muted-foreground">
                {t('自动探测：')} {detectedMac || t('未获取到网关 MAC，可先 ping 一次网关再刷新')}
              </p>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="extra-args">{t("Masscan 额外参数")}</Label>
            <Input
              id="extra-args"
              value={extraArgs}
              onChange={(e) => setExtraArgs(e.target.value)}
              placeholder="--retries 3 --wait 3 --source-port 40000"
              className="font-mono text-xs"
              disabled={!isAdmin}
            />
            <p className="text-[11px] text-muted-foreground">
              {t('原样追加到 masscan 命令行；扫描目标、端口、速率与输出格式由系统管理，无需填写。')}
            </p>
          </div>

          <Separator />

          <div className="space-y-1.5">
            <Label htmlFor="default-ports">{t("默认扫描端口")}</Label>
            <Input
              id="default-ports"
              value={defaultPorts}
              onChange={(e) => setDefaultPorts(e.target.value)}
              placeholder={t("80,443,8080,8443 或 1-1024")}
              className="font-mono text-xs"
              disabled={!isAdmin}
            />
            <p className="text-[11px] text-muted-foreground">
              {t('新建任务时端口留空即使用这里的值；支持逗号分隔与区间（如 1-1024）。留空则回退为 80,443,8080,8443。')}
            </p>
          </div>

          <Separator />

          <div className="space-y-2">
            <Label>{t("禁止扫描网段")}</Label>
            <div className="flex flex-wrap gap-1.5">
              {deny.length === 0 && (
                <span className="text-[11px] text-muted-foreground">{t("当前不做限制，任意地址段都可以提交扫描")}</span>
              )}
              {deny.map((item) => (
                <Badge key={item} variant="secondary" className="gap-1 font-mono">
                  {item}
                  {isAdmin && (
                    <button
                      type="button"
                      onClick={() => setDeny((prev) => prev.filter((v) => v !== item))}
                      aria-label={t('移除 {item}', { item })}
                      className="opacity-60 transition hover:opacity-100"
                    >
                      <X className="size-3" />
                    </button>
                  )}
                </Badge>
              ))}
            </div>
            <div className="flex gap-2">
              <Input
                value={denyInput}
                onChange={(e) => setDenyInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    addDeny()
                  }
                }}
                placeholder={t("192.168.0.0/16 或 10.0.0.1-10.0.0.255")}
                className="font-mono text-xs"
                disabled={!isAdmin}
              />
              <Button type="button" variant="outline" size="sm" className="shrink-0" onClick={addDeny} disabled={!isAdmin || !denyInput.trim()}>
                <Plus />
                {t('添加')}
              </Button>
            </div>
            <p className="text-[11px] text-muted-foreground">
              {t('支持 IP、CIDR 与 a-b 区间，命中其中任一条的任务会在端口发现前被拒绝；清空即不限制。')}
            </p>
          </div>

          <Notice {...notice} />

          {isAdmin ? (
            <Button size="sm" onClick={save} disabled={busy}>
              <Save />
              {busy ? t('保存中…') : t('保存配置')}
            </Button>
          ) : (
            <p className="text-[11px] text-muted-foreground">{t("仅管理员可修改扫描配置。")}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("扫描引擎")}</CardTitle>
          <CardDescription>{t("本机可用的探测能力")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {!tools && <Loading label={t("检测中")} />}
          {(tools?.tools ?? []).map((tool) => (
            <div key={tool.name} className="rounded-lg border border-border p-3">
              <div className="flex items-center gap-2">
                <span className="font-mono text-sm">{tool.name}</span>
                {tool.available ? (
                  <Badge variant="success" className="gap-1">
                    <CheckCircle2 className="size-3" />
                    {t('可用')}
                  </Badge>
                ) : (
                  <Badge variant="destructive" className="gap-1">
                    <XCircle className="size-3" />
                    {t('不可用')}
                  </Badge>
                )}
                {tool.version && <span className="text-[11px] text-muted-foreground">{tool.version}</span>}
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-muted-foreground">{t(tool.purpose)}</p>
              {!tool.available && tool.reason && (
                <p className="mt-1 flex items-start gap-1 text-[11px] text-warn">
                  <ShieldAlert className="mt-0.5 size-3 shrink-0" />
                  {t(tool.reason)}
                </p>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("运行限制")}</CardTitle>
          <CardDescription>{t("服务端强制约束")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("单任务目标上限")}</span>
            <span className="font-mono text-xs">{tools?.limits.maxTargets ?? '—'}</span>
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("速率上限（pps）")}</span>
            <span className="font-mono text-xs">{tools?.limits.maxRate ?? '—'}</span>
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">{t("地理信息")}</span>
            <Badge variant={tools?.geo.ready ? 'success' : 'secondary'}>
              {tools?.geo.downloading ? t('更新中') : tools?.geo.ready ? t('本地库已就绪') : t('未就绪')}
            </Badge>
          </div>
          <Separator />
          <div className="space-y-1">
            <span className="text-muted-foreground">{t("禁止扫描网段")}</span>
            <p className="break-all font-mono text-[11px]">{(tools?.limits.denyCIDRs ?? []).join('、') || '—'}</p>
          </div>
        </CardContent>
      </Card>
      <GeoPanel />
    </div>
  )
}

function GeoPanel() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const [status, setStatus] = useState<GeoLibraryStatus | null>(null)
  const [enabled, setEnabled] = useState(true)
  const [auto, setAuto] = useState(true)
  const [cityUrl, setCityUrl] = useState('')
  const [asnUrl, setAsnUrl] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ tone: 'ok' | 'error'; message: string }>({ tone: 'ok', message: '' })

  const load = useCallback(async () => {
    try {
      const res = await api.geo()
      setStatus(res.data.status)
      setEnabled(res.data.settings.enabled)
      setAuto(res.data.settings.autoDownload)
      setCityUrl(res.data.settings.cityUrl)
      setAsnUrl(res.data.settings.asnUrl)
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('加载地理库配置失败') })
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (!status?.downloading) return
    const timer = setInterval(() => {
      void api.geo().then((res) => setStatus(res.data.status)).catch(() => undefined)
    }, 2000)
    return () => clearInterval(timer)
  }, [status?.downloading])

  const save = async () => {
    setBusy(true)
    setNotice({ tone: 'ok', message: '' })
    try {
      const res = await api.saveGeo({
        enabled,
        autoDownload: auto,
        cityUrl: cityUrl.trim(),
        asnUrl: asnUrl.trim(),
      })
      setStatus(res.data.status)
      setNotice({ tone: 'ok', message: t('地理库配置已保存') })
      if (res.data.status.downloading) {
        setNotice({ tone: 'ok', message: t('地理库配置已保存，正在后台下载') })
      }
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('保存失败') })
    } finally {
      setBusy(false)
    }
  }

  const refresh = async () => {
    setBusy(true)
    setNotice({ tone: 'ok', message: '' })
    try {
      const res = await api.refreshGeo()
      setStatus(res.data.status)
      setNotice({ tone: 'ok', message: t('已开始后台更新，下载完成后自动生效') })
    } catch (err) {
      setNotice({ tone: 'error', message: err instanceof Error ? err.message : t('更新失败') })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card className="lg:col-span-2">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          {t('地理信息库')}
          <Badge variant={status?.ready ? 'success' : status?.downloading ? 'default' : 'secondary'}>
            {status?.downloading ? t('更新中') : status?.ready ? t('已就绪') : t('未就绪')}
          </Badge>
        </CardTitle>
        <CardDescription>
          {t('自动拉取公开的 DB-IP Lite（mmdb）离线库，扫描时本地解析国家、省市与 ASN，不依赖任何在线查询接口')}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {notice.message && <Notice tone={notice.tone} message={notice.message} />}
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="rounded-lg border border-border p-3 text-sm">
            <p className="text-muted-foreground text-xs">{t("城市库")}</p>
            <p className="mt-1 font-mono text-xs">
              {status?.ready ? formatTime(status.cityDate) : t('未加载')}
            </p>
          </div>
          <div className="rounded-lg border border-border p-3 text-sm">
            <p className="text-muted-foreground text-xs">{t("ASN 库")}</p>
            <p className="mt-1 font-mono text-xs">{status?.asnReady ? t('已加载') : t('未加载')}</p>
          </div>
        </div>
        {status?.lastError && (
          <p className="flex items-start gap-2 text-xs text-destructive">
            <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span className="break-all">{status.lastError}</span>
          </p>
        )}
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="geo-city">{t("城市库下载地址")}</Label>
            <Input
              id="geo-city"
              value={cityUrl}
              disabled={!isAdmin || busy}
              onChange={(e) => setCityUrl(e.target.value)}
              placeholder="https://…/dbip-city-lite-{month}.mmdb.gz"
              className="font-mono text-xs"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="geo-asn">{t("ASN 库下载地址")}</Label>
            <Input
              id="geo-asn"
              value={asnUrl}
              disabled={!isAdmin || busy}
              onChange={(e) => setAsnUrl(e.target.value)}
              placeholder="https://…/dbip-asn-lite-{month}.mmdb.gz"
              className="font-mono text-xs"
            />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              className="h-4 w-4 accent-primary"
              checked={enabled}
              disabled={!isAdmin || busy}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            {t('启用本地地理库')}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              className="h-4 w-4 accent-primary"
              checked={auto}
              disabled={!isAdmin || busy}
              onChange={(e) => setAuto(e.target.checked)}
            />
            {t('库过期时自动更新')}
          </label>
          <span className="text-muted-foreground text-xs">
            {'{month}'} 会替换为当前月份，如 2026-10
          </span>
        </div>
        {isAdmin && (
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => void save()} disabled={busy}>
              <Save className="mr-1.5 h-4 w-4" />
              {t('保存配置')}
            </Button>
            <Button variant="outline" onClick={() => void refresh()} disabled={busy || !enabled}>
              <RefreshCw className={`mr-1.5 h-4 w-4 ${status?.downloading ? 'animate-spin' : ''}`} />
              {t('立即更新')}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function UsersPanel() {
  const { user } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('viewer')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await api.users()
      setUsers(res.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('加载用户失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const create = async () => {
    setBusy(true)
    setError('')
    try {
      await api.createUser(username.trim(), password, role)
      setOpen(false)
      setUsername('')
      setPassword('')
      setRole('viewer')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('创建用户失败'))
    } finally {
      setBusy(false)
    }
  }

  if (user?.role !== 'admin') {
    return <p className="text-sm text-muted-foreground">{t("仅管理员可管理用户。")}</p>
  }

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <div className="space-y-1">
          <CardTitle className="text-base">{t("用户管理")}</CardTitle>
          <CardDescription>{t("管理员可创建只读或管理账号")}</CardDescription>
        </div>
        <Button size="sm" onClick={() => setOpen(true)}>
          <UserPlus />
          {t('新建用户')}
        </Button>
      </CardHeader>
      <CardContent>
        {error && <p className="mb-3 text-xs text-destructive">{error}</p>}
        {loading ? (
          <Loading label={t("加载用户")} />
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("用户名")}</TableHead>
                  <TableHead>{t("角色")}</TableHead>
                  <TableHead>{t("创建时间")}</TableHead>
                  <TableHead>{t("最近登录")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.username}</TableCell>
                    <TableCell>
                      <Badge variant={item.role === 'admin' ? 'default' : 'secondary'}>
                        {item.role === 'admin' ? t('管理员') : t('只读')}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">{formatTime(item.createdAt)}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{formatTime(item.lastLoginAt)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("新建用户")}</DialogTitle>
            <DialogDescription>{t("用户名至少 3 位，密码至少 8 位")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="new-username">{t("用户名")}</Label>
              <Input id="new-username" value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new-user-password">{t("密码")}</Label>
              <Input
                id="new-user-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t("角色")}</Label>
              <Select value={role} onValueChange={setRole}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="viewer">{t("只读用户")}</SelectItem>
                  <SelectItem value="admin">{t("管理员")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={busy}>
              {t('取消')}
            </Button>
            <Button onClick={create} disabled={busy || username.trim().length < 3 || password.length < 8}>
              <Plus />
              {t('创建')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

export default function Settings() {
  const { user } = useAuth()

  return (
    <div className="space-y-5">
      <PageHeader title={t("系统设置")} description={t("账户安全、扫描引擎与用户管理")} />

      <Tabs defaultValue="account" className="space-y-4">
        <TabsList>
          <TabsTrigger value="account">{t("账户与密钥")}</TabsTrigger>
          <TabsTrigger value="scanner">{t("扫描引擎")}</TabsTrigger>
          {user?.role === 'admin' && <TabsTrigger value="users">{t("用户管理")}</TabsTrigger>}
        </TabsList>

        <TabsContent value="account">
          <AccountPanel />
        </TabsContent>
        <TabsContent value="scanner">
          <ScannerPanel />
        </TabsContent>
        {user?.role === 'admin' && (
          <TabsContent value="users">
            <UsersPanel />
          </TabsContent>
        )}
      </Tabs>
    </div>
  )
}
