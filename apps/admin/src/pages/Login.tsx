import { useState } from 'react'
import { adminApi } from '../api'

export default function Login({ onAuth }: { onAuth: (email: string) => void }) {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Check if we just came back from a verify redirect
  const params = new URLSearchParams(window.location.search)
  if (params.get('error')) {
    if (!error) setError(params.get('error') === 'unauthorized' ? 'Not authorized.' : 'Invalid or expired link.')
  }

  // After verify redirect, check if session is now valid
  if (window.location.pathname === '/') {
    adminApi.me().then(data => onAuth(data.email)).catch(() => {})
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      await adminApi.sendLink(email)
      setSent(true)
    } catch {
      setError('Failed to send. Try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-wrap">
      <div className="login-card">
        <span className="logo">writekit admin</span>

        {sent ? (
          <>
            <h2>Check your email</h2>
            <p className="msg">We sent a sign-in link to <strong>{email}</strong></p>
          </>
        ) : (
          <>
            <h2>Sign in</h2>
            {error && <p className="msg error">{error}</p>}
            <form onSubmit={handleSubmit}>
              <input
                type="email"
                className="input"
                placeholder="admin@example.com"
                value={email}
                onChange={e => setEmail(e.target.value)}
                required
                autoFocus
              />
              <button type="submit" className="btn" disabled={loading}>
                {loading ? 'Sending...' : 'Continue with email'}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  )
}
