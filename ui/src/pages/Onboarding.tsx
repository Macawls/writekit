import { useState } from 'react'
import { api, type User } from '../api'

export default function Onboarding({ user, onComplete }: { user: User; onComplete: () => void }) {
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const host = window.location.hostname.replace(/^app\./, '')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await api.createBlog(slug, name || slug)
      onComplete()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create blog')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="container">
      <header>
        <h1>WriteKit</h1>
      </header>

      <h2>Welcome, {user.name || user.email}!</h2>
      <p className="muted" style={{ marginTop: '0.5rem', marginBottom: '2rem' }}>
        Choose a subdomain for your blog. You can change this later.
      </p>

      <div className="card">
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="slug">Subdomain</label>
            <div className="slug-input">
              <input
                id="slug"
                type="text"
                value={slug}
                onChange={e => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                placeholder="my-blog"
                required
                pattern="[a-z0-9][a-z0-9-]{1,62}[a-z0-9]"
              />
              <span>.{host}</span>
            </div>
          </div>
          <div className="form-group">
            <label htmlFor="name">Blog name</label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="My Blog"
            />
          </div>
          {error && <p className="error">{error}</p>}
          <button type="submit" className="btn" disabled={loading} style={{ marginTop: '0.5rem' }}>
            {loading ? 'Creating...' : 'Create Blog'}
          </button>
        </form>
      </div>
    </div>
  )
}
