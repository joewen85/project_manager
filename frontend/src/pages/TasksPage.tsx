import { FormEvent, useEffect, useState } from 'react'
import { api } from '../services/api'

const statusLabel: Record<string, string> = {
  pending: '待处理',
  queued: '排队中',
  processing: '处理中',
  completed: '已完成'
}

interface TaskForm {
  id?: number
  taskNo: string
  title: string
  description: string
  status: string
  progress: number
  startAt: string
  endAt: string
  projectId: number
  parentId?: number
  assigneeIds: number[]
}

const initialForm: TaskForm = {
  taskNo: '',
  title: '',
  description: '',
  status: 'pending',
  progress: 0,
  startAt: '',
  endAt: '',
  projectId: 0,
  assigneeIds: []
}

export function TasksPage() {
  const [tasks, setTasks] = useState<any[]>([])
  const [users, setUsers] = useState<any[]>([])
  const [projects, setProjects] = useState<any[]>([])
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [form, setForm] = useState<TaskForm>(initialForm)

  const load = async () => {
    const query = new URLSearchParams({ page: '1', pageSize: '50' })
    if (keyword) query.set('keyword', keyword)
    if (status) query.set('status', status)

    const [taskRes, userRes, projectRes] = await Promise.all([
      api.get(`/tasks?${query.toString()}`),
      api.get('/users?page=1&pageSize=100', { silent: true } as any).catch(() => ({ data: { list: [] } })),
      api.get('/projects?page=1&pageSize=100')
    ])

    setTasks(taskRes.data.list ?? taskRes.data)
    setUsers(userRes.data.list ?? [])
    const projectList = projectRes.data.list ?? []
    setProjects(projectList)
    if (!form.projectId && projectList.length > 0) {
      setForm((prev) => ({ ...prev, projectId: projectList[0].id }))
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const payload = {
      ...form,
      startAt: form.startAt ? new Date(form.startAt).toISOString() : '',
      endAt: form.endAt ? new Date(form.endAt).toISOString() : '',
      parentId: form.parentId || undefined
    }

    if (form.id) {
      await api.put(`/tasks/${form.id}`, payload)
    } else {
      await api.post('/tasks', payload)
    }

    setForm(initialForm)
    await load()
  }

  const edit = (item: any) => {
    setForm({
      id: item.id,
      taskNo: item.taskNo,
      title: item.title,
      description: item.description,
      status: item.status,
      progress: item.progress,
      startAt: item.startAt ? item.startAt.slice(0, 16) : '',
      endAt: item.endAt ? item.endAt.slice(0, 16) : '',
      projectId: item.projectId,
      parentId: item.parentId,
      assigneeIds: (item.assignees || []).map((user: any) => user.id)
    })
  }

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该任务？')) return
    await api.delete(`/tasks/${id}`)
    await load()
  }

  return (
    <section>
      <h2>任务列表</h2>

      <form className="card form-grid" onSubmit={submit}>
        <h3>{form.id ? '编辑任务' : '新增任务'}</h3>
        <label htmlFor="task-no">任务编号（可空自动生成）</label>
        <input id="task-no" value={form.taskNo} onChange={(e) => setForm((prev) => ({ ...prev, taskNo: e.target.value }))} />
        <label htmlFor="task-title">标题</label>
        <input id="task-title" value={form.title} onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))} required />
        <label htmlFor="task-description">描述</label>
        <input id="task-description" value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />

        <label htmlFor="task-project">项目</label>
        <select id="task-project" value={form.projectId} onChange={(e) => setForm((prev) => ({ ...prev, projectId: Number(e.target.value) }))} required>
          {projects.map((project) => <option key={project.id} value={project.id}>{project.code} - {project.name}</option>)}
        </select>

        <label htmlFor="task-parent">父任务ID（可选）</label>
        <input id="task-parent" type="number" value={form.parentId ?? ''} onChange={(e) => setForm((prev) => ({ ...prev, parentId: e.target.value ? Number(e.target.value) : undefined }))} />

        <label htmlFor="task-status">状态</label>
        <select id="task-status" value={form.status} onChange={(e) => setForm((prev) => ({ ...prev, status: e.target.value }))}>
          {Object.keys(statusLabel).map((key) => <option key={key} value={key}>{statusLabel[key]}</option>)}
        </select>

        <label htmlFor="task-progress">进度</label>
        <input id="task-progress" type="number" min={0} max={100} value={form.progress} onChange={(e) => setForm((prev) => ({ ...prev, progress: Number(e.target.value) }))} />

        <label htmlFor="task-start">开始时间</label>
        <input id="task-start" type="datetime-local" value={form.startAt} onChange={(e) => setForm((prev) => ({ ...prev, startAt: e.target.value }))} />
        <label htmlFor="task-end">结束时间</label>
        <input id="task-end" type="datetime-local" value={form.endAt} onChange={(e) => setForm((prev) => ({ ...prev, endAt: e.target.value }))} />

        <label htmlFor="task-assignees">执行人（多人）</label>
        <select id="task-assignees" multiple value={form.assigneeIds.map(String)} onChange={(event) => {
          const selectedIds = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
          setForm((prev) => ({ ...prev, assigneeIds: selectedIds }))
        }}>
          {users.map((user) => <option key={user.id} value={user.id}>{user.name}</option>)}
        </select>

        <div className="row-actions">
          <button type="submit" className="btn">保存任务</button>
          <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
        </div>
      </form>

      <div className="card form-grid">
        <h3>筛选</h3>
        <input value={keyword} placeholder="编号/标题/描述" onChange={(e) => setKeyword(e.target.value)} />
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">全部状态</option>
          {Object.keys(statusLabel).map((key) => <option key={key} value={key}>{statusLabel[key]}</option>)}
        </select>
        <div className="row-actions"><button className="btn" onClick={() => void load()}>查询</button></div>
      </div>

      <div className="card">
        <table><thead><tr><th>任务编号</th><th>标题</th><th>状态</th><th>进度</th><th>开始</th><th>结束</th><th>执行人</th><th>操作</th></tr></thead><tbody>
          {tasks.map((task) => (
            <tr key={task.id}>
              <td>{task.taskNo}</td><td>{task.title}</td><td>{statusLabel[task.status]}</td><td>{task.progress}%</td>
              <td>{task.startAt ? new Date(task.startAt).toLocaleString() : '-'}</td>
              <td>{task.endAt ? new Date(task.endAt).toLocaleString() : '-'}</td>
              <td>{(task.assignees || []).length}</td>
              <td>
                <button className="btn secondary" onClick={() => edit(task)}>编辑</button>
                <button className="btn danger" onClick={() => onDelete(task.id)}>删除</button>
              </td>
            </tr>
          ))}
        </tbody></table>
      </div>
    </section>
  )
}
