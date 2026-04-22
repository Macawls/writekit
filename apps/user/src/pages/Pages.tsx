import { useEffect, useMemo, useState } from 'react'
import { api, type PageListItem, type CollectionLight } from '../api'
import { navigate } from '../stores/router'
import { Select } from '../components/Select'

type StatusFilter = 'all' | 'published' | 'draft'
type VisibilityFilter = 'all' | 'public' | 'unlisted' | 'private'
type SortKey = 'recent' | 'title' | 'published' | 'created'

const PAGE_SIZE_KEY = 'writekit:pages:pageSize'
const PAGE_SIZE_OPTIONS = [10, 15, 25, 50]
const DEFAULT_PAGE_SIZE = 15

function loadPageSize(): number {
  try {
    const raw = localStorage.getItem(PAGE_SIZE_KEY)
    if (!raw) return DEFAULT_PAGE_SIZE
    const n = parseInt(raw, 10)
    return PAGE_SIZE_OPTIONS.includes(n) ? n : DEFAULT_PAGE_SIZE
  } catch {
    return DEFAULT_PAGE_SIZE
  }
}

export default function Pages() {
  const [pages, setPages] = useState<PageListItem[] | null>(null)
  const [collections, setCollections] = useState<CollectionLight[]>([])
  const [allTags, setAllTags] = useState<string[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [status, setStatus] = useState<StatusFilter>('all')
  const [visibility, setVisibility] = useState<VisibilityFilter>('all')
  const [collection, setCollection] = useState<string>('all')
  const [tag, setTag] = useState<string>('all')
  const [sort, setSort] = useState<SortKey>('recent')
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [pageSize, setPageSize] = useState<number>(() => loadPageSize())

  useEffect(() => {
    try { localStorage.setItem(PAGE_SIZE_KEY, String(pageSize)) } catch {}
  }, [pageSize])

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 200)
    return () => clearTimeout(t)
  }, [search])

  useEffect(() => {
    setLoading(true)
    setError(null)
    api.listPages({ limit: pageSize, offset, status, collection, visibility, tag, sort, q: debouncedSearch })
      .then(r => {
        setPages(r.pages)
        setCollections(r.collections)
        setAllTags(r.tags ?? [])
        setTotal(r.total)
      })
      .catch(e => setError(e instanceof Error ? e.message : 'failed'))
      .finally(() => setLoading(false))
  }, [offset, pageSize, status, collection, visibility, tag, sort, debouncedSearch])

  useEffect(() => { setOffset(0) }, [status, collection, visibility, tag, debouncedSearch, pageSize])

  const colById = useMemo(() => new Map(collections.map(c => [c.id, c])), [collections])

  const activeFilterCount =
    (status !== 'all' ? 1 : 0) +
    (visibility !== 'all' ? 1 : 0) +
    (collection !== 'all' ? 1 : 0) +
    (tag !== 'all' ? 1 : 0)
  const filtersActive = activeFilterCount > 0 || debouncedSearch.length > 0
  const clearFilters = () => {
    setSearch('')
    setDebouncedSearch('')
    setStatus('all')
    setVisibility('all')
    setCollection('all')
    setTag('all')
  }

  const visible = pages ?? []
  const end = pages ? Math.min(offset + pages.length, total) : 0

  const openPage = (p: PageListItem) => {
    const col = p.collection_id ? colById.get(p.collection_id) : null
    navigate('pageView', col ? `${col.slug}/${p.slug}` : p.slug)
  }

  return (
    <>
      <h2>Pages</h2>
      <p className="muted" style={{ marginTop: '.25rem' }}>
        {pages === null
          ? 'Loading…'
          : total === 0
            ? 'No pages yet.'
            : `${total} page${total === 1 ? '' : 's'}${collections.length > 0 ? ` · ${collections.length} collection${collections.length === 1 ? '' : 's'}` : ''}${allTags.length > 0 ? ` · ${allTags.length} tag${allTags.length === 1 ? '' : 's'}` : ''}`}
      </p>

      {error && <p className="error">{error}</p>}

      <div className="card pages-card" style={{ marginTop: '1.5rem' }}>
        <div className="pages-toolbar">
          <input
            type="search"
            className="pages-search"
            placeholder="Search pages…"
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
          <button
            type="button"
            className={`pages-filters-btn ${filtersOpen || activeFilterCount > 0 ? 'active' : ''}`}
            onClick={() => setFiltersOpen(v => !v)}
            aria-expanded={filtersOpen}
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" /></svg>
            Filters
            {activeFilterCount > 0 && <span className="pages-filters-count">{activeFilterCount}</span>}
          </button>
          <Select
            className="pages-select pages-sort"
            value={sort}
            onChange={v => setSort(v as SortKey)}
            ariaLabel="Sort"
            leftIcon={
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m3 16 4 4 4-4" /><path d="M7 20V4" /><path d="m21 8-4-4-4 4" /><path d="M17 4v16" /></svg>
            }
            options={[
              { value: 'recent', label: 'recent' },
              { value: 'published', label: 'published' },
              { value: 'created', label: 'created' },
              { value: 'title', label: 'title' },
            ]}
          />
        </div>

        {filtersOpen && (
          <div className="pages-filters-panel">
            <Select
              className="pages-select"
              value={status}
              onChange={v => setStatus(v as StatusFilter)}
              ariaLabel="Status filter"
              options={[
                { value: 'all', label: 'any status' },
                { value: 'published', label: 'published' },
                { value: 'draft', label: 'draft' },
              ]}
            />
            <Select
              className="pages-select"
              value={visibility}
              onChange={v => setVisibility(v as VisibilityFilter)}
              ariaLabel="Visibility filter"
              options={[
                { value: 'all', label: 'any visibility' },
                { value: 'public', label: 'public' },
                { value: 'unlisted', label: 'unlisted' },
                { value: 'private', label: 'private' },
              ]}
            />
            {collections.length > 0 && (
              <Select
                className="pages-select"
                value={collection}
                onChange={setCollection}
                ariaLabel="Collection filter"
                searchable={collections.length > 6}
                searchPlaceholder="Search collections…"
                options={[
                  { value: 'all', label: 'any collection' },
                  { value: 'none', label: 'uncategorised' },
                  ...collections.map(c => ({ value: c.id, label: c.title })),
                ]}
              />
            )}
            {allTags.length > 0 && (
              <Select
                className="pages-select"
                value={tag}
                onChange={setTag}
                ariaLabel="Tag filter"
                searchable={allTags.length > 6}
                searchPlaceholder="Search tags…"
                options={[
                  { value: 'all', label: 'any tag' },
                  ...allTags.map(t => ({ value: t, label: t })),
                ]}
              />
            )}
            {filtersActive && (
              <button type="button" className="pages-clear" onClick={clearFilters}>clear</button>
            )}
          </div>
        )}

        {!pages ? null : visible.length === 0 ? (
          <div className="pages-empty muted">
            {filtersActive
              ? 'No pages match these filters.'
              : 'No pages yet. Ask your AI assistant to create one.'}
          </div>
        ) : (
          <ul className="pages-list" role="list">
            {visible.map(p => {
              const col = p.collection_id ? colById.get(p.collection_id) : null
              const when = p.published_at || p.updated_at
              const isDraft = p.status !== 'published'
              return (
                <li
                  key={p.id}
                  className="pages-row"
                  role="button"
                  tabIndex={0}
                  onClick={() => openPage(p)}
                  onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openPage(p) } }}
                  title={`/${col ? col.slug + '/' : ''}${p.slug}${p.tags.length > 0 ? ' · #' + p.tags.join(' #') : ''}`}
                >
                  <span className="pages-title-cell">
                    <span className="pages-title-text">{p.title || <span className="muted">Untitled</span>}</span>
                    {isDraft && (
                      <span className="visibility-badge visibility-draft" title="Draft — not yet published">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /></svg>
                        draft
                      </span>
                    )}
                    {p.visibility === 'unlisted' && (
                      <span className="visibility-badge visibility-unlisted" title="Hidden from index and sitemap, accessible via URL">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" /><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" /><line x1="1" y1="1" x2="23" y2="23" /></svg>
                        unlisted
                      </span>
                    )}
                    {p.visibility === 'private' && (
                      <span className="visibility-badge visibility-private" title="Only visible to team members">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>
                        private
                      </span>
                    )}
                  </span>
                  {col ? (
                    <button
                      type="button"
                      className="visibility-badge pages-col-badge"
                      title={`Filter by ${col.title}`}
                      onClick={e => { e.stopPropagation(); setCollection(col.id) }}
                    >
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                      </svg>
                      {col.title}
                    </button>
                  ) : <span />}
                  <span className="pages-date-cell" title={when}>{formatDate(when)}</span>
                </li>
              )
            })}
          </ul>
        )}

        {pages && visible.length > 0 && (
          <div className="pages-pagination">
            <div className="pages-pagination-left">
              <span className="muted pages-range">{offset + 1}–{end} of {total}</span>
              <Select
                className="pages-select pages-pagesize"
                value={String(pageSize)}
                onChange={v => setPageSize(parseInt(v, 10))}
                ariaLabel="Pages per page"
                options={PAGE_SIZE_OPTIONS.map(n => ({ value: String(n), label: `${n} / page` }))}
              />
            </div>
            {total > pageSize && (
              <div className="pages-pager">
                <button
                  type="button"
                  className="pages-pager-btn"
                  disabled={offset === 0 || loading}
                  onClick={() => setOffset(Math.max(0, offset - pageSize))}
                  aria-label="Previous page"
                >‹</button>
                {pageNumbers(offset, total, pageSize).map((n, i) =>
                  n === '…' ? (
                    <span key={`e${i}`} className="pages-pager-ellipsis">…</span>
                  ) : (
                    <button
                      key={n}
                      type="button"
                      className={`pages-pager-btn pages-pager-num ${n === Math.floor(offset / pageSize) + 1 ? 'active' : ''}`}
                      disabled={loading}
                      onClick={() => setOffset(((n as number) - 1) * pageSize)}
                      aria-label={`Page ${n}`}
                      aria-current={n === Math.floor(offset / pageSize) + 1 ? 'page' : undefined}
                    >{n}</button>
                  )
                )}
                <button
                  type="button"
                  className="pages-pager-btn"
                  disabled={end >= total || loading}
                  onClick={() => setOffset(offset + pageSize)}
                  aria-label="Next page"
                >›</button>
              </div>
            )}
          </div>
        )}
      </div>
    </>
  )
}

function pageNumbers(offset: number, total: number, size: number): (number | '…')[] {
  const totalPages = Math.max(1, Math.ceil(total / size))
  const current = Math.floor(offset / size) + 1
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i + 1)
  }
  const out: (number | '…')[] = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(totalPages - 1, current + 1)
  if (start > 2) out.push('…')
  for (let i = start; i <= end; i++) out.push(i)
  if (end < totalPages - 1) out.push('…')
  out.push(totalPages)
  return out
}

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - d.getTime()) / 86400000)
  if (diffDays < 1) {
    const diffHrs = Math.floor((now.getTime() - d.getTime()) / 3600000)
    if (diffHrs < 1) return 'now'
    return `${diffHrs}h`
  }
  if (diffDays < 7) return `${diffDays}d`
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: diffDays > 365 ? 'numeric' : undefined })
}
