import { useEffect, useMemo, useRef, useState } from 'react'
import { fetchGraph } from '../api/graph'
import type { GraphResponse, GraphNode } from '../graph/types'
import { GraphRenderer } from '../graph/renderer'
import { computeInsights, formatPercent, type GraphInsights } from '../graph/insights'

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

  const neighborIndex = useMemo(() => data ? buildNeighborIndex(data) : new Map<string, RelatedEntry[]>(), [data])
  const insights = useMemo(() => data ? computeInsights(data) : null, [data])

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
      rendererRef.current.setEdges(data.edges)
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

      <div className="graph-stats">
        <div className="graph-stats-pill">
          {data.nodes.length} pages · {data.edges.length} connections
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
      <div className="graph-panel-eyebrow">Page</div>
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
  return (
    <div className="graph-panel-inner">
      <div className="graph-panel-eyebrow">Insights</div>
      <h3 className="graph-panel-title">Your graph at a glance</h3>

      <div className="graph-panel-stats">
        <div>
          <div className="graph-panel-stat-value">{formatPercent(insights.avgSimilarity)}</div>
          <div className="graph-panel-stat-label">avg similarity</div>
        </div>
        <div>
          <div className="graph-panel-stat-value">{insights.clusters.length}</div>
          <div className="graph-panel-stat-label">clusters</div>
        </div>
        <div>
          <div className="graph-panel-stat-value">{insights.orphans.length}</div>
          <div className="graph-panel-stat-label">orphans</div>
        </div>
      </div>

      {insights.hubs.length > 0 && (
        <InsightList
          label="Hub pages"
          hint="most connected"
          items={insights.hubs.map(h => ({
            id: h.node.id, node: h.node,
            primary: h.node.title || h.node.slug,
            secondary: `${h.degree} links`,
            barPct: null,
          }))}
          onSelect={onSelect}
          onHover={onHover}
        />
      )}

      {insights.strongestPairs.length > 0 && (
        <InsightList
          label="Closest pairs"
          hint="highest similarity"
          items={insights.strongestPairs.map((p, i) => ({
            id: `pair-${i}`, node: p.a,
            primary: `${p.a.title || p.a.slug}  ↔  ${p.b.title || p.b.slug}`,
            secondary: `${Math.round(p.weight * 100)}%${p.weight >= 0.9 ? ' · near-duplicate' : ''}`,
            barPct: Math.round(p.weight * 100),
          }))}
          onSelect={onSelect}
          onHover={onHover}
        />
      )}

      {insights.clusters.length > 0 && (
        <InsightList
          label="Clusters"
          hint="click to focus"
          items={insights.clusters.map(c => ({
            id: `cluster-${c.id}`, node: c.nodes[0],
            primary: `${c.nodes.length} pages`,
            secondary: c.nodes.slice(0, 3).map(n => n.title || n.slug).join(' · '),
            barPct: null,
          }))}
          onSelect={onSelect}
          onHover={onHover}
        />
      )}

      {insights.orphans.length > 0 && (
        <InsightList
          label="Orphans"
          hint="no strong relationships yet"
          items={insights.orphans.slice(0, 5).map(n => ({
            id: n.id, node: n,
            primary: n.title || n.slug,
            secondary: '',
            barPct: null,
          }))}
          onSelect={onSelect}
          onHover={onHover}
        />
      )}

      {insights.untaggedCount > 0 && (
        <div className="graph-panel-note">
          <span className="graph-panel-note-dot" />
          {insights.untaggedCount} {insights.untaggedCount === 1 ? 'page has' : 'pages have'} no tags
        </div>
      )}
    </div>
  )
}

interface InsightItem {
  id: string
  node: GraphNode
  primary: string
  secondary: string
  barPct: number | null
}

function InsightList({ label, hint, items, onSelect, onHover }: {
  label: string
  hint?: string
  items: InsightItem[]
  onSelect: (n: GraphNode) => void
  onHover: (id: string | null) => void
}) {
  return (
    <div className="graph-panel-section">
      <div className="graph-panel-section-label">
        {label}
        {hint && <span className="graph-panel-section-hint"> · {hint}</span>}
      </div>
      <ul className="graph-panel-list">
        {items.map(item => (
          <li key={item.id}>
            <button
              className="graph-panel-item"
              onClick={() => onSelect(item.node)}
              onMouseEnter={() => onHover(item.node.id)}
              onMouseLeave={() => onHover(null)}
            >
              {item.barPct !== null && (
                <span className="graph-panel-bar" style={{ width: `${item.barPct}%` }} />
              )}
              <span className="graph-panel-item-title">{item.primary}</span>
              {item.secondary && <span className="graph-panel-item-value">{item.secondary}</span>}
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
