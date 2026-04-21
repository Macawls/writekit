import { useState, useEffect } from 'react'
import { api, type User, type Site } from '../api'

type Step = 'welcome' | 'connect' | 'done'

const clients = ['Claude Code', 'Cursor', 'Windsurf', 'VS Code', 'OpenCode', 'Any Client'] as const
type Client = typeof clients[number]

const snippets: Record<Client, { note?: string; code: string }> = {
  'Claude Code': {
    code: 'claude mcp add --transport http writekit https://mcp.writekit.dev',
  },
  'Cursor': {
    note: 'Add to .cursor/mcp.json:',
    code: `{
  "mcpServers": {
    "writekit": {
      "url": "https://mcp.writekit.dev"
    }
  }
}`,
  },
  'Windsurf': {
    note: 'Add to your Windsurf MCP config:',
    code: `{
  "mcpServers": {
    "writekit": {
      "url": "https://mcp.writekit.dev"
    }
  }
}`,
  },
  'VS Code': {
    note: 'Add to .vscode/mcp.json:',
    code: `{
  "servers": {
    "writekit": {
      "type": "http",
      "url": "https://mcp.writekit.dev"
    }
  }
}`,
  },
  'OpenCode': {
    note: 'Add to opencode.json:',
    code: `{
  "mcp": {
    "writekit": {
      "type": "remote",
      "url": "https://mcp.writekit.dev"
    }
  }
}`,
  },
  'Any Client': {
    note: 'Point any Streamable HTTP MCP client at:',
    code: 'https://mcp.writekit.dev',
  },
}

interface Props {
  user: User
  site: Site
  onComplete: () => void
}

export default function Onboarding({ user, site, onComplete }: Props) {
  const isDesktop = user.id === 'local'
  const [step, setStep] = useState<Step>(isDesktop ? 'connect' : 'welcome')
  const [visible, setVisible] = useState(false)
  const [slug, setSlug] = useState(site.ID)
  const [saving, setSaving] = useState(false)
  const [slugError, setSlugError] = useState<string | null>(null)
  const [client, setClient] = useState<Client>('Claude Code')
  const [copied, setCopied] = useState(false)

  const host = window.location.hostname.replace(/^app\./, '')
  const siteUrl = isDesktop
    ? `${window.location.origin}/site`
    : `https://${slug}.${host}`

  useEffect(() => {
    requestAnimationFrame(() => setVisible(true))
  }, [])

  const transition = (next: Step) => {
    setVisible(false)
    setTimeout(() => {
      setStep(next)
      requestAnimationFrame(() => setVisible(true))
    }, 300)
  }

  const handleSlugSave = async () => {
    if (slug === site.ID) return
    setSaving(true)
    setSlugError(null)
    try {
      await api.updateSlug(slug)
      site.ID = slug
    } catch (err) {
      setSlugError(err instanceof Error ? err.message : 'Failed to update')
      setSlug(site.ID)
    } finally {
      setSaving(false)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const firstName = user.name?.split(' ')[0] || ''

  return (
    <div className="ob">
      <style>{`
        .ob {
          min-height: 100vh;
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
          padding: 2rem;
          position: relative;
        }
        .ob::before {
          content: '';
          position: fixed;
          inset: 0;
          background-image: radial-gradient(circle, var(--border) .5px, transparent .5px);
          background-size: 24px 24px;
          opacity: .3;
          pointer-events: none;
        }

        .ob-step {
          position: relative;
          z-index: 1;
          max-width: 480px;
          width: 100%;
          text-align: center;
          opacity: 0;
          transform: translateY(16px);
          transition: opacity .4s cubic-bezier(.19,1,.22,1), transform .4s cubic-bezier(.19,1,.22,1);
        }
        .ob-step.visible {
          opacity: 1;
          transform: translateY(0);
        }

        .ob-brand {
          font-family: var(--sans);
          font-weight: 700;
          font-size: .8rem;
          letter-spacing: -.04em;
          color: var(--faint);
          margin-bottom: 2.5rem;
        }

        .ob-step h1 {
          font-size: 1.4rem;
          font-weight: 700;
          letter-spacing: -.03em;
          line-height: 1.2;
          margin-bottom: .6rem;
        }
        .ob-step .desc {
          font-size: .86rem;
          color: var(--muted);
          line-height: 1.5;
          margin-bottom: 2rem;
        }

        .ob-site-url {
          display: inline-flex;
          align-items: center;
          gap: .5rem;
          padding: .65rem 1rem;
          background: var(--white);
          border: 1px solid var(--border);
          border-radius: var(--radius);
          font-size: .85rem;
          color: var(--fg);
          margin-bottom: .5rem;
          box-shadow: 0 1px 2px rgba(0,0,0,.02), 0 4px 16px rgba(0,0,0,.03);
        }
        .ob-site-url a {
          color: var(--fg);
          font-weight: 500;
        }

        .ob-edit-link {
          display: inline-block;
          font-size: .72rem;
          color: var(--faint);
          cursor: pointer;
          background: none;
          border: none;
          font-family: var(--sans);
          margin-bottom: 2rem;
        }
        .ob-edit-link:hover { color: var(--muted); }

        .ob-slug-edit {
          display: flex;
          align-items: center;
          gap: .25rem;
          max-width: 320px;
          margin: 0 auto .5rem;
        }
        .ob-slug-edit input {
          flex: 1;
          padding: .5rem .65rem;
          border: 1px solid var(--border);
          border-radius: var(--radius);
          font-family: var(--sans);
          font-size: .82rem;
          background: var(--white);
          color: var(--fg);
          outline: none;
          text-align: right;
        }
        .ob-slug-edit input:focus { border-color: var(--border2); }
        .ob-slug-edit .suffix {
          color: var(--faint);
          font-size: .82rem;
          white-space: nowrap;
        }
        .ob-slug-actions {
          display: flex;
          gap: .4rem;
          justify-content: center;
          margin-bottom: 2rem;
        }
        .ob-slug-actions button {
          font-family: var(--sans);
          font-size: .72rem;
          font-weight: 500;
          padding: .3rem .65rem;
          border-radius: 6px;
          cursor: pointer;
          border: 1px solid var(--border);
          background: var(--white);
          color: var(--muted);
        }
        .ob-slug-actions button:hover { color: var(--fg); border-color: var(--border2); }
        .ob-slug-actions .save-btn {
          background: var(--surface);
          color: var(--white);
          border-color: var(--surface);
        }
        .ob-slug-actions .save-btn:hover { opacity: .85; color: var(--white); }

        .ob-btn {
          display: inline-block;
          padding: .6rem 1.25rem;
          background: var(--surface);
          color: var(--white);
          border: none;
          border-radius: var(--radius);
          cursor: pointer;
          font-size: .82rem;
          font-family: var(--sans);
          font-weight: 600;
          transition: opacity .15s;
        }
        .ob-btn:hover { opacity: .85; }
        .ob-btn-ghost {
          background: transparent;
          color: var(--muted);
          font-size: .78rem;
          font-weight: 500;
          border: none;
          cursor: pointer;
          font-family: var(--sans);
          margin-top: .75rem;
          display: inline-block;
        }
        .ob-btn-ghost:hover { color: var(--fg); }

        .ob-tabs {
          display: flex;
          gap: 0;
          border-bottom: 1px solid var(--border);
          margin-bottom: 0;
          justify-content: center;
        }
        .ob-tab {
          padding: .45rem .7rem;
          font-size: .72rem;
          font-weight: 500;
          color: var(--faint);
          cursor: pointer;
          border: none;
          background: none;
          font-family: var(--sans);
          border-bottom: 2px solid transparent;
          margin-bottom: -1px;
        }
        .ob-tab:hover { color: var(--muted); }
        .ob-tab.active { color: var(--fg); border-bottom-color: var(--surface); font-weight: 600; }

        .ob-code-panel {
          text-align: left;
          padding: 1rem 0;
        }
        .ob-code-panel .note {
          font-size: .78rem;
          color: var(--muted);
          margin-bottom: .5rem;
          text-align: center;
        }
        .ob-code-block {
          position: relative;
          background: var(--surface);
          color: #e4e4e7;
          border-radius: var(--radius);
          padding: .85rem 1.1rem;
          padding-right: 2.5rem;
          font-family: 'JetBrains Mono', ui-monospace, monospace;
          font-size: .78rem;
          line-height: 1.7;
          overflow-x: auto;
          white-space: pre;
        }
        .ob-copy-btn {
          position: absolute;
          top: .6rem;
          right: .6rem;
          background: transparent;
          border: none;
          color: #71717a;
          cursor: pointer;
          padding: .2rem;
          display: flex;
          align-items: center;
        }
        .ob-copy-btn:hover { color: #a1a1aa; }
        .ob-copy-btn.ok { color: #e4e4e7; }

        .ob-done-icon {
          width: 48px;
          height: 48px;
          border-radius: 50%;
          background: var(--surface);
          display: flex;
          align-items: center;
          justify-content: center;
          margin: 0 auto 1.25rem;
        }

        .ob-site-link {
          display: block;
          margin: .5rem auto 1.5rem;
          padding: .75rem 1rem;
          max-width: 320px;
          background: var(--white);
          border: 1px solid var(--border);
          border-radius: var(--radius);
          font-size: .82rem;
          color: var(--fg);
          font-weight: 500;
          text-align: center;
          text-decoration: none;
          box-shadow: 0 1px 2px rgba(0,0,0,.02), 0 4px 16px rgba(0,0,0,.03);
        }
        .ob-site-link:hover {
          border-color: var(--border2);
          box-shadow: 0 1px 2px rgba(0,0,0,.02), 0 8px 32px rgba(0,0,0,.06);
          text-decoration: none;
        }
        .ob-site-link .arrow { color: var(--faint); margin-left: .35rem; }

        .ob-error {
          color: #ef4444;
          font-size: .75rem;
          margin-bottom: .5rem;
        }

        .ob-steps-indicator {
          display: flex;
          gap: .35rem;
          justify-content: center;
          margin-bottom: 2rem;
        }
        .ob-dot {
          width: 6px;
          height: 6px;
          border-radius: 50%;
          background: var(--border);
          transition: background .3s;
        }
        .ob-dot.active {
          background: var(--surface);
        }

        @media (max-width: 480px) {
          .ob { padding: 1.5rem; }
          .ob-step h1 { font-size: 1.2rem; }
          .ob-tabs { flex-wrap: wrap; }
        }
      `}</style>

      <div className={`ob-step ${visible ? 'visible' : ''}`}>
        <div className="ob-brand">writekit</div>

        <div className="ob-steps-indicator">
          {!isDesktop && <div className={`ob-dot ${step === 'welcome' ? 'active' : ''}`} />}
          <div className={`ob-dot ${step === 'connect' ? 'active' : ''}`} />
          <div className={`ob-dot ${step === 'done' ? 'active' : ''}`} />
        </div>

        {step === 'welcome' && (
          <>
            <h1>{firstName ? `Hey ${firstName}` : 'Welcome'}</h1>
            <p className="desc">Your site is ready.</p>

            <div className="ob-slug-edit">
              <input
                type="text"
                value={slug}
                onChange={e => {
                  setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))
                  setSlugError(null)
                }}
              />
              <span className="suffix">.{host}</span>
            </div>
            {slugError && <p className="ob-error">{slugError}</p>}
            {slug !== site.ID && (
              <div className="ob-slug-actions">
                <button onClick={() => { setSlug(site.ID); setSlugError(null) }}>Reset</button>
                <button className="save-btn" onClick={handleSlugSave} disabled={saving}>
                  {saving ? 'Saving...' : 'Save'}
                </button>
              </div>
            )}

            <button className="ob-btn" onClick={() => transition('connect')}>
              Continue
            </button>
          </>
        )}

        {step === 'connect' && isDesktop && (
          <>
            <h1>{firstName ? `Hey ${firstName}` : 'Welcome'}</h1>
            <p className="desc">
              Open the <strong>Connect</strong> tab to link WriteKit to Claude, Cursor, or any MCP client.
            </p>

            <button className="ob-btn" onClick={() => transition('done')}>
              Got it
            </button>
          </>
        )}

        {step === 'connect' && !isDesktop && (
          <>
            <h1>Connect your editor</h1>
            <p className="desc">
              Add the MCP server so your AI assistant can publish to your site.
            </p>

            <div className="ob-tabs">
              {clients.map(c => (
                <button
                  key={c}
                  className={`ob-tab ${client === c ? 'active' : ''}`}
                  onClick={() => { setClient(c); setCopied(false) }}
                >
                  {c}
                </button>
              ))}
            </div>

            <div className="ob-code-panel">
              {snippets[client].note && (
                <p className="note">{snippets[client].note}</p>
              )}
              <div className="ob-code-block">
                {snippets[client].code}
                <button
                  className={`ob-copy-btn ${copied ? 'ok' : ''}`}
                  onClick={() => copyToClipboard(snippets[client].code)}
                  aria-label="Copy"
                >
                  {copied ? (
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                  ) : (
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                  )}
                </button>
              </div>
            </div>

            <button className="ob-btn" onClick={() => transition('done')}>
              Continue
            </button>
            <br />
            <button className="ob-btn-ghost" onClick={() => transition('welcome')}>
              Back
            </button>
          </>
        )}

        {step === 'done' && (
          <>
            <h1>You're all set</h1>
            <p className="desc">
              Start a conversation with your AI assistant and tell it what to write. It handles the rest.
            </p>

            <a
              className="ob-site-link"
              href={siteUrl}
              target={isDesktop ? undefined : '_blank'}
              rel={isDesktop ? undefined : 'noopener noreferrer'}
            >
              {isDesktop ? 'View my pages' : `${slug}.${host}`} <span className="arrow">&rarr;</span>
            </a>

            <button className="ob-btn" onClick={onComplete}>
              Go to dashboard
            </button>
            <br />
            <button className="ob-btn-ghost" onClick={() => transition('connect')}>
              Back
            </button>
          </>
        )}
      </div>
    </div>
  )
}
