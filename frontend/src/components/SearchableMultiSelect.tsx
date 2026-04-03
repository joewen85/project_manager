import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown, X } from 'lucide-react'
import { SearchField } from './SearchField'

export interface SearchableMultiSelectOption {
  value: string
  label: string
  keywords?: string[]
}

interface SearchableMultiSelectProps {
  values: string[]
  options: SearchableMultiSelectOption[]
  onChange: (values: string[]) => void
  ariaLabel: string
  placeholder?: string
  noResultsText?: string
  summaryNoun?: string
  className?: string
}

export function SearchableMultiSelect({
  values,
  options,
  onChange,
  ariaLabel,
  placeholder = '搜索',
  noResultsText = '没有匹配项',
  summaryNoun = '项',
  className = ''
}: SearchableMultiSelectProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const listId = useId()
  const statusId = useId()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const selectedSet = useMemo(() => new Set(values), [values])
  const selectedOptions = useMemo(
    () => options.filter((option) => selectedSet.has(option.value)),
    [options, selectedSet]
  )

  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) return options
    return options.filter((option) => {
      const haystack = [option.label, ...(option.keywords || [])]
        .join(' ')
        .toLowerCase()
      return haystack.includes(normalizedQuery)
    })
  }, [options, query])

  const visibleOptions = useMemo(() => {
    if (!query.trim()) return filteredOptions
    const selected = filteredOptions.filter((option) => selectedSet.has(option.value))
    const unselected = filteredOptions.filter((option) => !selectedSet.has(option.value))
    return [...selected, ...unselected]
  }, [filteredOptions, query, selectedSet])

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
    window.requestAnimationFrame(() => searchInputRef.current?.focus())
  }, [open])

  const toggleValue = (nextValue: string) => {
    if (selectedSet.has(nextValue)) {
      onChange(values.filter((value) => value !== nextValue))
      return
    }
    onChange([...values, nextValue])
  }

  const summaryLabel = values.length === 0
    ? `选择${summaryNoun}`
    : `已选 ${values.length} 个${summaryNoun}`

  return (
    <div className={`combo-wrap searchable-multi-select${className ? ` ${className}` : ''}`} ref={wrapRef}>
      <button
        type="button"
        className={`searchable-multi-select-trigger${open ? ' open' : ''}`}
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={listId}
        onClick={() => {
          setOpen((prev) => !prev)
          if (open) setQuery('')
        }}
      >
        <span className="searchable-multi-select-summary">{summaryLabel}</span>
        <ChevronDown size={16} className={open ? 'searchable-select-toggle-icon open' : 'searchable-select-toggle-icon'} />
      </button>

      <div id={statusId} className="sr-only" aria-live="polite">
        {`已选 ${selectedOptions.length} 个${summaryNoun}，当前匹配 ${visibleOptions.length} 个`}
      </div>

      {selectedOptions.length > 0 && (
        <div className="searchable-multi-select-tags" aria-label={`已选${summaryNoun}`}>
          {selectedOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              className="searchable-multi-select-tag"
              onClick={() => toggleValue(option.value)}
            >
              <span>{option.label}</span>
              <X size={12} aria-hidden="true" />
            </button>
          ))}
        </div>
      )}

      {open && (
        <div id={listId} className="combo-menu searchable-multi-select-menu" role="listbox" aria-multiselectable="true" aria-describedby={statusId}>
          <div className="searchable-multi-select-search">
            <SearchField
              ref={searchInputRef}
              aria-label={`${ariaLabel}搜索`}
              value={query}
              placeholder={placeholder}
              onChange={setQuery}
            />
          </div>
          <div className="searchable-multi-select-actions">
            <span className="helper-text">{`已选 ${selectedOptions.length} 个${summaryNoun}`}</span>
            <button
              type="button"
              className="btn secondary"
              onClick={() => onChange([])}
              disabled={selectedOptions.length === 0}
            >
              清空
            </button>
          </div>
          <div className="multi-checklist compact searchable-multi-select-list">
            {visibleOptions.map((option) => (
              <label key={option.value} className="multi-check-item" role="option" aria-selected={selectedSet.has(option.value)}>
                <input
                  type="checkbox"
                  checked={selectedSet.has(option.value)}
                  onChange={() => toggleValue(option.value)}
                />
                <span>{option.label}</span>
              </label>
            ))}
            {visibleOptions.length === 0 && (
              <div className="combo-empty">{noResultsText}</div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
