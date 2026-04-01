import { useState } from 'react'
import { useStore } from '@nanostores/react'
import { $site, loadAuth } from '../stores/auth'
import { api } from '../api'

export default function Site() {
  const site = useStore($site)

  const [slug, setSlug] = useState(site?.ID ?? '')
  const [slugError, setSlugError] = useState<string | null>(null)
  const [slugSuccess, setSlugSuccess] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const host = window.location.hostname.replace(/^app\./, '')
  const siteUrl = site ? `https://${site.ID}.${host}` : ''

  const handleSlugChange = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!site || slug === site.ID) return
    setSlugError(null)
    setSlugSuccess(null)
    setSaving(true)
    try {
      await api.updateSlug(slug)
      setSlugSuccess('Subdomain updated!')
      loadAuth()
    } catch (err) {
      setSlugError(err instanceof Error ? err.message : 'Failed to update slug')
      setSlug(site.ID)
    } finally {
      setSaving(false)
    }
  }

  if (!site) return null

  return (
    <>
      <h2>Site</h2>
      <p className="muted" style={{ marginTop: '.25rem' }}>{site.Name}</p>

      <a href={siteUrl} target="_blank" rel="noopener noreferrer" className="site-link">
        {site.ID}.{host}
        <span>&rarr;</span>
      </a>

      <div className="card" style={{ marginTop: '1.5rem' }}>
        <h3>Change subdomain</h3>
        <form onSubmit={handleSlugChange} style={{ marginTop: '0.5rem' }}>
          <div className="form-group">
            <div className="slug-input">
              <input
                type="text"
                value={slug}
                onChange={e => {
                  setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))
                  setSlugError(null)
                  setSlugSuccess(null)
                }}
                pattern="[a-z0-9][a-z0-9-]{1,62}[a-z0-9]"
                required
              />
              <span>.{host}</span>
            </div>
          </div>
          {slugError && <p className="error">{slugError}</p>}
          {slugSuccess && <p className="success">{slugSuccess}</p>}
          <button type="submit" className="btn" disabled={saving || slug === site.ID}>
            {saving ? 'Saving...' : 'Update'}
          </button>
        </form>
      </div>
    </>
  )
}
