import { Bar, BarChart, CartesianGrid, Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { Status } from '../types'

export interface DashboardProgressItem {
  status: Status
  count: number
  statusLabel: string
  fill: string
}

export function DashboardCharts({ progress }: { progress: DashboardProgressItem[] }) {
  return (
    <div className="charts">
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
    </div>
  )
}
