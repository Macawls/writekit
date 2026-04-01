import { useState } from 'react'
import { useStore } from '@nanostores/react'
import { $route, navigate, type Route } from '../stores/router'
import { $user, $site, logout } from '../stores/auth'

const navItems: { route: Route; label: string; icon: string }[] = [
  { route: 'site', label: 'Site', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1' },
  { route: 'team', label: 'Team', icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197' },
  { route: 'settings', label: 'Settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
  { route: 'billing', label: 'Billing', icon: 'M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z' },
]

function NavIcon({ path }: { path: string }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <path d={path} />
    </svg>
  )
}

export default function Layout({ children }: { children: React.ReactNode }) {
  const route = useStore($route)
  const user = useStore($user)
  const site = useStore($site)
  const [mobileOpen, setMobileOpen] = useState(false)

  const host = window.location.hostname.replace(/^app\./, '')
  const siteUrl = site ? `https://${site.ID}.${host}` : ''

  const handleNav = (r: Route) => {
    navigate(r)
    setMobileOpen(false)
  }

  const sidebar = (
    <>
      <div className="sidebar-brand">WriteKit</div>
      <nav className="sidebar-nav">
        {navItems.map(item => (
          <button
            key={item.route}
            className={`sidebar-link ${route === item.route ? 'active' : ''}`}
            onClick={() => handleNav(item.route)}
          >
            <NavIcon path={item.icon} />
            {item.label}
          </button>
        ))}
      </nav>
      <div className="sidebar-footer">
        {site && (
          <a href={siteUrl} target="_blank" rel="noopener noreferrer" className="sidebar-site-link">
            {site.ID}.{host}
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3" /></svg>
          </a>
        )}
        <div className="sidebar-user">
          <div className="sidebar-avatar">
            {user?.avatar_url
              ? <img src={user.avatar_url} alt="" />
              : <span>{(user?.name || user?.email || '?')[0].toUpperCase()}</span>
            }
          </div>
          <div className="sidebar-user-info">
            <span className="sidebar-user-name">{user?.name || user?.email}</span>
          </div>
          <button className="sidebar-logout" onClick={logout} title="Sign out">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4M16 17l5-5-5-5M21 12H9" /></svg>
          </button>
        </div>
      </div>
    </>
  )

  return (
    <div className="layout">
      <aside className="sidebar">{sidebar}</aside>

      {/* Mobile header */}
      <div className="mobile-header">
        <button className="mobile-menu-btn" onClick={() => setMobileOpen(true)}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" /></svg>
        </button>
        <span className="mobile-brand">WriteKit</span>
      </div>

      {/* Mobile drawer overlay */}
      {mobileOpen && (
        <div className="drawer-overlay" onClick={() => setMobileOpen(false)}>
          <aside className="drawer" onClick={e => e.stopPropagation()}>
            <button className="drawer-close" onClick={() => setMobileOpen(false)}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
            </button>
            {sidebar}
          </aside>
        </div>
      )}

      <main className="content">
        {children}
      </main>
    </div>
  )
}
