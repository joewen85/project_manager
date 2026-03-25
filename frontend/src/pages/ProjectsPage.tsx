import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api } from '../services/api'
import { GanttChart } from '../components/GanttChart'
import { TaskTree } from '../components/TaskTree'
import { Task } from '../types'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { DataState } from '../components/DataState'
import { formatDateTime } from '../utils/datetime'

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
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]

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
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailProject, setDetailProject] = useState<any | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [ownerKeyword, setOwnerKeyword] = useState('')
  const [departmentKeyword, setDepartmentKeyword] = useState('')

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const projectRes = await api.get('/projects?page=1&pageSize=200')
      const rawList = projectRes.data?.list ?? projectRes.data
      const list = Array.isArray(rawList) ? rawList : []
      setProjects(list)
      if (list.length > 0 && !selected) setSelected(list[0].id)
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || '项目列表加载失败')
      setProjects([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (!selected) return
    void api.get(`/projects/${selected}/gantt`).then((res) => setGantt(Array.isArray(res.data) ? res.data : []))
    void api.get(`/projects/${selected}/task-tree`).then((res) => setTree(Array.isArray(res.data) ? res.data : []))
  }, [selected])

  useEffect(() => {
    if (!modalOpen) return
    const timer = window.setTimeout(() => {
      const userQuery = ownerKeyword.trim() ? `&keyword=${encodeURIComponent(ownerKeyword.trim())}` : ''
      const departmentQuery = departmentKeyword.trim() ? `&keyword=${encodeURIComponent(departmentKeyword.trim())}` : ''
      void Promise.all([
        api.get(`/users?page=1&pageSize=200${userQuery}`, { silent: true } as any).catch(() => ({ data: { list: [] } })),
        api.get(`/departments?page=1&pageSize=200${departmentQuery}`, { silent: true } as any).catch(() => ({ data: { list: [] } }))
      ]).then(([userRes, departmentRes]) => {
        setUsers(Array.isArray(userRes.data?.list) ? userRes.data.list : [])
        setDepartments(Array.isArray(departmentRes.data?.list) ? departmentRes.data.list : [])
      })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [modalOpen, ownerKeyword, departmentKeyword])

  const openCreateModal = () => {
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setOwnerKeyword('')
    setDepartmentKeyword('')
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
    setFormError('')
    setFormSuccess('')
    setOwnerKeyword('')
    setDepartmentKeyword('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.startAt && form.endAt && new Date(form.startAt) > new Date(form.endAt)) {
      setFormError('结束时间必须晚于开始时间')
      return
    }
    const payload = {
      ...form,
      startAt: form.startAt ? new Date(form.startAt).toISOString() : '',
      endAt: form.endAt ? new Date(form.endAt).toISOString() : ''
    }

    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) await api.put(`/projects/${form.id}`, payload)
      else await api.post('/projects', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError: any) {
      setFormError(submitError?.response?.data?.message || '保存项目失败')
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该项目？')) return
    await api.delete(`/projects/${id}`)
    await load()
  }

  const viewDetail = async (item: any) => {
    try {
      const res = await api.get(`/projects/${item.id}`)
      setDetailProject(res.data || item)
    } catch {
      setDetailProject(item)
    }
    setDetailOpen(true)
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
    <section className="page-section">
      <div className="card toolbar-grid">
        <input aria-label="搜索项目" value={keyword} placeholder="搜索：编码/名称/描述" onChange={(e) => { setKeyword(e.target.value); setPage(1) }} />
        <select aria-label="项目筛选" value={filter} onChange={(e) => { setFilter(e.target.value as any); setPage(1) }}>
          <option value="all">全部项目</option>
          <option value="hasOwner">仅有负责人</option>
          <option value="hasDepartment">仅有部门</option>
        </select>
        <select aria-label="项目排序字段" value={sortKey} onChange={(e) => setSortKey(e.target.value as SortKey)}>
          <option value="createdAt">按创建时间</option>
          <option value="code">按编码</option>
          <option value="name">按名称</option>
          <option value="owners">按负责人数量</option>
          <option value="departments">按部门数量</option>
        </select>
        <select aria-label="项目排序方式" value={sortOrder} onChange={(e) => setSortOrder(e.target.value as SortOrder)}>
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
        <DataState loading={loading} error={error} empty={!loading && !error && pagedProjects.length === 0} emptyText="暂无匹配的项目" onRetry={() => { void load() }} />
        {!loading && !error && pagedProjects.length > 0 && (
          <table><thead><tr><th>编码</th><th>名称</th><th>描述</th><th>负责人</th><th>部门</th><th>操作</th></tr></thead><tbody>
            {pagedProjects.map((p) => (
              <tr key={p.id}>
                <td>{p.code}</td>
                <td>{p.name}</td>
                <td><span className="cell-ellipsis" title={p.description || '-'}>{p.description || '-'}</span></td>
                <td>{(p.users || []).length}</td>
                <td>{(p.departments || []).length}</td>
                <td>
                  <button className="btn secondary" onClick={() => { void viewDetail(p) }}>查看详情</button>
                  <button className="btn secondary" onClick={() => edit(p)}>编辑</button>
                  <button className="btn danger" onClick={() => onDelete(p.id)}>删除</button>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={currentPage} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <GanttChart tasks={gantt} />

      <Modal open={detailOpen} title="项目详情" onClose={() => setDetailOpen(false)}>
        {detailProject && (
          <div className="detail-grid">
            <section className="detail-section">
              <h4>基础信息</h4>
              <div className="detail-columns">
                <div><strong>项目ID：</strong>{detailProject.id}</div>
                <div><strong>编码：</strong>{detailProject.code || '-'}</div>
                <div><strong>名称：</strong>{detailProject.name || '-'}</div>
              </div>
              <div className="detail-description-card">
                <strong>描述</strong>
                <p>{detailProject.description || '-'}</p>
              </div>
            </section>
            <section className="detail-section">
              <h4>时间信息</h4>
              <div className="detail-columns">
                <div className="detail-time-line"><strong>项目周期：</strong>{formatDateTime(detailProject.startAt)} - {formatDateTime(detailProject.endAt)}</div>
              </div>
            </section>
            <section className="detail-section">
              <h4>关联信息</h4>
              <div className="detail-columns">
                <div><strong>负责人：</strong>{(detailProject.users || []).map((u: any) => `${u.name}(${u.username})`).join('，') || '-'}</div>
                <div><strong>参与部门：</strong>{(detailProject.departments || []).map((d: any) => d.name).join('，') || '-'}</div>
                <div><strong>任务数量：</strong>{Array.isArray(detailProject.tasks) ? detailProject.tasks.length : '-'}</div>
              </div>
            </section>
          </div>
        )}
      </Modal>

      <Modal open={modalOpen} title={form.id ? '编辑项目' : '新增项目'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="project-code">编码</label>
          <input id="project-code" value={form.code} onChange={(e) => setForm((prev) => ({ ...prev, code: e.target.value }))} required />
          <label className="required-label" htmlFor="project-name">名称</label>
          <input id="project-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <label htmlFor="project-description">描述</label>
          <textarea id="project-description" rows={4} value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
          <label htmlFor="project-start">开始时间</label>
          <input id="project-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} />
          <label htmlFor="project-end">结束时间</label>
          <input id="project-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} />
          <label htmlFor="project-users">项目负责人</label>
          <input
            aria-label="搜索负责人"
            placeholder="搜索负责人：姓名/用户名/邮箱"
            value={ownerKeyword}
            onChange={(e) => setOwnerKeyword(e.target.value)}
          />
          <div id="project-users" className="multi-checklist">
            {users.map((user) => (
              <label key={user.id} className="multi-check-item">
                <input
                  type="checkbox"
                  checked={form.userIds.includes(user.id)}
                  onChange={() => setForm((prev) => ({ ...prev, userIds: toggleNumber(prev.userIds, user.id) }))}
                />
                <span>{user.name} ({user.username})</span>
              </label>
            ))}
          </div>
          <label htmlFor="project-departments">参与部门</label>
          <input
            aria-label="搜索部门"
            placeholder="搜索部门名称"
            value={departmentKeyword}
            onChange={(e) => setDepartmentKeyword(e.target.value)}
          />
          <div id="project-departments" className="multi-checklist">
            {departments.map((department) => (
              <label key={department.id} className="multi-check-item">
                <input
                  type="checkbox"
                  checked={form.departmentIds.includes(department.id)}
                  onChange={() => setForm((prev) => ({ ...prev, departmentIds: toggleNumber(prev.departmentIds, department.id) }))}
                />
                <span>{department.name}</span>
              </label>
            ))}
          </div>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存项目'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
