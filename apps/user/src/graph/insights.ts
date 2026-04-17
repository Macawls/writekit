import type { GraphEdge, GraphNode, GraphResponse } from './types'

const STRONG_THRESHOLD = 0.7
const HUB_LIMIT = 5
const PAIR_LIMIT = 5
const CLUSTER_LIMIT = 8

export interface Cluster {
  id: number
  nodes: GraphNode[]
}

export interface HubEntry {
  node: GraphNode
  degree: number
}

export interface PairEntry {
  a: GraphNode
  b: GraphNode
  weight: number
}

export interface GraphInsights {
  avgSimilarity: number
  embeddedPct: number
  clusters: Cluster[]
  hubs: HubEntry[]
  orphans: GraphNode[]
  strongestPairs: PairEntry[]
  untaggedCount: number
}

export function computeInsights(data: GraphResponse): GraphInsights {
  const byId = new Map<string, GraphNode>()
  data.nodes.forEach(n => byId.set(n.id, n))

  const degree = new Map<string, number>()
  let weightSum = 0
  for (const e of data.edges) {
    degree.set(e.source, (degree.get(e.source) ?? 0) + 1)
    degree.set(e.target, (degree.get(e.target) ?? 0) + 1)
    weightSum += e.weight
  }
  const avgSimilarity = data.edges.length > 0 ? weightSum / data.edges.length : 0

  const hubs = data.nodes
    .map(n => ({ node: n, degree: degree.get(n.id) ?? 0 }))
    .filter(h => h.degree > 0)
    .sort((a, b) => b.degree - a.degree)
    .slice(0, HUB_LIMIT)

  const orphans = data.nodes.filter(n => (degree.get(n.id) ?? 0) === 0)

  const strongestPairs = [...data.edges]
    .sort((a, b) => b.weight - a.weight)
    .slice(0, PAIR_LIMIT)
    .map(e => ({ a: byId.get(e.source)!, b: byId.get(e.target)!, weight: e.weight }))
    .filter(p => p.a && p.b)

  const clusters = findClusters(data.nodes, data.edges)
  const untaggedCount = data.nodes.filter(n => n.tags.length === 0).length

  const embeddedPct = data.total_page_count > 0
    ? Math.round((data.embedded_count / data.total_page_count) * 100)
    : 0

  return {
    avgSimilarity,
    embeddedPct,
    clusters: clusters.slice(0, CLUSTER_LIMIT),
    hubs,
    orphans,
    strongestPairs,
    untaggedCount,
  }
}

function findClusters(nodes: GraphNode[], edges: GraphEdge[]): Cluster[] {
  const parent = new Map<string, string>()
  nodes.forEach(n => parent.set(n.id, n.id))

  const find = (x: string): string => {
    let r = x
    while (parent.get(r)! !== r) r = parent.get(r)!
    let cur = x
    while (parent.get(cur)! !== cur) {
      const nxt = parent.get(cur)!
      parent.set(cur, r)
      cur = nxt
    }
    return r
  }
  const union = (a: string, b: string) => {
    const ra = find(a), rb = find(b)
    if (ra !== rb) parent.set(ra, rb)
  }

  for (const e of edges) {
    if (e.weight < STRONG_THRESHOLD) continue
    if (parent.has(e.source) && parent.has(e.target)) union(e.source, e.target)
  }

  const groups = new Map<string, GraphNode[]>()
  for (const n of nodes) {
    if ((parent.get(n.id) ?? n.id) === n.id && !edgeTouches(n.id, edges, STRONG_THRESHOLD)) continue
    const root = find(n.id)
    let list = groups.get(root)
    if (!list) { list = []; groups.set(root, list) }
    list.push(n)
  }

  let idx = 0
  return [...groups.values()]
    .filter(list => list.length >= 2)
    .sort((a, b) => b.length - a.length)
    .map(list => ({ id: idx++, nodes: list }))
}

function edgeTouches(id: string, edges: GraphEdge[], min: number): boolean {
  for (const e of edges) {
    if (e.weight < min) continue
    if (e.source === id || e.target === id) return true
  }
  return false
}

export function formatPercent(x: number): string {
  return `${Math.round(x * 100)}%`
}
