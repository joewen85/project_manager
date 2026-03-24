import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'
import { GanttChart } from '../components/GanttChart'
import { TaskTree } from '../components/TaskTree'
import { Task } from '../types'

interface ProjectForm {
  id?: number
  code: string
  name: string
  description: string
  startAt: string
  endAt: string
  userIds: number[]
  departmentIds: number[]
}

const initialForm: ProjectForm = { code: '', name: '', description: '', startAt: '', endAt: '', userIds: [], departmentIds: [] }

export function ProjectsPage() {
  const [projects, setProjects] = useState<any[]>([])
  const [users, setUsers] = useState<any[]>([])
  const [departments, setDepartments] = useState<any[]>([])
  const [selected, setSelected] = useState<number>()
  const [keyword, setKeyword] = useState('')
  const [gantt, setGantt] = useState<Task[]>([])
  const [tree, setTree] = useState<Task[]>([])
  const [form, setForm] = useState<ProjectForm>(initialForm)

  const load = async () => {
    const query = keyword ? `&keyword=${encodeURIComponent(keyword)}` : ''
    const projectRes = await api.get(`/projects?page=1&pageSize=50${query}`)
    const list = projectRes.data.list ?? projectRes.data
    setProjects(list)
    if (list.length > 0 && !selected) setSelected(list[0].id)
    const userRes = await api.get('/users?page=1&pageSize=100')
    setUsers(userRes.data.list ?? [])
    const departmentRes = await api.get('/departments?page=1&pageSize=100')
    setDepartments(departmentRes.data.list ?? [])
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (!selected) return
    void api.get(`/projects/${selected}/gantt`).then((res) => setGantt(res.data))
    void api.get(`/projects/${selected}/task-tree`).then((res) => setTree(res.data))
  }, [selected])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const payload = {
      ...form,
      startAt: form.startAt ? new Date(form.startAt).toISOString() : '',
      endAt: form.endAt ? new Date(form.endAt).toISOString() : ''
    }

    if (form.id) {
      await api.put(`/projects/${form.id}`, payload)
    } else {
      await api.post('/projects', payload)
    }

    setForm(initialForm)
    await load()
  }

  const edit = (item: any) => {
    setForm({
      id: item.id,
      code: item.code,
      name: item.name,
      description: item.description,
      startAt: item.startAt ? item.startAt.slice(0, 16) : '',
      endAt: item.endAt ? item.endAt.slice(0, 16) : '',
      userIds: (item.users || []).map((user: any) => user.id),
      departmentIds: (item.departments || []).map((department: any) => department.id)
    })
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该项目？')) return
    await api.delete(`/projects/${id}`)
    await load()
  }

  return (
    <section>
      <h2>项目列表 / 项目详情</h2>

      <form className="card form-grid" onSubmit={submit}>
        <h3>{form.id ? '编辑项目' : '新增项目'}</h3>
        <label htmlFor="project-code">编码</label>
        <input id="project-code" value={form.code} onChange={(e) => setForm((prev) => ({ ...prev, code: e.target.value }))} required />
        <label htmlFor="project-name">名称</label>
        <input id="project-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
        <label htmlFor="project-description">描述</label>
        <input id="project-description" value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
        <label htmlFor="project-start">开始时间</label>
        <input id="project-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} />
        <label htmlFor="project-end">结束时间</label>
        <input id="project-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} />

        <label htmlFor="project-users">项目负责人</label>
        <select id="project-users" multiple value={form.userIds.map(String)} onChange={(event) => {
          const selectedIds = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
          setForm((prev) => ({ ...prev, userIds: selectedIds }))
        }}>
          {users.map((user) => <option key={user.id} value={user.id}>{user.name}</option>)}
        </select>

        <label htmlFor="project-departments">参与部门</label>
        <select id="project-departments" multiple value={form.departmentIds.map(String)} onChange={(event) => {
          const selectedIds = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
          setForm((prev) => ({ ...prev, departmentIds: selectedIds }))
        }}>
          {departments.map((department) => <option key={department.id} value={department.id}>{department.name}</option>)}
        </select>

        <div className="row-actions">
          <button type="submit" className="btn">保存项目</button>
          <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
        </div>
      </form>

      <div className="card form-grid">
        <h3>搜索</h3>
        <input value={keyword} placeholder="编码/名称/描述" onChange={(e) => setKeyword(e.target.value)} />
        <div className="row-actions"><button className="btn" onClick={() => void load()}>查询</button></div>
      </div>

      <div className="card">
        <label htmlFor="project-select">选择项目</label>
        <select id="project-select" value={selected} onChange={(e) => setSelected(Number(e.target.value))}>
          {projects.map((p) => <option key={p.id} value={p.id}>{p.code} - {p.name}</option>)}
        </select>
      </div>

      <div className="card">
        <table><thead><tr><th>编码</th><th>名称</th><th>描述</th><th>负责人</th><th>部门</th><th>操作</th></tr></thead><tbody>
          {projects.map((p) => (
            <tr key={p.id}>
              <td>{p.code}</td><td>{p.name}</td><td>{p.description}</td><td>{(p.users || []).length}</td><td>{(p.departments || []).length}</td>
              <td>
                <button className="btn secondary" onClick={() => edit(p)}>编辑</button>
                <button className="btn danger" onClick={() => onDelete(p.id)}>删除</button>
              </td>
            </tr>
          ))}
        </tbody></table>
      </div>
      <GanttChart tasks={gantt} />
      <TaskTree tasks={tree} />
    </section>
  )
}
