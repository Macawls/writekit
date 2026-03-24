import { useState } from 'react'
import { api, type Subscription } from '../api'

export default function Billing({ subscription }: { subscription: Subscription | null }) {
  const [loading, setLoading] = useState(false)

  const handleCheckout = async () => {
    setLoading(true)
    try {
      const { url } = await api.billingCheckout()
      window.location.href = url
    } catch {
      setLoading(false)
    }
  }

  const handlePortal = async () => {
    setLoading(true)
    try {
      const { url } = await api.billingPortal()
      window.location.href = url
    } catch {
      setLoading(false)
    }
  }

  const isActive = subscription?.Status === 'active'

  return (
    <>
      <h2>Billing</h2>
      <div className="card" style={{ marginTop: '1rem' }}>
        <h3>Subscription</h3>
        <p className="muted" style={{ marginTop: '0.5rem' }}>
          Status: <strong>{isActive ? 'Active' : 'Inactive'}</strong>
        </p>
        <div style={{ marginTop: '1rem' }}>
          {isActive ? (
            <button className="btn btn-outline" onClick={handlePortal} disabled={loading}>
              {loading ? 'Loading...' : 'Manage Subscription'}
            </button>
          ) : (
            <button className="btn" onClick={handleCheckout} disabled={loading}>
              {loading ? 'Loading...' : 'Subscribe'}
            </button>
          )}
        </div>
      </div>
    </>
  )
}
