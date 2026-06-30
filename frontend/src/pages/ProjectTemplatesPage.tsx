import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { Project, ProjectTemplate, TemplateTask } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface TemplateForm {
  id?: number
  name: string
  description: string
  taskTreeJson: string
}

interface GenerateProjectForm {
  code: string
  name: string
  description: string
  startAt: string
  endAt: string
  userIds: number[]
  departmentIds: number[]
}

interface ProjectEditorOptionsResponse {
  users?: Array<{ id: number; name: string; username: string; email: string }>
  departments?: Array<{ id: number; name: string }>
}

interface CreateProjectFromTemplateResponse {
  project: Project
}

const sampleTaskTree: TemplateTask[] = [
  {
    key: 'plan',
    title: '制定项目计划',
    description: '确认范围、目标和里程碑',
    priority: 'high',
    isMilestone: true,
    relativeStartDay: 0,
    durationDays: 1,
    children: [
      {
        key: 'design',
        title: '设计实施方案',
        description: '拆解方案、风险和回滚策略',
        priority: 'medium',
        relativeStartDay: 1,
        durationDays: 3,
        dependencies: [{ dependsOnKey: 'plan', lagDays: 0, type: 'FS' }]
      }
    ]
  },
  {
    key: 'release',
    title: '上线验收',
    priority: 'high',
    isMilestone: true,
    relativeStartDay: 5,
    durationDays: 1,
    dependencies: [{ dependsOnKey: 'design', lagDays: 0, type: 'FS' }]
  }
]

const formatTaskTreeJson = (taskTree: TemplateTask[]) => JSON.stringify(taskTree, null, 2)

const initialTemplateForm: TemplateForm = {
  name: '',
  description: '',
  taskTreeJson: formatTaskTreeJson(sampleTaskTree)
}

const initialGenerateForm: GenerateProjectForm = {
  code: '',
  name: '',
  description: '',
  startAt: '',
  endAt: '',
  userIds: [],
  departmentIds: []
}

const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]

const countTemplateTasks = (tasks: TemplateTask[]) => {
  let total = 0
  const walk = (items: TemplateTask[]) => {
    items.forEach((item) => {
      total += 1
      walk(item.children || [])
    })
  }
  walk(tasks)
  return total
}

const countTemplateDependencies = (tasks: TemplateTask[]) => {
  let total = 0
  const walk = (items: TemplateTask[]) => {
    items.forEach((item) => {
      total += (item.dependencies || []).length
      walk(item.children || [])
    })
  }
  walk(tasks)
  return total
}

export function ProjectTemplatesPage() {
  const permissions = usePermissions()
  const navigate = useNavigate()
  const canCreateTemplate = hasPermission('templates.create', permissions)
  const canUpdateTemplate = hasPermission('templates.update', permissions)
  const canDeleteTemplate = hasPermission('templates.delete', permissions)
  const canCreateProject = hasPermission('projects.create', permissions)
  const canReadProjects = hasPermission('projects.read', permissions)
  const [items, setItems] = useState<ProjectTemplate[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [form, setForm] = useState<TemplateForm>(initialTemplateForm)
  const [generateForm, setGenerateForm] = useState<GenerateProjectForm>(initialGenerateForm)
  const [activeTemplate, setActiveTemplate] = useState<ProjectTemplate | null>(null)
  const [users, setUsers] = useState<ProjectEditorOptionsResponse['users']>([])
  const [departments, setDepartments] = useState<ProjectEditorOptionsResponse['departments']>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [generateOpen, setGenerateOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim()))

  const templateStats = useMemo(() => new Map(items.map((item) => [
    item.id,
    {
      tasks: countTemplateTasks(item.taskTree || []),
      dependencies: countTemplateDependencies(item.taskTree || [])
    }
  ])), [items])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const templatePage = await fetchPage<ProjectTemplate>('/project-templates', { page, pageSize, keyword }, { page, pageSize })
      setItems(templatePage.list)
      setTotal(templatePage.total)
    } catch (loadError) {
      setError(readApiError(loadError, '模板列表加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword])

  useEffect(() => {
    if (!canReadProjects) {
      setUsers([])
      setDepartments([])
      return
    }
    void fetchData<ProjectEditorOptionsResponse>('/projects/editor-options', { pageSize: 100 }, { silent: true })
      .then((data) => {
        setUsers(data.users || [])
        setDepartments(data.departments || [])
      })
      .catch(() => {
        setUsers([])
        setDepartments([])
      })
  }, [canReadProjects])

  const openCreateModal = () => {
    if (!canCreateTemplate) return
    setForm(initialTemplateForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openEditModal = (item: ProjectTemplate) => {
    if (!canUpdateTemplate) return
    setForm({
      id: item.id,
      name: item.name,
      description: item.description || '',
      taskTreeJson: formatTaskTreeJson(item.taskTree || [])
    })
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openGenerateModal = (item: ProjectTemplate) => {
    if (!canCreateProject) return
    setActiveTemplate(item)
    setGenerateForm({
      ...initialGenerateForm,
      name: `${item.name}项目`,
      description: item.description || ''
    })
    setFormError('')
    setFormSuccess('')
    setGenerateOpen(true)
  }

  const parseTaskTree = () => {
    let taskTree: unknown
    try {
      taskTree = JSON.parse(form.taskTreeJson)
    } catch {
      throw new Error('任务树 JSON 格式不正确')
    }
    if (!Array.isArray(taskTree) || taskTree.length === 0) {
      throw new Error('任务树至少需要一个任务')
    }
    return taskTree
  }

  const submitTemplate = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateTemplate) return
    if (!form.id && !canCreateTemplate) return
    try {
      setSubmitting(true)
      setFormError('')
      const taskTree = parseTaskTree()
      const payload = {
        name: form.name,
        description: form.description,
        taskTree
      }
      if (form.id) await api.put(`/project-templates/${form.id}`, payload)
      else await api.post('/project-templates', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialTemplateForm)
      await load()
    } catch (submitError) {
      setFormError(submitError instanceof Error ? submitError.message : readApiError(submitError, '保存模板失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const submitGenerate = async (event: FormEvent) => {
    event.preventDefault()
    if (!activeTemplate || !canCreateProject) return
    try {
      setSubmitting(true)
      setFormError('')
      const res = await api.post<CreateProjectFromTemplateResponse>(`/project-templates/${activeTemplate.id}/create-project`, {
        ...generateForm,
        startAt: generateForm.startAt ? new Date(generateForm.startAt).toISOString() : '',
        endAt: generateForm.endAt ? new Date(generateForm.endAt).toISOString() : ''
      })
      setFormSuccess('项目已生成')
      setGenerateOpen(false)
      setActiveTemplate(null)
      if (res.data.project?.id) navigate(`/projects?projectId=${res.data.project.id}`)
    } catch (submitError) {
      setFormError(readApiError(submitError, '生成项目失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    if (!canDeleteTemplate) return
    if (!confirm('确认删除该模板？')) return
    try {
      await api.delete(`/project-templates/${id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除模板失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="模板筛选"
        activeCount={activeFilterCount}
        actions={canCreateTemplate ? <button className="btn" onClick={openCreateModal}>新增模板</button> : undefined}
        bodyClassName="form-grid"
      >
        <SearchField
          aria-label="模板关键词搜索"
          value={keywordInput}
          placeholder="搜索模板名称/描述"
          onChange={setKeywordInput}
          onClear={() => {
            setPage(1)
            setKeyword('')
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              setPage(1)
              setKeyword(keywordInput.trim())
            }
          }}
        />
        <div className="row-actions">
          <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
        </div>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无项目模板" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>ID</th><th>模板名称</th><th>描述</th><th>任务数</th><th>依赖数</th><th>更新时间</th><th>操作</th>
          </tr></thead><tbody>
            {items.map((item) => {
              const stats = templateStats.get(item.id) || { tasks: 0, dependencies: 0 }
              return (
                <tr key={item.id}>
                  <td data-label="ID">{item.id}</td>
                  <td data-label="模板名称">{item.name}</td>
                  <td data-label="描述">{item.description || '-'}</td>
                  <td data-label="任务数">{stats.tasks}</td>
                  <td data-label="依赖数">{stats.dependencies}</td>
                  <td data-label="更新时间">{formatDateTime(item.updatedAt)}</td>
                  <td data-label="操作">
                    <div className="table-actions">
                      {canCreateProject && <button className="btn secondary" onClick={() => openGenerateModal(item)}>生成项目</button>}
                      {canUpdateTemplate && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                      {canDeleteTemplate && <button className="btn danger" onClick={() => { void onDelete(item.id) }}>删除</button>}
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑模板' : '新增模板'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submitTemplate}>
          <label className="required-label" htmlFor="template-name">模板名称</label>
          <input id="template-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="template-description">描述</label>
          <textarea id="template-description" rows={3} value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label className="required-label" htmlFor="template-task-tree">任务树 JSON</label>
          <textarea
            id="template-task-tree"
            rows={14}
            value={form.taskTreeJson}
            spellCheck={false}
            onChange={(event) => setForm((prev) => ({ ...prev, taskTreeJson: event.target.value }))}
            required
          />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateTemplate : !canCreateTemplate)}>{submitting ? '保存中...' : '保存模板'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialTemplateForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>

      <Modal open={generateOpen} title="从模板生成项目" onClose={() => setGenerateOpen(false)}>
        <form className="form-grid" onSubmit={submitGenerate}>
          <label htmlFor="template-project-code">项目编码（可空自动生成）</label>
          <input id="template-project-code" value={generateForm.code} onChange={(event) => setGenerateForm((prev) => ({ ...prev, code: event.target.value }))} />
          <label className="required-label" htmlFor="template-project-name">项目名称</label>
          <input id="template-project-name" value={generateForm.name} onChange={(event) => setGenerateForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="template-project-description">项目描述</label>
          <textarea id="template-project-description" rows={3} value={generateForm.description} onChange={(event) => setGenerateForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="template-project-start">开始时间</label>
          <DateTimeQuickField inputId="template-project-start" value={generateForm.startAt} onChange={(value) => setGenerateForm((prev) => ({ ...prev, startAt: value }))} />
          <label htmlFor="template-project-end">结束时间</label>
          <DateTimeQuickField inputId="template-project-end" value={generateForm.endAt} onChange={(value) => setGenerateForm((prev) => ({ ...prev, endAt: value }))} />
          {canReadProjects && (
            <>
              <label htmlFor="template-project-users">项目负责人</label>
              <div id="template-project-users" className="multi-checklist">
                {(users || []).map((user) => (
                  <label key={user.id} className="multi-check-item">
                    <input type="checkbox" checked={generateForm.userIds.includes(user.id)} onChange={() => setGenerateForm((prev) => ({ ...prev, userIds: toggleNumber(prev.userIds, user.id) }))} />
                    <span>{user.name} ({user.username})</span>
                  </label>
                ))}
              </div>
              <label htmlFor="template-project-departments">参与部门</label>
              <div id="template-project-departments" className="multi-checklist">
                {(departments || []).map((department) => (
                  <label key={department.id} className="multi-check-item">
                    <input type="checkbox" checked={generateForm.departmentIds.includes(department.id)} onChange={() => setGenerateForm((prev) => ({ ...prev, departmentIds: toggleNumber(prev.departmentIds, department.id) }))} />
                    <span>{department.name}</span>
                  </label>
                ))}
              </div>
            </>
          )}
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || !canCreateProject}>{submitting ? '生成中...' : '生成项目'}</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
