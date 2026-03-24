import { useEffect, useState } from 'react'
import { Bar, BarChart, CartesianGrid, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { api } from '../services/api'

interface DashboardStats {
  users: number
  projects: number
  tasks: number
  completedTasks: number
  completionRate: number
}

interface ProgressItem {
  status: string
  count: number
}

export function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats>()
  const [progress, setProgress] = useState<ProgressItem[]>([])

  useEffect(() => {
    void api.get('/stats/dashboard').then((res) => setStats(res.data))
    void api.get('/tasks/progress-list').then((res) => {
      const raw = Array.isArray(res.data) ? res.data : res.data?.list
      if (!Array.isArray(raw)) {
        setProgress([])
        return
      }
      const sanitized = raw
        .filter((item) => item && typeof item === 'object')
        .map((item) => ({
          status: String(item.status ?? 'unknown'),
          count: Number(item.count ?? 0)
        }))
      setProgress(sanitized)
    })
  }, [])

  return (
    <section>
      <h2>统计分析</h2>
      <div className="cards">
        <article className="card"><p>用户</p><strong>{stats?.users ?? 0}</strong></article>
        <article className="card"><p>项目</p><strong>{stats?.projects ?? 0}</strong></article>
        <article className="card"><p>任务</p><strong>{stats?.tasks ?? 0}</strong></article>
        <article className="card"><p>完成率</p><strong>{((stats?.completionRate ?? 0) * 100).toFixed(1)}%</strong></article>
      </div>
      <div className="charts">
        <div className="card chart-card">
          <h3>进度列表</h3>
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={progress}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="status" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="count" fill="#2563EB" />
            </BarChart>
          </ResponsiveContainer>
        </div>
        <div className="card chart-card">
          <h3>任务状态占比</h3>
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie data={progress} dataKey="count" nameKey="status" outerRadius={90} fill="#f97316" />
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>
    </section>
  )
}
