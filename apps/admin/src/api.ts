export interface User {
  ID: string
  Email: string
  Name: string
  AvatarURL: string
  CreatedAt: string
  UpdatedAt: string
}

export interface Tenant {
  ID: string
  UserID: string
  Name: string
  CreatedAt: string
}

export interface LinkedAccount {
  ID: string
  Provider: string
  ProviderID: string
  Email: string
  EmailVerified: boolean
}

export interface Subscription {
  ID: string
  Status: string
  StripeCustomerID: string
  CurrentPeriodEnd: string | null
}

export interface TenantStorage {
  id: string
  name: string
  bytes: number
}

export interface Stats {
  total_users: number
  total_tenants: number
  active_subscriptions: number
  recent_users: User[]
  total_storage_bytes: number
  tenant_storage: TenantStorage[]
}

export interface UsersResponse {
  users: User[]
  total: number
  page: number
  per_page: number
}

export interface UserDetail {
  user: User
  tenant: Tenant | null
  linked_accounts: LinkedAccount[]
  subscription: Subscription | null
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
  }
  if (body) opts.body = JSON.stringify(body)

  const res = await fetch(path, opts)
  if (res.status === 401) throw new Error('unauthorized')

  if (method === 'DELETE' && res.ok) return {} as T

  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'request failed')
  return data as T
}

export const adminApi = {
  me: () => request<{ email: string }>('GET', '/admin/api/me'),
  sendLink: (email: string) => request<{ status: string }>('POST', '/admin/api/auth/send', { email }),
  logout: () => request<{ status: string }>('POST', '/admin/api/auth/logout'),
  stats: () => request<Stats>('GET', '/admin/api/stats'),
  listUsers: (page: number, q?: string) =>
    request<UsersResponse>('GET', `/admin/api/users?page=${page}${q ? `&q=${encodeURIComponent(q)}` : ''}`),
  getUser: (id: string) => request<UserDetail>('GET', `/admin/api/users/${id}`),
  deleteUser: (id: string) => request<void>('DELETE', `/admin/api/users/${id}`),
}
