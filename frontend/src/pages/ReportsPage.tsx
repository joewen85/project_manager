import { FormEvent, useEffect, useMemo, useState } from 'react'
import { BarChart3, Download, Play, Printer, RefreshCcw, Send, Settings } from 'lucide-react'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { RemoteProjectSelect } from '../components/RemoteProjectSelect'
import { ReportResultChart } from '../components/ReportResultChart'
import { SearchField } from '../components/SearchField'
import { STATUS_META, STATUS_ORDER } from '../constants/status'
import { Department, ReportRunResult, ReportSubscription, SavedReport, SavedReportType, Status, User } from '../types'
import { usePermissions } from '../hooks/usePermissions'

interface ReportForm {
  id?: number
  name: string
  description: string
  type: SavedReportType
  projectId: string
  departmentId: string
  ownerId: string
  dateFrom: string
  dateTo: string
  keyword: string
  statuses: Status[]
  displayMode: string
}

interface SubscriptionForm {
  isEnabled: boolean
  weekday: number
  hour: number
  channels: string[]
  recipientUserIds: string
}

const reportTypeLabel: Record<SavedReportType, string> = {
  project_health: '项目健康',
  member_workload: '成员负载',
  task_status: '任务状态',
  task_throughput: '任务吞吐',
  overdue_trend: '逾期趋势',
  department_distribution: '部门分布'
}

const reportTypeHint: Record<SavedReportType, string> = {
  project_health: '健康度、逾期、风险和问题',
  member_workload: '成员容量、估算工时和过载',
  task_status: '按状态统计当前任务分布',
  task_throughput: '按日期对比新增与完成任务',
  overdue_trend: '按到期日查看逾期积压',
  department_distribution: '按部门汇总项目和任务'
}

const weekdayOptions = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const channelOptions = [
  { value: 'in_app', label: '站内' },
  { value: 'email', label: '邮件' },
  { value: 'wecom', label: '企业微信' },
  { value: 'dingtalk', label: '钉钉' },
  { value: 'feishu', label: '飞书' }
]

const initialForm: ReportForm = {
  name: '',
  description: '',
  type: 'project_health',
  projectId: '',
  departmentId: '',
  ownerId: '',
  dateFrom: '',
  dateTo: '',
  keyword: '',
  statuses: [],
  displayMode: 'table'
}

const initialSubscriptionForm: SubscriptionForm = {
  isEnabled: true,
  weekday: 1,
  hour: 9,
  channels: ['in_app'],
  recipientUserIds: ''
}

const normalizeReports = (value: SavedReport | null): ReportForm => ({
  id: value?.id,
  name: value?.name || '',
  description: value?.description || '',
  type: value?.type || 'project_health',
  projectId: value?.filters?.projectId ? String(value.filters.projectId) : '',
  departmentId: value?.filters?.departmentId ? String(value.filters.departmentId) : '',
  ownerId: value?.filters?.ownerId ? String(value.filters.ownerId) : '',
  dateFrom: value?.filters?.dateFrom || '',
  dateTo: value?.filters?.dateTo || '',
  keyword: value?.filters?.keyword || '',
  statuses: value?.filters?.statuses || [],
  displayMode: value?.chartConfig?.displayMode || 'table'
})

const subscriptionToForm = (value?: ReportSubscription): SubscriptionForm => ({
  isEnabled: value?.isEnabled ?? true,
  weekday: Number.isFinite(value?.weekday) ? Number(value?.weekday) : 1,
  hour: Number.isFinite(value?.hour) ? Number(value?.hour) : 9,
  channels: Array.isArray(value?.channels) && value.channels.length > 0 ? value.channels : ['in_app'],
  recipientUserIds: (value?.recipientUserIds || []).join(',')
})

const parseRecipientIds = (value: string) => value
  .split(',')
  .map((item) => Number(item.trim()))
  .filter((item) => Number.isFinite(item) && item > 0)

const formatGeneratedAt = (value?: string) => {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

const formatCell = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '-'
  return String(value)
}

export function ReportsPage() {
  const permissions = usePermissions()
  const canCreate = hasPermission('reports.create', permissions)
  const canUpdate = hasPermission('reports.update', permissions)
  const canDelete = hasPermission('reports.delete', permissions)
  const canReadDepartments = hasPermission('system.departments.read', permissions)
  const canReadUsers = hasPermission('system.users.read', permissions)

  const [reports, setReports] = useState<SavedReport[]>([])
  const [keyword, setKeyword] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [form, setForm] = useState<ReportForm>(initialForm)
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [departments, setDepartments] = useState<Department[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [activeReport, setActiveReport] = useState<SavedReport>()
  const [result, setResult] = useState<ReportRunResult>()
  const [resultLoading, setResultLoading] = useState(false)
  const [resultError, setResultError] = useState('')
  const [exporting, setExporting] = useState(false)
  const [subscription, setSubscription] = useState<ReportSubscription>()
  const [subscriptionForm, setSubscriptionForm] = useState<SubscriptionForm>(initialSubscriptionForm)
  const [subscriptionError, setSubscriptionError] = useState('')
  const [subscriptionSaving, setSubscriptionSaving] = useState(false)
  const [sending, setSending] = useState(false)

  const loadReports = async () => {
    try {
      setLoading(true)
      setError('')
      const data = await fetchPage<SavedReport>('/insights/reports', { page, pageSize, keyword, type: typeFilter }, { page, pageSize })
      setReports(data.list)
      setTotal(data.total)
    } catch (loadError) {
      setError(readApiError(loadError, '报表列表加载失败'))
      setReports([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadOptions = async () => {
    const [departmentPage, userPage] = await Promise.all([
      canReadDepartments
        ? fetchPage<Department>('/system/departments', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 100 }))
        : Promise.resolve({ list: [] as Department[], total: 0, page: 1, pageSize: 100 }),
      canReadUsers
        ? fetchPage<User>('/system/users', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 100 }))
        : Promise.resolve({ list: [] as User[], total: 0, page: 1, pageSize: 100 })
    ])
    setDepartments(departmentPage.list)
    setUsers(userPage.list)
  }

  useEffect(() => { void loadReports() }, [page, pageSize, keyword, typeFilter])
  useEffect(() => { void loadOptions() }, [canReadDepartments, canReadUsers])

  const reportCounts = useMemo(() => {
    const counts = new Map<SavedReportType, number>()
    reports.forEach((item) => counts.set(item.type, (counts.get(item.type) || 0) + 1))
    return counts
  }, [reports])

  const activeRows = result?.rows || []
  const activeColumns = result?.columns || []
  // Prefer the freshly-run result's config, fall back to the saved report; default to table.
  const displayMode = result?.chartConfig?.displayMode || activeReport?.chartConfig?.displayMode || 'table'

  const openCreate = () => {
    setForm(initialForm)
    setFormError('')
    setModalOpen(true)
  }

  const openEdit = (item: SavedReport) => {
    setForm(normalizeReports(item))
    setFormError('')
    setModalOpen(true)
  }

  const runReport = async (item: SavedReport) => {
    try {
      setActiveReport(item)
      setResultLoading(true)
      setResultError('')
      setSubscriptionError('')
      const [reportResult, subscriptionResult] = await Promise.all([
        fetchData<ReportRunResult>(`/insights/reports/${item.id}/run`),
        fetchData<ReportSubscription>(`/insights/reports/${item.id}/subscription`, undefined, { silent: true }).catch(() => undefined)
      ])
      setResult(reportResult)
      setSubscription(subscriptionResult)
      setSubscriptionForm(subscriptionToForm(subscriptionResult))
    } catch (loadError) {
      setResult(undefined)
      setSubscription(undefined)
      setSubscriptionForm(initialSubscriptionForm)
      setResultError(readApiError(loadError, '报表运行失败'))
    } finally {
      setResultLoading(false)
    }
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdate) return
    if (!form.id && !canCreate) return
    const payload = {
      name: form.name.trim(),
      description: form.description.trim(),
      type: form.type,
      filters: {
        projectId: form.projectId ? Number(form.projectId) : 0,
        departmentId: form.departmentId ? Number(form.departmentId) : 0,
        ownerId: form.ownerId ? Number(form.ownerId) : 0,
        dateFrom: form.dateFrom,
        dateTo: form.dateTo,
        keyword: form.keyword.trim(),
        statuses: form.statuses
      },
      chartConfig: {
        displayMode: form.displayMode
      }
    }
    try {
      setSubmitting(true)
      setFormError('')
      let saved: SavedReport
      if (form.id) {
        const res = await api.put<SavedReport>(`/insights/reports/${form.id}`, payload)
        saved = res.data
      } else {
        const res = await api.post<SavedReport>('/insights/reports', payload)
        saved = res.data
      }
      setModalOpen(false)
      setForm(initialForm)
      await loadReports()
      await runReport(saved)
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存报表失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const deleteReport = async (item: SavedReport) => {
    if (!canDelete) return
    if (!window.confirm(`确认删除报表「${item.name}」？`)) return
    try {
      await api.delete(`/insights/reports/${item.id}`)
      if (activeReport?.id === item.id) {
        setActiveReport(undefined)
        setResult(undefined)
        setSubscription(undefined)
      }
      await loadReports()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除报表失败'))
    }
  }

  const exportCSV = async (item: SavedReport) => {
    try {
      setExporting(true)
      const res = await api.get<Blob>(`/insights/reports/${item.id}/export.csv`, { responseType: 'blob' })
      const url = window.URL.createObjectURL(res.data)
      const link = document.createElement('a')
      link.href = url
      link.download = `${item.name || 'report'}.csv`
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
    } catch (exportError) {
      setResultError(readApiError(exportError, 'CSV 导出失败'))
    } finally {
      setExporting(false)
    }
  }

  const saveSubscription = async () => {
    if (!activeReport || !canUpdate) return
    try {
      setSubscriptionSaving(true)
      setSubscriptionError('')
      const res = await api.put<ReportSubscription>(`/insights/reports/${activeReport.id}/subscription`, {
        isEnabled: subscriptionForm.isEnabled,
        schedule: 'weekly',
        weekday: subscriptionForm.weekday,
        hour: subscriptionForm.hour,
        channels: subscriptionForm.channels,
        recipientUserIds: parseRecipientIds(subscriptionForm.recipientUserIds)
      })
      setSubscription(res.data)
      setSubscriptionForm(subscriptionToForm(res.data))
    } catch (saveError) {
      setSubscriptionError(readApiError(saveError, '保存订阅失败'))
    } finally {
      setSubscriptionSaving(false)
    }
  }

  const sendNow = async () => {
    if (!activeReport || !canUpdate) return
    try {
      setSending(true)
      setSubscriptionError('')
      const res = await api.post<ReportSubscription>(`/insights/reports/${activeReport.id}/subscription/run`)
      setSubscription(res.data)
      setSubscriptionForm(subscriptionToForm(res.data))
    } catch (sendError) {
      setSubscriptionError(readApiError(sendError, '发送周报失败'))
    } finally {
      setSending(false)
    }
  }

  const toggleStatus = (status: Status) => {
    setForm((prev) => ({
      ...prev,
      statuses: prev.statuses.includes(status) ? prev.statuses.filter((item) => item !== status) : [...prev.statuses, status]
    }))
  }

  const toggleChannel = (channel: string) => {
    setSubscriptionForm((prev) => ({
      ...prev,
      channels: prev.channels.includes(channel) ? prev.channels.filter((item) => item !== channel) : [...prev.channels, channel]
    }))
  }

  return (
    <section className="page-section reports-page">
      <div className="report-preview-grid">
        {(Object.keys(reportTypeLabel) as SavedReportType[]).slice(0, 5).map((type) => (
          <article key={type} className="card report-preview-card">
            <div className="report-preview-title"><BarChart3 size={18} /><h3>{reportTypeLabel[type]}</h3></div>
            <strong>{reportCounts.get(type) || 0}</strong>
            <span>{reportTypeHint[type]}</span>
          </article>
        ))}
      </div>

      <div className="card report-filter-bar">
        <SearchField aria-label="搜索保存报表" placeholder="搜索报表名称/描述" value={keyword} onChange={(value) => { setKeyword(value); setPage(1) }} />
        <select value={typeFilter} onChange={(event) => { setTypeFilter(event.target.value); setPage(1) }} aria-label="报表类型筛选">
          <option value="">全部类型</option>
          {(Object.keys(reportTypeLabel) as SavedReportType[]).map((type) => <option key={type} value={type}>{reportTypeLabel[type]}</option>)}
        </select>
        {canCreate && <button className="btn" onClick={openCreate}><Settings size={16} />新增报表</button>}
      </div>

      <div className="reports-workspace">
        <div className="card reports-list-panel">
          <DataState loading={loading} error={error} empty={!loading && !error && reports.length === 0} emptyText="暂无保存报表" onRetry={() => { void loadReports() }} />
          {!loading && !error && reports.length > 0 && (
            <table className="responsive-table"><thead><tr><th>名称</th><th>类型</th><th>筛选</th><th>操作</th></tr></thead><tbody>
              {reports.map((item) => (
                <tr key={item.id} className={activeReport?.id === item.id ? 'report-active-row' : undefined}>
                  <td data-label="名称"><strong>{item.name}</strong><p className="table-subtext">{item.description || reportTypeHint[item.type]}</p></td>
                  <td data-label="类型">{reportTypeLabel[item.type]}</td>
                  <td data-label="筛选">{item.filters?.keyword || item.filters?.projectId || item.filters?.departmentId || item.filters?.ownerId || item.filters?.dateFrom || item.filters?.dateTo || (item.filters?.statuses || []).length > 0 ? '已保存' : '-'}</td>
                  <td data-label="操作">
                    <div className="table-actions">
                      <button className="btn secondary" onClick={() => { void runReport(item) }}><Play size={15} />运行</button>
                      {canUpdate && <button className="btn secondary" onClick={() => openEdit(item)}>编辑</button>}
                      {canDelete && <button className="btn danger" onClick={() => { void deleteReport(item) }}>删除</button>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody></table>
          )}
          {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}
        </div>

        <div className="report-output-panel">
          <section className="card report-result-panel">
            <div className="report-panel-header">
              <div>
                <h3>{activeReport ? activeReport.name : '运行报表'}</h3>
                <p>{result ? `${reportTypeLabel[result.type]} · 生成于 ${formatGeneratedAt(result.generatedAt)}` : '选择左侧保存报表查看实时结果'}</p>
              </div>
              {activeReport && (
                <div className="row-actions">
                  <button className="btn secondary" disabled={resultLoading} onClick={() => { void runReport(activeReport) }}><RefreshCcw size={15} />刷新</button>
                  <button className="btn secondary" disabled={exporting} onClick={() => { void exportCSV(activeReport) }}><Download size={15} />CSV</button>
                  <button className="btn secondary" onClick={() => window.print()}><Printer size={15} />打印/PDF</button>
                </div>
              )}
            </div>
            <DataState loading={resultLoading} error={resultError} empty={!resultLoading && !resultError && !result} emptyText="尚未运行报表" onRetry={() => activeReport && void runReport(activeReport)} />
            {!resultLoading && !resultError && result && (
              <>
                <div className="report-result-metrics">
                  {(result.summary || []).map((metric) => (
                    <span key={`${metric.label}-${metric.value}`} className={metric.tone ? `metric-${metric.tone}` : undefined}>
                      {metric.label}<strong>{metric.value}</strong>
                    </span>
                  ))}
                </div>
                {displayMode !== 'summary' && activeRows.length === 0 && <p className="inline-tip">当前筛选下暂无报表明细</p>}
                {displayMode === 'chart' && activeRows.length > 0 && (
                  <ReportResultChart columns={activeColumns} rows={activeRows} />
                )}
                {displayMode === 'table' && activeRows.length > 0 && (
                  <table className="responsive-table report-result-table"><thead><tr>
                    {activeColumns.map((column) => <th key={column.key}>{column.label}</th>)}
                  </tr></thead><tbody>
                    {activeRows.map((row, index) => (
                      <tr key={index}>
                        {activeColumns.map((column) => <td key={column.key} data-label={column.label}>{formatCell(row[column.key])}</td>)}
                      </tr>
                    ))}
                  </tbody></table>
                )}
              </>
            )}
          </section>

          <section className="card report-subscription-panel">
            <div className="report-panel-header">
              <div>
                <h3>周报订阅</h3>
                <p>{subscription?.lastRunAt ? `上次发送 ${formatGeneratedAt(subscription.lastRunAt)} · ${subscription.lastStatus || '-'}` : '保存后系统每周自动发送项目周报'}</p>
              </div>
              {canUpdate && activeReport && <button className="btn secondary" disabled={sending || !subscription} onClick={() => { void sendNow() }}><Send size={15} />立即发送</button>}
            </div>
            {!activeReport && <p className="inline-tip">先运行一个报表，再配置订阅。</p>}
            {activeReport && (
              <div className="subscription-form-grid">
                <label className="switch-row">
                  <input type="checkbox" checked={subscriptionForm.isEnabled} onChange={(event) => setSubscriptionForm((prev) => ({ ...prev, isEnabled: event.target.checked }))} />
                  <span>启用每周发送</span>
                </label>
                <label>
                  发送日
                  <select value={subscriptionForm.weekday} onChange={(event) => setSubscriptionForm((prev) => ({ ...prev, weekday: Number(event.target.value) }))}>
                    {weekdayOptions.map((label, index) => <option key={label} value={index}>{label}</option>)}
                  </select>
                </label>
                <label>
                  小时
                  <input type="number" min={0} max={23} value={subscriptionForm.hour} onChange={(event) => setSubscriptionForm((prev) => ({ ...prev, hour: Number(event.target.value) }))} />
                </label>
                <label>
                  接收人 ID
                  <input value={subscriptionForm.recipientUserIds} placeholder="留空默认本人，多个用逗号分隔" onChange={(event) => setSubscriptionForm((prev) => ({ ...prev, recipientUserIds: event.target.value }))} />
                </label>
                <div className="report-channel-list">
                  {channelOptions.map((channel) => (
                    <label key={channel.value} className="multi-check-item">
                      <input type="checkbox" checked={subscriptionForm.channels.includes(channel.value)} onChange={() => toggleChannel(channel.value)} />
                      <span>{channel.label}</span>
                    </label>
                  ))}
                </div>
                {canUpdate && <button className="btn" disabled={subscriptionSaving} onClick={() => { void saveSubscription() }}>{subscriptionSaving ? '保存中...' : '保存订阅'}</button>}
              </div>
            )}
            {subscriptionError && <p className="error">{subscriptionError}</p>}
            {subscription?.lastError && <p className="inline-tip">{subscription.lastError}</p>}
          </section>
        </div>
      </div>

      <Modal open={modalOpen} title={form.id ? '编辑报表' : '新增报表'} onClose={() => setModalOpen(false)}>
        <form className="form-grid report-form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="report-name">名称</label>
          <input id="report-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="report-type">类型</label>
          <select id="report-type" value={form.type} onChange={(event) => setForm((prev) => ({ ...prev, type: event.target.value as SavedReportType }))}>
            {(Object.keys(reportTypeLabel) as SavedReportType[]).map((type) => <option key={type} value={type}>{reportTypeLabel[type]}</option>)}
          </select>
          <label htmlFor="report-description">描述</label>
          <textarea id="report-description" rows={3} value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="report-project-id">项目</label>
          <RemoteProjectSelect
            ariaLabel="报表项目"
            value={form.projectId}
            defaultOptionLabel="全部项目"
            placeholder="搜索项目"
            onChange={(value) => setForm((prev) => ({ ...prev, projectId: value }))}
          />
          <label htmlFor="report-department-id">部门</label>
          {canReadDepartments && departments.length > 0 ? (
            <select id="report-department-id" value={form.departmentId} onChange={(event) => setForm((prev) => ({ ...prev, departmentId: event.target.value }))}>
              <option value="">全部部门</option>
              {departments.map((department) => <option key={department.id} value={department.id}>{department.name}</option>)}
            </select>
          ) : (
            <input id="report-department-id" type="number" min={0} value={form.departmentId} onChange={(event) => setForm((prev) => ({ ...prev, departmentId: event.target.value }))} />
          )}
          <label htmlFor="report-owner-id">负责人</label>
          {canReadUsers && users.length > 0 ? (
            <select id="report-owner-id" value={form.ownerId} onChange={(event) => setForm((prev) => ({ ...prev, ownerId: event.target.value }))}>
              <option value="">全部负责人</option>
              {users.map((user) => <option key={user.id} value={user.id}>{user.name || user.username}</option>)}
            </select>
          ) : (
            <input id="report-owner-id" type="number" min={0} value={form.ownerId} onChange={(event) => setForm((prev) => ({ ...prev, ownerId: event.target.value }))} />
          )}
          <label htmlFor="report-date-from">开始日期</label>
          <input id="report-date-from" type="date" value={form.dateFrom} onChange={(event) => setForm((prev) => ({ ...prev, dateFrom: event.target.value }))} />
          <label htmlFor="report-date-to">结束日期</label>
          <input id="report-date-to" type="date" value={form.dateTo} onChange={(event) => setForm((prev) => ({ ...prev, dateTo: event.target.value }))} />
          <label htmlFor="report-keyword">关键词</label>
          <input id="report-keyword" value={form.keyword} onChange={(event) => setForm((prev) => ({ ...prev, keyword: event.target.value }))} />
          <label>状态</label>
          <div className="multi-checklist">
            {STATUS_ORDER.map((status) => (
              <label key={status} className="multi-check-item">
                <input type="checkbox" checked={form.statuses.includes(status)} onChange={() => toggleStatus(status)} />
                <span>{STATUS_META[status].label}</span>
              </label>
            ))}
          </div>
          <label htmlFor="report-display-mode">展示方式</label>
          <select id="report-display-mode" value={form.displayMode} onChange={(event) => setForm((prev) => ({ ...prev, displayMode: event.target.value }))}>
            <option value="summary">摘要</option>
            <option value="table">表格</option>
            <option value="chart">图表</option>
          </select>
          <div className="row-actions">
            <button className="btn" type="submit" disabled={submitting}>{submitting ? '保存中...' : '保存并运行'}</button>
            <button className="btn secondary" type="button" onClick={() => setModalOpen(false)}>取消</button>
          </div>
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>
    </section>
  )
}
