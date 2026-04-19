import { useEffect, useState } from 'react'
import { useStore } from '@nanostores/react'
import { $isDesktop } from '../stores/auth'
import { api, type DesktopSettings } from '../api'

export default function DataWelcome() {
  const isDesktop = useStore($isDesktop)
  const [settings, setSettings] = useState<DesktopSettings | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isDesktop) return
    api.getDesktopSettings().then(setSettings).catch(() => {})
  }, [isDesktop])

  if (!isDesktop || !settings || settings.onboarding_complete) return null

  const dismiss = async (dataDir?: string) => {
    setError(null)
    setBusy(true)
    try {
      const next = { ...settings, onboarding_complete: true } as DesktopSettings
      if (dataDir !== undefined) next.data_dir = dataDir
      const saved = await api.updateDesktopSettings(next)
      setSettings(saved)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  const pick = async () => {
    setError(null)
    setBusy(true)
    try {
      const res = await api.pickDataFolder()
      if (!res.path) {
        setBusy(false)
        return
      }
      await dismiss(res.path)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed')
      setBusy(false)
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
        padding: '1rem',
      }}
    >
      <div
        style={{
          background: 'var(--bg, #fff)',
          borderRadius: 8,
          maxWidth: 520,
          width: '100%',
          padding: '1.5rem',
          boxShadow: '0 20px 50px rgba(0,0,0,0.25)',
        }}
      >
        <h2 style={{ marginTop: 0, marginBottom: '.5rem' }}>Welcome to WriteKit</h2>
        <p style={{ marginTop: 0, color: 'var(--muted, #555)' }}>
          Your pages, settings, and embeddings live in a local folder on this machine. Choose where to keep them.
        </p>
        <div
          style={{
            marginTop: '1rem',
            padding: '.65rem .8rem',
            background: 'var(--bg-subtle, #f5f5f5)',
            borderRadius: 4,
            fontFamily: 'ui-monospace, monospace',
            fontSize: '.8rem',
            wordBreak: 'break-all',
          }}
        >
          {settings.effective_data_dir}
        </div>
        <p style={{ marginTop: '.75rem', fontSize: '.85rem', color: 'var(--muted, #666)' }}>
          The default is fine for most people. Change it later in Settings if you want your data on a different drive.
        </p>
        {error && <p className="error">{error}</p>}
        <div style={{ display: 'flex', gap: '.5rem', justifyContent: 'flex-end', marginTop: '1.25rem' }}>
          <button type="button" onClick={pick} disabled={busy}>
            Choose a different folder…
          </button>
          <button type="button" onClick={() => dismiss()} disabled={busy} style={{ fontWeight: 600 }}>
            Use this folder
          </button>
        </div>
      </div>
    </div>
  )
}
