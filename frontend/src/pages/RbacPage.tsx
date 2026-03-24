import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'

interface RoleForm {
  id?: number
  name: string
  description: string
  permissionIds: number[]
}

interface PermissionForm {
  id?: number
  code: string
  name: string
  description: string
}

const initialRoleForm: RoleForm = { name: '', description: '', permissionIds: [] }
const initialPermissionForm: PermissionForm = { code: '', name: '', description: '' }

export function RbacPage() {
  const [roles, setRoles] = useState<any[]>([])
  const [permissions, setPermissions] = useState<any[]>([])
  const [roleForm, setRoleForm] = useState<RoleForm>(initialRoleForm)
  const [permissionForm, setPermissionForm] = useState<PermissionForm>(initialPermissionForm)

  const load = () => {
    void api.get('/rbac/roles').then((res) => setRoles(res.data))
    void api.get('/rbac/permissions').then((res) => setPermissions(res.data))
  }

  useEffect(() => {
    load()
  }, [])

  const submitRole = async (event: FormEvent) => {
    event.preventDefault()
    if (roleForm.id) {
      await api.put(`/rbac/roles/${roleForm.id}`, roleForm)
    } else {
      await api.post('/rbac/roles', roleForm)
    }
    setRoleForm(initialRoleForm)
    load()
  }

  const submitPermission = async (event: FormEvent) => {
    event.preventDefault()
    if (permissionForm.id) {
      await api.put(`/rbac/permissions/${permissionForm.id}`, permissionForm)
    } else {
      await api.post('/rbac/permissions', permissionForm)
    }
    setPermissionForm(initialPermissionForm)
    load()
  }

  const editRole = (role: any) => {
    setRoleForm({
      id: role.id,
      name: role.name,
      description: role.description,
      permissionIds: (role.permissions || []).map((item: any) => item.id)
    })
  }

  const editPermission = (permission: any) => {
    setPermissionForm({
      id: permission.id,
      code: permission.code,
      name: permission.name,
      description: permission.description
    })
  }

  const onDeleteRole = async (id: number, name: string) => {
    if (!confirm(`确认删除角色 ${name}？`)) return
    await api.delete(`/rbac/roles/${id}`)
    load()
  }

  const onDeletePermission = async (id: number, name: string) => {
    if (!confirm(`确认删除权限 ${name}？`)) return
    await api.delete(`/rbac/permissions/${id}`)
    load()
  }

  return (
    <section>
      <h2>RBAC 权限管理</h2>
      <div className="cards">
        <div className="card"><h3>角色数量</h3><strong>{roles.length}</strong></div>
        <div className="card"><h3>权限数量</h3><strong>{permissions.length}</strong></div>
      </div>

      <form className="card form-grid" onSubmit={submitPermission}>
        <h3>{permissionForm.id ? '编辑权限' : '新增权限'}</h3>
        <label htmlFor="permission-code">编码</label>
        <input id="permission-code" value={permissionForm.code} onChange={(e) => setPermissionForm((prev) => ({ ...prev, code: e.target.value }))} required />
        <label htmlFor="permission-name">名称</label>
        <input id="permission-name" value={permissionForm.name} onChange={(e) => setPermissionForm((prev) => ({ ...prev, name: e.target.value }))} required />
        <label htmlFor="permission-desc">描述</label>
        <input id="permission-desc" value={permissionForm.description} onChange={(e) => setPermissionForm((prev) => ({ ...prev, description: e.target.value }))} />
        <div className="row-actions">
          <button type="submit" className="btn">保存权限</button>
          <button type="button" className="btn secondary" onClick={() => setPermissionForm(initialPermissionForm)}>重置</button>
        </div>
      </form>

      <form className="card form-grid" onSubmit={submitRole}>
        <h3>{roleForm.id ? '编辑角色' : '新增角色'}</h3>
        <label htmlFor="role-name">角色名称</label>
        <input id="role-name" value={roleForm.name} onChange={(e) => setRoleForm((prev) => ({ ...prev, name: e.target.value }))} required />
        <label htmlFor="role-desc">描述</label>
        <input id="role-desc" value={roleForm.description} onChange={(e) => setRoleForm((prev) => ({ ...prev, description: e.target.value }))} />
        <label htmlFor="role-perms">权限分配</label>
        <select
          id="role-perms"
          multiple
          value={roleForm.permissionIds.map(String)}
          onChange={(event) => {
            const selected = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
            setRoleForm((prev) => ({ ...prev, permissionIds: selected }))
          }}
        >
          {permissions.map((permission) => <option key={permission.id} value={permission.id}>{permission.code}</option>)}
        </select>
        <div className="row-actions">
          <button type="submit" className="btn">保存角色</button>
          <button type="button" className="btn secondary" onClick={() => setRoleForm(initialRoleForm)}>重置</button>
        </div>
      </form>

      <div className="card">
        <h3>权限列表</h3>
        <table>
          <thead><tr><th>ID</th><th>编码</th><th>名称</th><th>描述</th><th>操作</th></tr></thead>
          <tbody>
            {permissions.map((p) => (
              <tr key={p.id}>
                <td>{p.id}</td><td>{p.code}</td><td>{p.name}</td><td>{p.description}</td>
                <td>
                  <button className="btn secondary" onClick={() => editPermission(p)}>编辑</button>
                  <button className="btn danger" onClick={() => onDeletePermission(p.id, p.name)}>删除</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>角色列表</h3>
        <table><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>权限数</th><th>操作</th></tr></thead><tbody>
          {roles.map((r) => (
            <tr key={r.id}>
              <td>{r.id}</td><td>{r.name}</td><td>{r.description}</td><td>{(r.permissions || []).length}</td>
              <td>
                <button className="btn secondary" onClick={() => editRole(r)}>编辑</button>
                {r.name === 'admin' ? '-' : <button className="btn danger" onClick={() => onDeleteRole(r.id, r.name)}>删除</button>}
              </td>
            </tr>
          ))}
        </tbody></table>
      </div>
    </section>
  )
}
