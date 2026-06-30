import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Settings2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api, fetchArray, fetchData, fetchPage, hasPermission, readApiError } from '../services/api'
import { TaskTree } from '../components/TaskTree'
import { Task, ContractAttachment, Department, Project, UploadAttachment, User, emptyUploadAttachments } from '../types'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { FieldSettingItem, FieldSettingsModal } from '../components/FieldSettingsModal'
import { SearchField } from '../components/SearchField'
import { formatDateTime } from '../utils/datetime'
import { AttachmentField } from '../components/AttachmentField'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { usePermissions } from '../hooks/usePermissions'

interface ProjectForm {
  id?: number
  code: string
  name: string
  description: string
  startAt: string
  endAt: string
  budgetAmount: string
  actualCostAmount: string
  expectedRevenueAmount: string
  contractNo: string
  contractAttachments: ContractAttachment[]
  attachments: UploadAttachment[]
  userIds: number[]
  departmentIds: number[]
}

const normalizeAttachments = (item: { attachments?: UploadAttachment[]; attachment?: UploadAttachment }) => {
  if (Array.isArray(item.attachments)) return item.attachments
  if (item.attachment?.filePath) return [item.attachment]
  return emptyUploadAttachments()
}

const emptyProjectForm = (): ProjectForm => ({
  code: '',
  name: '',
  description: '',
  startAt: '',
  endAt: '',
  budgetAmount: '',
  actualCostAmount: '',
  expectedRevenueAmount: '',
  contractNo: '',
  contractAttachments: [],
  attachments: emptyUploadAttachments(),
  userIds: [],
  departmentIds: []
})

const initialForm: ProjectForm = emptyProjectForm()

type SortKey = 'code' | 'name' | 'createdAt' | 'updatedAt' | 'startAt' | 'endAt'
type SortOrder = 'asc' | 'desc'
type ProjectColumnKey = 'code' | 'name' | 'description' | 'owners' | 'departments' | 'budgetAmount' | 'actualCostAmount' | 'contractNo' | 'budgetUsageRate' | 'startAt' | 'endAt' | 'createdAt' | 'updatedAt'
interface ProjectFieldSetting extends FieldSettingItem {
  key: ProjectColumnKey
}
const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]
const projectFieldSettingsStorageKey = 'projects_field_settings'
const projectDefaultFieldSettings: ProjectFieldSetting[] = [
  { key: 'code', label: '编码', visible: true, editable: true, sortable: true, searchable: true, filterable: false, custom: false },
  { key: 'name', label: '名称', visible: true, editable: true, sortable: true, searchable: true, filterable: false, custom: false },
  { key: 'description', label: '描述', visible: false, editable: true, sortable: false, searchable: true, filterable: false, custom: false },
  { key: 'owners', label: '负责人', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'departments', label: '部门', visible: true, editable: true, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'budgetAmount', label: '预算', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'actualCostAmount', label: '实际成本', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'contractNo', label: '合同编号', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'budgetUsageRate', label: '预算使用率', visible: false, editable: false, sortable: false, searchable: false, filterable: false, custom: false },
  { key: 'startAt', label: '开始时间', visible: false, editable: true, sortable: true, searchable: false, filterable: false, custom: false },
  { key: 'endAt', label: '结束时间', visible: false, editable: true, sortable: true, searchable: false, filterable: false, custom: false },
  { key: 'createdAt', label: '创建时间', visible: false, editable: false, sortable: true, searchable: false, filterable: false, custom: false },
  { key: 'updatedAt', label: '更新时间', visible: false, editable: false, sortable: true, searchable: false, filterable: false, custom: false }
]
const projectFinanceColumnKeys = new Set<ProjectColumnKey>(['budgetAmount', 'actualCostAmount', 'contractNo', 'budgetUsageRate'])
const isProjectFinanceColumn = (key: ProjectColumnKey) => projectFinanceColumnKeys.has(key)

const getProjectOwnerNames = (project: Project) => (project.users || []).map((user) => user.username || user.name).filter(Boolean)
const getProjectDepartmentNames = (project: Project) => (project.departments || []).map((department) => department.name).filter(Boolean)
const contractCategoryLabel: Record<NonNullable<ContractAttachment['category']>, string> = {
  contract: '合同',
  invoice: '发票',
  acceptance: '验收',
  change: '变更',
  other: '其他'
}
const contractAccessLevelLabel: Record<NonNullable<ContractAttachment['accessLevel']>, string> = {
  finance: '财务敏感',
  internal: '内部可见',
  external: '可对外'
}

const formatMoney = (value?: number) => typeof value === 'number' ? value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '-'
const formatRate = (value?: number) => typeof value === 'number' ? `${value.toFixed(1)}%` : '-'

const toLocalDateTimeInput = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const parseOptionalDateTime = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

const normalizeContractAttachments = (items?: ContractAttachment[]) => (items || []).map((item) => ({
  ...item,
  category: item.category || 'contract',
  accessLevel: item.accessLevel || 'finance',
  version: item.version || '',
  expiresAt: toLocalDateTimeInput(item.expiresAt)
}))

const mergeContractAttachmentMetadata = (origin: ContractAttachment[], incoming: UploadAttachment[]) => incoming.map((item) => {
  const current = origin.find((originItem) => originItem.filePath === item.filePath)
  return {
    ...item,
    category: current?.category || 'contract',
    version: current?.version || '',
    accessLevel: current?.accessLevel || 'finance',
    expiresAt: current?.expiresAt || ''
  } as ContractAttachment
})

const normalizeProjectFieldSettings = (raw: unknown): ProjectFieldSetting[] => {
  const fallbackMap = new Map(projectDefaultFieldSettings.map((field) => [field.key, field]))
  if (!Array.isArray(raw)) return projectDefaultFieldSettings

  const parsed = raw
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const key = String((item as { key?: string }).key || '') as ProjectColumnKey
      const base = fallbackMap.get(key)
      if (!base) return null
      return {
        ...base,
        ...item,
        key: base.key,
        label: base.label
      } as ProjectFieldSetting
    })
    .filter(Boolean) as ProjectFieldSetting[]

  const seen = new Set(parsed.map((item) => item.key))
  const missing = projectDefaultFieldSettings.filter((item) => !seen.has(item.key))
  return [...parsed, ...missing]
}

export function ProjectsPage() {
  const navigate = useNavigate()
  const permissions = usePermissions()
  const canCreateProject = hasPermission('projects.create', permissions)
  const canUpdateProject = hasPermission('projects.update', permissions)
  const canDeleteProject = hasPermission('projects.delete', permissions)
  const canUploadAttachment = hasPermission('uploads.create', permissions)
  const canReadFinance = hasPermission('finance.read', permissions) || hasPermission('finance.update', permissions)
  const canUpdateFinance = hasPermission('finance.update', permissions)
  const [projects, setProjects] = useState<Project[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [departments, setDepartments] = useState<Department[]>([])
  const [selected, setSelected] = useState<number>()
  const [keyword, setKeyword] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('updatedAt')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')
  const [fieldSettingsOpen, setFieldSettingsOpen] = useState(false)
  const [fieldSettings, setFieldSettings] = useState<ProjectFieldSetting[]>(() => {
    try {
      return normalizeProjectFieldSettings(JSON.parse(localStorage.getItem(projectFieldSettingsStorageKey) || '[]'))
    } catch {
      return projectDefaultFieldSettings
    }
  })
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [tree, setTree] = useState<Task[]>([])
  const [form, setForm] = useState<ProjectForm>(initialForm)
  const [modalOpen, setModalOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailProject, setDetailProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [ownerKeyword, setOwnerKeyword] = useState('')
  const [departmentKeyword, setDepartmentKeyword] = useState('')
  const availableFieldSettings = useMemo(
    () => canReadFinance ? fieldSettings : fieldSettings.filter((field) => !isProjectFinanceColumn(field.key)),
    [canReadFinance, fieldSettings]
  )
  const availableDefaultFieldSettings = useMemo(
    () => canReadFinance ? projectDefaultFieldSettings : projectDefaultFieldSettings.filter((field) => !isProjectFinanceColumn(field.key)),
    [canReadFinance]
  )
  const fieldSettingsMap = useMemo(() => new Map(availableFieldSettings.map((field) => [field.key, field])), [availableFieldSettings])
  const visibleColumns = useMemo(() => availableFieldSettings.filter((field) => field.visible).map((field) => field.key), [availableFieldSettings])
  const searchableFields = useMemo(() => availableFieldSettings.filter((field) => field.searchable).map((field) => field.key), [availableFieldSettings])
  const sortableFields = useMemo(() => new Set(availableFieldSettings.filter((field) => field.sortable).map((field) => field.key)), [availableFieldSettings])
  const isProjectFieldEditable = (key: ProjectColumnKey) => fieldSettingsMap.get(key)?.editable ?? true
  const activeFilterCount = Number(Boolean(keyword.trim()) && searchableFields.length > 0) + Number(sortKey !== 'updatedAt') + Number(sortOrder !== 'desc')

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const projectPage = await fetchPage<Project>(
        '/projects',
        { page, pageSize, keyword: searchableFields.length > 0 ? keyword : '', searchFields: searchableFields.join(','), sortBy: sortKey, sortOrder },
        { page, pageSize }
      )
      setProjects(projectPage.list)
      setTotal(projectPage.total)
      if (projectPage.list.length > 0 && !selected) setSelected(projectPage.list[0].id)
      if (projectPage.list.length === 0) setSelected(undefined)
    } catch (loadError) {
      setError(readApiError(loadError, '项目列表加载失败'))
      setProjects([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [page, pageSize, keyword, sortKey, sortOrder, searchableFields])

  useEffect(() => {
    if (sortKey === 'updatedAt') return
    if (!sortableFields.has(sortKey as ProjectColumnKey)) {
      setSortKey('updatedAt')
      setSortOrder('desc')
    }
  }, [sortKey, sortableFields])

  useEffect(() => {
    localStorage.setItem(projectFieldSettingsStorageKey, JSON.stringify(fieldSettings))
  }, [fieldSettings])

  useEffect(() => {
    if (!selected) return
    void fetchArray<Task>(`/projects/${selected}/task-tree`, undefined, { silent: true }).then(setTree).catch(() => setTree([]))
  }, [selected])

  useEffect(() => {
    if (!modalOpen) return
    const timer = window.setTimeout(() => {
      void Promise.all([
        fetchPage<User>('/users', { page: 1, pageSize: 200, keyword: ownerKeyword.trim() }, { page: 1, pageSize: 200 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 200 })),
        fetchPage<Department>('/departments', { page: 1, pageSize: 200, keyword: departmentKeyword.trim() }, { page: 1, pageSize: 200 }, { silent: true }).catch(() => ({ list: [], total: 0, page: 1, pageSize: 200 }))
      ]).then(([userPage, departmentPage]) => {
        setUsers(userPage.list)
        setDepartments(departmentPage.list)
      })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [modalOpen, ownerKeyword, departmentKeyword])

  const openCreateModal = () => {
    if (!canCreateProject) return
    setForm(emptyProjectForm())
    setFormError('')
    setFormSuccess('')
    setOwnerKeyword('')
    setDepartmentKeyword('')
    setModalOpen(true)
  }

  const edit = (item: Project) => {
    if (!canUpdateProject) return
    setForm({
      id: item.id,
      code: item.code,
      name: item.name,
      description: item.description,
      startAt: item.startAt ? item.startAt.slice(0, 16) : '',
      endAt: item.endAt ? item.endAt.slice(0, 16) : '',
      budgetAmount: typeof item.budgetAmount === 'number' ? String(item.budgetAmount) : '',
      actualCostAmount: typeof item.actualCostAmount === 'number' ? String(item.actualCostAmount) : '',
      expectedRevenueAmount: typeof item.expectedRevenueAmount === 'number' ? String(item.expectedRevenueAmount) : '',
      contractNo: item.contractNo || '',
      contractAttachments: normalizeContractAttachments(item.contractAttachments),
      attachments: normalizeAttachments(item),
      userIds: (item.users || []).map((user) => user.id),
      departmentIds: (item.departments || []).map((department) => department.id)
    })
    setFormError('')
    setFormSuccess('')
    setOwnerKeyword('')
    setDepartmentKeyword('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateProject) return
    if (!form.id && !canCreateProject) return
    if (form.startAt && form.endAt && new Date(form.startAt) > new Date(form.endAt)) {
      setFormError('结束时间必须晚于开始时间')
      return
    }
    const numericFields = [form.budgetAmount, form.actualCostAmount, form.expectedRevenueAmount]
    if (canUpdateFinance && numericFields.some((value) => value !== '' && (Number.isNaN(Number(value)) || Number(value) < 0))) {
      setFormError('预算、成本和预计收益必须是非负数字')
      return
    }
    const contractAttachments = form.contractAttachments.map((item) => {
      const expiresAt = parseOptionalDateTime(item.expiresAt)
      if (expiresAt === null) return null
      return {
        ...item,
        expiresAt,
        category: item.category || 'contract',
        accessLevel: item.accessLevel || 'finance',
        version: item.version || ''
      }
    })
    if (contractAttachments.some((item) => item === null)) {
      setFormError('合同附件到期时间格式不正确')
      return
    }
    const payload = {
      code: form.code,
      name: form.name,
      description: form.description,
      attachments: form.attachments,
      userIds: form.userIds,
      departmentIds: form.departmentIds,
      startAt: form.startAt ? new Date(form.startAt).toISOString() : '',
      endAt: form.endAt ? new Date(form.endAt).toISOString() : '',
      ...(canUpdateFinance ? {
        budgetAmount: form.budgetAmount === '' ? 0 : Number(form.budgetAmount),
        actualCostAmount: form.actualCostAmount === '' ? 0 : Number(form.actualCostAmount),
        expectedRevenueAmount: form.expectedRevenueAmount === '' ? 0 : Number(form.expectedRevenueAmount),
        contractNo: form.contractNo,
        contractAttachments
      } : {})
    }

    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) await api.put(`/projects/${form.id}`, payload)
      else await api.post('/projects', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(emptyProjectForm())
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存项目失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    if (!canDeleteProject) return
    if (!confirm('确认删除该项目？')) return
    try {
      await api.delete(`/projects/${id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除项目失败'))
    }
  }

  const viewDetail = async (item: Project) => {
    try {
      const detail = await fetchData<Project>(`/projects/${item.id}`)
      setDetailProject(detail || item)
    } catch {
      setDetailProject(item)
    }
    setDetailOpen(true)
  }

  const updateContractAttachment = (filePath: string, patch: Partial<ContractAttachment>) => {
    setForm((prev) => ({
      ...prev,
      contractAttachments: prev.contractAttachments.map((item) => item.filePath === filePath ? { ...item, ...patch } : item)
    }))
  }

  const renderProjectHeaderCell = (key: ProjectColumnKey) => {
    const label = fieldSettingsMap.get(key)?.label || key
    return <th key={key}>{label}</th>
  }

  const renderProjectCell = (project: Project, key: ProjectColumnKey) => {
    switch (key) {
      case 'code':
        return <td key={key} data-label="编码">{project.code}</td>
      case 'name':
        return <td key={key} data-label="名称">{project.name}</td>
      case 'description':
        return <td key={key} data-label="描述">{project.description || '-'}</td>
      case 'owners':
        return (
          <td key={key} data-label="负责人">
            <div className="task-user-stack">
              {getProjectOwnerNames(project).length > 0 ? getProjectOwnerNames(project).map((name) => (
                <span key={name} className="task-user-line">{name}</span>
              )) : <span>-</span>}
            </div>
          </td>
        )
      case 'departments':
        return (
          <td key={key} data-label="部门">
            <div className="task-user-stack">
              {getProjectDepartmentNames(project).length > 0 ? getProjectDepartmentNames(project).map((name) => (
                <span key={name} className="task-user-line">{name}</span>
              )) : <span>-</span>}
            </div>
          </td>
        )
      case 'budgetAmount':
        return <td key={key} data-label="预算">{formatMoney(project.budgetAmount)}</td>
      case 'actualCostAmount':
        return <td key={key} data-label="实际成本">{formatMoney(project.actualCostAmount)}</td>
      case 'contractNo':
        return <td key={key} data-label="合同编号">{project.contractNo || '-'}</td>
      case 'budgetUsageRate':
        return <td key={key} data-label="预算使用率">{formatRate(project.budgetUsageRate)}</td>
      case 'startAt':
        return <td key={key} data-label="开始时间">{formatDateTime(project.startAt)}</td>
      case 'endAt':
        return <td key={key} data-label="结束时间">{formatDateTime(project.endAt)}</td>
      case 'createdAt':
        return <td key={key} data-label="创建时间">{formatDateTime(project.createdAt)}</td>
      case 'updatedAt':
        return <td key={key} data-label="更新时间">{formatDateTime(project.updatedAt)}</td>
      default:
        return null
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="项目筛选"
        activeCount={activeFilterCount}
        actions={canCreateProject ? <button className="btn" onClick={openCreateModal}>新增项目</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        {searchableFields.length > 0 && <SearchField className="toolbar-search-field" aria-label="搜索项目" value={keyword} placeholder="搜索：已启用可搜索字段" onChange={(value) => { setKeyword(value); setPage(1) }} />}
        <select aria-label="项目排序字段" value={sortKey} onChange={(e) => { setSortKey(e.target.value as SortKey); setPage(1) }}>
          <option value="updatedAt">按更新时间</option>
          <option value="createdAt">按创建时间</option>
          {sortableFields.has('code') && <option value="code">按编码</option>}
          {sortableFields.has('name') && <option value="name">按名称</option>}
          {sortableFields.has('startAt') && <option value="startAt">按开始时间</option>}
          {sortableFields.has('endAt') && <option value="endAt">按结束时间</option>}
        </select>
        <select aria-label="项目排序方式" value={sortOrder} onChange={(e) => { setSortOrder(e.target.value as SortOrder); setPage(1) }}>
          <option value="desc">降序</option>
          <option value="asc">升序</option>
        </select>
      </FilterPanel>

      <div className="card">
        <label htmlFor="project-select">选择项目</label>
        <div className="row-actions">
          <select id="project-select" value={selected} onChange={(e) => setSelected(Number(e.target.value))}>
            {projects.map((p) => <option key={p.id} value={p.id}>{p.code} - {p.name}</option>)}
          </select>
          <button className="btn secondary" onClick={() => navigate(selected ? `/gantt?projectId=${selected}` : '/gantt')}>
            打开甘特模块
          </button>
        </div>
      </div>

      <TaskTree tasks={tree} />

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && projects.length === 0} emptyText="暂无匹配的项目" onRetry={() => { void load() }} />
        {!loading && !error && projects.length > 0 && (
          <table className="responsive-table"><thead><tr>
            {visibleColumns.map((columnKey) => renderProjectHeaderCell(columnKey))}
            <th className="field-settings-header-cell">
              <span className="field-settings-header-inline">
                <span>操作</span>
                <button type="button" className="field-settings-icon-btn" aria-label="项目列表字段设置" onClick={() => setFieldSettingsOpen(true)}>
                  <Settings2 size={16} />
                </button>
              </span>
            </th>
          </tr></thead><tbody>
            {projects.map((p) => (
              <tr key={p.id}>
                {visibleColumns.map((columnKey) => renderProjectCell(p, columnKey))}
                <td data-label="操作">
                  <div className="table-actions">
                    <button className="btn secondary" onClick={() => { void viewDetail(p) }}>查看详情</button>
                    {canUpdateProject && <button className="btn secondary" onClick={() => edit(p)}>编辑</button>}
                    {canDeleteProject && <button className="btn danger" onClick={() => onDelete(p.id)}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <FieldSettingsModal
        open={fieldSettingsOpen}
        title="项目列表字段设置"
        fields={availableFieldSettings}
        defaultFields={availableDefaultFieldSettings}
        onClose={() => setFieldSettingsOpen(false)}
        onSave={(fields) => {
          const nextFields = canReadFinance ? fields : [
            ...fields,
            ...fieldSettings.filter((field) => isProjectFinanceColumn(field.key))
          ]
          setFieldSettings(normalizeProjectFieldSettings(nextFields))
          setFieldSettingsOpen(false)
        }}
      />

      <Modal open={detailOpen} title="项目详情" onClose={() => setDetailOpen(false)}>
        {detailProject && (
          <div className="detail-grid">
            <section className="detail-section">
              <h4>基础信息</h4>
              <div className="detail-columns">
                <div><strong>项目ID：</strong>{detailProject.id}</div>
                <div><strong>编码：</strong>{detailProject.code || '-'}</div>
                <div><strong>名称：</strong>{detailProject.name || '-'}</div>
              </div>
              <div className="detail-description-card">
                <strong>描述</strong>
                <p>{detailProject.description || '-'}</p>
              </div>
              <div className="detail-columns">
                <div>
                  <strong>附件：</strong>
                  {normalizeAttachments(detailProject).length > 0 ? normalizeAttachments(detailProject).map((item) => (
                    <a key={item.filePath} href={item.filePath} target="_blank" rel="noreferrer">{item.relativePath || item.fileName || '附件'}</a>
                  )) : '-'}
                </div>
              </div>
            </section>
            <section className="detail-section">
              <h4>时间信息</h4>
              <div className="detail-columns">
                <div className="detail-time-line"><strong>项目周期：</strong>{formatDateTime(detailProject.startAt)} - {formatDateTime(detailProject.endAt)}</div>
              </div>
            </section>
            {canReadFinance && (
              <section className="detail-section">
                <h4>预算与合同</h4>
                <div className="finance-summary-grid">
                  <div className="finance-summary-item"><span>预算</span><strong>{formatMoney(detailProject.budgetAmount)}</strong></div>
                  <div className="finance-summary-item"><span>实际成本</span><strong>{formatMoney(detailProject.actualCostAmount)}</strong></div>
                  <div className="finance-summary-item"><span>预计收益</span><strong>{formatMoney(detailProject.expectedRevenueAmount)}</strong></div>
                  <div className={`finance-summary-item${detailProject.costOverBudget ? ' danger' : ''}`}><span>预算使用率</span><strong>{formatRate(detailProject.budgetUsageRate)}</strong></div>
                </div>
                <div className="detail-columns">
                  <div><strong>合同编号：</strong>{detailProject.contractNo || '-'}</div>
                  <div><strong>成本状态：</strong>{detailProject.costOverBudget ? '超预算' : '未超预算'}</div>
                </div>
                <div className="contract-attachment-list">
                  {(detailProject.contractAttachments || []).length > 0 ? (detailProject.contractAttachments || []).map((item) => (
                    <article key={item.filePath} className="contract-attachment-item">
                      <a href={item.filePath} target="_blank" rel="noreferrer">{item.relativePath || item.fileName || '合同附件'}</a>
                      <span>{contractCategoryLabel[item.category || 'contract']}</span>
                      <span>{item.version || '未标版本'}</span>
                      <span>{contractAccessLevelLabel[item.accessLevel || 'finance']}</span>
                      <span>{item.expiresAt ? `到期：${formatDateTime(item.expiresAt)}` : '无到期提醒'}</span>
                    </article>
                  )) : <p className="inline-tip">暂无合同附件</p>}
                </div>
              </section>
            )}
            <section className="detail-section">
              <h4>关联信息</h4>
              <div className="detail-columns">
                <div><strong>负责人：</strong>{(detailProject.users || []).map((u) => `${u.name}(${u.username})`).join('，') || '-'}</div>
                <div><strong>参与部门：</strong>{(detailProject.departments || []).map((d) => d.name).join('，') || '-'}</div>
                <div><strong>任务数量：</strong>{Array.isArray(detailProject.tasks) ? detailProject.tasks.length : '-'}</div>
              </div>
            </section>
          </div>
        )}
      </Modal>

      <Modal open={modalOpen} title={form.id ? '编辑项目' : '新增项目'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label htmlFor="project-code">编码（可空自动生成）</label>
          <input id="project-code" value={form.code} onChange={(e) => setForm((prev) => ({ ...prev, code: e.target.value }))} disabled={!isProjectFieldEditable('code')} />
          <label className="required-label" htmlFor="project-name">名称</label>
          <input id="project-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required disabled={!isProjectFieldEditable('name')} />
          <label htmlFor="project-description">描述</label>
          <textarea id="project-description" rows={4} value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} disabled={!isProjectFieldEditable('description')} />
          <label htmlFor="project-attachment">附件</label>
          <AttachmentField inputId="project-attachment" value={form.attachments} disabled={!canUploadAttachment} onChange={(attachments) => setForm((prev) => ({ ...prev, attachments }))} />
          {canUpdateFinance && (
            <>
              <label htmlFor="project-budget">预算</label>
              <input id="project-budget" type="number" min="0" step="0.01" value={form.budgetAmount} onChange={(e) => setForm((prev) => ({ ...prev, budgetAmount: e.target.value }))} />
              <label htmlFor="project-actual-cost">实际成本</label>
              <input id="project-actual-cost" type="number" min="0" step="0.01" value={form.actualCostAmount} onChange={(e) => setForm((prev) => ({ ...prev, actualCostAmount: e.target.value }))} />
              <label htmlFor="project-expected-revenue">预计收益</label>
              <input id="project-expected-revenue" type="number" min="0" step="0.01" value={form.expectedRevenueAmount} onChange={(e) => setForm((prev) => ({ ...prev, expectedRevenueAmount: e.target.value }))} />
              <label htmlFor="project-contract-no">合同编号</label>
              <input id="project-contract-no" value={form.contractNo} onChange={(e) => setForm((prev) => ({ ...prev, contractNo: e.target.value }))} />
              <label htmlFor="project-contract-attachments">合同附件</label>
              <div className="contract-attachment-editor">
                <AttachmentField
                  inputId="project-contract-attachments"
                  value={form.contractAttachments}
                  disabled={!canUploadAttachment}
                  onChange={(attachments) => setForm((prev) => ({ ...prev, contractAttachments: mergeContractAttachmentMetadata(prev.contractAttachments, attachments) }))}
                />
                {form.contractAttachments.length > 0 && (
                  <div className="contract-attachment-meta-list">
                    {form.contractAttachments.map((item) => (
                      <article key={item.filePath} className="contract-attachment-meta">
                        <strong>{item.relativePath || item.fileName || '合同附件'}</strong>
                        <label>
                          分类
                          <select value={item.category || 'contract'} onChange={(event) => updateContractAttachment(item.filePath, { category: event.target.value as ContractAttachment['category'] })}>
                            {Object.entries(contractCategoryLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                          </select>
                        </label>
                        <label>
                          版本
                          <input value={item.version || ''} onChange={(event) => updateContractAttachment(item.filePath, { version: event.target.value })} />
                        </label>
                        <label>
                          权限级别
                          <select value={item.accessLevel || 'finance'} onChange={(event) => updateContractAttachment(item.filePath, { accessLevel: event.target.value as ContractAttachment['accessLevel'] })}>
                            {Object.entries(contractAccessLevelLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                          </select>
                        </label>
                        <label>
                          到期提醒
                          <DateTimeQuickField inputId={`contract-expires-${item.filePath.replace(/[^a-zA-Z0-9_-]/g, '-')}`} value={item.expiresAt || ''} onChange={(value) => updateContractAttachment(item.filePath, { expiresAt: value })} />
                        </label>
                      </article>
                    ))}
                  </div>
                )}
              </div>
            </>
          )}
          <label htmlFor="project-start">开始时间</label>
          <DateTimeQuickField
            inputId="project-start"
            value={form.startAt}
            disabled={!isProjectFieldEditable('startAt')}
            onChange={(value) => setForm((prev) => ({ ...prev, startAt: value }))}
          />
          <label htmlFor="project-end">结束时间</label>
          <DateTimeQuickField
            inputId="project-end"
            value={form.endAt}
            disabled={!isProjectFieldEditable('endAt')}
            onChange={(value) => setForm((prev) => ({ ...prev, endAt: value }))}
          />
          <label htmlFor="project-users">项目负责人</label>
          <SearchField aria-label="搜索负责人" placeholder="搜索负责人：姓名/用户名/邮箱" value={ownerKeyword} onChange={setOwnerKeyword} disabled={!isProjectFieldEditable('owners')} />
          <div id="project-users" className="multi-checklist">
            {users.map((user) => (
              <label key={user.id} className="multi-check-item">
                <input type="checkbox" checked={form.userIds.includes(user.id)} onChange={() => setForm((prev) => ({ ...prev, userIds: toggleNumber(prev.userIds, user.id) }))} disabled={!isProjectFieldEditable('owners')} />
                <span>{user.name} ({user.username})</span>
              </label>
            ))}
          </div>
          <label htmlFor="project-departments">参与部门</label>
          <SearchField aria-label="搜索部门" placeholder="搜索部门名称" value={departmentKeyword} onChange={setDepartmentKeyword} disabled={!isProjectFieldEditable('departments')} />
          <div id="project-departments" className="multi-checklist">
            {departments.map((department) => (
              <label key={department.id} className="multi-check-item">
                <input type="checkbox" checked={form.departmentIds.includes(department.id)} onChange={() => setForm((prev) => ({ ...prev, departmentIds: toggleNumber(prev.departmentIds, department.id) }))} disabled={!isProjectFieldEditable('departments')} />
                <span>{department.name}</span>
              </label>
            ))}
          </div>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateProject : !canCreateProject)}>{submitting ? '保存中...' : '保存项目'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(emptyProjectForm())}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
