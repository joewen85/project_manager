interface DataStateProps {
  loading?: boolean
  error?: string
  empty?: boolean
  emptyText?: string
  onRetry?: () => void
}

export function DataState({ loading, error, empty, emptyText = '暂无数据', onRetry }: DataStateProps) {
  if (!loading && !error && !empty) return null

  return (
    <div className="data-state">
      {loading && <p>加载中...</p>}
      {!loading && error && <p className="error">{error}</p>}
      {!loading && !error && empty && <p>{emptyText}</p>}
      {!loading && onRetry && (error || empty) && (
        <button type="button" className="btn secondary" onClick={onRetry}>
          刷新
        </button>
      )}
    </div>
  )
}
