import { Crosshair, Fingerprint, Radar, Search, TriangleAlert } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { LangToggle } from '@/components/LangToggle'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Spinner } from '@/components/shared'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/lib/auth'
import { t } from '@/lib/i18n'

const FEATURES = [
  { icon: Radar, title: '多引擎扫描', desc: '原生 TCP 探测、Nmap 服务识别、Masscan 高速发现' },
  { icon: Fingerprint, title: '原生指纹识别', desc: '50+ Web 指纹、TLS 证书、favicon 哈希与协议 banner' },
  { icon: Search, title: t('统一检索语法'), desc: 'app:nginx && country:CN && port:[80 TO 8080]' },
]

export default function Login() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (busy) return
    setError('')
    setBusy(true)
    try {
      await login(username.trim(), password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('登录失败'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="relative flex min-h-dvh flex-col overflow-y-auto bg-background">
      <div className="pointer-events-none absolute inset-0 grid-bg" />
      <div className="pointer-events-none absolute -top-40 left-1/2 size-[42rem] -translate-x-1/2 rounded-full bg-primary/10 blur-[140px]" />

      <header className="relative z-10 flex items-center justify-between px-4 py-4 sm:px-8">
        <div className="flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/12 text-primary ring-1 ring-primary/25">
            <Crosshair className="size-4" />
          </div>
          <div className="leading-tight">
            <p className="text-sm font-semibold tracking-wide">SCOPEX</p>
            <p className="text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Cyber asset map</p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <LangToggle />
          <ThemeToggle />
        </div>
      </header>

      <main className="relative z-10 mx-auto grid w-full max-w-5xl flex-1 items-center gap-10 px-4 py-8 sm:px-8 lg:grid-cols-[1.1fr_minmax(0,380px)]">
        <section className="hidden space-y-8 lg:block">
          <div className="space-y-3">
            <h1 className="text-3xl font-semibold leading-tight tracking-tight">{t("网络空间资产测绘引擎")}</h1>
            <p className="max-w-md text-sm leading-relaxed text-muted-foreground">
              {t('以任务化的方式发现、识别并持续追踪网络资产，用一套统一的检索语法完成检索与研判。')}
            </p>
          </div>

          <ul className="space-y-4">
            {FEATURES.map((item) => (
              <li key={t(item.title)} className="flex gap-3">
                <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-primary">
                  <item.icon className="size-4" />
                </div>
                <div className="space-y-0.5">
                  <p className="text-sm font-medium">{t(item.title)}</p>
                  <p className="text-xs leading-relaxed text-muted-foreground">{t(item.desc)}</p>
                </div>
              </li>
            ))}
          </ul>

          <p className="text-[11px] uppercase tracking-[0.2em] text-muted-foreground/70">
            Local deployment · data stays in your network
          </p>
        </section>

        <Card className="shadow-xl">
          <CardHeader>
            <CardTitle className="text-lg">{t("登录控制台")}</CardTitle>
            <CardDescription>{t("使用管理台账号进入 SCOPEX")}</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="username">{t("用户名")}</Label>
                <Input
                  id="username"
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="admin"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t("密码")}</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  required
                />
              </div>

              {error && (
                <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-xs text-destructive">
                  <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              <Button type="submit" className="w-full" disabled={busy}>
                {busy && <Spinner className="size-4" />}
                {t('进入控制台')}
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>

      <footer className="relative z-10 px-4 pb-5 text-center text-[11px] text-muted-foreground/70 sm:px-8">
        {t('首次启动可通过环境变量指定管理员密码：')}{' '}
        <code className="rounded bg-muted px-1 py-0.5 font-mono">SCOPEX_ADMIN_PASS</code>
      </footer>
    </div>
  )
}
