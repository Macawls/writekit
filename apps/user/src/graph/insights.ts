import type { GraphNode, GraphView } from './types'

const ANCHOR_LIMIT = 3
const ORPHAN_LIMIT = 5

export interface AnchorEntry {
  node: GraphNode
  degree: number
}

export interface GraphInsights {
  headline: string
  anchors: AnchorEntry[]
  orphans: GraphNode[]
  orphanOverflow: number
}

export function computeInsights(data: GraphView): GraphInsights {
  const degree = new Map<string, number>()
  for (const e of data.edges) {
    degree.set(e.source, (degree.get(e.source) ?? 0) + 1)
    degree.set(e.target, (degree.get(e.target) ?? 0) + 1)
  }

  const anchors = data.nodes
    .map(n => ({ node: n, degree: degree.get(n.id) ?? 0 }))
    .filter(a => a.degree > 0)
    .sort((a, b) => b.degree - a.degree)
    .slice(0, ANCHOR_LIMIT)

  const allOrphans = data.nodes.filter(n => (degree.get(n.id) ?? 0) === 0)
  const orphans = allOrphans.slice(0, ORPHAN_LIMIT)
  const orphanOverflow = allOrphans.length - orphans.length

  return {
    headline: buildHeadline(data, allOrphans.length, anchors.length),
    anchors,
    orphans,
    orphanOverflow,
  }
}

function buildHeadline(data: { nodes: GraphNode[]; edges: { source: string; target: string }[] }, orphanCount: number, anchorCount: number): string {
  const n = data.nodes.length
  if (n === 0) return 'No published pages yet'
  if (data.edges.length === 0) {
    return `${n} ${n === 1 ? 'page' : 'pages'} · none are related yet`
  }
  if (anchorCount === 0) {
    return `${n} pages with loose connections`
  }
  if (orphanCount === 0) {
    return `${n} pages · everything is connected`
  }
  const connected = n - orphanCount
  return `${connected} of ${n} pages are connected`
}
