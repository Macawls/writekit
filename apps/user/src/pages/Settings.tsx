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
      </div>
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
