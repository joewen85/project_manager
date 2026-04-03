import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown, X } from 'lucide-react'
import { SearchField } from './SearchField'
import { useRemoteProjects } from './useRemoteProjects'

interface RemoteProjectMultiSelectProps {
  values: string[]
  onChange: (values: string[]) => void
  ariaLabel: string
  placeholder?: string
  noResultsText?: string
  summaryNoun?: string
  className?: string
}

export function RemoteProjectMultiSelect({
  values,
  onChange,
  ariaLabel,
  placeholder = '搜索项目',
  noResultsText = '没有匹配的项目',
  summaryNoun = '项目',
  className = ''
}: RemoteProjectMultiSelectProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const listId = useId()
  const statusId = useId()
  const [open, setOpen] = useState(false)
  const { projects, query, setQuery, resetQuery, loading, loadingMore, error, hasMore, handleListScroll, selectedProjects } = useRemoteProjects(values)

  const selectedSet = useMemo(() => new Set(values), [values])
  const visibleProjects = useMemo(() => {
    const selectedVisible = projects.filter((project) => selectedSet.has(String(project.id)))
    const unselectedVisible = projects.filter((project) => !selectedSet.has(String(project.id)))
    return [...selectedVisible, ...unselectedVisible]
  }, [projects, selectedSet])

  useEffect(() => {
    if (!open) {
      resetQuery()
    }
  }, [open, resetQuery])

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: MouseEvent) => {
      const target = event.target as Node
      if (!wrapRef.current?.contains(target)) {
        setOpen(false)
      }
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
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

  const summaryLabel = values.length === 0 ? `选择${summaryNoun}` : `已选 ${values.length} 个${summaryNoun}`

  return (
    <div className={`combo-wrap searchable-multi-select${className ? ` ${className}` : ''}`} ref={wrapRef}>
      <button
        type="button"
        className={`searchable-multi-select-trigger${open ? ' open' : ''}`}
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={listId}
        onClick={() => setOpen((prev) => !prev)}
      >
        <span className="searchable-multi-select-summary">{summaryLabel}</span>
        <ChevronDown size={16} className={open ? 'searchable-select-toggle-icon open' : 'searchable-select-toggle-icon'} />
      </button>

      <div id={statusId} className="sr-only" aria-live="polite">
        {`已选 ${selectedProjects.length} 个${summaryNoun}，当前已加载 ${visibleProjects.length} 个`}
      </div>

      {selectedProjects.length > 0 && (
        <div className="searchable-multi-select-tags" aria-label={`已选${summaryNoun}`}>
          {selectedProjects.map((project) => (
            <button
              key={project.id}
              type="button"
              className="searchable-multi-select-tag"
              onClick={() => toggleValue(String(project.id))}
            >
              <span>{project.code} - {project.name}</span>
              <X size={12} aria-hidden="true" />
            </button>
          ))}
        </div>
      )}

      {open && (
        <div id={listId} className="combo-menu searchable-multi-select-menu remote-project-menu" role="listbox" aria-multiselectable="true" aria-describedby={statusId}>
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
            <span className="helper-text">{`已选 ${selectedProjects.length} 个${summaryNoun}`}</span>
            <button
              type="button"
              className="btn secondary"
              onClick={() => onChange([])}
              disabled={selectedProjects.length === 0}
            >
              清空
            </button>
          </div>
          <div className="multi-checklist compact searchable-multi-select-list" onScroll={handleListScroll}>
            {visibleProjects.map((project) => (
              <label key={project.id} className="multi-check-item" role="option" aria-selected={selectedSet.has(String(project.id))}>
                <input
                  type="checkbox"
                  checked={selectedSet.has(String(project.id))}
                  onChange={() => toggleValue(String(project.id))}
                />
                <span>{project.code} - {project.name}</span>
              </label>
            ))}
            {loading && <div className="combo-empty">加载项目中...</div>}
            {!loading && visibleProjects.length === 0 && !error && <div className="combo-empty">{noResultsText}</div>}
            {error && <div className="combo-empty">{error}</div>}
            {!loading && loadingMore && <div className="combo-empty">正在加载更多项目...</div>}
            {!loading && !loadingMore && hasMore && visibleProjects.length > 0 && <div className="combo-empty">滚动加载更多项目</div>}
          </div>
        </div>
      )}
    </div>
  )
}
