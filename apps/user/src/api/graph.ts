import type { GraphResponse } from '../graph/types'

export async function fetchGraph(): Promise<GraphResponse> {
  const res = await authFetch('/api/graph')
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'failed to load graph')
  return data as GraphResponse
}

export interface EmbeddingSourceItem {
  id: string
  text: string
}

export async function fetchEmbeddingSource(): Promise<EmbeddingSourceItem[]> {
  const res = await authFetch('/api/embedding-source')
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'failed to load embedding source')
  return data as EmbeddingSourceItem[]
}

async function authFetch(path: string) {
  const res = await fetch(path, { credentials: 'include' })
  if (res.status === 401) {
    const host = window.location.hostname.replace(/^app\./, '')
    window.location.href = `${window.location.protocol}//${host}/auth/login`
    throw new Error('unauthorized')
  }
  return res
}
