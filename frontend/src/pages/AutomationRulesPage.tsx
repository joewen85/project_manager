import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { AssigneeChangeType, AutomationExecutionLog, AutomationRule, AutomationTrigger, Project, Status, Tag } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface AutomationRuleForm {
  id?: number
  trigger: AutomationTrigger
  name: string
  isEnabled: boolean
  overdueDays: number
  projectIds: number[]
  fromStatuses: Status[]
  toStatuses: Status[]
  fromProgressMin: string
  fromProgressMax: string
  toProgressMin: string
  toProgressMax: string
  assigneeChangeTypes: AssigneeChangeType[]
  notifyAssignees: boolean
  notifyProjectOwners: boolean
  addComment: boolean
  commentContent: string
  addTags: boolean
  tagIds: number[]
}

const initialForm: AutomationRuleForm = {
  trigger: 'task_overdue',
  name: '',
  isEnabled: true,
  overdueDays: 1,
  projectIds: [],
  fromStatuses: [],
  toStatuses: [],
  fromProgressMin: '',
  fromProgressMax: '',
  toProgressMin: '',
  toProgressMax: '',
  assigneeChangeTypes: ['added', 'removed'],
  notifyAssignees: true,
  notifyProjectOwners: true,
  addComment: false,
  commentContent: '',
  addTags: false,
  tagIds: []
}

const triggerLabel: Record<AutomationTrigger, string> = {
  task_overdue: '任务逾期',
  task_status_changed: '任务状态变更',
  task_progress_changed: '任务进度变更',
  task_assignee_changed: '任务执行人变更'
}

const assigneeChangeTypeLabel: Record<AssigneeChangeType, string> = {
  added: '新增执行人',
  removed: '移除执行人'
}

const taskStatusLabel: Record<Status, string> = {
  pending: '待处理',
  queued: '排队中',
  processing: '进行中',
  reviewing: '审核中',
  completed: '已完成'
}

const statusLabel = {
  success: '成功',
  skipped: '跳过',
  failed: '失败'
}

const sourceLabel = {
  manual: '手动',
  scheduled: '定时',
  event: '事件'
}

const statusOptions: Status[] = ['pending', 'queued', 'processing', 'reviewing', 'completed']
const assigneeChangeTypeOptions: AssigneeChangeType[] = ['added', 'removed']
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]
const toggleStatus = (list: Status[], status: Status) => list.includes(status) ? list.filter((item) => item !== status) : [...list, status]
const toggleAssigneeChangeType = (list: AssigneeChangeType[], changeType: AssigneeChangeType) => list.includes(changeType) ? list.filter((item) => item !== changeType) : [...list, changeType]
const formatStatuses = (statuses?: Status[]) => statuses?.length ? statuses.map((status) => taskStatusLabel[status]).join('，') : '任意'
const formatAssigneeChangeTypes = (changeTypes?: AssigneeChangeType[]) => changeTypes?.length ? changeTypes.map((changeType) => assigneeChangeTypeLabel[changeType]).join('，') : '未设置'
const optionalProgress = (value: string) => value.trim() === '' ? undefined : Number(value)
const progressText = (min?: number, max?: number) => {
  if (min === undefined && max === undefined) return '任意'
  if (min !== undefined && max !== undefined) return `${min}% 至 ${max}%`
  if (min !== undefined) return `不低于 ${min}%`
  return `不高于 ${max}%`
}
const isEventTrigger = (trigger: AutomationTrigger) => trigger === 'task_status_changed' || trigger === 'task_progress_changed' || trigger === 'task_assignee_changed'

export function AutomationRulesPage() {
  const permissions = usePermissions()
  const canCreateRule = hasPermission('automations.create', permissions)
  const canUpdateRule = hasPermission('automations.update', permissions)
  const canDeleteRule = hasPermission('automations.delete', permissions)
  const canReadProjects = hasPermission('projects.read', permissions)
  const canReadTags = hasPermission('tags.read', permissions)
  const [items, setItems] = useState<AutomationRule[]>([])
  const [logs, setLogs] = useState<AutomationExecutionLog[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [enabledFilter, setEnabledFilter] = useState<'all' | 'enabled' | 'disabled'>('all')
  const [triggerFilter, setTriggerFilter] = useState<'all' | AutomationTrigger>('all')
  const [form, setForm] = useState<AutomationRuleForm>(initialForm)
  const [loading, setLoading] = useState(false)
  const [logLoading, setLogLoading] = useState(false)
  const [error, setError] = useState('')
  const [logError, setLogError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(enabledFilter !== 'all') + Number(triggerFilter !== 'all')

  const projectNameById = useMemo(() => new Map(projects.map((project) => [project.id, `${project.code} - ${project.name}`])), [projects])
  const tagNameById = useMemo(() => new Map(tags.map((tag) => [tag.id, tag.name])), [tags])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const isEnabled = enabledFilter === 'all' ? '' : enabledFilter === 'enabled'
      const trigger = triggerFilter === 'all' ? '' : triggerFilter
      const pageData = await fetchPage<AutomationRule>(
        '/automation-rules',
        { page, pageSize, keyword, trigger, isEnabled },
        { page, pageSize }
      )
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      setError(readApiError(loadError, '自动化规则加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadLogs = async () => {
    try {
      setLogLoading(true)
      setLogError('')
      const pageData = await fetchPage<AutomationExecutionLog>('/automation-rules/logs', { page: 1, pageSize: 8 }, { page: 1, pageSize: 8 }, { silent: true })
      setLogs(pageData.list)
    } catch (loadError) {
      setLogError(readApiError(loadError, '执行日志加载失败'))
      setLogs([])
    } finally {
      setLogLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, enabledFilter, triggerFilter])
  useEffect(() => { void loadLogs() }, [])

  useEffect(() => {
    if (!canReadProjects) {
      setProjects([])
      return
    }
    void fetchPage<Project>('/projects', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      .then((data) => setProjects(data.list))
      .catch(() => setProjects([]))
  }, [canReadProjects])

  useEffect(() => {
    if (!canReadTags) {
      setTags([])
      return
    }
    void fetchPage<Tag>('/tags', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      .then((data) => setTags(data.list))
      .catch(() => setTags([]))
  }, [canReadTags])

  const openCreateModal = () => {
    if (!canCreateRule) return
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openEditModal = (item: AutomationRule) => {
    if (!canUpdateRule) return
    setForm({
      id: item.id,
      trigger: item.trigger,
      name: item.name,
      isEnabled: item.isEnabled,
      overdueDays: item.conditions?.overdueDays ?? 1,
      projectIds: item.conditions?.projectIds || [],
      fromStatuses: item.conditions?.fromStatuses || [],
      toStatuses: item.conditions?.toStatuses || [],
      fromProgressMin: item.conditions?.fromProgressMin === undefined ? '' : String(item.conditions.fromProgressMin),
      fromProgressMax: item.conditions?.fromProgressMax === undefined ? '' : String(item.conditions.fromProgressMax),
      toProgressMin: item.conditions?.toProgressMin === undefined ? '' : String(item.conditions.toProgressMin),
      toProgressMax: item.conditions?.toProgressMax === undefined ? '' : String(item.conditions.toProgressMax),
      assigneeChangeTypes: item.conditions?.assigneeChangeTypes?.length ? item.conditions.assigneeChangeTypes : ['added', 'removed'],
      notifyAssignees: item.actions?.notifyAssignees ?? true,
      notifyProjectOwners: item.actions?.notifyProjectOwners ?? true,
      addComment: item.actions?.addComment ?? false,
      commentContent: item.actions?.commentContent || '',
      addTags: item.actions?.addTags ?? false,
      tagIds: item.actions?.tagIds || []
    })
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateRule) return
    if (!form.id && !canCreateRule) return
    const hasNotificationAction = form.notifyAssignees || form.notifyProjectOwners
    const hasCommentAction = isEventTrigger(form.trigger) && form.addComment
    const hasTagAction = form.addTags
    if (!hasNotificationAction && !hasCommentAction && !hasTagAction) {
      setFormError('至少选择一个动作')
      return
    }
    if (form.addTags && form.tagIds.length === 0) {
      setFormError('至少选择一个要添加的标签')
      return
    }
    if (form.trigger === 'task_status_changed' && form.fromStatuses.length === 0 && form.toStatuses.length === 0) {
      setFormError('至少选择一个状态条件')
      return
    }
    const progressValues = [form.fromProgressMin, form.fromProgressMax, form.toProgressMin, form.toProgressMax]
    if (form.trigger === 'task_progress_changed' && progressValues.every((value) => value.trim() === '')) {
      setFormError('至少填写一个进度条件')
      return
    }
    if (form.trigger === 'task_assignee_changed' && form.assigneeChangeTypes.length === 0) {
      setFormError('至少选择一种执行人变更类型')
      return
    }
    if (form.trigger === 'task_progress_changed') {
      const parsedProgressValues = progressValues
        .filter((value) => value.trim() !== '')
        .map(Number)
      if (parsedProgressValues.some((value) => !Number.isFinite(value) || value < 0 || value > 100)) {
        setFormError('进度条件必须在 0 到 100 之间')
        return
      }
      const fromMin = optionalProgress(form.fromProgressMin)
      const fromMax = optionalProgress(form.fromProgressMax)
      const toMin = optionalProgress(form.toProgressMin)
      const toMax = optionalProgress(form.toProgressMax)
      if ((fromMin !== undefined && fromMax !== undefined && fromMin > fromMax) || (toMin !== undefined && toMax !== undefined && toMin > toMax)) {
        setFormError('进度下限不能大于上限')
        return
      }
    }
    try {
      setSubmitting(true)
      setFormError('')
      const payload = {
        name: form.name,
        trigger: form.trigger,
        isEnabled: form.isEnabled,
        conditions: {
          overdueDays: Number(form.overdueDays || 0),
          projectIds: form.projectIds,
          fromStatuses: form.trigger === 'task_status_changed' ? form.fromStatuses : [],
          toStatuses: form.trigger === 'task_status_changed' ? form.toStatuses : [],
          fromProgressMin: form.trigger === 'task_progress_changed' ? optionalProgress(form.fromProgressMin) : undefined,
          fromProgressMax: form.trigger === 'task_progress_changed' ? optionalProgress(form.fromProgressMax) : undefined,
          toProgressMin: form.trigger === 'task_progress_changed' ? optionalProgress(form.toProgressMin) : undefined,
          toProgressMax: form.trigger === 'task_progress_changed' ? optionalProgress(form.toProgressMax) : undefined,
          assigneeChangeTypes: form.trigger === 'task_assignee_changed' ? form.assigneeChangeTypes : []
        },
        actions: {
          notifyAssignees: form.notifyAssignees,
          notifyProjectOwners: form.notifyProjectOwners,
          addComment: isEventTrigger(form.trigger) ? form.addComment : false,
          commentContent: isEventTrigger(form.trigger) ? form.commentContent.trim() : '',
          addTags: form.addTags,
          tagIds: form.addTags ? form.tagIds : []
        }
      }
      if (form.id) await api.put(`/automation-rules/${form.id}`, payload)
      else await api.post('/automation-rules', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存自动化规则失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const runRule = async (item: AutomationRule) => {
    if (!canUpdateRule) return
    try {
      setError('')
      await api.post(`/automation-rules/${item.id}/run`)
      await load()
      await loadLogs()
      window.dispatchEvent(new Event('notifications:changed'))
    } catch (runError) {
      setError(readApiError(runError, '执行自动化规则失败'))
    }
  }

  const onDelete = async (id: number) => {
    if (!canDeleteRule) return
    if (!confirm('确认删除该自动化规则？')) return
    try {
      await api.delete(`/automation-rules/${id}`)
      await load()
      await loadLogs()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除自动化规则失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="自动化筛选"
        activeCount={activeFilterCount}
        actions={canCreateRule ? <button className="btn" onClick={openCreateModal}>新增规则</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="自动化规则关键词搜索"
          value={keywordInput}
          placeholder="搜索规则名称"
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
        <select value={enabledFilter} aria-label="启用状态筛选" onChange={(event) => { setEnabledFilter(event.target.value as typeof enabledFilter); setPage(1) }}>
          <option value="all">全部状态</option>
          <option value="enabled">已启用</option>
          <option value="disabled">已停用</option>
        </select>
        <select value={triggerFilter} aria-label="触发器筛选" onChange={(event) => { setTriggerFilter(event.target.value as typeof triggerFilter); setPage(1) }}>
          <option value="all">全部触发器</option>
          <option value="task_overdue">任务逾期</option>
          <option value="task_status_changed">任务状态变更</option>
          <option value="task_progress_changed">任务进度变更</option>
          <option value="task_assignee_changed">任务执行人变更</option>
        </select>
        <div className="row-actions">
          <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
        </div>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无自动化规则" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>ID</th><th>规则名称</th><th>触发器</th><th>状态</th><th>条件</th><th>动作</th><th>最近执行</th><th>操作</th>
          </tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="ID">{item.id}</td>
                <td data-label="规则名称">{item.name}</td>
                <td data-label="触发器">{triggerLabel[item.trigger]}</td>
                <td data-label="状态">{item.isEnabled ? '已启用' : '已停用'}</td>
                <td data-label="条件">
                  {item.trigger === 'task_overdue'
                    ? `逾期 ${item.conditions?.overdueDays ?? 1} 天`
                    : item.trigger === 'task_status_changed'
                      ? `从 ${formatStatuses(item.conditions?.fromStatuses)} 到 ${formatStatuses(item.conditions?.toStatuses)}`
                      : item.trigger === 'task_progress_changed'
                        ? `从 ${progressText(item.conditions?.fromProgressMin, item.conditions?.fromProgressMax)} 到 ${progressText(item.conditions?.toProgressMin, item.conditions?.toProgressMax)}`
                        : formatAssigneeChangeTypes(item.conditions?.assigneeChangeTypes)}
                  {item.conditions?.projectIds?.length ? `；项目 ${item.conditions.projectIds.map((id) => projectNameById.get(id) || id).join('，')}` : ''}
                </td>
                <td data-label="动作">
                  {[
                    item.actions?.notifyAssignees ? '通知执行人' : '',
                    item.actions?.notifyProjectOwners ? '通知项目负责人' : '',
                    item.actions?.addComment ? '添加评论' : '',
                    item.actions?.addTags ? `添加标签${item.actions.tagIds?.length ? `（${item.actions.tagIds.map((id) => tagNameById.get(id) || id).join('，')}）` : ''}` : ''
                  ].filter(Boolean).join('，') || '-'}
                </td>
                <td data-label="最近执行">{formatDateTime(item.lastRunAt)}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    {canUpdateRule && <button className="btn secondary" onClick={() => { void runRule(item) }}>执行</button>}
                    {canUpdateRule && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                    {canDeleteRule && <button className="btn danger" onClick={() => { void onDelete(item.id) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <div className="card">
        <div className="row-actions">
          <h3>执行日志</h3>
          <button className="btn secondary" onClick={() => { void loadLogs() }}>刷新</button>
        </div>
        <DataState loading={logLoading} error={logError} empty={!logLoading && !logError && logs.length === 0} emptyText="暂无执行日志" onRetry={() => { void loadLogs() }} />
        {!logLoading && !logError && logs.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>时间</th><th>规则</th><th>来源</th><th>状态</th><th>匹配</th><th>动作</th><th>结果</th>
          </tr></thead><tbody>
            {logs.map((item) => (
              <tr key={item.id}>
                <td data-label="时间">{formatDateTime(item.createdAt)}</td>
                <td data-label="规则">{item.rule?.name || item.ruleId}</td>
                <td data-label="来源">{sourceLabel[item.runSource] || item.runSource}</td>
                <td data-label="状态">{statusLabel[item.status] || item.status}</td>
                <td data-label="匹配">{item.matchedCount}</td>
                <td data-label="动作">{item.actionCount}</td>
                <td data-label="结果">{item.message || '-'}</td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      <Modal open={modalOpen} title={form.id ? '编辑自动化规则' : '新增自动化规则'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="automation-name">规则名称</label>
          <input id="automation-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="automation-trigger">触发器</label>
          <select
            id="automation-trigger"
            value={form.trigger}
            onChange={(event) => {
              const nextTrigger = event.target.value as AutomationTrigger
              setForm((prev) => ({
                ...prev,
                trigger: nextTrigger,
                fromStatuses: [],
                toStatuses: [],
                fromProgressMin: '',
                fromProgressMax: '',
                toProgressMin: '',
                toProgressMax: '',
                assigneeChangeTypes: nextTrigger === 'task_assignee_changed' ? ['added', 'removed'] : [],
                addComment: false,
                commentContent: ''
              }))
            }}
          >
            <option value="task_overdue">任务逾期</option>
            <option value="task_status_changed">任务状态变更</option>
            <option value="task_progress_changed">任务进度变更</option>
            <option value="task_assignee_changed">任务执行人变更</option>
          </select>
          <label htmlFor="automation-enabled">启用状态</label>
          <select id="automation-enabled" value={form.isEnabled ? 'enabled' : 'disabled'} onChange={(event) => setForm((prev) => ({ ...prev, isEnabled: event.target.value === 'enabled' }))}>
            <option value="enabled">已启用</option>
            <option value="disabled">已停用</option>
          </select>
          {form.trigger === 'task_overdue' && (
            <>
              <label className="required-label" htmlFor="automation-overdue-days">逾期天数</label>
              <input id="automation-overdue-days" type="number" min={0} value={form.overdueDays} onChange={(event) => setForm((prev) => ({ ...prev, overdueDays: Number(event.target.value) }))} required />
            </>
          )}
          {form.trigger === 'task_status_changed' && (
            <>
              <label htmlFor="automation-from-statuses">变更前状态</label>
              <div id="automation-from-statuses" className="multi-checklist">
                {statusOptions.map((status) => (
                  <label key={status} className="multi-check-item">
                    <input type="checkbox" checked={form.fromStatuses.includes(status)} onChange={() => setForm((prev) => ({ ...prev, fromStatuses: toggleStatus(prev.fromStatuses, status) }))} />
                    <span>{taskStatusLabel[status]}</span>
                  </label>
                ))}
              </div>
              <label htmlFor="automation-to-statuses">变更后状态</label>
              <div id="automation-to-statuses" className="multi-checklist">
                {statusOptions.map((status) => (
                  <label key={status} className="multi-check-item">
                    <input type="checkbox" checked={form.toStatuses.includes(status)} onChange={() => setForm((prev) => ({ ...prev, toStatuses: toggleStatus(prev.toStatuses, status) }))} />
                    <span>{taskStatusLabel[status]}</span>
                  </label>
                ))}
              </div>
            </>
          )}
          {form.trigger === 'task_progress_changed' && (
            <>
              <label htmlFor="automation-from-progress-min">变更前进度下限</label>
              <input id="automation-from-progress-min" type="number" min={0} max={100} value={form.fromProgressMin} onChange={(event) => setForm((prev) => ({ ...prev, fromProgressMin: event.target.value }))} />
              <label htmlFor="automation-from-progress-max">变更前进度上限</label>
              <input id="automation-from-progress-max" type="number" min={0} max={100} value={form.fromProgressMax} onChange={(event) => setForm((prev) => ({ ...prev, fromProgressMax: event.target.value }))} />
              <label htmlFor="automation-to-progress-min">变更后进度下限</label>
              <input id="automation-to-progress-min" type="number" min={0} max={100} value={form.toProgressMin} onChange={(event) => setForm((prev) => ({ ...prev, toProgressMin: event.target.value }))} />
              <label htmlFor="automation-to-progress-max">变更后进度上限</label>
              <input id="automation-to-progress-max" type="number" min={0} max={100} value={form.toProgressMax} onChange={(event) => setForm((prev) => ({ ...prev, toProgressMax: event.target.value }))} />
            </>
          )}
          {form.trigger === 'task_assignee_changed' && (
            <>
              <label htmlFor="automation-assignee-change-types">执行人变更类型</label>
              <div id="automation-assignee-change-types" className="multi-checklist">
                {assigneeChangeTypeOptions.map((changeType) => (
                  <label key={changeType} className="multi-check-item">
                    <input type="checkbox" checked={form.assigneeChangeTypes.includes(changeType)} onChange={() => setForm((prev) => ({ ...prev, assigneeChangeTypes: toggleAssigneeChangeType(prev.assigneeChangeTypes, changeType) }))} />
                    <span>{assigneeChangeTypeLabel[changeType]}</span>
                  </label>
                ))}
              </div>
            </>
          )}
          {canReadProjects && (
            <>
              <label htmlFor="automation-projects">项目范围</label>
              <div id="automation-projects" className="multi-checklist">
                {projects.map((project) => (
                  <label key={project.id} className="multi-check-item">
                    <input type="checkbox" checked={form.projectIds.includes(project.id)} onChange={() => setForm((prev) => ({ ...prev, projectIds: toggleNumber(prev.projectIds, project.id) }))} />
                    <span>{project.code} - {project.name}</span>
                  </label>
                ))}
                {projects.length === 0 && <p className="inline-tip">不限制项目范围</p>}
              </div>
            </>
          )}
          <label htmlFor="automation-actions">通知对象</label>
          <div id="automation-actions" className="multi-checklist">
            <label className="multi-check-item">
              <input type="checkbox" checked={form.notifyAssignees} onChange={() => setForm((prev) => ({ ...prev, notifyAssignees: !prev.notifyAssignees }))} />
              <span>任务执行人</span>
            </label>
            <label className="multi-check-item">
              <input type="checkbox" checked={form.notifyProjectOwners} onChange={() => setForm((prev) => ({ ...prev, notifyProjectOwners: !prev.notifyProjectOwners }))} />
              <span>项目负责人</span>
            </label>
          </div>
          {canReadTags && (
            <>
              <label htmlFor="automation-tag-action">标签动作</label>
              <div id="automation-tag-action" className="multi-checklist">
                <label className="multi-check-item">
                  <input type="checkbox" checked={form.addTags} onChange={() => setForm((prev) => ({ ...prev, addTags: !prev.addTags, tagIds: prev.addTags ? [] : prev.tagIds }))} />
                  <span>添加标签</span>
                </label>
              </div>
              {form.addTags && (
                <>
                  <label htmlFor="automation-tags">选择标签</label>
                  <div id="automation-tags" className="multi-checklist">
                    {tags.map((tag) => (
                      <label key={tag.id} className="multi-check-item">
                        <input type="checkbox" checked={form.tagIds.includes(tag.id)} onChange={() => setForm((prev) => ({ ...prev, tagIds: toggleNumber(prev.tagIds, tag.id) }))} />
                        <span>{tag.name}</span>
                      </label>
                    ))}
                    {tags.length === 0 && <p className="inline-tip">暂无可选标签</p>}
                  </div>
                </>
              )}
            </>
          )}
          {isEventTrigger(form.trigger) && (
            <>
              <label htmlFor="automation-comment-action">自动评论</label>
              <div id="automation-comment-action" className="multi-checklist">
                <label className="multi-check-item">
                  <input type="checkbox" checked={form.addComment} onChange={() => setForm((prev) => ({ ...prev, addComment: !prev.addComment }))} />
                  <span>添加评论</span>
                </label>
              </div>
              {form.addComment && (
                <>
                  <label htmlFor="automation-comment-content">评论内容</label>
                  <textarea id="automation-comment-content" value={form.commentContent} onChange={(event) => setForm((prev) => ({ ...prev, commentContent: event.target.value }))} rows={3} />
                </>
              )}
            </>
          )}
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateRule : !canCreateRule)}>{submitting ? '保存中...' : '保存规则'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
