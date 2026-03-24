import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'

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

  const load = () => {
    const query = keyword ? `&keyword=${encodeURIComponent(keyword)}` : ''
    void api.get(`/departments?page=1&pageSize=50${query}`).then((res) => setItems(res.data.list ?? res.data))
    void api.get('/users?page=1&pageSize=100').then((res) => setUsers(res.data.list ?? []))
  }
  useEffect(() => { load() }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id) {
      await api.put(`/departments/${form.id}`, form)
    } else {
      await api.post('/departments', form)
    }
    setForm(initialForm)
    load()
  }

  const edit = (item: any) => {
    setForm({
      id: item.id,
      name: item.name,
      description: item.description,
      userIds: (item.users || []).map((user: any) => user.id)
    })
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该部门？')) return
    await api.delete(`/departments/${id}`)
    load()
  }

  return (
    <section>
      <h2>部门管理</h2>

      <form className="card form-grid" onSubmit={submit}>
        <h3>{form.id ? '编辑部门' : '新增部门'}</h3>
        <label htmlFor="department-name">名称</label>
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
          <button type="submit" className="btn">保存</button>
          <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
        </div>
      </form>

      <div className="card form-grid">
        <h3>搜索</h3>
        <input value={keyword} placeholder="名称/描述" onChange={(e) => setKeyword(e.target.value)} />
        <div className="row-actions"><button className="btn" onClick={load}>查询</button></div>
      </div>

      <div className="card">
        <table><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>成员数</th><th>操作</th></tr></thead><tbody>
          {items.map((item) => (
            <tr key={item.id}>
              <td>{item.id}</td><td>{item.name}</td><td>{item.description}</td><td>{(item.users || []).length}</td>
              <td>
                <button className="btn secondary" onClick={() => edit(item)}>编辑</button>
                <button className="btn danger" onClick={() => onDelete(item.id)}>删除</button>
              </td>
            </tr>
          ))}
        </tbody></table>
      </div>
    </section>
  )
}
