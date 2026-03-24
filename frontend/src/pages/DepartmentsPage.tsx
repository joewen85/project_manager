import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function DepartmentsPage() {
  const [items, setItems] = useState<any[]>([])
  useEffect(() => { void api.get('/departments').then((res) => setItems(res.data)) }, [])

  return (
    <section>
      <h2>部门管理</h2>
      <div className="card">
        <table><thead><tr><th>ID</th><th>名称</th><th>描述</th></tr></thead><tbody>
          {items.map((item) => <tr key={item.id}><td>{item.id}</td><td>{item.name}</td><td>{item.description}</td></tr>)}
        </tbody></table>
      </div>
    </section>
  )
}
