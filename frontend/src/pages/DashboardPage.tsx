import { lazy, Suspense, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchArray, fetchData, hasAnyPermission, readApiError } from '../services/api'
import { STATUS_META, STATUS_ORDER } from '../constants/status'
import { ProjectHealth, Status } from '../types'
import { DataState } from '../components/DataState'
import type { DashboardProgressItem } from '../components/DashboardCharts'
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

const healthLabel: Record<ProjectHealth['health'], string> = {
  green: '健康',
  yellow: '关注',
  red: '高风险'
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
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const canViewUsers = hasAnyPermission(['users.read', 'users.create', 'users.update', 'users.delete', 'users.write', 'rbac.manage'], permissions)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      setProgressError('')
      setProjectHealthError('')
      setMemberWorkloadError('')
      const [statsData, raw, healthData, workloadData] = await Promise.all([
        fetchData<DashboardStats>('/stats/dashboard'),
        fetchArray<DashboardProgressRaw>('/tasks/progress-list', undefined, { silent: true }).catch((progressLoadError) => {
          setProgressError(readApiError(progressLoadError, '任务状态分布加载失败'))
          return []
        }),
        fetchData<ProjectHealthResponse>('/stats/project-health', undefined, { silent: true }).catch((healthError) => {
          setProjectHealthError(readApiError(healthError, '项目健康度加载失败'))
          return { projects: [] }
        }),
        fetchData<MemberWorkloadResponse>('/stats/member-workload', undefined, { silent: true }).catch((workloadError) => {
          setMemberWorkloadError(readApiError(workloadError, '成员负载加载失败'))
          return { members: [] } as MemberWorkloadResponse
        })
      ])
      setStats(statsData)
      setProjectHealth(Array.isArray(healthData.projects) ? healthData.projects : [])
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
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  return (
    <section className="page-section">
      <DataState loading={loading} error={error} onRetry={() => { void load() }} />
      <div className="cards">
        {canViewUsers && <article className="card metric-card"><p>用户</p><strong>{stats?.users ?? 0}</strong></article>}
        <article className="card metric-card"><p>项目</p><strong>{stats?.projects ?? 0}</strong></article>
        <article className="card metric-card"><p>任务</p><strong>{stats?.tasks ?? 0}</strong></article>
        <article className="card metric-card"><p>完成率</p><strong>{((stats?.completionRate ?? 0) * 100).toFixed(1)}%</strong></article>
        <article className="card metric-card"><p>风险项目</p><strong>{projectHealth.filter((item) => item.health === 'red').length}</strong></article>
        <article className="card metric-card"><p>过载成员</p><strong>{(memberWorkload.members || []).filter((item) => item.overloaded).length}</strong></article>
      </div>
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
                <Link key={item.projectId} className={`dashboard-health-item health-${item.health}`} to={`/tasks?projectId=${item.projectId}`}>
                  <div className="dashboard-health-main">
                    <span className="dashboard-health-badge">{healthLabel[item.health]}</span>
                    <strong>{item.projectCode} - {item.projectName}</strong>
                    <span>得分 {item.score} · 完成率 {(item.completionRate * 100).toFixed(1)}%</span>
                  </div>
                  <div className="dashboard-health-metrics">
                    <span>逾期 {item.overdueTasks}</span>
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
      {!loading && !error && progressError && <p className="inline-tip">{progressError}</p>}
      {!loading && !error && (
        <Suspense fallback={<div className="card">图表加载中...</div>}>
          <DashboardCharts progress={progress} />
        </Suspense>
      )}
    </section>
  )
}
