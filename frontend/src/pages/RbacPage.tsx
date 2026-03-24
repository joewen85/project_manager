import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function RbacPage() {
  const [roles, setRoles] = useState<any[]>([])
  const [permissions, setPermissions] = useState<any[]>([])
  const load = () => {
    void api.get('/rbac/roles').then((res) => setRoles(res.data))
    void api.get('/rbac/permissions').then((res) => setPermissions(res.data))
  }

  useEffect(() => {
    load()
  }, [])

  const onDelete = async (id: number, name: string) => {
    if (!confirm(`确认删除角色 ${name}？`)) return
    await api.delete(`/rbac/roles/${id}`)
    load()
  }

  return (
    <section>
      <h2>RBAC 权限管理</h2>
      <div className="cards">
        <div className="card"><h3>角色数量</h3><strong>{roles.length}</strong></div>
        <div className="card"><h3>权限数量</h3><strong>{permissions.length}</strong></div>
      </div>
      <div className="card">
        <h3>角色列表</h3>
        <table><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>操作</th></tr></thead><tbody>
          {roles.map((r) => <tr key={r.id}><td>{r.id}</td><td>{r.name}</td><td>{r.description}</td><td>{r.name === 'admin' ? '-' : <button className="btn danger" onClick={() => onDelete(r.id, r.name)}>删除</button>}</td></tr>)}
        </tbody></table>
      </div>
    </section>
  )
}
