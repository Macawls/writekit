import { atom, computed } from 'nanostores'
import { fetchGraph, fetchEmbeddingSource } from '../api/graph'
import type { GraphEdge, GraphNode, GraphResponse, Visibility } from '../graph/types'
import { computeInsights } from '../graph/insights'
import { computeEdges } from '../graph/edges'
import { embeddingController, $vectorsTick } from '../embedding/controller'
import { $embeddingPrefs } from '../embedding/settings'

const STANDALONE_KEY = '__standalone__'

export interface RelatedEntry {
  node: GraphNode
  weight: number
}

export const $graphData = atom<GraphResponse | null>(null)
export const $loading = atom<boolean>(true)
export const $error = atom<string | null>(null)
export const $focused = atom<GraphNode | null>(null)
export const $hoverId = atom<string | null>(null)
export const $excludedVis = atom<Set<Visibility>>(new Set())
export const $excludedCols = atom<Set<string>>(new Set())
export const $view3D = atom<boolean>(false)

export const $edges = atom<GraphEdge[]>([])

async function recomputeEdges() {
  const vectors = await embeddingController.getVectors()
  $edges.set(computeEdges(vectors.map(v => ({ pageId: v.pageId, vec: v.vec }))))
}

$vectorsTick.subscribe(() => { recomputeEdges() })

export const $visibleData = computed(
  [$graphData, $edges, $excludedVis, $excludedCols],
  (data, edges, exVis, exCols) => {
    if (!data) return null
    const nodes = data.nodes.filter(n =>
      !exVis.has(n.visibility) &&
      !exCols.has(n.collection_id ?? STANDALONE_KEY))
    const ids = new Set(nodes.map(n => n.id))
    const filteredEdges = edges.filter(e => ids.has(e.source) && ids.has(e.target))
    return { nodes, edges: filteredEdges, collections: data.collections }
  },
)

export const $hiddenNodeIds = computed(
  [$graphData, $visibleData],
  (data, vis) => {
    if (!data || !vis) return new Set<string>()
    const visibleIds = new Set(vis.nodes.map(n => n.id))
    const hidden = new Set<string>()
    for (const n of data.nodes) if (!visibleIds.has(n.id)) hidden.add(n.id)
    return hidden
  },
)

export const $neighborIndex = computed($visibleData, vis => {
  const index = new Map<string, RelatedEntry[]>()
  if (!vis) return index
  const byId = new Map<string, GraphNode>()
  for (const n of vis.nodes) byId.set(n.id, n)
  const push = (from: string, to: string, w: number) => {
    const target = byId.get(to)
    if (!target) return
    let list = index.get(from)
    if (!list) { list = []; index.set(from, list) }
    list.push({ node: target, weight: w })
  }
  for (const e of vis.edges) {
    push(e.source, e.target, e.weight)
    push(e.target, e.source, e.weight)
  }
  for (const list of index.values()) list.sort((a, b) => b.weight - a.weight)
  return index
})

export const $insights = computed($visibleData, vis => vis ? computeInsights(vis) : null)

export async function loadGraph(tenantId: string) {
  $error.set(null)
  try {
    const d = await fetchGraph()
    $graphData.set(d)
    $loading.set(false)
    await recomputeEdges()

    const prefs = $embeddingPrefs.get()
    if (prefs.enabled) {
      await embeddingController.start(tenantId, prefs.modelId)
      const sources = await fetchEmbeddingSource()
      embeddingController.syncPages(sources)
    }
  } catch (e: unknown) {
    $error.set(e instanceof Error ? e.message : 'failed to load')
    $loading.set(false)
  }
}

export function stopGraphPolling() {
  // retained for backward compat; no-op now that backfill polling is gone
}

export function toggleVis(v: Visibility) {
  const next = new Set($excludedVis.get())
  if (next.has(v)) next.delete(v); else next.add(v)
  $excludedVis.set(next)
}

export function toggleCol(id: string) {
  const next = new Set($excludedCols.get())
  if (next.has(id)) next.delete(id); else next.add(id)
  $excludedCols.set(next)
}

export function focusNode(n: GraphNode | null) {
  $focused.set(n)
}

export function setHover(id: string | null) {
  $hoverId.set(id)
}

export function toggleView3D() {
  $view3D.set(!$view3D.get())
}

export { STANDALONE_KEY }
