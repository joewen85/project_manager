import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { SearchField } from '../components/SearchField'
import { Notification } from '../types'
import { Pagination } from '../components/Pagination'
import { formatDateTime } from '../utils/datetime'
import { usePermissions } from '../hooks/usePermissions'

export function NotificationsPage() {
  const navigate = useNavigate()
  const permissions = usePermissions()
  const canManageRBAC = hasPermission('rbac.update', permissions) || hasPermission('rbac.manage', permissions)
  const canUpdateNotifications = hasPermission('notifications.update', permissions) || hasPermission('notifications.write', permissions)
  const [items, setItems] = useState<Notification[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [isReadFilter, setIsReadFilter] = useState<'all' | 'unread' | 'read'>('all')
  const [moduleFilter, setModuleFilter] = useState<'all' | 'tasks' | 'projects'>('all')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [forbidden, setForbidden] = useState(false)
  const activeFilterCount = Number(isReadFilter !== 'all') + Number(moduleFilter !== 'all') + Number(Boolean(keyword.trim()))

  const parseReadFilter = (value: string): 'all' | 'unread' | 'read' =>
    value === 'unread' || value === 'read' ? value : 'all'

  const parseModuleFilter = (value: string): 'all' | 'tasks' | 'projects' =>
    value === 'tasks' || value === 'projects' ? value : 'all'

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const isRead = isReadFilter === 'all' ? '' : (isReadFilter === 'read' ? 'true' : 'false')
      const module = moduleFilter === 'all' ? '' : moduleFilter
      const pageData = await fetchPage<Notification>(
        '/notifications',
        { page, pageSize, isRead, module, keyword: keyword.trim() },
        { page, pageSize }
      )
      setItems(pageData.list)
      setTotal(pageData.total)
    } catch (loadError) {
      const status = (loadError as { response?: { status?: number } })?.response?.status
      if (status === 403) {
        setForbidden(true)
        setError('当前账号未分配通知权限（notifications.read），请联系管理员在 RBAC 中授权。')
      } else if (status === 404) {
        setForbidden(false)
        setError('后端尚未启用通知接口（404）。请重启后端服务并确认已升级到最新代码。')
      } else {
        setForbidden(false)
        setError(readApiError(loadError, '通知加载失败'))
      }
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [page, pageSize, isReadFilter, moduleFilter, keyword])

  useEffect(() => {
    const handler = () => {
      void load()
    }
    window.addEventListener('notifications:changed', handler as EventListener)
    return () => window.removeEventListener('notifications:changed', handler as EventListener)
  }, [page, pageSize, isReadFilter, moduleFilter, keyword])

  const markRead = async (id: number) => {
    if (!canUpdateNotifications) return
    await api.patch(`/notifications/${id}/read`)
    window.dispatchEvent(new Event('notifications:changed'))
  }

  const markAllRead = async () => {
    if (!canUpdateNotifications) return
    await api.patch('/notifications/read-all')
    window.dispatchEvent(new Event('notifications:changed'))
  }

  const openTarget = async (item: Notification) => {
    if (!item.isRead && canUpdateNotifications) {
      await markRead(item.id)
    }
    if (item.module === 'tasks' && item.targetId) {
      navigate(`/tasks?taskId=${item.targetId}&view=1`)
      return
    }
    if (item.module === 'projects' && item.targetId) {
      navigate(`/projects?projectId=${item.targetId}`)
      return
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="通知筛选"
        activeCount={activeFilterCount}
        actions={
          <>
            <button className="btn secondary" onClick={() => { void load() }}>刷新</button>
            {canUpdateNotifications && <button className="btn" onClick={() => { void markAllRead() }}>全部已读</button>}
          </>
        }
        bodyClassName="toolbar-grid"
      >
        <select value={isReadFilter} aria-label="通知筛选" onChange={(e) => { setIsReadFilter(parseReadFilter(e.target.value)); setPage(1) }}>
          <option value="all">全部通知</option>
          <option value="unread">仅未读</option>
          <option value="read">仅已读</option>
        </select>
        <select value={moduleFilter} aria-label="通知模块筛选" onChange={(e) => { setModuleFilter(parseModuleFilter(e.target.value)); setPage(1) }}>
          <option value="all">全部模块</option>
          <option value="tasks">任务模块</option>
          <option value="projects">项目模块</option>
        </select>
        <SearchField aria-label="通知关键字搜索" placeholder="搜索标题/内容" value={keyword} onChange={(value) => { setKeyword(value); setPage(1) }} />
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无通知" onRetry={() => { void load() }} />
        {forbidden && canManageRBAC && (
          <p className="inline-link-tip">
            你可直接前往 <a href="/rbac">RBAC 权限页</a> 为角色分配 `notifications.read`。
          </p>
        )}
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table">
            <thead>
              <tr><th>标题</th><th>内容</th><th>模块</th><th>时间</th><th>状态</th><th>操作</th></tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td data-label="标题">
                    <div className="table-actions">
                      <button className="btn secondary" onClick={() => { void openTarget(item) }}>查看详情</button>
                      <span>{item.title}</span>
                    </div>
                  </td>
                  <td data-label="内容">{item.content}</td>
                  <td data-label="模块">{item.module}</td>
                  <td data-label="时间">{formatDateTime(item.createdAt)}</td>
                  <td data-label="状态">{item.isRead ? '已读' : '未读'}</td>
                  <td data-label="操作">
                    <div className="table-actions">
                      {!item.isRead && canUpdateNotifications && <button className="btn secondary" onClick={() => { void markRead(item.id) }}>标记已读</button>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}
    </section>
  )
}
