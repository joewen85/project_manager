import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { CalendarDays, ChevronLeft, ChevronRight, Download } from 'lucide-react'
import { api, fetchData, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { STATUS_META } from '../constants/status'
import { Status, TaskCalendarItem, TaskCalendarResponse } from '../types'
import { formatDateTime } from '../utils/datetime'

type CalendarMode = 'month' | 'week' | 'day'

const modeLabel: Record<CalendarMode, string> = {
  month: '月',
  week: '周',
  day: '日'
}

const priorityLabel = {
  high: '高',
  medium: '中',
  low: '低'
}

const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate(), 0, 0, 0, 0)
const endOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59, 999)
const addDays = (date: Date, days: number) => new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)
const addMonths = (date: Date, months: number) => new Date(date.getFullYear(), date.getMonth() + months, 1)
const startOfWeek = (date: Date) => {
  const day = startOfDay(date)
  const offset = day.getDay() === 0 ? 6 : day.getDay() - 1
  return addDays(day, -offset)
}
const toDateKey = (date: Date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const datesBetween = (start: Date, end: Date) => {
  const dates: Date[] = []
  let cursor = startOfDay(start)
  const last = startOfDay(end)
  while (cursor <= last) {
    dates.push(cursor)
    cursor = addDays(cursor, 1)
  }
  return dates
}

const getRange = (mode: CalendarMode, anchor: Date) => {
  if (mode === 'day') {
    return { start: startOfDay(anchor), end: endOfDay(anchor) }
  }
  if (mode === 'week') {
    const start = startOfWeek(anchor)
    return { start, end: endOfDay(addDays(start, 6)) }
  }
  const monthStart = new Date(anchor.getFullYear(), anchor.getMonth(), 1)
  const start = startOfWeek(monthStart)
  return { start, end: endOfDay(addDays(start, 41)) }
}

const getTitle = (mode: CalendarMode, anchor: Date, range: { start: Date; end: Date }) => {
  if (mode === 'month') return anchor.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long' })
  if (mode === 'week') return `${range.start.toLocaleDateString('zh-CN')} - ${range.end.toLocaleDateString('zh-CN')}`
  return anchor.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long' })
}

const moveAnchor = (mode: CalendarMode, anchor: Date, direction: number) => {
  if (mode === 'month') return addMonths(anchor, direction)
  if (mode === 'week') return addDays(anchor, direction * 7)
  return addDays(anchor, direction)
}

const itemStart = (item: TaskCalendarItem) => item.startAt ? new Date(item.startAt) : (item.endAt ? new Date(item.endAt) : undefined)
const itemEnd = (item: TaskCalendarItem) => item.endAt ? new Date(item.endAt) : itemStart(item)
const itemDateKey = (item: TaskCalendarItem) => {
  const start = itemStart(item)
  return start ? `${toDateKey(start)}-${item.id}` : String(item.id)
}
const itemOnDate = (item: TaskCalendarItem, date: Date) => {
  const start = itemStart(item)
  const end = itemEnd(item)
  if (!start || !end) return false
  const dayStart = startOfDay(date)
  const dayEnd = endOfDay(date)
  return start <= dayEnd && end >= dayStart
}
const sortCalendarItems = (items: TaskCalendarItem[]) => [...items].sort((left, right) => {
  const leftTime = itemStart(left)?.getTime() ?? 0
  const rightTime = itemStart(right)?.getTime() ?? 0
  if (leftTime !== rightTime) return leftTime - rightTime
  return left.id - right.id
})

const normalizeItems = (value: TaskCalendarResponse | null | undefined) => Array.isArray(value?.items) ? value.items : []

export function CalendarPage() {
  const [mode, setMode] = useState<CalendarMode>('month')
  const [anchor, setAnchor] = useState(() => new Date())
  const [items, setItems] = useState<TaskCalendarItem[]>([])
  const [loading, setLoading] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState('')
  const [downloadError, setDownloadError] = useState('')

  const range = useMemo(() => getRange(mode, anchor), [mode, anchor])
  const title = useMemo(() => getTitle(mode, anchor, range), [mode, anchor, range])
  const calendarDays = useMemo(() => datesBetween(range.start, range.end), [range])
  const sortedItems = useMemo(() => sortCalendarItems(items), [items])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const payload = await fetchData<TaskCalendarResponse>('/tasks/calendar', {
        start: range.start.toISOString(),
        end: range.end.toISOString(),
        mine: true
      })
      setItems(normalizeItems(payload))
    } catch (loadError) {
      setError(readApiError(loadError, '日程加载失败'))
      setItems([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [range.start, range.end])

  const downloadICS = async () => {
    try {
      setDownloading(true)
      setDownloadError('')
      const response = await api.get('/tasks/calendar.ics', {
        params: {
          start: range.start.toISOString(),
          end: range.end.toISOString(),
          mine: true
        },
        responseType: 'blob'
      })
      const blob = new Blob([response.data], { type: 'text/calendar;charset=utf-8' })
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `task-calendar-${mode}.ics`
      link.click()
      window.URL.revokeObjectURL(url)
    } catch (downloadFailure) {
      setDownloadError(readApiError(downloadFailure, '日历导出失败'))
    } finally {
      setDownloading(false)
    }
  }

  const renderItem = (item: TaskCalendarItem, compact = false) => {
    const meta = STATUS_META[item.status as Status] || STATUS_META.pending
    return (
      <Link key={itemDateKey(item)} className={`calendar-task-link${compact ? ' compact' : ''}`} to={`/tasks?taskId=${item.id}&view=1`}>
        <span className="calendar-task-status" style={{ background: meta.color }} />
        <span className="calendar-task-main">
          <strong>{item.taskNo || `#${item.id}`} · {item.title || '未命名任务'}</strong>
          {!compact && (
            <span>
              {item.projectName || '-'} · {meta.label} · 优先级 {priorityLabel[item.priority || 'high']}
            </span>
          )}
        </span>
        {!compact && <span className="calendar-task-time">{formatDateTime(item.startAt)} - {formatDateTime(item.endAt)}</span>}
      </Link>
    )
  }

  return (
    <section className="page-section">
      <div className="calendar-toolbar card">
        <div className="calendar-title-wrap">
          <CalendarDays size={20} />
          <h3>{title}</h3>
        </div>
        <div className="calendar-actions">
          <div className="calendar-view-toggle" role="group" aria-label="日程视图">
            {(Object.keys(modeLabel) as CalendarMode[]).map((key) => (
              <button key={key} type="button" className={mode === key ? 'active' : ''} onClick={() => setMode(key)}>{modeLabel[key]}</button>
            ))}
          </div>
          <button type="button" className="btn secondary calendar-icon-btn" aria-label="上一段日程" onClick={() => setAnchor((prev) => moveAnchor(mode, prev, -1))}>
            <ChevronLeft size={16} />
          </button>
          <button type="button" className="btn secondary" onClick={() => setAnchor(new Date())}>今天</button>
          <button type="button" className="btn secondary calendar-icon-btn" aria-label="下一段日程" onClick={() => setAnchor((prev) => moveAnchor(mode, prev, 1))}>
            <ChevronRight size={16} />
          </button>
          <button type="button" className="btn" onClick={() => { void downloadICS() }} disabled={downloading}>
            <Download size={16} />
            {downloading ? '导出中...' : 'iCal'}
          </button>
        </div>
      </div>
      {downloadError && <p className="error">{downloadError}</p>}
      <DataState loading={loading} error={error} empty={!loading && !error && sortedItems.length === 0} emptyText="暂无日程" onRetry={() => { void load() }} />
      {!loading && !error && mode === 'month' && sortedItems.length > 0 && (
        <div className="calendar-month-grid">
          {['一', '二', '三', '四', '五', '六', '日'].map((label) => <div key={label} className="calendar-weekday">{label}</div>)}
          {calendarDays.map((date) => {
            const dayItems = sortedItems.filter((item) => itemOnDate(item, date))
            const outsideMonth = date.getMonth() !== anchor.getMonth()
            return (
              <div key={toDateKey(date)} className={`calendar-day-cell${outsideMonth ? ' muted' : ''}`}>
                <span className="calendar-day-number">{date.getDate()}</span>
                <div className="calendar-day-items">
                  {dayItems.slice(0, 3).map((item) => renderItem(item, true))}
                  {dayItems.length > 3 && <span className="calendar-more">+{dayItems.length - 3}</span>}
                </div>
              </div>
            )
          })}
        </div>
      )}
      {!loading && !error && mode !== 'month' && sortedItems.length > 0 && (
        <div className="calendar-agenda">
          {calendarDays.map((date) => {
            const dayItems = sortedItems.filter((item) => itemOnDate(item, date))
            if (dayItems.length === 0) return null
            return (
              <section key={toDateKey(date)} className="calendar-agenda-day">
                <h3>{date.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' })}</h3>
                <div className="calendar-agenda-list">
                  {dayItems.map((item) => renderItem(item))}
                </div>
              </section>
            )
          })}
        </div>
      )}
    </section>
  )
}
