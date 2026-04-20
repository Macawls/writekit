import { useEffect, useRef, type ReactNode } from 'react'
import { useStore } from '@nanostores/react'
import type { GraphNode, Visibility } from '../graph/types'
import { GraphRenderer } from '../graph/renderer'
import type { GraphInsights } from '../graph/insights'
import {
  $graphData, $loading, $error, $focused, $hoverId,
  $excludedVis, $excludedCols, $view3D,
  $visibleData, $hiddenNodeIds, $neighborIndex, $insights,
  loadGraph, toggleVis, toggleCol,
  focusNode, setHover, toggleView3D,
  STANDALONE_KEY,
  type RelatedEntry,
} from '../stores/graph'
import { $site } from '../stores/auth'
import { $embeddingPrefs } from '../embedding/settings'
import { $embeddingStatus } from '../embedding/controller'

const ALL_VISIBILITIES: Visibility[] = ['public', 'unlisted', 'private']
const ZOOM_IN = 1.3
const ZOOM_OUT = 1 / 1.3
const RELATED_LIMIT = 8

export default function Graph() {
  const hostRef = useRef<HTMLDivElement>(null)
  const rendererRef = useRef<GraphRenderer | null>(null)

  const data = useStore($graphData)
  const loading = useStore($loading)
  const error = useStore($error)
  const focused = useStore($focused)
  const hoverId = useStore($hoverId)
  const excludedVis = useStore($excludedVis)
  const excludedCols = useStore($excludedCols)
  const view3D = useStore($view3D)
  const visibleData = useStore($visibleData)
  const hiddenNodeIds = useStore($hiddenNodeIds)
  const neighborIndex = useStore($neighborIndex)
  const insights = useStore($insights)
  const site = useStore($site)
  const prefs = useStore($embeddingPrefs)
  const embedStatus = useStore($embeddingStatus)

  useEffect(() => {
    if (site?.ID) loadGraph(site.ID)
  }, [site?.ID])

  useEffect(() => {
    if (!data || !hostRef.current) return
    if (data.nodes.length === 0) return

    const initialEdges = visibleData?.edges ?? []
    if (!rendererRef.current) {
      rendererRef.current = new GraphRenderer(
        hostRef.current,
        data.nodes,
        initialEdges,
        {
          onNodeClick: (n) => focusNode(n),
          onBackgroundClick: () => focusNode(null),
        },
        view3D ? '3d' : '2d',
      )
    } else {
      rendererRef.current.setGraph(
        visibleData?.nodes ?? data.nodes,
        initialEdges,
      )
    }
  }, [data, visibleData])

  useEffect(() => {
    if (!rendererRef.current) return
    rendererRef.current.setHiddenNodes(hiddenNodeIds)
    if (focused && hiddenNodeIds.has(focused.id)) focusNode(null)
  }, [hiddenNodeIds, focused])

  useEffect(() => {
    rendererRef.current?.setFocusedNode(focused?.id ?? null)
  }, [focused])

  useEffect(() => {
    rendererRef.current?.setHoverHighlight(hoverId)
  }, [hoverId])

  useEffect(() => {
    rendererRef.current?.setMode(view3D ? '3d' : '2d')
  }, [view3D])

  useEffect(() => () => {
    if (rendererRef.current) {
      rendererRef.current.dispose()
      rendererRef.current = null
    }
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null
      if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
      const r = rendererRef.current
      if (!r) return
      switch (e.key) {
        case 'Escape': focusNode(null); break
        case '0': r.fit(); break
        case '+': case '=': r.zoomBy(ZOOM_IN); break
        case '-': case '_': r.zoomBy(ZOOM_OUT); break
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

  const zoomIn = () => rendererRef.current?.zoomBy(ZOOM_IN)
  const zoomOut = () => rendererRef.current?.zoomBy(ZOOM_OUT)
  const fit = () => rendererRef.current?.fit()
  const openFocused = () => {
    if (focused?.url) window.open(focused.url, '_blank', 'noopener,noreferrer')
  }

  return (
    <div className="graph-page">
      <div className="graph-canvas" ref={hostRef} />

      <FilterBar
        collections={data.collections}
        nodes={data.nodes}
        excludedVis={excludedVis}
        excludedCols={excludedCols}
      />

      <div className="graph-stats">
        <div className="graph-stats-pill">
          {visibleData?.nodes.length ?? data.nodes.length} pages · {visibleData?.edges.length ?? 0} connections
          {prefs.enabled && ` · ${prefs.modelId.split('/').pop()}`}
        </div>
        {prefs.enabled && embedStatus.state === 'loading' && (
          <div className="graph-notice">
            Loading model… {embedStatus.loaded && embedStatus.total
              ? `${Math.round((embedStatus.loaded / embedStatus.total) * 100)}%`
              : ''}
          </div>
        )}
        {prefs.enabled && embedStatus.state === 'ready' && embedStatus.pending > 0 && (
          <div className="graph-notice">
            Embedding {embedStatus.pending} page{embedStatus.pending === 1 ? '' : 's'}…
          </div>
        )}
        {prefs.enabled && embedStatus.state === 'error' && (
          <div className="graph-notice graph-notice-error">
            Embedding error: {embedStatus.message || 'see console for details'}
          </div>
        )}
        {!prefs.enabled && (
          <div className="graph-notice">
            Enable semantic graph in Settings to see connections.
          </div>
        )}
      </div>

      <div className="graph-toolbar">
        <button
          className={view3D ? 'is-active' : ''}
          onClick={toggleView3D}
          title={view3D ? 'Switch to 2D' : 'Switch to 3D'}
          aria-label="Toggle 2D/3D"
        >
          <span style={{ fontSize: 10, fontWeight: 700, letterSpacing: '.02em' }}>{view3D ? '3D' : '2D'}</span>
        </button>
        {!view3D && (
          <>
            <button onClick={zoomIn} title="Zoom in (+)" aria-label="Zoom in">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            </button>
            <button onClick={zoomOut} title="Zoom out (−)" aria-label="Zoom out">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="5" y1="12" x2="19" y2="12" /></svg>
            </button>
          </>
        )}
        <button onClick={fit} title="Fit to screen (0)" aria-label="Fit to screen">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 9V5a2 2 0 012-2h4M21 9V5a2 2 0 00-2-2h-4M3 15v4a2 2 0 002 2h4M21 15v4a2 2 0 01-2 2h-4" /></svg>
        </button>
      </div>

      <div className={`graph-panel ${focused ? 'is-focused' : 'is-insights'}`}>
        {focused ? (
          <NodeDetail
            node={focused}
            related={neighborIndex.get(focused.id) ?? []}
            onClose={() => focusNode(null)}
            onOpen={openFocused}
          />
        ) : insights ? (
          <InsightsView insights={insights} />
        ) : null}
      </div>
    </div>
  )
}

function NodeDetail({ node, related, onClose, onOpen }: {
  node: GraphNode
  related: RelatedEntry[]
  onClose: () => void
  onOpen: () => void
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
                  onClick={() => focusNode(r.node)}
                  onMouseEnter={() => setHover(r.node.id)}
                  onMouseLeave={() => setHover(null)}
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

function InsightsView({ insights }: { insights: GraphInsights }) {
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
                  onClick={() => focusNode(a.node)}
                  onMouseEnter={() => setHover(a.node.id)}
                  onMouseLeave={() => setHover(null)}
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
                  onClick={() => focusNode(n)}
                  onMouseEnter={() => setHover(n.id)}
                  onMouseLeave={() => setHover(null)}
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

function FilterBar({ collections, nodes, excludedVis, excludedCols }: {
  collections: { id: string; title: string }[]
  nodes: GraphNode[]
  excludedVis: Set<Visibility>
  excludedCols: Set<string>
}) {
  const visCounts: Record<Visibility, number> = { public: 0, unlisted: 0, private: 0 }
  for (const n of nodes) visCounts[n.visibility]++

  const colCounts = new Map<string, number>()
  for (const n of nodes) {
    const key = n.collection_id ?? STANDALONE_KEY
    colCounts.set(key, (colCounts.get(key) ?? 0) + 1)
  }

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
        <div className="graph-filters-row">
          <span className="graph-filters-label">Visibility</span>
          <TokenRow>
            {visibleVisibilities.map(v => (
              <FilterToken
                key={v}
                active={!excludedVis.has(v)}
                onClick={() => toggleVis(v)}
                label={v}
                count={visCounts[v]}
                dot={v}
              />
            ))}
          </TokenRow>
        </div>
      )}

      {(colsWithCounts.length > 0 || showStandalone) && (
        <div className="graph-filters-row">
          <span className="graph-filters-label">Collections</span>
          <TokenRow>
            {showStandalone && (
              <FilterToken
                active={!excludedCols.has(STANDALONE_KEY)}
                onClick={() => toggleCol(STANDALONE_KEY)}
                label="Standalone"
                count={standaloneCount}
              />
            )}
            {colsWithCounts.map(c => (
              <FilterToken
                key={c.id}
                active={!excludedCols.has(c.id)}
                onClick={() => toggleCol(c.id)}
                label={c.title}
                count={c.count}
              />
            ))}
          </TokenRow>
        </div>
      )}
    </div>
  )
}

function TokenRow({ children }: { children: ReactNode }) {
  return <span className="graph-filters-tokens">{children}</span>
}

function FilterToken({ active, onClick, label, count, dot }: {
  active: boolean
  onClick: () => void
  label: string
  count: number
  dot?: Visibility
}) {
  return (
    <button
      className={`graph-filter-token ${active ? 'is-active' : 'is-muted'}`}
      onClick={onClick}
      aria-pressed={active}
      title={`${active ? 'Hide' : 'Show'} ${label}`}
    >
      {dot && <span className={`graph-filter-token-dot graph-filter-token-dot--${dot}`} aria-hidden="true" />}
      <span>{label}</span>
      <span className="graph-filter-token-count">{count}</span>
    </button>
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
