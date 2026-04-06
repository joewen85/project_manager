import { KeyboardEvent, useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { useRemoteProjects } from './useRemoteProjects'

interface RemoteProjectSelectProps {
  value: string
  onChange: (value: string) => void
  ariaLabel: string
  placeholder?: string
  defaultOptionLabel?: string
  noResultsText?: string
  className?: string
  disabled?: boolean
}

export function RemoteProjectSelect({
  value,
  onChange,
  ariaLabel,
  placeholder = '',
  defaultOptionLabel = '全部项目',
  noResultsText = '没有匹配的项目',
  className = '',
  disabled = false
}: RemoteProjectSelectProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const listboxId = useId()
  const [open, setOpen] = useState(false)
  const [highlightIndex, setHighlightIndex] = useState(0)
  const { projects, query, setQuery, resetQuery, loading, loadingMore, error, hasMore, handleListScroll, selectedProjects } = useRemoteProjects(value ? [value] : [])

  const selectedProject = selectedProjects[0]
  const selectedLabel = value ? (selectedProject ? `${selectedProject.code} - ${selectedProject.name}` : `项目 #${value}`) : defaultOptionLabel
  const options = useMemo(() => {
    if (query.trim()) return projects
    if (!selectedProject) return projects
    return [selectedProject, ...projects.filter((project) => project.id !== selectedProject.id)]
  }, [projects, query, selectedProject])

  useEffect(() => {
    if (!open) {
      resetQuery()
      setHighlightIndex(0)
    }
  }, [open, resetQuery])

  useEffect(() => {
    if (disabled) setOpen(false)
  }, [disabled])

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: MouseEvent) => {
      const target = event.target as Node
      if (!wrapRef.current?.contains(target)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleOutside)
    return () => document.removeEventListener('mousedown', handleOutside)
  }, [open])

  const commitValue = (nextValue: string) => {
    onChange(nextValue)
    setOpen(false)
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return
    const maxIndex = options.length
    if (!open && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
      setOpen(true)
      event.preventDefault()
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
      return
    }
    if (event.key === 'Tab') {
      setOpen(false)
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
      const target = options[highlightIndex - 1]
      if (target) commitValue(String(target.id))
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
          value={open ? query : selectedLabel}
          placeholder={placeholder || defaultOptionLabel}
          disabled={disabled}
          onFocus={() => {
            if (disabled) return
            setOpen(true)
            window.requestAnimationFrame(() => inputRef.current?.select())
          }}
          onClick={() => {
            if (disabled) return
            setOpen(true)
            window.requestAnimationFrame(() => inputRef.current?.select())
          }}
          onKeyDown={handleKeyDown}
          onChange={(event) => {
            if (disabled) return
            setQuery(event.target.value)
            setOpen(true)
          }}
        />
        <button
          type="button"
          className="searchable-select-toggle"
          aria-label={`${ariaLabel}下拉`}
          aria-haspopup="listbox"
          aria-expanded={open}
          disabled={disabled}
          onClick={() => {
            if (disabled) return
            if (open) {
              setOpen(false)
              return
            }
            setOpen(true)
            window.requestAnimationFrame(() => inputRef.current?.focus())
          }}
        >
          <ChevronDown size={16} className={open ? 'searchable-select-toggle-icon open' : 'searchable-select-toggle-icon'} />
        </button>
      </div>

      {open && !disabled && (
        <div id={listboxId} role="listbox" className="combo-menu remote-project-menu" onScroll={handleListScroll}>
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
          {options.map((project, index) => (
            <button
              type="button"
              key={project.id}
              role="option"
              aria-selected={value === String(project.id)}
              className={`combo-option${highlightIndex === index + 1 || value === String(project.id) ? ' active' : ''}`}
              onMouseEnter={() => setHighlightIndex(index + 1)}
              onClick={() => commitValue(String(project.id))}
            >
              {project.code} - {project.name}
            </button>
          ))}
          {loading && <div className="combo-empty">加载项目中...</div>}
          {!loading && options.length === 0 && !error && <div className="combo-empty">{noResultsText}</div>}
          {error && <div className="combo-empty">{error}</div>}
          {!loading && loadingMore && <div className="combo-empty">正在加载更多项目...</div>}
          {!loading && !loadingMore && hasMore && options.length > 0 && <div className="combo-empty">滚动加载更多项目</div>}
        </div>
      )}
    </div>
  )
}
