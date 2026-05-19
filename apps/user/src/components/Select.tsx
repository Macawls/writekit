import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'

export interface SelectOption<T extends string = string> {
  value: T
  label: string
  hint?: string
  icon?: ReactNode
}

interface Props<T extends string> {
  value: T
  options: SelectOption<T>[]
  onChange: (v: T) => void
  className?: string
  disabled?: boolean
  ariaLabel?: string
  leftIcon?: ReactNode
  searchable?: boolean
  searchPlaceholder?: string
}

export function Select<T extends string>({ value, options, onChange, className, disabled, ariaLabel, leftIcon, searchable, searchPlaceholder }: Props<T>) {
  const [open, setOpen] = useState(false)
  const [activeIdx, setActiveIdx] = useState(-1)
  const [query, setQuery] = useState('')
  const rootRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const current = options.find(o => o.value === value)

  const filtered = useMemo(() => {
    if (!searchable || !query.trim()) return options
    const q = query.trim().toLowerCase()
    return options.filter(o => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q))
  }, [options, query, searchable])

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setOpen(false); return }
      if (e.key === 'ArrowDown') { e.preventDefault(); setActiveIdx(i => Math.min(filtered.length - 1, i + 1)) }
      else if (e.key === 'ArrowUp') { e.preventDefault(); setActiveIdx(i => Math.max(0, i - 1)) }
      else if (e.key === 'Enter') {
        e.preventDefault()
        if (activeIdx >= 0 && activeIdx < filtered.length) {
          onChange(filtered[activeIdx].value)
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
  }, [open, activeIdx, filtered, onChange])

  useEffect(() => {
    if (open) {
      setQuery('')
      const idx = options.findIndex(o => o.value === value)
      setActiveIdx(idx >= 0 ? idx : 0)
      if (searchable) {
        queueMicrotask(() => searchRef.current?.focus())
      }
    }
  }, [open, value, options, searchable])

  useEffect(() => {
    if (open && searchable) setActiveIdx(filtered.length > 0 ? 0 : -1)
  }, [query, searchable, open, filtered.length])

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
        {leftIcon && <span className="wk-select-icon" aria-hidden="true">{leftIcon}</span>}
        <span className="wk-select-value">{current?.label ?? ''}</span>
        <span className="wk-select-chev" aria-hidden="true">
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
            <path d="M2 4l3 3 3-3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </span>
      </button>
      {open && (
        <div className="wk-select-popover">
          {searchable && (
            <div className="wk-select-search">
              <input
                ref={searchRef}
                type="text"
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder={searchPlaceholder ?? 'Search…'}
                aria-label="Filter options"
              />
            </div>
          )}
          <ul ref={listRef} className="wk-select-list" role="listbox">
            {filtered.length === 0 ? (
              <li className="wk-select-empty">No matches</li>
            ) : filtered.map((o, i) => (
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
                <span className="wk-select-option-row">
                  {o.icon && <span className="wk-select-option-icon" aria-hidden="true">{o.icon}</span>}
                  <span className="wk-select-option-label">{o.label}</span>
                </span>
                {o.hint && <span className="wk-select-option-hint">{o.hint}</span>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
