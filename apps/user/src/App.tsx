import { useEffect, useState } from 'react'
import { api, type MeResponse } from './api'
import Onboarding from './pages/Onboarding'
import Dashboard from './pages/Dashboard'

type View = 'loading' | 'onboarding' | 'dashboard'

export default function App() {
  const [view, setView] = useState<View>('loading')
  const [data, setData] = useState<MeResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const me = await api.me()
      setData(me)
      setView(me.blog ? 'dashboard' : 'onboarding')
    } catch (e) {
      if (e instanceof Error && e.message !== 'unauthorized') {
        setError(e.message)
      }
    }
  }

  useEffect(() => { load() }, [])

  if (error) {
    return (
      <div className="container">
        <p className="error">Something went wrong: {error}</p>
      </div>
    )
  }

  if (view === 'loading' || !data) {
    return (
      <div className="container">
        <p className="muted">Loading...</p>
      </div>
    )
  }

  if (view === 'onboarding') {
    return <Onboarding user={data.user} onComplete={load} />
  }

  return (
    <Dashboard
      user={data.user}
      blog={data.blog!}
      subscription={data.subscription}
      onUpdate={load}
    />
  )
}
