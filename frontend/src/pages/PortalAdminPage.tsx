import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { AttachmentField } from '../components/AttachmentField'
import { DataState } from '../components/DataState'
import { DateTimeQuickField } from '../components/DateTimeQuickField'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { RemoteProjectSelect } from '../components/RemoteProjectSelect'
import { SearchField } from '../components/SearchField'
import { PortalInvite, PortalInviteCreateResponse, UploadAttachment, emptyUploadAttachments } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface PortalInviteForm {
  id?: number
  name: string
  company: string
  contactName: string
  contactEmail: string
  contactType: 'customer' | 'supplier'
  isEnabled: boolean
  expiresAt: string
  projectId: number
  allowedAttachments: UploadAttachment[]
}

const initialForm: PortalInviteForm = {
  name: '',
  company: '',
  contactName: '',
  contactEmail: '',
  contactType: 'customer',
  isEnabled: true,
  expiresAt: '',
  projectId: 0,
  allowedAttachments: emptyUploadAttachments()
}

const toRequestTime = (value: string) => value ? new Date(value).toISOString() : ''

const toFormTime = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const offsetMs = date.getTimezoneOffset() * 60000
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16)
}

const contactTypeLabel: Record<'customer' | 'supplier', string> = {
  customer: '客户',
  supplier: '供应商'
}

const inviteStatusText = (item: PortalInvite) => {
  if (item.revokedAt) return '已撤销'
  if (!item.isEnabled) return '停用'
  if (item.expiresAt && new Date(item.expiresAt).getTime() <= Date.now()) return '已过期'
  return '启用'
}

export function PortalAdminPage() {
  const permissions = usePermissions()
  const canCreateInvite = hasPermission('portal.create', permissions)
  const canUpdateInvite = hasPermission('portal.update', permissions)
  const canDeleteInvite = hasPermission('portal.delete', permissions)
  const canUploadAttachment = hasPermission('uploads.create', permissions)
  const [items, setItems] = useState<PortalInvite[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [enabledFilter, setEnabledFilter] = useState('')
  const [projectFilter, setProjectFilter] = useState('')
  const [form, setForm] = useState<PortalInviteForm>(initialForm)
  const [createdToken, setCreatedToken] = useState('')
  const [copySuccess, setCopySuccess] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(enabledFilter)) + Number(Boolean(projectFilter))
  const createdPortalLink = useMemo(() => createdToken ? `${window.location.origin}/portal/${createdToken}` : '', [createdToken])

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const pageData = await fetchPage<PortalInvite>('/portal-invites', {
        page,
        pageSize,
        keyword,
        isEnabled: enabledFilter,
        projectId: projectFilter
      }, { page, pageSize })
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      setError(readApiError(loadError, '外部门户邀请加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, enabledFilter, projectFilter])

  const openCreateModal = () => {
    if (!canCreateInvite) return
    setForm(initialForm)
    setFormError('')
    setCreatedToken('')
    setCopySuccess('')
    setModalOpen(true)
  }

  const openEditModal = (item: PortalInvite) => {
    if (!canUpdateInvite) return
    setForm({
      id: item.id,
      name: item.name,
      company: item.company || '',
      contactName: item.contactName || '',
      contactEmail: item.contactEmail || '',
      contactType: item.contactType || 'customer',
      isEnabled: item.isEnabled,
      expiresAt: toFormTime(item.expiresAt),
      projectId: item.projectId,
      allowedAttachments: item.allowedAttachments || emptyUploadAttachments()
    })
    setFormError('')
    setCreatedToken('')
    setCopySuccess('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateInvite) return
    if (!form.id && !canCreateInvite) return
    if (!form.projectId) {
      setFormError('请选择授权项目')
      return
    }
    try {
      setSubmitting(true)
      setFormError('')
      setCreatedToken('')
      setCopySuccess('')
      const payload = {
        name: form.name.trim(),
        company: form.company.trim(),
        contactName: form.contactName.trim(),
        contactEmail: form.contactEmail.trim(),
        contactType: form.contactType,
        isEnabled: form.isEnabled,
        expiresAt: toRequestTime(form.expiresAt),
        projectId: form.projectId,
        allowedAttachments: form.allowedAttachments
      }
      if (form.id) {
        await api.put(`/portal-invites/${form.id}`, payload)
        setModalOpen(false)
      } else {
        const res = await api.post<PortalInviteCreateResponse>('/portal-invites', payload)
        setCreatedToken(res.data.token)
      }
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存外部门户邀请失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const copyLink = async () => {
    if (!createdPortalLink) return
    try {
      await navigator.clipboard.writeText(createdPortalLink)
      setCopySuccess('访问链接已复制')
    } catch {
      setCopySuccess('请手动选择链接复制')
    }
  }

  const revokeInvite = async (item: PortalInvite) => {
    if (!canUpdateInvite) return
    if (!confirm(`确认撤销外部门户邀请「${item.name}」？`)) return
    try {
      await api.patch(`/portal-invites/${item.id}/revoke`)
      await load()
    } catch (revokeError) {
      setError(readApiError(revokeError, '撤销外部门户邀请失败'))
    }
  }

  const deleteInvite = async (item: PortalInvite) => {
    if (!canDeleteInvite) return
    if (!confirm(`确认删除外部门户邀请「${item.name}」？`)) return
    try {
      await api.delete(`/portal-invites/${item.id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除外部门户邀请失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="外部门户筛选"
        activeCount={activeFilterCount}
        actions={canCreateInvite ? <button className="btn secondary" onClick={openCreateModal}>新增邀请</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="外部门户关键词搜索"
          value={keywordInput}
          placeholder="搜索名称/公司/联系人/邮箱/Token 前缀"
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
        <RemoteProjectSelect
          ariaLabel="外部门户项目筛选"
          value={projectFilter}
          defaultOptionLabel="全部项目"
          placeholder="搜索项目：编码/名称/描述"
          noResultsText="没有匹配的项目"
          onChange={(value) => {
            setProjectFilter(value)
            setPage(1)
          }}
        />
        <select aria-label="外部门户启用状态筛选" value={enabledFilter} onChange={(event) => { setEnabledFilter(event.target.value); setPage(1) }}>
          <option value="">全部状态</option>
          <option value="true">启用</option>
          <option value="false">停用</option>
        </select>
        <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无外部门户邀请" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table">
            <thead><tr><th>邀请</th><th>项目</th><th>联系人</th><th>授权附件</th><th>状态</th><th>最近访问</th><th>操作</th></tr></thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td data-label="邀请">
                    <strong>{item.name}</strong>
                    <p className="table-subtext">{item.tokenPrefix}****{item.tokenLastFour}</p>
                  </td>
                  <td data-label="项目">{item.project ? `${item.project.code} - ${item.project.name}` : item.projectId}</td>
                  <td data-label="联系人">
                    {item.contactName || '-'}
                    <p className="table-subtext">{contactTypeLabel[item.contactType] || item.contactType} {item.company || ''}</p>
                    {item.contactEmail && <p className="table-subtext">{item.contactEmail}</p>}
                  </td>
                  <td data-label="授权附件">
                    {(item.allowedAttachments || []).length > 0 ? `${(item.allowedAttachments || []).length} 个` : '-'}
                  </td>
                  <td data-label="状态">
                    {inviteStatusText(item)}
                    {item.expiresAt && <p className="table-subtext">过期：{formatDateTime(item.expiresAt)}</p>}
                  </td>
                  <td data-label="最近访问">
                    {formatDateTime(item.lastUsedAt)}
                    {item.lastUsedIp && <p className="table-subtext">{item.lastUsedIp}</p>}
                  </td>
                  <td data-label="操作">
                    <div className="table-actions">
                      {canUpdateInvite && !item.revokedAt && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                      {canUpdateInvite && !item.revokedAt && <button className="btn danger" onClick={() => { void revokeInvite(item) }}>撤销</button>}
                      {canDeleteInvite && <button className="btn danger" onClick={() => { void deleteInvite(item) }}>删除</button>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑外部门户邀请' : '新增外部门户邀请'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="portal-invite-name">邀请名称</label>
          <input id="portal-invite-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label className="required-label">授权项目</label>
          <RemoteProjectSelect
            ariaLabel="外部门户授权项目"
            value={form.projectId ? String(form.projectId) : ''}
            defaultOptionLabel="请选择项目"
            placeholder="搜索项目：编码/名称/描述"
            noResultsText="没有匹配的项目"
            onChange={(value) => {
              const projectId = Number(value)
              setForm((prev) => ({ ...prev, projectId: Number.isFinite(projectId) && projectId > 0 ? projectId : 0 }))
            }}
          />
          <label htmlFor="portal-company">公司</label>
          <input id="portal-company" value={form.company} onChange={(event) => setForm((prev) => ({ ...prev, company: event.target.value }))} />
          <label htmlFor="portal-contact-name">联系人</label>
          <input id="portal-contact-name" value={form.contactName} onChange={(event) => setForm((prev) => ({ ...prev, contactName: event.target.value }))} />
          <label htmlFor="portal-contact-email">联系邮箱</label>
          <input id="portal-contact-email" type="email" value={form.contactEmail} onChange={(event) => setForm((prev) => ({ ...prev, contactEmail: event.target.value }))} />
          <label htmlFor="portal-contact-type">类型</label>
          <select id="portal-contact-type" value={form.contactType} onChange={(event) => setForm((prev) => ({ ...prev, contactType: event.target.value as 'customer' | 'supplier' }))}>
            <option value="customer">客户</option>
            <option value="supplier">供应商</option>
          </select>
          <label htmlFor="portal-expires-at">过期时间</label>
          <DateTimeQuickField inputId="portal-expires-at" value={form.expiresAt} onChange={(value) => setForm((prev) => ({ ...prev, expiresAt: value }))} />
          <label htmlFor="portal-enabled">启用</label>
          <div className="multi-checklist compact">
            <label className="multi-check-item">
              <input id="portal-enabled" type="checkbox" checked={form.isEnabled} onChange={() => setForm((prev) => ({ ...prev, isEnabled: !prev.isEnabled }))} />
              <span>允许外部通过邀请链接访问项目门户</span>
            </label>
          </div>
          <label htmlFor="portal-attachments">指定附件</label>
          <AttachmentField
            inputId="portal-attachments"
            value={form.allowedAttachments}
            disabled={!canUploadAttachment}
            onChange={(allowedAttachments) => setForm((prev) => ({ ...prev, allowedAttachments }))}
          />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存邀请'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {createdPortalLink && (
            <div className="success portal-created-link">
              <label htmlFor="created-portal-link">一次性访问链接</label>
              <input id="created-portal-link" readOnly value={createdPortalLink} onFocus={(event) => event.currentTarget.select()} />
              <button type="button" className="btn secondary" onClick={() => { void copyLink() }}>复制链接</button>
              {copySuccess && <span>{copySuccess}</span>}
            </div>
          )}
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>
    </section>
  )
}
