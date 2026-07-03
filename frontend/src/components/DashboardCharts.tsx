import { Area, AreaChart, Bar, BarChart, CartesianGrid, Cell, Legend, Pie, PieChart, RadialBar, RadialBarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { Status } from '../types'

export interface DashboardProgressItem {
  status: Status
  count: number
  statusLabel: string
  fill: string
}

export interface DashboardHealthChartItem {
  name: string
  value: number
  fill: string
}

export interface DashboardProjectProgressItem {
  name: string
  completionRate: number
  score: number
  fill: string
}

export interface DashboardRiskChartItem {
  name: string
  value: number
  fill: string
}

export interface DashboardWorkloadChartItem {
  name: string
  utilization: number
  taskCount: number
  fill: string
}

export interface DashboardTrendItem {
  stage: string
  count: number
  cumulative: number
}

export interface DashboardDeliveryItem {
  name: string
  value: number
  fill: string
}

interface DashboardChartsProps {
  progress: DashboardProgressItem[]
  completionRate: number
  completedTasks: number
  totalTasks: number
  health: DashboardHealthChartItem[]
  projectProgress: DashboardProjectProgressItem[]
  riskItems: DashboardRiskChartItem[]
  workload: DashboardWorkloadChartItem[]
  trend: DashboardTrendItem[]
  delivery: DashboardDeliveryItem[]
}

export function DashboardCharts({
  progress,
  completionRate,
  completedTasks,
  totalTasks,
  health,
  projectProgress,
  riskItems,
  workload,
  trend,
  delivery
}: DashboardChartsProps) {
  const gaugeData = [{ name: '完成率', value: completionRate, fill: '#2563eb' }]

  return (
    <div className="dashboard-chart-grid">
      <div className="card chart-card data-viz-card dashboard-gauge-card">
        <div className="chart-card-header">
          <h3>总体完成率</h3>
          <span>{completedTasks}/{totalTasks} 任务</span>
        </div>
        <div className="data-viz-surface dashboard-gauge-surface">
          <ResponsiveContainer width="100%" height={220}>
            <RadialBarChart data={gaugeData} cx="50%" cy="70%" innerRadius="72%" outerRadius="100%" startAngle={180} endAngle={0}>
              <RadialBar dataKey="value" background cornerRadius={12} />
              <Tooltip />
            </RadialBarChart>
          </ResponsiveContainer>
          <div className="dashboard-gauge-value">
            <strong>{completionRate.toFixed(1)}%</strong>
            <span>完成进度</span>
          </div>
        </div>
      </div>

      <div className="card chart-card data-viz-card">
        <div className="chart-card-header">
          <h3>任务阶段分布</h3>
          <span>{totalTasks} 项任务</span>
        </div>
        <div className="data-viz-surface">
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
      </div>

      <div className="card chart-card data-viz-card">
        <div className="chart-card-header">
          <h3>任务状态占比</h3>
          <span>按阶段聚合</span>
        </div>
        <div className="data-viz-surface">
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

      <div className="card chart-card data-viz-card dashboard-wide-chart">
        <div className="chart-card-header">
          <h3>项目完成率排行</h3>
          <span>按可见项目统计</span>
        </div>
        <div className="data-viz-surface">
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={projectProgress} layout="vertical" margin={{ left: 18, right: 18 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#dbeafe" />
              <XAxis type="number" domain={[0, 100]} tickFormatter={(value: number) => `${value}%`} />
              <YAxis dataKey="name" type="category" width={116} />
              <Tooltip />
              <Legend />
              <Bar dataKey="completionRate" name="完成率 %" radius={[0, 8, 8, 0]}>
                {projectProgress.map((item) => (
                  <Cell key={item.name} fill={item.fill} />
                ))}
              </Bar>
              <Bar dataKey="score" name="健康分" fill="#64748b" radius={[0, 8, 8, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="card chart-card data-viz-card">
        <div className="chart-card-header">
          <h3>项目健康占比</h3>
          <span>健康 / 关注 / 高风险</span>
        </div>
        <div className="data-viz-surface">
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie data={health} dataKey="value" nameKey="name" innerRadius={58} outerRadius={92}>
                {health.map((item) => (
                  <Cell key={item.name} fill={item.fill} />
                ))}
              </Pie>
              <Legend />
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="card chart-card data-viz-card">
        <div className="chart-card-header">
          <h3>风险问题占比</h3>
          <span>风险、问题与逾期</span>
        </div>
        <div className="data-viz-surface">
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={riskItems}>
              <CartesianGrid strokeDasharray="3 3" stroke="#dbeafe" />
              <XAxis dataKey="name" />
              <YAxis allowDecimals={false} />
              <Tooltip />
              <Bar dataKey="value" name="数量" radius={[8, 8, 0, 0]}>
                {riskItems.map((item) => (
                  <Cell key={item.name} fill={item.fill} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="card chart-card data-viz-card dashboard-wide-chart">
        <div className="chart-card-header">
          <h3>成员负载使用率</h3>
          <span>Top 8</span>
        </div>
        <div className="data-viz-surface">
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={workload} margin={{ left: 8, right: 18 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#dbeafe" />
              <XAxis dataKey="name" />
              <YAxis tickFormatter={(value: number) => `${value}%`} />
              <Tooltip />
              <Legend />
              <Bar dataKey="utilization" name="使用率 %" radius={[8, 8, 0, 0]}>
                {workload.map((item) => (
                  <Cell key={item.name} fill={item.fill} />
                ))}
              </Bar>
              <Bar dataKey="taskCount" name="任务数" fill="#38bdf8" radius={[8, 8, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="dashboard-split-row">
        <div className="card chart-card data-viz-card">
          <div className="chart-card-header">
            <h3>任务推进面积图</h3>
            <span>阶段累计视图</span>
          </div>
          <div className="data-viz-surface">
            <ResponsiveContainer width="100%" height={300}>
              <AreaChart data={trend}>
                <CartesianGrid strokeDasharray="3 3" stroke="#dbeafe" />
                <XAxis dataKey="stage" />
                <YAxis allowDecimals={false} />
                <Tooltip />
                <Legend />
                <Area type="monotone" dataKey="cumulative" name="累计任务" stroke="#2563eb" fill="#bfdbfe" />
                <Area type="monotone" dataKey="count" name="当前阶段" stroke="#16a34a" fill="#bbf7d0" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="card chart-card data-viz-card">
          <div className="chart-card-header">
            <h3>任务交付关注项</h3>
            <span>全部可见项目聚合</span>
          </div>
          <div className="data-viz-surface">
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={delivery}>
                <CartesianGrid strokeDasharray="3 3" stroke="#dbeafe" />
                <XAxis dataKey="name" />
                <YAxis allowDecimals={false} />
                <Tooltip />
                <Bar dataKey="value" name="任务数" radius={[8, 8, 0, 0]}>
                  {delivery.map((item) => (
                    <Cell key={item.name} fill={item.fill} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  )
}
