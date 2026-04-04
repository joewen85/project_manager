import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { Filter } from 'lucide-react'
import { SearchField } from './SearchField'

export interface TableHeaderFilterOption {
  value: string
  label: string
  keywords?: string[]
}

interface TableHeaderFilterProps {
  label: string
  values: string[]
  options: TableHeaderFilterOption[]
  onChange: (values: string[]) => void
  placeholder?: string
  noResultsText?: string
}

export function TableHeaderFilter({
  label,
  values,
  options,
  onChange,
  placeholder = '搜索',
  noResultsText = '没有匹配项'
}: TableHeaderFilterProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const listId = useId()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const selectedSet = useMemo(() => new Set(values), [values])
  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) return options
    return options.filter((option) => [option.label, ...(option.keywords || [])].join(' ').toLowerCase().includes(normalizedQuery))
  }, [options, query])

  const allVisibleSelected = filteredOptions.length > 0 && filteredOptions.every((option) => selectedSet.has(option.value))

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: MouseEvent) => {
      const target = event.target as Node
      if (!wrapRef.current?.contains(target)) {
        setOpen(false)
        setQuery('')
      }
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
        setQuery('')
      }
    }
    document.addEventListener('mousedown', handleOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    window.requestAnimationFrame(() => inputRef.current?.focus())
  }, [open])

  const toggleValue = (value: string) => {
    if (selectedSet.has(value)) {
      onChange(values.filter((item) => item !== value))
      return
    }
    onChange([...values, value])
  }

  const toggleAllVisible = (checked: boolean) => {
    if (!checked) {
      onChange(values.filter((value) => !filteredOptions.some((option) => option.value === value)))
      return
    }
    onChange(Array.from(new Set([...values, ...filteredOptions.map((option) => option.value)])))
  }

  return (
    <div className="table-header-filter" ref={wrapRef}>
      <button
        type="button"
        className={`table-header-filter-trigger${open ? ' open' : ''}${values.length > 0 ? ' active' : ''}`}
        aria-label={`${label}筛选`}
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => {
          setOpen((prev) => !prev)
          if (open) setQuery('')
        }}
      >
        <span>{label}</span>
        <span className="table-header-filter-icon-wrap">
          <Filter size={14} aria-hidden="true" />
          {values.length > 0 && <span className="table-header-filter-count">{values.length}</span>}
        </span>
      </button>

      {open && (
        <div id={listId} className="combo-menu table-header-filter-menu">
          <div className="table-header-filter-search">
            <SearchField
              ref={inputRef}
              aria-label={`${label}筛选搜索`}
              value={query}
              placeholder={placeholder}
              onChange={setQuery}
            />
          </div>
          <div className="table-header-filter-actions">
            <button type="button" className="btn secondary" onClick={() => onChange([])} disabled={values.length === 0}>清空</button>
          </div>
          <div className="multi-checklist compact table-header-filter-list">
            {filteredOptions.length > 0 && (
              <label className="multi-check-item">
                <input type="checkbox" checked={allVisibleSelected} onChange={(event) => toggleAllVisible(event.target.checked)} />
                <span>（全选）</span>
              </label>
            )}
            {filteredOptions.map((option) => (
              <label key={option.value} className="multi-check-item">
                <input type="checkbox" checked={selectedSet.has(option.value)} onChange={() => toggleValue(option.value)} />
                <span>{option.label}</span>
              </label>
            ))}
            {filteredOptions.length === 0 && <div className="combo-empty">{noResultsText}</div>}
          </div>
        </div>
      )}
    </div>
  )
}
