import { FormEvent, ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { Settings2 } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { FieldSettingItem, FieldSettingsModal } from '../components/FieldSettingsModal'
import { SearchField } from '../components/SearchField'
import { SearchableMultiSelect } from '../components/SearchableMultiSelect'
import { SearchableSelect } from '../components/SearchableSelect'
import { TableHeaderFilter } from '../components/TableHeaderFilter'
import { RemoteProjectSelect } from '../components/RemoteProjectSelect'
import { formatDateTime } from '../utils/datetime'
import { Project, Tag, Task, TaskPriority, UploadAttachment, User, emptyUploadAttachments } from '../types'
import { AttachmentField } from '../components/AttachmentField'
import { usePermissions } from '../hooks/usePermissions'

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
  customField1: string
  customField2: string
  customField3: string
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
  customField1: '',
  customField2: '',
  customField3: '',
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

type TaskSortKey = 'createdAt' | 'progress'
type TaskSortOrder = 'asc' | 'desc'
type TaskColumnKey = 'taskNo' | 'title' | 'projectName' | 'priority' | 'status' | 'progress' | 'tags' | 'startAt' | 'endAt' | 'assignees' | 'description' | 'customField1' | 'customField2' | 'customField3'
interface TaskFieldSetting extends FieldSettingItem {
  key: TaskColumnKey
}
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]
const noParentLabel = '不关联父任务'
const taskFieldSettingsStorageKey = 'tasks_field_settings'
const taskDefaultFieldSettings: TaskFieldSetting[] = [
  { key: 'taskNo', label: '任务编号', visible: true, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'title', label: '标题', visible: true, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'projectName', label: '项目名称', visible: true, editable: true, sortable: false, searchable: true, filterable: true, custom: false },
  { key: 'priority', label: '优先级', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'status', label: '状态', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'progress', label: '进度', visible: true, editable: true, sortable: true, searchable: false, filterable: false, custom: false },
  { key: 'tags', label: '标签', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'startAt', label: '开始', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'endAt', label: '结束', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'assignees', label: '执行人', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'description', label: '描述', visible: false, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'customField1', label: '自定义内容 1', visible: false, editable: true, sortable: false, searchable: true, filterable: false, custom: true },
  { key: 'customField2', label: '自定义内容 2', visible: false, editable: true, sortable: false, searchable: true, filterable: false, custom: true },
  { key: 'customField3', label: '自定义内容 3', visible: false, editable: true, sortable: false, searchable: true, filterable: false, custom: true }
]

const getTaskProjectName = (task: Task, projects: Project[]) => {
  if (task.projectName) return task.projectName
  return projects.find((project) => project.id === task.projectId)?.name || '-'
}

const getTaskAssigneeNames = (task: Task) => (task.assignees || []).map((user) => user.username || user.name).filter(Boolean)

const normalizeTaskFieldSettings = (raw: unknown): TaskFieldSetting[] => {
  const fallbackMap = new Map(taskDefaultFieldSettings.map((field) => [field.key, field]))
  if (!Array.isArray(raw)) return taskDefaultFieldSettings

  const parsed = raw
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const key = String((item as { key?: string }).key || '') as TaskColumnKey
      const base = fallbackMap.get(key)
      if (!base) return null
      const normalized = {
        ...base,
        ...item,
        key: base.key,
        label: base.label
      } as TaskFieldSetting
      if (base.key === 'projectName') normalized.editable = true
      return normalized
    })
    .filter(Boolean) as TaskFieldSetting[]

  const seen = new Set(parsed.map((item) => item.key))
  const missing = taskDefaultFieldSettings.filter((item) => !seen.has(item.key))
  return [...parsed, ...missing]
}

export function TasksPage() {
  const permissions = usePermissions()
  const canCreateTask = hasPermission('tasks.create', permissions)
  const canUpdateTask = hasPermission('tasks.update', permissions)
  const canDeleteTask = hasPermission('tasks.delete', permissions)
  const canCreateTag = hasPermission('tags.create', permissions)
  const canUploadAttachment = hasPermission('uploads.create', permissions)
  const [searchParams, setSearchParams] = useSearchParams()
  const [tasks, setTasks] = useState<Task[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [filterUsers, setFilterUsers] = useState<User[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [parentTasks, setParentTasks] = useState<Task[]>([])
  const [keyword, setKeyword] = useState('')
  const [projectFilter, setProjectFilter] = useState('')
  const [assigneeFilters, setAssigneeFilters] = useState<string[]>([])
  const [statusFilters, setStatusFilters] = useState<string[]>([])
  const [priorityFilters, setPriorityFilters] = useState<string[]>([])
  const [fieldSettingsOpen, setFieldSettingsOpen] = useState(false)
  const [fieldSettings, setFieldSettings] = useState<TaskFieldSetting[]>(() => {
    try {
      return normalizeTaskFieldSettings(JSON.parse(localStorage.getItem(taskFieldSettingsStorageKey) || '[]'))
    } catch {
      return taskDefaultFieldSettings
    }
  })
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
  const fieldSettingsMap = useMemo(() => new Map(fieldSettings.map((field) => [field.key, field])), [fieldSettings])
  const visibleColumns = useMemo(() => fieldSettings.filter((field) => field.visible).map((field) => field.key), [fieldSettings])
  const searchableFields = useMemo(() => fieldSettings.filter((field) => field.searchable).map((field) => field.key), [fieldSettings])
  const isProjectFilterEnabled = fieldSettingsMap.get('projectName')?.filterable ?? true
  const isAssigneeFilterEnabled = fieldSettingsMap.get('assignees')?.filterable ?? true
  const isStatusFilterEnabled = fieldSettingsMap.get('status')?.filterable ?? true
  const isPriorityFilterEnabled = fieldSettingsMap.get('priority')?.filterable ?? true
  const isProgressSortable = fieldSettingsMap.get('progress')?.sortable ?? true
  const isTaskFieldEditable = (key: TaskColumnKey) => fieldSettingsMap.get(key)?.editable ?? true
  const activeFilterCount =
    Number(Boolean(keyword.trim()) && searchableFields.length > 0) +
    Number(Boolean(projectFilter) && isProjectFilterEnabled) +
    Number(assigneeFilters.length > 0 && isAssigneeFilterEnabled) +
    Number(statusFilters.length > 0 && isStatusFilterEnabled) +
    Number(priorityFilters.length > 0 && isPriorityFilterEnabled) +
    Number(sortKey === 'progress' && isProgressSortable)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [taskPage, projectPage] = await Promise.all([
        fetchPage<Task>(
          '/tasks',
          {
            page,
            pageSize,
            keyword: searchableFields.length > 0 ? keyword : '',
            projectId: isProjectFilterEnabled ? projectFilter : '',
            assigneeIds: isAssigneeFilterEnabled ? assigneeFilters.join(',') : '',
            statuses: isStatusFilterEnabled ? statusFilters.join(',') : '',
            priorities: isPriorityFilterEnabled ? priorityFilters.join(',') : '',
            searchFields: searchableFields.join(','),
            sortBy: sortKey,
            sortOrder
          },
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
  }, [page, pageSize, keyword, projectFilter, assigneeFilters, statusFilters, priorityFilters, sortKey, sortOrder, searchableFields, isProjectFilterEnabled, isAssigneeFilterEnabled, isStatusFilterEnabled, isPriorityFilterEnabled])

  useEffect(() => {
    if (sortKey !== 'progress') return
    if (!isProgressSortable) {
      setSortKey('createdAt')
      setSortOrder('desc')
    }
  }, [sortKey, isProgressSortable])

  useEffect(() => {
    void fetchData<{ users?: User[] }>('/tasks/assignee-options', { pageSize: 100 }, { silent: true })
      .then((data) => setFilterUsers(data?.users || []))
      .catch(() => setFilterUsers([]))
  }, [])

  useEffect(() => {
    localStorage.setItem(taskFieldSettingsStorageKey, JSON.stringify(fieldSettings))
  }, [fieldSettings])

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
    if (!canCreateTask) return
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
    if (form.id && !canUpdateTask) return
    if (!form.id && !canCreateTask) return
    if (!form.projectId) {
      setFormError('请选择项目')
      return
    }
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
    if (!canUpdateTask) return
    setForm({
      id: item.id,
      taskNo: item.taskNo,
      title: item.title,
      description: item.description,
      customField1: item.customField1 || '',
      customField2: item.customField2 || '',
      customField3: item.customField3 || '',
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
    const canMutateCurrentTask = form.id ? canUpdateTask : canCreateTask
    if (!canCreateTag || !canMutateCurrentTask) return
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
    if (!canDeleteTask) return
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

  const assigneeFilterOptions = useMemo(() => (
    filterUsers.map((user) => ({
      value: String(user.id),
      label: `${user.username}${user.name ? ` (${user.name})` : ''}`,
      keywords: [user.username, user.name, user.email].filter(Boolean)
    }))
  ), [filterUsers])

  const statusFilterOptions = useMemo(() => (
    Object.entries(statusLabel).map(([value, label]) => ({ value, label }))
  ), [])

  const priorityFilterOptions = useMemo(() => (
    Object.entries(priorityLabel).map(([value, label]) => ({ value, label }))
  ), [])

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

  const toggleProgressSort = () => {
    if (!isProgressSortable) return
    setPage(1)
    if (sortKey !== 'progress') {
      setSortKey('progress')
      setSortOrder('asc')
      return
    }
    if (sortOrder === 'asc') {
      setSortOrder('desc')
      return
    }
    setSortKey('createdAt')
    setSortOrder('desc')
  }

  const renderTaskHeaderCell = (key: TaskColumnKey) => {
    const setting = fieldSettingsMap.get(key)
    if (!setting) return null

    let content: ReactNode = setting.label
    if (key === 'priority' && isPriorityFilterEnabled) {
      content = <TableHeaderFilter label="优先级" values={priorityFilters} options={priorityFilterOptions} onChange={(values) => { setPriorityFilters(values); setPage(1) }} placeholder="搜索优先级" noResultsText="没有匹配的优先级" />
    } else if (key === 'status' && isStatusFilterEnabled) {
      content = <TableHeaderFilter label="状态" values={statusFilters} options={statusFilterOptions} onChange={(values) => { setStatusFilters(values); setPage(1) }} placeholder="搜索状态" noResultsText="没有匹配的状态" />
    } else if (key === 'progress' && isProgressSortable) {
      content = (
        <button type="button" className={`table-header-sort-trigger${sortKey === 'progress' ? ' active' : ''}`} onClick={toggleProgressSort}>
          进度
          <span className="table-header-sort-indicator">{sortKey === 'progress' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕'}</span>
        </button>
      )
    }

    return <th key={key}>{content}</th>
  }

  const renderTaskCell = (task: Task, key: TaskColumnKey, assigneeNames: string[], isExpanded: boolean, visibleAssigneeNames: string[]) => {
    switch (key) {
      case 'taskNo':
        return <td key={key} data-label="任务编号">{task.taskNo}</td>
      case 'title':
        return <td key={key} data-label="标题">{task.title}</td>
      case 'projectName':
        return <td key={key} data-label="项目名称">{getTaskProjectName(task, projects)}</td>
      case 'priority':
        return <td key={key} data-label="优先级">{priorityLabel[(task.priority || 'high') as TaskPriority]}</td>
      case 'status':
        return <td key={key} data-label="状态">{statusLabel[task.status]}</td>
      case 'progress':
        return <td key={key} data-label="进度">{task.progress}%</td>
      case 'tags':
        return (
          <td key={key} data-label="标签">
            <div className="task-tag-stack">
              {(task.tags || []).length > 0 ? (task.tags || []).map((tag) => (
                <span key={tag.id} className="task-tag-badge">{tag.name}</span>
              )) : <span>-</span>}
            </div>
          </td>
        )
      case 'startAt':
        return <td key={key} data-label="开始">{formatDateTime(task.startAt)}</td>
      case 'endAt':
        return <td key={key} data-label="结束">{formatDateTime(task.endAt)}</td>
      case 'assignees':
        return (
          <td key={key} data-label="执行人">
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
        )
      case 'description':
        return <td key={key} data-label="描述">{task.description || '-'}</td>
      case 'customField1':
        return <td key={key} data-label="自定义内容 1">{task.customField1 || '-'}</td>
      case 'customField2':
        return <td key={key} data-label="自定义内容 2">{task.customField2 || '-'}</td>
      case 'customField3':
        return <td key={key} data-label="自定义内容 3">{task.customField3 || '-'}</td>
      default:
        return null
    }
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
        actions={canCreateTask ? <button className="btn" onClick={openCreateModal}>新增任务</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        {searchableFields.length > 0 && <SearchField className="toolbar-search-field" aria-label="搜索任务" value={keyword} placeholder="搜索：已启用可搜索字段" onChange={(value) => { setKeyword(value); setPage(1) }} />}
        {isProjectFilterEnabled && (
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
        )}
        {isAssigneeFilterEnabled && (
          <SearchableMultiSelect
            ariaLabel="任务人员筛选"
            values={assigneeFilters}
            options={assigneeFilterOptions}
            onChange={(values) => {
              setAssigneeFilters(values)
              setPage(1)
            }}
            placeholder="搜索人员：用户名/姓名/邮箱"
            noResultsText="没有匹配的人员"
            summaryNoun="人员"
          />
        )}
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && tasks.length === 0} emptyText="暂无匹配的任务" onRetry={() => { void load() }} />
        {!loading && !error && tasks.length > 0 && (
          <table className="responsive-table"><thead><tr>
            {visibleColumns.map((columnKey) => renderTaskHeaderCell(columnKey))}
            <th className="field-settings-header-cell">
              <span className="field-settings-header-inline">
                <span>操作</span>
                <button type="button" className="field-settings-icon-btn" aria-label="任务列表字段设置" onClick={() => setFieldSettingsOpen(true)}>
                  <Settings2 size={16} />
                </button>
              </span>
            </th>
          </tr></thead><tbody>
            {tasks.map((task) => {
              const assigneeNames = getTaskAssigneeNames(task)
              const isExpanded = expandedAssigneeTaskIds.includes(task.id)
              const visibleAssigneeNames = isExpanded ? assigneeNames : assigneeNames.slice(0, 3)

              return (
                <tr key={task.id} id={`task-row-${task.id}`} className={focusedTaskId === task.id ? 'task-row-focused' : ''}>
                  {visibleColumns.map((columnKey) => renderTaskCell(task, columnKey, assigneeNames, isExpanded, visibleAssigneeNames))}
                  <td data-label="操作">
                    <div className="table-actions">
                      <button className="btn secondary" onClick={() => viewDetail(task)}>查看详情</button>
                      {canUpdateTask && <button className="btn secondary" onClick={() => edit(task)}>编辑</button>}
                      {canDeleteTask && <button className="btn danger" onClick={() => onDelete(task.id)}>删除</button>}
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <FieldSettingsModal
        open={fieldSettingsOpen}
        title="任务列表字段设置"
        fields={fieldSettings}
        defaultFields={taskDefaultFieldSettings}
        onClose={() => setFieldSettingsOpen(false)}
        onSave={(fields) => {
          setFieldSettings(fields)
          setFieldSettingsOpen(false)
        }}
      />

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
              <div className="detail-description-card">
                <strong>自定义内容 1</strong>
                <p>{detailTask.customField1 || '-'}</p>
              </div>
              <div className="detail-description-card">
                <strong>自定义内容 2</strong>
                <p>{detailTask.customField2 || '-'}</p>
              </div>
              <div className="detail-description-card">
                <strong>自定义内容 3</strong>
                <p>{detailTask.customField3 || '-'}</p>
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
          <input id="task-no" value={form.taskNo} onChange={(e) => setForm((prev) => ({ ...prev, taskNo: e.target.value }))} disabled={!isTaskFieldEditable('taskNo')} />
          <label className="required-label" htmlFor="task-title">标题</label>
          <input id="task-title" value={form.title} onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))} required disabled={!isTaskFieldEditable('title')} />
          <label htmlFor="task-description">描述</label>
          <textarea id="task-description" rows={4} value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} disabled={!isTaskFieldEditable('description')} />
          <label htmlFor="task-attachment">附件</label>
          <AttachmentField inputId="task-attachment" value={form.attachments} disabled={!canUploadAttachment} onChange={(attachments) => setForm((prev) => ({ ...prev, attachments }))} />
          <label className="required-label">项目</label>
          <RemoteProjectSelect
            ariaLabel="任务所属项目"
            value={form.projectId ? String(form.projectId) : ''}
            defaultOptionLabel="请选择项目"
            placeholder="搜索项目：编码/名称/描述"
            noResultsText="没有匹配的项目"
            disabled={!isTaskFieldEditable('projectName')}
            onChange={(value) => {
              const projectId = Number(value)
              setForm((prev) => ({
                ...prev,
                projectId: Number.isFinite(projectId) && projectId > 0 ? projectId : 0,
                parentId: undefined
              }))
              setParentTaskInput(noParentLabel)
            }}
          />
          <label htmlFor="task-parent">父任务（同项目，可选）</label>
          <div className="combo-wrap" ref={parentTaskWrapRef}>
            <input
              id="task-parent"
              aria-label="搜索父任务"
              placeholder="搜索父任务：任务编号/标题/描述"
              value={parentTaskInput}
              disabled={!isTaskFieldEditable('title')}
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
          <select id="task-status" value={form.status} onChange={(e) => setForm((prev) => ({ ...prev, status: e.target.value }))} disabled={!isTaskFieldEditable('status')}>
            {Object.keys(statusLabel).map((key) => <option key={key} value={key}>{statusLabel[key]}</option>)}
          </select>
          <label htmlFor="task-priority">优先级</label>
          <select id="task-priority" value={form.priority} onChange={(e) => setForm((prev) => ({ ...prev, priority: e.target.value as TaskPriority }))} disabled={!isTaskFieldEditable('priority')}>
            <option value="high">高</option>
            <option value="medium">中</option>
            <option value="low">低</option>
          </select>
          <label htmlFor="task-progress">进度</label>
          <input id="task-progress" type="number" inputMode="numeric" min={0} max={100} value={form.progress} onChange={(e) => setForm((prev) => ({ ...prev, progress: Number(e.target.value) }))} disabled={!isTaskFieldEditable('progress')} />
          <label htmlFor="task-assignees">执行人（多人）</label>
          <SearchField aria-label="搜索执行人" placeholder="搜索执行人：姓名/用户名/邮箱" value={assigneeKeyword} onChange={setAssigneeKeyword} disabled={!isTaskFieldEditable('assignees')} />
          <div id="task-assignees" className="multi-checklist">
            {users.map((user) => (
              <label key={user.id} className="multi-check-item">
                <input type="checkbox" checked={form.assigneeIds.includes(user.id)} onChange={() => setForm((prev) => ({ ...prev, assigneeIds: toggleNumber(prev.assigneeIds, user.id) }))} disabled={!isTaskFieldEditable('assignees')} />
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
            disabled={!isTaskFieldEditable('tags')}
            onKeyDown={(event) => {
              const canMutateCurrentTask = form.id ? canUpdateTask : canCreateTask
              if (event.key === 'Enter' && canCreateTagFromKeyword && canCreateTag && canMutateCurrentTask && isTaskFieldEditable('tags')) {
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
                disabled={!isTaskFieldEditable('tags')}
              />
                <span>全选搜索到的标签（{tags.length}）</span>
              </label>
            )}
            {canCreateTagFromKeyword && canCreateTag && (form.id ? canUpdateTask : canCreateTask) && (
              <button
                type="button"
                className="multi-check-action"
                onClick={() => { void createTagInline(tagKeyword) }}
                disabled={creatingTag || !isTaskFieldEditable('tags')}
              >
                {creatingTag ? '新增中...' : `新增标签「${tagKeyword.trim()}」`}
              </button>
            )}
            {tags.map((tag) => (
              <label key={tag.id} className="multi-check-item">
                <input type="checkbox" checked={form.tagIds.includes(tag.id)} onChange={() => setForm((prev) => ({ ...prev, tagIds: toggleNumber(prev.tagIds, tag.id) }))} disabled={!isTaskFieldEditable('tags')} />
                <span>{tag.name}</span>
              </label>
            ))}
            {!canCreateTagFromKeyword && tags.length === 0 && <p className="inline-tip">暂无匹配标签，请输入名称后直接新增。</p>}
          </div>
          <label htmlFor="task-start">开始时间</label>
          <input id="task-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} disabled={!isTaskFieldEditable('startAt')} />
          <label htmlFor="task-end">结束时间</label>
          <input id="task-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} disabled={!isTaskFieldEditable('endAt')} />
          <label htmlFor="task-milestone">里程碑</label>
          <select id="task-milestone" value={form.isMilestone ? '1' : '0'} onChange={(e) => setForm((prev) => ({ ...prev, isMilestone: e.target.value === '1' }))}>
            <option value="0">否</option>
            <option value="1">是</option>
          </select>
          <label htmlFor="task-custom-field-1">自定义内容 1</label>
          <textarea id="task-custom-field-1" rows={4} value={form.customField1} onChange={(e) => setForm((prev) => ({ ...prev, customField1: e.target.value }))} disabled={!isTaskFieldEditable('customField1')} />
          <label htmlFor="task-custom-field-2">自定义内容 2</label>
          <textarea id="task-custom-field-2" rows={4} value={form.customField2} onChange={(e) => setForm((prev) => ({ ...prev, customField2: e.target.value }))} disabled={!isTaskFieldEditable('customField2')} />
          <label htmlFor="task-custom-field-3">自定义内容 3</label>
          <textarea id="task-custom-field-3" rows={4} value={form.customField3} onChange={(e) => setForm((prev) => ({ ...prev, customField3: e.target.value }))} disabled={!isTaskFieldEditable('customField3')} />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateTask : !canCreateTask)}>{submitting ? '保存中...' : '保存任务'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
