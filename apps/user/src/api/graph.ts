import type { GraphResponse } from '../graph/types'

export async function fetchGraph(): Promise<GraphResponse> {
  const res = await fetch('/api/graph', { credentials: 'include' })
  if (res.status === 401) {
    const host = window.location.hostname.replace(/^app\./, '')
    window.location.href = `${window.location.protocol}//${host}/auth/login`
    throw new Error('unauthorized')
  }
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || 'failed to load graph')
  }
  return data as GraphResponse
}
