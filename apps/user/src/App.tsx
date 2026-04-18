import { useEffect, useRef, useState } from 'react'
import { useStore } from '@nanostores/react'
import { $user, $site, $loading, $error, loadAuth } from './stores/auth'
import { $route } from './stores/router'
import Layout from './components/Layout'
import Onboarding from './pages/Onboarding'
import Site from './pages/Site'
import Team from './pages/Team'
import Settings from './pages/Settings'
import Billing from './pages/Billing'
import Graph from './pages/Graph'
import Connect from './pages/Connect'

const ONBOARDED_KEY = 'writekit_onboarded'
const POLL_INTERVAL = 2000
const MAX_RETRIES = 7

function Router() {
  const route = useStore($route)
  switch (route) {
    case 'site': return <Site />
    case 'team': return <Team />
    case 'settings': return <Settings />
    case 'billing': return <Billing />
    case 'graph': return <Graph />
    case 'connect': return <Connect />
    default: return <Site />
  }
}

export default function App() {
  const user = useStore($user)
  const site = useStore($site)
  const loading = useStore($loading)
  const error = useStore($error)
  const [view, setView] = useState<'loading' | 'onboarding' | 'dashboard'>('loading')
  const retriesRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const load = async () => {
    await loadAuth()
  }

  useEffect(() => {
    load()
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [])

  // Poll for site creation if needed
  useEffect(() => {
    if (loading) return

    if (!user) {
      setView('loading')
      return
    }

    if (!site) {
      if (retriesRef.current < MAX_RETRIES) {
        retriesRef.current++
        timerRef.current = setTimeout(load, POLL_INTERVAL)
      }
      return
    }

    if (!localStorage.getItem(ONBOARDED_KEY)) {
      setView('onboarding')
    } else {
      setView('dashboard')
    }
  }, [user, site, loading])

  const completeOnboarding = () => {
    localStorage.setItem(ONBOARDED_KEY, '1')
    setView('dashboard')
  }

  if (error) {
    return (
      <div className="centered">
        <p className="error">{error}</p>
      </div>
    )
  }

  if (view === 'loading' || loading || !user) {
    return (
      <div className="centered">
        <p className="muted">Loading...</p>
      </div>
    )
  }

  if (!site) {
    return (
      <div className="centered">
        <p className="muted">Setting up your site...</p>
      </div>
    )
  }

  if (view === 'onboarding') {
    return <Onboarding user={user} site={site} onComplete={completeOnboarding} />
  }

  return (
    <Layout>
      <Router />
    </Layout>
  )
}
