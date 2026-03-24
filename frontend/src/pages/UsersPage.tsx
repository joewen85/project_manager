import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'

interface UserForm {
  id?: number
  username: string
  name: string
  email: string
  password: string
  isActive: boolean
  roleIds: number[]
  departmentIds: number[]
}

const initialForm: UserForm = {
  username: '',
  name: '',
  email: '',
  password: '',
  isActive: true,
  roleIds: [],
  departmentIds: []
}

export function UsersPage() {
  const [users, setUsers] = useState<any[]>([])
  const [roles, setRoles] = useState<any[]>([])
  const [departments, setDepartments] = useState<any[]>([])
  const [keyword, setKeyword] = useState('')
  const [form, setForm] = useState<UserForm>(initialForm)

  const load = () => {
    const query = keyword ? `&keyword=${encodeURIComponent(keyword)}` : ''
    void api.get(`/users?page=1&pageSize=50${query}`).then((res) => setUsers(res.data.list ?? res.data))
    void api.get('/rbac/roles').then((res) => setRoles(res.data))
    void api.get('/departments?page=1&pageSize=100').then((res) => setDepartments(res.data.list ?? []))
  }

  useEffect(() => { load() }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id) {
      await api.put(`/users/${form.id}`, {
        name: form.name,
        email: form.email,
        password: form.password,
        isActive: form.isActive,
        roleIds: form.roleIds,
        departmentIds: form.departmentIds
      })
    } else {
      await api.post('/users', {
        username: form.username,
        name: form.name,
        email: form.email,
        password: form.password,
        roleIds: form.roleIds,
        departmentIds: form.departmentIds
      })
    }
    setForm(initialForm)
    load()
  }

  const edit = (item: any) => {
    setForm({
      id: item.id,
      username: item.username,
      name: item.name,
      email: item.email,
      password: '',
      isActive: item.isActive,
      roleIds: (item.roles || []).map((role: any) => role.id),
      departmentIds: (item.departments || []).map((department: any) => department.id)
    })
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该用户？')) return
    await api.delete(`/users/${id}`)
    load()
  }

  return (
    <section>
      <h2>用户管理</h2>
      <form className="card form-grid" onSubmit={submit}>
        <h3>{form.id ? '编辑用户' : '新增用户'}</h3>
        {!form.id && <><label htmlFor="username">用户名</label><input id="username" value={form.username} onChange={(e) => setForm((prev) => ({ ...prev, username: e.target.value }))} required /></>}
        <label htmlFor="name">姓名</label>
        <input id="name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
        <label htmlFor="email">邮箱</label>
        <input id="email" value={form.email} onChange={(e) => setForm((prev) => ({ ...prev, email: e.target.value }))} required />
        <label htmlFor="password">密码{form.id ? '（留空不修改）' : ''}</label>
        <input id="password" type="password" value={form.password} onChange={(e) => setForm((prev) => ({ ...prev, password: e.target.value }))} required={!form.id} />

        <label htmlFor="roleIds">角色</label>
        <select id="roleIds" multiple value={form.roleIds.map(String)} onChange={(event) => {
          const selected = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
          setForm((prev) => ({ ...prev, roleIds: selected }))
        }}>
          {roles.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}
        </select>

        <label htmlFor="departmentIds">部门</label>
        <select id="departmentIds" multiple value={form.departmentIds.map(String)} onChange={(event) => {
          const selected = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
          setForm((prev) => ({ ...prev, departmentIds: selected }))
        }}>
          {departments.map((department) => <option key={department.id} value={department.id}>{department.name}</option>)}
        </select>

        {form.id && (
          <label>
            <input type="checkbox" checked={form.isActive} onChange={(e) => setForm((prev) => ({ ...prev, isActive: e.target.checked }))} /> 启用状态
          </label>
        )}

        <div className="row-actions">
          <button type="submit" className="btn">保存</button>
          <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
        </div>
      </form>

      <div className="card form-grid">
        <h3>搜索</h3>
        <input value={keyword} placeholder="用户名/姓名/邮箱" onChange={(e) => setKeyword(e.target.value)} />
        <div className="row-actions">
          <button className="btn" onClick={load}>查询</button>
        </div>
      </div>

      <div className="card">
        <table><thead><tr><th>ID</th><th>用户名</th><th>姓名</th><th>邮箱</th><th>状态</th><th>操作</th></tr></thead><tbody>
          {users.map((u) => (
            <tr key={u.id}>
              <td>{u.id}</td><td>{u.username}</td><td>{u.name}</td><td>{u.email}</td><td>{u.isActive ? '启用' : '禁用'}</td>
              <td>
                <button className="btn secondary" onClick={() => edit(u)}>编辑</button>
                <button className="btn danger" onClick={() => onDelete(u.id)}>删除</button>
              </td>
            </tr>
          ))}
        </tbody></table>
      </div>
    </section>
  )
}
