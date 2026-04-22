import { useEffect, useState } from 'react'
import { api, type ClientInfo, type LocalInfo } from '../api'

export default function Connect() {
  const [info, setInfo] = useState<LocalInfo | null>(null)
  const [clients, setClients] = useState<ClientInfo[] | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [justConnected, setJustConnected] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const load = async () => {
    try {
      const [i, cs] = await Promise.all([api.localInfo(), api.listClients()])
      setInfo(i)
      setClients(cs)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to load')
    }
  }

  useEffect(() => { load() }, [])

  const connect = async (c: ClientInfo) => {
    setBusy(c.id)
    setError(null)
    try {
      await api.connectClient(c.id)
      setJustConnected(c.id)
      setTimeout(() => setJustConnected(null), 4000)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to connect')
    } finally {
      setBusy(null)
    }
  }

  const disconnect = async (c: ClientInfo) => {
    setBusy(c.id)
    setError(null)
    try {
      await api.disconnectClient(c.id)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to disconnect')
    } finally {
      setBusy(null)
    }
  }

  const copy = () => {
    if (!info) return
    navigator.clipboard.writeText(info.mcp_url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  if (!info || !clients) {
    return <><h2>Connect</h2><p className="muted">Loading...</p></>
  }

  const autoDetected = clients.filter(c => c.detected && !c.manual)
  const manualClients = clients.filter(c => c.manual)
  const undetected = clients.filter(c => !c.detected && !c.manual)

  return (
    <>
      <h2>Connect</h2>
      <p className="muted" style={{ marginTop: '.25rem', maxWidth: 540 }}>
        Add WriteKit to any AI assistant on this machine. One click writes the MCP config — restart the client to see WriteKit's tools.
      </p>

      <div className="card" style={{ marginTop: '1.25rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: '.72rem', color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', marginBottom: '.35rem' }}>
              Local MCP endpoint
            </div>
            <code style={{ fontSize: '.85rem', wordBreak: 'break-all' }}>{info.mcp_url}</code>
          </div>
          <button className="btn-secondary" onClick={copy}>
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
      </div>

      {error && (
        <div className="card" style={{ marginTop: '1rem', borderColor: '#ef4444', color: '#ef4444' }}>
          {error}
        </div>
      )}

      <h3 style={{ marginTop: '2rem' }}>Detected on this machine</h3>
      {autoDetected.length === 0 && (
        <p className="muted" style={{ marginTop: '.5rem' }}>No MCP-compatible clients found. Install one of the options below to get started.</p>
      )}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '.5rem', marginTop: '.75rem' }}>
        {autoDetected.map(c => (
          <ClientRow
            key={c.id}
            client={c}
            busy={busy === c.id}
            justConnected={justConnected === c.id}
            onConnect={() => connect(c)}
            onDisconnect={() => disconnect(c)}
          />
        ))}
      </div>

      {manualClients.length > 0 && (
        <>
          <h3 style={{ marginTop: '2rem' }}>Manual setup</h3>
          <p className="muted" style={{ marginTop: '.25rem', fontSize: '.82rem' }}>These clients don't expose a config file — paste the MCP URL using their in-app settings.</p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '.5rem', marginTop: '.75rem' }}>
            {manualClients.map(c => (
              <ManualRow key={c.id} client={c} />
            ))}
          </div>
        </>
      )}

      {undetected.length > 0 && (
        <>
          <h3 style={{ marginTop: '2rem' }}>Other clients</h3>
          <p className="muted" style={{ marginTop: '.25rem', fontSize: '.82rem' }}>Install any of these to connect — WriteKit will detect them automatically.</p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '.5rem', marginTop: '.75rem' }}>
            {undetected.map(c => (
              <div key={c.id} className="card" style={{ padding: '.85rem 1rem', opacity: .6 }}>
                <div style={{ fontWeight: 500 }}>{c.name}</div>
                <div className="muted" style={{ fontSize: '.75rem', marginTop: '.2rem' }}>Not installed</div>
              </div>
            ))}
          </div>
        </>
      )}
    </>
  )
}

function ClientRow({
  client,
  busy,
  justConnected,
  onConnect,
  onDisconnect,
}: {
  client: ClientInfo
  busy: boolean
  justConnected: boolean
  onConnect: () => void
  onDisconnect: () => void
}) {
  return (
    <div className="card" style={{ padding: '.85rem 1rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}>
            <span style={{ fontWeight: 500 }}>{client.name}</span>
            {client.connected && <StatusPill color="#16a34a">Connected</StatusPill>}
            {client.requires_npx && <StatusPill color="#a1a1aa">Needs Node</StatusPill>}
          </div>
          <div className="muted" style={{ fontSize: '.72rem', marginTop: '.2rem', fontFamily: 'ui-monospace, monospace', wordBreak: 'break-all' }}>
            {client.config_path}
          </div>
          {justConnected && (
            <div style={{ fontSize: '.78rem', color: '#16a34a', marginTop: '.4rem' }}>
              Added. Restart {client.name} to see WriteKit's tools.
            </div>
          )}
        </div>
        <div style={{ flexShrink: 0 }}>
          {client.connected ? (
            <button className="btn-secondary" onClick={onDisconnect} disabled={busy}>
              {busy ? '...' : 'Remove'}
            </button>
          ) : (
            <button className="btn" onClick={onConnect} disabled={busy}>
              {busy ? '...' : 'Connect'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

function ManualRow({ client }: { client: ClientInfo }) {
  return (
    <div className="card" style={{ padding: '.85rem 1rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}>
        <span style={{ fontWeight: 500 }}>{client.name}</span>
        <StatusPill color="#a1a1aa">Manual</StatusPill>
      </div>
      {client.instructions && client.instructions.length > 0 && (
        <ol style={{ margin: '.55rem 0 0', paddingLeft: '1.1rem', fontSize: '.82rem', color: 'var(--muted)', lineHeight: 1.5 }}>
          {client.instructions.map((step, i) => <li key={i}>{step}</li>)}
        </ol>
      )}
    </div>
  )
}

function StatusPill({ color, children }: { color: string; children: React.ReactNode }) {
  return (
    <span style={{
      fontSize: '.62rem',
      fontWeight: 600,
      letterSpacing: '.05em',
      textTransform: 'uppercase',
      color,
      border: `1px solid ${color}`,
      borderRadius: 4,
      padding: '.1rem .35rem',
      lineHeight: 1,
    }}>{children}</span>
  )
}
