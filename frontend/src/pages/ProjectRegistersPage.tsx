import { FormEvent, ReactNode, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Settings2, Sparkles } from 'lucide-react'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { postAIStream } from '../services/aiStream'
import { DataState } from '../components/DataState'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { FieldSettingItem, FieldSettingsModal } from '../components/FieldSettingsModal'
import { FilterPanel } from '../components/FilterPanel'
import { ImageAttachmentField } from '../components/ImageAttachmentField'
import { ImagePreviewOverlay } from '../components/ImagePreviewOverlay'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { RemoteProjectSelect } from '../components/RemoteProjectSelect'
import { SearchField } from '../components/SearchField'
import { SearchableMultiSelect } from '../components/SearchableMultiSelect'
import { SearchableSelect } from '../components/SearchableSelect'
import { formatDateTime } from '../utils/datetime'
import { ProjectRegister, ProjectRegisterActivity, ProjectRegisterImage, ProjectRegisterProbability, ProjectRegisterSeverity, ProjectRegisterStatus, ProjectRegisterType, User, emptyUploadAttachments } from '../types'
import { usePermissions } from '../hooks/usePermissions'

interface RegisterForm {
  id?: number
  type: ProjectRegisterType
  projectId: string
  taskId: string
  title: string
  description: string
  status: ProjectRegisterStatus
  severity: ProjectRegisterSeverity
  probability: '' | ProjectRegisterProbability
  impact: '' | ProjectRegisterSeverity
  source: string
  responsePlan: string
  resolution: string
  decisionDetail: string
  background: string
  impactScope: string
  images: ProjectRegisterImage[]
  dueAt: string
  ownerId: string
  participantIds: string[]
}

const registerTypeLabel: Record<ProjectRegisterType, string> = {
  risk: '风险',
  issue: '问题',
  decision: '决策'
}

const registerStatusLabel: Record<ProjectRegisterStatus, string> = {
  open: '未关闭',
  in_progress: '处理中',
  resolved: '已解决',
  closed: '已关闭'
}

const registerSeverityLabel: Record<ProjectRegisterSeverity, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重'
}

const registerProbabilityLabel: Record<ProjectRegisterProbability, string> = {
  low: '低',
  medium: '中',
  high: '高'
}

const activityTypeLabel: Record<string, string> = {
  'register.created': '创建',
  'register.updated': '更新'
}

type RegisterColumnKey =
  | 'type' | 'title' | 'projectName' | 'status' | 'severity' | 'probability' | 'impact'
  | 'owner' | 'participants' | 'source' | 'description' | 'responsePlan' | 'resolution'
  | 'decisionDetail' | 'background' | 'impactScope' | 'dueAt' | 'lastActivityAt' | 'createdAt' | 'updatedAt'

interface RegisterFieldSetting extends FieldSettingItem {
  key: RegisterColumnKey
}

const registerFieldSettingsStorageKey = 'project_registers_field_settings'

// The register API searches title/description/source and filters by type/status/severity/project;
// there is no server-side sort, so no column is marked sortable.
const registerDefaultFieldSettings: RegisterFieldSetting[] = [
  { key: 'type', label: '类型', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'title', label: '标题', visible: true, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'description', label: '说明', visible: true, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'projectName', label: '项目', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'status', label: '状态', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'severity', label: '等级', visible: true, editable: true, sortable: false, searchable: false, filterable: true, custom: false },
  { key: 'probability', label: '概率', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'impact', label: '影响', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'owner', label: '负责人', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'participants', label: '参与人', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'source', label: '来源', visible: false, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'responsePlan', label: '应对策略', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: true },
  { key: 'resolution', label: '解决方案', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: true },
  { key: 'decisionDetail', label: '决策内容', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: true },
  { key: 'background', label: '背景', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: true },
  { key: 'impactScope', label: '影响范围', visible: false, editable: true, sortable: false, searchable: false, filterable: false, custom: true },
  { key: 'dueAt', label: '截止时间', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'lastActivityAt', label: '最近动态', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'createdAt', label: '创建时间', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'updatedAt', label: '更新时间', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false }
]

const normalizeRegisterFieldSettings = (raw: unknown): RegisterFieldSetting[] => {
  const fallbackMap = new Map(registerDefaultFieldSettings.map((field) => [field.key, field]))
  if (!Array.isArray(raw)) return registerDefaultFieldSettings

  const parsed = raw
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const key = String((item as { key?: string }).key || '') as RegisterColumnKey
      const base = fallbackMap.get(key)
      if (!base) return null
      return {
        ...base,
        ...item,
        key: base.key,
        label: base.label
      } as RegisterFieldSetting
    })
    .filter(Boolean) as RegisterFieldSetting[]

  const seen = new Set(parsed.map((item) => item.key))
  const missing = registerDefaultFieldSettings.filter((item) => !seen.has(item.key))
  return [...parsed, ...missing]
}

const initialForm: RegisterForm = {
  type: 'risk',
  projectId: '',
  taskId: '',
  title: '',
  description: '',
  status: 'open',
  severity: 'medium',
  probability: '',
  impact: '',
  source: '',
  responsePlan: '',
  resolution: '',
  decisionDetail: '',
  background: '',
  impactScope: '',
  images: emptyUploadAttachments(),
  dueAt: '',
  ownerId: '',
  participantIds: []
}

const formatUserName = (user?: User) => {
  if (!user) return '-'
  if (user.name && user.username) return `${user.name}（${user.username}）`
  return user.name || user.username || `用户 ${user.id}`
}

const truncateText = (value: string, max = 40) => {
  const text = value.trim()
  return text.length > max ? `${text.slice(0, max)}...` : text
}

const toLocalDateTimeInput = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const normalizeForm = (item?: ProjectRegister | null): RegisterForm => ({
  id: item?.id,
  type: item?.type || 'risk',
  projectId: item?.projectId ? String(item.projectId) : '',
  taskId: item?.taskId ? String(item.taskId) : '',
  title: item?.title || '',
  description: item?.description || '',
  status: item?.status || 'open',
  severity: item?.severity || 'medium',
  probability: item?.probability || '',
  impact: item?.impact || '',
  source: item?.source || '',
  responsePlan: item?.responsePlan || '',
  resolution: item?.resolution || '',
  decisionDetail: item?.decisionDetail || '',
  background: item?.background || '',
  impactScope: item?.impactScope || '',
  images: item?.images || emptyUploadAttachments(),
  dueAt: toLocalDateTimeInput(item?.dueAt),
  ownerId: item?.ownerId ? String(item.ownerId) : '',
  participantIds: (item?.participantIds || []).map(String)
})

const parseOptionalDateTime = (value: string) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

export function ProjectRegistersPage() {
  const permissions = usePermissions()
  const [searchParams, setSearchParams] = useSearchParams()
  const canCreate = hasPermission('registers.create', permissions)
  const canUpdate = hasPermission('registers.update', permissions)
  const canDelete = hasPermission('registers.delete', permissions)
  const canUseAI = hasPermission('ai.read', permissions)
  const canUploadAttachment = hasPermission('uploads.create', permissions)
  const canReadUsers = hasPermission('system.users.read', permissions)
  const [items, setItems] = useState<ProjectRegister[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [keywordInput, setKeywordInput] = useState(searchParams.get('keyword') || '')
  const [keyword, setKeyword] = useState(searchParams.get('keyword') || '')
  const [typeFilter, setTypeFilter] = useState(searchParams.get('type') || '')
  const [statusFilter, setStatusFilter] = useState(searchParams.get('status') || '')
  const [severityFilter, setSeverityFilter] = useState(searchParams.get('severity') || '')
  const [projectFilter, setProjectFilter] = useState(searchParams.get('projectId') || '')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [form, setForm] = useState<RegisterForm>(initialForm)
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [activeItem, setActiveItem] = useState<ProjectRegister | null>(null)
  const [openedRegisterId, setOpenedRegisterId] = useState('')
  const [activities, setActivities] = useState<ProjectRegisterActivity[]>([])
  const [activityLoading, setActivityLoading] = useState(false)
  const [activityError, setActivityError] = useState('')
  const [previewImage, setPreviewImage] = useState<ProjectRegisterImage | null>(null)
  const [aiGenerating, setAiGenerating] = useState<'' | 'responsePlan' | 'impactScope'>('')
  const [aiError, setAiError] = useState<{ field: string; message: string } | null>(null)
  const [fieldSettingsOpen, setFieldSettingsOpen] = useState(false)
  const [fieldSettings, setFieldSettings] = useState<RegisterFieldSetting[]>(() => {
    try {
      return normalizeRegisterFieldSettings(JSON.parse(localStorage.getItem(registerFieldSettingsStorageKey) || '[]'))
    } catch {
      return registerDefaultFieldSettings
    }
  })

  const fieldSettingsMap = useMemo(() => new Map(fieldSettings.map((field) => [field.key, field])), [fieldSettings])
  const visibleColumns = useMemo(() => fieldSettings.filter((field) => field.visible).map((field) => field.key), [fieldSettings])
  const searchableFields = useMemo(() => fieldSettings.filter((field) => field.searchable).map((field) => field.key), [fieldSettings])
  const isSearchEnabled = searchableFields.length > 0
  const isTypeFilterEnabled = fieldSettingsMap.get('type')?.filterable ?? true
  const isStatusFilterEnabled = fieldSettingsMap.get('status')?.filterable ?? true
  const isSeverityFilterEnabled = fieldSettingsMap.get('severity')?.filterable ?? true
  const isProjectFilterEnabled = fieldSettingsMap.get('projectName')?.filterable ?? true
  const isRegisterFieldEditable = (key: RegisterColumnKey) => fieldSettingsMap.get(key)?.editable ?? true

  const userOptions = useMemo(() => users.map((user) => ({
    value: String(user.id),
    label: formatUserName(user),
    keywords: [user.name, user.username, user.email]
  })), [users])

  const typeOptions = useMemo(() => Object.entries(registerTypeLabel).map(([value, label]) => ({ value, label })), [])
  const statusOptions = useMemo(() => Object.entries(registerStatusLabel).map(([value, label]) => ({ value, label })), [])
  const severityOptions = useMemo(() => Object.entries(registerSeverityLabel).map(([value, label]) => ({ value, label })), [])
  const activeFilterCount =
    Number(Boolean(keyword) && isSearchEnabled) +
    Number(Boolean(typeFilter) && isTypeFilterEnabled) +
    Number(Boolean(statusFilter) && isStatusFilterEnabled) +
    Number(Boolean(severityFilter) && isSeverityFilterEnabled) +
    Number(Boolean(projectFilter) && isProjectFilterEnabled)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const data = await fetchPage<ProjectRegister>(
        '/project-registers',
        {
          page,
          pageSize,
          keyword: isSearchEnabled ? keyword : '',
          type: isTypeFilterEnabled ? typeFilter : '',
          status: isStatusFilterEnabled ? statusFilter : '',
          severity: isSeverityFilterEnabled ? severityFilter : '',
          projectId: isProjectFilterEnabled ? projectFilter : ''
        },
        { page, pageSize }
      )
      setItems(data.list)
      setTotal(data.total)
    } catch (loadError) {
      setError(readApiError(loadError, '登记册加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, typeFilter, statusFilter, severityFilter, projectFilter, isSearchEnabled, isTypeFilterEnabled, isStatusFilterEnabled, isSeverityFilterEnabled, isProjectFilterEnabled])

  useEffect(() => {
    localStorage.setItem(registerFieldSettingsStorageKey, JSON.stringify(fieldSettings))
  }, [fieldSettings])

  useEffect(() => {
    const next = new URLSearchParams()
    if (keyword) next.set('keyword', keyword)
    if (typeFilter) next.set('type', typeFilter)
    if (statusFilter) next.set('status', statusFilter)
    if (severityFilter) next.set('severity', severityFilter)
    if (projectFilter) next.set('projectId', projectFilter)
    setSearchParams(next, { replace: true })
  }, [keyword, typeFilter, statusFilter, severityFilter, projectFilter, setSearchParams])

  useEffect(() => {
    if (!canReadUsers) {
      setUsers([])
      return
    }
    void fetchPage<User>('/system/users', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      .then((data) => setUsers(data.list))
      .catch(() => setUsers([]))
  }, [canReadUsers])

  const openCreate = () => {
    if (!canCreate) return
    setForm({ ...initialForm, projectId: projectFilter, images: emptyUploadAttachments() })
    setFormError('')
    setAiError(null)
    setModalOpen(true)
  }

  const openEdit = (item: ProjectRegister) => {
    if (!canUpdate) return
    setForm(normalizeForm(item))
    setFormError('')
    setAiError(null)
    setModalOpen(true)
  }

  const openDetail = async (item: ProjectRegister) => {
    setActiveItem(item)
    setDetailOpen(true)
    setActivities([])
    setActivityError('')
    try {
      setActivityLoading(true)
      const data = await fetchPage<ProjectRegisterActivity>(`/project-registers/${item.id}/activities`, { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      setActivities(data.list)
    } catch (detailError) {
      setActivityError(readApiError(detailError, '登记项动态加载失败'))
      setActivities([])
    } finally {
      setActivityLoading(false)
    }
  }

  useEffect(() => {
    const registerId = searchParams.get('registerId') || ''
    if (!registerId || registerId === openedRegisterId) return
    void fetchData<ProjectRegister>(`/project-registers/${registerId}`, undefined, { silent: true })
      .then((item) => {
        setOpenedRegisterId(registerId)
        void openDetail(item)
      })
      .catch(() => setOpenedRegisterId(registerId))
  }, [searchParams, openedRegisterId])

  const generateRegisterField = async (field: 'responsePlan' | 'impactScope') => {
    if (!canUseAI || aiGenerating) return
    if (!form.projectId) {
      setAiError({ field, message: '请先选择项目，再让 AI 生成' })
      return
    }
    try {
      setAiGenerating(field)
      setAiError(null)
      // Stream tokens live into the field so slow completions stay responsive
      // and avoid proxy timeouts (the plain POST could be cut off with a 502).
      setForm((prev) => ({ ...prev, [field]: '' }))
      let streamed = ''
      const result = await postAIStream<{ content?: string }>(
        '/ai/register-analysis',
        {
          projectId: Number(form.projectId),
          registerId: form.id,
          field,
          type: form.type,
          title: form.title.trim(),
          description: form.description.trim(),
          severity: form.severity,
          probability: form.probability || undefined,
          impact: form.impact || undefined,
          source: form.source.trim(),
          background: form.background.trim(),
          decisionDetail: form.decisionDetail.trim(),
          responsePlan: form.responsePlan.trim(),
          impactScope: form.impactScope.trim()
        },
        () => {},
        (text) => {
          streamed += text
          setForm((prev) => ({ ...prev, [field]: streamed }))
        }
      )
      const content = (result?.content || streamed).trim()
      setForm((prev) => ({ ...prev, [field]: content }))
    } catch (generateError) {
      setAiError({ field, message: readApiError(generateError, 'AI 生成失败') })
    } finally {
      setAiGenerating('')
    }
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdate) return
    if (!form.id && !canCreate) return
    if (!form.projectId) {
      setFormError('请选择项目')
      return
    }
    const dueAt = parseOptionalDateTime(form.dueAt)
    if (dueAt === null) {
      setFormError('请填写有效的截止时间')
      return
    }
    const payload = {
      type: form.type,
      projectId: Number(form.projectId),
      taskId: form.taskId ? Number(form.taskId) : undefined,
      title: form.title.trim(),
      description: form.description.trim(),
      status: form.status,
      severity: form.severity,
      probability: form.probability || undefined,
      impact: form.impact || undefined,
      source: form.source.trim(),
      responsePlan: form.responsePlan.trim(),
      resolution: form.resolution.trim(),
      decisionDetail: form.decisionDetail.trim(),
      background: form.background.trim(),
      impactScope: form.impactScope.trim(),
      images: form.images,
      dueAt,
      ownerId: form.ownerId ? Number(form.ownerId) : undefined,
      participantIds: form.participantIds.map((value) => Number(value)).filter((value) => Number.isFinite(value))
    }
    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) {
        await api.put(`/project-registers/${form.id}`, payload)
      } else {
        await api.post('/project-registers', payload)
      }
      setModalOpen(false)
      setForm({ ...initialForm, images: emptyUploadAttachments() })
      await load()
      window.dispatchEvent(new Event('notifications:changed'))
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存登记项失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const deleteItem = async (item: ProjectRegister) => {
    if (!canDelete) return
    if (!window.confirm(`确认删除登记项「${item.title}」？`)) return
    try {
      await api.delete(`/project-registers/${item.id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除登记项失败'))
    }
  }

  const renderRegisterHeaderCell = (key: RegisterColumnKey) => {
    const label = fieldSettingsMap.get(key)?.label || key
    return <th key={key}>{label}</th>
  }

  const renderTextCell = (key: RegisterColumnKey, label: string, value?: string) => {
    const text = (value || '').trim()
    return <td key={key} data-label={label} title={text || undefined}>{text ? truncateText(text) : '-'}</td>
  }

  const renderRegisterCell = (item: ProjectRegister, key: RegisterColumnKey): ReactNode => {
    switch (key) {
      case 'type':
        return <td key={key} data-label="类型">{registerTypeLabel[item.type]}</td>
      case 'title':
        return <td key={key} data-label="标题"><span className="register-title-text">{item.title}</span></td>
      case 'projectName':
        return <td key={key} data-label="项目">{item.project ? `${item.project.code} - ${item.project.name}` : `#${item.projectId}`}</td>
      case 'status':
        return <td key={key} data-label="状态">{registerStatusLabel[item.status]}</td>
      case 'severity':
        return <td key={key} data-label="等级">{registerSeverityLabel[item.severity]}</td>
      case 'probability':
        return <td key={key} data-label="概率">{item.probability ? registerProbabilityLabel[item.probability] : '-'}</td>
      case 'impact':
        return <td key={key} data-label="影响">{item.impact ? registerSeverityLabel[item.impact] : '-'}</td>
      case 'owner':
        return <td key={key} data-label="负责人">{formatUserName(item.owner)}</td>
      case 'participants':
        return <td key={key} data-label="参与人">{(item.participantIds?.length || 0) > 0 ? `${item.participantIds?.length} 人` : '-'}</td>
      case 'source':
        return renderTextCell(key, '来源', item.source)
      case 'description':
        return renderTextCell(key, '说明', item.description)
      case 'responsePlan':
        return renderTextCell(key, '应对策略', item.responsePlan)
      case 'resolution':
        return renderTextCell(key, '解决方案', item.resolution)
      case 'decisionDetail':
        return renderTextCell(key, '决策内容', item.decisionDetail)
      case 'background':
        return renderTextCell(key, '背景', item.background)
      case 'impactScope':
        return renderTextCell(key, '影响范围', item.impactScope)
      case 'dueAt':
        return <td key={key} data-label="截止时间">{formatDateTime(item.dueAt)}</td>
      case 'lastActivityAt':
        return <td key={key} data-label="最近动态">{formatDateTime(item.lastActivityAt)}</td>
      case 'createdAt':
        return <td key={key} data-label="创建时间">{formatDateTime(item.createdAt)}</td>
      case 'updatedAt':
        return <td key={key} data-label="更新时间">{formatDateTime(item.updatedAt)}</td>
      default:
        return null
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="登记册筛选"
        activeCount={activeFilterCount}
        actions={canCreate ? <button className="btn" onClick={openCreate}>新增登记项</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        {isSearchEnabled && (
          <SearchField
            className="toolbar-search-field"
            aria-label="登记册关键词搜索"
            value={keywordInput}
            placeholder="搜索标题/说明/来源"
            onChange={setKeywordInput}
            onClear={() => { setKeywordInput(''); setKeyword(''); setPage(1) }}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                setKeyword(keywordInput.trim())
                setPage(1)
              }
            }}
          />
        )}
        {isTypeFilterEnabled && <SearchableSelect ariaLabel="登记类型筛选" value={typeFilter} options={typeOptions} defaultOptionLabel="全部类型" placeholder="搜索类型" noResultsText="没有匹配类型" onChange={(value) => { setTypeFilter(value); setPage(1) }} />}
        {isStatusFilterEnabled && <SearchableSelect ariaLabel="登记状态筛选" value={statusFilter} options={statusOptions} defaultOptionLabel="全部状态" placeholder="搜索状态" noResultsText="没有匹配状态" onChange={(value) => { setStatusFilter(value); setPage(1) }} />}
        {isSeverityFilterEnabled && <SearchableSelect ariaLabel="登记等级筛选" value={severityFilter} options={severityOptions} defaultOptionLabel="全部等级" placeholder="搜索等级" noResultsText="没有匹配等级" onChange={(value) => { setSeverityFilter(value); setPage(1) }} />}
        {isProjectFilterEnabled && <RemoteProjectSelect ariaLabel="登记册项目筛选" value={projectFilter} defaultOptionLabel="全部项目" placeholder="搜索项目" onChange={(value) => { setProjectFilter(value); setPage(1) }} />}
        {isSearchEnabled && (
          <div className="row-actions">
            <button className="btn" onClick={() => { setKeyword(keywordInput.trim()); setPage(1) }}>查询</button>
          </div>
        )}
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无登记项" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr>
            {visibleColumns.map((columnKey) => renderRegisterHeaderCell(columnKey))}
            <th className="field-settings-header-cell">
              <span className="field-settings-header-inline">
                <span>操作</span>
                <button type="button" className="field-settings-icon-btn" aria-label="登记册字段设置" onClick={() => setFieldSettingsOpen(true)}>
                  <Settings2 size={16} />
                </button>
              </span>
            </th>
          </tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                {visibleColumns.map((columnKey) => renderRegisterCell(item, columnKey))}
                <td data-label="操作">
                  <div className="table-actions">
                    <button className="btn secondary" onClick={() => { void openDetail(item) }}>查看</button>
                    {canUpdate && <button className="btn secondary" onClick={() => openEdit(item)}>编辑</button>}
                    {canDelete && <button className="btn danger" onClick={() => { void deleteItem(item) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>
      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <FieldSettingsModal
        open={fieldSettingsOpen}
        title="登记册字段设置"
        fields={fieldSettings}
        defaultFields={registerDefaultFieldSettings}
        onClose={() => setFieldSettingsOpen(false)}
        onSave={(fields) => {
          setFieldSettings(fields)
          setFieldSettingsOpen(false)
        }}
      />

      <Modal open={modalOpen} title={form.id ? '编辑登记项' : '新增登记项'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label htmlFor="register-type">类型</label>
          <select id="register-type" value={form.type} disabled={!isRegisterFieldEditable('type')} onChange={(event) => setForm((prev) => ({ ...prev, type: event.target.value as ProjectRegisterType }))}>
            {Object.entries(registerTypeLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <label className="required-label" htmlFor="register-project">项目</label>
          <RemoteProjectSelect ariaLabel="登记项项目" value={form.projectId} defaultOptionLabel="请选择项目" placeholder="搜索项目" disabled={!isRegisterFieldEditable('projectName')} onChange={(value) => setForm((prev) => ({ ...prev, projectId: value }))} />
          <label className="required-label" htmlFor="register-title">标题</label>
          <input id="register-title" value={form.title} disabled={!isRegisterFieldEditable('title')} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} required />
          <label htmlFor="register-description">说明</label>
          <textarea id="register-description" rows={3} value={form.description} disabled={!isRegisterFieldEditable('description')} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="register-status">状态</label>
          <select id="register-status" value={form.status} disabled={!isRegisterFieldEditable('status')} onChange={(event) => setForm((prev) => ({ ...prev, status: event.target.value as ProjectRegisterStatus }))}>
            {Object.entries(registerStatusLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <label htmlFor="register-severity">等级</label>
          <select id="register-severity" value={form.severity} disabled={!isRegisterFieldEditable('severity')} onChange={(event) => setForm((prev) => ({ ...prev, severity: event.target.value as ProjectRegisterSeverity }))}>
            {Object.entries(registerSeverityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          {form.type === 'risk' && (
            <>
              <label htmlFor="register-probability">概率</label>
              <select id="register-probability" value={form.probability} disabled={!isRegisterFieldEditable('probability')} onChange={(event) => setForm((prev) => ({ ...prev, probability: event.target.value as RegisterForm['probability'] }))}>
                <option value="">未设置</option>
                {Object.entries(registerProbabilityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
              <label htmlFor="register-impact">影响</label>
              <select id="register-impact" value={form.impact} disabled={!isRegisterFieldEditable('impact')} onChange={(event) => setForm((prev) => ({ ...prev, impact: event.target.value as RegisterForm['impact'] }))}>
                <option value="">未设置</option>
                {Object.entries(registerSeverityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
              <div className="field-label-row">
                <label htmlFor="register-response-plan">应对策略</label>
                {canUseAI && Boolean(form.id) && (
                  <button
                    type="button"
                    className="btn secondary field-ai-btn"
                    onClick={() => { void generateRegisterField('responsePlan') }}
                    disabled={submitting || aiGenerating !== '' || !form.projectId || !isRegisterFieldEditable('responsePlan')}
                  >
                    <Sparkles size={14} aria-hidden="true" />
                    {aiGenerating === 'responsePlan' ? '生成中...' : 'AI生成'}
                  </button>
                )}
              </div>
              <textarea id="register-response-plan" rows={3} value={form.responsePlan} disabled={!isRegisterFieldEditable('responsePlan')} onChange={(event) => setForm((prev) => ({ ...prev, responsePlan: event.target.value }))} />
              {aiError?.field === 'responsePlan' && <p className="error">{aiError.message}</p>}
            </>
          )}
          {form.type === 'issue' && (
            <>
              <label htmlFor="register-source">问题来源</label>
              <input id="register-source" value={form.source} disabled={!isRegisterFieldEditable('source')} onChange={(event) => setForm((prev) => ({ ...prev, source: event.target.value }))} />
              <label htmlFor="register-resolution">解决方案</label>
              <textarea id="register-resolution" rows={3} value={form.resolution} disabled={!isRegisterFieldEditable('resolution')} onChange={(event) => setForm((prev) => ({ ...prev, resolution: event.target.value }))} />
            </>
          )}
          {form.type === 'decision' && (
            <>
              <label htmlFor="register-background">背景</label>
              <textarea id="register-background" rows={3} value={form.background} disabled={!isRegisterFieldEditable('background')} onChange={(event) => setForm((prev) => ({ ...prev, background: event.target.value }))} />
              <label htmlFor="register-decision-detail">决策内容</label>
              <textarea id="register-decision-detail" rows={3} value={form.decisionDetail} disabled={!isRegisterFieldEditable('decisionDetail')} onChange={(event) => setForm((prev) => ({ ...prev, decisionDetail: event.target.value }))} />
            </>
          )}
          <div className="field-label-row">
            <label htmlFor="register-impact-scope">影响范围</label>
            {canUseAI && Boolean(form.id) && (
              <button
                type="button"
                className="btn secondary field-ai-btn"
                onClick={() => { void generateRegisterField('impactScope') }}
                disabled={submitting || aiGenerating !== '' || !form.projectId || !isRegisterFieldEditable('impactScope')}
              >
                <Sparkles size={14} aria-hidden="true" />
                {aiGenerating === 'impactScope' ? '生成中...' : 'AI生成'}
              </button>
            )}
          </div>
          <textarea id="register-impact-scope" rows={3} value={form.impactScope} disabled={!isRegisterFieldEditable('impactScope')} onChange={(event) => setForm((prev) => ({ ...prev, impactScope: event.target.value }))} />
          {aiError?.field === 'impactScope' && <p className="error">{aiError.message}</p>}
          <label htmlFor="register-images">图片</label>
          <ImageAttachmentField
            inputId="register-images"
            value={form.images}
            disabled={submitting}
            uploadDisabled={!canUploadAttachment}
            projectId={form.projectId}
            registerId={form.id}
            canGenerateDescription={canUseAI}
            onChange={(images) => setForm((prev) => ({ ...prev, images }))}
          />
          <label htmlFor="register-due-at">截止时间</label>
          <DateTimeQuickField inputId="register-due-at" value={form.dueAt} disabled={!isRegisterFieldEditable('dueAt')} onChange={(value) => setForm((prev) => ({ ...prev, dueAt: value }))} />
          <label htmlFor="register-owner">负责人</label>
          <select id="register-owner" value={form.ownerId} disabled={!isRegisterFieldEditable('owner')} onChange={(event) => setForm((prev) => ({ ...prev, ownerId: event.target.value }))}>
            <option value="">未设置</option>
            {users.map((user) => <option key={user.id} value={user.id}>{formatUserName(user)}</option>)}
          </select>
          <label htmlFor="register-participants">参与人</label>
          <SearchableMultiSelect
            ariaLabel="登记项参与人"
            values={form.participantIds}
            options={userOptions}
            summaryNoun="参与人"
            placeholder="搜索参与人"
            noResultsText="没有匹配的参与人"
            disabled={!isRegisterFieldEditable('participants')}
            onChange={(values) => setForm((prev) => ({ ...prev, participantIds: values }))}
          />
          {!canReadUsers && <p className="inline-tip">当前账号无用户查看权限，负责人和参与人只能保持为空。</p>}
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存'}</button>
          </div>
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>

      <Modal open={detailOpen} title="登记项详情" onClose={() => setDetailOpen(false)}>
        {activeItem && (
          <div className="register-detail">
            <section className="detail-section">
              <h4>{activeItem.title}</h4>
              <div className="detail-columns">
                <div><strong>类型：</strong>{registerTypeLabel[activeItem.type]}</div>
                <div><strong>状态：</strong>{registerStatusLabel[activeItem.status]}</div>
                <div><strong>等级：</strong>{registerSeverityLabel[activeItem.severity]}</div>
                <div><strong>负责人：</strong>{formatUserName(activeItem.owner)}</div>
                <div><strong>项目：</strong>{activeItem.project ? `${activeItem.project.code} - ${activeItem.project.name}` : `#${activeItem.projectId}`}</div>
                <div><strong>截止：</strong>{formatDateTime(activeItem.dueAt)}</div>
              </div>
              {activeItem.description && <p>{activeItem.description}</p>}
              {Boolean(activeItem.images?.length) && (
                <div className="register-image-gallery">
                  {activeItem.images?.map((image) => (
                    <figure key={image.filePath} className="register-image-gallery-item">
                      <button type="button" onClick={() => setPreviewImage(image)} aria-label={`预览${image.relativePath || image.fileName || '登记项图片'}`}>
                        <img src={image.filePath} alt={image.remark || image.relativePath || image.fileName || '登记项图片'} />
                      </button>
                      {image.remark && <figcaption>{image.remark}</figcaption>}
                    </figure>
                  ))}
                </div>
              )}
            </section>
            <section className="detail-section">
              <h4>动态</h4>
              <DataState loading={activityLoading} error={activityError} empty={!activityLoading && !activityError && activities.length === 0} emptyText="暂无动态" onRetry={() => activeItem && void openDetail(activeItem)} />
              {!activityLoading && !activityError && activities.length > 0 && (
                <div className="register-activity-list">
                  {activities.map((activity) => (
                    <article key={activity.id} className="task-timeline-item">
                      <div className="task-timeline-meta">
                        <span className="task-timeline-badge">{activityTypeLabel[activity.type] || '动态'}</span>
                        <span>{formatUserName(activity.actor)}</span>
                        <span>{formatDateTime(activity.createdAt)}</span>
                      </div>
                      <strong>{activity.summary}</strong>
                      {activity.detail && <p>{activity.detail}</p>}
                    </article>
                  ))}
                </div>
              )}
            </section>
          </div>
        )}
      </Modal>
      <ImagePreviewOverlay image={previewImage} onClose={() => setPreviewImage(null)} />
    </section>
  )
}
