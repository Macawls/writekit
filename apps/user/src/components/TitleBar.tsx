import { useEffect, useState } from 'react'
import { useStore } from '@nanostores/react'
import { $route, $pageSlug, navigate, type Route } from '../stores/router'
import { api } from '../api'

type Platform = 'darwin' | 'windows' | 'linux' | ''

declare global {
  interface Window {
    runtime?: {
      Environment: () => Promise<{ platform: string }>
      WindowMinimise: () => void
      WindowToggleMaximise: () => void
      WindowIsMaximised: () => Promise<boolean>
      Quit: () => void
      EventsOn?: (name: string, cb: () => void) => void
    }
  }
}

const routeLabels: Record<Route, string> = {
  site: 'Site',
  pages: 'Pages',
  pageView: 'Pages',
  team: 'Team',
  settings: 'Settings',
  billing: 'Billing',
  graph: 'Graph',
  connect: 'Connect',
  database: 'Database',
}

export default function TitleBar() {
  const [platform, setPlatform] = useState<Platform>('')
  const [port, setPort] = useState<number | null>(null)
  const route = useStore($route)
  const pageSlug = useStore($pageSlug)

  useEffect(() => {
    window.runtime?.Environment().then(e => setPlatform(e.platform as Platform))
    api.localInfo().then(i => setPort(i.port)).catch(() => setPort(null))
  }, [])

  if (!window.runtime) return null

  const isMac = platform === 'darwin'

  const onDoubleClick = () => {
    if (!isMac) window.runtime?.WindowToggleMaximise()
  }

  return (
    <div
      className="titlebar"
      style={{ paddingLeft: isMac ? 78 : 0 } as React.CSSProperties}
      onDoubleClick={onDoubleClick}
    >
      <span className="titlebar-brand">WriteKit</span>
      <span className="titlebar-sep">/</span>
      {route === 'pageView' ? (
        <>
          <button type="button" className="titlebar-crumb" onClick={() => navigate('pages')}>Pages</button>
          <span className="titlebar-sep">/</span>
          <span className="titlebar-route" title={pageSlug}>{pageSlug}</span>
        </>
      ) : (
        <span className="titlebar-route">{routeLabels[route] ?? ''}</span>
      )}

      <div className="titlebar-spacer" />

      {port !== null && (
        <button
          className="titlebar-chip"
          onClick={() => navigate('connect')}
          title="Local MCP server — click to open Connect"
        >
          <span className="titlebar-dot" />
          <span className="titlebar-chip-label">MCP</span>
          <span className="titlebar-chip-port">:{port}</span>
        </button>
      )}

      {!isMac && (
        <div className="titlebar-controls">
          <button
            className="titlebar-btn"
            onClick={() => window.runtime?.WindowMinimise()}
            aria-label="Minimize"
          >
            <svg width="10" height="10" viewBox="0 0 10 10"><path d="M0 5h10" stroke="currentColor" strokeWidth="1" /></svg>
          </button>
          <button
            className="titlebar-btn"
            onClick={() => window.runtime?.WindowToggleMaximise()}
            aria-label="Maximize"
          >
            <svg width="10" height="10" viewBox="0 0 10 10"><rect x="0.5" y="0.5" width="9" height="9" fill="none" stroke="currentColor" strokeWidth="1" /></svg>
          </button>
          <button
            className="titlebar-btn titlebar-btn-close"
            onClick={() => window.runtime?.Quit()}
            aria-label="Close"
          >
            <svg width="10" height="10" viewBox="0 0 10 10"><path d="M0 0l10 10M10 0L0 10" stroke="currentColor" strokeWidth="1" /></svg>
          </button>
        </div>
      )}
    </div>
  )
}
