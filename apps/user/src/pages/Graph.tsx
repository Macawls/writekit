import { useEffect, useRef, useState } from 'react'
import { fetchGraph } from '../api/graph'
import type { GraphResponse, GraphNode } from '../graph/types'
import { GraphRenderer } from '../graph/renderer'

const BACKFILL_POLL_MS = 5000

export default function Graph() {
  const hostRef = useRef<HTMLDivElement>(null)
  const [data, setData] = useState<GraphResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

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

  const rendererRef = useRef<GraphRenderer | null>(null)
  const nodeCountRef = useRef(0)

  useEffect(() => {
    if (!data || !hostRef.current) return
    if (data.nodes.length === 0) return

    const handleClick = (node: GraphNode) => {
      if (node.url) window.open(node.url, '_blank', 'noopener,noreferrer')
    }

    if (rendererRef.current && nodeCountRef.current === data.nodes.length) {
      rendererRef.current.setEdges(data.edges)
      return
    }

    if (rendererRef.current) {
      rendererRef.current.dispose()
    }
    rendererRef.current = new GraphRenderer(hostRef.current, data.nodes, data.edges, handleClick)
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

  if (loading) {
    return <div className="page"><p className="muted">Loading graph…</p></div>
  }

  if (error) {
    return <div className="page"><p className="error">{error}</p></div>
  }

  if (!data || data.nodes.length === 0) {
    return (
      <div className="page">
        <h1>Graph</h1>
        <p className="muted">Publish a few pages to see your graph.</p>
      </div>
    )
  }

  const degraded = data.embedded_count < data.total_page_count

  return (
    <div className="graph-page">
      <div className="graph-header">
        <h1>Graph</h1>
        <p className="muted">
          {data.nodes.length} pages · {data.edges.length} relationships
          {data.model && ` · ${data.model}`}
        </p>
        {degraded && (
          <p className="graph-notice">
            Relationships are still being computed ({data.embedded_count}/{data.total_page_count}).
          </p>
        )}
      </div>
      <div className="graph-canvas" ref={hostRef} />
    </div>
  )
}
