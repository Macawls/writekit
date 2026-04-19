import { useEffect, useMemo, useState } from 'react'
import { api, type DBTable, type DBTableRows, type DBQueryResult } from '../api'

type Tab = 'browse' | 'query'

export default function Database() {
  const [tables, setTables] = useState<DBTable[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [tab, setTab] = useState<Tab>('browse')
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
        <div className="db-header-row">
          <div>
            <h2>Database</h2>
            <p className="muted" style={{ marginTop: '.25rem' }}>
              Read-only view of this site's SQLite database.
            </p>
          </div>
          <a className="btn btn-outline" href="/api/db/export" download>
            Download .db
          </a>
        </div>
      </header>

      {error && <p className="error">{error}</p>}

      <div className="db-tabs">
        <button type="button" className={`db-tab ${tab === 'browse' ? 'active' : ''}`} onClick={() => setTab('browse')}>
          Browse
        </button>
        <button type="button" className={`db-tab ${tab === 'query' ? 'active' : ''}`} onClick={() => setTab('query')}>
          SQL
        </button>
      </div>

      {tab === 'browse' && (
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
          <section className="db-main">
            {selected ? <TableBrowser name={selected} /> : <div className="muted">Select a table.</div>}
          </section>
        </div>
      )}

      {tab === 'query' && <QueryConsole />}
    </div>
  )
}

function TableBrowser({ name }: { name: string }) {
  const [data, setData] = useState<DBTableRows | null>(null)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedRow, setSelectedRow] = useState<number | null>(null)
  const limit = 50

  useEffect(() => {
    setOffset(0)
    setSelectedRow(null)
  }, [name])

  useEffect(() => {
    setLoading(true)
    setError(null)
    setSelectedRow(null)
    api.dbTableRows(name, limit, offset)
      .then(setData)
      .catch(e => setError(e instanceof Error ? e.message : 'failed'))
      .finally(() => setLoading(false))
  }, [name, offset])

  if (error) return <p className="error">{error}</p>
  if (!data) return <div className="muted">Loading…</div>

  const end = Math.min(offset + data.rows.length, data.total)
  const schemaByCol = new Map(data.schema?.map(c => [c.name, c]) ?? [])

  return (
    <div className="db-browser">
      <div className="db-toolbar">
        <div className="db-toolbar-left">
          <span className="db-table-title">{name}</span>
          <span className="muted db-range">
            {data.total === 0 ? 'no rows' : `${offset + 1}–${end} of ${formatCount(data.total)}`}
          </span>
        </div>
        <div className="db-toolbar-right">
          <button
            type="button"
            className="db-icon-btn"
            disabled={offset === 0 || loading}
            onClick={() => setOffset(Math.max(0, offset - limit))}
            aria-label="Previous page"
          >
            ‹
          </button>
          <button
            type="button"
            className="db-icon-btn"
            disabled={end >= data.total || loading}
            onClick={() => setOffset(offset + limit)}
            aria-label="Next page"
          >
            ›
          </button>
        </div>
      </div>

      {data.rows.length === 0 ? (
        <div className="db-empty-state">
          <p className="muted">This table is empty.</p>
        </div>
      ) : (
        <DataGrid
          columns={data.columns}
          rows={data.rows}
          schema={schemaByCol}
          onRowClick={(i) => setSelectedRow(selectedRow === i ? null : i)}
          selectedRow={selectedRow}
        />
      )}

      {selectedRow !== null && data.rows[selectedRow] && (
        <RowDetail columns={data.columns} row={data.rows[selectedRow]} onClose={() => setSelectedRow(null)} />
      )}
    </div>
  )
}

function QueryConsole() {
  const [sql, setSQL] = useState("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;")
  const [result, setResult] = useState<DBQueryResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [elapsed, setElapsed] = useState<number | null>(null)

  const run = async () => {
    setError(null)
    setBusy(true)
    const t0 = performance.now()
    try {
      setResult(await api.dbQuery(sql))
      setElapsed(performance.now() - t0)
    } catch (e) {
      setResult(null)
      setElapsed(null)
      setError(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="db-query">
      <div className="db-editor">
        <textarea
          value={sql}
          onChange={e => setSQL(e.target.value)}
          spellCheck={false}
          rows={8}
          onKeyDown={e => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              e.preventDefault()
              run()
            }
          }}
          className="db-textarea"
        />
        <div className="db-editor-bar">
          <button type="button" className="btn" onClick={run} disabled={busy}>
            {busy ? 'Running…' : 'Run'}
          </button>
          <span className="muted" style={{ fontSize: '.75rem' }}>
            Read-only · SELECT / WITH / PRAGMA / EXPLAIN · <kbd>⌘/Ctrl</kbd>+<kbd>↵</kbd>
          </span>
          {elapsed !== null && !busy && !error && (
            <span className="muted" style={{ fontSize: '.75rem', marginLeft: 'auto' }}>
              {result?.rows.length ?? 0} rows · {elapsed.toFixed(0)}ms
            </span>
          )}
        </div>
      </div>

      {error && <p className="error">{error}</p>}
      {result && (
        <div className="db-result">
          {result.truncated && <div className="muted db-truncated">Truncated to 500 rows.</div>}
          {result.rows.length === 0 ? (
            <div className="muted db-empty-state">No rows.</div>
          ) : (
            <DataGrid columns={result.columns} rows={result.rows} />
          )}
        </div>
      )}
    </div>
  )
}

function DataGrid({
  columns,
  rows,
  schema,
  onRowClick,
  selectedRow,
}: {
  columns: string[]
  rows: unknown[][]
  schema?: Map<string, { type: string; pk: boolean }>
  onRowClick?: (i: number) => void
  selectedRow?: number | null
}) {
  return (
    <div className="db-grid-wrap">
      <table className="db-grid">
        <thead>
          <tr>
            {columns.map(c => {
              const s = schema?.get(c)
              return (
                <th key={c}>
                  <div className="db-th">
                    <span>{c}</span>
                    {s?.pk && <span className="db-pk">PK</span>}
                    {s?.type && <span className="db-type">{s.type.toLowerCase()}</span>}
                  </div>
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr
              key={i}
              onClick={onRowClick ? () => onRowClick(i) : undefined}
              className={selectedRow === i ? 'selected' : undefined}
              style={onRowClick ? { cursor: 'pointer' } : undefined}
            >
              {r.map((v, j) => (
                <td key={j}>{renderCell(v)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function RowDetail({ columns, row, onClose }: { columns: string[]; row: unknown[]; onClose: () => void }) {
  return (
    <div className="db-detail">
      <div className="db-detail-header">
        <span className="muted" style={{ fontSize: '.75rem' }}>Row detail</span>
        <button type="button" className="db-close" onClick={onClose} aria-label="Close detail">×</button>
      </div>
      <dl className="db-detail-list">
        {columns.map((c, i) => (
          <div key={c} className="db-detail-row">
            <dt>{c}</dt>
            <dd>{renderCellLong(row[i])}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}

function renderCell(v: unknown): React.ReactNode {
  if (v === null || v === undefined) return <span className="db-null">NULL</span>
  if (typeof v === 'string') {
    if (v === '') return <span className="db-null">''</span>
    const truncated = v.length > 120 ? v.slice(0, 120) + '…' : v
    return <span title={v.length > 120 ? v : undefined}>{truncated}</span>
  }
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try { return JSON.stringify(v) } catch { return String(v) }
}

function renderCellLong(v: unknown): React.ReactNode {
  if (v === null || v === undefined) return <span className="db-null">NULL</span>
  if (typeof v === 'string') {
    if (v === '') return <span className="db-null">empty string</span>
    return <pre className="db-detail-pre">{v}</pre>
  }
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try { return <pre className="db-detail-pre">{JSON.stringify(v, null, 2)}</pre> } catch { return String(v) }
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}
