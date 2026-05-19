import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import type { SelectOption } from './Select'

interface Props {
  values: string[]
  options: SelectOption[]
  onChange: (v: string[]) => void
  className?: string
  disabled?: boolean
  ariaLabel?: string
  leftIcon?: ReactNode
  searchable?: boolean
  searchPlaceholder?: string
  allValue?: string
  pluralLabel?: string
}

export function MultiSelect({
  values, options, onChange, className, disabled, ariaLabel, leftIcon,
  searchable, searchPlaceholder, allValue = 'all', pluralLabel = 'items',
}: Props) {
  const [open, setOpen] = useState(false)
  const [activeIdx, setActiveIdx] = useState(-1)
  const [query, setQuery] = useState('')
  const rootRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)

  const selectedSet = useMemo(() => new Set(values), [values])
  const isAll = values.length === 0 || selectedSet.has(allValue)
  const allOption = options.find(o => o.value === allValue)

  const filtered = useMemo(() => {
    if (!searchable || !query.trim()) return options
    const q = query.trim().toLowerCase()
    return options.filter(o => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q))
  }, [options, query, searchable])

  const toggle = (v: string) => {
    if (v === allValue) {
      onChange([])
      return
    }
    const next = new Set(values.filter(x => x !== allValue))
    if (next.has(v)) next.delete(v)
    else next.add(v)
    onChange(Array.from(next))
  }

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
        if (activeIdx >= 0 && activeIdx < filtered.length) toggle(filtered[activeIdx].value)
      }
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, activeIdx, filtered, values])

  useEffect(() => {
    if (open) {
      setQuery('')
      setActiveIdx(0)
      if (searchable) queueMicrotask(() => searchRef.current?.focus())
    }
  }, [open, searchable])

  useEffect(() => {
    if (open && searchable) setActiveIdx(filtered.length > 0 ? 0 : -1)
  }, [query, searchable, open, filtered.length])

  useEffect(() => {
    if (!open || activeIdx < 0) return
    const el = listRef.current?.children[activeIdx] as HTMLElement | undefined
    el?.scrollIntoView({ block: 'nearest' })
  }, [activeIdx, open])

  const displayLabel = (() => {
    if (isAll) return allOption?.label ?? ''
    const picked = values.filter(v => v !== allValue)
    if (picked.length === 1) {
      const o = options.find(o => o.value === picked[0])
      return o?.label ?? picked[0]
    }
    return `${picked.length} ${pluralLabel}`
  })()

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
        <span className="wk-select-value">{displayLabel}</span>
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
          <ul ref={listRef} className="wk-select-list" role="listbox" aria-multiselectable="true">
            {filtered.length === 0 ? (
              <li className="wk-select-empty">No matches</li>
            ) : filtered.map((o, i) => {
              const checked = o.value === allValue ? isAll : selectedSet.has(o.value)
              return (
                <li
                  key={o.value}
                  role="option"
                  aria-selected={checked}
                  className={`wk-select-option wk-select-multi-option ${i === activeIdx ? 'active' : ''} ${checked ? 'selected' : ''}`}
                  onMouseEnter={() => setActiveIdx(i)}
                  onMouseDown={e => {
                    e.preventDefault()
                    toggle(o.value)
                  }}
                >
                  <span className="wk-select-check" aria-hidden="true">
                    {checked && (
                      <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                        <path d="M2 5.2l2 2 4-4.4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    )}
                  </span>
                  {o.icon && <span className="wk-select-option-icon" aria-hidden="true">{o.icon}</span>}
                  <span className="wk-select-option-label">{o.label}</span>
                  {o.hint && <span className="wk-select-option-hint">{o.hint}</span>}
                </li>
              )
            })}
          </ul>
        </div>
      )}
    </div>
  )
}
