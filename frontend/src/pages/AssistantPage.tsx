import { FormEvent, useMemo, useState } from 'react'
import {
  AlertTriangle,
  CalendarRange,
  Check,
  Copy,
  Eye,
  FileText,
  Flag,
  Lightbulb,
  ListChecks,
  Pencil,
  RotateCcw,
  Sparkles
} from 'lucide-react'
import { Link } from 'react-router-dom'
import { api, readApiError } from '../services/api'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { RemoteProjectSelect } from '../components/RemoteProjectSelect'
import { MarkdownView } from '../components/MarkdownView'
import { AIDraftResponse, AISourceRef, AITaskBreakdownResponse, TaskPriority } from '../types'

type AssistantMode = 'weekly_report' | 'risk_summary' | 'task_breakdown'
type DraftView = 'preview' | 'edit'

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

const modeMeta: Record<AssistantMode, { label: string; hint: string }> = {
  weekly_report: { label: '周报草稿', hint: '汇总本周期进展、动态与风险，生成可编辑周报' },
  risk_summary: { label: '风险摘要', hint: '基于健康度与登记项，梳理风险与建议动作' },
  task_breakdown: { label: '任务拆解', hint: '把目标或描述拆成可执行的任务草稿' }
}

const modeOrder: AssistantMode[] = ['weekly_report', 'risk_summary', 'task_breakdown']

const priorityMeta: Record<TaskPriority, { label: string; className: string }> = {
  high: { label: '高', className: 'priority-high' },
  medium: { label: '中', className: 'priority-medium' },
  low: { label: '低', className: 'priority-low' }
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

const formatGeneratedAt = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString('zh-CN', { hour12: false })
}

function SourceList({ sources }: { sources: AISourceRef[] }) {
  if (sources.length === 0) return <p className="inline-tip">生成后将在此列出引用的项目、任务与登记项</p>
  return (
    <div className="assistant-source-list">
      {sources.map((source, index) => {
        const inner = (
          <>
            <span className="assistant-source-type">{sourceTypeLabel(source.type)}</span>
            <strong>{source.label || '-'}</strong>
          </>
        )
        return source.path ? (
          <Link key={`${source.type}-${source.id || index}`} to={source.path} className="assistant-source-item is-link">
            {inner}
          </Link>
        ) : (
          <div key={`${source.type}-${source.id || index}`} className="assistant-source-item">
            {inner}
          </div>
        )
      })}
    </div>
  )
}

export function AssistantPage() {
  const [form, setForm] = useState<AssistantForm>(initialForm)
  const [draftResult, setDraftResult] = useState<AIDraftResponse | null>(null)
  const [taskResult, setTaskResult] = useState<AITaskBreakdownResponse | null>(null)
  const [editableDraft, setEditableDraft] = useState('')
  const [draftView, setDraftView] = useState<DraftView>('preview')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [copySuccess, setCopySuccess] = useState('')

  const currentSources = useMemo(
    () => draftResult?.sourceRefs || taskResult?.sourceRefs || [],
    [draftResult, taskResult]
  )
  const highlights = draftResult?.highlights?.filter((item) => item.trim()) || []
  const recommendations = draftResult?.recommendations?.filter((item) => item.trim()) || []
  const hasResult = Boolean(draftResult || taskResult)
  const resultTitle = draftResult?.title || taskResult?.title || modeMeta[form.mode].label
  const generatedAt = formatGeneratedAt(draftResult?.generatedAt || taskResult?.generatedAt)

  const resetOutput = () => {
    setDraftResult(null)
    setTaskResult(null)
    setEditableDraft('')
    setError('')
  }

  const selectMode = (mode: AssistantMode) => {
    if (mode === form.mode) return
    setForm((prev) => ({ ...prev, mode }))
    setDraftView('preview')
    resetOutput()
    setCopySuccess('')
  }

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
      setDraftView('preview')
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
      setCopySuccess('已复制到剪贴板')
    } catch {
      setCopySuccess('复制失败')
    }
    window.setTimeout(() => setCopySuccess(''), 2400)
  }

  return (
    <section className="page-section assistant-page">
      <header className="assistant-hero card">
        <div className="assistant-hero-icon"><Sparkles size={22} /></div>
        <div className="assistant-hero-text">
          <h2>AI 助理</h2>
          <p>基于项目只读上下文生成周报、风险摘要与任务拆解草稿，结果需人工确认后使用。</p>
        </div>
      </header>

      <div className="assistant-grid">
        <form className="card assistant-control-panel" onSubmit={submit}>
          <div className="assistant-mode-tabs" role="tablist" aria-label="AI 助理类型">
            {modeOrder.map((mode) => (
              <button
                key={mode}
                type="button"
                role="tab"
                aria-selected={form.mode === mode}
                className={`assistant-mode-tab${form.mode === mode ? ' is-active' : ''}`}
                onClick={() => selectMode(mode)}
              >
                {modeMeta[mode].label}
              </button>
            ))}
          </div>
          <p className="assistant-mode-hint">{modeMeta[form.mode].hint}</p>

          <div className="assistant-field">
            <label className={form.mode === 'task_breakdown' ? '' : 'required-label'} htmlFor="assistant-project">
              项目
            </label>
            <RemoteProjectSelect
              value={form.projectId}
              onChange={(value) => setForm((prev) => ({ ...prev, projectId: value }))}
              ariaLabel="选择 AI 助理项目"
              defaultOptionLabel={form.mode === 'task_breakdown' ? '不指定项目' : '请选择项目'}
            />
          </div>

          {form.mode === 'weekly_report' && (
            <div className="assistant-field-row">
              <div className="assistant-field">
                <label htmlFor="assistant-week-start">开始时间</label>
                <DateTimeQuickField
                  inputId="assistant-week-start"
                  value={form.weekStart}
                  onChange={(value) => setForm((prev) => ({ ...prev, weekStart: value }))}
                />
              </div>
              <div className="assistant-field">
                <label htmlFor="assistant-week-end">结束时间</label>
                <DateTimeQuickField
                  inputId="assistant-week-end"
                  value={form.weekEnd}
                  onChange={(value) => setForm((prev) => ({ ...prev, weekEnd: value }))}
                />
              </div>
            </div>
          )}

          {form.mode === 'task_breakdown' && (
            <>
              <div className="assistant-field">
                <label htmlFor="assistant-title">标题</label>
                <input
                  id="assistant-title"
                  placeholder="例如：客户门户上线"
                  value={form.title}
                  onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))}
                />
              </div>
              <div className="assistant-field">
                <label htmlFor="assistant-description">描述</label>
                <textarea
                  id="assistant-description"
                  rows={5}
                  placeholder="补充目标、范围或关键约束，帮助 AI 拆解更贴合实际"
                  value={form.description}
                  onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))}
                />
              </div>
            </>
          )}

          <div className="assistant-actions">
            <button className="btn" type="submit" disabled={submitting}>
              <Sparkles size={16} />
              {submitting ? '生成中...' : '生成草稿'}
            </button>
            <button
              className="btn secondary"
              type="button"
              onClick={() => {
                setForm(initialForm)
                setDraftView('preview')
                resetOutput()
                setCopySuccess('')
              }}
            >
              <RotateCcw size={15} />
              重置
            </button>
          </div>
          {error && <p className="error assistant-error">{error}</p>}
        </form>

        <div className="assistant-output-panel">
          <article className="card assistant-output-card">
            <div className="assistant-output-header">
              <div className="assistant-output-title">
                <h3>{resultTitle}</h3>
                {generatedAt && <span className="assistant-generated-at">生成于 {generatedAt}</span>}
              </div>
              {hasResult && (
                <div className="assistant-output-tools">
                  {draftResult && (
                    <div className="assistant-view-toggle" role="tablist" aria-label="草稿视图">
                      <button
                        type="button"
                        role="tab"
                        aria-selected={draftView === 'preview'}
                        className={draftView === 'preview' ? 'is-active' : ''}
                        onClick={() => setDraftView('preview')}
                      >
                        <Eye size={14} /> 预览
                      </button>
                      <button
                        type="button"
                        role="tab"
                        aria-selected={draftView === 'edit'}
                        className={draftView === 'edit' ? 'is-active' : ''}
                        onClick={() => setDraftView('edit')}
                      >
                        <Pencil size={14} /> 编辑
                      </button>
                    </div>
                  )}
                  <button className="btn secondary assistant-copy-btn" type="button" onClick={() => { void copyDraft() }}>
                    {copySuccess === '已复制到剪贴板' ? <Check size={15} /> : <Copy size={15} />}
                    复制
                  </button>
                </div>
              )}
            </div>
            {copySuccess && <p className={copySuccess === '复制失败' ? 'error' : 'success'}>{copySuccess}</p>}

            {draftResult ? (
              <div className="assistant-draft">
                {highlights.length > 0 && (
                  <div className="assistant-highlight-grid">
                    {highlights.map((item) => (
                      <span key={item} className="assistant-highlight">{item}</span>
                    ))}
                  </div>
                )}
                {draftView === 'preview' ? (
                  <MarkdownView content={editableDraft} className="assistant-draft-preview" />
                ) : (
                  <textarea
                    className="assistant-draft-editor"
                    value={editableDraft}
                    onChange={(event) => setEditableDraft(event.target.value)}
                  />
                )}
                {recommendations.length > 0 && (
                  <div className="assistant-reco">
                    <div className="assistant-section-label"><Lightbulb size={15} /> 建议动作</div>
                    <ul>
                      {recommendations.map((item) => (
                        <li key={item}>{item}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            ) : taskResult ? (
              <div className="assistant-tasks">
                <p className="assistant-task-summary">{taskResult.summary}</p>
                <div className="assistant-task-list">
                  {taskResult.tasks.map((task, index) => (
                    <article key={`${task.relativeStartDay}-${task.title}-${index}`} className="assistant-task-item">
                      <div className="assistant-task-head">
                        <span className={`assistant-priority-dot ${priorityMeta[task.priority].className}`} />
                        <strong>{task.title}</strong>
                        {task.isMilestone && (
                          <span className="assistant-milestone"><Flag size={12} /> 里程碑</span>
                        )}
                      </div>
                      {task.description && <p className="assistant-task-desc">{task.description}</p>}
                      <div className="assistant-task-meta">
                        <span className={`assistant-task-priority ${priorityMeta[task.priority].className}`}>
                          {priorityMeta[task.priority].label}优先级
                        </span>
                        <span><CalendarRange size={12} /> 第 {task.relativeStartDay} 天开始</span>
                        <span>持续 {task.durationDays} 天</span>
                      </div>
                    </article>
                  ))}
                </div>
              </div>
            ) : (
              <div className="assistant-empty">
                <FileText size={30} />
                <p>填写左侧参数并点击「生成草稿」，结果将在这里展示。</p>
                <span>{modeMeta[form.mode].hint}</span>
              </div>
            )}
          </article>

          <article className="card assistant-output-card assistant-source-card">
            <div className="assistant-section-label"><ListChecks size={15} /> 引用来源</div>
            <SourceList sources={currentSources} />
            {hasResult && (
              <p className="assistant-confirm-note">
                <AlertTriangle size={13} /> AI 生成内容仅供参考，请确认后再对外使用或创建任务。
              </p>
            )}
          </article>
        </div>
      </div>
    </section>
  )
}
