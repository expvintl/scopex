export interface Facet {
  value: string
  count: number
}

export interface Service {
  name?: string
  product?: string
  version?: string
  extraInfo?: string
  osType?: string
  deviceType?: string
  method?: string
  confidence?: number
  banner?: string
}

export interface Favicon {
  md5: string
  mmh3: number
  url: string
  size: number
}

export interface TLSInfo {
  subject?: string
  issuer?: string
  commonName?: string
  sans?: string[]
  serial?: string
  version?: string
  cipher?: string
  notBefore?: string
  notAfter?: string
}

export interface Geo {
  countryCode?: string
  country?: string
  province?: string
  city?: string
  isp?: string
  latitude?: number
  longitude?: number
  private?: boolean
}

export interface ASN {
  number: number
  org: string
}

export interface Host {
  id: string
  ip: string
  ipNum: number
  port: number
  protocol: string
  service: Service
  banner: string
  title: string
  headers: Record<string, string>
  headerText: string
  statusCode: number
  favicon?: Favicon
  tls?: TLSInfo
  apps: string[]
  cpe: string[]
  device: string
  os: string
  hostnames: string[]
  domains: string[]
  geo: Geo
  asn?: ASN
  scripts?: Record<string, string>
  tags: string[]
  source: string
  firstSeen: string
  lastSeen: string
}

export interface TaskStats {
  hostsScanned: number
  openPorts: number
  newAssets: number
  updated: number
  errors: number
}

export interface Task {
  id: string
  name: string
  targets: string[]
  ports: string
  rate: number
  tool: string
  deep: boolean
  status: 'pending' | 'running' | 'done' | 'failed' | 'canceled' | 'interrupted'
  phase: string
  progress: number
  totalHosts: number
  stats: TaskStats
  error?: string
  createdBy: string
  createdAt: string
  startedAt?: string
  finishedAt?: string
  updatedAt: string
}

export interface TaskLog {
  id: string
  taskId: string
  seq: number
  level: string
  msg: string
  at: string
}

export interface Trend {
  date: string
  count: number
}

export interface Overview {
  totalAssets: number
  uniqueIPs: number
  new24h: number
  new7d: number
  privateHosts: number
  publicHosts: number
  topPorts: Facet[]
  topServices: Facet[]
  topCountries: Facet[]
  topApps: Facet[]
  trend: Trend[]
  taskCounts: Record<string, number>
  generatedAt: string
}

export interface Facets {
  ports: Facet[]
  services: Facet[]
  countries: Facet[]
  apps: Facet[]
}

export interface ToolStatus {
  name: string
  path: string
  available: boolean
  version?: string
  reason?: string
  purpose: string
}

export interface ToolInfo {
  tools: ToolStatus[]
  limits: { maxTargets: number; maxRate: number; denyCIDRs: string[] }
  geo: GeoLibraryStatus
}

export interface FieldDoc {
  name: string
  type: string
  desc: string
  example: string
}

export interface User {
  id: string
  username: string
  role: string
  apiKey?: string
  createdAt: string
  lastLoginAt?: string
}

export interface PortBrief {
  port: number
  protocol: string
  service: string
  product: string
  title: string
  statusCode: number
  apps: string[]
}

export interface IPGroup {
  ip: string
  ipNum: number
  geo: Geo
  hostnames: string[]
  domains: string[]
  portCount: number
  ports: PortBrief[]
  firstSeen: string
  lastSeen: string
}

export interface IPGroupPage {
  data: IPGroup[]
  total: number
  page: number
  size: number
}

export interface HostPage {
  data: Host[]
  total: number
  page: number
  size: number
}

export interface TaskPage {
  data: Task[]
  total: number
  page: number
  size: number
}

export interface ScanInterface {
  name: string
  mac?: string
  ipv4?: string
  addresses: string[]
  up: boolean
  loopback: boolean
  default: boolean
  gateway?: string
  gatewayMac?: string
}

export interface ScanRoute {
  interface: string
  gateway?: string
  gatewayMac?: string
}

export interface ScanSettings {
  scanInterface: string
  masscanInterface: string
  masscanRouterMac: string
  masscanExtraArgs: string
  defaultPorts: string
  denyCidrs: string[]
}

export interface ScanSettingsPayload {
  settings: ScanSettings
  interfaces: ScanInterface[]
  defaultRoute: ScanRoute
}

export interface GeoLibraryStatus {
  enabled: boolean
  ready: boolean
  cityDate: string
  asnReady: boolean
  downloading: boolean
  lastError?: string
  updatedAt: string
  dir: string
}

export interface GeoSettings {
  enabled: boolean
  autoDownload: boolean
  cityUrl: string
  asnUrl: string
}

export interface GeoSettingsPayload {
  settings: GeoSettings
  status: GeoLibraryStatus
}
