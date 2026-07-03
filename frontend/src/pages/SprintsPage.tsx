import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { SearchableMultiSelect } from '../components/SearchableMultiSelect'
import { Sprint, SprintStatus, Task } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface SprintForm {
  id?: number
  name: string
  goal: string
  status: SprintStatus
  startAt: string
  endAt: string
  capacityHours: number
}

const sprintStatusLabel: Record<SprintStatus, string> = {
  planned: '计划中',
  active: '进行中',
  closed: '已关闭'
}

const initialForm: SprintForm = {
  name: '',
  goal: '',
  status: 'planned',
  startAt: '',
  endAt: '',
  capacityHours: 0
}

const toLocalInputValue = (value?: string) => value ? value.slice(0, 16) : ''
const toRequestTime = (value: string) => value ? new Date(value).toISOString() : ''
const formatPercent = (value?: number) => `${Number(value || 0).toFixed(0)}%`
const taskOptionLabel = (task: Task) => `${task.taskNo || `#${task.id}`}｜${task.title}`

export function SprintsPage() {
  const permissions = usePermissions()
  const canCreateSprint = hasPermission('sprints.create', permissions)
  const canUpdateSprint = hasPermission('sprints.update', permissions)
  const canDeleteSprint = hasPermission('sprints.delete', permissions)
  const canReadTasks = hasPermission('tasks.read', permissions)
  const [items, setItems] = useState<Sprint[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [form, setForm] = useState<SprintForm>(initialForm)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [activeSprint, setActiveSprint] = useState<Sprint | null>(null)
  const [taskOptions, setTaskOptions] = useState<Task[]>([])
  const [taskKeyword, setTaskKeyword] = useState('')
  const [taskListKeyword, setTaskListKeyword] = useState('')
  const [selectedTaskIds, setSelectedTaskIds] = useState<string[]>([])
  const [taskSubmitting, setTaskSubmitting] = useState(false)
  const [taskActionError, setTaskActionError] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(statusFilter))

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const sprintPage = await fetchPage<Sprint>('/sprints', { page, pageSize, keyword, status: statusFilter }, { page, pageSize })
      setItems(sprintPage.list)
      setTotal(sprintPage.total)
    } catch (loadError) {
      setError(readApiError(loadError, '迭代列表加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadDetail = async (id: number) => {
    try {
      setDetailLoading(true)
      setDetailError('')
      const sprint = await fetchData<Sprint>(`/sprints/${id}`)
      setActiveSprint(sprint)
    } catch (detailLoadError) {
      setDetailError(readApiError(detailLoadError, '迭代任务加载失败'))
      setActiveSprint((prev) => prev ? { ...prev, tasks: [] } : prev)
    } finally {
      setDetailLoading(false)
    }
  }

  const loadTaskOptions = async () => {
    if (!canReadTasks) {
      setTaskOptions([])
      return
    }
    try {
      const taskPage = await fetchPage<Task>(
        '/tasks',
        { page: 1, pageSize: 100, keyword: taskKeyword.trim(), searchFields: 'taskNo,title,description' },
        { page: 1, pageSize: 100 },
        { silent: true }
      )
      setTaskOptions(taskPage.list)
    } catch {
      setTaskOptions([])
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, statusFilter])

  useEffect(() => {
    if (!detailOpen) return
    const timer = window.setTimeout(() => { void loadTaskOptions() }, 250)
    return () => window.clearTimeout(timer)
  }, [detailOpen, taskKeyword, canReadTasks])

  const availableTaskOptions = useMemo(() => {
    const existing = new Set((activeSprint?.tasks || []).map((task) => task.id))
    return taskOptions
      .filter((task) => !existing.has(task.id))
      .map((task) => ({
        value: String(task.id),
        label: taskOptionLabel(task),
        keywords: [task.taskNo, task.title, task.description || '', task.projectName || '']
      }))
  }, [activeSprint, taskOptions])

  const visibleSprintTasks = useMemo(() => {
    const tasks = activeSprint?.tasks || []
    const normalizedKeyword = taskListKeyword.trim().toLowerCase()
    if (!normalizedKeyword) return tasks
    return tasks.filter((task) => {
      const haystack = [
        task.taskNo,
        task.title,
        task.description || '',
        task.status,
        task.projectName || '',
        task.projectCode || ''
      ].join(' ').toLowerCase()
      return haystack.includes(normalizedKeyword)
    })
  }, [activeSprint, taskListKeyword])

  const openCreateModal = () => {
    if (!canCreateSprint) return
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openEditModal = (item: Sprint) => {
    if (!canUpdateSprint) return
    setForm({
      id: item.id,
      name: item.name,
      goal: item.goal || '',
      status: item.status || 'planned',
      startAt: toLocalInputValue(item.startAt),
      endAt: toLocalInputValue(item.endAt),
      capacityHours: Number(item.capacityHours || 0)
    })
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openDetailModal = (item: Sprint) => {
    setActiveSprint(item)
    setSelectedTaskIds([])
    setTaskKeyword('')
    setTaskListKeyword('')
    setTaskActionError('')
    setDetailOpen(true)
    void loadDetail(item.id)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateSprint) return
    if (!form.id && !canCreateSprint) return
    if (form.startAt && form.endAt && new Date(form.startAt) > new Date(form.endAt)) {
      setFormError('结束时间必须晚于开始时间')
      return
    }
    if (form.capacityHours < 0) {
      setFormError('迭代容量不能小于 0')
      return
    }
    const payload = {
      name: form.name.trim(),
      goal: form.goal.trim(),
      status: form.status,
      startAt: toRequestTime(form.startAt),
      endAt: toRequestTime(form.endAt),
      capacityHours: Number(form.capacityHours || 0)
    }
    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) await api.put(`/sprints/${form.id}`, payload)
      else await api.post('/sprints', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存迭代失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (item: Sprint) => {
    if (!canDeleteSprint) return
    if (!confirm(`确认删除迭代「${item.name}」？`)) return
    try {
      await api.delete(`/sprints/${item.id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除迭代失败'))
    }
  }

  const addTasks = async () => {
    if (!activeSprint || !canUpdateSprint || selectedTaskIds.length === 0) return
    try {
      setTaskSubmitting(true)
      setTaskActionError('')
      const taskIds = selectedTaskIds.map((value) => Number(value)).filter((value) => Number.isFinite(value) && value > 0)
      await api.post(`/sprints/${activeSprint.id}/tasks`, { taskIds })
      setSelectedTaskIds([])
      await loadDetail(activeSprint.id)
      await load()
    } catch (taskError) {
      setTaskActionError(readApiError(taskError, '加入迭代失败'))
    } finally {
      setTaskSubmitting(false)
    }
  }

  const removeTask = async (task: Task) => {
    if (!activeSprint || !canUpdateSprint) return
    try {
      setTaskSubmitting(true)
      setTaskActionError('')
      await api.delete(`/sprints/${activeSprint.id}/tasks/${task.id}`)
      await loadDetail(activeSprint.id)
      await load()
    } catch (taskError) {
      setTaskActionError(readApiError(taskError, '移出迭代失败'))
    } finally {
      setTaskSubmitting(false)
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="迭代筛选"
        activeCount={activeFilterCount}
        actions={canCreateSprint ? <button className="btn secondary" onClick={openCreateModal}>新增迭代</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="迭代关键词搜索"
          value={keywordInput}
          placeholder="搜索迭代名称/目标"
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
        <select aria-label="迭代状态筛选" value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value); setPage(1) }}>
          <option value="">全部状态</option>
          {(Object.keys(sprintStatusLabel) as SprintStatus[]).map((status) => <option key={status} value={status}>{sprintStatusLabel[status]}</option>)}
        </select>
        <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无迭代数据" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr><th>名称</th><th>状态</th><th>周期</th><th>容量</th><th>完成率</th><th>任务</th><th>操作</th></tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="名称"><strong>{item.name}</strong><p className="table-subtext">{item.goal || '-'}</p></td>
                <td data-label="状态">{sprintStatusLabel[item.status] || item.status}</td>
                <td data-label="周期">{formatDateTime(item.startAt)} - {formatDateTime(item.endAt)}</td>
                <td data-label="容量">{Number(item.capacityHours || 0)}h</td>
                <td data-label="完成率">{formatPercent(item.completionRate)}</td>
                <td data-label="任务">{Number(item.completedTaskCount || 0)} / {Number(item.taskCount || 0)}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    <button className="btn secondary" onClick={() => openDetailModal(item)}>任务范围</button>
                    {canUpdateSprint && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                    {canDeleteSprint && <button className="btn danger" onClick={() => { void onDelete(item) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑迭代' : '新增迭代'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="sprint-name">名称</label>
          <input id="sprint-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="sprint-status">状态</label>
          <select id="sprint-status" value={form.status} onChange={(event) => setForm((prev) => ({ ...prev, status: event.target.value as SprintStatus }))}>
            {(Object.keys(sprintStatusLabel) as SprintStatus[]).map((status) => <option key={status} value={status}>{sprintStatusLabel[status]}</option>)}
          </select>
          <label htmlFor="sprint-goal">目标</label>
          <textarea id="sprint-goal" rows={3} value={form.goal} onChange={(event) => setForm((prev) => ({ ...prev, goal: event.target.value }))} />
          <label htmlFor="sprint-start">开始时间</label>
          <DateTimeQuickField inputId="sprint-start" value={form.startAt} onChange={(value) => setForm((prev) => ({ ...prev, startAt: value }))} />
          <label htmlFor="sprint-end">结束时间</label>
          <DateTimeQuickField inputId="sprint-end" value={form.endAt} onChange={(value) => setForm((prev) => ({ ...prev, endAt: value }))} />
          <label htmlFor="sprint-capacity">容量（小时）</label>
          <input id="sprint-capacity" type="number" min={0} step={0.25} value={form.capacityHours} onChange={(event) => setForm((prev) => ({ ...prev, capacityHours: Number(event.target.value) }))} />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>

      <Modal open={detailOpen} title={activeSprint ? `迭代任务：${activeSprint.name}` : '迭代任务'} onClose={() => setDetailOpen(false)}>
        <div className="detail-grid sprint-detail-grid">
          <section className="detail-section sprint-summary-section">
            <h4>迭代概况</h4>
            <div className="detail-columns">
              <div><strong>状态：</strong>{activeSprint ? sprintStatusLabel[activeSprint.status] : '-'}</div>
              <div><strong>周期：</strong>{formatDateTime(activeSprint?.startAt)} - {formatDateTime(activeSprint?.endAt)}</div>
              <div><strong>容量：</strong>{Number(activeSprint?.capacityHours || 0)}h</div>
              <div><strong>完成率：</strong>{formatPercent(activeSprint?.completionRate)}</div>
              <div><strong>任务：</strong>{Number(activeSprint?.completedTaskCount || 0)} / {Number(activeSprint?.taskCount || 0)}</div>
            </div>
            {activeSprint?.goal && <div className="detail-description-card"><strong>目标</strong><p>{activeSprint.goal}</p></div>}
          </section>
          <div className="sprint-task-workspace">
            {canUpdateSprint && canReadTasks && (
              <section className="detail-section sprint-task-add-section">
                <h4>加入任务</h4>
                <div className="sprint-task-add-grid">
                  <SearchableMultiSelect
                    className="sprint-task-picker"
                    ariaLabel="选择加入迭代的任务"
                    values={selectedTaskIds}
                    options={availableTaskOptions}
                    onChange={setSelectedTaskIds}
                    onSearchChange={setTaskKeyword}
                    placeholder="搜索待加入任务编号/标题/描述"
                    noResultsText={taskKeyword.trim() ? '没有匹配的待加入任务' : '没有可加入的任务'}
                    summaryNoun="任务"
                  />
                  <button className="btn sprint-task-add-button" disabled={taskSubmitting || selectedTaskIds.length === 0} onClick={() => { void addTasks() }}>
                    {taskSubmitting ? '处理中...' : '加入迭代'}
                  </button>
                </div>
                {taskActionError && <p className="error">{taskActionError}</p>}
              </section>
            )}
            <section className="detail-section sprint-task-list-section">
              <h4>任务列表</h4>
              {(taskListKeyword || (activeSprint?.tasks || []).length > 0) && (
                <SearchField className="sprint-task-list-search" aria-label="搜索迭代任务列表" value={taskListKeyword} placeholder="搜索已加入任务编号/标题/状态/项目" onChange={setTaskListKeyword} />
              )}
              <DataState loading={detailLoading} error={detailError} empty={!detailLoading && !detailError && visibleSprintTasks.length === 0} emptyText={taskListKeyword.trim() ? '没有匹配的任务' : '暂无可见任务'} onRetry={() => { if (activeSprint) void loadDetail(activeSprint.id) }} />
              {!detailLoading && !detailError && visibleSprintTasks.length > 0 && (
                <div className="sprint-task-table-wrap">
                  <table className="responsive-table"><thead><tr><th>任务</th><th>状态</th><th>进度</th><th>项目</th><th>操作</th></tr></thead><tbody>
                    {visibleSprintTasks.map((task) => (
                      <tr key={task.id}>
                        <td data-label="任务"><strong>{task.taskNo || `#${task.id}`}</strong><p className="table-subtext">{task.title}</p></td>
                        <td data-label="状态">{task.status}</td>
                        <td data-label="进度">{Number(task.progress || 0)}%</td>
                        <td data-label="项目">{task.projectName || task.projectId || '-'}</td>
                        <td data-label="操作">
                          {canUpdateSprint && <button className="btn danger" disabled={taskSubmitting} onClick={() => { void removeTask(task) }}>移出</button>}
                        </td>
                      </tr>
                    ))}
                  </tbody></table>
                </div>
              )}
            </section>
          </div>
        </div>
      </Modal>
    </section>
  )
}
