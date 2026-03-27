import { useState } from 'react'
import { api, type User, type Site, type Subscription } from '../api'
import Settings from './Settings'
import Billing from './Billing'

type Tab = 'site' | 'settings' | 'billing'

interface Props {
  user: User
  site: Site
  subscription: Subscription | null
  onUpdate: () => void
}

export default function Dashboard({ user, site, subscription, onUpdate }: Props) {
  const [tab, setTab] = useState<Tab>('site')
  const [slug, setSlug] = useState(site.ID)
  const [slugError, setSlugError] = useState<string | null>(null)
  const [slugSuccess, setSlugSuccess] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const host = window.location.hostname.replace(/^app\./, '')
  const siteUrl = `https://${site.ID}.${host}`

  const handleLogout = () => {
    document.cookie = 'session=; path=/; max-age=0; domain=.' + host
    window.location.href = `${window.location.protocol}//${host}`
  }

  const handleSlugChange = async (e: React.FormEvent) => {
    e.preventDefault()
    if (slug === site.ID) return
    setSlugError(null)
    setSlugSuccess(null)
    setSaving(true)
    try {
      await api.updateSlug(slug)
      setSlugSuccess('Subdomain updated!')
      onUpdate()
    } catch (err) {
      setSlugError(err instanceof Error ? err.message : 'Failed to update slug')
      setSlug(site.ID)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="container">
      <header>
        <h1>WriteKit</h1>
        <nav>
          <button className={tab === 'site' ? 'active' : ''} onClick={() => setTab('site')}>Site</button>
          <button className={tab === 'settings' ? 'active' : ''} onClick={() => setTab('settings')}>Settings</button>
          <button className={tab === 'billing' ? 'active' : ''} onClick={() => setTab('billing')}>Billing</button>
          <button onClick={handleLogout}>Logout</button>
        </nav>
      </header>

      {tab === 'site' && (
        <>
          <h2>{site.Name}</h2>
          <a href={siteUrl} target="_blank" rel="noopener noreferrer" className="blog-link">
            {site.ID}.{host} &rarr;
          </a>

          <div className="card" style={{ marginTop: '2rem' }}>
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
      )}

      {tab === 'settings' && <Settings user={user} onUpdate={onUpdate} />}
      {tab === 'billing' && <Billing subscription={subscription} />}
    </div>
  )
}
