import { useEffect, useState } from 'react'
import { useStore } from '@nanostores/react'
import { $user, $isDesktop, loadAuth } from '../stores/auth'
import { api, type DesktopSettings } from '../api'

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

      {isDesktop && <DesktopSection />}
    </>
  )
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
        const ok = confirm(
          `That folder already contains WriteKit data.\n\nUse the existing data at:\n${res.path}\n\nYour current data will be left in place — you can switch back by selecting the old folder.`,
        )
        if (!ok) return
      } else {
        const ok = confirm(
          `Set data folder to:\n${res.path}\n\nWriteKit will use this folder for new content after restart. Existing data will remain at the old location.`,
        )
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
    if (!confirm('Reset data folder to the default location? You will need to restart WriteKit.')) return
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
