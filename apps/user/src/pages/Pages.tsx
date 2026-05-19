import { useEffect, useMemo } from 'react'
import { useStore } from '@nanostores/react'
import { type PageListItem } from '../api'
import { navigate } from '../stores/router'
import { Select } from '../components/Select'
import { MultiSelect } from '../components/MultiSelect'
import { atom } from 'nanostores'
import {
  $query, $result, $loading, $error,
  setQuery, resetQuery, ensurePagesLoaded,
  PAGE_SIZE_OPTIONS,
  type StatusFilter, type VisibilityFilter, type SortKey,
} from '../stores/pages'

const $filtersOpen = atom(false)

export default function Pages() {
  const q = useStore($query)
  const result = useStore($result)
  const loading = useStore($loading)
  const error = useStore($error)
  const filtersOpen = useStore($filtersOpen)

  useEffect(() => { ensurePagesLoaded() }, [])

  const pages = result?.pages ?? null
  const collections = result?.collections ?? []
  const allTags = result?.tags ?? []
  const total = result?.total ?? 0

  const colById = useMemo(() => new Map(collections.map(c => [c.id, c])), [collections])

  const activeFilterCount =
    (q.status !== 'all' ? 1 : 0) +
    (q.visibility !== 'all' ? 1 : 0) +
    q.collection.length +
    q.tag.length
  const filtersActive = activeFilterCount > 0 || q.search.length > 0

  const visible = pages ?? []
  const end = pages ? Math.min(q.offset + pages.length, total) : 0

  const openPage = (p: PageListItem) => {
    const col = p.collection_id ? colById.get(p.collection_id) : null
    navigate('pageView', col ? `${col.slug}/${p.slug}` : p.slug)
  }

  return (
    <>
      <h2>Pages</h2>
      <p className="muted" style={{ marginTop: '.25rem' }}>
        {pages === null
          ? (loading ? 'Loading…' : ' ')
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
            value={q.search}
            onChange={e => setQuery({ search: e.target.value })}
          />
          <button
            type="button"
            className={`pages-filters-btn ${filtersOpen || activeFilterCount > 0 ? 'active' : ''}`}
            onClick={() => $filtersOpen.set(!filtersOpen)}
            aria-expanded={filtersOpen}
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" /></svg>
            Filters
            {activeFilterCount > 0 && <span className="pages-filters-count">{activeFilterCount}</span>}
          </button>
        </div>

        {filtersOpen && (
          <div className="pages-filters-panel">
            <Select
              className="pages-select"
              value={q.status}
              onChange={v => setQuery({ status: v as StatusFilter })}
              ariaLabel="Status filter"
              leftIcon={
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10" /><path d="m9 12 2 2 4-4" /></svg>
              }
              options={[
                { value: 'all', label: 'any status', icon: <Icon path="M3 12h18M3 6h18M3 18h18" /> },
                { value: 'published', label: 'published', icon: <Icon path="M20 6 9 17l-5-5" /> },
                { value: 'draft', label: 'draft', icon: <Icon path="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /> },
              ]}
            />
            <Select
              className="pages-select"
              value={q.visibility}
              onChange={v => setQuery({ visibility: v as VisibilityFilter })}
              ariaLabel="Visibility filter"
              leftIcon={
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg>
              }
              options={[
                { value: 'all', label: 'any visibility', icon: <Icon path="M3 12h18M3 6h18M3 18h18" /> },
                { value: 'public', label: 'public', icon: <Icon path="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM2 12h20M12 2a15.3 15.3 0 0 1 0 20M12 2a15.3 15.3 0 0 0 0 20" /> },
                { value: 'unlisted', label: 'unlisted', icon: <Icon path="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19M1 1l22 22" /> },
                { value: 'private', label: 'private', icon: <Icon path="M3 11h18v11H3zM7 11V7a5 5 0 0 1 10 0v4" /> },
              ]}
            />
            {collections.length > 0 && (
              <MultiSelect
                className="pages-select"
                values={q.collection}
                onChange={v => setQuery({ collection: v })}
                ariaLabel="Collection filter"
                searchable
                searchPlaceholder="Search collections…"
                pluralLabel="collections"
                leftIcon={
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" /></svg>
                }
                options={[
                  { value: 'all', label: 'any collection' },
                  { value: 'none', label: 'no collection' },
                  ...collections.map(c => ({ value: c.id, label: c.title })),
                ]}
              />
            )}
            {allTags.length > 0 && (
              <MultiSelect
                className="pages-select"
                values={q.tag}
                onChange={v => setQuery({ tag: v })}
                ariaLabel="Tag filter"
                searchable
                searchPlaceholder="Search tags…"
                pluralLabel="tags"
                leftIcon={
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20.59 13.41 13.42 20.58a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z" /><line x1="7" y1="7" x2="7.01" y2="7" /></svg>
                }
                options={[
                  { value: 'all', label: 'any tag' },
                  ...allTags.map(t => ({ value: t, label: t })),
                ]}
              />
            )}
            <Select
              className="pages-select pages-sort"
              value={q.sort}
              onChange={v => setQuery({ sort: v as SortKey })}
              ariaLabel="Sort"
              leftIcon={
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m3 16 4 4 4-4" /><path d="M7 20V4" /><path d="m21 8-4-4-4 4" /><path d="M17 4v16" /></svg>
              }
              options={[
                { value: 'recent', label: 'last updated', icon: <Icon path="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /> },
                { value: 'published', label: 'last published', icon: <Icon path="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09zM12 15l-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2zM9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5" /> },
                { value: 'created', label: 'date created', icon: <Icon path="M21 13V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h8M16 2v4M8 2v4M3 10h18M18 16v6M15 19h6" /> },
                { value: 'title', label: 'title (a–z)', icon: <Icon path="M4 6h12M4 12h8M4 18h4M14 16l4 4 4-4M18 12v8" /> },
              ]}
            />
            {filtersActive && (
              <button type="button" className="pages-clear" onClick={resetQuery}>clear</button>
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
                      onClick={e => { e.stopPropagation(); setQuery({ collection: [col.id] }) }}
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
              <span className="muted pages-range">{q.offset + 1}–{end} of {total}</span>
              <Select
                className="pages-select pages-pagesize"
                value={String(q.pageSize)}
                onChange={v => setQuery({ pageSize: parseInt(v, 10) })}
                ariaLabel="Pages per page"
                options={PAGE_SIZE_OPTIONS.map(n => ({ value: String(n), label: `${n} / page` }))}
              />
            </div>
            {total > q.pageSize && (
              <div className="pages-pager">
                <button
                  type="button"
                  className="pages-pager-btn"
                  disabled={q.offset === 0 || loading}
                  onClick={() => setQuery({ offset: Math.max(0, q.offset - q.pageSize) })}
                  aria-label="Previous page"
                >‹</button>
                {pageNumbers(q.offset, total, q.pageSize).map((n, i) =>
                  n === '…' ? (
                    <span key={`e${i}`} className="pages-pager-ellipsis">…</span>
                  ) : (
                    <button
                      key={n}
                      type="button"
                      className={`pages-pager-btn pages-pager-num ${n === Math.floor(q.offset / q.pageSize) + 1 ? 'active' : ''}`}
                      disabled={loading}
                      onClick={() => setQuery({ offset: ((n as number) - 1) * q.pageSize })}
                      aria-label={`Page ${n}`}
                      aria-current={n === Math.floor(q.offset / q.pageSize) + 1 ? 'page' : undefined}
                    >{n}</button>
                  )
                )}
                <button
                  type="button"
                  className="pages-pager-btn"
                  disabled={end >= total || loading}
                  onClick={() => setQuery({ offset: q.offset + q.pageSize })}
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

function Icon({ path }: { path: string }) {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d={path} />
    </svg>
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
    if (diffHrs < 1) return 'now'
    return `${diffHrs}h`
  }
  if (diffDays < 7) return `${diffDays}d`
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: diffDays > 365 ? 'numeric' : undefined })
}
