import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown, X } from 'lucide-react'
import { SearchField } from './SearchField'

export interface SearchableMultiSelectOption {
  value: string
  label: string
  keywords?: string[]
}

// Nearest scrollable/clipping ancestor. The floating menu is clipped by it, so
// placement is measured against its visible box rather than the whole viewport.
function getClipParent(el: HTMLElement | null): HTMLElement | null {
  let node = el?.parentElement ?? null
  while (node) {
    const overflowY = getComputedStyle(node).overflowY
    if (overflowY === 'auto' || overflowY === 'scroll' || overflowY === 'hidden') {
      return node
    }
    node = node.parentElement
  }
  return null
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
  disabled?: boolean
  onSearchChange?: (query: string) => void
  /**
   * Menu renders inline in the document flow (position: static) rather than as a
   * floating overlay. Disables the flip / height-clamp placement logic.
   */
  inlineMenu?: boolean
}

export function SearchableMultiSelect({
  values,
  options,
  onChange,
  ariaLabel,
  placeholder = '搜索',
  noResultsText = '没有匹配项',
  summaryNoun = '项',
  className = '',
  disabled = false,
  onSearchChange,
  inlineMenu = false
}: SearchableMultiSelectProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const triggerRef = useRef<HTMLButtonElement | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const listId = useId()
  const statusId = useId()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  // Whether the menu should open upward and how tall the scrollable list may be,
  // so the menu stays on screen (and off the sticky save button) when the field
  // sits near the bottom of a modal.
  const [dropUp, setDropUp] = useState(false)
  const [listMaxHeight, setListMaxHeight] = useState<number | null>(null)

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

  const updateQuery = useCallback((nextQuery: string) => {
    setQuery(nextQuery)
    onSearchChange?.(nextQuery)
  }, [onSearchChange])

  const updatePlacement = useCallback(() => {
    const trigger = triggerRef.current
    if (!trigger) return
    const rect = trigger.getBoundingClientRect()
    const margin = 12
    // Bound placement by the clipping container's visible box (e.g. the modal
    // body), so the whole menu stays on screen without scrolling the container.
    const clip = getClipParent(trigger)?.getBoundingClientRect()
    const boundsTop = Math.max(clip?.top ?? 0, 0)
    const boundsBottom = Math.min(clip?.bottom ?? window.innerHeight, window.innerHeight)
    const spaceBelow = boundsBottom - rect.bottom - margin
    const spaceAbove = rect.top - boundsTop - margin
    // Search box + actions row that sit above the scrollable list.
    const chrome = 104
    // Room for ~3 rows of options below before we flip the menu upward.
    const shouldDropUp = spaceBelow - chrome < 132 && spaceAbove > spaceBelow
    const available = shouldDropUp ? spaceAbove : spaceBelow
    setDropUp(shouldDropUp)
    setListMaxHeight(Math.max(120, Math.min(360, available - chrome)))
  }, [])

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: MouseEvent) => {
      const target = event.target as Node
      if (!wrapRef.current?.contains(target)) {
        setOpen(false)
        updateQuery('')
      }
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
        updateQuery('')
      }
    }
    document.addEventListener('mousedown', handleOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open, updateQuery])

  useEffect(() => {
    if (!open || inlineMenu) return
    updatePlacement()
    const handleReposition = () => updatePlacement()
    window.addEventListener('scroll', handleReposition, true)
    window.addEventListener('resize', handleReposition)
    return () => {
      window.removeEventListener('scroll', handleReposition, true)
      window.removeEventListener('resize', handleReposition)
    }
  }, [open, inlineMenu, updatePlacement])

  useEffect(() => {
    if (!open) return
    window.requestAnimationFrame(() => searchInputRef.current?.focus())
  }, [open])

  useEffect(() => {
    if (disabled) setOpen(false)
  }, [disabled])

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
        ref={triggerRef}
        className={`searchable-multi-select-trigger${open ? ' open' : ''}`}
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={listId}
        disabled={disabled}
        onClick={() => {
          if (disabled) return
          setOpen((prev) => !prev)
          if (open) updateQuery('')
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
              disabled={disabled}
              onClick={() => toggleValue(option.value)}
            >
              <span>{option.label}</span>
              <X size={12} aria-hidden="true" />
            </button>
          ))}
        </div>
      )}

      {open && (
        <div
          id={listId}
          className={`combo-menu searchable-multi-select-menu${dropUp ? ' drop-up' : ''}`}
          role="listbox"
          aria-multiselectable="true"
          aria-describedby={statusId}
        >
          <div className="searchable-multi-select-search">
            <SearchField
              ref={searchInputRef}
              aria-label={`${ariaLabel}搜索`}
              value={query}
              placeholder={placeholder}
              onChange={updateQuery}
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
          <div
            className="multi-checklist compact searchable-multi-select-list"
            style={listMaxHeight ? { maxHeight: listMaxHeight } : undefined}
          >
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
