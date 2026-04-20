import { useEffect, useState } from 'react'
import { useStore } from '@nanostores/react'
import { $user, $site, $isDesktop, loadAuth } from '../stores/auth'
import { api, type DesktopSettings } from '../api'
import { $embeddingPrefs, setEmbeddingPrefs } from '../embedding/settings'
import { $embeddingStatus, embeddingController } from '../embedding/controller'
import { MODELS, findModel } from '../embedding/models'
import { fetchEmbeddingSource } from '../api/graph'
import { confirmDialog } from '../components/ConfirmDialog'
import { Select } from '../components/Select'

export default function Settings() {
  const user = useStore($user)
  const isDesktop = useStore($isDesktop)
  const [name, setName] = useState(user?.name ?? '')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)
    setSaving(true)
    try {
      await api.updateProfile(name)
      setSuccess('Profile updated!')
      loadAuth()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update profile')
    } finally {
      setSaving(false)
    }
  }

  if (!user) return null

  return (
    <>
      <h2>Settings</h2>
      <div className="card" style={{ marginTop: '1.5rem' }}>
        <h3>Profile</h3>
        <form onSubmit={handleSubmit} style={{ marginTop: '0.5rem' }}>
          <div className="form-group">
            <label htmlFor="email">Email</label>
            <input id="email" type="email" value={user.email} disabled style={{ opacity: 0.6 }} />
          </div>
          <div className="form-group">
            <label htmlFor="name">Name</label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={e => {
                setName(e.target.value)
                setSuccess(null)
              }}
              required
            />
          </div>
          {error && <p className="error">{error}</p>}
          {success && <p className="success">{success}</p>}
          <button type="submit" className="btn" disabled={saving || name === user.name}>
            {saving ? 'Saving...' : 'Save'}
          </button>
        </form>
      </div>

      <SemanticGraphSection />

      {isDesktop && <DesktopSection />}
    </>
  )
}

function SemanticGraphSection() {
  const prefs = useStore($embeddingPrefs)
  const status = useStore($embeddingStatus)
  const site = useStore($site)
  const [error, setError] = useState<string | null>(null)
  const current = findModel(prefs.modelId)

  const enable = async (modelId: string) => {
    if (!site?.ID) return
    setError(null)
    const model = findModel(modelId)
    if (!model) return
    const ok = await confirmDialog({
      title: 'Enable semantic graph?',
      body: (
        <>
          This downloads <strong>{model.label}</strong> (~{model.approxSizeMB}MB) to this browser. It runs locally — your content stays on your machine.
        </>
      ),
      confirmLabel: 'Download & enable',
    })
    if (!ok) return
    setEmbeddingPrefs({ enabled: true, modelId })
    try {
      await embeddingController.start(site.ID, modelId)
      const sources = await fetchEmbeddingSource()
      embeddingController.syncPages(sources)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to enable')
    }
  }

  const disable = async () => {
    setEmbeddingPrefs({ ...prefs, enabled: false })
    await embeddingController.stop()
  }

  const switchModel = async (modelId: string) => {
    if (!site?.ID) return
    setError(null)
    setEmbeddingPrefs({ ...prefs, modelId })
    try {
      await embeddingController.start(site.ID, modelId)
      const sources = await fetchEmbeddingSource()
      embeddingController.syncPages(sources)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to switch model')
    }
  }

  const clearCache = async () => {
    const ok = await confirmDialog({
      title: 'Clear cached embeddings?',
      body: 'They\u2019ll be recomputed next time you open the graph.',
      confirmLabel: 'Clear',
      destructive: true,
    })
    if (!ok) return
    await embeddingController.clear()
  }

  const progressPct = status.loaded && status.total
    ? Math.round((status.loaded / status.total) * 100)
    : null

  return (
    <div className="card" style={{ marginTop: '1.5rem' }}>
      <h3>Semantic graph</h3>
      <p style={{ marginTop: '.25rem', fontSize: '.85rem', color: 'var(--muted)' }}>
        Generate embeddings in this browser to power the Graph view. Models run locally — content never leaves your machine.
      </p>

      {error && <p className="error" style={{ marginTop: '.5rem' }}>{error}</p>}

      {!prefs.enabled ? (
        <div style={{ marginTop: '.85rem', display: 'flex', flexDirection: 'column', gap: '.5rem' }}>
          <label style={{ fontSize: '.85rem', fontWeight: 500 }}>Model</label>
          <Select
            value={prefs.modelId}
            onChange={v => setEmbeddingPrefs({ ...prefs, modelId: v })}
            ariaLabel="Embedding model"
            options={MODELS.map(m => ({
              value: m.id,
              label: m.label,
              hint: `${m.dims}d · ~${m.approxSizeMB}MB`,
            }))}
            className="settings-model-select"
          />
          {current && (
            <div style={{ fontSize: '.78rem', color: 'var(--muted)' }}>{current.description}</div>
          )}
          <button type="button" className="btn" onClick={() => enable(prefs.modelId)} style={{ alignSelf: 'flex-start', marginTop: '.35rem' }}>
            Enable
          </button>
        </div>
      ) : (
        <div style={{ marginTop: '.85rem', display: 'flex', flexDirection: 'column', gap: '.65rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '.5rem' }}>
            <label style={{ fontSize: '.85rem', fontWeight: 500, flexShrink: 0 }}>Model</label>
            <Select
              value={prefs.modelId}
              onChange={v => switchModel(v)}
              ariaLabel="Embedding model"
              options={MODELS.map(m => ({
                value: m.id,
                label: m.label,
                hint: `${m.dims}d · ~${m.approxSizeMB}MB`,
              }))}
              className="settings-model-select"
            />
          </div>
          {current && (
            <div style={{ fontSize: '.78rem', color: 'var(--muted)' }}>{current.description}</div>
          )}

          <StatusRow status={status} progressPct={progressPct} />

          <div style={{ display: 'flex', gap: '.5rem', marginTop: '.25rem' }}>
            <button type="button" onClick={clearCache}>Clear cached embeddings</button>
            <button type="button" onClick={disable}>Disable</button>
          </div>
        </div>
      )}
    </div>
  )
}

function StatusRow({ status, progressPct }: { status: ReturnType<typeof $embeddingStatus.get>; progressPct: number | null }) {
  if (status.state === 'loading') {
    return (
      <div style={{ fontSize: '.8rem', color: 'var(--muted)' }}>
        {status.message ?? 'Loading…'}{progressPct !== null && ` · ${progressPct}%`}
      </div>
    )
  }
  if (status.state === 'error') {
    return <div className="error" style={{ fontSize: '.8rem' }}>{status.message ?? 'failed'}</div>
  }
  if (status.state === 'ready') {
    return (
      <div style={{ fontSize: '.8rem', color: 'var(--muted)' }}>
        Embedded {status.embedded} of {status.total_pages} pages
        {status.pending > 0 && ` · ${status.pending} in queue`}
      </div>
    )
  }
  return null
}

function DesktopSection() {
  const [settings, setSettings] = useState<DesktopSettings | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.getDesktopSettings().then(setSettings).catch(e => setError(e instanceof Error ? e.message : 'failed'))
  }, [])

  const update = async (patch: Partial<DesktopSettings>) => {
    if (!settings) return
    const next = { ...settings, ...patch }
    setSettings(next)
    setBusy(true)
    setError(null)
    try {
      const saved = await api.updateDesktopSettings(next)
      setSettings(saved)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to save')
      // Revert on error
      api.getDesktopSettings().then(setSettings).catch(() => {})
    } finally {
      setBusy(false)
    }
  }

  if (!settings) return null

  return (
    <div className="card" style={{ marginTop: '1.5rem' }}>
      <h3>Desktop</h3>
      {error && <p className="error">{error}</p>}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '.85rem', marginTop: '.75rem' }}>
        <Toggle
          label="Launch WriteKit when I sign in"
          description="Starts WriteKit automatically on login — the local MCP server is ready whenever you open Claude or Cursor."
          checked={settings.autostart}
          disabled={busy}
          onChange={v => update({ autostart: v })}
        />
        <Toggle
          label="Close to tray"
          description="Closing the window hides WriteKit to the system tray instead of quitting. Click the tray icon to bring it back."
          checked={settings.close_to_tray}
          disabled={busy}
          onChange={v => update({ close_to_tray: v })}
        />
        <DataFolderRow settings={settings} busy={busy} update={update} />
      </div>
    </div>
  )
}

function DataFolderRow({
  settings,
  busy,
  update,
}: {
  settings: DesktopSettings
  busy: boolean
  update: (patch: Partial<DesktopSettings>) => Promise<void>
}) {
  const [picking, setPicking] = useState(false)
  const [pickError, setPickError] = useState<string | null>(null)
  const [restart, setRestart] = useState(false)

  const choose = async () => {
    setPickError(null)
    setPicking(true)
    try {
      const res = await api.pickDataFolder()
      if (!res.path) return
      if (res.has_existing_data) {
        const ok = await confirmDialog({
          title: 'Use existing data?',
          body: (
            <>
              That folder already contains WriteKit data:
              <br /><code style={{ fontSize: '.78rem' }}>{res.path}</code>
              <br /><br />
              Your current data will be left in place — you can switch back by selecting the old folder.
            </>
          ),
          confirmLabel: 'Use this folder',
        })
        if (!ok) return
      } else {
        const ok = await confirmDialog({
          title: 'Set data folder?',
          body: (
            <>
              WriteKit will use this folder for new content after restart:
              <br /><code style={{ fontSize: '.78rem' }}>{res.path}</code>
              <br /><br />
              Existing data will remain at the old location.
            </>
          ),
          confirmLabel: 'Set folder',
        })
        if (!ok) return
      }
      await update({ data_dir: res.path })
      setRestart(true)
    } catch (e) {
      setPickError(e instanceof Error ? e.message : 'failed to pick folder')
    } finally {
      setPicking(false)
    }
  }

  const reset = async () => {
    const ok = await confirmDialog({
      title: 'Reset to default folder?',
      body: 'You will need to restart WriteKit for the change to take effect.',
      confirmLabel: 'Reset',
    })
    if (!ok) return
    try {
      await update({ data_dir: '' })
      setRestart(true)
    } catch (e) {
      setPickError(e instanceof Error ? e.message : 'failed to reset')
    }
  }

  const shown = settings.data_dir || settings.effective_data_dir

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '.35rem' }}>
      <div style={{ fontWeight: 500 }}>Data folder</div>
      <div style={{ fontSize: '.875rem', color: 'var(--muted, #666)' }}>
        Where your data lives on this computer. Stick with the default unless you have a reason to move it — e.g., you want it on a different drive or you manage your own backups.
      </div>
      <div
        style={{
          marginTop: '.25rem',
          padding: '.5rem .65rem',
          background: 'var(--bg-subtle, #f5f5f5)',
          borderRadius: 4,
          fontFamily: 'ui-monospace, monospace',
          fontSize: '.8rem',
          wordBreak: 'break-all',
        }}
      >
        {shown}
        {!settings.data_dir && <span style={{ marginLeft: '.5rem', fontStyle: 'italic', opacity: 0.7 }}>(default)</span>}
      </div>
      <div style={{ display: 'flex', gap: '.5rem', marginTop: '.25rem' }}>
        <button type="button" onClick={choose} disabled={busy || picking}>
          {picking ? 'Choose…' : 'Change…'}
        </button>
        {settings.data_dir && (
          <button type="button" onClick={reset} disabled={busy || picking}>
            Reset to default
          </button>
        )}
      </div>
      {pickError && <p className="error" style={{ marginTop: '.25rem' }}>{pickError}</p>}
      {restart && (
        <p style={{ marginTop: '.25rem', color: 'var(--warn, #b45309)' }}>
          Quit and reopen WriteKit for the new data folder to take effect.
        </p>
      )}
    </div>
  )
}

function Toggle({
  label,
  description,
  checked,
  disabled,
  onChange,
}: {
  label: string
  description: string
  checked: boolean
  disabled: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <label style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', cursor: disabled ? 'not-allowed' : 'pointer', opacity: disabled ? 0.55 : 1 }}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={e => onChange(e.target.checked)}
        style={{ marginTop: '.25rem', flexShrink: 0 }}
      />
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: '.9rem', fontWeight: 500 }}>{label}</div>
        <div style={{ fontSize: '.78rem', color: 'var(--muted)', marginTop: '.15rem', lineHeight: 1.4 }}>{description}</div>
      </div>
    </label>
  )
}
