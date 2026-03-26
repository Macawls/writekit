import { useEffect, useState } from 'react'
import { adminApi, type Stats, type User } from '../api'

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)

  useEffect(() => {
    adminApi.stats().then(setStats)
  }, [])

  if (!stats) return <p className="msg">Loading...</p>

  return (
    <>
      <div className="stat-grid">
        <div className="stat-card">
          <div className="label">Users</div>
          <div className="value">{stats.total_users}</div>
        </div>
        <div className="stat-card">
          <div className="label">Sites</div>
          <div className="value">{stats.total_tenants}</div>
        </div>
        <div className="stat-card">
          <div className="label">Active Subs</div>
          <div className="value">{stats.active_subscriptions}</div>
        </div>
      </div>

      <h3 className="section-title">Recent signups</h3>
      <table>
        <thead>
          <tr>
            <th>Email</th>
            <th>Name</th>
            <th>Joined</th>
          </tr>
        </thead>
        <tbody>
          {stats.recent_users?.map((u: User) => (
            <tr key={u.ID}>
              <td>{u.Email}</td>
              <td>{u.Name || <span style={{ color: 'var(--faint)' }}>—</span>}</td>
              <td>{new Date(u.CreatedAt).toLocaleDateString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  )
}
