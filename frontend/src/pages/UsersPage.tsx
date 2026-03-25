import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'
import { DataState } from '../components/DataState'
import { Modal } from '../components/Modal'

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
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [modalOpen, setModalOpen] = useState(false)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const query = keyword ? `&keyword=${encodeURIComponent(keyword)}` : ''
      const [usersRes, rolesRes, departmentsRes] = await Promise.all([
        api.get(`/users?page=1&pageSize=50${query}`),
        api.get('/rbac/roles'),
        api.get('/departments?page=1&pageSize=100')
      ])
      setUsers(usersRes.data.list ?? usersRes.data ?? [])
      setRoles(rolesRes.data ?? [])
      setDepartments(departmentsRes.data.list ?? [])
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || '用户列表加载失败')
      setUsers([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      setSubmitting(true)
      setFormError('')
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
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError: any) {
      setFormError(submitError?.response?.data?.message || '保存用户失败')
    } finally {
      setSubmitting(false)
    }
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
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openCreateModal = () => {
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该用户？')) return
    await api.delete(`/users/${id}`)
    await load()
  }

  return (
    <section className="page-section">
      <div className="card form-grid">
        <h3>搜索</h3>
        <input aria-label="用户关键词搜索" value={keyword} placeholder="用户名/姓名/邮箱" onChange={(e) => setKeyword(e.target.value)} />
        <div className="row-actions">
          <button className="btn" onClick={() => { void load() }}>查询</button>
          <button className="btn secondary" onClick={openCreateModal}>新增用户</button>
        </div>
      </div>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && users.length === 0} emptyText="暂无用户数据" onRetry={() => { void load() }} />
        {!loading && !error && users.length > 0 && (
          <table><thead><tr><th>ID</th><th>用户名</th><th>姓名</th><th>邮箱</th><th>状态</th><th>操作</th></tr></thead><tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.id}</td><td>{u.username}</td><td>{u.name}</td><td>{u.email}</td><td>{u.isActive ? '启用' : '禁用'}</td>
                <td>
                  <button className="btn secondary" onClick={() => edit(u)}>编辑</button>
                  <button className="btn danger" onClick={() => { void onDelete(u.id) }}>删除</button>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      <Modal open={modalOpen} title={form.id ? '编辑用户' : '新增用户'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          {!form.id && <><label className="required-label" htmlFor="username">用户名</label><input id="username" value={form.username} onChange={(e) => setForm((prev) => ({ ...prev, username: e.target.value }))} required /></>}
          <label className="required-label" htmlFor="name">姓名</label>
          <input id="name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <label className="required-label" htmlFor="email">邮箱</label>
          <input id="email" value={form.email} onChange={(e) => setForm((prev) => ({ ...prev, email: e.target.value }))} required />
          <label className={!form.id ? 'required-label' : ''} htmlFor="password">密码{form.id ? '（留空不修改）' : ''}</label>
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
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
