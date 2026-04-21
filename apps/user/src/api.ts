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

export interface TeamInvitation {
  id: string
  email: string
  role: string
  inviter_name: string
  expires_at: string
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
  const text = await res.text()
  let data: any
  try {
    data = text === '' ? null : JSON.parse(text)
  } catch (e) {
    const preview = text.slice(0, 120).replace(/\s+/g, ' ')
    throw new Error(`bad JSON from ${method} ${path} (status ${res.status}): ${preview}`)
  }
  if (!res.ok) {
    throw new Error(data?.error || `request failed (${res.status})`)
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
  inviteTeamMember: (email: string, role: string) => request<TeamInvitation>('POST', '/api/team', { email, role }),
  updateTeamMemberRole: (userId: string, role: string) => request<{ status: string }>('PUT', `/api/team/${userId}`, { role }),
  removeTeamMember: (userId: string) => request<{ status: string }>('DELETE', `/api/team/${userId}`),
  listInvitations: () => request<TeamInvitation[]>('GET', '/api/team/invitations'),
  revokeInvitation: (id: string) => request<{ status: string }>('DELETE', `/api/team/invitations/${id}`),
  resendInvitation: (id: string) => request<TeamInvitation>('POST', `/api/team/invitations/${id}/resend`),
  localInfo: () => request<LocalInfo>('GET', '/api/local/info'),
  listClients: () => request<ClientInfo[]>('GET', '/api/local/clients'),
  connectClient: (id: string) => request<{ status: string; needs_restart: boolean }>('POST', `/api/local/clients/${id}/connect`),
  disconnectClient: (id: string) => request<{ status: string }>('POST', `/api/local/clients/${id}/disconnect`),
  getDesktopSettings: () => request<DesktopSettings>('GET', '/api/local/settings'),
  updateDesktopSettings: (s: DesktopSettings) => request<DesktopSettings>('PUT', '/api/local/settings', s),
  pickDataFolder: () => request<PickFolderResult>('POST', '/api/local/pick-folder'),
  listPages: (params: { limit?: number; offset?: number; status?: string; collection?: string; visibility?: string; q?: string } = {}) => {
    const qs = new URLSearchParams()
    if (params.limit) qs.set('limit', String(params.limit))
    if (params.offset) qs.set('offset', String(params.offset))
    if (params.status && params.status !== 'all') qs.set('status', params.status)
    if (params.collection && params.collection !== 'all') qs.set('collection', params.collection)
    if (params.visibility && params.visibility !== 'all') qs.set('visibility', params.visibility)
    if (params.q && params.q.trim()) qs.set('q', params.q.trim())
    const q = qs.toString()
    return request<PageListResponse>('GET', q ? `/api/pages?${q}` : '/api/pages')
  },
  dbTables: () => request<DBTable[]>('GET', '/api/db/tables'),
  dbTableRows: (name: string, limit: number, offset: number, sort?: string, dir?: 'asc' | 'desc') => {
    const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (sort) qs.set('sort', sort)
    if (dir) qs.set('dir', dir)
    return request<DBTableRows>('GET', `/api/db/tables/${encodeURIComponent(name)}?${qs}`)
  },
  dbTableSchema: (name: string) => request<DBSchema>('GET', `/api/db/tables/${encodeURIComponent(name)}/schema`),
  dbQuery: (sql: string) => request<DBQueryResult>('POST', '/api/db/query', { sql }),
}

export interface PageListItem {
  id: string
  title: string
  slug: string
  status: string
  visibility: string
  collection_id?: string | null
  updated_at: string
  published_at?: string | null
}

export interface CollectionLight {
  id: string
  title: string
  slug: string
}

export interface PageListResponse {
  pages: PageListItem[]
  collections: CollectionLight[]
  total: number
  limit: number
  offset: number
}

export interface DBSchema {
  name: string
  type: string
  create_sql: string
  columns: DBColumnInfo[]
  indexes: DBIndexInfo[]
  foreign_keys: DBFKInfo[]
  row_count: number
}

export interface DBIndexInfo {
  name: string
  unique: boolean
  partial: boolean
  origin: string
  columns: string[]
}

export interface DBFKInfo {
  from: string
  table: string
  to: string
  on_update: string
  on_delete: string
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
