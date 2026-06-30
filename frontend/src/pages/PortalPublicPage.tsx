import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { DataState } from '../components/DataState'
import { fetchPublicData, publicApi, readApiError, uploadPublicAttachments } from '../services/api'
import type { UploadSourceFile } from '../services/api'
import { PortalCommentView, PortalStatusResponse, PortalTaskView, TaskPriority, UploadAttachment, WorkRequestType } from '../types'
import { formatDateTime } from '../utils/datetime'

const statusLabel: Record<string, string> = {
  pending: '待处理',
  queued: '排队中',
  processing: '处理中',
  reviewing: '待审核',
  completed: '已完成'
}

const priorityLabel: Record<TaskPriority, string> = {
  high: '高',
  medium: '中',
  low: '低'
}

interface PortalRequestForm {
  type: WorkRequestType
  title: string
  description: string
  priority: TaskPriority
  targetTaskId: string
  externalName: string
  externalEmail: string
  attachments: UploadAttachment[]
}

interface PortalCommentForm {
  taskId: string
  content: string
  externalName: string
  externalEmail: string
  attachments: UploadAttachment[]
}

const initialRequestForm: PortalRequestForm = {
  type: 'task',
  title: '',
  description: '',
  priority: 'medium',
  targetTaskId: '',
  externalName: '',
  externalEmail: '',
  attachments: []
}

const initialCommentForm: PortalCommentForm = {
  taskId: '',
  content: '',
  externalName: '',
  externalEmail: '',
  attachments: []
}

const toSourceFiles = (files: FileList): UploadSourceFile[] => Array.from(files).map((file) => ({ file, relativePath: file.name }))

const formatFileSize = (size: number) => {
  if (!size) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`
  return `${(size / (1024 * 1024)).toFixed(2)} MB`
}

const mergeAttachments = (origin: UploadAttachment[], incoming: UploadAttachment[]) => {
  const map = new Map<string, UploadAttachment>()
  origin.forEach((item) => item.filePath && map.set(item.filePath, item))
  incoming.forEach((item) => item.filePath && map.set(item.filePath, item))
  return Array.from(map.values())
}

export function PortalPublicPage() {
  const { token = '' } = useParams()
  const [data, setData] = useState<PortalStatusResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [requestForm, setRequestForm] = useState<PortalRequestForm>(initialRequestForm)
  const [commentForm, setCommentForm] = useState<PortalCommentForm>(initialCommentForm)
  const [requestSubmitting, setRequestSubmitting] = useState(false)
  const [commentSubmitting, setCommentSubmitting] = useState(false)
  const [requestError, setRequestError] = useState('')
  const [commentError, setCommentError] = useState('')
  const [requestSuccess, setRequestSuccess] = useState('')
  const [commentSuccess, setCommentSuccess] = useState('')
  const [requestUploading, setRequestUploading] = useState(false)
  const [commentUploading, setCommentUploading] = useState(false)
  const requestFileRef = useRef<HTMLInputElement | null>(null)
  const commentFileRef = useRef<HTMLInputElement | null>(null)

  const load = async () => {
    if (!token) return
    try {
      setLoading(true)
      setError('')
      const portalData = await fetchPublicData<PortalStatusResponse>(`/portal/${encodeURIComponent(token)}`)
      setData(portalData)
      setCommentForm((prev) => ({ ...prev, taskId: prev.taskId || String(portalData.tasks[0]?.id || '') }))
      setRequestForm((prev) => ({ ...prev, targetTaskId: prev.targetTaskId || String(portalData.tasks[0]?.id || '') }))
    } catch (loadError) {
      setError(readApiError(loadError, '外部门户加载失败'))
      setData(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [token])

  const commentsByTask = useMemo(() => {
    const map = new Map<number, PortalCommentView[]>()
    ;(data?.comments || []).forEach((comment) => {
      const list = map.get(comment.taskId) || []
      list.push(comment)
      map.set(comment.taskId, list)
    })
    return map
  }, [data?.comments])

  const uploadForRequest = async (files: FileList | null) => {
    if (!files || files.length === 0 || !token) return
    try {
      setRequestUploading(true)
      setRequestError('')
      const attachments = await uploadPublicAttachments(`/portal/${encodeURIComponent(token)}/uploads`, toSourceFiles(files))
      setRequestForm((prev) => ({ ...prev, attachments: mergeAttachments(prev.attachments, attachments) }))
    } catch (uploadError) {
      setRequestError(readApiError(uploadError, '上传请求附件失败'))
    } finally {
      setRequestUploading(false)
    }
  }

  const uploadForComment = async (files: FileList | null) => {
    if (!files || files.length === 0 || !token) return
    try {
      setCommentUploading(true)
      setCommentError('')
      const attachments = await uploadPublicAttachments(`/portal/${encodeURIComponent(token)}/uploads`, toSourceFiles(files))
      setCommentForm((prev) => ({ ...prev, attachments: mergeAttachments(prev.attachments, attachments) }))
    } catch (uploadError) {
      setCommentError(readApiError(uploadError, '上传评论附件失败'))
    } finally {
      setCommentUploading(false)
    }
  }

  const submitRequest = async (event: FormEvent) => {
    event.preventDefault()
    if (!token) return
    if (requestForm.type === 'change' && !requestForm.targetTaskId) {
      setRequestError('请选择变更关联任务')
      return
    }
    try {
      setRequestSubmitting(true)
      setRequestError('')
      setRequestSuccess('')
      await publicApi.post(`/portal/${encodeURIComponent(token)}/requests`, {
        type: requestForm.type,
        title: requestForm.title.trim(),
        description: requestForm.description.trim(),
        priority: requestForm.priority,
        targetTaskId: requestForm.type === 'change' ? Number(requestForm.targetTaskId) : undefined,
        changePayload: requestForm.type === 'change' ? { scopeDescription: requestForm.description.trim() } : undefined,
        externalName: requestForm.externalName.trim(),
        externalEmail: requestForm.externalEmail.trim(),
        attachments: requestForm.attachments
      })
      setRequestSuccess('请求已提交')
      setRequestForm((prev) => ({ ...initialRequestForm, externalName: prev.externalName, externalEmail: prev.externalEmail, targetTaskId: String(data?.tasks[0]?.id || '') }))
      await load()
    } catch (submitError) {
      setRequestError(readApiError(submitError, '提交请求失败'))
    } finally {
      setRequestSubmitting(false)
    }
  }

  const submitComment = async (event: FormEvent) => {
    event.preventDefault()
    if (!token || !commentForm.taskId) return
    try {
      setCommentSubmitting(true)
      setCommentError('')
      setCommentSuccess('')
      await publicApi.post(`/portal/${encodeURIComponent(token)}/tasks/${commentForm.taskId}/comments`, {
        content: commentForm.content.trim(),
        externalName: commentForm.externalName.trim(),
        externalEmail: commentForm.externalEmail.trim(),
        attachments: commentForm.attachments
      })
      setCommentSuccess('评论已提交')
      setCommentForm((prev) => ({ ...initialCommentForm, externalName: prev.externalName, externalEmail: prev.externalEmail, taskId: prev.taskId }))
      await load()
    } catch (submitError) {
      setCommentError(readApiError(submitError, '提交评论失败'))
    } finally {
      setCommentSubmitting(false)
    }
  }

  const renderAttachmentList = (items: UploadAttachment[], onRemove?: (target: UploadAttachment) => void) => {
    if (items.length === 0) return null
    return (
      <div className="attachment-list portal-attachment-list">
        {items.map((item) => (
          <div key={item.filePath} className="attachment-item">
            <a href={item.filePath} target="_blank" rel="noreferrer">{item.relativePath || item.fileName || '附件'}</a>
            <span>{formatFileSize(item.fileSize)}</span>
            {onRemove && <button type="button" className="btn secondary" onClick={() => onRemove(item)}>移除</button>}
          </div>
        ))}
      </div>
    )
  }

  const removeRequestAttachment = (target: UploadAttachment) => {
    setRequestForm((prev) => ({ ...prev, attachments: prev.attachments.filter((item) => item.filePath !== target.filePath) }))
  }

  const removeCommentAttachment = (target: UploadAttachment) => {
    setCommentForm((prev) => ({ ...prev, attachments: prev.attachments.filter((item) => item.filePath !== target.filePath) }))
  }

  const renderTask = (task: PortalTaskView) => {
    const comments = commentsByTask.get(task.id) || []
    return (
      <article key={task.id} className="portal-task-card">
        <div className="portal-task-head">
          <span className="kanban-task-no">{task.taskNo || `#${task.id}`}</span>
          <span className="task-visibility-badge visible">对外可见</span>
        </div>
        <h3>{task.title}</h3>
        <p>{task.description || '-'}</p>
        <div className="portal-task-meta">
          <span>{statusLabel[task.status] || task.status}</span>
          <span>{priorityLabel[(task.priority || 'medium') as TaskPriority]}</span>
          <span>{task.progress}%</span>
          <span>{formatDateTime(task.startAt)} - {formatDateTime(task.endAt)}</span>
        </div>
        <div className="kanban-progress" aria-label={`任务进度 ${task.progress}%`}>
          <span style={{ width: `${Math.max(0, Math.min(100, Number(task.progress || 0)))}%` }} />
        </div>
        {(task.tags || []).length > 0 && (
          <div className="task-tag-stack">
            {(task.tags || []).map((tag) => <span key={tag.id} className="task-tag-badge">{tag.name}</span>)}
          </div>
        )}
        {comments.length > 0 && (
          <div className="portal-comment-list">
            {comments.map((comment) => (
              <div key={comment.id} className="task-comment-item">
                <div className="task-comment-meta">
                  <strong>{comment.externalName || '外部联系人'}</strong>
                  <span>{formatDateTime(comment.createdAt)}</span>
                </div>
                <p>{comment.content}</p>
                {renderAttachmentList(comment.attachments || [])}
              </div>
            ))}
          </div>
        )}
      </article>
    )
  }

  return (
    <main className="portal-public-shell">
      <DataState loading={loading} error={error} empty={!loading && !error && !data} emptyText="暂无门户数据" onRetry={() => { void load() }} />
      {!loading && !error && data && (
        <>
          <header className="portal-public-header card">
            <div>
              <p className="table-subtext">{data.project.code}</p>
              <h1>{data.project.name}</h1>
              <p>{data.project.description || '-'}</p>
            </div>
            <div className={`portal-health-badge ${data.statusReport.health}`}>
              <span>{data.statusReport.health === 'red' ? '需关注' : data.statusReport.health === 'yellow' ? '推进中' : '正常'}</span>
              <strong>{Math.round(data.statusReport.averageProgress)}%</strong>
            </div>
          </header>

          <section className="cards portal-metric-grid">
            <article className="card metric-card"><p>对外任务</p><strong>{data.statusReport.taskCount}</strong></article>
            <article className="card metric-card"><p>已完成</p><strong>{data.statusReport.completedTaskCount}</strong></article>
            <article className="card metric-card"><p>逾期任务</p><strong>{data.statusReport.overdueTaskCount}</strong></article>
            <article className="card metric-card"><p>完成率</p><strong>{(data.statusReport.completionRate * 100).toFixed(0)}%</strong></article>
          </section>

          <section className="card portal-status-card">
            <h2>状态报告</h2>
            <p>{data.statusReport.summary}</p>
            <div className="detail-columns">
              <div><strong>项目周期：</strong>{formatDateTime(data.project.startAt)} - {formatDateTime(data.project.endAt)}</div>
              <div><strong>更新时间：</strong>{formatDateTime(data.statusReport.generatedAt)}</div>
              <div><strong>联系人：</strong>{data.contactName || data.company || '-'}</div>
            </div>
            {renderAttachmentList(data.allowedAttachments || [])}
          </section>

          <section className="portal-main-grid">
            <div className="portal-task-list">
              {data.tasks.length > 0 ? data.tasks.map((task) => renderTask(task)) : <div className="card">暂无对外可见任务</div>}
            </div>

            <aside className="portal-side-panel">
              <form className="card form-grid" onSubmit={submitRequest}>
                <h2>提交请求</h2>
                <label className="required-label" htmlFor="portal-request-title">标题</label>
                <input id="portal-request-title" value={requestForm.title} onChange={(event) => setRequestForm((prev) => ({ ...prev, title: event.target.value }))} required />
                <label htmlFor="portal-request-type">类型</label>
                <select id="portal-request-type" value={requestForm.type} onChange={(event) => setRequestForm((prev) => ({ ...prev, type: event.target.value as WorkRequestType }))}>
                  <option value="task">任务</option>
                  <option value="bug">缺陷</option>
                  <option value="project">项目</option>
                  <option value="change">变更</option>
                </select>
                {requestForm.type === 'change' && (
                  <>
                    <label className="required-label" htmlFor="portal-request-task">关联任务</label>
                    <select id="portal-request-task" value={requestForm.targetTaskId} onChange={(event) => setRequestForm((prev) => ({ ...prev, targetTaskId: event.target.value }))} required>
                      {data.tasks.map((task) => <option key={task.id} value={task.id}>{task.taskNo} - {task.title}</option>)}
                    </select>
                  </>
                )}
                <label htmlFor="portal-request-priority">优先级</label>
                <select id="portal-request-priority" value={requestForm.priority} onChange={(event) => setRequestForm((prev) => ({ ...prev, priority: event.target.value as TaskPriority }))}>
                  <option value="high">高</option>
                  <option value="medium">中</option>
                  <option value="low">低</option>
                </select>
                <label htmlFor="portal-request-description">描述</label>
                <textarea id="portal-request-description" rows={4} value={requestForm.description} onChange={(event) => setRequestForm((prev) => ({ ...prev, description: event.target.value }))} />
                <label htmlFor="portal-request-name">姓名</label>
                <input id="portal-request-name" value={requestForm.externalName} onChange={(event) => setRequestForm((prev) => ({ ...prev, externalName: event.target.value }))} />
                <label htmlFor="portal-request-email">邮箱</label>
                <input id="portal-request-email" type="email" value={requestForm.externalEmail} onChange={(event) => setRequestForm((prev) => ({ ...prev, externalEmail: event.target.value }))} />
                <input ref={requestFileRef} className="sr-only" type="file" multiple onChange={(event: ChangeEvent<HTMLInputElement>) => { void uploadForRequest(event.target.files); event.target.value = '' }} />
                <div className="row-actions">
                  <button type="button" className="btn secondary" disabled={requestUploading} onClick={() => requestFileRef.current?.click()}>{requestUploading ? '上传中...' : '添加附件'}</button>
                </div>
                {renderAttachmentList(requestForm.attachments, removeRequestAttachment)}
                <div className="row-actions">
                  <button type="submit" className="btn" disabled={requestSubmitting || !requestForm.title.trim()}>{requestSubmitting ? '提交中...' : '提交请求'}</button>
                </div>
                {requestSuccess && <p className="success">{requestSuccess}</p>}
                {requestError && <p className="error">{requestError}</p>}
              </form>

              <form className="card form-grid" onSubmit={submitComment}>
                <h2>任务评论</h2>
                <label className="required-label" htmlFor="portal-comment-task">任务</label>
                <select id="portal-comment-task" value={commentForm.taskId} onChange={(event) => setCommentForm((prev) => ({ ...prev, taskId: event.target.value }))} required>
                  {data.tasks.map((task) => <option key={task.id} value={task.id}>{task.taskNo} - {task.title}</option>)}
                </select>
                <label className="required-label" htmlFor="portal-comment-content">内容</label>
                <textarea id="portal-comment-content" rows={4} value={commentForm.content} onChange={(event) => setCommentForm((prev) => ({ ...prev, content: event.target.value }))} required />
                <label htmlFor="portal-comment-name">姓名</label>
                <input id="portal-comment-name" value={commentForm.externalName} onChange={(event) => setCommentForm((prev) => ({ ...prev, externalName: event.target.value }))} />
                <label htmlFor="portal-comment-email">邮箱</label>
                <input id="portal-comment-email" type="email" value={commentForm.externalEmail} onChange={(event) => setCommentForm((prev) => ({ ...prev, externalEmail: event.target.value }))} />
                <input ref={commentFileRef} className="sr-only" type="file" multiple onChange={(event: ChangeEvent<HTMLInputElement>) => { void uploadForComment(event.target.files); event.target.value = '' }} />
                <div className="row-actions">
                  <button type="button" className="btn secondary" disabled={commentUploading} onClick={() => commentFileRef.current?.click()}>{commentUploading ? '上传中...' : '添加附件'}</button>
                </div>
                {renderAttachmentList(commentForm.attachments, removeCommentAttachment)}
                <div className="row-actions">
                  <button type="submit" className="btn" disabled={commentSubmitting || !commentForm.taskId || !commentForm.content.trim()}>{commentSubmitting ? '提交中...' : '提交评论'}</button>
                </div>
                {commentSuccess && <p className="success">{commentSuccess}</p>}
                {commentError && <p className="error">{commentError}</p>}
              </form>
            </aside>
          </section>
        </>
      )}
    </main>
  )
}
