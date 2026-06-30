import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { STATUS_META } from '../constants/status'
import { CriticalPathResult, Project, ProjectBaseline, ProjectBaselineDetail } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface BaselineForm {
  projectId: string
  name: string
  description: string
}

const initialForm: BaselineForm = {
  projectId: '',
  name: '',
  description: ''
}

const formatVariance = (value?: number) => {
  const days = Number(value || 0)
  if (days === 0) return '0天'
  return `${days > 0 ? '+' : ''}${days}天`
}

const projectLabel = (project?: Project) => {
  if (!project) return '-'
  return `${project.code || project.id} - ${project.name}`
}

export function ProjectBaselinesPage() {
  const permissions = usePermissions()
  const canCreateBaseline = hasPermission('baselines.create', permissions)
  const canDeleteBaseline = hasPermission('baselines.delete', permissions)
  const [items, setItems] = useState<ProjectBaseline[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [projectFilter, setProjectFilter] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [form, setForm] = useState<BaselineForm>(initialForm)
  const [modalOpen, setModalOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [activeDetail, setActiveDetail] = useState<ProjectBaselineDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [criticalPath, setCriticalPath] = useState<CriticalPathResult | null>(null)
  const [criticalLoading, setCriticalLoading] = useState(false)
  const [criticalError, setCriticalError] = useState('')
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(projectFilter))

  const projectByID = useMemo(() => {
    const map = new Map<number, Project>()
    projects.forEach((item) => map.set(item.id, item))
    return map
  }, [projects])

  const loadProjects = async () => {
    try {
      const pageData = await fetchPage<Project>('/projects', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      setProjects(pageData.list)
    } catch {
      setProjects([])
    }
  }

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const pageData = await fetchPage<ProjectBaseline>(
        '/project-baselines',
        { page, pageSize, keyword, projectId: projectFilter },
        { page, pageSize }
      )
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      setError(readApiError(loadError, '项目基线加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadCriticalPath = async (projectId: number) => {
    try {
      setCriticalLoading(true)
      setCriticalError('')
      const result = await fetchData<CriticalPathResult>(`/projects/${projectId}/critical-path`)
      setCriticalPath(result)
    } catch (loadError) {
      setCriticalPath(null)
      setCriticalError(readApiError(loadError, '关键路径加载失败'))
    } finally {
      setCriticalLoading(false)
    }
  }

  const loadDetail = async (item: ProjectBaseline) => {
    try {
      setDetailLoading(true)
      setDetailError('')
      const detail = await fetchData<ProjectBaselineDetail>(`/project-baselines/${item.id}`)
      setActiveDetail(detail)
      await loadCriticalPath(detail.projectId)
    } catch (loadError) {
      setActiveDetail(null)
      setCriticalPath(null)
      setDetailError(readApiError(loadError, '基线详情加载失败'))
    } finally {
      setDetailLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, projectFilter])
  useEffect(() => { void loadProjects() }, [])

  const openCreateModal = () => {
    if (!canCreateBaseline) return
    setForm({
      ...initialForm,
      projectId: projectFilter || (projects[0]?.id ? String(projects[0].id) : '')
    })
    setFormError('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!canCreateBaseline) return
    try {
      setSubmitting(true)
      setFormError('')
      const projectId = Number(form.projectId)
      if (!Number.isFinite(projectId) || projectId <= 0) {
        setFormError('请选择项目')
        return
      }
      await api.post('/project-baselines', {
        projectId,
        name: form.name.trim(),
        description: form.description.trim()
      })
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '创建项目基线失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const deleteBaseline = async (item: ProjectBaseline) => {
    if (!canDeleteBaseline) return
    if (!confirm(`确认删除基线「${item.name}」？`)) return
    try {
      await api.delete(`/project-baselines/${item.id}`)
      if (activeDetail?.id === item.id) {
        setActiveDetail(null)
        setCriticalPath(null)
      }
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除项目基线失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="基线筛选"
        activeCount={activeFilterCount}
        actions={canCreateBaseline ? <button className="btn secondary" onClick={openCreateModal}>创建基线</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="项目基线关键词搜索"
          value={keywordInput}
          placeholder="搜索名称/说明"
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
        <select aria-label="项目筛选" value={projectFilter} onChange={(event) => { setProjectFilter(event.target.value); setPage(1) }}>
          <option value="">全部项目</option>
          {projects.map((project) => <option key={project.id} value={project.id}>{projectLabel(project)}</option>)}
        </select>
        <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无项目基线" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table">
            <thead><tr><th>基线</th><th>项目</th><th>任务</th><th>计划周期</th><th>创建人</th><th>操作</th></tr></thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td data-label="基线">
                    <strong>{item.name}</strong>
                    <p className="table-subtext">{item.description || `#${item.id}`}</p>
                  </td>
                  <td data-label="项目">{projectLabel(item.project || projectByID.get(item.projectId))}</td>
                  <td data-label="任务">{item.completedTaskCount} / {item.taskCount}</td>
                  <td data-label="计划周期">{formatDateTime(item.plannedStartAt)} - {formatDateTime(item.plannedEndAt)}</td>
                  <td data-label="创建人">
                    {item.createdBy?.name || item.createdBy?.username || item.createdById}
                    <p className="table-subtext">{formatDateTime(item.createdAt)}</p>
                  </td>
                  <td data-label="操作">
                    <div className="table-actions">
                      <button className="btn secondary" onClick={() => { void loadDetail(item) }}>查看</button>
                      {canDeleteBaseline && <button className="btn danger" onClick={() => { void deleteBaseline(item) }}>删除</button>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />

      <div className="cards">
        <article className="card metric-card">
          <p>当前基线</p>
          <strong>{activeDetail ? activeDetail.taskCount : '-'}</strong>
          <small>{activeDetail ? activeDetail.name : '选择一条基线查看'}</small>
        </article>
        <article className="card metric-card">
          <p>延期任务</p>
          <strong>{activeDetail?.compare?.delayedTaskCount ?? '-'}</strong>
          <small>相对基线结束时间</small>
        </article>
        <article className="card metric-card">
          <p>关键路径</p>
          <strong>{criticalPath?.tasks?.length ?? '-'}</strong>
          <small>{criticalPath ? `${criticalPath.totalDurationDays}天` : '未加载'}</small>
        </article>
      </div>

      <section className="card">
        <div className="dashboard-health-header">
          <h3>基线对比</h3>
          {detailError && <span className="error">{detailError}</span>}
        </div>
        <DataState loading={detailLoading} empty={!detailLoading && !detailError && !activeDetail} emptyText="请选择基线查看偏差" onRetry={() => activeDetail && void loadDetail(activeDetail)} />
        {!detailLoading && activeDetail && (
          <div className="detail-grid">
            <section className="detail-section">
              <h4>摘要</h4>
              <div className="detail-columns">
                <div><strong>项目：</strong>{projectLabel(activeDetail.project || projectByID.get(activeDetail.projectId))}</div>
                <div><strong>任务数：</strong>{activeDetail.compare.currentTaskCount} / {activeDetail.compare.baselineTaskCount}</div>
                <div><strong>完成数：</strong>{activeDetail.compare.currentCompletedCount} / {activeDetail.compare.baselineCompletedCount}</div>
                <div><strong>结束偏差：</strong>{formatVariance(activeDetail.compare.endVarianceDays)}</div>
                <div><strong>延期任务：</strong>{activeDetail.compare.delayedTaskCount}</div>
                <div><strong>缺失任务：</strong>{activeDetail.compare.missingTaskCount}</div>
              </div>
            </section>
            <section className="detail-section">
              <h4>变更任务</h4>
              {activeDetail.compare.changedTasks.length === 0 && <p className="inline-tip">暂无偏差</p>}
              {activeDetail.compare.changedTasks.length > 0 && (
                <table className="responsive-table">
                  <thead><tr><th>任务</th><th>基线周期</th><th>当前周期</th><th>偏差</th><th>变化</th></tr></thead>
                  <tbody>
                    {activeDetail.compare.changedTasks.slice(0, 8).map((task) => (
                      <tr key={task.taskId}>
                        <td data-label="任务"><strong>{task.taskNo || task.taskId}</strong><p className="table-subtext">{task.title}</p></td>
                        <td data-label="基线周期">{formatDateTime(task.baselineStartAt)} - {formatDateTime(task.baselineEndAt)}</td>
                        <td data-label="当前周期">{task.missingCurrentTask ? '已缺失' : `${formatDateTime(task.currentStartAt)} - ${formatDateTime(task.currentEndAt)}`}</td>
                        <td data-label="偏差">开始 {formatVariance(task.startVarianceDays)} · 结束 {formatVariance(task.endVarianceDays)}</td>
                        <td data-label="变化">
                          {[
                            task.statusChanged ? '状态' : '',
                            task.progressChanged ? '进度' : '',
                            task.missingCurrentTask ? '缺失' : ''
                          ].filter(Boolean).join('、') || '排期'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
          </div>
        )}
      </section>

      <section className="card">
        <div className="dashboard-health-header">
          <h3>关键路径</h3>
          {criticalError && <span className="error">{criticalError}</span>}
        </div>
        <DataState loading={criticalLoading} empty={!criticalLoading && !criticalError && !criticalPath} emptyText="请选择基线查看关键路径" onRetry={() => activeDetail && void loadCriticalPath(activeDetail.projectId)} />
        {!criticalLoading && criticalPath && (
          <div className="detail-grid">
            <section className="detail-section">
              <div className="detail-columns">
                <div><strong>项目ID：</strong>{criticalPath.projectId}</div>
                <div><strong>总时长：</strong>{criticalPath.totalDurationDays}天</div>
                <div><strong>计划结束：</strong>{formatDateTime(criticalPath.projectEndAt)}</div>
                <div><strong>路径任务：</strong>{criticalPath.criticalTaskIds.length}</div>
              </div>
            </section>
            <section className="detail-section">
              {criticalPath.tasks.length === 0 && <p className="inline-tip">暂无可计算任务</p>}
              {criticalPath.tasks.length > 0 && (
                <table className="responsive-table">
                  <thead><tr><th>顺序</th><th>任务</th><th>状态</th><th>进度</th><th>周期</th><th>时长</th></tr></thead>
                  <tbody>
                    {criticalPath.tasks.map((task, index) => {
                      const meta = STATUS_META[task.status] || STATUS_META.pending
                      return (
                        <tr key={task.id}>
                          <td data-label="顺序">{index + 1}</td>
                          <td data-label="任务"><strong>{task.taskNo || task.id}</strong><p className="table-subtext">{task.title}</p></td>
                          <td data-label="状态"><span className="status-dot" style={{ background: meta.color }}>{meta.label}</span></td>
                          <td data-label="进度">{task.progress}%</td>
                          <td data-label="周期">{formatDateTime(task.startAt)} - {formatDateTime(task.endAt)}</td>
                          <td data-label="时长">{task.durationDays}天</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </section>
          </div>
        )}
      </section>

      <Modal open={modalOpen} title="创建项目基线" onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          {formError && <p className="error">{formError}</p>}
          <label htmlFor="baseline-project">项目</label>
          <select id="baseline-project" value={form.projectId} onChange={(event) => setForm((prev) => ({ ...prev, projectId: event.target.value }))} required>
            <option value="">选择项目</option>
            {projects.map((project) => <option key={project.id} value={project.id}>{projectLabel(project)}</option>)}
          </select>
          <label htmlFor="baseline-name">基线名称</label>
          <input id="baseline-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="baseline-description">说明</label>
          <textarea id="baseline-description" value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <button className="btn" disabled={submitting}>{submitting ? '创建中...' : '创建'}</button>
        </form>
      </Modal>
    </section>
  )
}
