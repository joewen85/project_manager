import { useEffect, useState } from 'react'
import { api } from '../services/api'

const statusLabel: Record<string, string> = {
  pending: '待处理',
  queued: '排队中',
  processing: '处理中',
  completed: '已完成'
}

export function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  const load = () => {
    void api.get('/tasks?page=1&pageSize=50').then((res) => setTasks(res.data.list ?? res.data))
  }
  useEffect(() => { load() }, [])

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该任务？')) return
    await api.delete(`/tasks/${id}`)
    load()
  }

  return (
    <section>
      <h2>任务列表</h2>
      <div className="card">
        <table><thead><tr><th>任务编号</th><th>标题</th><th>状态</th><th>进度</th><th>开始</th><th>结束</th><th>操作</th></tr></thead><tbody>
          {tasks.map((task) => (
            <tr key={task.id}>
              <td>{task.taskNo}</td><td>{task.title}</td><td>{statusLabel[task.status]}</td><td>{task.progress}%</td>
              <td>{task.startAt ? new Date(task.startAt).toLocaleString() : '-'}</td>
              <td>{task.endAt ? new Date(task.endAt).toLocaleString() : '-'}</td>
              <td><button className="btn danger" onClick={() => onDelete(task.id)}>删除</button></td>
            </tr>
          ))}
        </tbody></table>
      </div>
    </section>
  )
}
