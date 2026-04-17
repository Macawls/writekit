import { useEffect, useRef, useState } from 'react'
import { fetchGraph } from '../api/graph'
import type { GraphResponse, GraphNode } from '../graph/types'
import { GraphRenderer } from '../graph/renderer'

const BACKFILL_POLL_MS = 5000
const ZOOM_IN = 1.3
const ZOOM_OUT = 1 / 1.3

export default function Graph() {
  const hostRef = useRef<HTMLDivElement>(null)
  const [data, setData] = useState<GraphResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [focused, setFocused] = useState<GraphNode | null>(null)

  const rendererRef = useRef<GraphRenderer | null>(null)
  const nodeCountRef = useRef(0)

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
    const handleBackgroundClick = () => {}

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

      {focused && <NodeDetail node={focused} onClose={clearFocus} onOpen={openFocused} />}
    </div>
  )
}

function NodeDetail({ node, onClose, onOpen }: {
  node: GraphNode
  onClose: () => void
  onOpen: () => void
}) {
  const host = node.url ? new URL(node.url).host : ''
  const path = node.url ? new URL(node.url).pathname : ''
  return (
    <div className="graph-detail" role="dialog">
      <button className="graph-detail-close" onClick={onClose} aria-label="Close">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
      </button>
      <h3>{node.title || node.slug}</h3>
      {node.url && <div className="graph-detail-meta">{host}{path}</div>}
      {node.tags.length > 0 && (
        <div className="graph-detail-tags">
          {node.tags.map(t => <span key={t} className="graph-detail-tag">{t}</span>)}
        </div>
      )}
      <button className="graph-detail-open" onClick={onOpen}>
        Open
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3" /></svg>
      </button>
      <div className="graph-detail-hint"><kbd>Esc</kbd> to close</div>
    </div>
  )
}
