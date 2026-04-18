import { useEffect, useMemo, useRef, useState } from 'react'
import { fetchGraph } from '../api/graph'
import type { GraphResponse, GraphNode, GraphEdge, Visibility } from '../graph/types'
import { GraphRenderer } from '../graph/renderer'
import { computeInsights, type GraphInsights } from '../graph/insights'

const STANDALONE_KEY = '__standalone__'
const ALL_VISIBILITIES: Visibility[] = ['public', 'unlisted', 'private']

const BACKFILL_POLL_MS = 5000
const ZOOM_IN = 1.3
const ZOOM_OUT = 1 / 1.3
const RELATED_LIMIT = 8

interface RelatedEntry {
  node: GraphNode
  weight: number
}

function buildNeighborIndex(data: GraphResponse): Map<string, RelatedEntry[]> {
  const byId = new Map<string, GraphNode>()
  for (const n of data.nodes) byId.set(n.id, n)
  const index = new Map<string, RelatedEntry[]>()
  const push = (from: string, to: string, w: number) => {
    const target = byId.get(to)
    if (!target) return
    let list = index.get(from)
    if (!list) { list = []; index.set(from, list) }
    list.push({ node: target, weight: w })
  }
  for (const e of data.edges) {
    push(e.source, e.target, e.weight)
    push(e.target, e.source, e.weight)
  }
  for (const list of index.values()) list.sort((a, b) => b.weight - a.weight)
  return index
}

export default function Graph() {
  const hostRef = useRef<HTMLDivElement>(null)
  const [data, setData] = useState<GraphResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [focused, setFocused] = useState<GraphNode | null>(null)
  const [hoverId, setHoverId] = useState<string | null>(null)

  const rendererRef = useRef<GraphRenderer | null>(null)
  const nodeCountRef = useRef(0)

  const [excludedVis, setExcludedVis] = useState<Set<Visibility>>(new Set())
  const [excludedCols, setExcludedCols] = useState<Set<string>>(new Set())

  const visibleData = useMemo<GraphResponse | null>(() => {
    if (!data) return null
    const visibleNodes = data.nodes.filter(n =>
      !excludedVis.has(n.visibility) &&
      !excludedCols.has(n.collection_id ?? STANDALONE_KEY))
    const visibleIds = new Set(visibleNodes.map(n => n.id))
    const visibleEdges: GraphEdge[] = data.edges.filter(e =>
      visibleIds.has(e.source) && visibleIds.has(e.target))
    return { ...data, nodes: visibleNodes, edges: visibleEdges }
  }, [data, excludedVis, excludedCols])

  const hiddenNodeIds = useMemo<Set<string>>(() => {
    if (!data || !visibleData) return new Set()
    const visible = new Set(visibleData.nodes.map(n => n.id))
    const hidden = new Set<string>()
    for (const n of data.nodes) if (!visible.has(n.id)) hidden.add(n.id)
    return hidden
  }, [data, visibleData])

  const neighborIndex = useMemo(() => visibleData ? buildNeighborIndex(visibleData) : new Map<string, RelatedEntry[]>(), [visibleData])
  const insights = useMemo(() => visibleData ? computeInsights(visibleData) : null, [visibleData])

  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined

    const load = async () => {
      try {
        const d = await fetchGraph()
        if (cancelled) return
        setData(d)
        setLoading(false)
        if (d.model && d.embedded_count < d.total_page_count) {
          timer = setTimeout(load, BACKFILL_POLL_MS)
        }
      } catch (e: unknown) {
        if (cancelled) return
        setError(e instanceof Error ? e.message : 'failed to load')
        setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [])

  useEffect(() => {
    if (!data || !hostRef.current) return
    if (data.nodes.length === 0) return

    const handleNodeClick = (node: GraphNode) => {
      setFocused(node)
      rendererRef.current?.setFocusedNode(node.id)
    }
    const handleBackgroundClick = () => {
      setFocused(null)
      rendererRef.current?.setFocusedNode(null)
    }

    if (rendererRef.current && nodeCountRef.current === data.nodes.length) {
      rendererRef.current.setEdges(visibleData?.edges ?? data.edges)
      return
    }

    if (rendererRef.current) rendererRef.current.dispose()
    rendererRef.current = new GraphRenderer(
      hostRef.current,
      data.nodes,
      data.edges,
      { onNodeClick: handleNodeClick, onBackgroundClick: handleBackgroundClick },
    )
    nodeCountRef.current = data.nodes.length
  }, [data])

  useEffect(() => {
    if (!rendererRef.current || !visibleData) return
    rendererRef.current.setHiddenNodes(hiddenNodeIds)
    rendererRef.current.setEdges(visibleData.edges)
    if (focused && hiddenNodeIds.has(focused.id)) {
      setFocused(null)
      rendererRef.current.setFocusedNode(null)
    }
  }, [hiddenNodeIds, visibleData, focused])

  useEffect(() => {
    return () => {
      if (rendererRef.current) {
        rendererRef.current.dispose()
        rendererRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    rendererRef.current?.setHoverHighlight(hoverId)
  }, [hoverId])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null
      if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
      const r = rendererRef.current
      if (!r) return
      switch (e.key) {
        case 'Escape':
          setFocused(null)
          r.setFocusedNode(null)
          break
        case '0':
          r.fit()
          break
        case '+':
        case '=':
          r.zoomBy(ZOOM_IN)
          break
        case '-':
        case '_':
          r.zoomBy(ZOOM_OUT)
          break
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  if (loading) {
    return <div className="centered"><p className="muted">Loading graph…</p></div>
  }

  if (error) {
    return <div className="centered"><p className="error">{error}</p></div>
  }

  if (!data || data.nodes.length === 0) {
    return (
      <div className="centered">
        <div style={{ textAlign: 'center' }}>
          <h1 style={{ fontSize: '1.1rem', marginBottom: '.35rem' }}>Graph</h1>
          <p className="muted">Publish a few pages to see your graph.</p>
        </div>
      </div>
    )
  }

  const degraded = data.embedded_count < data.total_page_count
  const zoomIn = () => rendererRef.current?.zoomBy(ZOOM_IN)
  const zoomOut = () => rendererRef.current?.zoomBy(ZOOM_OUT)
  const fit = () => rendererRef.current?.fit()
  const clearFocus = () => {
    setFocused(null)
    rendererRef.current?.setFocusedNode(null)
  }
  const openFocused = () => {
    if (focused?.url) window.open(focused.url, '_blank', 'noopener,noreferrer')
  }
  const focusNode = (node: GraphNode) => {
    setFocused(node)
    rendererRef.current?.setFocusedNode(node.id)
  }

  return (
    <div className="graph-page">
      <div className="graph-canvas" ref={hostRef} />

      <FilterBar
        collections={data.collections}
        nodes={data.nodes}
        excludedVis={excludedVis}
        excludedCols={excludedCols}
        onToggleVis={(v) => setExcludedVis(prev => toggle(prev, v))}
        onToggleCol={(id) => setExcludedCols(prev => toggle(prev, id))}
      />

      <div className="graph-stats">
        <div className="graph-stats-pill">
          {visibleData?.nodes.length ?? data.nodes.length} pages · {visibleData?.edges.length ?? data.edges.length} connections
          {data.model && ` · ${data.model}`}
        </div>
        {degraded && (
          <div className="graph-notice">
            Computing relationships · {data.embedded_count}/{data.total_page_count}
          </div>
        )}
      </div>

      <div className="graph-toolbar">
        <button onClick={zoomIn} title="Zoom in (+)" aria-label="Zoom in">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
        </button>
        <button onClick={zoomOut} title="Zoom out (−)" aria-label="Zoom out">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="5" y1="12" x2="19" y2="12" /></svg>
        </button>
        <button onClick={fit} title="Fit to screen (0)" aria-label="Fit to screen">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 9V5a2 2 0 012-2h4M21 9V5a2 2 0 00-2-2h-4M3 15v4a2 2 0 002 2h4M21 15v4a2 2 0 01-2 2h-4" /></svg>
        </button>
      </div>

      <div className={`graph-panel ${focused ? 'is-focused' : 'is-insights'}`}>
        {focused ? (
          <NodeDetail
            node={focused}
            related={neighborIndex.get(focused.id) ?? []}
            onClose={clearFocus}
            onOpen={openFocused}
            onSelectRelated={focusNode}
            onHoverRelated={setHoverId}
          />
        ) : insights ? (
          <InsightsView
            insights={insights}
            onSelect={focusNode}
            onHover={setHoverId}
          />
        ) : null}
      </div>
    </div>
  )
}

function NodeDetail({ node, related, onClose, onOpen, onSelectRelated, onHoverRelated }: {
  node: GraphNode
  related: RelatedEntry[]
  onClose: () => void
  onOpen: () => void
  onSelectRelated: (node: GraphNode) => void
  onHoverRelated: (id: string | null) => void
}) {
  const host = node.url ? new URL(node.url).host : ''
  const path = node.url ? new URL(node.url).pathname : ''
  const shown = related.slice(0, RELATED_LIMIT)
  const more = related.length - shown.length
  return (
    <div className="graph-panel-inner" role="dialog">
      <button className="graph-panel-close" onClick={onClose} aria-label="Close">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
      </button>
      <div className="graph-panel-eyebrow">
        <span className={`graph-panel-visibility graph-panel-visibility--${node.visibility}`}>
          {node.visibility}
        </span>
      </div>
      <h3 className="graph-panel-title">{node.title || node.slug}</h3>
      {node.url && <div className="graph-panel-meta">{host}{path}</div>}
      {node.tags.length > 0 && (
        <div className="graph-panel-tags">
          {node.tags.map(t => <span key={t} className="graph-panel-tag">{t}</span>)}
        </div>
      )}
      {shown.length > 0 && (
        <div className="graph-panel-section">
          <div className="graph-panel-section-label">Related</div>
          <ul className="graph-panel-list">
            {shown.map(r => (
              <li key={r.node.id}>
                <button
                  className="graph-panel-item"
                  onClick={() => onSelectRelated(r.node)}
                  onMouseEnter={() => onHoverRelated(r.node.id)}
                  onMouseLeave={() => onHoverRelated(null)}
                  title={`Cosine similarity ${r.weight.toFixed(3)}`}
                >
                  <span className="graph-panel-bar" style={{ width: `${Math.round(r.weight * 100)}%` }} />
                  <VisibilityDot node={r.node} />
                  <span className="graph-panel-item-title">{r.node.title || r.node.slug}</span>
                  <span className="graph-panel-item-value">{Math.round(r.weight * 100)}%</span>
                </button>
              </li>
            ))}
          </ul>
          {more > 0 && <div className="graph-panel-more">+{more} more</div>}
        </div>
      )}
      <div className="graph-panel-footer">
        <button className="graph-panel-open" onClick={onOpen}>
          Open
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3" /></svg>
        </button>
        <div className="graph-panel-hint"><kbd>Esc</kbd></div>
      </div>
    </div>
  )
}

function InsightsView({ insights, onSelect, onHover }: {
  insights: GraphInsights
  onSelect: (node: GraphNode) => void
  onHover: (id: string | null) => void
}) {
  const hasContent = insights.anchors.length > 0 || insights.orphans.length > 0
  return (
    <div className="graph-panel-inner">
      <h3 className="graph-panel-title">{insights.headline}</h3>
      <div className="graph-panel-subtitle">Click a page to explore its relationships.</div>

      {insights.anchors.length > 0 && (
        <div className="graph-panel-section">
          <div className="graph-panel-section-label">Anchors</div>
          <ul className="graph-panel-list">
            {insights.anchors.map(a => (
              <li key={a.node.id}>
                <button
                  className="graph-panel-item"
                  onClick={() => onSelect(a.node)}
                  onMouseEnter={() => onHover(a.node.id)}
                  onMouseLeave={() => onHover(null)}
                >
                  <VisibilityDot node={a.node} />
                  <span className="graph-panel-item-title">{a.node.title || a.node.slug}</span>
                  <span className="graph-panel-item-value">{a.degree}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {insights.orphans.length > 0 && (
        <div className="graph-panel-section">
          <div className="graph-panel-section-label">Not yet connected</div>
          <ul className="graph-panel-list">
            {insights.orphans.map(n => (
              <li key={n.id}>
                <button
                  className="graph-panel-item"
                  onClick={() => onSelect(n)}
                  onMouseEnter={() => onHover(n.id)}
                  onMouseLeave={() => onHover(null)}
                >
                  <VisibilityDot node={n} />
                  <span className="graph-panel-item-title">{n.title || n.slug}</span>
                </button>
              </li>
            ))}
          </ul>
          {insights.orphanOverflow > 0 && (
            <div className="graph-panel-more">+{insights.orphanOverflow} more</div>
          )}
        </div>
      )}

      {!hasContent && (
        <div className="graph-panel-subtitle" style={{ marginTop: '.5rem' }}>
          Publish a few more pages to see relationships emerge.
        </div>
      )}
    </div>
  )
}

function toggle<T>(set: Set<T>, value: T): Set<T> {
  const next = new Set(set)
  if (next.has(value)) next.delete(value)
  else next.add(value)
  return next
}

function FilterBar({ collections, nodes, excludedVis, excludedCols, onToggleVis, onToggleCol }: {
  collections: { id: string; title: string }[]
  nodes: GraphNode[]
  excludedVis: Set<Visibility>
  excludedCols: Set<string>
  onToggleVis: (v: Visibility) => void
  onToggleCol: (id: string) => void
}) {
  const visCounts = useMemo(() => {
    const c: Record<Visibility, number> = { public: 0, unlisted: 0, private: 0 }
    for (const n of nodes) c[n.visibility]++
    return c
  }, [nodes])
  const colCounts = useMemo(() => {
    const c = new Map<string, number>()
    for (const n of nodes) {
      const key = n.collection_id ?? STANDALONE_KEY
      c.set(key, (c.get(key) ?? 0) + 1)
    }
    return c
  }, [nodes])

  const visibleVisibilities = ALL_VISIBILITIES.filter(v => visCounts[v] > 0)
  const standaloneCount = colCounts.get(STANDALONE_KEY) ?? 0
  const showStandalone = standaloneCount > 0
  const colsWithCounts = collections
    .map(c => ({ ...c, count: colCounts.get(c.id) ?? 0 }))
    .filter(c => c.count > 0)

  if (visibleVisibilities.length <= 1 && colsWithCounts.length === 0 && !showStandalone) return null

  return (
    <div className="graph-filters">
      {visibleVisibilities.length > 1 && (
        <div className="graph-filters-group">
          <span className="graph-filters-label">Visibility</span>
          {visibleVisibilities.map(v => {
            const active = !excludedVis.has(v)
            return (
              <button
                key={v}
                className={`graph-filters-chip ${active ? 'is-active' : 'is-muted'} graph-filters-chip--${v}`}
                onClick={() => onToggleVis(v)}
                title={`${active ? 'Hide' : 'Show'} ${v} pages`}
              >
                <span className={`graph-filters-dot graph-filters-dot--${v}`} />
                <span>{v}</span>
                <span className="graph-filters-count">{visCounts[v]}</span>
              </button>
            )
          })}
        </div>
      )}

      {(colsWithCounts.length > 0 || showStandalone) && (
        <div className="graph-filters-group">
          <span className="graph-filters-label">Collections</span>
          {showStandalone && (
            <button
              className={`graph-filters-chip ${!excludedCols.has(STANDALONE_KEY) ? 'is-active' : 'is-muted'}`}
              onClick={() => onToggleCol(STANDALONE_KEY)}
            >
              <span>Standalone</span>
              <span className="graph-filters-count">{standaloneCount}</span>
            </button>
          )}
          {colsWithCounts.map(c => {
            const active = !excludedCols.has(c.id)
            return (
              <button
                key={c.id}
                className={`graph-filters-chip ${active ? 'is-active' : 'is-muted'}`}
                onClick={() => onToggleCol(c.id)}
              >
                <span>{c.title}</span>
                <span className="graph-filters-count">{c.count}</span>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}

function VisibilityDot({ node }: { node: GraphNode }) {
  if (node.visibility === 'public') return null
  const label = node.visibility === 'private' ? 'Private' : 'Unlisted'
  return (
    <span
      className={`graph-panel-visibility-dot graph-panel-visibility-dot--${node.visibility}`}
      title={label}
      aria-label={label}
    />
  )
}
