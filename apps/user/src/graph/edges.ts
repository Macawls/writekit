import type { GraphEdge } from './types'

const TOP_K_NEIGHBORS = 6
const MIN_SIMILARITY = 0.22

interface VectorEntry {
  pageId: string
  vec: Float32Array
}

export function computeEdges(input: VectorEntry[]): GraphEdge[] {
  if (input.length < 2) return []

  const entries = input.map(e => ({ pageId: e.pageId, vec: new Float32Array(e.vec) }))
  normalize(entries)
  center(entries)
  normalize(entries)

  const edges: GraphEdge[] = []
  const seen = new Set<string>()

  for (let i = 0; i < entries.length; i++) {
    const top: { idx: number; weight: number }[] = []
    for (let j = 0; j < entries.length; j++) {
      if (i === j) continue
      const sim = dot(entries[i].vec, entries[j].vec)
      if (sim < MIN_SIMILARITY) continue
      insertTop(top, { idx: j, weight: sim })
    }
    for (const nb of top) {
      const a = entries[i].pageId
      const b = entries[nb.idx].pageId
      const key = a < b ? `${a}\x00${b}` : `${b}\x00${a}`
      if (seen.has(key)) continue
      seen.add(key)
      edges.push({ source: a, target: b, weight: nb.weight })
    }
  }
  return edges
}

function center(entries: VectorEntry[]) {
  if (entries.length < 2) return
  const dims = entries[0].vec.length
  if (dims === 0) return
  const mean = new Float32Array(dims)
  for (const e of entries) {
    if (e.vec.length !== dims) return
    for (let j = 0; j < dims; j++) mean[j] += e.vec[j]
  }
  const inv = 1 / entries.length
  for (let j = 0; j < dims; j++) mean[j] *= inv
  for (const e of entries) {
    for (let j = 0; j < dims; j++) e.vec[j] -= mean[j]
  }
}

function normalize(entries: VectorEntry[]) {
  for (const e of entries) {
    let sum = 0
    for (let j = 0; j < e.vec.length; j++) sum += e.vec[j] * e.vec[j]
    if (sum === 0) continue
    const inv = 1 / Math.sqrt(sum)
    for (let j = 0; j < e.vec.length; j++) e.vec[j] *= inv
  }
}

function dot(a: Float32Array, b: Float32Array): number {
  const n = Math.min(a.length, b.length)
  let sum = 0
  for (let i = 0; i < n; i++) sum += a[i] * b[i]
  return sum
}

function insertTop(top: { idx: number; weight: number }[], cand: { idx: number; weight: number }) {
  let pos = top.length
  for (let i = 0; i < top.length; i++) {
    if (cand.weight > top[i].weight) { pos = i; break }
  }
  top.splice(pos, 0, cand)
  if (top.length > TOP_K_NEIGHBORS) top.length = TOP_K_NEIGHBORS
}
