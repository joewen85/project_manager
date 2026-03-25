import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api } from '../services/api'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { DataState } from '../components/DataState'

const statusLabel: Record<string, string> = {
  pending: '待处理',
  queued: '排队中',
  processing: '处理中',
  completed: '已完成'
}

interface TaskForm {
  id?: number
  taskNo: string
  title: string
  description: string
  status: string
  progress: number
  startAt: string
  endAt: string
  projectId: number
  parentId?: number
  assigneeIds: number[]
}

const initialForm: TaskForm = {
  taskNo: '',
  title: '',
  description: '',
  status: 'pending',
  progress: 0,
  startAt: '',
  endAt: '',
  projectId: 0,
  assigneeIds: []
}

type SortKey = 'taskNo' | 'title' | 'status' | 'progress' | 'startAt' | 'endAt'
type SortOrder = 'asc' | 'desc'
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]

export function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  const [users, setUsers] = useState<any[]>([])
  const [projects, setProjects] = useState<any[]>([])
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [projectFilter, setProjectFilter] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('startAt')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [form, setForm] = useState<TaskForm>(initialForm)
  const [modalOpen, setModalOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [assigneeKeyword, setAssigneeKeyword] = useState('')

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [taskRes, projectRes] = await Promise.all([
        api.get('/tasks?page=1&pageSize=500'),
        api.get('/projects?page=1&pageSize=200')
      ])

      const rawTasks = taskRes.data?.list ?? taskRes.data
      setTasks(Array.isArray(rawTasks) ? rawTasks : [])
      const projectList = Array.isArray(projectRes.data?.list) ? projectRes.data.list : []
      setProjects(projectList)
      if (!form.projectId && projectList.length > 0) {
        setForm((prev) => ({ ...prev, projectId: projectList[0].id }))
      }
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || '任务列表加载失败')
      setTasks([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (!modalOpen) return
    const timer = window.setTimeout(() => {
      const query = assigneeKeyword.trim() ? `&keyword=${encodeURIComponent(assigneeKeyword.trim())}` : ''
      void api.get(`/users?page=1&pageSize=200${query}`, { silent: true } as any)
        .then((res) => setUsers(Array.isArray(res.data?.list) ? res.data.list : []))
        .catch(() => setUsers([]))
    }, 300)
    return () => window.clearTimeout(timer)
  }, [modalOpen, assigneeKeyword])

  const openCreateModal = () => {
    setForm((prev) => ({ ...initialForm, projectId: prev.projectId || projects[0]?.id || 0 }))
    setFormError('')
    setFormSuccess('')
    setAssigneeKeyword('')
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
      endAt: form.endAt ? new Date(form.endAt).toISOString() : '',
      parentId: form.parentId || undefined
    }

    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) await api.put(`/tasks/${form.id}`, payload)
      else await api.post('/tasks', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError: any) {
      setFormError(submitError?.response?.data?.message || '保存任务失败')
    } finally {
      setSubmitting(false)
    }
  }

  const edit = (item: any) => {
    setForm({
      id: item.id,
      taskNo: item.taskNo,
      title: item.title,
      description: item.description,
      status: item.status,
      progress: item.progress,
      startAt: item.startAt ? item.startAt.slice(0, 16) : '',
      endAt: item.endAt ? item.endAt.slice(0, 16) : '',
      projectId: item.projectId,
      parentId: item.parentId,
      assigneeIds: (item.assignees || []).map((user: any) => user.id)
    })
    setFormError('')
    setFormSuccess('')
    setAssigneeKeyword('')
    setModalOpen(true)
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该任务？')) return
    await api.delete(`/tasks/${id}`)
    await load()
  }

  const processedTasks = useMemo(() => {
    const lowerKeyword = keyword.trim().toLowerCase()
    let result = tasks.filter((task) => {
      const keywordMatched = !lowerKeyword || [task.taskNo, task.title, task.description].some((value) => String(value || '').toLowerCase().includes(lowerKeyword))
      if (!keywordMatched) return false
      if (status && task.status !== status) return false
      if (projectFilter && String(task.projectId) !== projectFilter) return false
      return true
    })

    const getter = (task: any): string | number => {
      if (sortKey === 'progress') return Number(task.progress || 0)
      return String(task[sortKey] ?? '')
    }

    result = [...result].sort((a, b) => {
      const left = getter(a)
      const right = getter(b)
      if (typeof left === 'number' && typeof right === 'number') return sortOrder === 'asc' ? left - right : right - left
      return sortOrder === 'asc' ? String(left).localeCompare(String(right)) : String(right).localeCompare(String(left))
    })

    return result
  }, [tasks, keyword, status, projectFilter, sortKey, sortOrder])

  const total = processedTasks.length
  const totalPages = Math.max(Math.ceil(total / pageSize), 1)
  const currentPage = Math.min(page, totalPages)
  const pagedTasks = processedTasks.slice((currentPage - 1) * pageSize, currentPage * pageSize)

  return (
    <section className="page-section">
      <div className="card toolbar-grid">
        <input aria-label="搜索任务" value={keyword} placeholder="搜索：编号/标题/描述" onChange={(e) => { setKeyword(e.target.value); setPage(1) }} />
        <select aria-label="任务状态筛选" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
          <option value="">全部状态</option>
          {Object.keys(statusLabel).map((key) => <option key={key} value={key}>{statusLabel[key]}</option>)}
        </select>
        <select aria-label="任务项目筛选" value={projectFilter} onChange={(e) => { setProjectFilter(e.target.value); setPage(1) }}>
          <option value="">全部项目</option>
          {projects.map((project) => <option key={project.id} value={project.id}>{project.code} - {project.name}</option>)}
        </select>
        <select aria-label="任务排序字段" value={sortKey} onChange={(e) => setSortKey(e.target.value as SortKey)}>
          <option value="startAt">按开始时间</option>
          <option value="endAt">按结束时间</option>
          <option value="taskNo">按任务编号</option>
          <option value="title">按标题</option>
          <option value="status">按状态</option>
          <option value="progress">按进度</option>
        </select>
        <select aria-label="任务排序方式" value={sortOrder} onChange={(e) => setSortOrder(e.target.value as SortOrder)}>
          <option value="desc">降序</option>
          <option value="asc">升序</option>
        </select>
        <button className="btn" onClick={openCreateModal}>新增任务</button>
      </div>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && pagedTasks.length === 0} emptyText="暂无匹配的任务" onRetry={() => { void load() }} />
        {!loading && !error && pagedTasks.length > 0 && (
          <table><thead><tr><th>任务编号</th><th>标题</th><th>状态</th><th>进度</th><th>开始</th><th>结束</th><th>执行人</th><th>操作</th></tr></thead><tbody>
            {pagedTasks.map((task) => (
              <tr key={task.id}>
                <td>{task.taskNo}</td><td>{task.title}</td><td>{statusLabel[task.status]}</td><td>{task.progress}%</td>
                <td>{task.startAt ? new Date(task.startAt).toLocaleString() : '-'}</td>
                <td>{task.endAt ? new Date(task.endAt).toLocaleString() : '-'}</td>
                <td>{(task.assignees || []).length}</td>
                <td>
                  <button className="btn secondary" onClick={() => edit(task)}>编辑</button>
                  <button className="btn danger" onClick={() => onDelete(task.id)}>删除</button>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={currentPage} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑任务' : '新增任务'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label htmlFor="task-no">任务编号（可空自动生成）</label>
          <input id="task-no" value={form.taskNo} onChange={(e) => setForm((prev) => ({ ...prev, taskNo: e.target.value }))} />
          <label className="required-label" htmlFor="task-title">标题</label>
          <input id="task-title" value={form.title} onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))} required />
          <label htmlFor="task-description">描述</label>
          <input id="task-description" value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
          <label className="required-label" htmlFor="task-project">项目</label>
          <select id="task-project" value={form.projectId} onChange={(e) => setForm((prev) => ({ ...prev, projectId: Number(e.target.value) }))} required>
            {projects.map((project) => <option key={project.id} value={project.id}>{project.code} - {project.name}</option>)}
          </select>
          <label htmlFor="task-parent">父任务ID（可选）</label>
          <input id="task-parent" type="number" value={form.parentId ?? ''} onChange={(e) => setForm((prev) => ({ ...prev, parentId: e.target.value ? Number(e.target.value) : undefined }))} />
          <label htmlFor="task-status">状态</label>
          <select id="task-status" value={form.status} onChange={(e) => setForm((prev) => ({ ...prev, status: e.target.value }))}>
            {Object.keys(statusLabel).map((key) => <option key={key} value={key}>{statusLabel[key]}</option>)}
          </select>
          <label htmlFor="task-progress">进度</label>
          <input id="task-progress" type="number" min={0} max={100} value={form.progress} onChange={(e) => setForm((prev) => ({ ...prev, progress: Number(e.target.value) }))} />
          <label htmlFor="task-start">开始时间</label>
          <input id="task-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} />
          <label htmlFor="task-end">结束时间</label>
          <input id="task-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} />
          <label htmlFor="task-assignees">执行人（多人）</label>
          <input
            aria-label="搜索执行人"
            placeholder="搜索执行人：姓名/用户名/邮箱"
            value={assigneeKeyword}
            onChange={(e) => setAssigneeKeyword(e.target.value)}
          />
          <div id="task-assignees" className="multi-checklist">
            {users.map((user) => (
              <label key={user.id} className="multi-check-item">
                <input
                  type="checkbox"
                  checked={form.assigneeIds.includes(user.id)}
                  onChange={() => setForm((prev) => ({ ...prev, assigneeIds: toggleNumber(prev.assigneeIds, user.id) }))}
                />
                <span>{user.name} ({user.username})</span>
              </label>
            ))}
          </div>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存任务'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
