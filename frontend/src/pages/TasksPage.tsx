import { FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, fetchPage, readApiError } from '../services/api'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { SearchField } from '../components/SearchField'
import { SearchableSelect } from '../components/SearchableSelect'
import { formatDateTime } from '../utils/datetime'
import { Project, Tag, Task, TaskPriority, UploadAttachment, User, emptyUploadAttachments } from '../types'
import { AttachmentField } from '../components/AttachmentField'

const statusLabel: Record<string, string> = {
  pending: '待处理',
  queued: '排队中',
  processing: '处理中',
  completed: '已完成'
}

const priorityLabel: Record<TaskPriority, string> = {
  high: '高',
  medium: '中',
  low: '低'
}

interface TaskForm {
  id?: number
  taskNo: string
  title: string
  description: string
  status: string
  priority: TaskPriority
  isMilestone: boolean
  progress: number
  startAt: string
  endAt: string
  attachments: UploadAttachment[]
  projectId: number
  parentId?: number
  assigneeIds: number[]
  tagIds: number[]
}

const normalizeAttachments = (item: { attachments?: UploadAttachment[]; attachment?: UploadAttachment }) => {
  if (Array.isArray(item.attachments)) return item.attachments
  if (item.attachment?.filePath) return [item.attachment]
  return emptyUploadAttachments()
}

const initialForm: TaskForm = {
  taskNo: '',
  title: '',
  description: '',
  status: 'pending',
  priority: 'high',
  isMilestone: false,
  progress: 0,
  startAt: '',
  endAt: '',
  attachments: emptyUploadAttachments(),
  projectId: 0,
  assigneeIds: [],
  tagIds: []
}

type TaskSortKey = 'createdAt' | 'progress' | 'status' | 'priority'
type TaskSortOrder = 'asc' | 'desc'
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]
const noParentLabel = '不关联父任务'

const getTaskProjectName = (task: Task, projects: Project[]) => {
  if (task.projectName) return task.projectName
  return projects.find((project) => project.id === task.projectId)?.name || '-'
}

const getTaskAssigneeNames = (task: Task) => (task.assignees || []).map((user) => user.username || user.name).filter(Boolean)

export function TasksPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [tasks, setTasks] = useState<Task[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [parentTasks, setParentTasks] = useState<Task[]>([])
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [projectFilter, setProjectFilter] = useState('')
  const [sortKey, setSortKey] = useState<TaskSortKey>('createdAt')
  const [sortOrder, setSortOrder] = useState<TaskSortOrder>('desc')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [form, setForm] = useState<TaskForm>(initialForm)
  const [modalOpen, setModalOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailTask, setDetailTask] = useState<Task | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [assigneeKeyword, setAssigneeKeyword] = useState('')
  const [tagKeyword, setTagKeyword] = useState('')
  const [creatingTag, setCreatingTag] = useState(false)
  const [parentTaskInput, setParentTaskInput] = useState(noParentLabel)
  const [parentDropdownOpen, setParentDropdownOpen] = useState(false)
  const [parentHighlightIndex, setParentHighlightIndex] = useState(0)
  const [focusedTaskId, setFocusedTaskId] = useState<number | null>(null)
  const [pendingOpenTaskId, setPendingOpenTaskId] = useState<number | null>(null)
  const [pendingViewTaskId, setPendingViewTaskId] = useState<number | null>(null)
  const [expandedAssigneeTaskIds, setExpandedAssigneeTaskIds] = useState<number[]>([])
  const parentTaskWrapRef = useRef<HTMLDivElement | null>(null)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(status)) + Number(Boolean(projectFilter)) + Number(sortKey !== 'createdAt') + Number(sortOrder !== 'desc')

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [taskPage, projectPage] = await Promise.all([
        fetchPage<Task>(
          '/tasks',
          { page, pageSize, keyword, status, projectId: projectFilter, sortBy: sortKey, sortOrder },
          { page, pageSize }
        ),
        fetchPage<Project>('/projects', { page: 1, pageSize: 200 }, { page: 1, pageSize: 200 })
      ])
      setTasks(taskPage.list)
      setTotal(taskPage.total)
      const projectList = projectPage.list
      setProjects(projectList)
      if (!form.projectId && projectList.length > 0) {
        setForm((prev) => ({ ...prev, projectId: prev.projectId || projectList[0].id }))
      }
    } catch (loadError) {
      setError(readApiError(loadError, '任务列表加载失败'))
      setTasks([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [page, pageSize, keyword, status, projectFilter, sortKey, sortOrder])

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
      void Promise.all([
        fetchPage<User>('/users', { page: 1, pageSize: 200, keyword: assigneeKeyword.trim() }, { page: 1, pageSize: 200 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 200 })),
        fetchPage<Tag>('/tags', { page: 1, pageSize: 200, keyword: tagKeyword.trim() }, { page: 1, pageSize: 200 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 200 }))
      ]).then(([userPage, tagPage]) => {
        setUsers(userPage.list)
        setTags(tagPage.list)
      })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [modalOpen, assigneeKeyword, tagKeyword])

  useEffect(() => {
    if (!modalOpen || !form.projectId) return
    void fetchPage<Task>('/tasks', { page: 1, pageSize: 500, projectId: form.projectId }, { page: 1, pageSize: 500 }, { silent: true })
      .then((res) => setParentTasks(res.list))
      .catch(() => setParentTasks([]))
  }, [modalOpen, form.projectId])

  const openCreateModal = () => {
    setForm((prev) => ({ ...initialForm, projectId: prev.projectId || projects[0]?.id || 0 }))
    setFormError('')
    setFormSuccess('')
    setAssigneeKeyword('')
    setTagKeyword('')
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
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存任务失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const edit = (item: Task) => {
    setForm({
      id: item.id,
      taskNo: item.taskNo,
      title: item.title,
      description: item.description,
      status: item.status,
      priority: item.priority || 'high',
      progress: item.progress,
      isMilestone: Boolean(item.isMilestone),
      startAt: item.startAt ? item.startAt.slice(0, 16) : '',
      endAt: item.endAt ? item.endAt.slice(0, 16) : '',
      attachments: normalizeAttachments(item),
      projectId: item.projectId,
      parentId: item.parentId,
      assigneeIds: (item.assignees || []).map((user) => user.id),
      tagIds: (item.tags || []).map((tag) => tag.id)
    })
    setFormError('')
    setFormSuccess('')
    setAssigneeKeyword('')
    setTagKeyword('')
    const parent = parentTasks.find((task) => task.id === item.parentId) || tasks.find((task) => task.id === item.parentId)
    setParentTaskInput(parent ? `${parent.taskNo}｜${parent.title}` : noParentLabel)
    setModalOpen(true)
  }

  const createTagInline = async (rawName?: string) => {
    const name = (rawName ?? tagKeyword).trim()
    if (!name) {
      setFormError('请输入标签名称')
      return
    }
    try {
      setCreatingTag(true)
      setFormError('')
      const response = await api.post<Tag>('/tags', { name })
      const created = response.data
      setTags((prev) => [created, ...prev.filter((item) => item.id !== created.id)])
      setForm((prev) => ({ ...prev, tagIds: prev.tagIds.includes(created.id) ? prev.tagIds : [...prev.tagIds, created.id] }))
      setTagKeyword(created.name)
    } catch (error) {
      setFormError(readApiError(error, '新增标签失败'))
    } finally {
      setCreatingTag(false)
    }
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该任务？')) return
    try {
      await api.delete(`/tasks/${id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除任务失败'))
    }
  }

  const toggleAssigneeExpand = (taskId: number) => {
    setExpandedAssigneeTaskIds((prev) => prev.includes(taskId) ? prev.filter((id) => id !== taskId) : [...prev, taskId])
  }

  const viewDetail = (item: Task) => {
    setDetailTask(item)
    setDetailOpen(true)
  }

  const parentTaskOptions = useMemo(() => {
    return parentTasks.filter((task) => {
      if (task.projectId !== form.projectId) return false
      if (form.id && task.id === form.id) return false
      return true
    })
  }, [parentTasks, form.projectId, form.id])

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

  const projectFilterOptions = useMemo(() => (
    projects.map((project) => ({
      value: String(project.id),
      label: `${project.code} - ${project.name}`,
      keywords: [project.code, project.name, project.description || '']
    }))
  ), [projects])

  const normalizedTagKeyword = tagKeyword.trim().toLowerCase()
  const canCreateTagFromKeyword = Boolean(normalizedTagKeyword) && !tags.some((tag) => tag.name.trim().toLowerCase() === normalizedTagKeyword)
  const hasTagSearchKeyword = Boolean(tagKeyword.trim())
  const allSearchedTagsSelected = tags.length > 0 && tags.every((tag) => form.tagIds.includes(tag.id))

  const toggleSelectAllSearchedTags = (checked: boolean) => {
    const searchedTagIds = tags.map((tag) => tag.id)
    setForm((prev) => {
      if (checked) {
        return { ...prev, tagIds: Array.from(new Set([...prev.tagIds, ...searchedTagIds])) }
      }
      return { ...prev, tagIds: prev.tagIds.filter((id) => !searchedTagIds.includes(id)) }
    })
  }

  const selectNoParent = () => {
    setForm((prev) => ({ ...prev, parentId: undefined }))
    setParentTaskInput(noParentLabel)
    setParentDropdownOpen(false)
  }

  const selectParentTask = (task: Task) => {
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

  useEffect(() => {
    if (!focusedTaskId) return
    const timer = window.setTimeout(() => {
      const target = document.getElementById(`task-row-${focusedTaskId}`)
      target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }, 120)
    return () => window.clearTimeout(timer)
  }, [focusedTaskId, tasks])

  useEffect(() => {
    if (!pendingOpenTaskId) return
    const localTask = tasks.find((item) => item.id === pendingOpenTaskId)
    if (localTask) {
      edit(localTask)
      setPendingOpenTaskId(null)
      return
    }
    void fetchPage<Task>('/tasks', { page: 1, pageSize: 500 }, { page: 1, pageSize: 500 }, { silent: true }).then((res) => {
      const list = res.list
      const target = list.find((item: Task) => item.id === pendingOpenTaskId)
      if (target) edit(target)
      setPendingOpenTaskId(null)
    }).catch(() => setPendingOpenTaskId(null))
  }, [pendingOpenTaskId, tasks])

  useEffect(() => {
    if (!pendingViewTaskId) return
    const localTask = tasks.find((item) => item.id === pendingViewTaskId)
    if (localTask) {
      viewDetail(localTask)
      setPendingViewTaskId(null)
      return
    }
    void fetchPage<Task>('/tasks', { page: 1, pageSize: 500 }, { page: 1, pageSize: 500 }, { silent: true }).then((res) => {
      const list = res.list
      const target = list.find((item: Task) => item.id === pendingViewTaskId)
      if (target) viewDetail(target)
      setPendingViewTaskId(null)
    }).catch(() => setPendingViewTaskId(null))
  }, [pendingViewTaskId, tasks])

  return (
    <section className="page-section">
      <FilterPanel
        title="任务筛选"
        activeCount={activeFilterCount}
        actions={<button className="btn" onClick={openCreateModal}>新增任务</button>}
        bodyClassName="toolbar-grid"
      >
        <SearchField className="toolbar-search-field" aria-label="搜索任务" value={keyword} placeholder="搜索：编号/标题/描述" onChange={(value) => { setKeyword(value); setPage(1) }} />
        <select aria-label="任务状态筛选" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
          <option value="">全部状态</option>
          {Object.keys(statusLabel).map((key) => <option key={key} value={key}>{statusLabel[key]}</option>)}
        </select>
        <SearchableSelect
          ariaLabel="任务项目筛选"
          value={projectFilter}
          options={projectFilterOptions}
          defaultOptionLabel="全部项目"
          placeholder="搜索项目：编码/名称/描述"
          noResultsText="没有匹配的项目"
          onChange={(value) => {
            setProjectFilter(value)
            setPage(1)
          }}
        />
        <select aria-label="任务排序字段" value={sortKey} onChange={(e) => { setSortKey(e.target.value as TaskSortKey); setPage(1) }}>
          <option value="createdAt">按创建时间</option>
          <option value="progress">按进度</option>
          <option value="status">按状态</option>
          <option value="priority">按优先级</option>
        </select>
        <select aria-label="任务排序方式" value={sortOrder} onChange={(e) => { setSortOrder(e.target.value as TaskSortOrder); setPage(1) }}>
          <option value="desc">降序</option>
          <option value="asc">升序</option>
        </select>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && tasks.length === 0} emptyText="暂无匹配的任务" onRetry={() => { void load() }} />
        {!loading && !error && tasks.length > 0 && (
          <table className="responsive-table"><thead><tr><th>任务编号</th><th>标题</th><th>项目名称</th><th>优先级</th><th>状态</th><th>进度</th><th>标签</th><th>开始</th><th>结束</th><th>执行人</th><th>操作</th></tr></thead><tbody>
            {tasks.map((task) => {
              const assigneeNames = getTaskAssigneeNames(task)
              const isExpanded = expandedAssigneeTaskIds.includes(task.id)
              const visibleAssigneeNames = isExpanded ? assigneeNames : assigneeNames.slice(0, 3)

              return (
                <tr key={task.id} id={`task-row-${task.id}`} className={focusedTaskId === task.id ? 'task-row-focused' : ''}>
                  <td data-label="任务编号">{task.taskNo}</td>
                  <td data-label="标题">{task.title}</td>
                  <td data-label="项目名称">{getTaskProjectName(task, projects)}</td>
                  <td data-label="优先级">{priorityLabel[(task.priority || 'high') as TaskPriority]}</td>
                  <td data-label="状态">{statusLabel[task.status]}</td>
                  <td data-label="进度">{task.progress}%</td>
                  <td data-label="标签">
                    <div className="task-tag-stack">
                      {(task.tags || []).length > 0 ? (task.tags || []).map((tag) => (
                        <span key={tag.id} className="task-tag-badge">{tag.name}</span>
                      )) : <span>-</span>}
                    </div>
                  </td>
                  <td data-label="开始">{formatDateTime(task.startAt)}</td>
                  <td data-label="结束">{formatDateTime(task.endAt)}</td>
                  <td data-label="执行人">
                    {assigneeNames.length > 0 ? (
                      <div className="task-user-stack">
                        {visibleAssigneeNames.map((name) => (
                          <span key={name} className="task-user-line">{name}</span>
                        ))}
                        {assigneeNames.length > 3 && (
                          <button type="button" className="task-user-more" onClick={() => toggleAssigneeExpand(task.id)}>
                            {isExpanded ? '收起' : `显示更多（${assigneeNames.length - 3}）`}
                          </button>
                        )}
                      </div>
                    ) : '-'}
                  </td>
                  <td data-label="操作">
                    <div className="table-actions">
                      <button className="btn secondary" onClick={() => viewDetail(task)}>查看详情</button>
                      <button className="btn secondary" onClick={() => edit(task)}>编辑</button>
                      <button className="btn danger" onClick={() => onDelete(task.id)}>删除</button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

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
                <div><strong>优先级：</strong>{priorityLabel[(detailTask.priority || 'high') as TaskPriority]}</div>
                <div><strong>里程碑：</strong>{detailTask.isMilestone ? '是' : '否'}</div>
                <div><strong>状态：</strong>{statusLabel[detailTask.status] || detailTask.status || '-'}</div>
                <div><strong>进度：</strong>{Number(detailTask.progress || 0)}%</div>
                <div><strong>项目ID：</strong>{detailTask.projectId || '-'}</div>
                <div><strong>父任务ID：</strong>{detailTask.parentId || '-'}</div>
                <div className="detail-time-line"><strong>标签：</strong>{(detailTask.tags || []).map((tag) => tag.name).join('，') || '-'}</div>
              </div>
            </section>
            <section className="detail-section">
              <h4>人员与时间</h4>
              <div className="detail-columns">
                <div><strong>创建人ID：</strong>{detailTask.creatorId || '-'}</div>
                <div><strong>执行人：</strong>{(detailTask.assignees || []).map((u) => `${u.name}(${u.username})`).join('，') || '-'}</div>
                <div className="detail-time-line"><strong>任务周期：</strong>{formatDateTime(detailTask.startAt)} - {formatDateTime(detailTask.endAt)}</div>
                <div className="detail-time-line">
                  <strong>附件：</strong>
                  {normalizeAttachments(detailTask).length > 0 ? normalizeAttachments(detailTask).map((item) => (
                    <a key={item.filePath} href={item.filePath} target="_blank" rel="noreferrer">{item.relativePath || item.fileName || '附件'}</a>
                  )) : '-'}
                </div>
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
          <label htmlFor="task-attachment">附件</label>
          <AttachmentField inputId="task-attachment" value={form.attachments} onChange={(attachments) => setForm((prev) => ({ ...prev, attachments }))} />
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
          <label htmlFor="task-priority">优先级</label>
          <select id="task-priority" value={form.priority} onChange={(e) => setForm((prev) => ({ ...prev, priority: e.target.value as TaskPriority }))}>
            <option value="high">高</option>
            <option value="medium">中</option>
            <option value="low">低</option>
          </select>
          <label htmlFor="task-progress">进度</label>
          <input id="task-progress" type="number" inputMode="numeric" min={0} max={100} value={form.progress} onChange={(e) => setForm((prev) => ({ ...prev, progress: Number(e.target.value) }))} />
          <label htmlFor="task-assignees">执行人（多人）</label>
          <SearchField aria-label="搜索执行人" placeholder="搜索执行人：姓名/用户名/邮箱" value={assigneeKeyword} onChange={setAssigneeKeyword} />
          <div id="task-assignees" className="multi-checklist">
            {users.map((user) => (
              <label key={user.id} className="multi-check-item">
                <input type="checkbox" checked={form.assigneeIds.includes(user.id)} onChange={() => setForm((prev) => ({ ...prev, assigneeIds: toggleNumber(prev.assigneeIds, user.id) }))} />
                <span>{user.name} ({user.username})</span>
              </label>
            ))}
          </div>
          <label htmlFor="task-tags">标签（多选）</label>
          <SearchField
            id="task-tags"
            aria-label="搜索标签"
            placeholder="搜索标签名称；没有则可直接新增"
            value={tagKeyword}
            onChange={setTagKeyword}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && canCreateTagFromKeyword) {
                event.preventDefault()
                void createTagInline(tagKeyword)
              }
            }}
          />
          <div className="multi-checklist">
            {hasTagSearchKeyword && tags.length > 0 && (
              <label className="multi-check-item multi-check-item-action">
                <input
                  type="checkbox"
                  checked={allSearchedTagsSelected}
                  onChange={(event) => toggleSelectAllSearchedTags(event.target.checked)}
                />
                <span>全选搜索到的标签（{tags.length}）</span>
              </label>
            )}
            {canCreateTagFromKeyword && (
              <button
                type="button"
                className="multi-check-action"
                onClick={() => { void createTagInline(tagKeyword) }}
                disabled={creatingTag}
              >
                {creatingTag ? '新增中...' : `新增标签「${tagKeyword.trim()}」`}
              </button>
            )}
            {tags.map((tag) => (
              <label key={tag.id} className="multi-check-item">
                <input type="checkbox" checked={form.tagIds.includes(tag.id)} onChange={() => setForm((prev) => ({ ...prev, tagIds: toggleNumber(prev.tagIds, tag.id) }))} />
                <span>{tag.name}</span>
              </label>
            ))}
            {!canCreateTagFromKeyword && tags.length === 0 && <p className="inline-tip">暂无匹配标签，请输入名称后直接新增。</p>}
          </div>
          <label htmlFor="task-start">开始时间</label>
          <input id="task-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} />
          <label htmlFor="task-end">结束时间</label>
          <input id="task-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} />
          <label htmlFor="task-milestone">里程碑</label>
          <select id="task-milestone" value={form.isMilestone ? '1' : '0'} onChange={(e) => setForm((prev) => ({ ...prev, isMilestone: e.target.value === '1' }))}>
            <option value="0">否</option>
            <option value="1">是</option>
          </select>
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
