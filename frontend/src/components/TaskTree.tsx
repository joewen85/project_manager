import { ReactElement } from 'react'
import { Task } from '../types'

interface Props {
  tasks?: Task[] | null
}

const renderNode = (task: Task, level = 0): ReactElement => (
  <li key={task.id} style={{ paddingLeft: level * 14 }}>
    <span>{task.taskNo} - {task.title}</span>
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
