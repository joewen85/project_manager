import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, fetchArray, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { ApiToken, ApiTokenCreateResponse, Permission } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface ApiTokenForm {
  id?: number
  name: string
  description: string
  permissionIds: number[]
  isEnabled: boolean
  expiresAt: string
}

const initialForm: ApiTokenForm = {
  name: '',
  description: '',
  permissionIds: [],
  isEnabled: true,
  expiresAt: ''
}

const toRequestTime = (value: string) => value ? new Date(value).toISOString() : ''

const toFormTime = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const offsetMs = date.getTimezoneOffset() * 60000
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16)
}

export function ApiTokensPage() {
  const permissions = usePermissions()
  const canCreateToken = hasPermission('system.api_tokens.create', permissions)
  const canUpdateToken = hasPermission('system.api_tokens.update', permissions)
  const canDeleteToken = hasPermission('system.api_tokens.delete', permissions)
  const [items, setItems] = useState<ApiToken[]>([])
  const [permissionOptions, setPermissionOptions] = useState<Permission[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [enabledFilter, setEnabledFilter] = useState('')
  const [form, setForm] = useState<ApiTokenForm>(initialForm)
  const [createdToken, setCreatedToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(enabledFilter))

  const permissionByCode = useMemo(() => {
    const map = new Map<string, Permission>()
    permissionOptions.forEach((item) => map.set(item.code, item))
    return map
  }, [permissionOptions])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const pageData = await fetchPage<ApiToken>('/system/api-tokens', { page, pageSize, keyword, isEnabled: enabledFilter }, { page, pageSize })
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      setError(readApiError(loadError, 'API Token 加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadPermissionOptions = async () => {
    try {
      const list = await fetchArray<Permission>('/system/api-tokens/permission-options')
      setPermissionOptions(list)
    } catch {
      setPermissionOptions([])
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, enabledFilter])
  useEffect(() => { void loadPermissionOptions() }, [])

  const openCreateModal = () => {
    if (!canCreateToken) return
    setForm(initialForm)
    setFormError('')
    setCreatedToken('')
    setModalOpen(true)
  }

  const openEditModal = (item: ApiToken) => {
    if (!canUpdateToken) return
    const permissionIds = item.permissionCodes
      .map((code) => permissionByCode.get(code)?.id || 0)
      .filter((id) => id > 0)
    setForm({
      id: item.id,
      name: item.name,
      description: item.description || '',
      permissionIds,
      isEnabled: item.isEnabled,
      expiresAt: toFormTime(item.expiresAt)
    })
    setFormError('')
    setCreatedToken('')
    setModalOpen(true)
  }

  const togglePermission = (id: number) => {
    setForm((prev) => {
      const exists = prev.permissionIds.includes(id)
      return {
        ...prev,
        permissionIds: exists ? prev.permissionIds.filter((item) => item !== id) : [...prev.permissionIds, id]
      }
    })
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateToken) return
    if (!form.id && !canCreateToken) return
    try {
      setSubmitting(true)
      setFormError('')
      setCreatedToken('')
      const payload = {
        name: form.name.trim(),
        description: form.description.trim(),
        permissionIds: form.permissionIds,
        isEnabled: form.isEnabled,
        expiresAt: toRequestTime(form.expiresAt)
      }
      if (form.id) {
        await api.put(`/system/api-tokens/${form.id}`, payload)
        setModalOpen(false)
      } else {
        const res = await api.post<ApiTokenCreateResponse>('/system/api-tokens', payload)
        setCreatedToken(res.data.token)
      }
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存 API Token 失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const revokeToken = async (item: ApiToken) => {
    if (!canDeleteToken) return
    if (!confirm(`确认撤销 API Token「${item.name}」？`)) return
    try {
      await api.delete(`/system/api-tokens/${item.id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '撤销 API Token 失败'))
    }
  }

  const permissionLabel = (code: string) => permissionByCode.get(code)?.name || code

  return (
    <section className="page-section">
      <FilterPanel
        title="Token 筛选"
        activeCount={activeFilterCount}
        actions={canCreateToken ? <button className="btn secondary" onClick={openCreateModal}>新增 Token</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="API Token 关键词搜索"
          value={keywordInput}
          placeholder="搜索名称/描述/前缀"
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
        <select aria-label="API Token 启用状态筛选" value={enabledFilter} onChange={(event) => { setEnabledFilter(event.target.value); setPage(1) }}>
          <option value="">全部状态</option>
          <option value="true">启用</option>
          <option value="false">停用</option>
        </select>
        <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无 API Token" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table">
            <thead><tr><th>名称</th><th>服务账号</th><th>权限</th><th>状态</th><th>最近使用</th><th>操作</th></tr></thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td data-label="名称">
                    <strong>{item.name}</strong>
                    <p className="table-subtext">{item.tokenPrefix}****{item.tokenLastFour}</p>
                  </td>
                  <td data-label="服务账号">{item.serviceAccount?.username || item.serviceAccountId}</td>
                  <td data-label="权限">
                    <span className="table-subtext">{item.permissionCodes.slice(0, 4).map(permissionLabel).join('、') || '-'}</span>
                    {item.permissionCodes.length > 4 && <p className="table-subtext">另 {item.permissionCodes.length - 4} 项</p>}
                  </td>
                  <td data-label="状态">
                    {item.revokedAt ? '已撤销' : item.isEnabled ? '启用' : '停用'}
                    {item.expiresAt && <p className="table-subtext">过期：{formatDateTime(item.expiresAt)}</p>}
                  </td>
                  <td data-label="最近使用">
                    {formatDateTime(item.lastUsedAt)}
                    {item.lastUsedIp && <p className="table-subtext">{item.lastUsedIp}</p>}
                  </td>
                  <td data-label="操作">
                    <div className="table-actions">
                      {canUpdateToken && !item.revokedAt && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                      {canDeleteToken && !item.revokedAt && <button className="btn danger" onClick={() => { void revokeToken(item) }}>撤销</button>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑 API Token' : '新增 API Token'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="api-token-name">名称</label>
          <input id="api-token-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="api-token-description">描述</label>
          <input id="api-token-description" value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          <label htmlFor="api-token-expires">过期时间</label>
          <DateTimeQuickField inputId="api-token-expires" value={form.expiresAt} onChange={(value) => setForm((prev) => ({ ...prev, expiresAt: value }))} />
          <label htmlFor="api-token-enabled">启用</label>
          <div className="multi-checklist compact">
            <label className="multi-check-item">
              <input id="api-token-enabled" type="checkbox" checked={form.isEnabled} onChange={() => setForm((prev) => ({ ...prev, isEnabled: !prev.isEnabled }))} />
              <span>允许通过 Bearer Token 调用接口</span>
            </label>
          </div>
          <label className="required-label">权限</label>
          <div className="multi-checklist">
            {permissionOptions.map((permission) => (
              <label className="multi-check-item" key={permission.id}>
                <input type="checkbox" checked={form.permissionIds.includes(permission.id)} onChange={() => togglePermission(permission.id)} />
                <span>{permission.name}<small>{permission.code}</small></span>
              </label>
            ))}
          </div>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {createdToken && (
            <div className="success">
              <label htmlFor="created-api-token">一次性 Token</label>
              <input id="created-api-token" readOnly value={createdToken} onFocus={(event) => event.currentTarget.select()} />
            </div>
          )}
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>
    </section>
  )
}
