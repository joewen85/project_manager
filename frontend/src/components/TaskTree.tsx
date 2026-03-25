import { ReactElement } from 'react'
import { Task } from '../types'
import { STATUS_META } from '../constants/status'

interface Props {
  tasks?: Task[] | null
}

const renderNode = (task: Task, level = 0): ReactElement => (
  <li key={task.id} style={{ paddingLeft: level * 14 }}>
    <div className="tree-node-wrap">
      <span className="tree-node">
        {task.taskNo} - {task.title}
        <em className="status-dot" style={{ background: (STATUS_META[task.status] || STATUS_META.pending).color }}>
          {(STATUS_META[task.status] || STATUS_META.pending).label}
        </em>
      </span>
      <span className="tree-progress-line">
        <span className="tree-progress-bar" style={{ width: `${Math.max(0, Math.min(100, Number(task.progress || 0)))}%` }} />
      </span>
      <span className="tree-progress-text">{Math.max(0, Math.min(100, Number(task.progress || 0)))}%</span>
    </div>
    {task.children && task.children.length > 0 && (
      <ul>{task.children.map((child) => renderNode(child as Task, level + 1))}</ul>
    )}
  </li>
)

export function TaskTree({ tasks }: Props) {
  const safeTasks = Array.isArray(tasks) ? tasks : []
  return (
    <div className="card">
      <h3>项目分解树结构</h3>
      <ul className="tree">{safeTasks.map((task) => renderNode(task))}</ul>
    </div>
  )
}
