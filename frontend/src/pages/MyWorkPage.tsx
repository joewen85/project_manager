import { useEffect, useState } from 'react'
import { fetchData, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { MyWorkData } from '../types'

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

  return (
    <section className="page-section">
      <DataState loading={loading} error={error} onRetry={() => { void load() }} />
      <div className="cards">
        <article className="card metric-card"><p>我的任务</p><strong>{data.myTasks.length}</strong></article>
        <article className="card metric-card"><p>我的创建</p><strong>{data.myCreated.length}</strong></article>
        <article className="card metric-card"><p>我的参与</p><strong>{data.myParticipate.length}</strong></article>
      </div>
      <div className="card"><h3>我的任务编号</h3><p>{data.myTasks.map((t) => t.taskNo).join(' / ') || '暂无'}</p></div>
    </section>
  )
}
