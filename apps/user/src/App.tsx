import { useEffect, useRef, useState } from 'react'
import { api, type MeResponse } from './api'
import Onboarding from './pages/Onboarding'
import Dashboard from './pages/Dashboard'

type View = 'loading' | 'onboarding' | 'dashboard'

const ONBOARDED_KEY = 'writekit_onboarded'
const POLL_INTERVAL = 2000
const MAX_RETRIES = 7

export default function App() {
  const [view, setView] = useState<View>('loading')
  const [data, setData] = useState<MeResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const retriesRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const load = async () => {
    try {
      const me = await api.me()
      setData(me)

      if (!me.site) {
        if (retriesRef.current < MAX_RETRIES) {
          retriesRef.current++
          timerRef.current = setTimeout(load, POLL_INTERVAL)
        } else {
          setError('Site setup is taking longer than expected. Please refresh the page.')
        }
        return
      }

      if (!localStorage.getItem(ONBOARDED_KEY)) {
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

  useEffect(() => {
    load()
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [])

  if (error) {
    return (
      <div className="container">
        <p className="error">{error}</p>
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
