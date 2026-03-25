import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'
import { DataState } from '../components/DataState'
import { Modal } from '../components/Modal'

interface DepartmentForm {
  id?: number
  name: string
  description: string
  userIds: number[]
}

const initialForm: DepartmentForm = { name: '', description: '', userIds: [] }

export function DepartmentsPage() {
  const [items, setItems] = useState<any[]>([])
  const [users, setUsers] = useState<any[]>([])
  const [keyword, setKeyword] = useState('')
  const [form, setForm] = useState<DepartmentForm>(initialForm)
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
      const [departmentsRes, usersRes] = await Promise.all([
        api.get(`/departments?page=1&pageSize=50${query}`),
        api.get('/users?page=1&pageSize=100')
      ])
      setItems(departmentsRes.data.list ?? departmentsRes.data ?? [])
      setUsers(usersRes.data.list ?? [])
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || '部门列表加载失败')
      setItems([])
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
        await api.put(`/departments/${form.id}`, form)
      } else {
        await api.post('/departments', form)
      }
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError: any) {
      setFormError(submitError?.response?.data?.message || '保存部门失败')
    } finally {
      setSubmitting(false)
    }
  }

  const edit = (item: any) => {
    setForm({
      id: item.id,
      name: item.name,
      description: item.description,
      userIds: (item.users || []).map((user: any) => user.id)
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
    if (!confirm('确认删除该部门？')) return
    await api.delete(`/departments/${id}`)
    await load()
  }

  return (
    <section className="page-section">
      <div className="card form-grid">
        <h3>搜索</h3>
        <input aria-label="部门关键词搜索" value={keyword} placeholder="名称/描述" onChange={(e) => setKeyword(e.target.value)} />
        <div className="row-actions">
          <button className="btn" onClick={() => { void load() }}>查询</button>
          <button className="btn secondary" onClick={openCreateModal}>新增部门</button>
        </div>
      </div>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无部门数据" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>成员数</th><th>操作</th></tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{item.id}</td><td>{item.name}</td><td>{item.description}</td><td>{(item.users || []).length}</td>
                <td>
                  <button className="btn secondary" onClick={() => edit(item)}>编辑</button>
                  <button className="btn danger" onClick={() => { void onDelete(item.id) }}>删除</button>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      <Modal open={modalOpen} title={form.id ? '编辑部门' : '新增部门'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="department-name">名称</label>
          <input id="department-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <label htmlFor="department-description">描述</label>
          <input id="department-description" value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
          <label htmlFor="department-users">成员</label>
          <select id="department-users" multiple value={form.userIds.map(String)} onChange={(event) => {
            const selected = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
            setForm((prev) => ({ ...prev, userIds: selected }))
          }}>
            {users.map((user) => <option key={user.id} value={user.id}>{user.name}</option>)}
          </select>
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
