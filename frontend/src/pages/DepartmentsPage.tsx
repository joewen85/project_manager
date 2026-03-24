import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function DepartmentsPage() {
  const [items, setItems] = useState<any[]>([])
  const load = () => {
    void api.get('/departments?page=1&pageSize=50').then((res) => setItems(res.data.list ?? res.data))
  }
  useEffect(() => { load() }, [])

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该部门？')) return
    await api.delete(`/departments/${id}`)
    load()
  }

  return (
    <section>
      <h2>部门管理</h2>
      <div className="card">
        <table><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>操作</th></tr></thead><tbody>
          {items.map((item) => <tr key={item.id}><td>{item.id}</td><td>{item.name}</td><td>{item.description}</td><td><button className="btn danger" onClick={() => onDelete(item.id)}>删除</button></td></tr>)}
        </tbody></table>
      </div>
    </section>
  )
}
