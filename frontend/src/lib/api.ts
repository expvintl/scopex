import { langHeader, t } from './i18n'
import type {
  Facets,
  FieldDoc,
  GeoLibraryStatus,
  GeoSettings,
  GeoSettingsPayload,
  Host,
  HostPage,
  IPGroupPage,
  Overview,
  ScanSettings,
  ScanSettingsPayload,
  Task,
  TaskLog,
  TaskPage,
  ToolInfo,
  User,
} from './types'

const TOKEN_KEY = 'scopex.token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Accept-Language': langHeader(),
    ...((init?.headers as Record<string, string>) ?? {}),
  }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`/api${path}`, { ...init, headers })
  const text = await res.text()
  let body: any = {}
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = { error: text }
    }
  }
  if (res.status === 401 && !path.startsWith('/auth/login')) {
    setToken(null)
    window.dispatchEvent(new Event('scopex:unauthorized'))
  }
  if (!res.ok) {
    throw new ApiError(t(body?.error ?? '请求失败（{status}）', { status: res.status }), res.status)
  }
  return body as T
}

function qs(params: Record<string, string | number | boolean | undefined | null>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') continue
    sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}

export const api = {
  login: (username: string, password: string) =>
    request<{ data: { token: string; expiresAt: string; user: User } }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  me: () => request<{ data: User }>('/auth/me'),

  overview: () => request<{ data: Overview }>('/stats/overview'),

  tools: () => request<{ data: ToolInfo }>('/system/tools'),

  scanSettings: () => request<{ data: ScanSettingsPayload }>('/system/scan-settings'),

  saveScanSettings: (body: ScanSettings) =>
    request<{ data: ScanSettings }>('/system/scan-settings', {
      method: 'PUT',
      body: JSON.stringify(body),
    }),

  geo: () => request<{ data: GeoSettingsPayload }>('/system/geo'),

  saveGeo: (body: GeoSettings) =>
    request<{ data: GeoSettingsPayload }>('/system/geo', {
      method: 'PUT',
      body: JSON.stringify(body),
    }),

  refreshGeo: () =>
    request<{ data: { started: boolean; status: GeoLibraryStatus } }>('/system/geo/refresh', {
      method: 'POST',
    }),

  querySyntax: () => request<{ data: FieldDoc[] }>('/meta/query-syntax'),

  hosts: (params: {
    q?: string
    page?: number
    size?: number
    sort?: string
    desc?: boolean
  }) => request<HostPage>(`/hosts${qs(params)}`),

  ipGroups: (params: { q?: string; page?: number; size?: number; sort?: string }) =>
    request<IPGroupPage>(`/hosts${qs({ ...params, group: 'ip' })}`),

  ipHosts: (ip: string) => request<{ data: Host[]; total: number }>(`/ips/${encodeURIComponent(ip)}`),

  host: (id: string) => request<{ data: Host }>(`/hosts/${id}`),

  refreshIP: (ip: string) =>
    request<{ data: { ip: string; refreshed: number; partial: boolean } }>(
      `/ips/${encodeURIComponent(ip)}/refresh`,
      { method: 'POST' },
    ),
  deleteHost: (id: string) => request<{ data: { deleted: string } }>(`/hosts/${id}`, { method: 'DELETE' }),

  deleteHostsByIP: (ip: string) =>
    request<{ data: { ip: string; deleted: number } }>(`/hosts${qs({ ip })}`, { method: 'DELETE' }),

  deleteHostsBulk: (params: { cidr?: string; pattern?: string }) =>
    request<{ data: { mode: string; value: string; deleted: number } }>(`/hosts${qs(params)}`, {
      method: 'DELETE',
    }),

  clearTasks: () =>
    request<{ data: { deleted: number } }>('/tasks', { method: 'DELETE' }),

  facets: (q?: string) => request<{ data: Facets }>(`/hosts/facets${qs({ q })}`),

  tasks: (params: { status?: string; page?: number; size?: number }) =>
    request<TaskPage>(`/tasks${qs(params)}`),

  task: (id: string) => request<{ data: { task: Task; running: boolean } }>(`/tasks/${id}`),

  createTask: (body: {
    name?: string
    targets: string[]
    ports?: string
    rate?: number
    tool?: string
    deep?: boolean
  }) => request<{ data: Task }>('/tasks', { method: 'POST', body: JSON.stringify(body) }),

  cancelTask: (id: string) => request<{ data: { message: string } }>(`/tasks/${id}/cancel`, { method: 'POST' }),

  deleteTask: (id: string) => request<{ data: { message: string } }>(`/tasks/${id}`, { method: 'DELETE' }),

  taskLogs: (id: string, after = 0) =>
    request<{ data: { logs: TaskLog[]; last: number } }>(`/tasks/${id}/logs${qs({ after })}`),

  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ data: { message: string; token: string; expiresAt: string } }>('/auth/password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),

  rotateAPIKey: () => request<{ data: { apiKey: string } }>('/auth/apikey', { method: 'POST' }),

  users: () => request<{ data: User[] }>('/users'),

  createUser: (username: string, password: string, role: string) =>
    request<{ data: User }>('/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    }),

  exportUrl: (q: string, format: 'csv' | 'json') =>
    `/api/hosts/export${qs({ q, format, token: getToken() })}`,

  faviconUrl: (id: string) => `/api/hosts/${id}/favicon${qs({ token: getToken() })}`,
}
