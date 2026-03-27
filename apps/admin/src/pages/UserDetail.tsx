import { useEffect, useState } from 'react'
import { adminApi, type UserDetail as UserDetailType } from '../api'

export default function UserDetail({ userId, onNavigate }: { userId: string; onNavigate: (path: string) => void }) {
  const [data, setData] = useState<UserDetailType | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    adminApi.getUser(userId).then(setData)
  }, [userId])

  const handleDelete = async () => {
    setDeleting(true)
    try {
      await adminApi.deleteUser(userId)
      onNavigate('/users')
    } catch {
      setDeleting(false)
      setShowConfirm(false)
    }
  }

  if (!data) return <p className="msg">Loading...</p>

  const { user, tenant, linked_accounts, subscription } = data

  return (
    <>
      <a href="/users" className="back-link" onClick={e => { e.preventDefault(); onNavigate('/users') }}>&larr; Back to users</a>

      <div className="detail-card">
        <h3>User</h3>
        <div className="detail-row"><span className="label">ID</span><span>{user.ID}</span></div>
        <div className="detail-row"><span className="label">Email</span><span>{user.Email}</span></div>
        <div className="detail-row"><span className="label">Name</span><span>{user.Name || '—'}</span></div>
        <div className="detail-row"><span className="label">Joined</span><span>{new Date(user.CreatedAt).toLocaleString()}</span></div>
      </div>

      <div className="detail-card">
        <h3>Site</h3>
        {tenant ? (
          <>
            <div className="detail-row"><span className="label">Subdomain</span><span>{tenant.ID}</span></div>
            <div className="detail-row"><span className="label">Name</span><span>{tenant.Name}</span></div>
          </>
        ) : (
          <p className="msg">No site created</p>
        )}
      </div>

      <div className="detail-card">
        <h3>Linked Accounts</h3>
        {linked_accounts.length > 0 ? (
          linked_accounts.map(a => (
            <div className="detail-row" key={a.ID}>
              <span className="label">{a.Provider}</span>
              <span>
                {a.Email}
                {a.EmailVerified && <span className="badge badge-green" style={{ marginLeft: '.5rem' }}>verified</span>}
              </span>
            </div>
          ))
        ) : (
          <p className="msg">No linked accounts</p>
        )}
      </div>

      <div className="detail-card">
        <h3>Subscription</h3>
        {subscription ? (
          <>
            <div className="detail-row">
              <span className="label">Status</span>
              <span>
                <span className={`badge ${subscription.Status === 'active' ? 'badge-green' : 'badge-gray'}`}>
                  {subscription.Status}
                </span>
              </span>
            </div>
            {subscription.CurrentPeriodEnd && (
              <div className="detail-row">
                <span className="label">Period End</span>
                <span>{new Date(subscription.CurrentPeriodEnd).toLocaleDateString()}</span>
              </div>
            )}
          </>
        ) : (
          <p className="msg">No subscription</p>
        )}
      </div>

      <button className="btn btn-danger btn-small" onClick={() => setShowConfirm(true)}>Delete User</button>

      {showConfirm && (
        <div className="confirm-overlay" onClick={() => setShowConfirm(false)}>
          <div className="confirm-box" onClick={e => e.stopPropagation()}>
            <h3>Delete {user.Email}?</h3>
            <p>This will permanently delete the user, their site, and all content. This cannot be undone.</p>
            <div className="confirm-actions">
              <button className="btn btn-small" style={{ background: 'var(--border)', color: 'var(--fg)' }} onClick={() => setShowConfirm(false)}>Cancel</button>
              <button className="btn btn-danger btn-small" onClick={handleDelete} disabled={deleting}>
                {deleting ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
