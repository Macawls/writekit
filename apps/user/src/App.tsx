import { useEffect, useState } from 'react'
import { api, type MeResponse } from './api'
import Onboarding from './pages/Onboarding'
import Dashboard from './pages/Dashboard'

type View = 'loading' | 'onboarding' | 'dashboard'

const ONBOARDED_KEY = 'writekit_onboarded'

export default function App() {
  const [view, setView] = useState<View>('loading')
  const [data, setData] = useState<MeResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      const me = await api.me()
      setData(me)

      if (!me.site) {
        setView('onboarding')
      } else if (!localStorage.getItem(ONBOARDED_KEY)) {
        setView('onboarding')
      } else {
        setView('dashboard')
      }
    } catch (e) {
      if (e instanceof Error && e.message !== 'unauthorized') {
        setError(e.message)
      }
    }
  }

  const completeOnboarding = () => {
    localStorage.setItem(ONBOARDED_KEY, '1')
    setView('dashboard')
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

  if (view === 'onboarding' && data.site) {
    return <Onboarding user={data.user} site={data.site} onComplete={completeOnboarding} />
  }

  if (!data.site) {
    return (
      <div className="container">
        <p className="muted">Setting up your site...</p>
      </div>
    )
  }

  return (
    <Dashboard
      user={data.user}
      site={data.site}
      subscription={data.subscription}
      onUpdate={load}
    />
  )
}
