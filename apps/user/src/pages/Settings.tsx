import { useState } from 'react'
import { api, type User } from '../api'

export default function Settings({ user, onUpdate }: { user: User; onUpdate: () => void }) {
  const [name, setName] = useState(user.name)
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
      onUpdate()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update profile')
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <h2>Settings</h2>
      <div className="card" style={{ marginTop: '1rem' }}>
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
    </>
  )
}
