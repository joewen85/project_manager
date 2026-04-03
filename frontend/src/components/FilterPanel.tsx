import { ReactNode, useEffect, useState } from 'react'
import { ChevronDown, SlidersHorizontal } from 'lucide-react'

interface FilterPanelProps {
  title: string
  activeCount?: number
  actions?: ReactNode
  children: ReactNode
  bodyClassName?: string
}

export function FilterPanel({ title, activeCount = 0, actions, children, bodyClassName = '' }: FilterPanelProps) {
  const [compact, setCompact] = useState(() => window.matchMedia('(max-width: 768px)').matches)
  const [expanded, setExpanded] = useState(() => !window.matchMedia('(max-width: 768px)').matches)

  useEffect(() => {
    const mediaQuery = window.matchMedia('(max-width: 768px)')
    const handleChange = (event: MediaQueryListEvent) => {
      setCompact(event.matches)
      setExpanded((prev) => (event.matches ? prev : true))
    }

    setCompact(mediaQuery.matches)
    setExpanded((prev) => (mediaQuery.matches ? prev : true))

    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handleChange)
      return () => mediaQuery.removeEventListener('change', handleChange)
    }

    mediaQuery.addListener(handleChange)
    return () => mediaQuery.removeListener(handleChange)
  }, [])

  const resolvedExpanded = compact ? expanded : true

  return (
    <section className={`filter-panel card${resolvedExpanded ? ' expanded' : ''}`}>
      <div className="filter-panel-header">
        <div className="filter-panel-title-wrap">
          <h3>{title}</h3>
          {activeCount > 0 && <span className="filter-panel-badge">{activeCount} 项已启用</span>}
        </div>
        <div className="filter-panel-actions">
          {actions}
          {compact && (
            <button
              type="button"
              className="filter-panel-toggle"
              aria-expanded={resolvedExpanded}
              onClick={() => setExpanded((prev) => !prev)}
            >
              <SlidersHorizontal size={16} />
              <span>{resolvedExpanded ? '收起筛选' : '展开筛选'}</span>
              <ChevronDown size={16} className={`filter-panel-toggle-icon${resolvedExpanded ? ' open' : ''}`} />
            </button>
          )}
        </div>
      </div>
      <div className={`filter-panel-body${resolvedExpanded ? ' visible' : ''}${bodyClassName ? ` ${bodyClassName}` : ''}`}>
        {children}
      </div>
    </section>
  )
}
