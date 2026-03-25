import { useEffect, useState } from 'react'
import { Bar, BarChart, CartesianGrid, Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { api } from '../services/api'
import { STATUS_META, STATUS_ORDER } from '../constants/status'
import { Status } from '../types'
import { DataState } from '../components/DataState'
import { getPermissions } from '../services/api'

interface DashboardStats {
  users: number
  projects: number
  tasks: number
  completedTasks: number
  completionRate: number
}

interface ProgressItem {
  status: Status
  count: number
  statusLabel: string
  fill: string
}

export function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats>()
  const [progress, setProgress] = useState<ProgressItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [canViewUsers, setCanViewUsers] = useState(() => {
    const permissions = getPermissions()
    return permissions.includes('rbac.manage') || permissions.includes('users.read') || permissions.includes('users.write')
  })

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [statsRes, progressRes] = await Promise.all([
        api.get('/stats/dashboard'),
        api.get('/tasks/progress-list')
      ])
      setStats(statsRes.data)

      const raw = Array.isArray(progressRes.data) ? progressRes.data : progressRes.data?.list
      if (!Array.isArray(raw)) {
        setProgress(STATUS_ORDER.map((status) => ({
          status,
          count: 0,
          statusLabel: STATUS_META[status].label,
          fill: STATUS_META[status].color
        })))
        return
      }
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
    } catch (loadError: any) {
      setError(loadError?.response?.data?.message || '统计数据加载失败')
      setStats(undefined)
      setProgress([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    const timer = window.setInterval(() => {
      const permissions = getPermissions()
      setCanViewUsers(permissions.includes('rbac.manage') || permissions.includes('users.read') || permissions.includes('users.write'))
    }, 5000)
    return () => window.clearInterval(timer)
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
      {!loading && !error && <div className="charts">
        <div className="card chart-card">
          <h3>进度列表</h3>
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={progress}>
              <CartesianGrid strokeDasharray="3 3" stroke="#dbeafe" />
              <XAxis dataKey="statusLabel" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Bar dataKey="count" name="任务数量" radius={[8, 8, 0, 0]}>
                {progress.map((item) => (
                  <Cell key={item.status} fill={item.fill} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
        <div className="card chart-card">
          <h3>任务状态占比</h3>
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie data={progress} dataKey="count" nameKey="statusLabel" outerRadius={90}>
                {progress.map((item) => (
                  <Cell key={`pie-${item.status}`} fill={item.fill} />
                ))}
              </Pie>
              <Legend />
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>}
    </section>
  )
}
