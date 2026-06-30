import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { AutomationExecutionLog, AutomationRule, Project } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface AutomationRuleForm {
  id?: number
  name: string
  isEnabled: boolean
  overdueDays: number
  projectIds: number[]
  notifyAssignees: boolean
  notifyProjectOwners: boolean
}

const initialForm: AutomationRuleForm = {
  name: '',
  isEnabled: true,
  overdueDays: 1,
  projectIds: [],
  notifyAssignees: true,
  notifyProjectOwners: true
}

const triggerLabel = {
  task_overdue: '任务逾期'
}

const statusLabel = {
  success: '成功',
  skipped: '跳过',
  failed: '失败'
}

const sourceLabel = {
  manual: '手动',
  scheduled: '定时'
}

const toggleNumber = (list: number[], id: number) => list.includes(id) ? list.filter((item) => item !== id) : [...list, id]

export function AutomationRulesPage() {
  const permissions = usePermissions()
  const canCreateRule = hasPermission('automations.create', permissions)
  const canUpdateRule = hasPermission('automations.update', permissions)
  const canDeleteRule = hasPermission('automations.delete', permissions)
  const canReadProjects = hasPermission('projects.read', permissions)
  const [items, setItems] = useState<AutomationRule[]>([])
  const [logs, setLogs] = useState<AutomationExecutionLog[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [enabledFilter, setEnabledFilter] = useState<'all' | 'enabled' | 'disabled'>('all')
  const [form, setForm] = useState<AutomationRuleForm>(initialForm)
  const [loading, setLoading] = useState(false)
  const [logLoading, setLogLoading] = useState(false)
  const [error, setError] = useState('')
  const [logError, setLogError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(enabledFilter !== 'all')

  const projectNameById = useMemo(() => new Map(projects.map((project) => [project.id, `${project.code} - ${project.name}`])), [projects])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const isEnabled = enabledFilter === 'all' ? '' : enabledFilter === 'enabled'
      const pageData = await fetchPage<AutomationRule>(
        '/automation-rules',
        { page, pageSize, keyword, trigger: 'task_overdue', isEnabled },
        { page, pageSize }
      )
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      setError(readApiError(loadError, '自动化规则加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadLogs = async () => {
    try {
      setLogLoading(true)
      setLogError('')
      const pageData = await fetchPage<AutomationExecutionLog>('/automation-rules/logs', { page: 1, pageSize: 8 }, { page: 1, pageSize: 8 }, { silent: true })
      setLogs(pageData.list)
    } catch (loadError) {
      setLogError(readApiError(loadError, '执行日志加载失败'))
      setLogs([])
    } finally {
      setLogLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, enabledFilter])
  useEffect(() => { void loadLogs() }, [])

  useEffect(() => {
    if (!canReadProjects) {
      setProjects([])
      return
    }
    void fetchPage<Project>('/projects', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true })
      .then((data) => setProjects(data.list))
      .catch(() => setProjects([]))
  }, [canReadProjects])

  const openCreateModal = () => {
    if (!canCreateRule) return
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openEditModal = (item: AutomationRule) => {
    if (!canUpdateRule) return
    setForm({
      id: item.id,
      name: item.name,
      isEnabled: item.isEnabled,
      overdueDays: item.conditions?.overdueDays ?? 1,
      projectIds: item.conditions?.projectIds || [],
      notifyAssignees: item.actions?.notifyAssignees ?? true,
      notifyProjectOwners: item.actions?.notifyProjectOwners ?? true
    })
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateRule) return
    if (!form.id && !canCreateRule) return
    if (!form.notifyAssignees && !form.notifyProjectOwners) {
      setFormError('至少选择一个通知对象')
      return
    }
    try {
      setSubmitting(true)
      setFormError('')
      const payload = {
        name: form.name,
        trigger: 'task_overdue',
        isEnabled: form.isEnabled,
        conditions: {
          overdueDays: Number(form.overdueDays || 0),
          projectIds: form.projectIds
        },
        actions: {
          notifyAssignees: form.notifyAssignees,
          notifyProjectOwners: form.notifyProjectOwners
        }
      }
      if (form.id) await api.put(`/automation-rules/${form.id}`, payload)
      else await api.post('/automation-rules', payload)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存自动化规则失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const runRule = async (item: AutomationRule) => {
    if (!canUpdateRule) return
    try {
      setError('')
      await api.post(`/automation-rules/${item.id}/run`)
      await load()
      await loadLogs()
      window.dispatchEvent(new Event('notifications:changed'))
    } catch (runError) {
      setError(readApiError(runError, '执行自动化规则失败'))
    }
  }

  const onDelete = async (id: number) => {
    if (!canDeleteRule) return
    if (!confirm('确认删除该自动化规则？')) return
    try {
      await api.delete(`/automation-rules/${id}`)
      await load()
      await loadLogs()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除自动化规则失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="自动化筛选"
        activeCount={activeFilterCount}
        actions={canCreateRule ? <button className="btn" onClick={openCreateModal}>新增规则</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="自动化规则关键词搜索"
          value={keywordInput}
          placeholder="搜索规则名称"
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
        <select value={enabledFilter} aria-label="启用状态筛选" onChange={(event) => { setEnabledFilter(event.target.value as typeof enabledFilter); setPage(1) }}>
          <option value="all">全部状态</option>
          <option value="enabled">已启用</option>
          <option value="disabled">已停用</option>
        </select>
        <div className="row-actions">
          <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
        </div>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无自动化规则" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>ID</th><th>规则名称</th><th>触发器</th><th>状态</th><th>条件</th><th>动作</th><th>最近执行</th><th>操作</th>
          </tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="ID">{item.id}</td>
                <td data-label="规则名称">{item.name}</td>
                <td data-label="触发器">{triggerLabel[item.trigger]}</td>
                <td data-label="状态">{item.isEnabled ? '已启用' : '已停用'}</td>
                <td data-label="条件">
                  逾期 {item.conditions?.overdueDays ?? 1} 天
                  {item.conditions?.projectIds?.length ? `；项目 ${item.conditions.projectIds.map((id) => projectNameById.get(id) || id).join('，')}` : ''}
                </td>
                <td data-label="动作">
                  {[
                    item.actions?.notifyAssignees ? '通知执行人' : '',
                    item.actions?.notifyProjectOwners ? '通知项目负责人' : ''
                  ].filter(Boolean).join('，') || '-'}
                </td>
                <td data-label="最近执行">{formatDateTime(item.lastRunAt)}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    {canUpdateRule && <button className="btn secondary" onClick={() => { void runRule(item) }}>执行</button>}
                    {canUpdateRule && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                    {canDeleteRule && <button className="btn danger" onClick={() => { void onDelete(item.id) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <div className="card">
        <div className="row-actions">
          <h3>执行日志</h3>
          <button className="btn secondary" onClick={() => { void loadLogs() }}>刷新</button>
        </div>
        <DataState loading={logLoading} error={logError} empty={!logLoading && !logError && logs.length === 0} emptyText="暂无执行日志" onRetry={() => { void loadLogs() }} />
        {!logLoading && !logError && logs.length > 0 && (
          <table className="responsive-table"><thead><tr>
            <th>时间</th><th>规则</th><th>来源</th><th>状态</th><th>匹配</th><th>动作</th><th>结果</th>
          </tr></thead><tbody>
            {logs.map((item) => (
              <tr key={item.id}>
                <td data-label="时间">{formatDateTime(item.createdAt)}</td>
                <td data-label="规则">{item.rule?.name || item.ruleId}</td>
                <td data-label="来源">{sourceLabel[item.runSource] || item.runSource}</td>
                <td data-label="状态">{statusLabel[item.status] || item.status}</td>
                <td data-label="匹配">{item.matchedCount}</td>
                <td data-label="动作">{item.actionCount}</td>
                <td data-label="结果">{item.message || '-'}</td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      <Modal open={modalOpen} title={form.id ? '编辑自动化规则' : '新增自动化规则'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="automation-name">规则名称</label>
          <input id="automation-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="automation-trigger">触发器</label>
          <select id="automation-trigger" value="task_overdue" disabled>
            <option value="task_overdue">任务逾期</option>
          </select>
          <label htmlFor="automation-enabled">启用状态</label>
          <select id="automation-enabled" value={form.isEnabled ? 'enabled' : 'disabled'} onChange={(event) => setForm((prev) => ({ ...prev, isEnabled: event.target.value === 'enabled' }))}>
            <option value="enabled">已启用</option>
            <option value="disabled">已停用</option>
          </select>
          <label className="required-label" htmlFor="automation-overdue-days">逾期天数</label>
          <input id="automation-overdue-days" type="number" min={0} value={form.overdueDays} onChange={(event) => setForm((prev) => ({ ...prev, overdueDays: Number(event.target.value) }))} required />
          {canReadProjects && (
            <>
              <label htmlFor="automation-projects">项目范围</label>
              <div id="automation-projects" className="multi-checklist">
                {projects.map((project) => (
                  <label key={project.id} className="multi-check-item">
                    <input type="checkbox" checked={form.projectIds.includes(project.id)} onChange={() => setForm((prev) => ({ ...prev, projectIds: toggleNumber(prev.projectIds, project.id) }))} />
                    <span>{project.code} - {project.name}</span>
                  </label>
                ))}
                {projects.length === 0 && <p className="inline-tip">不限制项目范围</p>}
              </div>
            </>
          )}
          <label htmlFor="automation-actions">通知对象</label>
          <div id="automation-actions" className="multi-checklist">
            <label className="multi-check-item">
              <input type="checkbox" checked={form.notifyAssignees} onChange={() => setForm((prev) => ({ ...prev, notifyAssignees: !prev.notifyAssignees }))} />
              <span>任务执行人</span>
            </label>
            <label className="multi-check-item">
              <input type="checkbox" checked={form.notifyProjectOwners} onChange={() => setForm((prev) => ({ ...prev, notifyProjectOwners: !prev.notifyProjectOwners }))} />
              <span>项目负责人</span>
            </label>
          </div>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateRule : !canCreateRule)}>{submitting ? '保存中...' : '保存规则'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
