import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'
import { DataState } from '../components/DataState'
import { Modal } from '../components/Modal'

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
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submittingRole, setSubmittingRole] = useState(false)
  const [submittingPermission, setSubmittingPermission] = useState(false)
  const [roleFormError, setRoleFormError] = useState('')
  const [permissionFormError, setPermissionFormError] = useState('')
  const [roleFormSuccess, setRoleFormSuccess] = useState('')
  const [permissionFormSuccess, setPermissionFormSuccess] = useState('')
  const [permissionModalOpen, setPermissionModalOpen] = useState(false)
  const [roleModalOpen, setRoleModalOpen] = useState(false)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [rolesRes, permissionsRes] = await Promise.all([
        api.get('/rbac/roles'),
        api.get('/rbac/permissions')
      ])
      setRoles(rolesRes.data ?? [])
      setPermissions(permissionsRes.data ?? [])
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || 'RBAC 数据加载失败')
      setRoles([])
      setPermissions([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const submitRole = async (event: FormEvent) => {
    event.preventDefault()
    try {
      setSubmittingRole(true)
      setRoleFormError('')
      if (roleForm.id) {
        await api.put(`/rbac/roles/${roleForm.id}`, roleForm)
      } else {
        await api.post('/rbac/roles', roleForm)
      }
      setRoleFormSuccess('保存成功')
      setRoleModalOpen(false)
      setRoleForm(initialRoleForm)
      await load()
    } catch (submitError: any) {
      setRoleFormError(submitError?.response?.data?.message || '保存角色失败')
    } finally {
      setSubmittingRole(false)
    }
  }

  const submitPermission = async (event: FormEvent) => {
    event.preventDefault()
    try {
      setSubmittingPermission(true)
      setPermissionFormError('')
      if (permissionForm.id) {
        await api.put(`/rbac/permissions/${permissionForm.id}`, permissionForm)
      } else {
        await api.post('/rbac/permissions', permissionForm)
      }
      setPermissionFormSuccess('保存成功')
      setPermissionModalOpen(false)
      setPermissionForm(initialPermissionForm)
      await load()
    } catch (submitError: any) {
      setPermissionFormError(submitError?.response?.data?.message || '保存权限失败')
    } finally {
      setSubmittingPermission(false)
    }
  }

  const editRole = (role: any) => {
    setRoleForm({
      id: role.id,
      name: role.name,
      description: role.description,
      permissionIds: (role.permissions || []).map((item: any) => item.id)
    })
    setRoleFormError('')
    setRoleFormSuccess('')
    setRoleModalOpen(true)
  }

  const editPermission = (permission: any) => {
    setPermissionForm({
      id: permission.id,
      code: permission.code,
      name: permission.name,
      description: permission.description
    })
    setPermissionFormError('')
    setPermissionFormSuccess('')
    setPermissionModalOpen(true)
  }

  const openCreateRoleModal = () => {
    setRoleForm(initialRoleForm)
    setRoleFormError('')
    setRoleFormSuccess('')
    setRoleModalOpen(true)
  }

  const openCreatePermissionModal = () => {
    setPermissionForm(initialPermissionForm)
    setPermissionFormError('')
    setPermissionFormSuccess('')
    setPermissionModalOpen(true)
  }

  const onDeleteRole = async (id: number, name: string) => {
    if (!confirm(`确认删除角色 ${name}？`)) return
    await api.delete(`/rbac/roles/${id}`)
    await load()
  }

  const onDeletePermission = async (id: number, name: string) => {
    if (!confirm(`确认删除权限 ${name}？`)) return
    await api.delete(`/rbac/permissions/${id}`)
    await load()
  }

  return (
    <section className="page-section">
      <div className="cards">
        <div className="card"><h3>角色数量</h3><strong>{roles.length}</strong></div>
        <div className="card"><h3>权限数量</h3><strong>{permissions.length}</strong></div>
      </div>

      <div className="card row-actions">
        <button className="btn" onClick={openCreatePermissionModal}>新增权限</button>
        <button className="btn secondary" onClick={openCreateRoleModal}>新增角色</button>
      </div>

      <div className="card">
        <h3>权限列表</h3>
        <DataState loading={loading} error={error} empty={!loading && !error && permissions.length === 0} emptyText="暂无权限数据" onRetry={() => { void load() }} />
        {!loading && !error && permissions.length > 0 && (
          <table>
            <thead><tr><th>ID</th><th>编码</th><th>名称</th><th>描述</th><th>操作</th></tr></thead>
            <tbody>
              {permissions.map((p) => (
                <tr key={p.id}>
                  <td>{p.id}</td><td>{p.code}</td><td>{p.name}</td><td>{p.description}</td>
                  <td>
                    <button className="btn secondary" onClick={() => editPermission(p)}>编辑</button>
                    <button className="btn danger" onClick={() => { void onDeletePermission(p.id, p.name) }}>删除</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="card">
        <h3>角色列表</h3>
        <DataState loading={loading} error={error} empty={!loading && !error && roles.length === 0} emptyText="暂无角色数据" onRetry={() => { void load() }} />
        {!loading && !error && roles.length > 0 && (
          <table><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>权限数</th><th>操作</th></tr></thead><tbody>
            {roles.map((r) => (
              <tr key={r.id}>
                <td>{r.id}</td><td>{r.name}</td><td>{r.description}</td><td>{(r.permissions || []).length}</td>
                <td>
                  <button className="btn secondary" onClick={() => editRole(r)}>编辑</button>
                  {r.name === 'admin' ? '-' : <button className="btn danger" onClick={() => { void onDeleteRole(r.id, r.name) }}>删除</button>}
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      <Modal open={permissionModalOpen} title={permissionForm.id ? '编辑权限' : '新增权限'} onClose={() => setPermissionModalOpen(false)}>
        <form className="form-grid" onSubmit={submitPermission}>
          <label className="required-label" htmlFor="permission-code">编码</label>
          <input id="permission-code" value={permissionForm.code} onChange={(e) => setPermissionForm((prev) => ({ ...prev, code: e.target.value }))} required />
          <label className="required-label" htmlFor="permission-name">名称</label>
          <input id="permission-name" value={permissionForm.name} onChange={(e) => setPermissionForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <label htmlFor="permission-desc">描述</label>
          <input id="permission-desc" value={permissionForm.description} onChange={(e) => setPermissionForm((prev) => ({ ...prev, description: e.target.value }))} />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submittingPermission}>{submittingPermission ? '保存中...' : '保存权限'}</button>
            <button type="button" className="btn secondary" onClick={() => setPermissionForm(initialPermissionForm)}>重置</button>
          </div>
          {permissionFormError && <p className="error">{permissionFormError}</p>}
          {permissionFormSuccess && <p className="success">{permissionFormSuccess}</p>}
        </form>
      </Modal>

      <Modal open={roleModalOpen} title={roleForm.id ? '编辑角色' : '新增角色'} onClose={() => setRoleModalOpen(false)}>
        <form className="form-grid" onSubmit={submitRole}>
          <label className="required-label" htmlFor="role-name">角色名称</label>
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
            <button type="submit" className="btn" disabled={submittingRole}>{submittingRole ? '保存中...' : '保存角色'}</button>
            <button type="button" className="btn secondary" onClick={() => setRoleForm(initialRoleForm)}>重置</button>
          </div>
          {roleFormError && <p className="error">{roleFormError}</p>}
          {roleFormSuccess && <p className="success">{roleFormSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
