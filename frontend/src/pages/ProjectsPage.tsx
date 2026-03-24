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
    void api.get('/projects').then((res) => {
      setProjects(res.data)
      if (res.data.length > 0) setSelected(res.data[0].id)
    })
  }, [])

  useEffect(() => {
    if (!selected) return
    void api.get(`/projects/${selected}/gantt`).then((res) => setGantt(res.data))
    void api.get(`/projects/${selected}/task-tree`).then((res) => setTree(res.data))
  }, [selected])

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
        <table><thead><tr><th>编码</th><th>名称</th><th>描述</th></tr></thead><tbody>
          {projects.map((p) => <tr key={p.id}><td>{p.code}</td><td>{p.name}</td><td>{p.description}</td></tr>)}
        </tbody></table>
      </div>
      <GanttChart tasks={gantt} />
      <TaskTree tasks={tree} />
    </section>
  )
}
