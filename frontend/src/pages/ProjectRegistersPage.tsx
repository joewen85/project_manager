import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
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
import { ProjectRegister, ProjectRegisterActivity, ProjectRegisterProbability, ProjectRegisterSeverity, ProjectRegisterStatus, ProjectRegisterType, UploadAttachment, User, emptyUploadAttachments } from '../types'
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
  images: UploadAttachment[]
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
  const [previewImage, setPreviewImage] = useState<UploadAttachment | null>(null)

  const userOptions = useMemo(() => users.map((user) => ({
    value: String(user.id),
    label: formatUserName(user),
    keywords: [user.name, user.username, user.email]
  })), [users])

  const typeOptions = useMemo(() => Object.entries(registerTypeLabel).map(([value, label]) => ({ value, label })), [])
  const statusOptions = useMemo(() => Object.entries(registerStatusLabel).map(([value, label]) => ({ value, label })), [])
  const severityOptions = useMemo(() => Object.entries(registerSeverityLabel).map(([value, label]) => ({ value, label })), [])
  const activeFilterCount = Number(Boolean(keyword)) + Number(Boolean(typeFilter)) + Number(Boolean(statusFilter)) + Number(Boolean(severityFilter)) + Number(Boolean(projectFilter))

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const data = await fetchPage<ProjectRegister>(
        '/project-registers',
        { page, pageSize, keyword, type: typeFilter, status: statusFilter, severity: severityFilter, projectId: projectFilter },
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

  useEffect(() => { void load() }, [page, pageSize, keyword, typeFilter, statusFilter, severityFilter, projectFilter])

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
    setModalOpen(true)
  }

  const openEdit = (item: ProjectRegister) => {
    if (!canUpdate) return
    setForm(normalizeForm(item))
    setFormError('')
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

  return (
    <section className="page-section">
      <FilterPanel
        title="登记册筛选"
        activeCount={activeFilterCount}
        actions={canCreate ? <button className="btn" onClick={openCreate}>新增登记项</button> : undefined}
        bodyClassName="toolbar-grid"
      >
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
        <SearchableSelect ariaLabel="登记类型筛选" value={typeFilter} options={typeOptions} defaultOptionLabel="全部类型" placeholder="搜索类型" noResultsText="没有匹配类型" onChange={(value) => { setTypeFilter(value); setPage(1) }} />
        <SearchableSelect ariaLabel="登记状态筛选" value={statusFilter} options={statusOptions} defaultOptionLabel="全部状态" placeholder="搜索状态" noResultsText="没有匹配状态" onChange={(value) => { setStatusFilter(value); setPage(1) }} />
        <SearchableSelect ariaLabel="登记等级筛选" value={severityFilter} options={severityOptions} defaultOptionLabel="全部等级" placeholder="搜索等级" noResultsText="没有匹配等级" onChange={(value) => { setSeverityFilter(value); setPage(1) }} />
        <RemoteProjectSelect ariaLabel="登记册项目筛选" value={projectFilter} defaultOptionLabel="全部项目" placeholder="搜索项目" onChange={(value) => { setProjectFilter(value); setPage(1) }} />
        <div className="row-actions">
          <button className="btn" onClick={() => { setKeyword(keywordInput.trim()); setPage(1) }}>查询</button>
        </div>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无登记项" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>类型</th><th>标题</th><th>项目</th><th>状态</th><th>等级</th><th>负责人</th><th>截止时间</th><th>操作</th>
          </tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="类型">{registerTypeLabel[item.type]}</td>
                <td data-label="标题">
                  <div className="register-title-cell">
                    <strong>{item.title}</strong>
                    <small>{item.description || item.source || '-'}</small>
                  </div>
                </td>
                <td data-label="项目">{item.project ? `${item.project.code} - ${item.project.name}` : `#${item.projectId}`}</td>
                <td data-label="状态">{registerStatusLabel[item.status]}</td>
                <td data-label="等级">{registerSeverityLabel[item.severity]}</td>
                <td data-label="负责人">{formatUserName(item.owner)}</td>
                <td data-label="截止时间">{formatDateTime(item.dueAt)}</td>
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

      <Modal open={modalOpen} title={form.id ? '编辑登记项' : '新增登记项'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label htmlFor="register-type">类型</label>
          <select id="register-type" value={form.type} onChange={(event) => setForm((prev) => ({ ...prev, type: event.target.value as ProjectRegisterType }))}>
            {Object.entries(registerTypeLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <label className="required-label" htmlFor="register-project">项目</label>
          <RemoteProjectSelect ariaLabel="登记项项目" value={form.projectId} defaultOptionLabel="请选择项目" placeholder="搜索项目" onChange={(value) => setForm((prev) => ({ ...prev, projectId: value }))} />
          <label className="required-label" htmlFor="register-title">标题</label>
          <input id="register-title" value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} required />
          <label htmlFor="register-description">说明</label>
          <textarea id="register-description" rows={3} value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="register-status">状态</label>
          <select id="register-status" value={form.status} onChange={(event) => setForm((prev) => ({ ...prev, status: event.target.value as ProjectRegisterStatus }))}>
            {Object.entries(registerStatusLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <label htmlFor="register-severity">等级</label>
          <select id="register-severity" value={form.severity} onChange={(event) => setForm((prev) => ({ ...prev, severity: event.target.value as ProjectRegisterSeverity }))}>
            {Object.entries(registerSeverityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          {form.type === 'risk' && (
            <>
              <label htmlFor="register-probability">概率</label>
              <select id="register-probability" value={form.probability} onChange={(event) => setForm((prev) => ({ ...prev, probability: event.target.value as RegisterForm['probability'] }))}>
                <option value="">未设置</option>
                {Object.entries(registerProbabilityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
              <label htmlFor="register-impact">影响</label>
              <select id="register-impact" value={form.impact} onChange={(event) => setForm((prev) => ({ ...prev, impact: event.target.value as RegisterForm['impact'] }))}>
                <option value="">未设置</option>
                {Object.entries(registerSeverityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
              <label htmlFor="register-response-plan">应对策略</label>
              <textarea id="register-response-plan" rows={3} value={form.responsePlan} onChange={(event) => setForm((prev) => ({ ...prev, responsePlan: event.target.value }))} />
            </>
          )}
          {form.type === 'issue' && (
            <>
              <label htmlFor="register-source">问题来源</label>
              <input id="register-source" value={form.source} onChange={(event) => setForm((prev) => ({ ...prev, source: event.target.value }))} />
              <label htmlFor="register-resolution">解决方案</label>
              <textarea id="register-resolution" rows={3} value={form.resolution} onChange={(event) => setForm((prev) => ({ ...prev, resolution: event.target.value }))} />
            </>
          )}
          {form.type === 'decision' && (
            <>
              <label htmlFor="register-background">背景</label>
              <textarea id="register-background" rows={3} value={form.background} onChange={(event) => setForm((prev) => ({ ...prev, background: event.target.value }))} />
              <label htmlFor="register-decision-detail">决策内容</label>
              <textarea id="register-decision-detail" rows={3} value={form.decisionDetail} onChange={(event) => setForm((prev) => ({ ...prev, decisionDetail: event.target.value }))} />
            </>
          )}
          <label htmlFor="register-impact-scope">影响范围</label>
          <textarea id="register-impact-scope" rows={3} value={form.impactScope} onChange={(event) => setForm((prev) => ({ ...prev, impactScope: event.target.value }))} />
          <label htmlFor="register-images">图片</label>
          <ImageAttachmentField inputId="register-images" value={form.images} disabled={!canUploadAttachment} onChange={(images) => setForm((prev) => ({ ...prev, images }))} />
          <label htmlFor="register-due-at">截止时间</label>
          <DateTimeQuickField inputId="register-due-at" value={form.dueAt} onChange={(value) => setForm((prev) => ({ ...prev, dueAt: value }))} />
          <label htmlFor="register-owner">负责人</label>
          <select id="register-owner" value={form.ownerId} onChange={(event) => setForm((prev) => ({ ...prev, ownerId: event.target.value }))}>
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
                    <button key={image.filePath} type="button" onClick={() => setPreviewImage(image)} aria-label={`预览${image.relativePath || image.fileName || '登记项图片'}`}>
                      <img src={image.filePath} alt={image.relativePath || image.fileName || '登记项图片'} />
                    </button>
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
