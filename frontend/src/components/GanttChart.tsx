import dayjs from 'dayjs'
import { Task } from '../types'

interface Props {
  tasks: Task[]
}

export function GanttChart({ tasks }: Props) {
  if (!tasks.length) return <div className="card">暂无甘特图数据</div>

  const validTasks = tasks.filter((t) => t.startAt && t.endAt)
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
        return (
          <div key={task.id} className="gantt-row">
            <span>{task.taskNo} {task.title}</span>
            <div className="gantt-track">
              <div
                className="gantt-bar"
                style={{
                  marginLeft: `${(startOffset / totalDays) * 100}%`,
                  width: `${(duration / totalDays) * 100}%`
                }}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}
