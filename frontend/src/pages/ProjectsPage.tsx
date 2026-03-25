import { FormEvent, useEffect, useState } from 'react'
import { api, fetchArray, fetchData, fetchPage, readApiError } from '../services/api'
import { GanttChart } from '../components/GanttChart'
import { TaskTree } from '../components/TaskTree'
import { Task, Department, Project, User } from '../types'
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

type SortKey = 'code' | 'name' | 'createdAt' | 'startAt' | 'endAt'
type SortOrder = 'asc' | 'desc'
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]

export function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [departments, setDepartments] = useState<Department[]>([])
  const [selected, setSelected] = useState<number>()
  const [keyword, setKeyword] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('createdAt')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [gantt, setGantt] = useState<Task[]>([])
  const [tree, setTree] = useState<Task[]>([])
  const [form, setForm] = useState<ProjectForm>(initialForm)
  const [modalOpen, setModalOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailProject, setDetailProject] = useState<Project | null>(null)
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
      const projectPage = await fetchPage<Project>(
        '/projects',
        { page, pageSize, keyword, sortBy: sortKey, sortOrder },
        { page, pageSize }
      )
      setProjects(projectPage.list)
      setTotal(projectPage.total)
      if (projectPage.list.length > 0 && !selected) setSelected(projectPage.list[0].id)
      if (projectPage.list.length === 0) setSelected(undefined)
    } catch (loadError) {
      setError(readApiError(loadError, '项目列表加载失败'))
      setProjects([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [page, pageSize, keyword, sortKey, sortOrder])

  useEffect(() => {
    if (!selected) return
    void fetchArray<Task>(`/projects/${selected}/gantt`, undefined, { silent: true }).then(setGantt).catch(() => setGantt([]))
    void fetchArray<Task>(`/projects/${selected}/task-tree`, undefined, { silent: true }).then(setTree).catch(() => setTree([]))
  }, [selected])

  useEffect(() => {
    if (!modalOpen) return
    const timer = window.setTimeout(() => {
      void Promise.all([
        fetchPage<User>('/users', { page: 1, pageSize: 200, keyword: ownerKeyword.trim() }, { page: 1, pageSize: 200 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 200 })),
        fetchPage<Department>('/departments', { page: 1, pageSize: 200, keyword: departmentKeyword.trim() }, { page: 1, pageSize: 200 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 200 }))
      ]).then(([userPage, departmentPage]) => {
        setUsers(userPage.list)
        setDepartments(departmentPage.list)
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

  const edit = (item: Project) => {
    setForm({
      id: item.id,
      code: item.code,
      name: item.name,
      description: item.description,
      startAt: item.startAt ? item.startAt.slice(0, 16) : '',
      endAt: item.endAt ? item.endAt.slice(0, 16) : '',
      userIds: (item.users || []).map((user) => user.id),
      departmentIds: (item.departments || []).map((department) => department.id)
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
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存项目失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该项目？')) return
    try {
      await api.delete(`/projects/${id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除项目失败'))
    }
  }

  const viewDetail = async (item: Project) => {
    try {
      const detail = await fetchData<Project>(`/projects/${item.id}`)
      setDetailProject(detail || item)
    } catch {
      setDetailProject(item)
    }
    setDetailOpen(true)
  }

  return (
    <section className="page-section">
      <div className="card toolbar-grid">
        <input aria-label="搜索项目" value={keyword} placeholder="搜索：编码/名称/描述" onChange={(e) => { setKeyword(e.target.value); setPage(1) }} />
        <select aria-label="项目排序字段" value={sortKey} onChange={(e) => { setSortKey(e.target.value as SortKey); setPage(1) }}>
          <option value="createdAt">按创建时间</option>
          <option value="code">按编码</option>
          <option value="name">按名称</option>
          <option value="startAt">按开始时间</option>
          <option value="endAt">按结束时间</option>
        </select>
        <select aria-label="项目排序方式" value={sortOrder} onChange={(e) => { setSortOrder(e.target.value as SortOrder); setPage(1) }}>
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
        <DataState loading={loading} error={error} empty={!loading && !error && projects.length === 0} emptyText="暂无匹配的项目" onRetry={() => { void load() }} />
        {!loading && !error && projects.length > 0 && (
          <table><thead><tr><th>编码</th><th>名称</th><th>描述</th><th>负责人</th><th>部门</th><th>操作</th></tr></thead><tbody>
            {projects.map((p) => (
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

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

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
                <div><strong>负责人：</strong>{(detailProject.users || []).map((u) => `${u.name}(${u.username})`).join('，') || '-'}</div>
                <div><strong>参与部门：</strong>{(detailProject.departments || []).map((d) => d.name).join('，') || '-'}</div>
                <div><strong>任务数量：</strong>{Array.isArray(detailProject.tasks) ? detailProject.tasks.length : '-'}</div>
              </div>
            </section>
          </div>
        )}
      </Modal>

      <Modal open={modalOpen} title={form.id ? '编辑项目' : '新增项目'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label htmlFor="project-code">编码（可空自动生成）</label>
          <input id="project-code" value={form.code} onChange={(e) => setForm((prev) => ({ ...prev, code: e.target.value }))} />
          <label className="required-label" htmlFor="project-name">名称</label>
          <input id="project-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <label htmlFor="project-description">描述</label>
          <textarea id="project-description" rows={4} value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
          <label htmlFor="project-start">开始时间</label>
          <input id="project-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} />
          <label htmlFor="project-end">结束时间</label>
          <input id="project-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} />
          <label htmlFor="project-users">项目负责人</label>
          <input aria-label="搜索负责人" placeholder="搜索负责人：姓名/用户名/邮箱" value={ownerKeyword} onChange={(e) => setOwnerKeyword(e.target.value)} />
          <div id="project-users" className="multi-checklist">
            {users.map((user) => (
              <label key={user.id} className="multi-check-item">
                <input type="checkbox" checked={form.userIds.includes(user.id)} onChange={() => setForm((prev) => ({ ...prev, userIds: toggleNumber(prev.userIds, user.id) }))} />
                <span>{user.name} ({user.username})</span>
              </label>
            ))}
          </div>
          <label htmlFor="project-departments">参与部门</label>
          <input aria-label="搜索部门" placeholder="搜索部门名称" value={departmentKeyword} onChange={(e) => setDepartmentKeyword(e.target.value)} />
          <div id="project-departments" className="multi-checklist">
            {departments.map((department) => (
              <label key={department.id} className="multi-check-item">
                <input type="checkbox" checked={form.departmentIds.includes(department.id)} onChange={() => setForm((prev) => ({ ...prev, departmentIds: toggleNumber(prev.departmentIds, department.id) }))} />
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
