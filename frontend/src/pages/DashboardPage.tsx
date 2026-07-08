import { lazy, Suspense, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchArray, fetchData, fetchPage, hasAnyPermission, hasPermission, readApiError } from '../services/api'
import { STATUS_META, STATUS_ORDER } from '../constants/status'
import { ProjectHealth, ProjectRegister, Status } from '../types'
import { DataState } from '../components/DataState'
import type {
  DashboardDeliveryItem,
  DashboardHealthChartItem,
  DashboardProgressItem,
  DashboardProjectProgressItem,
  DashboardRegisterSeverityItem,
  DashboardRegisterStatusItem,
  DashboardRiskChartItem,
  DashboardTrendItem,
  DashboardWorkloadChartItem,
  WorkloadSortKey
} from '../components/DashboardCharts'
import { usePermissions } from '../hooks/usePermissions'

const DashboardCharts = lazy(async () => import('../components/DashboardCharts').then((module) => ({ default: module.DashboardCharts })))

interface DashboardStats {
  users: number
  projects: number
  tasks: number
  completedTasks: number
  completionRate: number
}

interface DashboardProgressRaw {
  status?: string
  count?: number
}

interface ProjectHealthResponse {
  projects?: ProjectHealth[]
}

interface MemberWorkloadItem {
  userId: number
  name: string
  username: string
  email: string
  taskCount: number
  estimatedHours: number
  actualHours: number
  remainingHours: number
  capacityHours: number
  utilizationRate: number
  overloaded: boolean
}

interface MemberWorkloadResponse {
  weekStart?: string
  weekEnd?: string
  members?: MemberWorkloadItem[]
}

interface RegisterTypeStatusRaw {
  type?: string
  status?: string
  count?: number
}

interface RegisterTypeSeverityRaw {
  type?: string
  severity?: string
  count?: number
}

interface RegisterOverviewResponse {
  byTypeStatus?: RegisterTypeStatusRaw[]
  byTypeSeverity?: RegisterTypeSeverityRaw[]
}

const registerTypeLabel: Record<string, string> = {
  risk: '风险',
  issue: '问题',
  decision: '决策'
}

const registerSeverityLabel: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重'
}

const registerTypeOrder = ['risk', 'issue', 'decision'] as const
const registerSeverityOrder = ['low', 'medium', 'high', 'critical'] as const

const healthLabel: Record<ProjectHealth['health'], string> = {
  green: '健康',
  yellow: '关注',
  red: '高风险'
}

const healthColor: Record<ProjectHealth['health'], string> = {
  green: '#22c55e',
  yellow: '#f59e0b',
  red: '#ef4444'
}

const formatDate = (value?: string) => {
  if (!value) return '-'
  return new Date(value).toLocaleDateString('zh-CN')
}

const formatHours = (value?: number) => {
  const hours = Number(value ?? 0)
  if (!Number.isFinite(hours)) return '0h'
  return `${hours.toFixed(2).replace(/\.?0+$/, '')}h`
}

export function DashboardPage() {
  const permissions = usePermissions()

  const [stats, setStats] = useState<DashboardStats>()
  const [progress, setProgress] = useState<DashboardProgressItem[]>([])
  const [progressError, setProgressError] = useState('')
  const [projectHealth, setProjectHealth] = useState<ProjectHealth[]>([])
  const [projectHealthError, setProjectHealthError] = useState('')
  const [memberWorkload, setMemberWorkload] = useState<MemberWorkloadResponse>({ members: [] })
  const [memberWorkloadError, setMemberWorkloadError] = useState('')
  const [workloadSort, setWorkloadSort] = useState<WorkloadSortKey>('utilization')
  const [highRiskRegisterCount, setHighRiskRegisterCount] = useState(0)
  const [unresolvedIssueCount, setUnresolvedIssueCount] = useState(0)
  const [registerOverview, setRegisterOverview] = useState<RegisterOverviewResponse>({ byTypeStatus: [], byTypeSeverity: [] })
  const [registerError, setRegisterError] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const canViewUsers = hasAnyPermission(['system.users.read', 'system.users.create', 'system.users.update', 'system.users.delete', 'users.read', 'users.write', 'rbac.manage'], permissions)
  const canReadRegisters = hasPermission('registers.read', permissions)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      setProgressError('')
      setProjectHealthError('')
      setMemberWorkloadError('')
      setRegisterError('')
      const [statsData, raw, healthData, workloadData, highRiskData, unresolvedIssueData, registerOverviewData] = await Promise.all([
        fetchData<DashboardStats>('/insights/stats/dashboard'),
        fetchArray<DashboardProgressRaw>('/delivery/tasks/progress-list', undefined, { silent: true }).catch((progressLoadError) => {
          setProgressError(readApiError(progressLoadError, '任务状态分布加载失败'))
          return []
        }),
        fetchData<ProjectHealthResponse>('/insights/stats/project-health', undefined, { silent: true }).catch((healthError) => {
          setProjectHealthError(readApiError(healthError, '项目健康度加载失败'))
          return { projects: [] }
        }),
        fetchData<MemberWorkloadResponse>('/insights/stats/member-workload', undefined, { silent: true }).catch((workloadError) => {
          setMemberWorkloadError(readApiError(workloadError, '成员负载加载失败'))
          return { members: [] } as MemberWorkloadResponse
        }),
        canReadRegisters
          ? fetchPage<ProjectRegister>('/portfolio/registers', { page: 1, pageSize: 1, type: 'risk', statuses: 'open,in_progress', severities: 'high,critical' }, { page: 1, pageSize: 1 }, { silent: true }).catch((registerLoadError) => {
            setRegisterError(readApiError(registerLoadError, '登记册摘要加载失败'))
            return { list: [] as ProjectRegister[], total: 0, page: 1, pageSize: 1 }
          })
          : Promise.resolve({ list: [] as ProjectRegister[], total: 0, page: 1, pageSize: 1 }),
        canReadRegisters
          ? fetchPage<ProjectRegister>('/portfolio/registers', { page: 1, pageSize: 1, type: 'issue', statuses: 'open,in_progress' }, { page: 1, pageSize: 1 }, { silent: true }).catch((registerLoadError) => {
            setRegisterError(readApiError(registerLoadError, '登记册摘要加载失败'))
            return { list: [] as ProjectRegister[], total: 0, page: 1, pageSize: 1 }
          })
          : Promise.resolve({ list: [] as ProjectRegister[], total: 0, page: 1, pageSize: 1 }),
        canReadRegisters
          ? fetchData<RegisterOverviewResponse>('/insights/stats/register-overview', undefined, { silent: true }).catch((registerLoadError) => {
            setRegisterError(readApiError(registerLoadError, '登记册摘要加载失败'))
            return { byTypeStatus: [], byTypeSeverity: [] } as RegisterOverviewResponse
          })
          : Promise.resolve({ byTypeStatus: [], byTypeSeverity: [] } as RegisterOverviewResponse)
      ])
      setStats(statsData)
      setProjectHealth(Array.isArray(healthData.projects) ? healthData.projects : [])
      setHighRiskRegisterCount(highRiskData.total)
      setUnresolvedIssueCount(unresolvedIssueData.total)
      setRegisterOverview({
        byTypeStatus: Array.isArray(registerOverviewData.byTypeStatus) ? registerOverviewData.byTypeStatus : [],
        byTypeSeverity: Array.isArray(registerOverviewData.byTypeSeverity) ? registerOverviewData.byTypeSeverity : []
      })
      setMemberWorkload({
        weekStart: workloadData.weekStart,
        weekEnd: workloadData.weekEnd,
        members: Array.isArray(workloadData.members) ? workloadData.members : []
      })

      const source = raw
        .filter((item) => item && typeof item === 'object')
        .map((item) => ({
          status: String(item.status ?? 'pending') as Status,
          count: Number(item.count ?? 0)
        }))
      const merged = STATUS_ORDER.map((status) => {
        const found = source.find((item) => item.status === status)
        return {
          status,
          count: found?.count ?? 0,
          statusLabel: STATUS_META[status].label,
          fill: STATUS_META[status].color
        }
      })
      setProgress(merged)
    } catch (loadError) {
      setError(readApiError(loadError, '统计数据加载失败'))
      setStats(undefined)
      setProgress([])
      setProjectHealth([])
      setMemberWorkload({ members: [] })
      setHighRiskRegisterCount(0)
      setUnresolvedIssueCount(0)
      setRegisterOverview({ byTypeStatus: [], byTypeSeverity: [] })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [canReadRegisters])

  const totalTasks = stats?.tasks ?? progress.reduce((sum, item) => sum + item.count, 0)
  const completedTasks = stats?.completedTasks ?? progress.find((item) => item.status === 'completed')?.count ?? 0
  const completionRatePercent = Math.max(0, Math.min(100, (stats?.completionRate ?? (totalTasks > 0 ? completedTasks / totalTasks : 0)) * 100))
  const redProjectCount = projectHealth.filter((item) => item.health === 'red').length
  const overloadedMemberCount = (memberWorkload.members || []).filter((item) => item.overloaded).length
  const overdueTaskCount = projectHealth.reduce((sum, item) => sum + item.overdueTasks, 0)
  const averageProjectCompletion = projectHealth.length > 0
    ? projectHealth.reduce((sum, item) => sum + item.completionRate, 0) / projectHealth.length * 100
    : completionRatePercent

  const healthChart = useMemo<DashboardHealthChartItem[]>(() => {
    return (['green', 'yellow', 'red'] as ProjectHealth['health'][]).map((health) => ({
      name: healthLabel[health],
      value: projectHealth.filter((item) => item.health === health).length,
      fill: healthColor[health]
    }))
  }, [projectHealth])

  const projectProgressChart = useMemo<DashboardProjectProgressItem[]>(() => {
    return [...projectHealth]
      .sort((left, right) => right.completionRate - left.completionRate)
      .slice(0, 8)
      .map((item) => ({
        name: item.projectCode || item.projectName,
        completionRate: Number((item.completionRate * 100).toFixed(1)),
        score: item.score,
        fill: healthColor[item.health]
      }))
  }, [projectHealth])

  const riskChart = useMemo<DashboardRiskChartItem[]>(() => [
    { name: '风险项目', value: redProjectCount, fill: '#ef4444' },
    { name: '关注项目', value: projectHealth.filter((item) => item.health === 'yellow').length, fill: '#f59e0b' },
    { name: '高风险项', value: highRiskRegisterCount, fill: '#dc2626' },
    { name: '未解决问题', value: unresolvedIssueCount, fill: '#7c3aed' },
    { name: '逾期任务', value: overdueTaskCount, fill: '#f97316' }
  ], [highRiskRegisterCount, overdueTaskCount, projectHealth, redProjectCount, unresolvedIssueCount])

  const workloadChart = useMemo<DashboardWorkloadChartItem[]>(() => {
    const sortValue = (item: MemberWorkloadItem) => (workloadSort === 'taskCount' ? item.taskCount : item.utilizationRate)
    return [...(memberWorkload.members || [])]
      .sort((left, right) => sortValue(right) - sortValue(left))
      .slice(0, 8)
      .map((item) => ({
        name: item.name || item.username,
        utilization: Math.round((item.utilizationRate || 0) * 100),
        taskCount: item.taskCount,
        barFill: item.overloaded ? '#ef4444' : '#2563eb'
      }))
  }, [memberWorkload.members, workloadSort])

  const deliveryChart = useMemo<DashboardDeliveryItem[]>(() => {
    const sum = (pick: (item: ProjectHealth) => number) => projectHealth.reduce((total, item) => total + (pick(item) || 0), 0)
    return [
      { name: '未排期', value: sum((item) => item.unscheduledTasks), fill: '#6366f1' },
      { name: '待审核', value: sum((item) => item.reviewingTasks), fill: '#0ea5e9' },
      { name: '关键逾期', value: sum((item) => item.criticalOverdueTasks || 0), fill: '#dc2626' },
      { name: '里程碑逾期', value: sum((item) => item.milestoneOverdue), fill: '#f97316' }
    ]
  }, [projectHealth])

  const registerStatusChart = useMemo<DashboardRegisterStatusItem[]>(() => {
    const rows = registerOverview.byTypeStatus || []
    return registerTypeOrder
      .map((type) => {
        const forType = rows.filter((row) => row.type === type)
        const pick = (status: string) => Number(forType.find((row) => row.status === status)?.count ?? 0)
        return {
          type,
          typeLabel: registerTypeLabel[type] || type,
          open: pick('open'),
          in_progress: pick('in_progress'),
          resolved: pick('resolved'),
          closed: pick('closed')
        }
      })
      .filter((item) => item.open + item.in_progress + item.resolved + item.closed > 0)
  }, [registerOverview.byTypeStatus])

  const registerSeverityChart = useMemo<DashboardRegisterSeverityItem[]>(() => {
    const rows = registerOverview.byTypeSeverity || []
    return registerSeverityOrder
      .map((severity) => {
        const forSeverity = rows.filter((row) => row.severity === severity)
        const pick = (type: string) => Number(forSeverity.find((row) => row.type === type)?.count ?? 0)
        return {
          severity,
          severityLabel: registerSeverityLabel[severity] || severity,
          risk: pick('risk'),
          issue: pick('issue')
        }
      })
      .filter((item) => item.risk + item.issue > 0)
  }, [registerOverview.byTypeSeverity])

  const trendChart = useMemo<DashboardTrendItem[]>(() => {
    let cumulative = 0
    return progress.map((item) => {
      cumulative += item.count
      return {
        stage: item.statusLabel,
        count: item.count,
        cumulative
      }
    })
  }, [progress])

  return (
    <section className="page-section">
      <DataState loading={loading} error={error} onRetry={() => { void load() }} />
      {!loading && !error && (
        <header className="dashboard-hero">
          <div>
            <h3>项目 Dashboard</h3>
            <p>当前可见项目、任务推进、风险问题和成员负载的实时概览。</p>
          </div>
          <div className="dashboard-hero-metrics">
            <span>平均项目完成率 <strong>{averageProjectCompletion.toFixed(1)}%</strong></span>
            <span>逾期任务 <strong>{overdueTaskCount}</strong></span>
            <span>过载成员 <strong>{overloadedMemberCount}</strong></span>
          </div>
        </header>
      )}
      <div className="cards">
        {canViewUsers && <article className="card metric-card"><p>用户</p><strong>{stats?.users ?? 0}</strong><small>系统账号</small></article>}
        <article className="card metric-card"><p>项目</p><strong>{stats?.projects ?? 0}</strong><small>可见项目</small></article>
        <article className="card metric-card"><p>任务</p><strong>{totalTasks}</strong><small>任务总量</small></article>
        <article className="card metric-card"><p>完成率</p><strong>{completionRatePercent.toFixed(1)}%</strong><small>{completedTasks} 项已完成</small></article>
        <article className="card metric-card"><p>风险项目</p><strong>{redProjectCount}</strong><small>健康度红色</small></article>
        <article className="card metric-card"><p>过载成员</p><strong>{overloadedMemberCount}</strong><small>容量超限</small></article>
        {canReadRegisters && <article className="card metric-card"><p>高风险登记项</p><strong>{highRiskRegisterCount}</strong></article>}
        {canReadRegisters && <article className="card metric-card"><p>未解决问题</p><strong>{unresolvedIssueCount}</strong></article>}
      </div>
      {!loading && !error && registerError && <p className="inline-tip">{registerError}</p>}
      {!loading && !error && progressError && <p className="inline-tip">{progressError}</p>}
      {!loading && !error && (
        <Suspense fallback={<div className="card">图表加载中...</div>}>
          <DashboardCharts
            progress={progress}
            completionRate={completionRatePercent}
            completedTasks={completedTasks}
            totalTasks={totalTasks}
            health={healthChart}
            projectProgress={projectProgressChart}
            riskItems={riskChart}
            workload={workloadChart}
            workloadSort={workloadSort}
            onWorkloadSortChange={setWorkloadSort}
            trend={trendChart}
            delivery={deliveryChart}
            canReadRegisters={canReadRegisters}
            registerStatus={registerStatusChart}
            registerSeverity={registerSeverityChart}
          />
        </Suspense>
      )}
      {!loading && !error && (
        <section className="card dashboard-health-panel">
          <div className="dashboard-health-header">
            <h3>项目健康榜</h3>
            {projectHealthError && <span className="error">{projectHealthError}</span>}
          </div>
          {projectHealth.length === 0 && !projectHealthError && <p className="inline-tip">暂无可见项目健康数据</p>}
          {projectHealth.length > 0 && (
            <div className="dashboard-health-list">
              {projectHealth.map((item) => (
                <Link key={item.projectId} className={`dashboard-health-item health-${item.health}`} to={`/delivery/tasks?projectId=${item.projectId}`}>
                  <div className="dashboard-health-main">
                    <span className="dashboard-health-badge">{healthLabel[item.health]}</span>
                    <strong>{item.projectCode} - {item.projectName}</strong>
                    <span>得分 {item.score} · 完成率 {(item.completionRate * 100).toFixed(1)}%</span>
                  </div>
                  <div className="dashboard-health-metrics">
                    <span>逾期 {item.overdueTasks}</span>
                    <span>关键逾期 {item.criticalOverdueTasks || 0}</span>
                    <span>高风险 {item.highRiskRegisters || 0}</span>
                    <span>问题 {item.unresolvedIssues || 0}</span>
                    <span>里程碑 {item.milestoneOverdue}</span>
                    <span>未排期 {item.unscheduledTasks}</span>
                    <span>待审核 {item.reviewingTasks}</span>
                  </div>
                  <p>{item.reasons.join('；')}</p>
                </Link>
              ))}
            </div>
          )}
        </section>
      )}
      {!loading && !error && (
        <section className="card dashboard-workload-panel">
          <div className="dashboard-health-header">
            <h3>本周成员负载</h3>
            <span>{formatDate(memberWorkload.weekStart)} - {formatDate(memberWorkload.weekEnd)}</span>
            {memberWorkloadError && <span className="error">{memberWorkloadError}</span>}
          </div>
          {(memberWorkload.members || []).length === 0 && !memberWorkloadError && <p className="inline-tip">暂无可见成员负载数据</p>}
          {(memberWorkload.members || []).length > 0 && (
            <div className="dashboard-workload-list">
              {(memberWorkload.members || []).map((item) => {
                const utilizationPercent = Math.round((item.utilizationRate || 0) * 100)
                const cappedWidth = Math.max(0, Math.min(100, utilizationPercent))
                return (
                  <article key={item.userId} className={`dashboard-workload-item${item.overloaded ? ' overloaded' : ''}`}>
                    <div className="dashboard-workload-main">
                      <strong>{item.name || item.username}</strong>
                      <span>{item.taskCount} 项任务</span>
                      <span>{formatHours(item.estimatedHours)} / {formatHours(item.capacityHours)}</span>
                      {item.overloaded && <b>过载</b>}
                    </div>
                    <div className="dashboard-workload-bar" aria-label={`容量使用率 ${utilizationPercent}%`}>
                      <span style={{ width: `${cappedWidth}%` }} />
                    </div>
                    <div className="dashboard-health-metrics">
                      <span>实际 {formatHours(item.actualHours)}</span>
                      <span>剩余 {formatHours(item.remainingHours)}</span>
                      <span>使用率 {utilizationPercent}%</span>
                    </div>
                  </article>
                )
              })}
            </div>
          )}
        </section>
      )}
    </section>
  )
}
