import { useEffect, useMemo, useState } from 'react'
import { api, type DBTable, type DBTableRows, type DBSchema, type DBColumnInfo } from '../api'

type SortDir = 'asc' | 'desc'
type Drawer = { kind: 'row'; rowIdx: number; colName: string | null } | { kind: 'schema' } | null

export default function Database() {
  const [tables, setTables] = useState<DBTable[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    api.dbTables().then(ts => {
      setTables(ts)
      if (ts.length && !selected) setSelected(ts[0].name)
    }).catch(e => setError(e instanceof Error ? e.message : 'failed'))
  }, [])

  const filtered = useMemo(() => {
    if (!tables) return []
    const q = filter.trim().toLowerCase()
    if (!q) return tables
    return tables.filter(t => t.name.toLowerCase().includes(q))
  }, [tables, filter])

  return (
    <div className="db-page">
      <header className="db-header">
        <div>
          <h2>Database</h2>
          <p className="muted db-subtitle">Read-only view of this site's SQLite database.</p>
        </div>
        <a className="btn btn-outline db-download" href="/api/db/export" download>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          Download .db
        </a>
      </header>

      {error && <p className="error">{error}</p>}

      <div className="db-layout">
        <aside className="db-sidebar">
          <input
            className="db-filter"
            type="search"
            placeholder="Filter tables…"
            value={filter}
            onChange={e => setFilter(e.target.value)}
          />
          <div className="db-tables">
            {!tables && <div className="muted db-empty">Loading…</div>}
            {tables && filtered.length === 0 && <div className="muted db-empty">No matches.</div>}
            {filtered.map(t => (
              <button
                key={t.name}
                type="button"
                onClick={() => setSelected(t.name)}
                className={`db-table-item ${selected === t.name ? 'active' : ''}`}
              >
                <span className="db-table-name">
                  {t.name}
                  {t.type === 'view' && <span className="db-badge">view</span>}
                </span>
                <span className="db-table-count">{formatCount(t.rows)}</span>
              </button>
            ))}
          </div>
        </aside>

        {selected
          ? <TableView name={selected} key={selected} />
          : <section className="db-main"><div className="db-empty-state muted">Select a table.</div></section>}
      </div>
    </div>
  )
}

function TableView({ name }: { name: string }) {
  const [data, setData] = useState<DBTableRows | null>(null)
  const [schema, setSchema] = useState<DBSchema | null>(null)
  const [offset, setOffset] = useState(0)
  const [sort, setSort] = useState<{ col: string; dir: SortDir } | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [drawer, setDrawer] = useState<Drawer>(null)
  const limit = 50

  useEffect(() => {
    setLoading(true)
    setError(null)
    api.dbTableRows(name, limit, offset, sort?.col, sort?.dir)
      .then(d => { setData(d); setDrawer(null) })
      .catch(e => setError(e instanceof Error ? e.message : 'failed'))
      .finally(() => setLoading(false))
  }, [name, offset, sort])

  useEffect(() => {
    api.dbTableSchema(name).then(setSchema).catch(() => setSchema(null))
  }, [name])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setDrawer(null) }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const toggleSort = (col: string) => {
    setSort(s => {
      if (!s || s.col !== col) return { col, dir: 'asc' }
      if (s.dir === 'asc') return { col, dir: 'desc' }
      return null
    })
    setOffset(0)
  }

  const filteredRows = useMemo(() => {
    if (!data) return []
    const q = search.trim().toLowerCase()
    if (!q) return data.rows.map((r, i) => [r, i] as const)
    return data.rows
      .map((r, i) => [r, i] as const)
      .filter(([r]) => r.some(v => v != null && String(v).toLowerCase().includes(q)))
  }, [data, search])

  if (error) return <section className="db-main"><p className="error db-inline-error">{error}</p></section>
  if (!data) return <section className="db-main"><div className="db-empty-state muted">Loading…</div></section>

  const end = Math.min(offset + data.rows.length, data.total)

  return (
    <>
      <section className="db-main">
        <div className="db-toolbar">
          <div className="db-toolbar-left">
            <span className="db-table-title">{name}</span>
            <span className="muted db-range">
              {data.total === 0 ? 'empty' : `${offset + 1}–${end} of ${formatCount(data.total)}`}
            </span>
          </div>
          <div className="db-toolbar-right">
            <input
              type="search"
              className="db-search"
              placeholder="Search page…"
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
            <button
              type="button"
              className="db-toolbar-btn"
              onClick={() => setDrawer({ kind: 'schema' })}
              title="View schema"
            >Schema</button>
            <div className="db-pager">
              <button
                type="button"
                className="db-icon-btn"
                disabled={offset === 0 || loading}
                onClick={() => setOffset(Math.max(0, offset - limit))}
                aria-label="Previous page"
              >‹</button>
              <button
                type="button"
                className="db-icon-btn"
                disabled={end >= data.total || loading}
                onClick={() => setOffset(offset + limit)}
                aria-label="Next page"
              >›</button>
            </div>
          </div>
        </div>

        {data.rows.length === 0 ? (
          <div className="db-empty-state"><p className="muted">This table is empty.</p></div>
        ) : (
          <DataGrid
            columns={data.columns}
            rows={filteredRows}
            schema={data.schema}
            sort={sort}
            onSort={toggleSort}
            onCellClick={(ri, ci) => setDrawer({ kind: 'row', rowIdx: ri, colName: data.columns[ci] })}
            activeRow={drawer?.kind === 'row' ? drawer.rowIdx : null}
          />
        )}
      </section>

      {drawer && (
        <>
          <div className="db-drawer-scrim" onClick={() => setDrawer(null)} />
          <aside className="db-drawer" role="dialog" aria-modal="true">
            <div className="db-drawer-head">
              <div className="db-drawer-title">
                {drawer.kind === 'row' ? `Row ${drawer.rowIdx + 1 + offset}` : 'Schema'}
                <span className="muted db-drawer-sub">{name}</span>
              </div>
              <button type="button" className="db-drawer-close" onClick={() => setDrawer(null)} aria-label="Close">×</button>
            </div>
            <div className="db-drawer-body">
              {drawer.kind === 'row' && (
                <RowInspector
                  columns={data.columns}
                  schema={data.schema}
                  row={data.rows[drawer.rowIdx]}
                  focusCol={drawer.colName}
                />
              )}
              {drawer.kind === 'schema' && <SchemaInspector schema={schema} />}
            </div>
          </aside>
        </>
      )}
    </>
  )
}

function DataGrid({
  columns,
  rows,
  schema,
  sort,
  onSort,
  onCellClick,
  activeRow,
}: {
  columns: string[]
  rows: readonly (readonly [unknown[], number])[]
  schema?: DBColumnInfo[]
  sort: { col: string; dir: SortDir } | null
  onSort: (col: string) => void
  onCellClick: (rowIdx: number, colIdx: number) => void
  activeRow: number | null
}) {
  const schemaByCol = useMemo(() => new Map(schema?.map(c => [c.name, c]) ?? []), [schema])

  return (
    <div className="db-grid-wrap">
      <table className="db-grid">
        <thead>
          <tr>
            {columns.map(c => {
              const s = schemaByCol.get(c)
              const isSorted = sort?.col === c
              return (
                <th key={c} className={isSorted ? 'sorted' : undefined}>
                  <button type="button" className="db-th-btn" onClick={() => onSort(c)}>
                    <span className="db-th-name">{c}</span>
                    {s?.pk && <span className="db-pk">PK</span>}
                    {s?.type && <span className="db-type">{s.type.toLowerCase()}</span>}
                    <span className="db-sort-arrow">
                      {isSorted ? (sort!.dir === 'asc' ? '↑' : '↓') : ''}
                    </span>
                  </button>
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {rows.map(([r, originalIdx]) => (
            <tr key={originalIdx} className={activeRow === originalIdx ? 'selected' : undefined}>
              {r.map((v, j) => (
                <td
                  key={j}
                  onClick={() => onCellClick(originalIdx, j)}
                  title={cellTitle(v)}
                >
                  {renderCell(v)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function RowInspector({
  columns,
  schema,
  row,
  focusCol,
}: {
  columns: string[]
  schema?: DBColumnInfo[]
  row: unknown[]
  focusCol: string | null
}) {
  const schemaByCol = useMemo(() => new Map(schema?.map(c => [c.name, c]) ?? []), [schema])

  return (
    <div className="db-inspector-rows">
      {columns.map((c, i) => {
        const s = schemaByCol.get(c)
        const isFocus = c === focusCol
        return (
          <div key={c} className={`db-inspector-row ${isFocus ? 'focus' : ''}`}>
            <div className="db-inspector-colhead">
              <span className="db-inspector-colname">{c}</span>
              {s?.pk && <span className="db-pk">PK</span>}
              {s?.type && <span className="db-type">{s.type.toLowerCase()}</span>}
              <CopyButton value={row[i]} />
            </div>
            <div className="db-inspector-value">
              {renderValueRich(c, row[i])}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function SchemaInspector({ schema }: { schema: DBSchema | null }) {
  if (!schema) return <div className="muted db-drawer-empty">Loading schema…</div>
  return (
    <div className="db-inspector-rows">
      <div className="db-schema-section">
        <div className="db-schema-title">Columns</div>
        <table className="db-schema-table">
          <tbody>
            {schema.columns.map(c => (
              <tr key={c.name}>
                <td className="db-schema-col-name">
                  {c.name}
                  {c.pk && <span className="db-pk db-pk-inline">PK</span>}
                </td>
                <td className="db-type">{c.type.toLowerCase() || '—'}</td>
                <td className="muted db-schema-null">{c.not_null ? 'NOT NULL' : ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {schema.indexes.length > 0 && (
        <div className="db-schema-section">
          <div className="db-schema-title">Indexes</div>
          <ul className="db-schema-list">
            {schema.indexes.map(i => (
              <li key={i.name}>
                <code>{i.name}</code>
                {i.unique && <span className="db-badge db-pk-inline">unique</span>}
                <div className="muted db-schema-sub">on {i.columns.join(', ')}</div>
              </li>
            ))}
          </ul>
        </div>
      )}

      {schema.foreign_keys.length > 0 && (
        <div className="db-schema-section">
          <div className="db-schema-title">Foreign keys</div>
          <ul className="db-schema-list">
            {schema.foreign_keys.map((fk, i) => (
              <li key={i}>
                <code>{fk.from}</code> → <code>{fk.table}.{fk.to}</code>
                <div className="muted db-schema-sub">
                  on update {fk.on_update || 'NO ACTION'} · on delete {fk.on_delete || 'NO ACTION'}
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      {schema.create_sql && (
        <div className="db-schema-section">
          <div className="db-schema-title">Definition</div>
          <pre className="db-schema-sql">{schema.create_sql}</pre>
        </div>
      )}
    </div>
  )
}

function CopyButton({ value }: { value: unknown }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      const s = value == null ? '' : typeof value === 'string' ? value : JSON.stringify(value)
      await navigator.clipboard.writeText(s)
      setCopied(true)
      setTimeout(() => setCopied(false), 1200)
    } catch {}
  }
  return (
    <button type="button" className="db-copy-btn" onClick={copy} title="Copy value">
      {copied ? '✓' : 'copy'}
    </button>
  )
}

function isBlob(v: unknown): v is { __blob: true; size: number } {
  return typeof v === 'object' && v !== null && (v as { __blob?: unknown }).__blob === true
}

function isTruncated(v: unknown): v is { __truncated: true; size: number; preview: string } {
  return typeof v === 'object' && v !== null && (v as { __truncated?: unknown }).__truncated === true
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

function cellTitle(v: unknown): string | undefined {
  if (v == null) return 'NULL'
  if (isBlob(v)) return `BLOB (${formatBytes(v.size)})`
  if (isTruncated(v)) return `Truncated — ${formatBytes(v.size)} total`
  if (typeof v === 'string') return v
  return undefined
}

function renderCell(v: unknown): React.ReactNode {
  if (v === null || v === undefined) return <span className="db-null">NULL</span>
  if (isBlob(v)) return <span className="db-blob">BLOB · {formatBytes(v.size)}</span>
  if (isTruncated(v)) {
    const preview = v.preview.slice(0, 120).replace(/\s+/g, ' ')
    return <span title={`${formatBytes(v.size)} — click for full value`}>{preview}… <span className="db-blob">{formatBytes(v.size)}</span></span>
  }
  if (typeof v === 'string') {
    if (v === '') return <span className="db-null">''</span>
    const truncated = v.length > 120 ? v.slice(0, 120) + '…' : v
    return truncated
  }
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try { return JSON.stringify(v) } catch { return String(v) }
}

function renderValueRich(colName: string, v: unknown): React.ReactNode {
  if (v === null || v === undefined) return <span className="db-null">NULL</span>
  if (isBlob(v)) return <span className="db-blob">BLOB · {formatBytes(v.size)}</span>
  if (isTruncated(v)) {
    return (
      <div>
        <div className="muted db-value-hint">Showing first {formatBytes(v.preview.length)} of {formatBytes(v.size)}</div>
        {renderValueRich(colName, v.preview)}
      </div>
    )
  }
  if (typeof v === 'boolean' || typeof v === 'number') {
    return <span className="db-value-scalar">{String(v)}</span>
  }
  if (typeof v !== 'string') {
    try { return <pre className="db-value-pre">{JSON.stringify(v, null, 2)}</pre> } catch { return String(v) }
  }
  if (v === '') return <span className="db-null">empty string</span>

  if (colName.endsWith('_html') || /<(p|div|h\d|ul|ol|li|span|a|table|code|pre)\b/i.test(v)) {
    return <HtmlValue html={v} />
  }
  const trimmed = v.trim()
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      const parsed = JSON.parse(trimmed)
      return <pre className="db-value-pre">{JSON.stringify(parsed, null, 2)}</pre>
    } catch {}
  }
  if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}/.test(trimmed)) {
    const d = new Date(trimmed)
    if (!isNaN(d.getTime())) {
      return (
        <div>
          <div className="db-value-scalar">{trimmed}</div>
          <div className="muted db-value-hint">{d.toLocaleString()}</div>
        </div>
      )
    }
  }
  return <pre className="db-value-pre">{v}</pre>
}

function HtmlValue({ html }: { html: string }) {
  const [mode, setMode] = useState<'rendered' | 'source'>('rendered')
  return (
    <div className="db-html-value">
      <div className="db-html-toggle">
        <button type="button" className={mode === 'rendered' ? 'active' : ''} onClick={() => setMode('rendered')}>Rendered</button>
        <button type="button" className={mode === 'source' ? 'active' : ''} onClick={() => setMode('source')}>Source</button>
      </div>
      {mode === 'rendered'
        ? <div className="db-html-preview" dangerouslySetInnerHTML={{ __html: html }} />
        : <pre className="db-value-pre">{html}</pre>}
    </div>
  )
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}
