import dayjs from 'dayjs'
import { CSSProperties } from 'react'
import { Task } from '../types'
import { STATUS_META } from '../constants/status'

interface Props {
  tasks?: Task[] | null
}

export function GanttChart({ tasks }: Props) {
  const safeTasks = Array.isArray(tasks) ? tasks : []
  if (!safeTasks.length) return <div className="card">暂无甘特图数据</div>

  const validTasks = safeTasks.filter((t) => t && t.startAt && t.endAt)
  if (!validTasks.length) return <div className="card">请为任务设置开始与结束时间</div>

  const minDate = dayjs(Math.min(...validTasks.map((t) => dayjs(t.startAt).valueOf())))
  const maxDate = dayjs(Math.max(...validTasks.map((t) => dayjs(t.endAt).valueOf())))
  const totalDays = Math.max(maxDate.diff(minDate, 'day'), 1)

  return (
    <div className="card gantt">
      <h3>甘特图</h3>
      {validTasks.map((task) => {
        const startOffset = dayjs(task.startAt).diff(minDate, 'day')
        const duration = Math.max(dayjs(task.endAt).diff(dayjs(task.startAt), 'day'), 1)
        const progress = Math.max(0, Math.min(100, Number(task.progress || 0)))
        const statusMeta = STATUS_META[task.status] || STATUS_META.pending
        return (
          <div key={task.id} className="gantt-row">
            <span className="gantt-label">
              {task.taskNo} {task.title}
              <em className="status-dot" style={{ background: statusMeta.color }}>{statusMeta.label}</em>
            </span>
            <div className="gantt-track">
              <div
                className="gantt-bar"
                style={{
                  marginLeft: `${(startOffset / totalDays) * 100}%`,
                  width: `${(duration / totalDays) * 100}%`,
                  borderColor: statusMeta.color
                } as CSSProperties}
              >
                <span className="gantt-bar-progress" style={{ width: `${progress}%`, background: statusMeta.color }} />
                <span className="gantt-bar-text">{progress}%</span>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
