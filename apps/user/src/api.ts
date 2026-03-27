export interface User {
  id: string
  email: string
  name: string
  avatar_url: string
}

export interface Blog {
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
  blog: Blog | null
  subscription: Subscription | null
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
    // Redirect to login on the main domain
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
  createBlog: (slug: string, name: string) => request<Blog>('POST', '/api/blog', { slug, name }),
  updateSlug: (slug: string) => request<Blog>('PUT', '/api/blog/slug', { slug }),
  updateProfile: (name: string) => request<{ name: string }>('PUT', '/api/me', { name }),
  billingCheckout: () => request<{ url: string }>('POST', '/api/billing/checkout'),
  billingPortal: () => request<{ url: string }>('POST', '/api/billing/portal'),
}
