import { FormEvent, useEffect, useState } from 'react'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { WebhookDelivery, WebhookDeliveryStatus, WebhookEvent, WebhookSubscription } from '../types'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

interface WebhookForm {
  id?: number
  name: string
  event: WebhookEvent
  url: string
  isEnabled: boolean
}

const eventLabel: Record<WebhookEvent, string> = {
  task_status_changed: '任务状态变更'
}

const deliveryStatusLabel: Record<WebhookDeliveryStatus, string> = {
  pending: '待投递',
  success: '成功',
  failed: '失败'
}

const initialForm: WebhookForm = {
  name: '',
  event: 'task_status_changed',
  url: '',
  isEnabled: true
}

export function WebhooksPage() {
  const permissions = usePermissions()
  const canCreateWebhook = hasPermission('webhooks.create', permissions)
  const canUpdateWebhook = hasPermission('webhooks.update', permissions)
  const canDeleteWebhook = hasPermission('webhooks.delete', permissions)
  const [items, setItems] = useState<WebhookSubscription[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [enabledFilter, setEnabledFilter] = useState('')
  const [form, setForm] = useState<WebhookForm>(initialForm)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([])
  const [deliveryLoading, setDeliveryLoading] = useState(false)
  const [deliveryError, setDeliveryError] = useState('')
  const [deliveryPage, setDeliveryPage] = useState(1)
  const [deliveryPageSize, setDeliveryPageSize] = useState(10)
  const [deliveryTotal, setDeliveryTotal] = useState(0)
  const [deliveryStatusFilter, setDeliveryStatusFilter] = useState('')
  const [deliverySubscriptionFilter, setDeliverySubscriptionFilter] = useState('')
  const [retryingId, setRetryingId] = useState<number | null>(null)
  const activeFilterCount = Number(Boolean(keyword.trim())) + Number(Boolean(enabledFilter))

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const pageData = await fetchPage<WebhookSubscription>('/webhooks', { page, pageSize, keyword, isEnabled: enabledFilter }, { page, pageSize })
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      setError(readApiError(loadError, 'Webhook 订阅加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  const loadDeliveries = async () => {
    try {
      setDeliveryLoading(true)
      setDeliveryError('')
      const pageData = await fetchPage<WebhookDelivery>(
        '/webhooks/deliveries',
        {
          page: deliveryPage,
          pageSize: deliveryPageSize,
          status: deliveryStatusFilter,
          subscriptionId: deliverySubscriptionFilter
        },
        { page: deliveryPage, pageSize: deliveryPageSize }
      )
      setDeliveries(pageData.list)
      setDeliveryTotal(pageData.total)
    } catch (loadError) {
      setDeliveryError(readApiError(loadError, 'Webhook 投递日志加载失败'))
      setDeliveries([])
      setDeliveryTotal(0)
    } finally {
      setDeliveryLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword, enabledFilter])
  useEffect(() => { void loadDeliveries() }, [deliveryPage, deliveryPageSize, deliveryStatusFilter, deliverySubscriptionFilter])

  const openCreateModal = () => {
    if (!canCreateWebhook) return
    setForm(initialForm)
    setFormError('')
    setModalOpen(true)
  }

  const openEditModal = (item: WebhookSubscription) => {
    if (!canUpdateWebhook) return
    setForm({
      id: item.id,
      name: item.name,
      event: item.event,
      url: item.url,
      isEnabled: item.isEnabled
    })
    setFormError('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateWebhook) return
    if (!form.id && !canCreateWebhook) return
    try {
      setSubmitting(true)
      setFormError('')
      const payload = {
        name: form.name.trim(),
        event: form.event,
        url: form.url.trim(),
        isEnabled: form.isEnabled
      }
      if (form.id) await api.put(`/webhooks/${form.id}`, payload)
      else await api.post('/webhooks', payload)
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存 Webhook 失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const deleteWebhook = async (item: WebhookSubscription) => {
    if (!canDeleteWebhook) return
    if (!confirm(`确认删除 Webhook「${item.name}」？`)) return
    try {
      await api.delete(`/webhooks/${item.id}`)
      await load()
      await loadDeliveries()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除 Webhook 失败'))
    }
  }

  const retryDelivery = async (item: WebhookDelivery) => {
    if (!canUpdateWebhook) return
    try {
      setRetryingId(item.id)
      setDeliveryError('')
      await api.post(`/webhooks/deliveries/${item.id}/retry`)
      await load()
      await loadDeliveries()
    } catch (retryError) {
      setDeliveryError(readApiError(retryError, '重试投递失败'))
    } finally {
      setRetryingId(null)
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="Webhook 筛选"
        activeCount={activeFilterCount}
        actions={canCreateWebhook ? <button className="btn secondary" onClick={openCreateModal}>新增 Webhook</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <SearchField
          className="toolbar-search-field"
          aria-label="Webhook 关键词搜索"
          value={keywordInput}
          placeholder="搜索名称/URL"
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
        <select aria-label="Webhook 启用状态筛选" value={enabledFilter} onChange={(event) => { setEnabledFilter(event.target.value); setPage(1) }}>
          <option value="">全部状态</option>
          <option value="true">启用</option>
          <option value="false">停用</option>
        </select>
        <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无 Webhook 订阅" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr><th>名称</th><th>事件</th><th>URL</th><th>状态</th><th>最近投递</th><th>操作</th></tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="名称"><strong>{item.name}</strong><p className="table-subtext">#{item.id}</p></td>
                <td data-label="事件">{eventLabel[item.event]}</td>
                <td data-label="URL"><span className="table-subtext">{item.url}</span></td>
                <td data-label="状态">{item.isEnabled ? '启用' : '停用'}</td>
                <td data-label="最近投递">
                  {item.lastDeliveryStatus ? deliveryStatusLabel[item.lastDeliveryStatus] : '-'}
                  <p className="table-subtext">{item.lastError || formatDateTime(item.lastDeliveredAt)}</p>
                </td>
                <td data-label="操作">
                  <div className="table-actions">
                    <button className="btn secondary" onClick={() => { setDeliverySubscriptionFilter(String(item.id)); setDeliveryPage(1) }}>查看日志</button>
                    {canUpdateWebhook && <button className="btn secondary" onClick={() => openEditModal(item)}>编辑</button>}
                    {canDeleteWebhook && <button className="btn danger" onClick={() => { void deleteWebhook(item) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>
      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <FilterPanel
        title="投递日志"
        activeCount={Number(Boolean(deliveryStatusFilter)) + Number(Boolean(deliverySubscriptionFilter))}
        actions={deliverySubscriptionFilter ? <button className="btn secondary" onClick={() => { setDeliverySubscriptionFilter(''); setDeliveryPage(1) }}>查看全部日志</button> : undefined}
        bodyClassName="toolbar-grid"
      >
        <select aria-label="投递状态筛选" value={deliveryStatusFilter} onChange={(event) => { setDeliveryStatusFilter(event.target.value); setDeliveryPage(1) }}>
          <option value="">全部状态</option>
          {(Object.keys(deliveryStatusLabel) as WebhookDeliveryStatus[]).map((status) => <option key={status} value={status}>{deliveryStatusLabel[status]}</option>)}
        </select>
      </FilterPanel>
      <div className="card">
        <DataState loading={deliveryLoading} error={deliveryError} empty={!deliveryLoading && !deliveryError && deliveries.length === 0} emptyText="暂无投递日志" onRetry={() => { void loadDeliveries() }} />
        {!deliveryLoading && !deliveryError && deliveries.length > 0 && (
          <table className="responsive-table"><thead><tr><th>ID</th><th>订阅</th><th>事件</th><th>状态</th><th>次数</th><th>响应</th><th>时间</th><th>操作</th></tr></thead><tbody>
            {deliveries.map((item) => (
              <tr key={item.id}>
                <td data-label="ID">{item.id}</td>
                <td data-label="订阅">{item.subscription?.name || item.subscriptionId}</td>
                <td data-label="事件">{eventLabel[item.event]}</td>
                <td data-label="状态">{deliveryStatusLabel[item.status]}</td>
                <td data-label="次数">{item.attempts}</td>
                <td data-label="响应">
                  {item.responseStatus || '-'}
                  <p className="table-subtext">{item.errorMessage || '-'}</p>
                </td>
                <td data-label="时间">
                  {formatDateTime(item.deliveredAt || item.createdAt)}
                  {item.nextRetryAt && <p className="table-subtext">下次可重试：{formatDateTime(item.nextRetryAt)}</p>}
                </td>
                <td data-label="操作">
                  {canUpdateWebhook && item.status !== 'success' && (
                    <button className="btn secondary" disabled={retryingId === item.id} onClick={() => { void retryDelivery(item) }}>
                      {retryingId === item.id ? '重试中...' : '重试'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>
      {!deliveryLoading && !deliveryError && deliveryTotal > 0 && <Pagination total={deliveryTotal} page={deliveryPage} pageSize={deliveryPageSize} onPageChange={setDeliveryPage} onPageSizeChange={setDeliveryPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑 Webhook' : '新增 Webhook'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="webhook-name">名称</label>
          <input id="webhook-name" value={form.name} onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))} required />
          <label htmlFor="webhook-event">事件</label>
          <select id="webhook-event" value={form.event} onChange={(event) => setForm((prev) => ({ ...prev, event: event.target.value as WebhookEvent }))}>
            {(Object.keys(eventLabel) as WebhookEvent[]).map((event) => <option key={event} value={event}>{eventLabel[event]}</option>)}
          </select>
          <label className="required-label" htmlFor="webhook-url">URL</label>
          <input id="webhook-url" type="url" value={form.url} onChange={(event) => setForm((prev) => ({ ...prev, url: event.target.value }))} required />
          <label htmlFor="webhook-enabled">启用</label>
          <div className="multi-checklist compact">
            <label className="multi-check-item">
              <input type="checkbox" checked={form.isEnabled} onChange={() => setForm((prev) => ({ ...prev, isEnabled: !prev.isEnabled }))} />
              <span>接收事件并立即投递</span>
            </label>
          </div>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting}>{submitting ? '保存中...' : '保存'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
        </form>
      </Modal>
    </section>
  )
}
