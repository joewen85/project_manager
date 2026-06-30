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

const healthLabel: Record<ProjectHealth['health'], string> = {
  green: '健康',
  yellow: '关注',
  red: '高风险'
}

export function DashboardPage() {
  const permissions = usePermissions()

  const [stats, setStats] = useState<DashboardStats>()
  const [progress, setProgress] = useState<DashboardProgressItem[]>([])
  const [progressError, setProgressError] = useState('')
  const [projectHealth, setProjectHealth] = useState<ProjectHealth[]>([])
  const [projectHealthError, setProjectHealthError] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const canViewUsers = hasAnyPermission(['users.read', 'users.create', 'users.update', 'users.delete', 'users.write', 'rbac.manage'], permissions)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      setProgressError('')
      setProjectHealthError('')
      const [statsData, raw, healthData] = await Promise.all([
        fetchData<DashboardStats>('/stats/dashboard'),
        fetchArray<DashboardProgressRaw>('/tasks/progress-list', undefined, { silent: true }).catch((progressLoadError) => {
          setProgressError(readApiError(progressLoadError, '任务状态分布加载失败'))
          return []
        }),
        fetchData<ProjectHealthResponse>('/stats/project-health', undefined, { silent: true }).catch((healthError) => {
          setProjectHealthError(readApiError(healthError, '项目健康度加载失败'))
          return { projects: [] }
        })
      ])
      setStats(statsData)
      setProjectHealth(Array.isArray(healthData.projects) ? healthData.projects : [])

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
      {!loading && !error && progressError && <p className="inline-tip">{progressError}</p>}
      {!loading && !error && (
        <Suspense fallback={<div className="card">图表加载中...</div>}>
          <DashboardCharts progress={progress} />
        </Suspense>
      )}
    </section>
  )
}
