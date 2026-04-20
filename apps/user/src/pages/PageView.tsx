import { useRef } from 'react'
import { useStore } from '@nanostores/react'
import { $pageSlug, navigate } from '../stores/router'
import { $site, $isDesktop } from '../stores/auth'

export default function PageView() {
  const slug = useStore($pageSlug)
  const isDesktop = useStore($isDesktop)
  const site = useStore($site)
  const frameRef = useRef<HTMLIFrameElement>(null)

  const host = window.location.hostname.replace(/^app\./, '')
  const src = isDesktop
    ? `${window.location.origin}/site/${slug}`
    : site ? `https://${site.ID}.${host}/${slug}` : ''

  const externalUrl = src

  const openExternal = () => {
    const r = (window as any).runtime
    if (isDesktop && r?.BrowserOpenURL) r.BrowserOpenURL(externalUrl)
    else window.open(externalUrl, '_blank')
  }

  return (
    <div className="page-view">
      <div className="page-view-bar">
        <button type="button" className="page-view-back" onClick={() => navigate('pages')}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><polyline points="15 18 9 12 15 6" /></svg>
          Pages
        </button>
        <span className="muted page-view-slug">/{slug}</span>
        <button type="button" className="page-view-external" onClick={openExternal} title="Open in external browser">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3" /></svg>
        </button>
      </div>
      <iframe
        ref={frameRef}
        className="page-view-frame"
        src={src}
        title={slug}
        onLoad={() => {
          try {
            const path = frameRef.current?.contentWindow?.location.pathname || ''
            if (isDesktop && path && !path.startsWith('/site')) {
              navigate('pages')
            }
          } catch {
            /* cross-origin — hosted mode — ignore */
          }
        }}
      />
    </div>
  )
}
