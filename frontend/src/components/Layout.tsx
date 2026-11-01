import {
  Crosshair,
  LayoutDashboard,
  LogOut,
  Menu,
  Radar,
  Search,
  Settings,
  ShieldCheck,
  X,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'

import { ThemeToggle } from '@/components/ThemeToggle'
import { Badge } from '@/components/ui/badge'
import { LangToggle } from '@/components/LangToggle'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import { useAuth } from '@/lib/auth'
import { cn } from '@/lib/utils'
import { t } from '@/lib/i18n'

const NAV = [
  { to: '/', label: '态势总览', icon: LayoutDashboard, end: true },
  { to: '/search', label: '资产搜索', icon: Search },
  { to: '/tasks', label: '扫描任务', icon: Radar },
  { to: '/settings', label: '系统设置', icon: Settings },
]

const TITLES: Record<string, string> = {
  '/': '态势总览',
  '/search': '资产搜索',
  '/tasks': '扫描任务',
  '/settings': '系统设置',
}

export function Layout() {
  const [open, setOpen] = useState(false)
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    setOpen(false)
  }, [location.pathname])

  const title = t(TITLES[location.pathname] ?? 'SCOPEX')

  return (
    <div className="flex h-dvh overflow-hidden bg-background">
      {open && (
        <div
          className="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm lg:hidden"
          onClick={() => setOpen(false)}
          aria-hidden
        />
      )}

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-transform duration-200 lg:static lg:w-60 lg:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-14 shrink-0 items-center gap-2.5 px-4">
          <div className="relative flex size-8 items-center justify-center rounded-lg bg-primary/12 text-primary ring-1 ring-primary/25">
            <Crosshair className="size-4" />
          </div>
          <div className="min-w-0 leading-tight">
            <p className="truncate text-sm font-semibold tracking-wide">SCOPEX</p>
            <p className="truncate text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Cyber asset map</p>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            className="ml-auto lg:hidden"
            onClick={() => setOpen(false)}
            aria-label={t("关闭导航")}
          >
            <X />
          </Button>
        </div>

        <Separator />

        <nav className="flex-1 space-y-1 overflow-y-auto p-2.5">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground shadow-xs'
                    : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground',
                )
              }
            >
              <item.icon className="size-4" />
              {t(item.label)}
            </NavLink>
          ))}
        </nav>

        <div className="space-y-2 p-2.5">
          <div className="rounded-lg border border-sidebar-border bg-background/40 p-2.5">
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span className="size-1.5 rounded-full bg-ok pulse-dot" />
              {t('本地部署 · 数据不出域')}
            </div>
            <p className="mt-1 text-[10px] tracking-wide text-muted-foreground/70">v0.1.0</p>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border bg-background/80 px-3 backdrop-blur sm:gap-3 sm:px-6">
          <Button
            variant="ghost"
            size="icon-sm"
            className="lg:hidden"
            onClick={() => setOpen(true)}
            aria-label={t("打开导航")}
          >
            <Menu />
          </Button>

          <div className="flex min-w-0 items-center gap-2 text-sm">
            <span className="hidden text-muted-foreground sm:inline">{t("控制台")}</span>
            <span className="hidden text-muted-foreground/40 sm:inline">/</span>
            <span className="truncate font-medium">{title}</span>
          </div>

          <div className="ml-auto flex items-center gap-1">
            <LangToggle />
            <ThemeToggle />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="gap-2 px-1.5 sm:px-2">
                  <span className="flex size-6 items-center justify-center rounded-full bg-primary/12 text-[11px] font-semibold text-primary">
                    {(user?.username ?? '?').slice(0, 1).toUpperCase()}
                  </span>
                  <span className="hidden max-w-28 truncate sm:inline">{user?.username}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-52">
                <DropdownMenuLabel className="flex items-center justify-between gap-2">
                  <span className="truncate">{user?.username}</span>
                  <Badge variant={user?.role === 'admin' ? 'default' : 'secondary'}>
                    {user?.role === 'admin' ? t('管理员') : t('只读')}
                  </Badge>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => navigate('/settings')}>
                  <ShieldCheck />
                  {t('账户与设置')}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" onSelect={() => void logout()}>
                  <LogOut />
                  {t('退出登录')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        <main className="flex-1 overflow-y-auto p-4 sm:p-6">
          <div className="mx-auto w-full max-w-[1400px]">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
