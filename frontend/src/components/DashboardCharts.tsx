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
  barFill: string
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

export interface DashboardRegisterStatusItem {
  type: string
  typeLabel: string
  open: number
  in_progress: number
  resolved: number
  closed: number
}

export interface DashboardRegisterSeverityItem {
  severity: string
  severityLabel: string
  risk: number
  issue: number
}

export type WorkloadSortKey = 'utilization' | 'taskCount'

interface DashboardChartsProps {
  progress: DashboardProgressItem[]
  completionRate: number
  completedTasks: number
  totalTasks: number
  health: DashboardHealthChartItem[]
  projectProgress: DashboardProjectProgressItem[]
  riskItems: DashboardRiskChartItem[]
  workload: DashboardWorkloadChartItem[]
  workloadSort: WorkloadSortKey
  onWorkloadSortChange: (key: WorkloadSortKey) => void
  trend: DashboardTrendItem[]
  delivery: DashboardDeliveryItem[]
  canReadRegisters: boolean
  registerStatus: DashboardRegisterStatusItem[]
  registerSeverity: DashboardRegisterSeverityItem[]
}

// Fixed accent colors for secondary series, kept in sync with the design tokens.
const ACCENT_SCORE = '#94a3b8'
const ACCENT_TASKCOUNT = '#f59e0b'
const ACCENT_TREND_CUMULATIVE = '#2563eb'
const ACCENT_TREND_COUNT = '#22c55e'

// Register status / severity series — colors reuse the dashboard palette so the
// risk-issue-decision charts stay visually consistent with health and workload views.
const REGISTER_STATUS_SERIES = [
  { key: 'open', label: '未关闭', color: '#ef4444' },
  { key: 'in_progress', label: '处理中', color: '#f59e0b' },
  { key: 'resolved', label: '已解决', color: '#22c55e' },
  { key: 'closed', label: '已关闭', color: '#94a3b8' }
] as const

const REGISTER_TYPE_SERIES = [
  { key: 'risk', label: '风险', color: '#dc2626' },
  { key: 'issue', label: '问题', color: '#7c3aed' }
] as const

const sanitize = (color: string) => color.replace(/[^a-zA-Z0-9]/g, '')
const gradientId = (color: string) => `bar-grad-${sanitize(color)}`
const areaId = (color: string) => `area-grad-${sanitize(color)}`

// Shared axis presentation — no axis/tick lines, subtle themed ticks (color via CSS).
const axisProps = {
  tickLine: false,
  axisLine: false,
  tickMargin: 8,
  style: { fontSize: 12 }
} as const

// Vertical (or horizontal) gradient defs, one per unique color used in a chart.
function BarGradients({ colors, direction = 'vertical' }: { colors: string[]; direction?: 'vertical' | 'horizontal' }) {
  const unique = Array.from(new Set(colors))
  const coords = direction === 'vertical' ? { x1: 0, y1: 0, x2: 0, y2: 1 } : { x1: 0, y1: 0, x2: 1, y2: 0 }
  return (
    <defs>
      {unique.map((color) => (
        <linearGradient key={color} id={gradientId(color)} {...coords}>
          <stop offset="0%" stopColor={color} stopOpacity={0.95} />
          <stop offset="100%" stopColor={color} stopOpacity={0.5} />
        </linearGradient>
      ))}
    </defs>
  )
}

interface TooltipEntry {
  name?: string
  value?: number | string
  color?: string
  payload?: { fill?: string }
}

// Theme-aware tooltip that replaces the default white Recharts box.
function ChartTooltip({ active, payload, label, unit }: { active?: boolean; payload?: TooltipEntry[]; label?: string; unit?: string }) {
  if (!active || !payload || payload.length === 0) return null
  return (
    <div className="chart-tooltip">
      {label != null && label !== '' && <p className="chart-tooltip-label">{label}</p>}
      {payload.map((entry, index) => (
        <div className="chart-tooltip-row" key={`${entry.name ?? 'item'}-${index}`}>
          <span className="chart-tooltip-dot" style={{ background: entry.color || entry.payload?.fill || 'var(--color-primary)' }} />
          <span className="chart-tooltip-name">{entry.name}</span>
          <span className="chart-tooltip-value">{entry.value}{unit ?? ''}</span>
        </div>
      ))}
    </div>
  )
}

const tooltipCommon = { cursor: { className: 'chart-cursor' }, wrapperStyle: { outline: 'none' } } as const

export function DashboardCharts({
  progress,
  completionRate,
  completedTasks,
  totalTasks,
  health,
  projectProgress,
  riskItems,
  workload,
  workloadSort,
  onWorkloadSortChange,
  trend,
  delivery,
  canReadRegisters,
  registerStatus,
  registerSeverity
}: DashboardChartsProps) {
  const gaugeData = [{ name: '完成率', value: completionRate, fill: 'url(#gauge-grad)' }]

  return (
    <div className="dashboard-chart-grid">
      <div className="card chart-card data-viz-card dashboard-gauge-card">
        <div className="chart-card-header">
          <h3>总体完成率</h3>
          <span>{completedTasks}/{totalTasks} 任务</span>
        </div>
        <div className="data-viz-surface dashboard-gauge-surface">
          <ResponsiveContainer width="100%" height={220}>
            <RadialBarChart data={gaugeData} cx="50%" cy="72%" innerRadius="72%" outerRadius="100%" startAngle={180} endAngle={0} barSize={22}>
              <defs>
                <linearGradient id="gauge-grad" x1="0" y1="0" x2="1" y2="0">
                  <stop offset="0%" stopColor="#2563eb" />
                  <stop offset="100%" stopColor="#60a5fa" />
                </linearGradient>
              </defs>
              <RadialBar dataKey="value" background cornerRadius={12} />
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
            <BarChart data={progress} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
              <BarGradients colors={progress.map((item) => item.fill)} />
              <CartesianGrid vertical={false} strokeDasharray="4 4" />
              <XAxis dataKey="statusLabel" {...axisProps} />
              <YAxis allowDecimals={false} {...axisProps} />
              <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
              <Bar dataKey="count" name="任务数量" radius={[8, 8, 0, 0]} maxBarSize={48}>
                {progress.map((item) => (
                  <Cell key={item.status} fill={`url(#${gradientId(item.fill)})`} />
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
              <Pie data={progress} dataKey="count" nameKey="statusLabel" innerRadius={52} outerRadius={92} paddingAngle={2} cornerRadius={4}>
                {progress.map((item) => (
                  <Cell key={`pie-${item.status}`} fill={item.fill} />
                ))}
              </Pie>
              <Legend iconType="circle" />
              <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
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
            <BarChart data={projectProgress} layout="vertical" margin={{ top: 8, left: 18, right: 18, bottom: 0 }}>
              <BarGradients colors={[...projectProgress.map((item) => item.fill), ACCENT_SCORE]} direction="horizontal" />
              <CartesianGrid horizontal={false} strokeDasharray="4 4" />
              <XAxis type="number" domain={[0, 100]} tickFormatter={(value: number) => `${value}%`} {...axisProps} />
              <YAxis dataKey="name" type="category" width={116} {...axisProps} />
              <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
              <Legend iconType="circle" />
              <Bar dataKey="completionRate" name="完成率 %" radius={[0, 6, 6, 0]} maxBarSize={20}>
                {projectProgress.map((item) => (
                  <Cell key={item.name} fill={`url(#${gradientId(item.fill)})`} />
                ))}
              </Bar>
              <Bar dataKey="score" name="健康分" fill={`url(#${gradientId(ACCENT_SCORE)})`} radius={[0, 6, 6, 0]} maxBarSize={20} />
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
              <Pie data={health} dataKey="value" nameKey="name" innerRadius={58} outerRadius={92} paddingAngle={2} cornerRadius={4}>
                {health.map((item) => (
                  <Cell key={item.name} fill={item.fill} />
                ))}
              </Pie>
              <Legend iconType="circle" />
              <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="dashboard-split-row">
        <div className="card chart-card data-viz-card">
          <div className="chart-card-header">
            <h3>风险问题占比</h3>
            <span>风险、问题与逾期</span>
          </div>
          <div className="data-viz-surface">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={riskItems} layout="vertical" margin={{ top: 8, right: 18, left: 8, bottom: 0 }}>
                <BarGradients colors={riskItems.map((item) => item.fill)} direction="horizontal" />
                <CartesianGrid horizontal={false} strokeDasharray="4 4" />
                <XAxis type="number" allowDecimals={false} {...axisProps} />
                <YAxis type="category" dataKey="name" width={76} {...axisProps} />
                <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
                <Bar dataKey="value" name="数量" radius={[0, 6, 6, 0]} maxBarSize={22}>
                  {riskItems.map((item) => (
                    <Cell key={item.name} fill={`url(#${gradientId(item.fill)})`} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="card chart-card data-viz-card">
          <div className="chart-card-header">
            <h3>成员负载使用率</h3>
            <div className="chart-sort-toggle" role="group" aria-label="成员负载排序方式">
              <button type="button" className={workloadSort === 'utilization' ? 'active' : ''} aria-pressed={workloadSort === 'utilization'} onClick={() => onWorkloadSortChange('utilization')}>使用率</button>
              <button type="button" className={workloadSort === 'taskCount' ? 'active' : ''} aria-pressed={workloadSort === 'taskCount'} onClick={() => onWorkloadSortChange('taskCount')}>任务数</button>
            </div>
          </div>
          <div className="data-viz-surface">
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={workload} layout="vertical" margin={{ top: 8, right: 18, left: 8, bottom: 0 }}>
                <BarGradients colors={[...workload.map((item) => item.barFill), ACCENT_TASKCOUNT]} direction="horizontal" />
                <CartesianGrid horizontal={false} strokeDasharray="4 4" />
                <XAxis type="number" xAxisId="utilization" domain={[0, 100]} tickFormatter={(value: number) => `${value}%`} {...axisProps} />
                <XAxis type="number" xAxisId="taskCount" orientation="top" allowDecimals={false} {...axisProps} />
                <YAxis type="category" dataKey="name" width={76} {...axisProps} />
                <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
                <Legend iconType="circle" />
                <Bar xAxisId="utilization" dataKey="utilization" name="使用率 %" fill="#2563eb" radius={[0, 6, 6, 0]} maxBarSize={14}>
                  {workload.map((item) => (
                    <Cell key={item.name} fill={`url(#${gradientId(item.barFill)})`} />
                  ))}
                </Bar>
                <Bar xAxisId="taskCount" dataKey="taskCount" name="任务数" fill={ACCENT_TASKCOUNT} radius={[0, 6, 6, 0]} maxBarSize={14}>
                  {workload.map((item) => (
                    <Cell key={item.name} fill={`url(#${gradientId(ACCENT_TASKCOUNT)})`} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
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
              <AreaChart data={trend} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
                <defs>
                  <linearGradient id={areaId(ACCENT_TREND_CUMULATIVE)} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={ACCENT_TREND_CUMULATIVE} stopOpacity={0.35} />
                    <stop offset="100%" stopColor={ACCENT_TREND_CUMULATIVE} stopOpacity={0.02} />
                  </linearGradient>
                  <linearGradient id={areaId(ACCENT_TREND_COUNT)} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={ACCENT_TREND_COUNT} stopOpacity={0.35} />
                    <stop offset="100%" stopColor={ACCENT_TREND_COUNT} stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} strokeDasharray="4 4" />
                <XAxis dataKey="stage" {...axisProps} />
                <YAxis allowDecimals={false} {...axisProps} />
                <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
                <Legend iconType="circle" />
                <Area type="monotone" dataKey="cumulative" name="累计任务" stroke={ACCENT_TREND_CUMULATIVE} strokeWidth={2} fill={`url(#${areaId(ACCENT_TREND_CUMULATIVE)})`} activeDot={{ r: 4 }} />
                <Area type="monotone" dataKey="count" name="当前阶段" stroke={ACCENT_TREND_COUNT} strokeWidth={2} fill={`url(#${areaId(ACCENT_TREND_COUNT)})`} activeDot={{ r: 4 }} />
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
              <BarChart data={delivery} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
                <BarGradients colors={delivery.map((item) => item.fill)} />
                <CartesianGrid vertical={false} strokeDasharray="4 4" />
                <XAxis dataKey="name" {...axisProps} />
                <YAxis allowDecimals={false} {...axisProps} />
                <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
                <Bar dataKey="value" name="任务数" radius={[8, 8, 0, 0]} maxBarSize={44}>
                  {delivery.map((item) => (
                    <Cell key={item.name} fill={`url(#${gradientId(item.fill)})`} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {canReadRegisters && (
        <div className="dashboard-split-row">
          <div className="card chart-card data-viz-card">
            <div className="chart-card-header">
              <h3>风险问题决策状态</h3>
              <span>按类型的处理进度</span>
            </div>
            <div className="data-viz-surface">
              <ResponsiveContainer width="100%" height={300}>
                <BarChart data={registerStatus} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
                  <BarGradients colors={REGISTER_STATUS_SERIES.map((series) => series.color)} />
                  <CartesianGrid vertical={false} strokeDasharray="4 4" />
                  <XAxis dataKey="typeLabel" {...axisProps} />
                  <YAxis allowDecimals={false} {...axisProps} />
                  <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
                  <Legend iconType="circle" />
                  {REGISTER_STATUS_SERIES.map((series) => (
                    <Bar key={series.key} dataKey={series.key} name={series.label} stackId="register-status" fill={`url(#${gradientId(series.color)})`} maxBarSize={64} />
                  ))}
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div className="card chart-card data-viz-card">
            <div className="chart-card-header">
              <h3>风险与问题严重度</h3>
              <span>按严重度对比</span>
            </div>
            <div className="data-viz-surface">
              <ResponsiveContainer width="100%" height={300}>
                <BarChart data={registerSeverity} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
                  <BarGradients colors={REGISTER_TYPE_SERIES.map((series) => series.color)} />
                  <CartesianGrid vertical={false} strokeDasharray="4 4" />
                  <XAxis dataKey="severityLabel" {...axisProps} />
                  <YAxis allowDecimals={false} {...axisProps} />
                  <Tooltip {...tooltipCommon} content={<ChartTooltip />} />
                  <Legend iconType="circle" />
                  {REGISTER_TYPE_SERIES.map((series) => (
                    <Bar key={series.key} dataKey={series.key} name={series.label} fill={`url(#${gradientId(series.color)})`} radius={[8, 8, 0, 0]} maxBarSize={36} />
                  ))}
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
