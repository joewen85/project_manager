import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function UsersPage() {
  const [users, setUsers] = useState<any[]>([])
  const load = () => {
    void api.get('/users?page=1&pageSize=50').then((res) => setUsers(res.data.list ?? res.data))
  }
  useEffect(() => { load() }, [])

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该用户？')) return
    await api.delete(`/users/${id}`)
    load()
  }

  return (
    <section>
      <h2>用户管理</h2>
      <div className="card">
        <table><thead><tr><th>ID</th><th>用户名</th><th>姓名</th><th>邮箱</th><th>操作</th></tr></thead><tbody>
          {users.map((u) => <tr key={u.id}><td>{u.id}</td><td>{u.username}</td><td>{u.name}</td><td>{u.email}</td><td><button className="btn danger" onClick={() => onDelete(u.id)}>删除</button></td></tr>)}
        </tbody></table>
      </div>
    </section>
  )
}
