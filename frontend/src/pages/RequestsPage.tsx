import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { SearchableSelect } from '../components/SearchableSelect'
import { formatDateTime } from '../utils/datetime'
import { Project, Tag, TaskPriority, User, WorkRequest, WorkRequestStatus, WorkRequestType } from '../types'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { usePermissions } from '../hooks/usePermissions'

const requestTypeLabel: Record<WorkRequestType, string> = {
  project: '项目申请',
  task: '任务请求',
  bug: '缺陷/问题',
  change: '变更申请'
}

const requestStatusLabel: Record<WorkRequestStatus, string> = {
  submitted: '待审批',
  approved: '已通过',
  rejected: '已拒绝',
  converted: '已转任务'
}

const priorityLabel: Record<TaskPriority, string> = {
  high: '高',
  medium: '中',
  low: '低'
}

interface RequestForm {
  type: WorkRequestType
  title: string
  description: string
  priority: TaskPriority
  projectId: string
}

interface ReviewForm {
  status: 'approved' | 'rejected'
  note: string
}

interface ConvertForm {
  projectId: string
  assigneeIds: number[]
  reviewerIds: number[]
  tagIds: number[]
  startAt: string
  endAt: string
}

const initialRequestForm: RequestForm = {
  type: 'task',
  title: '',
  description: '',
  priority: 'medium',
  projectId: ''
}

const initialReviewForm: ReviewForm = {
  status: 'approved',
  note: ''
}

const initialConvertForm: ConvertForm = {
  projectId: '',
  assigneeIds: [],
  reviewerIds: [],
  tagIds: [],
  startAt: '',
  endAt: ''
}

const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]
const formatUserName = (user?: User) => user ? (user.name && user.username ? `${user.name}（${user.username}）` : user.name || user.username) : '-'

export function RequestsPage() {
  const navigate = useNavigate()
  const permissions = usePermissions()
  const canCreateRequest = hasPermission('requests.create', permissions)
  const canUpdateRequest = hasPermission('requests.update', permissions)
  const canReadProjects = hasPermission('projects.read', permissions)
  const canReadTasks = hasPermission('tasks.read', permissions)
  const [items, setItems] = useState<WorkRequest[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [requestForm, setRequestForm] = useState<RequestForm>(initialRequestForm)
  const [reviewForm, setReviewForm] = useState<ReviewForm>(initialReviewForm)
  const [convertForm, setConvertForm] = useState<ConvertForm>(initialConvertForm)
  const [activeRequest, setActiveRequest] = useState<WorkRequest | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [reviewOpen, setReviewOpen] = useState(false)
  const [convertOpen, setConvertOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(typeFilter)) + Number(Boolean(statusFilter))

  const projectOptions = useMemo(() => projects.map((project) => ({
    value: String(project.id),
    label: `${project.code} - ${project.name}`,
    keywords: [project.code, project.name, project.description || '']
  })), [projects])

  const requestTypeOptions = useMemo(() => Object.entries(requestTypeLabel).map(([value, label]) => ({ value, label })), [])
  const requestStatusOptions = useMemo(() => Object.entries(requestStatusLabel).map(([value, label]) => ({ value, label })), [])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const requestPage = await fetchPage<WorkRequest>(
        '/requests',
        { page, pageSize, keyword, type: typeFilter, status: statusFilter },
        { page, pageSize }
      )
      setItems(requestPage.list)
      setTotal(requestPage.total)
    } catch (loadError) {
      setError(readApiError(loadError, '请求列表加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, typeFilter, statusFilter])

  useEffect(() => {
    if (!canReadProjects) {
      setProjects([])
      return
    }
    void fetchPage<Project>('/projects', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      .then((res) => setProjects(res.list))
      .catch(() => setProjects([]))
  }, [canReadProjects])

  useEffect(() => {
    if (!canUpdateRequest) {
      setUsers([])
      setTags([])
      return
    }
    void Promise.all([
      fetchPage<User>('/users', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true }).catch(() => ({ list: [] as User[], total: 0, page: 1, pageSize: 100 })),
      fetchPage<Tag>('/tags', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true }).catch(() => ({ list: [] as Tag[], total: 0, page: 1, pageSize: 100 }))
    ]).then(([userPage, tagPage]) => {
      setUsers(userPage.list)
      setTags(tagPage.list)
    })
  }, [canUpdateRequest])

  const openCreateModal = () => {
    if (!canCreateRequest) return
    setRequestForm(initialRequestForm)
    setFormError('')
    setFormSuccess('')
    setCreateOpen(true)
  }

  const openReviewModal = (item: WorkRequest, status: 'approved' | 'rejected' = 'approved') => {
    if (!canUpdateRequest) return
    setActiveRequest(item)
    setReviewForm({ status, note: item.approvalNote || '' })
    setFormError('')
    setFormSuccess('')
    setReviewOpen(true)
  }

  const openConvertModal = (item: WorkRequest) => {
    if (!canUpdateRequest) return
    setActiveRequest(item)
    setConvertForm({
      ...initialConvertForm,
      projectId: item.projectId ? String(item.projectId) : projects[0]?.id ? String(projects[0].id) : ''
    })
    setFormError('')
    setFormSuccess('')
    setConvertOpen(true)
  }

  const submitRequest = async (event: FormEvent) => {
    event.preventDefault()
    if (!canCreateRequest) return
    try {
      setSubmitting(true)
      setFormError('')
      await api.post('/requests', {
        ...requestForm,
        projectId: requestForm.projectId ? Number(requestForm.projectId) : undefined
      })
      setFormSuccess('提交成功')
      setCreateOpen(false)
      setRequestForm(initialRequestForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '提交请求失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const submitReview = async (event: FormEvent) => {
    event.preventDefault()
    if (!activeRequest || !canUpdateRequest) return
    try {
      setSubmitting(true)
      setFormError('')
      await api.patch(`/requests/${activeRequest.id}/review`, { status: reviewForm.status, note: reviewForm.note })
      setFormSuccess('审批成功')
      setReviewOpen(false)
      setActiveRequest(null)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '审批请求失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const submitConvert = async (event: FormEvent) => {
    event.preventDefault()
    if (!activeRequest || !canUpdateRequest) return
    if (!convertForm.projectId) {
      setFormError('请选择目标项目')
      return
    }
    try {
      setSubmitting(true)
      setFormError('')
      await api.post(`/requests/${activeRequest.id}/convert-task`, {
        ...convertForm,
        projectId: Number(convertForm.projectId),
        startAt: convertForm.startAt ? new Date(convertForm.startAt).toISOString() : '',
        endAt: convertForm.endAt ? new Date(convertForm.endAt).toISOString() : ''
      })
      setFormSuccess('已转为任务')
      setConvertOpen(false)
      setActiveRequest(null)
      await load()
      window.dispatchEvent(new Event('notifications:changed'))
    } catch (submitError) {
      setFormError(readApiError(submitError, '转为任务失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const openTask = async (item: WorkRequest) => {
    if (!item.convertedTaskId || !canReadTasks) return
    navigate(`/tasks?taskId=${item.convertedTaskId}&view=1`)
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="请求筛选"
        activeCount={activeFilterCount}
        actions={canCreateRequest ? <button className="btn" onClick={openCreateModal}>提交请求</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="请求关键词搜索"
          value={keywordInput}
          placeholder="搜索请求标题/描述"
          onChange={setKeywordInput}
          onClear={() => {
            setPage(1)
            setKeyword('')
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              setPage(1)
              setKeyword(keywordInput.trim())
            }
          }}
        />
        <SearchableSelect
          ariaLabel="请求类型筛选"
          value={typeFilter}
          options={requestTypeOptions}
          defaultOptionLabel="全部类型"
          placeholder="搜索请求类型"
          noResultsText="没有匹配的类型"
          onChange={(value) => {
            setTypeFilter(value)
            setPage(1)
          }}
        />
        <SearchableSelect
          ariaLabel="请求状态筛选"
          value={statusFilter}
          options={requestStatusOptions}
          defaultOptionLabel="全部状态"
          placeholder="搜索请求状态"
          noResultsText="没有匹配的状态"
          onChange={(value) => {
            setStatusFilter(value)
            setPage(1)
          }}
        />
        <div className="row-actions">
          <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
        </div>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无请求数据" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>ID</th><th>类型</th><th>标题</th><th>状态</th><th>优先级</th><th>关联项目</th><th>提交人</th><th>审批人</th><th>创建时间</th><th>操作</th>
          </tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="ID">{item.id}</td>
                <td data-label="类型">{requestTypeLabel[item.type]}</td>
                <td data-label="标题">{item.title}</td>
                <td data-label="状态">{requestStatusLabel[item.status]}</td>
                <td data-label="优先级">{priorityLabel[item.priority || 'medium']}</td>
                <td data-label="关联项目">{item.project ? `${item.project.code} - ${item.project.name}` : '-'}</td>
                <td data-label="提交人">{formatUserName(item.requester)}</td>
                <td data-label="审批人">{formatUserName(item.reviewer)}</td>
                <td data-label="创建时间">{formatDateTime(item.createdAt)}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    {canUpdateRequest && item.status !== 'converted' && (
                      <>
                        <button className="btn secondary" onClick={() => openReviewModal(item, 'approved')}>审批</button>
                        <button className="btn secondary" disabled={item.status === 'rejected'} onClick={() => openConvertModal(item)}>转任务</button>
                      </>
                    )}
                    {item.convertedTaskId && canReadTasks && <button className="btn secondary" onClick={() => { void openTask(item) }}>查看任务</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={createOpen} title="提交请求" onClose={() => setCreateOpen(false)}>
        <form className="form-grid" onSubmit={submitRequest}>
          <label className="required-label" htmlFor="request-type">类型</label>
          <select id="request-type" value={requestForm.type} onChange={(event) => setRequestForm((prev) => ({ ...prev, type: event.target.value as WorkRequestType }))}>
            {Object.entries(requestTypeLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <label className="required-label" htmlFor="request-title">标题</label>
          <input id="request-title" value={requestForm.title} onChange={(event) => setRequestForm((prev) => ({ ...prev, title: event.target.value }))} required />
          <label htmlFor="request-description">描述</label>
          <textarea id="request-description" rows={4} value={requestForm.description} onChange={(event) => setRequestForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="request-priority">优先级</label>
          <select id="request-priority" value={requestForm.priority} onChange={(event) => setRequestForm((prev) => ({ ...prev, priority: event.target.value as TaskPriority }))}>
            {Object.entries(priorityLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          {canReadProjects && (
            <>
              <label htmlFor="request-project">关联项目（可选）</label>
              <SearchableSelect
                ariaLabel="请求关联项目"
                value={requestForm.projectId}
                options={projectOptions}
                defaultOptionLabel="不关联项目"
                placeholder="搜索项目"
                noResultsText="没有匹配的项目"
                onChange={(value) => setRequestForm((prev) => ({ ...prev, projectId: value }))}
              />
            </>
          )}
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || !canCreateRequest}>{submitting ? '提交中...' : '提交'}</button>
            <button type="button" className="btn secondary" onClick={() => setRequestForm(initialRequestForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>

      <Modal open={reviewOpen} title="审批请求" onClose={() => setReviewOpen(false)}>
        <form className="form-grid" onSubmit={submitReview}>
          <label htmlFor="review-status">审批结果</label>
          <select id="review-status" value={reviewForm.status} onChange={(event) => setReviewForm((prev) => ({ ...prev, status: event.target.value as ReviewForm['status'] }))}>
            <option value="approved">通过</option>
            <option value="rejected">拒绝</option>
          </select>
          <label htmlFor="review-note">审批意见</label>
          <textarea id="review-note" rows={4} value={reviewForm.note} onChange={(event) => setReviewForm((prev) => ({ ...prev, note: event.target.value }))} />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || !canUpdateRequest}>{submitting ? '保存中...' : '保存审批'}</button>
          </div>
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>

      <Modal open={convertOpen} title="转为任务" onClose={() => setConvertOpen(false)}>
        <form className="form-grid" onSubmit={submitConvert}>
          <label className="required-label" htmlFor="convert-project">目标项目</label>
          <SearchableSelect
            ariaLabel="转任务目标项目"
            value={convertForm.projectId}
            options={projectOptions}
            defaultOptionLabel="请选择项目"
            placeholder="搜索项目"
            noResultsText="没有匹配的项目"
            onChange={(value) => setConvertForm((prev) => ({ ...prev, projectId: value }))}
          />
          <label htmlFor="convert-assignees">执行人</label>
          <select id="convert-assignees" multiple value={convertForm.assigneeIds.map(String)} onChange={(event) => setConvertForm((prev) => ({ ...prev, assigneeIds: Array.from(event.target.selectedOptions).map((option) => Number(option.value)) }))}>
            {users.map((user) => <option key={user.id} value={user.id}>{formatUserName(user)}</option>)}
          </select>
          <label htmlFor="convert-reviewers">审核人</label>
          <select id="convert-reviewers" multiple value={convertForm.reviewerIds.map(String)} onChange={(event) => setConvertForm((prev) => ({ ...prev, reviewerIds: Array.from(event.target.selectedOptions).map((option) => Number(option.value)) }))}>
            {users.map((user) => <option key={user.id} value={user.id}>{formatUserName(user)}</option>)}
          </select>
          <label htmlFor="convert-tags">标签</label>
          <div id="convert-tags" className="multi-checklist">
            {tags.map((tag) => (
              <label key={tag.id} className="multi-check-item">
                <input type="checkbox" checked={convertForm.tagIds.includes(tag.id)} onChange={() => setConvertForm((prev) => ({ ...prev, tagIds: toggleNumber(prev.tagIds, tag.id) }))} />
                <span>{tag.name}</span>
              </label>
            ))}
            {tags.length === 0 && <p className="inline-tip">暂无可选标签</p>}
          </div>
          <label htmlFor="convert-start">开始时间</label>
          <DateTimeQuickField inputId="convert-start" value={convertForm.startAt} onChange={(value) => setConvertForm((prev) => ({ ...prev, startAt: value }))} />
          <label htmlFor="convert-end">结束时间</label>
          <DateTimeQuickField inputId="convert-end" value={convertForm.endAt} onChange={(value) => setConvertForm((prev) => ({ ...prev, endAt: value }))} />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || !canUpdateRequest}>{submitting ? '转换中...' : '转为任务'}</button>
          </div>
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>
    </section>
  )
}
