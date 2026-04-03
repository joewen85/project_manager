import { useEffect, useState } from 'react'
import { fetchPage, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { SearchField } from '../components/SearchField'
import { formatDateTime } from '../utils/datetime'
import { AuditLog } from '../types'

export function AuditPage() {
  const [items, setItems] = useState<AuditLog[]>([])
  const [module, setModule] = useState('')
  const [action, setAction] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const activeFilterCount = Number(Boolean(module.trim())) + Number(Boolean(action.trim()))

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const pageData = await fetchPage<AuditLog>('/audit/logs', { page: 1, pageSize: 100, module, action }, { page: 1, pageSize: 100 })
      setItems(pageData.list)
    } catch (loadError) {
      setError(readApiError(loadError, '审计日志加载失败'))
      setItems([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  return (
    <section className="page-section">
      <FilterPanel title="审计筛选" activeCount={activeFilterCount} bodyClassName="form-grid">
        <SearchField aria-label="审计模块筛选" placeholder="模块，如 tasks" value={module} onChange={setModule} />
        <SearchField aria-label="审计动作筛选" placeholder="动作，如 update" value={action} onChange={setAction} />
        <div className="row-actions"><button className="btn" onClick={() => { void load() }}>查询</button></div>
      </FilterPanel>
      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无审计日志" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table">
            <thead>
              <tr><th>ID</th><th>模块</th><th>动作</th><th>用户ID</th><th>目标ID</th><th>结果</th><th>时间</th></tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td data-label="ID">{item.id}</td><td data-label="模块">{item.module}</td><td data-label="动作">{item.action}</td><td data-label="用户ID">{item.userId}</td><td data-label="目标ID">{item.targetId}</td>
                  <td data-label="结果">{item.success ? '成功' : '失败'}</td><td data-label="时间">{formatDateTime(item.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}
