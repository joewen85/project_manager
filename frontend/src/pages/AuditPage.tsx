import { useEffect, useState } from 'react'
import { api } from '../services/api'
import { DataState } from '../components/DataState'
import { formatDateTime } from '../utils/datetime'

export function AuditPage() {
  const [items, setItems] = useState<any[]>([])
  const [module, setModule] = useState('')
  const [action, setAction] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const query = new URLSearchParams({ page: '1', pageSize: '100' })
      if (module) query.set('module', module)
      if (action) query.set('action', action)
      const res = await api.get(`/audit/logs?${query.toString()}`)
      setItems(res.data.list ?? [])
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || '审计日志加载失败')
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
      <div className="card form-grid">
        <input aria-label="审计模块筛选" placeholder="模块，如 tasks" value={module} onChange={(e) => setModule(e.target.value)} />
        <input aria-label="审计动作筛选" placeholder="动作，如 update" value={action} onChange={(e) => setAction(e.target.value)} />
        <div className="row-actions"><button className="btn" onClick={() => { void load() }}>查询</button></div>
      </div>
      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无审计日志" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table>
            <thead>
              <tr><th>ID</th><th>模块</th><th>动作</th><th>用户ID</th><th>目标ID</th><th>结果</th><th>时间</th></tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td>{item.id}</td><td>{item.module}</td><td>{item.action}</td><td>{item.userId}</td><td>{item.targetId}</td>
                  <td>{item.success ? '成功' : '失败'}</td><td>{formatDateTime(item.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}
