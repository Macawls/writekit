import { useEffect, useRef, useState } from 'react'

export interface ConfirmOptions {
  title: string
  body: React.ReactNode
  confirmLabel?: string
  cancelLabel?: string
  destructive?: boolean
}

let resolver: ((v: boolean) => void) | null = null
let setOpen: ((o: ConfirmOptions | null) => void) | null = null

export function confirmDialog(opts: ConfirmOptions): Promise<boolean> {
  if (!setOpen) {
    return Promise.resolve(window.confirm(typeof opts.body === 'string' ? opts.body : opts.title))
  }
  setOpen(opts)
  return new Promise<boolean>(resolve => { resolver = resolve })
}

export function ConfirmHost() {
  const [opts, setOptsState] = useState<ConfirmOptions | null>(null)
  const cancelRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    setOpen = setOptsState
    return () => { setOpen = null }
  }, [])

  useEffect(() => {
    if (!opts) return
    cancelRef.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') resolve(false)
      else if (e.key === 'Enter') resolve(true)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [opts])

  const resolve = (v: boolean) => {
    setOptsState(null)
    if (resolver) {
      resolver(v)
      resolver = null
    }
  }

  if (!opts) return null

  return (
    <div className="modal-backdrop" onMouseDown={() => resolve(false)}>
      <div className="modal" role="dialog" aria-modal="true" aria-labelledby="confirm-title" onMouseDown={e => e.stopPropagation()}>
        <h3 id="confirm-title" className="modal-title">{opts.title}</h3>
        <div className="modal-body">{opts.body}</div>
        <div className="modal-actions">
          <button ref={cancelRef} type="button" className="btn btn-outline" onClick={() => resolve(false)}>
            {opts.cancelLabel ?? 'Cancel'}
          </button>
          <button
            type="button"
            className={`btn ${opts.destructive ? 'btn-danger' : ''}`}
            onClick={() => resolve(true)}
          >
            {opts.confirmLabel ?? 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  )
}
