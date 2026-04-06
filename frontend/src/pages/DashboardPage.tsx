import { lazy, Suspense, useEffect, useState } from 'react'
import { fetchArray, fetchData, hasAnyPermission, readApiError } from '../services/api'
import { STATUS_META, STATUS_ORDER } from '../constants/status'
import { Status } from '../types'
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

export function DashboardPage() {
  const permissions = usePermissions()

  const [stats, setStats] = useState<DashboardStats>()
  const [progress, setProgress] = useState<DashboardProgressItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const canViewUsers = hasAnyPermission(['users.read', 'users.create', 'users.update', 'users.delete', 'users.write', 'rbac.manage'], permissions)

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [statsData, raw] = await Promise.all([
        fetchData<DashboardStats>('/stats/dashboard'),
        fetchArray<DashboardProgressRaw>('/tasks/progress-list')
      ])
      setStats(statsData)

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
      </div>
      {!loading && !error && (
        <Suspense fallback={<div className="card">图表加载中...</div>}>
          <DashboardCharts progress={progress} />
        </Suspense>
      )}
    </section>
  )
}
