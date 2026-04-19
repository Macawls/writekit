export interface User {
  id: string
  email: string
  name: string
  avatar_url: string
}

export interface Site {
  ID: string
  UserID: string
  Name: string
  CreatedAt: string
}

export interface Subscription {
  ID: string
  UserID: string
  StripeCustomerID: string
  Status: string
}

export interface MeResponse {
  user: User
  site: Site | null
  subscription: Subscription | null
  role: string
}

export interface TeamMember {
  user_id: string
  email: string
  name: string
  avatar_url: string
  role: string
  created_at: string
}

export interface LocalInfo {
  port: number
  data_dir: string
  mcp_url: string
}

export interface ClientInfo {
  id: string
  name: string
  detected: boolean
  connected: boolean
  config_path: string
  supports_http: boolean
  requires_npx: boolean
}

export interface DesktopSettings {
  autostart: boolean
  close_to_tray: boolean
  start_minimized: boolean
  data_dir: string
  effective_data_dir: string
  onboarding_complete: boolean
  needs_restart?: boolean
}

export interface PickFolderResult {
  path: string
  has_existing_data?: boolean
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
  }
  if (body) {
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  if (res.status === 401) {
    const host = window.location.hostname.replace(/^app\./, '')
    window.location.href = `${window.location.protocol}//${host}/auth/login`
    throw new Error('unauthorized')
  }
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || 'request failed')
  }
  return data as T
}

export const api = {
  me: () => request<MeResponse>('GET', '/api/me'),
  createSite: (slug: string, name: string) => request<Site>('POST', '/api/site', { slug, name }),
  updateSlug: (slug: string) => request<Site>('PUT', '/api/site/slug', { slug }),
  updateProfile: (name: string) => request<{ name: string }>('PUT', '/api/me', { name }),
  billingCheckout: () => request<{ url: string }>('POST', '/api/billing/checkout'),
  billingPortal: () => request<{ url: string }>('POST', '/api/billing/portal'),
  listTeamMembers: () => request<TeamMember[]>('GET', '/api/team'),
  inviteTeamMember: (email: string, role: string) => request<{ status: string }>('POST', '/api/team', { email, role }),
  updateTeamMemberRole: (userId: string, role: string) => request<{ status: string }>('PUT', `/api/team/${userId}`, { role }),
  removeTeamMember: (userId: string) => request<{ status: string }>('DELETE', `/api/team/${userId}`),
  localInfo: () => request<LocalInfo>('GET', '/api/local/info'),
  listClients: () => request<ClientInfo[]>('GET', '/api/local/clients'),
  connectClient: (id: string) => request<{ status: string; needs_restart: boolean }>('POST', `/api/local/clients/${id}/connect`),
  disconnectClient: (id: string) => request<{ status: string }>('POST', `/api/local/clients/${id}/disconnect`),
  getDesktopSettings: () => request<DesktopSettings>('GET', '/api/local/settings'),
  updateDesktopSettings: (s: DesktopSettings) => request<DesktopSettings>('PUT', '/api/local/settings', s),
  pickDataFolder: () => request<PickFolderResult>('POST', '/api/local/pick-folder'),
  dbTables: () => request<DBTable[]>('GET', '/api/db/tables'),
  dbTableRows: (name: string, limit: number, offset: number) =>
    request<DBTableRows>('GET', `/api/db/tables/${encodeURIComponent(name)}?limit=${limit}&offset=${offset}`),
  dbQuery: (sql: string) => request<DBQueryResult>('POST', '/api/db/query', { sql }),
}

export interface DBTable {
  name: string
  rows: number
  columns: number
  type: string
}

export interface DBColumnInfo {
  name: string
  type: string
  not_null: boolean
  pk: boolean
}

export interface DBTableRows {
  columns: string[]
  types: string[]
  rows: unknown[][]
  total: number
  limit: number
  offset: number
  schema?: DBColumnInfo[]
}

export interface DBQueryResult {
  columns: string[]
  rows: unknown[][]
  truncated: boolean
}
