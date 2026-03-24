import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function AuditPage() {
  const [items, setItems] = useState<any[]>([])
  const [module, setModule] = useState('')
  const [action, setAction] = useState('')

  const load = () => {
    const query = new URLSearchParams({ page: '1', pageSize: '100' })
    if (module) query.set('module', module)
    if (action) query.set('action', action)
    void api.get(`/audit/logs?${query.toString()}`).then((res) => setItems(res.data.list ?? []))
  }

  useEffect(() => {
    load()
  }, [])

  return (
    <section>
      <h2>操作审计日志</h2>
      <div className="card form-grid">
        <input placeholder="模块，如 tasks" value={module} onChange={(e) => setModule(e.target.value)} />
        <input placeholder="动作，如 update" value={action} onChange={(e) => setAction(e.target.value)} />
        <div className="row-actions"><button className="btn" onClick={load}>查询</button></div>
      </div>
      <div className="card">
        <table>
          <thead>
            <tr><th>ID</th><th>模块</th><th>动作</th><th>用户ID</th><th>目标ID</th><th>结果</th><th>时间</th></tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{item.id}</td><td>{item.module}</td><td>{item.action}</td><td>{item.userId}</td><td>{item.targetId}</td>
                <td>{item.success ? '成功' : '失败'}</td><td>{new Date(item.createdAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
