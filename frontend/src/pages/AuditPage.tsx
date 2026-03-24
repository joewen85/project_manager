import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function AuditPage() {
  const [items, setItems] = useState<any[]>([])

  useEffect(() => {
    void api.get('/audit/logs?page=1&pageSize=100').then((res) => setItems(res.data.list ?? []))
  }, [])

  return (
    <section>
      <h2>操作审计日志</h2>
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
