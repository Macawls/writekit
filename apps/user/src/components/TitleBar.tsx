import { useEffect, useState } from 'react'

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

export default function TitleBar() {
  const [platform, setPlatform] = useState<Platform>('')

  useEffect(() => {
    window.runtime?.Environment().then(e => setPlatform(e.platform as Platform))
  }, [])

  if (!window.runtime) return null

  const isMac = platform === 'darwin'

  const onDoubleClick = () => {
    if (!isMac) window.runtime?.WindowToggleMaximise()
  }

  return (
    <div
      className="titlebar"
      style={{ paddingLeft: isMac ? 78 : 12 } as React.CSSProperties}
      onDoubleClick={onDoubleClick}
    >
      <span className="titlebar-title">WriteKit</span>
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
