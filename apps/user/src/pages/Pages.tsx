import { useEffect, useMemo, useState } from 'react'
import { api, type PageListItem, type CollectionLight } from '../api'
import { navigate } from '../stores/router'
import { Select } from '../components/Select'

type StatusFilter = 'all' | 'published' | 'draft'
type VisibilityFilter = 'all' | 'public' | 'unlisted' | 'private'

const PAGE_SIZE = 25

export default function Pages() {
  const [pages, setPages] = useState<PageListItem[] | null>(null)
  const [collections, setCollections] = useState<CollectionLight[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [status, setStatus] = useState<StatusFilter>('all')
  const [visibility, setVisibility] = useState<VisibilityFilter>('all')
  const [collection, setCollection] = useState<string>('all')

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 200)
    return () => clearTimeout(t)
  }, [search])

  useEffect(() => {
    setLoading(true)
    setError(null)
    api.listPages({ limit: PAGE_SIZE, offset, status, collection, visibility, q: debouncedSearch })
      .then(r => {
        setPages(r.pages)
        setCollections(r.collections)
        setTotal(r.total)
      })
      .catch(e => setError(e instanceof Error ? e.message : 'failed'))
      .finally(() => setLoading(false))
  }, [offset, status, collection, visibility, debouncedSearch])

  useEffect(() => { setOffset(0) }, [status, collection, visibility, debouncedSearch])

  const colById = useMemo(() => new Map(collections.map(c => [c.id, c])), [collections])

  const visible = pages ?? []

  const pageSlugPath = (p: PageListItem) => {
    const col = p.collection_id ? colById.get(p.collection_id) : null
    return col ? `${col.slug}/${p.slug}` : p.slug
  }

  const openPage = (_e: React.MouseEvent, p: PageListItem) => {
    navigate('pageView', pageSlugPath(p))
  }

  const end = pages ? Math.min(offset + pages.length, total) : 0

  return (
    <div className="pages-page">
      <header className="pages-header">
        <div>
          <h2>Pages</h2>
          <p className="muted pages-subtitle">
            {pages === null ? 'Loading…' : total === 0 ? 'No pages' : `${total} page${total === 1 ? '' : 's'}`}
          </p>
        </div>
      </header>

      {error && <p className="error">{error}</p>}

      <div className="pages-toolbar">
        <input
          type="search"
          className="pages-search"
          placeholder="Search all pages…"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <div className="pages-chips">
          {(['all', 'published', 'draft'] as StatusFilter[]).map(s => (
            <button
              key={s}
              type="button"
              className={`pages-chip ${status === s ? 'active' : ''}`}
              onClick={() => setStatus(s)}
            >
              {s === 'all' ? 'All' : s[0].toUpperCase() + s.slice(1)}
            </button>
          ))}
        </div>
        <Select
          className="pages-select"
          value={visibility}
          onChange={v => setVisibility(v as VisibilityFilter)}
          ariaLabel="Visibility filter"
          options={[
            { value: 'all', label: 'Any visibility' },
            { value: 'public', label: 'Public' },
            { value: 'unlisted', label: 'Unlisted' },
            { value: 'private', label: 'Private' },
          ]}
        />
        {collections.length > 0 && (
          <Select
            className="pages-select"
            value={collection}
            onChange={setCollection}
            ariaLabel="Collection filter"
            options={[
              { value: 'all', label: 'All collections' },
              { value: 'none', label: 'Uncategorised' },
              ...collections.map(c => ({ value: c.id, label: c.title })),
            ]}
          />
        )}
      </div>

      {!pages ? null : visible.length === 0 ? (
        <div className="pages-empty muted">
          {total === 0 && !debouncedSearch && status === 'all' && visibility === 'all' && collection === 'all'
            ? 'No pages yet. Ask your AI assistant to create one.'
            : 'No pages match these filters.'}
        </div>
      ) : (
        <div className="pages-table-wrap">
          <table className="pages-table">
            <thead>
              <tr>
                <th className="pages-col-title">Title</th>
                <th className="pages-col-collection">Collection</th>
                <th className="pages-col-status">Status</th>
                <th className="pages-col-date">Updated</th>
              </tr>
            </thead>
            <tbody>
              {visible.map(p => {
                const col = p.collection_id ? colById.get(p.collection_id) : null
                const when = p.published_at || p.updated_at
                return (
                  <tr key={p.id} className="pages-tr" onClick={e => openPage(e, p)}>
                    <td className="pages-col-title">
                      <div className="pages-cell-title">{p.title || <span className="muted">Untitled</span>}</div>
                      <div className="pages-cell-slug">/{col ? col.slug + '/' : ''}{p.slug}</div>
                    </td>
                    <td className="pages-col-collection">
                      {col ? <span className="pages-col-tag">{col.title}</span> : <span className="muted">—</span>}
                    </td>
                    <td className="pages-col-status">
                      <StatusPill status={p.status} visibility={p.visibility} />
                    </td>
                    <td className="pages-col-date" title={when}>{formatDate(when)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {pages && total > PAGE_SIZE && (
        <div className="pages-pagination">
          <span className="muted pages-range">{offset + 1}–{end} of {total}</span>
          <div className="pages-pager">
            <button
              type="button"
              className="pages-pager-btn"
              disabled={offset === 0 || loading}
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              aria-label="Previous page"
            >‹ Prev</button>
            <button
              type="button"
              className="pages-pager-btn"
              disabled={end >= total || loading}
              onClick={() => setOffset(offset + PAGE_SIZE)}
              aria-label="Next page"
            >Next ›</button>
          </div>
        </div>
      )}
    </div>
  )
}

function StatusPill({ status, visibility }: { status: string; visibility: string }) {
  const label = status === 'published' ? 'Published' : status === 'draft' ? 'Draft' : status
  const cls = status === 'published' ? 'pill-published' : 'pill-draft'
  return (
    <span className={`pages-pill ${cls}`}>
      {label}
      {visibility && visibility !== 'public' && <span className="pages-pill-vis">· {visibility}</span>}
    </span>
  )
}

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - d.getTime()) / 86400000)
  if (diffDays < 1) {
    const diffHrs = Math.floor((now.getTime() - d.getTime()) / 3600000)
    if (diffHrs < 1) return 'just now'
    return `${diffHrs}h ago`
  }
  if (diffDays < 7) return `${diffDays}d ago`
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: diffDays > 365 ? 'numeric' : undefined })
}
