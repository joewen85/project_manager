import { FormEvent, useEffect, useMemo, useState } from 'react'
import { BarChart3 } from 'lucide-react'
import { api, fetchArray, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { STATUS_META, STATUS_ORDER } from '../constants/status'
import { ProjectHealth, SavedReport, SavedReportType, Status } from '../types'
import { usePermissions } from '../hooks/usePermissions'

interface ReportForm {
  id?: number
  name: string
  description: string
  type: SavedReportType
  projectId: string
  keyword: string
  statuses: Status[]
  displayMode: string
}

interface ProjectHealthResponse {
  projects?: ProjectHealth[]
}

interface MemberWorkloadItem {
  userId: number
  name: string
  username: string
  taskCount: number
  estimatedHours: number
  capacityHours: number
  overloaded: boolean
}

interface MemberWorkloadResponse {
  members?: MemberWorkloadItem[]
}

interface DashboardProgressRaw {
  status?: string
  count?: number
}

const reportTypeLabel: Record<SavedReportType, string> = {
  project_health: '项目健康',
  member_workload: '成员负载',
  task_status: '任务状态'
}

const initialForm: ReportForm = {
  name: '',
  description: '',
  type: 'project_health',
  projectId: '',
  keyword: '',
  statuses: [],
  displayMode: 'summary'
}

const normalizeReports = (value: SavedReport | null): ReportForm => ({
  id: value?.id,
  name: value?.name || '',
  description: value?.description || '',
  type: value?.type || 'project_health',
  projectId: value?.filters?.projectId ? String(value.filters.projectId) : '',
  keyword: value?.filters?.keyword || '',
  statuses: value?.filters?.statuses || [],
  displayMode: value?.chartConfig?.displayMode || 'summary'
})

export function ReportsPage() {
  const permissions = usePermissions()
  const canCreate = hasPermission('reports.create', permissions)
  const canUpdate = hasPermission('reports.update', permissions)
  const canDelete = hasPermission('reports.delete', permissions)
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
  const [previewError, setPreviewError] = useState('')
  const [health, setHealth] = useState<ProjectHealth[]>([])
  const [workload, setWorkload] = useState<MemberWorkloadItem[]>([])
  const [progress, setProgress] = useState<DashboardProgressRaw[]>([])

  const loadReports = async () => {
    try {
      setLoading(true)
      setError('')
      const data = await fetchPage<SavedReport>('/reports', { page, pageSize, keyword, type: typeFilter }, { page, pageSize })
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

  const loadPreview = async () => {
    try {
      setPreviewError('')
      const [healthData, workloadData, progressData] = await Promise.all([
        fetchData<ProjectHealthResponse>('/stats/project-health', undefined, { silent: true }),
        fetchData<MemberWorkloadResponse>('/stats/member-workload', undefined, { silent: true }),
        fetchArray<DashboardProgressRaw>('/tasks/progress-list', undefined, { silent: true })
      ])
      setHealth(Array.isArray(healthData.projects) ? healthData.projects : [])
      setWorkload(Array.isArray(workloadData.members) ? workloadData.members : [])
      setProgress(progressData)
    } catch (loadError) {
      setPreviewError(readApiError(loadError, '报表预览加载失败'))
      setHealth([])
      setWorkload([])
      setProgress([])
    }
  }

  useEffect(() => { void loadReports() }, [page, pageSize, keyword, typeFilter])
  useEffect(() => { void loadPreview() }, [])

  const redProjects = useMemo(() => health.filter((item) => item.health === 'red').length, [health])
  const overloadedMembers = useMemo(() => workload.filter((item) => item.overloaded).length, [workload])
  const progressTotal = useMemo(() => progress.reduce((sum, item) => sum + Number(item.count || 0), 0), [progress])

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
      if (form.id) {
        await api.put(`/reports/${form.id}`, payload)
      } else {
        await api.post('/reports', payload)
      }
      setModalOpen(false)
      setForm(initialForm)
      await loadReports()
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
      await api.delete(`/reports/${item.id}`)
      await loadReports()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除报表失败'))
    }
  }

  const toggleStatus = (status: Status) => {
    setForm((prev) => ({
      ...prev,
      statuses: prev.statuses.includes(status) ? prev.statuses.filter((item) => item !== status) : [...prev.statuses, status]
    }))
  }

  return (
    <section className="page-section">
      <div className="report-preview-grid">
        <article className="card report-preview-card">
          <div className="report-preview-title"><BarChart3 size={18} /><h3>项目健康</h3></div>
          <strong>{redProjects}</strong>
          <span>高风险 / {health.length} 项</span>
        </article>
        <article className="card report-preview-card">
          <div className="report-preview-title"><BarChart3 size={18} /><h3>成员负载</h3></div>
          <strong>{overloadedMembers}</strong>
          <span>过载 / {workload.length} 人</span>
        </article>
        <article className="card report-preview-card">
          <div className="report-preview-title"><BarChart3 size={18} /><h3>任务状态</h3></div>
          <strong>{progressTotal}</strong>
          <span>可见任务</span>
        </article>
      </div>
      {previewError && <p className="inline-tip">{previewError}</p>}
      <div className="card report-filter-bar">
        <SearchField aria-label="搜索保存报表" placeholder="搜索报表名称/描述" value={keyword} onChange={(value) => { setKeyword(value); setPage(1) }} />
        <select value={typeFilter} onChange={(event) => { setTypeFilter(event.target.value); setPage(1) }} aria-label="报表类型筛选">
          <option value="">全部类型</option>
          {(Object.keys(reportTypeLabel) as SavedReportType[]).map((type) => <option key={type} value={type}>{reportTypeLabel[type]}</option>)}
        </select>
        {canCreate && <button className="btn" onClick={openCreate}>新增报表</button>}
      </div>
      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && reports.length === 0} emptyText="暂无保存报表" onRetry={() => { void loadReports() }} />
        {!loading && !error && reports.length > 0 && (
          <table className="responsive-table"><thead><tr><th>名称</th><th>类型</th><th>筛选</th><th>展示</th><th>操作</th></tr></thead><tbody>
            {reports.map((item) => (
              <tr key={item.id}>
                <td data-label="名称"><strong>{item.name}</strong><p className="table-subtext">{item.description || '-'}</p></td>
                <td data-label="类型">{reportTypeLabel[item.type]}</td>
                <td data-label="筛选">{item.filters?.keyword || item.filters?.projectId || (item.filters?.statuses || []).length > 0 ? '已保存' : '-'}</td>
                <td data-label="展示">{item.chartConfig?.displayMode || 'summary'}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    {canUpdate && <button className="btn secondary" onClick={() => openEdit(item)}>编辑</button>}
                    {canDelete && <button className="btn danger" onClick={() => { void deleteReport(item) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>
      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑报表' : '新增报表'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="report-name">名称</label>
          <input id="report-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="report-type">类型</label>
          <select id="report-type" value={form.type} onChange={(event) => setForm((prev) => ({ ...prev, type: event.target.value as SavedReportType }))}>
            {(Object.keys(reportTypeLabel) as SavedReportType[]).map((type) => <option key={type} value={type}>{reportTypeLabel[type]}</option>)}
          </select>
          <label htmlFor="report-description">描述</label>
          <textarea id="report-description" rows={3} value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="report-project-id">项目ID筛选</label>
          <input id="report-project-id" type="number" min={0} value={form.projectId} onChange={(event) => setForm((prev) => ({ ...prev, projectId: event.target.value }))} />
          <label htmlFor="report-keyword">关键词筛选</label>
          <input id="report-keyword" value={form.keyword} onChange={(event) => setForm((prev) => ({ ...prev, keyword: event.target.value }))} />
          <label>状态筛选</label>
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
            <button className="btn" type="submit" disabled={submitting}>{submitting ? '保存中...' : '保存报表'}</button>
            <button className="btn secondary" type="button" onClick={() => setModalOpen(false)}>取消</button>
          </div>
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>
    </section>
  )
}
