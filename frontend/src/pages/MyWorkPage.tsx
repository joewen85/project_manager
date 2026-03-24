import { useEffect, useState } from 'react'
import { api } from '../services/api'

export function MyWorkPage() {
  const [data, setData] = useState<{ myTasks: any[]; myCreated: any[]; myParticipate: any[] }>({ myTasks: [], myCreated: [], myParticipate: [] })
  useEffect(() => { void api.get('/tasks/me').then((res) => setData(res.data)) }, [])

  return (
    <section>
      <h2>个人工作</h2>
      <div className="cards">
        <article className="card"><p>我的任务</p><strong>{data.myTasks.length}</strong></article>
        <article className="card"><p>我的创建</p><strong>{data.myCreated.length}</strong></article>
        <article className="card"><p>我的参与</p><strong>{data.myParticipate.length}</strong></article>
      </div>
      <div className="card"><h3>我的任务编号</h3><p>{data.myTasks.map((t) => t.taskNo).join(' / ') || '暂无'}</p></div>
    </section>
  )
}
