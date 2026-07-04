import { Bar, BarChart, CartesianGrid, Cell, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { ReportColumn } from '../types'

// Themed palette reused for report chart series / single-series bars.
const PALETTE = ['#2563eb', '#22c55e', '#f97316', '#a855f7', '#ef4444', '#14b8a6', '#eab308', '#ec4899']

// Coerce a report cell into a chart-usable number. Strips thousands separators
// and percent/currency-ish decorations so "1,280" or "82%" still plot.
const toNumber = (value: unknown): number | null => {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  if (typeof value === 'string') {
    const cleaned = value.replace(/[,%¥$\s]/g, '')
    if (cleaned === '') return null
    const parsed = Number(cleaned)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

const labelText = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '-'
  return String(value)
}

interface ReportResultChartProps {
  columns: ReportColumn[]
  rows: Record<string, unknown>[]
}

export function ReportResultChart({ columns, rows }: ReportResultChartProps) {
  if (columns.length === 0 || rows.length === 0) {
    return <p className="inline-tip">当前筛选下暂无可用于图表的数据</p>
  }

  // First column is the category axis; remaining columns that hold numbers become series.
  const categoryColumn = columns[0]
  const numericColumns = columns.slice(1).filter((column) => rows.some((row) => toNumber(row[column.key]) !== null))

  if (numericColumns.length === 0) {
    return <p className="inline-tip">当前报表没有可用于图表的数值列，请切换为表格展示。</p>
  }

  const data = rows.map((row) => {
    const point: Record<string, string | number> = { __label: labelText(row[categoryColumn.key]) }
    numericColumns.forEach((column) => {
      point[column.key] = toNumber(row[column.key]) ?? 0
    })
    return point
  })

  const singleSeries = numericColumns.length === 1

  return (
    <div className="data-viz-surface report-result-chart">
      <ResponsiveContainer width="100%" height={320}>
        <BarChart data={data} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
          <CartesianGrid vertical={false} strokeDasharray="4 4" />
          <XAxis dataKey="__label" tickLine={false} axisLine={false} tickMargin={8} style={{ fontSize: 12 }} />
          <YAxis allowDecimals={false} tickLine={false} axisLine={false} tickMargin={8} style={{ fontSize: 12 }} />
          <Tooltip cursor={{ className: 'chart-cursor' }} wrapperStyle={{ outline: 'none' }} />
          {!singleSeries && <Legend iconType="circle" />}
          {numericColumns.map((column, index) => (
            <Bar
              key={column.key}
              dataKey={column.key}
              name={column.label}
              radius={[8, 8, 0, 0]}
              maxBarSize={48}
              fill={PALETTE[index % PALETTE.length]}
            >
              {singleSeries && data.map((_, rowIndex) => <Cell key={rowIndex} fill={PALETTE[rowIndex % PALETTE.length]} />)}
            </Bar>
          ))}
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
