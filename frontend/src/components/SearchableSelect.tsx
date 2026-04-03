import { KeyboardEvent, useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown } from 'lucide-react'

export interface SearchableSelectOption {
  value: string
  label: string
  keywords?: string[]
}

interface SearchableSelectProps {
  value: string
  options: SearchableSelectOption[]
  onChange: (value: string) => void
  ariaLabel: string
  placeholder?: string
  defaultOptionLabel?: string
  noResultsText?: string
  className?: string
}

export function SearchableSelect({
  value,
  options,
  onChange,
  ariaLabel,
  placeholder = '',
  defaultOptionLabel = '全部',
  noResultsText = '没有匹配项',
  className = ''
}: SearchableSelectProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const listboxId = useId()
  const selectedOption = options.find((option) => option.value === value)
  const selectedLabel = value ? selectedOption?.label || '' : defaultOptionLabel
  const [inputValue, setInputValue] = useState(selectedLabel)
  const [open, setOpen] = useState(false)
  const [highlightIndex, setHighlightIndex] = useState(0)

  useEffect(() => {
    setInputValue(selectedLabel)
  }, [selectedLabel])

  const query = useMemo(() => {
    const raw = inputValue.trim()
    if (!raw || raw === selectedLabel) return ''
    return raw.toLowerCase()
  }, [inputValue, selectedLabel])

  const filteredOptions = useMemo(() => {
    if (!query) return options
    return options.filter((option) => {
      const haystack = [option.label, ...(option.keywords || [])]
        .join(' ')
        .toLowerCase()
      return haystack.includes(query)
    })
  }, [options, query])

  const syncToSelected = () => {
    setInputValue(selectedLabel)
    setOpen(false)
    setHighlightIndex(0)
  }

  const commitValue = (nextValue: string) => {
    onChange(nextValue)
    setOpen(false)
    setHighlightIndex(0)
    if (!nextValue) {
      setInputValue(defaultOptionLabel)
      return
    }
    const nextOption = options.find((option) => option.value === nextValue)
    setInputValue(nextOption?.label || '')
  }

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: MouseEvent) => {
      const target = event.target as Node
      if (!wrapRef.current?.contains(target)) {
        syncToSelected()
      }
    }
    document.addEventListener('mousedown', handleOutside)
    return () => document.removeEventListener('mousedown', handleOutside)
  }, [open, selectedLabel])

  useEffect(() => {
    if (!open) return
    setHighlightIndex(0)
  }, [open, query, options])

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    const maxIndex = filteredOptions.length
    if (!open && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
      setOpen(true)
      event.preventDefault()
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      syncToSelected()
      return
    }
    if (event.key === 'Tab') {
      syncToSelected()
      return
    }
    if (!open) return
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setHighlightIndex((prev) => Math.min(prev + 1, maxIndex))
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      setHighlightIndex((prev) => Math.max(prev - 1, 0))
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      if (highlightIndex === 0) {
        commitValue('')
        return
      }
      const target = filteredOptions[highlightIndex - 1]
      if (target) commitValue(target.value)
    }
  }

  return (
    <div className={`combo-wrap searchable-select${className ? ` ${className}` : ''}`} ref={wrapRef}>
      <div className="searchable-select-field">
        <input
          ref={inputRef}
          type="text"
          role="combobox"
          aria-label={ariaLabel}
          aria-expanded={open}
          aria-controls={listboxId}
          aria-autocomplete="list"
          value={inputValue}
          placeholder={placeholder || defaultOptionLabel}
          onFocus={() => {
            setOpen(true)
            if (inputValue === selectedLabel) {
              window.requestAnimationFrame(() => inputRef.current?.select())
            }
          }}
          onClick={() => {
            setOpen(true)
            if (inputValue === selectedLabel) {
              window.requestAnimationFrame(() => inputRef.current?.select())
            }
          }}
          onKeyDown={handleKeyDown}
          onChange={(event) => {
            setInputValue(event.target.value)
            setOpen(true)
          }}
        />
        <button
          type="button"
          className="searchable-select-toggle"
          aria-label={`${ariaLabel}下拉`}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => {
            if (open) {
              syncToSelected()
              return
            }
            setOpen(true)
            window.requestAnimationFrame(() => inputRef.current?.focus())
          }}
        >
          <ChevronDown size={16} className={open ? 'searchable-select-toggle-icon open' : 'searchable-select-toggle-icon'} />
        </button>
      </div>

      {open && (
        <div id={listboxId} role="listbox" className="combo-menu">
          <button
            type="button"
            role="option"
            aria-selected={value === ''}
            className={`combo-option${highlightIndex === 0 || value === '' ? ' active' : ''}`}
            onMouseEnter={() => setHighlightIndex(0)}
            onClick={() => commitValue('')}
          >
            {defaultOptionLabel}
          </button>
          {filteredOptions.map((option, index) => (
            <button
              type="button"
              key={option.value}
              role="option"
              aria-selected={value === option.value}
              className={`combo-option${highlightIndex === index + 1 || value === option.value ? ' active' : ''}`}
              onMouseEnter={() => setHighlightIndex(index + 1)}
              onClick={() => commitValue(option.value)}
            >
              {option.label}
            </button>
          ))}
          {filteredOptions.length === 0 && (
            <div className="combo-empty">{noResultsText}</div>
          )}
        </div>
      )}
    </div>
  )
}
