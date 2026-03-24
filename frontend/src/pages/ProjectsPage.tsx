import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api } from '../services/api'
import { GanttChart } from '../components/GanttChart'
import { TaskTree } from '../components/TaskTree'
import { Task } from '../types'
import { Modal } from '../components/Modal'

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

type SortKey = 'code' | 'name' | 'owners' | 'departments' | 'createdAt'
type SortOrder = 'asc' | 'desc'

export function ProjectsPage() {
  const [projects, setProjects] = useState<any[]>([])
  const [users, setUsers] = useState<any[]>([])
  const [departments, setDepartments] = useState<any[]>([])
  const [selected, setSelected] = useState<number>()
  const [keyword, setKeyword] = useState('')
  const [filter, setFilter] = useState<'all' | 'hasOwner' | 'hasDepartment'>('all')
  const [sortKey, setSortKey] = useState<SortKey>('createdAt')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [gantt, setGantt] = useState<Task[]>([])
  const [tree, setTree] = useState<Task[]>([])
  const [form, setForm] = useState<ProjectForm>(initialForm)
  const [modalOpen, setModalOpen] = useState(false)

  const load = async () => {
    const projectRes = await api.get('/projects?page=1&pageSize=200')
    const rawList = projectRes.data?.list ?? projectRes.data
    const list = Array.isArray(rawList) ? rawList : []
    setProjects(list)
    if (list.length > 0 && !selected) setSelected(list[0].id)

    const userRes = await api.get('/users?page=1&pageSize=200', { silent: true } as any).catch(() => ({ data: { list: [] } }))
    setUsers(Array.isArray(userRes.data?.list) ? userRes.data.list : [])
    const departmentRes = await api.get('/departments?page=1&pageSize=200', { silent: true } as any).catch(() => ({ data: { list: [] } }))
    setDepartments(Array.isArray(departmentRes.data?.list) ? departmentRes.data.list : [])
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (!selected) return
    void api.get(`/projects/${selected}/gantt`).then((res) => setGantt(Array.isArray(res.data) ? res.data : []))
    void api.get(`/projects/${selected}/task-tree`).then((res) => setTree(Array.isArray(res.data) ? res.data : []))
  }, [selected])

  const openCreateModal = () => {
    setForm(initialForm)
    setModalOpen(true)
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
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const payload = {
      ...form,
      startAt: form.startAt ? new Date(form.startAt).toISOString() : '',
      endAt: form.endAt ? new Date(form.endAt).toISOString() : ''
    }

    if (form.id) await api.put(`/projects/${form.id}`, payload)
    else await api.post('/projects', payload)

    setModalOpen(false)
    setForm(initialForm)
    await load()
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该项目？')) return
    await api.delete(`/projects/${id}`)
    await load()
  }

  const processedProjects = useMemo(() => {
    const lowerKeyword = keyword.trim().toLowerCase()
    let result = projects.filter((project) => {
      const keywordMatched = !lowerKeyword || [project.code, project.name, project.description].some((value) => String(value || '').toLowerCase().includes(lowerKeyword))
      if (!keywordMatched) return false

      if (filter === 'hasOwner') return (project.users || []).length > 0
      if (filter === 'hasDepartment') return (project.departments || []).length > 0
      return true
    })

    const getter = (project: any): string | number => {
      if (sortKey === 'owners') return (project.users || []).length
      if (sortKey === 'departments') return (project.departments || []).length
      return String(project[sortKey] ?? '')
    }

    result = [...result].sort((a, b) => {
      const left = getter(a)
      const right = getter(b)
      if (typeof left === 'number' && typeof right === 'number') return sortOrder === 'asc' ? left - right : right - left
      return sortOrder === 'asc' ? String(left).localeCompare(String(right)) : String(right).localeCompare(String(left))
    })

    return result
  }, [projects, keyword, filter, sortKey, sortOrder])

  const total = processedProjects.length
  const totalPages = Math.max(Math.ceil(total / pageSize), 1)
  const currentPage = Math.min(page, totalPages)
  const pagedProjects = processedProjects.slice((currentPage - 1) * pageSize, currentPage * pageSize)

  return (
    <section>
      <h2>项目列表 / 项目详情</h2>

      <div className="card toolbar-grid">
        <input value={keyword} placeholder="搜索：编码/名称/描述" onChange={(e) => { setKeyword(e.target.value); setPage(1) }} />
        <select value={filter} onChange={(e) => { setFilter(e.target.value as any); setPage(1) }}>
          <option value="all">全部项目</option>
          <option value="hasOwner">仅有负责人</option>
          <option value="hasDepartment">仅有部门</option>
        </select>
        <select value={sortKey} onChange={(e) => setSortKey(e.target.value as SortKey)}>
          <option value="createdAt">按创建时间</option>
          <option value="code">按编码</option>
          <option value="name">按名称</option>
          <option value="owners">按负责人数量</option>
          <option value="departments">按部门数量</option>
        </select>
        <select value={sortOrder} onChange={(e) => setSortOrder(e.target.value as SortOrder)}>
          <option value="desc">降序</option>
          <option value="asc">升序</option>
        </select>
        <button className="btn" onClick={openCreateModal}>新增项目</button>
      </div>

      <div className="card">
        <label htmlFor="project-select">选择项目</label>
        <select id="project-select" value={selected} onChange={(e) => setSelected(Number(e.target.value))}>
          {projects.map((p) => <option key={p.id} value={p.id}>{p.code} - {p.name}</option>)}
        </select>
      </div>

      <TaskTree tasks={tree} />

      <div className="card">
        <table><thead><tr><th>编码</th><th>名称</th><th>描述</th><th>负责人</th><th>部门</th><th>操作</th></tr></thead><tbody>
          {pagedProjects.map((p) => (
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

      <div className="card pagination-row">
        <span>共 {total} 条</span>
        <select value={pageSize} onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1) }}>
          <option value={10}>10/页</option>
          <option value={20}>20/页</option>
          <option value={50}>50/页</option>
        </select>
        <button className="btn secondary" disabled={currentPage <= 1} onClick={() => setPage((prev) => Math.max(prev - 1, 1))}>上一页</button>
        <span>{currentPage} / {totalPages}</span>
        <button className="btn secondary" disabled={currentPage >= totalPages} onClick={() => setPage((prev) => Math.min(prev + 1, totalPages))}>下一页</button>
      </div>

      <GanttChart tasks={gantt} />

      <Modal open={modalOpen} title={form.id ? '编辑项目' : '新增项目'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
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
      </Modal>
    </section>
  )
}
