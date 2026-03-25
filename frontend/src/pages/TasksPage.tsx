import { FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../services/api'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { DataState } from '../components/DataState'
import { formatDateTime } from '../utils/datetime'

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
const noParentLabel = '不关联父任务'

export function TasksPage() {
  const [searchParams, setSearchParams] = useSearchParams()
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
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailTask, setDetailTask] = useState<any | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [assigneeKeyword, setAssigneeKeyword] = useState('')
  const [parentTaskInput, setParentTaskInput] = useState(noParentLabel)
  const [parentDropdownOpen, setParentDropdownOpen] = useState(false)
  const [parentHighlightIndex, setParentHighlightIndex] = useState(0)
  const [focusedTaskId, setFocusedTaskId] = useState<number | null>(null)
  const [pendingOpenTaskId, setPendingOpenTaskId] = useState<number | null>(null)
  const [pendingViewTaskId, setPendingViewTaskId] = useState<number | null>(null)
  const parentTaskWrapRef = useRef<HTMLDivElement | null>(null)

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
    const taskId = Number(searchParams.get('taskId') || 0)
    if (!Number.isFinite(taskId) || taskId <= 0) return
    setFocusedTaskId(taskId)
    if (searchParams.get('view') === '1') {
      setPendingViewTaskId(taskId)
    }
    if (searchParams.get('open') === '1') {
      setPendingOpenTaskId(taskId)
    }
    setSearchParams({}, { replace: true })
  }, [searchParams, setSearchParams])

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
    setParentTaskInput(noParentLabel)
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
    const parent = tasks.find((task) => task.id === item.parentId)
    setParentTaskInput(parent ? `${parent.taskNo}｜${parent.title}` : noParentLabel)
    setModalOpen(true)
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该任务？')) return
    await api.delete(`/tasks/${id}`)
    await load()
  }

  const viewDetail = (item: any) => {
    setDetailTask(item)
    setDetailOpen(true)
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

  const parentTaskOptions = useMemo(() => {
    return tasks.filter((task) => {
      if (task.projectId !== form.projectId) return false
      if (form.id && task.id === form.id) return false
      return true
    })
  }, [tasks, form.projectId, form.id])

  const parentTaskLabelToId = useMemo(() => {
    const map = new Map<string, number>()
    parentTaskOptions.forEach((task) => {
      map.set(`${task.taskNo}｜${task.title}`, task.id)
      map.set(task.taskNo, task.id)
    })
    return map
  }, [parentTaskOptions])

  const filteredParentTaskOptions = useMemo(() => {
    const raw = parentTaskInput.trim()
    const query = raw === noParentLabel ? '' : raw.toLowerCase()
    if (!query) return parentTaskOptions
    return parentTaskOptions.filter((task) =>
      [task.taskNo, task.title, task.description].some((value) => String(value || '').toLowerCase().includes(query))
    )
  }, [parentTaskOptions, parentTaskInput])

  const selectNoParent = () => {
    setForm((prev) => ({ ...prev, parentId: undefined }))
    setParentTaskInput(noParentLabel)
    setParentDropdownOpen(false)
  }

  const selectParentTask = (task: any) => {
    const label = `${task.taskNo}｜${task.title}`
    setForm((prev) => ({ ...prev, parentId: task.id }))
    setParentTaskInput(label)
    setParentDropdownOpen(false)
  }

  useEffect(() => {
    if (!parentDropdownOpen) return
    const handleOutside = (event: MouseEvent) => {
      const target = event.target as Node
      if (!parentTaskWrapRef.current?.contains(target)) {
        setParentDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleOutside)
    return () => document.removeEventListener('mousedown', handleOutside)
  }, [parentDropdownOpen])

  useEffect(() => {
    if (!parentDropdownOpen) return
    setParentHighlightIndex(0)
  }, [parentDropdownOpen, parentTaskInput, form.projectId])

  const total = processedTasks.length
  const totalPages = Math.max(Math.ceil(total / pageSize), 1)
  const currentPage = Math.min(page, totalPages)
  const pagedTasks = processedTasks.slice((currentPage - 1) * pageSize, currentPage * pageSize)

  useEffect(() => {
    if (!focusedTaskId) return
    const index = processedTasks.findIndex((task) => task.id === focusedTaskId)
    if (index < 0) return
    const targetPage = Math.floor(index / pageSize) + 1
    if (targetPage !== page) {
      setPage(targetPage)
      return
    }
    const timer = window.setTimeout(() => {
      const target = document.getElementById(`task-row-${focusedTaskId}`)
      target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }, 120)
    return () => window.clearTimeout(timer)
  }, [focusedTaskId, processedTasks, pageSize, page])

  useEffect(() => {
    if (!pendingOpenTaskId) return
    const task = tasks.find((item) => item.id === pendingOpenTaskId)
    if (!task) return
    edit(task)
    setPendingOpenTaskId(null)
  }, [pendingOpenTaskId, tasks])

  useEffect(() => {
    if (!pendingViewTaskId) return
    const task = tasks.find((item) => item.id === pendingViewTaskId)
    if (!task) return
    viewDetail(task)
    setPendingViewTaskId(null)
  }, [pendingViewTaskId, tasks])

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
          <table><thead><tr><th>任务编号</th><th>标题</th><th>描述</th><th>状态</th><th>进度</th><th>开始</th><th>结束</th><th>执行人</th><th>操作</th></tr></thead><tbody>
            {pagedTasks.map((task) => (
              <tr key={task.id} id={`task-row-${task.id}`} className={focusedTaskId === task.id ? 'task-row-focused' : ''}>
                <td>{task.taskNo}</td>
                <td>{task.title}</td>
                <td><span className="cell-ellipsis" title={task.description || '-'}>{task.description || '-'}</span></td>
                <td>{statusLabel[task.status]}</td>
                <td>{task.progress}%</td>
                <td>{formatDateTime(task.startAt)}</td>
                <td>{formatDateTime(task.endAt)}</td>
                <td>{(task.assignees || []).length}</td>
                <td>
                  <button className="btn secondary" onClick={() => viewDetail(task)}>查看详情</button>
                  <button className="btn secondary" onClick={() => edit(task)}>编辑</button>
                  <button className="btn danger" onClick={() => onDelete(task.id)}>删除</button>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={currentPage} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={detailOpen} title="任务详情" onClose={() => setDetailOpen(false)}>
        {detailTask && (
          <div className="detail-grid">
            <section className="detail-section">
              <h4>基础信息</h4>
              <div className="detail-columns">
                <div><strong>任务ID：</strong>{detailTask.id}</div>
                <div><strong>任务编号：</strong>{detailTask.taskNo || '-'}</div>
                <div><strong>标题：</strong>{detailTask.title || '-'}</div>
              </div>
              <div className="detail-description-card">
                <strong>描述</strong>
                <p>{detailTask.description || '-'}</p>
              </div>
            </section>
            <section className="detail-section">
              <h4>状态与进度</h4>
              <div className="detail-columns">
                <div><strong>状态：</strong>{statusLabel[detailTask.status] || detailTask.status || '-'}</div>
                <div><strong>进度：</strong>{Number(detailTask.progress || 0)}%</div>
                <div><strong>项目ID：</strong>{detailTask.projectId || '-'}</div>
                <div><strong>父任务ID：</strong>{detailTask.parentId || '-'}</div>
              </div>
            </section>
            <section className="detail-section">
              <h4>人员与时间</h4>
              <div className="detail-columns">
                <div><strong>创建人ID：</strong>{detailTask.creatorId || '-'}</div>
                <div><strong>执行人：</strong>{(detailTask.assignees || []).map((u: any) => `${u.name}(${u.username})`).join('，') || '-'}</div>
                <div className="detail-time-line"><strong>任务周期：</strong>{formatDateTime(detailTask.startAt)} - {formatDateTime(detailTask.endAt)}</div>
              </div>
            </section>
          </div>
        )}
      </Modal>

      <Modal open={modalOpen} title={form.id ? '编辑任务' : '新增任务'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label htmlFor="task-no">任务编号（可空自动生成）</label>
          <input id="task-no" value={form.taskNo} onChange={(e) => setForm((prev) => ({ ...prev, taskNo: e.target.value }))} />
          <label className="required-label" htmlFor="task-title">标题</label>
          <input id="task-title" value={form.title} onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))} required />
          <label htmlFor="task-description">描述</label>
          <textarea id="task-description" rows={4} value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
          <label className="required-label" htmlFor="task-project">项目</label>
          <select
            id="task-project"
            value={form.projectId}
            onChange={(e) => {
              setForm((prev) => ({ ...prev, projectId: Number(e.target.value), parentId: undefined }))
              setParentTaskInput(noParentLabel)
            }}
            required
          >
            {projects.map((project) => <option key={project.id} value={project.id}>{project.code} - {project.name}</option>)}
          </select>
          <label htmlFor="task-parent">父任务（同项目，可选）</label>
          <div className="combo-wrap" ref={parentTaskWrapRef}>
            <input
              id="task-parent"
              aria-label="搜索父任务"
              placeholder="搜索父任务：任务编号/标题/描述"
              value={parentTaskInput}
              onFocus={() => setParentDropdownOpen(true)}
              onKeyDown={(event) => {
                if (!parentDropdownOpen && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
                  setParentDropdownOpen(true)
                  event.preventDefault()
                  return
                }
                if (!parentDropdownOpen) return
                const maxIndex = filteredParentTaskOptions.length
                if (event.key === 'ArrowDown') {
                  event.preventDefault()
                  setParentHighlightIndex((prev) => Math.min(prev + 1, maxIndex))
                  return
                }
                if (event.key === 'ArrowUp') {
                  event.preventDefault()
                  setParentHighlightIndex((prev) => Math.max(prev - 1, 0))
                  return
                }
                if (event.key === 'Escape') {
                  setParentDropdownOpen(false)
                  return
                }
                if (event.key === 'Enter') {
                  event.preventDefault()
                  if (parentHighlightIndex === 0) {
                    selectNoParent()
                    return
                  }
                  const target = filteredParentTaskOptions[parentHighlightIndex - 1]
                  if (target) selectParentTask(target)
                }
              }}
              onChange={(e) => {
                const value = e.target.value
                setParentTaskInput(value)
                setParentDropdownOpen(true)
                setForm((prev) => ({ ...prev, parentId: value && value !== noParentLabel ? parentTaskLabelToId.get(value) : undefined }))
              }}
            />
            {parentDropdownOpen && (
              <div className="combo-menu">
                <button
                  type="button"
                  className={`combo-option${parentHighlightIndex === 0 ? ' active' : ''}`}
                  onMouseEnter={() => setParentHighlightIndex(0)}
                  onClick={selectNoParent}
                >
                    {noParentLabel}
                  </button>
                {filteredParentTaskOptions.map((task, index) => {
                  const label = `${task.taskNo}｜${task.title}`
                  return (
                    <button
                      type="button"
                      key={task.id}
                      className={`combo-option${form.parentId === task.id || parentHighlightIndex === index + 1 ? ' active' : ''}`}
                      onMouseEnter={() => setParentHighlightIndex(index + 1)}
                      onClick={() => selectParentTask(task)}
                    >
                      {label}
                    </button>
                  )
                })}
                {filteredParentTaskOptions.length === 0 && (
                  <div className="combo-empty">没有匹配的父任务</div>
                )}
              </div>
            )}
          </div>
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
