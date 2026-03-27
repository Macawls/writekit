import { useEffect, useState } from 'react'
import { adminApi, type Stats, type User, type TenantStorage } from '../api'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

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

      <div className="stat-grid" style={{ gridTemplateColumns: '1fr' }}>
        <div className="stat-card">
          <div className="label">Total Storage</div>
          <div className="value">{formatBytes(stats.total_storage_bytes)}</div>
        </div>
      </div>

      {stats.tenant_storage && stats.tenant_storage.length > 0 && (
        <>
          <h3 className="section-title">Storage by site</h3>
          <table>
            <thead>
              <tr>
                <th>Site</th>
                <th>Name</th>
                <th style={{ textAlign: 'right' }}>Size</th>
              </tr>
            </thead>
            <tbody>
              {stats.tenant_storage
                .sort((a: TenantStorage, b: TenantStorage) => b.bytes - a.bytes)
                .map((t: TenantStorage) => (
                <tr key={t.id}>
                  <td>{t.id}</td>
                  <td>{t.name}</td>
                  <td style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{formatBytes(t.bytes)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      <h3 className="section-title" style={{ marginTop: '2rem' }}>Recent signups</h3>
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
