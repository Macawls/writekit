import { useEffect, useState } from 'react'
import { adminApi } from './api'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Users from './pages/Users'
import UserDetail from './pages/UserDetail'

type View = 'loading' | 'login' | 'dashboard' | 'users' | 'user-detail'

function getView(): View {
  const path = window.location.pathname
  if (path.startsWith('/users/')) return 'user-detail'
  if (path === '/users') return 'users'
  return 'dashboard'
}

function getUserId(): string {
  const match = window.location.pathname.match(/^\/users\/(.+)$/)
  return match ? match[1] : ''
}

export default function App() {
  const [view, setView] = useState<View>('loading')
  const [email, setEmail] = useState('')
  const [userId, setUserId] = useState('')

  useEffect(() => {
    adminApi.me()
      .then(data => {
        setEmail(data.email)
        const v = getView()
        setView(v)
        if (v === 'user-detail') setUserId(getUserId())
      })
      .catch(() => setView('login'))
  }, [])

  useEffect(() => {
    const onPop = () => {
      const v = getView()
      setView(v)
      if (v === 'user-detail') setUserId(getUserId())
    }
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const navigate = (path: string) => {
    window.history.pushState(null, '', path)
    const v = getView()
    setView(v)
    if (v === 'user-detail') setUserId(getUserId())
  }

  const handleLogout = async () => {
    await adminApi.logout()
    setView('login')
  }

  if (view === 'loading') return null
  if (view === 'login') return <Login onAuth={(e) => { setEmail(e); setView('dashboard') }} />

  return (
    <div className="shell">
      <div className="top-bar">
        <span className="logo">writekit admin</span>
        <nav>
          <a href="/" className={view === 'dashboard' ? 'active' : ''} onClick={e => { e.preventDefault(); navigate('/') }}>Dashboard</a>
          <a href="/users" className={view === 'users' || view === 'user-detail' ? 'active' : ''} onClick={e => { e.preventDefault(); navigate('/users') }}>Users</a>
          <span style={{ color: 'var(--faint)', fontSize: '.72rem' }}>{email}</span>
          <button onClick={handleLogout}>Logout</button>
        </nav>
      </div>

      {view === 'dashboard' && <Dashboard />}
      {view === 'users' && <Users onNavigate={navigate} />}
      {view === 'user-detail' && <UserDetail userId={userId} onNavigate={navigate} />}
    </div>
  )
}
