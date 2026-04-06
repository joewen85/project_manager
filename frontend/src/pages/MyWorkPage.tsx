import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchData, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { MyWorkData, Task } from '../types'

const emptyMyWorkData = (): MyWorkData => ({
  myTasks: [],
  myCreated: [],
  myParticipate: []
})

const normalizeTasks = (value: unknown) => Array.isArray(value) ? value : []

const normalizeMyWorkData = (value: MyWorkData | null | undefined): MyWorkData => ({
  myTasks: normalizeTasks(value?.myTasks),
  myCreated: normalizeTasks(value?.myCreated),
  myParticipate: normalizeTasks(value?.myParticipate)
})

export function MyWorkPage() {
  const [data, setData] = useState<MyWorkData>(emptyMyWorkData())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const payload = await fetchData<MyWorkData>('/tasks/me')
      setData(normalizeMyWorkData(payload))
    } catch (loadError) {
      setError(readApiError(loadError, '个人工作数据加载失败'))
      setData(emptyMyWorkData())
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const renderTaskList = (tasks: Task[], emptyText: string) => {
    if (tasks.length === 0) return <p className="my-work-empty">{emptyText}</p>
    return (
      <ul className="my-work-list">
        {tasks.map((task) => (
          <li key={task.id}>
            <Link className="my-work-link" to={`/tasks?taskId=${task.id}&view=1`}>
              <span className="my-work-task-no">{task.taskNo || '-'}</span>
              <span className="my-work-task-title">{task.title || '未命名任务'}</span>
            </Link>
          </li>
        ))}
      </ul>
    )
  }

  return (
    <section className="page-section">
      <DataState loading={loading} error={error} onRetry={() => { void load() }} />
      <div className="cards">
        <article className="card metric-card"><p>我的任务</p><strong>{data.myTasks.length}</strong></article>
        <article className="card metric-card"><p>我的创建</p><strong>{data.myCreated.length}</strong></article>
        <article className="card metric-card"><p>我的参与</p><strong>{data.myParticipate.length}</strong></article>
      </div>
      <div className="card">
        <h3>我的任务</h3>
        {renderTaskList(data.myTasks, '暂无任务')}
      </div>
      <div className="card">
        <h3>我的创建</h3>
        {renderTaskList(data.myCreated, '暂无创建任务')}
      </div>
      <div className="card">
        <h3>我的参与</h3>
        {renderTaskList(data.myParticipate, '暂无参与任务')}
      </div>
    </section>
  )
}
