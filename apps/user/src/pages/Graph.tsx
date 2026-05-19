import { useEffect, useRef, useState } from 'react'
import { useStore } from '@nanostores/react'
import type { GraphNode } from '../graph/types'
import { GraphRenderer } from '../graph/renderer'
import {
  $graphData, $loading, $error, $focused, $hoverId,
  $excludedCols, $view3D,
  $visibleData, $hiddenNodeIds, $neighborIndex,
  loadGraph, toggleCol, clearFilters,
  focusNode, setHover, toggleView3D,
  STANDALONE_KEY,
  type RelatedEntry,
} from '../stores/graph'
import { $site } from '../stores/auth'
import { $embeddingPrefs, setEmbeddingPrefs } from '../embedding/settings'
import { embeddingController } from '../embedding/controller'
import { MODELS, findModel } from '../embedding/models'
import { fetchEmbeddingSource } from '../api/graph'
import { Select } from '../components/Select'
import { collectionColor, STANDALONE_COLOR } from '../graph/colors'

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
  const excludedCols = useStore($excludedCols)
  const view3D = useStore($view3D)
  const visibleData = useStore($visibleData)
  const hiddenNodeIds = useStore($hiddenNodeIds)
  const neighborIndex = useStore($neighborIndex)
  const site = useStore($site)

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

  if (loading && !data) {
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

      <GraphControls
        collections={data.collections}
        nodes={data.nodes}
        excludedCols={excludedCols}
      />


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

      {focused && (
        <div className="graph-panel is-focused">
          <NodeDetail
            node={focused}
            related={neighborIndex.get(focused.id) ?? []}
            onClose={() => focusNode(null)}
            onOpen={openFocused}
          />
        </div>
      )}
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


function GraphControls({ collections, nodes, excludedCols }: {
  collections: { id: string; title: string }[]
  nodes: GraphNode[]
  excludedCols: Set<string>
}) {
  const [open, setOpen] = useState(false)
  const prefs = useStore($embeddingPrefs)
  const site = useStore($site)
  const [embedError, setEmbedError] = useState<string | null>(null)
  const current = findModel(prefs.modelId)

  const colCounts = new Map<string, number>()
  for (const n of nodes) {
    const key = n.collection_id ?? STANDALONE_KEY
    colCounts.set(key, (colCounts.get(key) ?? 0) + 1)
  }
  const standaloneCount = colCounts.get(STANDALONE_KEY) ?? 0
  const showStandalone = standaloneCount > 0
  const colsWithCounts = collections
    .map(c => ({ ...c, count: colCounts.get(c.id) ?? 0 }))
    .filter(c => c.count > 0)
  const showColsSection = colsWithCounts.length > 0 || showStandalone
  const hasActiveFilters = excludedCols.size > 0

  const toggleEnabled = async () => {
    setEmbedError(null)
    if (prefs.enabled) {
      setEmbeddingPrefs({ ...prefs, enabled: false })
      await embeddingController.stop()
      return
    }
    if (!site?.ID) return
    setEmbeddingPrefs({ ...prefs, enabled: true })
    try {
      await embeddingController.start(site.ID, prefs.modelId)
      const sources = await fetchEmbeddingSource()
      embeddingController.syncPages(sources)
    } catch (e) {
      setEmbedError(e instanceof Error ? e.message : 'failed to enable')
    }
  }

  const switchModel = async (modelId: string) => {
    if (!site?.ID) return
    setEmbedError(null)
    setEmbeddingPrefs({ ...prefs, modelId })
    if (!prefs.enabled) return
    try {
      await embeddingController.start(site.ID, modelId)
      const sources = await fetchEmbeddingSource()
      embeddingController.syncPages(sources)
    } catch (e) {
      setEmbedError(e instanceof Error ? e.message : 'failed to switch model')
    }
  }

  const clearCache = async () => {
    await embeddingController.clear()
  }

  return (
    <div className="graph-controls">
      <button
        className={`graph-controls-trigger${open ? ' is-open' : ''}`}
        onClick={() => setOpen(v => !v)}
        aria-label="Graph controls"
        aria-expanded={open}
        title="Graph controls"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <line x1="21" y1="4" x2="14" y2="4" />
          <line x1="10" y1="4" x2="3" y2="4" />
          <line x1="21" y1="12" x2="12" y2="12" />
          <line x1="8" y1="12" x2="3" y2="12" />
          <line x1="21" y1="20" x2="16" y2="20" />
          <line x1="12" y1="20" x2="3" y2="20" />
          <line x1="14" y1="2" x2="14" y2="6" />
          <line x1="8" y1="10" x2="8" y2="14" />
          <line x1="16" y1="18" x2="16" y2="22" />
        </svg>
        {hasActiveFilters && <span className="graph-controls-trigger-dot" aria-hidden="true" />}
      </button>
      {open && (
        <div className="graph-controls-popover" role="dialog" aria-label="Graph controls">
          <div className="graph-settings-header">
            <div className="graph-settings-title">Graph</div>
            <button className="graph-settings-close" onClick={() => setOpen(false)} aria-label="Close">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
            </button>
          </div>

          {showColsSection && (
            <div className="graph-controls-section">
              <div className="graph-controls-section-head">
                <label className="graph-settings-label">Collections</label>
                {hasActiveFilters && (
                  <button type="button" className="graph-controls-linkbtn" onClick={clearFilters}>
                    Clear
                  </button>
                )}
              </div>
              <span className="graph-filters-tokens">
                {showStandalone && (
                  <FilterToken
                    active={!excludedCols.has(STANDALONE_KEY)}
                    onClick={() => toggleCol(STANDALONE_KEY)}
                    label="Standalone"
                    count={standaloneCount}
                    color={STANDALONE_COLOR}
                  />
                )}
                {colsWithCounts.map(c => (
                  <FilterToken
                    key={c.id}
                    active={!excludedCols.has(c.id)}
                    onClick={() => toggleCol(c.id)}
                    label={c.title}
                    count={c.count}
                    color={collectionColor(c.id)}
                  />
                ))}
              </span>
            </div>
          )}

          <div className="graph-controls-section">
            <label className="graph-settings-label">Semantic graph</label>
            <p className="graph-settings-desc">
              Embeddings are generated locally in your browser to power semantic connections. Content never leaves your machine.
            </p>
            <Select
              value={prefs.modelId}
              onChange={switchModel}
              ariaLabel="Embedding model"
              options={MODELS.map(m => ({
                value: m.id,
                label: m.label,
                hint: `${m.dims}d · ~${m.approxSizeMB}MB`,
              }))}
              className="graph-settings-select"
            />
            {current && <div className="graph-settings-hint">{current.description}</div>}
            {embedError && <div className="graph-settings-error">{embedError}</div>}
            <div className="graph-settings-actions">
              <button
                type="button"
                className={prefs.enabled ? '' : 'is-primary'}
                onClick={toggleEnabled}
              >
                {prefs.enabled ? 'Disable' : 'Enable'}
              </button>
              <button type="button" onClick={clearCache} disabled={!prefs.enabled}>
                Clear cache
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function FilterToken({ active, onClick, label, count, color }: {
  active: boolean
  onClick: () => void
  label: string
  count: number
  color?: string
}) {
  return (
    <button
      className={`graph-filter-token ${active ? 'is-active' : 'is-muted'}`}
      onClick={onClick}
      aria-pressed={active}
      title={`${active ? 'Hide' : 'Show'} ${label}`}
    >
      {color && <span className="graph-filter-token-dot" style={{ background: color }} aria-hidden="true" />}
      <span>{label}</span>
      <span className="graph-filter-token-count">{count}</span>
    </button>
  )
}

