import { FormEvent, useMemo, useState } from 'react'
import { Sparkles } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api, readApiError } from '../services/api'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { RemoteProjectSelect } from '../components/RemoteProjectSelect'
import { AIDraftResponse, AISourceRef, AITaskBreakdownResponse, TaskPriority } from '../types'

type AssistantMode = 'weekly_report' | 'risk_summary' | 'task_breakdown'

interface AssistantForm {
  mode: AssistantMode
  projectId: string
  weekStart: string
  weekEnd: string
  title: string
  description: string
}

const initialForm: AssistantForm = {
  mode: 'weekly_report',
  projectId: '',
  weekStart: '',
  weekEnd: '',
  title: '',
  description: ''
}

const modeLabel: Record<AssistantMode, string> = {
  weekly_report: '周报草稿',
  risk_summary: '风险摘要',
  task_breakdown: '任务拆解'
}

const priorityLabel: Record<TaskPriority, string> = {
  high: '高',
  medium: '中',
  low: '低'
}

const parseOptionalDateTime = (value: string) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

const sourceTypeLabel = (value: string) => {
  switch (value) {
    case 'project':
      return '项目'
    case 'task':
      return '任务'
    case 'task_activity':
      return '任务动态'
    case 'task_comment':
      return '任务评论'
    case 'project_register':
      return '登记项'
    default:
      return value
  }
}

function SourceList({ sources }: { sources: AISourceRef[] }) {
  if (sources.length === 0) return <p className="inline-tip">暂无来源</p>
  return (
    <div className="assistant-source-list">
      {sources.map((source, index) => (
        source.path ? (
          <Link key={`${source.type}-${source.id || index}`} to={source.path} className="assistant-source-item">
            <span>{sourceTypeLabel(source.type)}</span>
            <strong>{source.label || '-'}</strong>
          </Link>
        ) : (
          <div key={`${source.type}-${source.id || index}`} className="assistant-source-item">
            <span>{sourceTypeLabel(source.type)}</span>
            <strong>{source.label || '-'}</strong>
          </div>
        )
      ))}
    </div>
  )
}

export function AssistantPage() {
  const [form, setForm] = useState<AssistantForm>(initialForm)
  const [draftResult, setDraftResult] = useState<AIDraftResponse | null>(null)
  const [taskResult, setTaskResult] = useState<AITaskBreakdownResponse | null>(null)
  const [editableDraft, setEditableDraft] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [copySuccess, setCopySuccess] = useState('')

  const currentSources = useMemo(() => draftResult?.sourceRefs || taskResult?.sourceRefs || [], [draftResult, taskResult])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setCopySuccess('')
    setError('')
    if ((form.mode === 'weekly_report' || form.mode === 'risk_summary') && !form.projectId) {
      setError('请选择项目')
      return
    }
    const weekStart = parseOptionalDateTime(form.weekStart)
    const weekEnd = parseOptionalDateTime(form.weekEnd)
    if (weekStart === null || weekEnd === null) {
      setError('时间格式不正确')
      return
    }
    try {
      setSubmitting(true)
      if (form.mode === 'task_breakdown') {
        const response = await api.post<AITaskBreakdownResponse>('/ai/task-breakdown', {
          projectId: form.projectId ? Number(form.projectId) : 0,
          title: form.title,
          description: form.description
        })
        setTaskResult(response.data)
        setDraftResult(null)
        setEditableDraft('')
        return
      }
      const path = form.mode === 'weekly_report' ? '/ai/project-weekly-report' : '/ai/project-risk-summary'
      const response = await api.post<AIDraftResponse>(path, {
        projectId: Number(form.projectId),
        weekStart,
        weekEnd
      })
      setDraftResult(response.data)
      setTaskResult(null)
      setEditableDraft(response.data.draft || '')
    } catch (submitError) {
      setError(readApiError(submitError, 'AI 助理生成失败'))
      setDraftResult(null)
      setTaskResult(null)
      setEditableDraft('')
    } finally {
      setSubmitting(false)
    }
  }

  const copyDraft = async () => {
    const content = draftResult ? editableDraft : JSON.stringify(taskResult?.tasks || [], null, 2)
    if (!content.trim()) return
    try {
      await navigator.clipboard.writeText(content)
      setCopySuccess('已复制')
    } catch {
      setCopySuccess('复制失败')
    }
  }

  return (
    <section className="page-section">
      <div className="assistant-grid">
        <form className="card assistant-control-panel" onSubmit={submit}>
          <div className="report-preview-title"><Sparkles size={18} /><h3>AI 助理</h3></div>
          <label htmlFor="assistant-mode">类型</label>
          <select id="assistant-mode" value={form.mode} onChange={(event) => {
            setForm((prev) => ({ ...prev, mode: event.target.value as AssistantMode }))
            setDraftResult(null)
            setTaskResult(null)
            setEditableDraft('')
            setError('')
          }}>
            {(Object.keys(modeLabel) as AssistantMode[]).map((mode) => <option key={mode} value={mode}>{modeLabel[mode]}</option>)}
          </select>

          <label className={form.mode === 'task_breakdown' ? '' : 'required-label'} htmlFor="assistant-project">项目</label>
          <RemoteProjectSelect
            value={form.projectId}
            onChange={(value) => setForm((prev) => ({ ...prev, projectId: value }))}
            ariaLabel="选择 AI 助理项目"
            defaultOptionLabel={form.mode === 'task_breakdown' ? '不指定项目' : '请选择项目'}
          />

          {form.mode === 'weekly_report' && (
            <>
              <label htmlFor="assistant-week-start">开始时间</label>
              <DateTimeQuickField inputId="assistant-week-start" value={form.weekStart} onChange={(value) => setForm((prev) => ({ ...prev, weekStart: value }))} />
              <label htmlFor="assistant-week-end">结束时间</label>
              <DateTimeQuickField inputId="assistant-week-end" value={form.weekEnd} onChange={(value) => setForm((prev) => ({ ...prev, weekEnd: value }))} />
            </>
          )}

          {form.mode === 'task_breakdown' && (
            <>
              <label htmlFor="assistant-title">标题</label>
              <input id="assistant-title" value={form.title} onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))} />
              <label htmlFor="assistant-description">描述</label>
              <textarea id="assistant-description" rows={5} value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
            </>
          )}

          <div className="row-actions">
            <button className="btn" type="submit" disabled={submitting}>{submitting ? '生成中...' : '生成草稿'}</button>
            <button className="btn secondary" type="button" onClick={() => {
              setForm(initialForm)
              setDraftResult(null)
              setTaskResult(null)
              setEditableDraft('')
              setError('')
              setCopySuccess('')
            }}>重置</button>
          </div>
          {error && <p className="error">{error}</p>}
        </form>

        <div className="assistant-output-panel">
          <article className="card assistant-output-card">
            <div className="dashboard-health-header">
              <h3>{draftResult?.title || taskResult?.title || modeLabel[form.mode]}</h3>
              {(draftResult || taskResult) && <button className="btn secondary" onClick={() => { void copyDraft() }}>复制</button>}
            </div>
            {draftResult ? (
              <>
                <div className="assistant-highlight-grid">
                  {draftResult.highlights.map((item) => <span key={item}>{item}</span>)}
                </div>
                <textarea className="assistant-draft-editor" value={editableDraft} onChange={(event) => setEditableDraft(event.target.value)} />
              </>
            ) : taskResult ? (
              <>
                <p className="table-subtext">{taskResult.summary}</p>
                <div className="assistant-task-list">
                  {taskResult.tasks.map((task) => (
                    <article key={`${task.relativeStartDay}-${task.title}`} className="assistant-task-item">
                      <div>
                        <strong>{task.title}</strong>
                        <p>{task.description}</p>
                      </div>
                      <span>{priorityLabel[task.priority]}</span>
                      <span>第 {task.relativeStartDay} 天</span>
                      <span>{task.durationDays} 天</span>
                      {task.isMilestone && <span>里程碑</span>}
                    </article>
                  ))}
                </div>
              </>
            ) : (
              <p className="inline-tip">暂无草稿</p>
            )}
            {copySuccess && <p className="success">{copySuccess}</p>}
          </article>

          <article className="card assistant-output-card">
            <h3>来源</h3>
            <SourceList sources={currentSources} />
          </article>
        </div>
      </div>
    </section>
  )
}
