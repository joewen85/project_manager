import { useEffect, useState } from 'react'
import { api } from '../services/api'
import { GanttChart } from '../components/GanttChart'
import { TaskTree } from '../components/TaskTree'
import { Task } from '../types'

export function ProjectsPage() {
  const [projects, setProjects] = useState<any[]>([])
  const [selected, setSelected] = useState<number>()
  const [gantt, setGantt] = useState<Task[]>([])
  const [tree, setTree] = useState<Task[]>([])

  useEffect(() => {
    void api.get('/projects?page=1&pageSize=50').then((res) => {
      const list = res.data.list ?? res.data
      setProjects(list)
      if (list.length > 0) setSelected(list[0].id)
    })
  }, [])

  useEffect(() => {
    if (!selected) return
    void api.get(`/projects/${selected}/gantt`).then((res) => setGantt(res.data))
    void api.get(`/projects/${selected}/task-tree`).then((res) => setTree(res.data))
  }, [selected])

  const onDelete = async (id: number) => {
    if (!confirm('确认删除该项目？')) return
    await api.delete(`/projects/${id}`)
    const res = await api.get('/projects?page=1&pageSize=50')
    const list = res.data.list ?? res.data
    setProjects(list)
    setSelected(list[0]?.id)
  }

  return (
    <section>
      <h2>项目列表 / 项目详情</h2>
      <div className="card">
        <label htmlFor="project-select">选择项目</label>
        <select id="project-select" value={selected} onChange={(e) => setSelected(Number(e.target.value))}>
          {projects.map((p) => <option key={p.id} value={p.id}>{p.code} - {p.name}</option>)}
        </select>
      </div>
      <div className="card">
        <table><thead><tr><th>编码</th><th>名称</th><th>描述</th><th>操作</th></tr></thead><tbody>
          {projects.map((p) => <tr key={p.id}><td>{p.code}</td><td>{p.name}</td><td>{p.description}</td><td><button className="btn danger" onClick={() => onDelete(p.id)}>删除</button></td></tr>)}
        </tbody></table>
      </div>
      <GanttChart tasks={gantt} />
      <TaskTree tasks={tree} />
    </section>
  )
}
