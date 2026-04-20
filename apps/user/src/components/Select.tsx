import { useEffect, useRef, useState } from 'react'

export interface SelectOption<T extends string = string> {
  value: T
  label: string
  hint?: string
}

interface Props<T extends string> {
  value: T
  options: SelectOption<T>[]
  onChange: (v: T) => void
  className?: string
  disabled?: boolean
  ariaLabel?: string
}

export function Select<T extends string>({ value, options, onChange, className, disabled, ariaLabel }: Props<T>) {
  const [open, setOpen] = useState(false)
  const [activeIdx, setActiveIdx] = useState(-1)
  const rootRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const current = options.find(o => o.value === value)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setOpen(false); return }
      if (e.key === 'ArrowDown') { e.preventDefault(); setActiveIdx(i => Math.min(options.length - 1, i + 1)) }
      else if (e.key === 'ArrowUp') { e.preventDefault(); setActiveIdx(i => Math.max(0, i - 1)) }
      else if (e.key === 'Enter') {
        e.preventDefault()
        if (activeIdx >= 0 && activeIdx < options.length) {
          onChange(options[activeIdx].value)
          setOpen(false)
        }
      }
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, activeIdx, options, onChange])

  useEffect(() => {
    if (open) {
      const idx = options.findIndex(o => o.value === value)
      setActiveIdx(idx >= 0 ? idx : 0)
    }
  }, [open, value, options])

  useEffect(() => {
    if (!open || activeIdx < 0) return
    const el = listRef.current?.children[activeIdx] as HTMLElement | undefined
    el?.scrollIntoView({ block: 'nearest' })
  }, [activeIdx, open])

  return (
    <div ref={rootRef} className={`wk-select ${className ?? ''} ${open ? 'is-open' : ''} ${disabled ? 'is-disabled' : ''}`}>
      <button
        type="button"
        className="wk-select-button"
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
        onClick={() => setOpen(o => !o)}
      >
        <span className="wk-select-value">{current?.label ?? ''}</span>
        <span className="wk-select-chev" aria-hidden="true">
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
            <path d="M2 4l3 3 3-3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </span>
      </button>
      {open && (
        <ul ref={listRef} className="wk-select-list" role="listbox">
          {options.map((o, i) => (
            <li
              key={o.value}
              role="option"
              aria-selected={o.value === value}
              className={`wk-select-option ${i === activeIdx ? 'active' : ''} ${o.value === value ? 'selected' : ''}`}
              onMouseEnter={() => setActiveIdx(i)}
              onMouseDown={e => {
                e.preventDefault()
                onChange(o.value)
                setOpen(false)
              }}
            >
              <span className="wk-select-option-label">{o.label}</span>
              {o.hint && <span className="wk-select-option-hint">{o.hint}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
