import { useMemo, useState } from 'react'

interface PaginationProps {
  total: number
  page: number
  pageSize: number
  pageSizeOptions?: number[]
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}

type PageItem = number | '...'

function buildPageItems(current: number, totalPages: number): PageItem[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1)
  }

  if (current <= 4) {
    return [1, 2, 3, 4, 5, '...', totalPages]
  }

  if (current >= totalPages - 3) {
    return [1, '...', totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1, totalPages]
  }

  return [1, '...', current - 1, current, current + 1, '...', totalPages]
}

export function Pagination({
  total,
  page,
  pageSize,
  pageSizeOptions = [10, 20, 50],
  onPageChange,
  onPageSizeChange
}: PaginationProps) {
  const totalPages = Math.max(Math.ceil(total / pageSize), 1)
  const current = Math.min(Math.max(page, 1), totalPages)
  const [jumpValue, setJumpValue] = useState(String(current))
  const pageItems = useMemo(() => buildPageItems(current, totalPages), [current, totalPages])

  const jump = () => {
    const target = Number(jumpValue)
    if (!Number.isFinite(target)) return
    const normalized = Math.min(Math.max(Math.trunc(target), 1), totalPages)
    onPageChange(normalized)
    setJumpValue(String(normalized))
  }

  return (
    <div className="card pagination-shell">
      <div className="pagination-total">Total {total}</div>

      <select
        className="pagination-size"
        value={pageSize}
        onChange={(event) => {
          const nextPageSize = Number(event.target.value)
          onPageSizeChange(nextPageSize)
          onPageChange(1)
          setJumpValue('1')
        }}
      >
        {pageSizeOptions.map((size) => (
          <option key={size} value={size}>
            {size}/page
          </option>
        ))}
      </select>

      <button className="pagination-btn" disabled={current <= 1} onClick={() => onPageChange(current - 1)}>
        <span className="sr-only">上一页</span>
        ‹
      </button>

      {pageItems.map((item, index) =>
        item === '...' ? (
          <span className="pagination-ellipsis" key={`ellipsis-${index}`}>
            ...
          </span>
        ) : (
          <button
            className={`pagination-btn page-number${item === current ? ' active' : ''}`}
            key={item}
            aria-current={item === current ? 'page' : undefined}
            aria-label={`第 ${item} 页`}
            onClick={() => {
              onPageChange(item)
              setJumpValue(String(item))
            }}
          >
            {item}
          </button>
        )
      )}

      <button className="pagination-btn" disabled={current >= totalPages} onClick={() => onPageChange(current + 1)}>
        <span className="sr-only">下一页</span>
        ›
      </button>

      <label className="pagination-goto">
        <span>Go to</span>
        <input
          aria-label="跳转页码"
          value={jumpValue}
          onChange={(event) => setJumpValue(event.target.value)}
          onBlur={jump}
          onKeyDown={(event) => {
            if (event.key === 'Enter') jump()
          }}
        />
      </label>
    </div>
  )
}
