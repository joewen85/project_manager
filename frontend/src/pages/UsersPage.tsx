import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function UsersPage() {
  const [users, setUsers] = useState<any[]>([])
  useEffect(() => { void api.get('/users').then((res) => setUsers(res.data)) }, [])

  return (
    <section>
      <h2>用户管理</h2>
      <div className="card">
        <table><thead><tr><th>ID</th><th>用户名</th><th>姓名</th><th>邮箱</th></tr></thead><tbody>
          {users.map((u) => <tr key={u.id}><td>{u.id}</td><td>{u.username}</td><td>{u.name}</td><td>{u.email}</td></tr>)}
        </tbody></table>
      </div>
    </section>
  )
}
