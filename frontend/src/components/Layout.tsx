import { AlertTriangle, BarChart3, Bell, Building2, CalendarDays, CalendarRange, ChevronDown, ClipboardList, FolderKanban, GitBranch, Globe2, KeyRound, ListChecks, LogOut, Menu, Moon, NotebookTabs, RefreshCcw, Settings, Shield, Sparkles, Sun, Tag, UserCircle2, Users, Webhook, Workflow, X } from 'lucide-react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, clearPermissions, fetchData, fetchPage, getPermissions, hasPermission, normalizePermissionCodes, readApiError, setPermissions } from '../services/api'
import { subscribeNotificationSocket } from '../services/notificationsSocket'
import { Modal } from './Modal'
import { Notification, Role } from '../types'

type NavIcon = typeof BarChart3

interface MenuItem {
  to: string
  label: string
  icon: NavIcon
  permission: string
}

interface MenuGroup {
  label: string
  icon: NavIcon
  children: MenuItem[]
}

type MenuEntry = MenuItem | MenuGroup

const notificationPagePath = '/workbench/notifications'

const workbenchMenus: MenuItem[] = [
  { to: '/workbench/me', label: '个人工作', icon: UserCircle2, permission: 'tasks.read' },
  { to: notificationPagePath, label: '站内通知', icon: Bell, permission: 'notifications.read' }
]

const portfolioMenus: MenuItem[] = [
  { to: '/portfolio/projects', label: '项目列表', icon: FolderKanban, permission: 'projects.read' },
  { to: '/portfolio/templates', label: '模板管理', icon: FolderKanban, permission: 'templates.read' },
  { to: '/portfolio/gantt', label: '甘特模块', icon: CalendarRange, permission: 'projects.read' },
  { to: '/portfolio/baselines', label: '基线关键路径', icon: GitBranch, permission: 'baselines.read' },
  { to: '/portfolio/registers', label: '风险问题决策', icon: AlertTriangle, permission: 'registers.read' }
]

const deliveryMenus: MenuItem[] = [
  { to: '/delivery/tasks', label: '任务列表', icon: ListChecks, permission: 'tasks.read' },
  { to: '/delivery/sprints', label: '迭代管理', icon: RefreshCcw, permission: 'sprints.read' },
  { to: '/delivery/calendar', label: '我的日程', icon: CalendarDays, permission: 'tasks.read' },
  { to: '/delivery/requests', label: '请求入口', icon: ClipboardList, permission: 'requests.read' }
]

const insightMenus: MenuItem[] = [
  { to: '/insights/dashboard', label: '项目 Dashboard', icon: BarChart3, permission: 'stats.read' },
  { to: '/insights/reports', label: '报表中心', icon: BarChart3, permission: 'reports.read' },
  { to: '/insights/assistant', label: 'AI 助理', icon: Sparkles, permission: 'ai.read' }
]

const integrationMenus: MenuItem[] = [
  { to: '/integrations/automations', label: '自动化规则', icon: Workflow, permission: 'automations.read' },
  { to: '/integrations/webhooks', label: 'Webhook订阅', icon: Webhook, permission: 'webhooks.read' },
  { to: '/integrations/portals', label: '外部门户', icon: Globe2, permission: 'portal.read' }
]

const settingsMenus: MenuItem[] = [
  { to: '/settings/tags', label: '标签管理', icon: Tag, permission: 'tags.read' }
]

const systemMenus: MenuItem[] = [
  { to: '/system/rbac', label: 'RBAC权限', icon: Shield, permission: 'system.rbac.read' },
  { to: '/system/users', label: '用户管理', icon: Users, permission: 'system.users.read' },
  { to: '/system/departments', label: '部门管理', icon: Building2, permission: 'system.departments.read' },
  { to: '/system/audit', label: '审计日志', icon: NotebookTabs, permission: 'system.audit.read' },
  { to: '/system/api-tokens', label: 'API Token', icon: KeyRound, permission: 'system.api_tokens.read' }
]

const menus: MenuEntry[] = [
  { label: '洞察分析', icon: BarChart3, children: insightMenus },
  { label: '工作台', icon: UserCircle2, children: workbenchMenus },
  { label: '项目管理', icon: FolderKanban, children: portfolioMenus },
  { label: '执行协作', icon: ListChecks, children: deliveryMenus },
  { label: '集成自动化', icon: Workflow, children: integrationMenus },
  { label: '基础配置', icon: Settings, children: settingsMenus },
  { label: '系统管理', icon: Settings, children: systemMenus }
]

const isMenuGroup = (entry: MenuEntry): entry is MenuGroup => 'children' in entry

interface ProfileResponse {
  name?: string
  username?: string
  email?: string
  roles?: Role[]
}

interface UnreadCountResponse {
  count?: number
}

interface ChangePasswordForm {
  oldPassword: string
  newPassword: string
  confirmPassword: string
}

const createEmptyChangePasswordForm = (): ChangePasswordForm => ({ oldPassword: '', newPassword: '', confirmPassword: '' })

const buildNotificationSocketUrl = () => {
  const token = localStorage.getItem('token')
  if (!token) return ''

  const baseURLRaw = String(import.meta.env.VITE_API_BASE_URL || '/api/v1').trim()
  if (!baseURLRaw) return ''

  try {
    const apiURL = /^https?:\/\//i.test(baseURLRaw)
      ? new URL(baseURLRaw)
      : new URL(baseURLRaw.startsWith('/') ? baseURLRaw : `/${baseURLRaw}`, window.location.origin)

    const basePath = apiURL.pathname.replace(/\/+$/, '')
    const wsURL = new URL(apiURL.origin)
    wsURL.pathname = `${basePath}/notifications/ws`
    wsURL.protocol = apiURL.protocol === 'https:' ? 'wss:' : 'ws:'
    wsURL.searchParams.set('token', token)
    return wsURL.toString()
  } catch {
    return ''
  }
}

export function Layout() {
  const isLegacyNotificationsApiEnabled = () => localStorage.getItem('notifications_api_enabled') !== 'false'
  const isNotificationsListApiEnabled = () => localStorage.getItem('notifications_list_api_enabled') !== 'false' && isLegacyNotificationsApiEnabled()
  const isNotificationsUnreadApiEnabled = () => localStorage.getItem('notifications_unread_api_enabled') !== 'false' && isLegacyNotificationsApiEnabled()
  const notificationProbeCooldownMs = 60000
  const shouldAttemptNotificationApi = (type: 'list' | 'unread') => {
    const enabled = type === 'list' ? isNotificationsListApiEnabled() : isNotificationsUnreadApiEnabled()
    if (enabled) return true
    const lastProbeAt = Number(localStorage.getItem(`notifications_${type}_last_probe_at`) || 0)
    if (!lastProbeAt) return true
    return Date.now() - lastProbeAt >= notificationProbeCooldownMs
  }
  const markNotificationApiFailed = (type: 'list' | 'unread') => {
    localStorage.setItem(`notifications_${type}_api_enabled`, 'false')
    localStorage.setItem(`notifications_${type}_last_probe_at`, String(Date.now()))
  }
  const markNotificationApiRecovered = (type: 'list' | 'unread') => {
    localStorage.setItem(`notifications_${type}_api_enabled`, 'true')
    localStorage.removeItem(`notifications_${type}_last_probe_at`)
  }
  const [profile, setProfile] = useState<{ name?: string; username?: string; email?: string }>({})
  const [permissions, setPermissionState] = useState<string[]>(() => getPermissions())
  const [unreadCount, setUnreadCount] = useState(0)
  const [latestNotifications, setLatestNotifications] = useState<Notification[]>([])
  const [notificationMenuOpen, setNotificationMenuOpen] = useState(false)
  const [notificationsApiReady, setNotificationsApiReady] = useState<boolean>(() => isNotificationsListApiEnabled())
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const [changePasswordOpen, setChangePasswordOpen] = useState(false)
  const [changePasswordSubmitting, setChangePasswordSubmitting] = useState(false)
  const [changePasswordError, setChangePasswordError] = useState('')
  const [changePasswordSuccess, setChangePasswordSuccess] = useState('')
  const [changePasswordForm, setChangePasswordForm] = useState<ChangePasswordForm>(createEmptyChangePasswordForm)
  const [navOpen, setNavOpen] = useState(false)
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({ '系统管理': true })
  const [isMobileNav, setIsMobileNav] = useState(() => window.matchMedia('(max-width: 1024px)').matches)
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const saved = localStorage.getItem('theme')
    if (saved === 'light' || saved === 'dark') return saved
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  })
  const location = useLocation()
  const navigate = useNavigate()
  const canAccess = useCallback((permission: string, permissionList: string[]) => {
    return hasPermission(permission, permissionList)
  }, [])
  const visibleMenus = menus
    .map((item) => {
      if (!isMenuGroup(item)) return canAccess(item.permission, permissions) ? item : null
      const children = item.children.filter((child) => canAccess(child.permission, permissions))
      return children.length > 0 ? { ...item, children } : null
    })
    .filter((item): item is MenuEntry => Boolean(item))
  const hasNotificationAccess = canAccess('notifications.read', permissions)
  const canUpdateNotifications = canAccess('notifications.update', permissions)
  const canManageRBAC = canAccess('system.rbac.update', permissions)
  const notifyAnchorRef = useRef<HTMLDivElement | null>(null)
  const notifyButtonRef = useRef<HTMLButtonElement | null>(null)
  const notifyMenuRef = useRef<HTMLDivElement | null>(null)
  const permissionsRef = useRef<string[]>(permissions)
  const groupedNotifications = useMemo(() => {
    const sorted = [...latestNotifications].sort((left, right) => {
      if (left.isRead !== right.isRead) return left.isRead ? 1 : -1
      return new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime()
    })
    const todayKey = new Date().toDateString()
    const today: Notification[] = []
    const earlier: Notification[] = []
    sorted.forEach((item) => {
      if (new Date(item.createdAt).toDateString() === todayKey) today.push(item)
      else earlier.push(item)
    })
    return { today, earlier }
  }, [latestNotifications])

  const normalizePermissions = useCallback((codes: string[]) => Array.from(new Set(codes)).sort(), [])
  const isSamePermissions = useCallback((left: string[], right: string[]) => {
    if (left.length !== right.length) return false
    for (let index = 0; index < left.length; index += 1) {
      if (left[index] !== right[index]) return false
    }
    return true
  }, [])

  useEffect(() => {
    const mediaQuery = window.matchMedia('(max-width: 1024px)')
    const handleMediaChange = (event: MediaQueryListEvent) => {
      setIsMobileNav(event.matches)
      if (!event.matches) {
        setNavOpen(false)
      }
    }

    setIsMobileNav(mediaQuery.matches)
    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handleMediaChange)
      return () => mediaQuery.removeEventListener('change', handleMediaChange)
    }

    mediaQuery.addListener(handleMediaChange)
    return () => mediaQuery.removeListener(handleMediaChange)
  }, [])

  useEffect(() => {
    setNavOpen(false)
  }, [location.pathname])

  useEffect(() => {
    document.body.classList.toggle('nav-locked', isMobileNav && navOpen)
    return () => document.body.classList.remove('nav-locked')
  }, [isMobileNav, navOpen])

  useEffect(() => {
    permissionsRef.current = permissions
  }, [permissions])

  const refreshUnreadCount = useCallback(async (permissionList: string[]) => {
    if (!canAccess('notifications.read', permissionList) || !shouldAttemptNotificationApi('unread')) {
      setUnreadCount(0)
      return
    }
    try {
      const countData = await fetchData<UnreadCountResponse>('/workbench/notifications/unread-count', undefined, { silent: true })
      setUnreadCount(Number(countData?.count || 0))
      markNotificationApiRecovered('unread')
    } catch (error) {
      const status = (error as { response?: { status?: number } })?.response?.status
      if (status === 404) {
        markNotificationApiFailed('unread')
      }
      setUnreadCount(0)
    }
  }, [canAccess])

  const refreshLatestNotifications = useCallback(async (permissionList: string[]) => {
    if (!canAccess('notifications.read', permissionList) || !shouldAttemptNotificationApi('list')) {
      setLatestNotifications([])
      setNotificationsApiReady(false)
      return
    }
    try {
      const listPage = await fetchPage<Notification>('/workbench/notifications', { page: 1, pageSize: 5 }, { page: 1, pageSize: 5 }, { silent: true })
      setLatestNotifications(listPage.list)
      markNotificationApiRecovered('list')
      setNotificationsApiReady(true)
    } catch (error) {
      const status = (error as { response?: { status?: number } })?.response?.status
      if (status === 404) {
        markNotificationApiFailed('list')
        setNotificationsApiReady(false)
      }
      setLatestNotifications([])
    }
  }, [canAccess])

  useEffect(() => {
    const refreshProfile = async () => {
      if (document.visibilityState !== 'visible') return
      const profileData = await fetchData<ProfileResponse>('/auth/profile')
      setProfile({
        name: profileData?.name,
        username: profileData?.username,
        email: profileData?.email
      })
      const rolePermissions = (profileData?.roles || []).flatMap((role) =>
        (role.permissions || []).map((permission) => String(permission.code))
      )
      const merged = normalizePermissions(normalizePermissionCodes(rolePermissions))
      const current = normalizePermissions(permissionsRef.current)
      if (!isSamePermissions(current, merged)) {
        permissionsRef.current = merged
        setPermissionState(merged)
        setPermissions(merged)
      }
    }

    void refreshProfile().catch((error) => {
      console.error(readApiError(error, '用户信息刷新失败'))
    })
    const profileTimer = window.setInterval(() => {
      void refreshProfile().catch(() => {})
    }, 60000)
    return () => {
      window.clearInterval(profileTimer)
    }
  }, [isSamePermissions, normalizePermissions])

  useEffect(() => {
    if (!hasNotificationAccess) return
    const socketURL = buildNotificationSocketUrl()
    if (!socketURL) return
    return subscribeNotificationSocket(socketURL, () => {
      window.dispatchEvent(new Event('notifications:changed'))
    })
  }, [hasNotificationAccess])

  useEffect(() => {
    const handler = () => {
      const currentPermissions = permissionsRef.current
      void refreshUnreadCount(currentPermissions)
      if (notificationMenuOpen || location.pathname.startsWith(notificationPagePath)) {
        if (!isNotificationsListApiEnabled()) return
        void refreshLatestNotifications(currentPermissions)
      }
    }
    window.addEventListener('notifications:changed', handler as EventListener)
    return () => window.removeEventListener('notifications:changed', handler as EventListener)
  }, [refreshUnreadCount, refreshLatestNotifications, notificationMenuOpen, location.pathname])

  useEffect(() => {
    if (!isNotificationsListApiEnabled()) return
    if (location.pathname.startsWith(notificationPagePath)) {
      void refreshUnreadCount(permissions)
      void refreshLatestNotifications(permissions)
    }
  }, [location.pathname, permissions, refreshUnreadCount, refreshLatestNotifications])

  useEffect(() => {
    if (!isNotificationsListApiEnabled()) return
    if (notificationMenuOpen) {
      void refreshLatestNotifications(permissions)
      void refreshUnreadCount(permissions)
    }
  }, [notificationMenuOpen, permissions, refreshLatestNotifications, refreshUnreadCount])

  const markNotificationRead = async (id: number) => {
    if (!canUpdateNotifications) return
    await api.patch(`/workbench/notifications/${id}/read`)
    await refreshUnreadCount(permissions)
    await refreshLatestNotifications(permissions)
    window.dispatchEvent(new Event('notifications:changed'))
  }

  const openNotificationTarget = async (item: Notification) => {
    if (!item.isRead && canUpdateNotifications) {
      await markNotificationRead(item.id)
    }
    if (item.module === 'tasks' && item.targetId) {
      navigate(`/delivery/tasks?taskId=${item.targetId}&view=1`)
      setNotificationMenuOpen(false)
      return
    }
    if (item.module === 'projects' && item.targetId) {
      navigate(`/portfolio/projects?projectId=${item.targetId}`)
      setNotificationMenuOpen(false)
      return
    }
    if (item.module === 'project_registers' && item.targetId) {
      navigate(`/portfolio/registers?registerId=${item.targetId}`)
      setNotificationMenuOpen(false)
      return
    }
    if (item.module === 'requests' && item.targetId) {
      navigate('/delivery/requests')
      setNotificationMenuOpen(false)
      return
    }
    navigate(notificationPagePath)
    setNotificationMenuOpen(false)
  }

  useEffect(() => {
    if (!notificationMenuOpen) return

    const handleOutsideClick = (event: MouseEvent) => {
      const target = event.target as Node
      if (!notifyAnchorRef.current?.contains(target)) {
        setNotificationMenuOpen(false)
      }
    }

    const handleEsc = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setNotificationMenuOpen(false)
        notifyButtonRef.current?.focus()
      }
    }

    const handleMenuKeydown = (event: KeyboardEvent) => {
      if (!notifyMenuRef.current) return
      const items = Array.from(
        notifyMenuRef.current.querySelectorAll<HTMLElement>('[role="menuitem"]')
      )
      if (!items.length) return
      const activeIndex = items.findIndex((item) => item === document.activeElement)
      const resolveIndex = (fallback: number) => (activeIndex >= 0 ? activeIndex : fallback)

      if (event.key === 'ArrowDown') {
        event.preventDefault()
        const nextIndex = (resolveIndex(-1) + 1) % items.length
        items[nextIndex]?.focus()
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        const prevIndex = (resolveIndex(0) - 1 + items.length) % items.length
        items[prevIndex]?.focus()
        return
      }
      if (event.key === 'Home') {
        event.preventDefault()
        items[0]?.focus()
        return
      }
      if (event.key === 'End') {
        event.preventDefault()
        items[items.length - 1]?.focus()
        return
      }
      if (event.key === 'Enter' || event.key === ' ') {
        const current = document.activeElement as HTMLElement | null
        if (current && items.includes(current)) {
          event.preventDefault()
          current.click()
        }
      }
    }

    document.addEventListener('mousedown', handleOutsideClick)
    document.addEventListener('keydown', handleEsc)
    document.addEventListener('keydown', handleMenuKeydown)
    return () => {
      document.removeEventListener('mousedown', handleOutsideClick)
      document.removeEventListener('keydown', handleEsc)
      document.removeEventListener('keydown', handleMenuKeydown)
    }
  }, [notificationMenuOpen])

  useEffect(() => {
    if (!notificationMenuOpen) return
    const menu = notifyAnchorRef.current?.querySelector('.notify-menu')
    const firstFocusable = menu?.querySelector<HTMLElement>('button, a, [tabindex]:not([tabindex="-1"])')
    firstFocusable?.focus()
  }, [notificationMenuOpen, latestNotifications])

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  const logout = () => {
    localStorage.removeItem('token')
    clearPermissions()
    window.location.href = '/login'
  }

  const openChangePasswordModal = () => {
    setUserMenuOpen(false)
    setChangePasswordError('')
    setChangePasswordSuccess('')
    setChangePasswordForm(createEmptyChangePasswordForm())
    setChangePasswordOpen(true)
  }

  const closeChangePasswordModal = () => {
    setChangePasswordOpen(false)
    setChangePasswordSubmitting(false)
    setChangePasswordError('')
    setChangePasswordSuccess('')
    setChangePasswordForm(createEmptyChangePasswordForm())
  }

  const submitChangePassword = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setChangePasswordError('')
    setChangePasswordSuccess('')

    if (changePasswordForm.newPassword.length < 6) {
      setChangePasswordError('新密码至少 6 位')
      return
    }
    if (changePasswordForm.newPassword !== changePasswordForm.confirmPassword) {
      setChangePasswordError('两次输入的新密码不一致')
      return
    }
    if (changePasswordForm.oldPassword === changePasswordForm.newPassword) {
      setChangePasswordError('新密码不能与旧密码一致')
      return
    }

    try {
      setChangePasswordSubmitting(true)
      await api.post('/auth/change-password', changePasswordForm)
      setChangePasswordSuccess('密码修改成功')
      setChangePasswordForm(createEmptyChangePasswordForm())
    } catch (error) {
      setChangePasswordError(readApiError(error, '密码修改失败'))
    } finally {
      setChangePasswordSubmitting(false)
    }
  }

  const initials = (profile.name || profile.username || 'U').slice(0, 2).toUpperCase()
  const titleEntries: Array<[string, string]> = [
    ['/', '项目 Dashboard'],
    ['/workbench/me', '个人工作'],
    ['/workbench/notifications', '站内通知'],
    ['/portfolio/projects', '项目列表'],
    ['/portfolio/templates', '模板管理'],
    ['/portfolio/baselines', '基线关键路径'],
    ['/portfolio/registers', '风险问题决策'],
    ['/portfolio/gantt', '甘特图模块'],
    ['/delivery/tasks', '任务列表'],
    ['/delivery/sprints', '迭代管理'],
    ['/delivery/calendar', '我的日程'],
    ['/delivery/requests', '请求入口'],
    ['/insights/dashboard', '项目 Dashboard'],
    ['/insights/reports', '报表中心'],
    ['/insights/assistant', 'AI 助理'],
    ['/integrations/automations', '自动化规则'],
    ['/integrations/webhooks', 'Webhook订阅'],
    ['/integrations/portals', '外部门户'],
    ['/settings/tags', '标签管理'],
    ['/system/rbac', 'RBAC 权限管理'],
    ['/system/users', '用户管理'],
    ['/system/departments', '部门管理'],
    ['/system/audit', '审计日志'],
    ['/system/api-tokens', 'API Token'],
    ['/tags', '标签管理'],
    ['/projects', '项目列表'],
    ['/project-baselines', '基线关键路径'],
    ['/registers', '风险问题决策'],
    ['/gantt', '甘特图模块'],
    ['/tasks', '任务列表'],
    ['/requests', '请求入口'],
    ['/portals', '外部门户'],
    ['/assistant', 'AI 助理'],
    ['/notifications', '站内通知'],
    ['/me', '个人工作']
  ]
  const currentTitle = titleEntries.find(([path]) => path === '/' ? location.pathname === '/' : location.pathname.startsWith(path))?.[1] || '项目管理系统'

  return (
    <div className={`app-shell${navOpen ? ' nav-open' : ''}`}>
      {isMobileNav && (
        <button
          type="button"
          className={`sidebar-backdrop${navOpen ? ' visible' : ''}`}
          aria-label="关闭侧边导航"
          aria-hidden={!navOpen}
          tabIndex={navOpen ? 0 : -1}
          onClick={() => setNavOpen(false)}
        />
      )}
      <aside className="sidebar" id="app-sidebar">
        <div className="sidebar-header">
          <h1 className="sidebar-title">Project Manager</h1>
          {isMobileNav && (
            <button
              type="button"
              className="icon-btn sidebar-close"
              aria-label="关闭导航菜单"
              onClick={() => setNavOpen(false)}
            >
              <X size={18} />
            </button>
          )}
        </div>
        <nav className="sidebar-nav" aria-label="主导航">
          {visibleMenus.map((menu) => {
            if (isMenuGroup(menu)) {
              const Icon = menu.icon
              const isExpanded = expandedGroups[menu.label] ?? true
              const isActive = menu.children.some((child) => location.pathname === child.to || location.pathname.startsWith(`${child.to}/`))
              return (
                <div className={`nav-group${isActive ? ' active' : ''}`} key={menu.label}>
                  <button
                    type="button"
                    className={`nav-group-trigger${isActive ? ' active' : ''}`}
                    aria-expanded={isExpanded}
                    onClick={() => setExpandedGroups((prev) => ({ ...prev, [menu.label]: !(prev[menu.label] ?? true) }))}
                  >
                    <Icon size={18} />
                    <span>{menu.label}</span>
                    <ChevronDown className={`nav-group-chevron${isExpanded ? ' expanded' : ''}`} size={16} aria-hidden="true" />
                  </button>
                  {isExpanded && (
                    <div className="nav-submenu" role="group" aria-label={menu.label}>
                      {menu.children.map((child) => {
                        const ChildIcon = child.icon
                        const isNotificationChild = child.to === notificationPagePath
                        return (
                          <NavLink
                            className={({ isActive: childActive }) => `nav-item nav-subitem${childActive ? ' active' : ''}`}
                            to={child.to}
                            key={child.to}
                            onClick={() => {
                              if (isMobileNav) setNavOpen(false)
                            }}
                          >
                            <ChildIcon size={16} />
                            <span>{child.label}</span>
                            {isNotificationChild && unreadCount > 0 && <em className="nav-badge">{unreadCount > 99 ? '99+' : unreadCount}</em>}
                          </NavLink>
                        )
                      })}
                    </div>
                  )}
                </div>
              )
            }
            const Icon = menu.icon
            const isNotification = menu.to === notificationPagePath
            return (
              <NavLink
                className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
                to={menu.to}
                key={menu.to}
                end={menu.to === '/'}
                onClick={() => {
                  if (isMobileNav) setNavOpen(false)
                }}
              >
                <Icon size={18} />
                <span>{menu.label}</span>
                {isNotification && unreadCount > 0 && <em className="nav-badge">{unreadCount > 99 ? '99+' : unreadCount}</em>}
              </NavLink>
            )
          })}
        </nav>
      </aside>
      <main className="content" aria-live="polite">
        <header className="topbar card">
          <div className="topbar-heading">
            {isMobileNav && (
              <button
                type="button"
                className="icon-btn nav-toggle"
                aria-label={navOpen ? '收起导航菜单' : '展开导航菜单'}
                aria-controls="app-sidebar"
                aria-expanded={navOpen}
                onClick={() => {
                  setNotificationMenuOpen(false)
                  setUserMenuOpen(false)
                  setNavOpen((prev) => !prev)
                }}
              >
                <Menu size={18} />
              </button>
            )}
            <h2 className="page-title">{currentTitle}</h2>
          </div>
          <div className="topbar-actions user-anchor">
            {hasNotificationAccess && notificationsApiReady && (
              <div className="notify-anchor" ref={notifyAnchorRef}>
                <button
                  ref={notifyButtonRef}
                  className="theme-toggle"
                  onClick={() => setNotificationMenuOpen((prev) => !prev)}
                  aria-label="通知菜单"
                  aria-haspopup="menu"
                  aria-expanded={notificationMenuOpen}
                >
                  <Bell size={16} />
                  {unreadCount > 0 && <span className="topbar-badge">{unreadCount > 99 ? '99+' : unreadCount}</span>}
                </button>
                {notificationMenuOpen && (
                  <div ref={notifyMenuRef} className="notify-menu card" role="menu" aria-label="站内通知下拉">
                    <div className="notify-header">
                      <strong>站内通知</strong>
                      <span>{unreadCount} 未读</span>
                    </div>
                    {latestNotifications.length === 0 && <p className="notify-empty">暂无通知</p>}
                    {groupedNotifications.today.length > 0 && <p className="notify-group-title">今天</p>}
                    {groupedNotifications.today.map((item) => (
                      <div key={item.id} className={`notify-item${item.isRead ? ' read' : ''}`}>
                        <button className="notify-item-main" role="menuitem" tabIndex={0} onClick={() => { void openNotificationTarget(item) }}>
                          <strong>{item.title}</strong>
                          <p>{item.content}</p>
                        </button>
                        {!item.isRead && canUpdateNotifications && <button className="btn secondary" role="menuitem" tabIndex={0} onClick={() => { void markNotificationRead(item.id) }}>已读</button>}
                      </div>
                    ))}
                    {groupedNotifications.earlier.length > 0 && <p className="notify-group-title">更早</p>}
                    {groupedNotifications.earlier.map((item) => (
                      <div key={item.id} className={`notify-item${item.isRead ? ' read' : ''}`}>
                        <button className="notify-item-main" role="menuitem" tabIndex={0} onClick={() => { void openNotificationTarget(item) }}>
                          <strong>{item.title}</strong>
                          <p>{item.content}</p>
                        </button>
                        {!item.isRead && canUpdateNotifications && <button className="btn secondary" role="menuitem" tabIndex={0} onClick={() => { void markNotificationRead(item.id) }}>已读</button>}
                      </div>
                    ))}
                    <NavLink to={notificationPagePath} role="menuitem" tabIndex={0} className="notify-more" onClick={() => setNotificationMenuOpen(false)}>
                      查看全部通知
                    </NavLink>
                  </div>
                )}
              </div>
            )}
            {!hasNotificationAccess && canManageRBAC && (
              <NavLink className="permission-hint link" to="/system/rbac" title="当前角色未分配 notifications.read 权限，点击前往 RBAC 授权">
                通知未授权
              </NavLink>
            )}
            {!hasNotificationAccess && !canManageRBAC && (
              <span className="permission-hint" title="当前角色未分配 notifications.read 权限，请联系管理员在 RBAC 中授权后重新登录">
                通知未授权
              </span>
            )}
            {hasNotificationAccess && !notificationsApiReady && (
              <span className="permission-hint" title="后端缺少通知接口，请重启到最新版本">
                通知接口未启用
              </span>
            )}
            <button className="theme-toggle" onClick={() => setTheme((prev) => (prev === 'light' ? 'dark' : 'light'))} aria-label={theme === 'light' ? '切换深色模式' : '切换浅色模式'}>
              {theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}
            </button>
            <button className="avatar-btn" onClick={() => setUserMenuOpen((prev) => !prev)} aria-label="用户菜单">
              {initials}
            </button>
            {userMenuOpen && (
              <div className="user-menu card">
                <div className="user-menu-profile">
                  <div className="avatar-btn small">{initials}</div>
                  <div>
                    <strong>{profile.username || profile.name || '当前用户'}</strong>
                    <p>{profile.email || '-'}</p>
                  </div>
                </div>
                <button className="logout-item" onClick={openChangePasswordModal}>
                  <span className="logout-item-label">
                    <KeyRound size={16} />
                    修改密码
                  </span>
                </button>
                <button className="logout-item" onClick={logout}>
                  <span className="logout-item-label">
                    <LogOut size={16} />
                    退出登录
                  </span>
                </button>
              </div>
            )}
          </div>
        </header>
        <Outlet />
        <Modal open={changePasswordOpen} title="修改密码" onClose={closeChangePasswordModal}>
          <form className="form-grid" onSubmit={submitChangePassword}>
            <label className="required-label" htmlFor="old-password">旧密码</label>
            <input
              id="old-password"
              type="password"
              autoComplete="current-password"
              value={changePasswordForm.oldPassword}
              onChange={(event) => setChangePasswordForm((prev) => ({ ...prev, oldPassword: event.target.value }))}
              required
            />
            <label className="required-label" htmlFor="new-password">新密码</label>
            <input
              id="new-password"
              type="password"
              autoComplete="new-password"
              value={changePasswordForm.newPassword}
              onChange={(event) => setChangePasswordForm((prev) => ({ ...prev, newPassword: event.target.value }))}
              minLength={6}
              required
            />
            <label className="required-label" htmlFor="confirm-password">确认密码</label>
            <input
              id="confirm-password"
              type="password"
              autoComplete="new-password"
              value={changePasswordForm.confirmPassword}
              onChange={(event) => setChangePasswordForm((prev) => ({ ...prev, confirmPassword: event.target.value }))}
              minLength={6}
              required
            />
            <div className="row-actions">
              <button type="submit" className="btn" disabled={changePasswordSubmitting}>
                {changePasswordSubmitting ? '保存中...' : '确认修改'}
              </button>
              <button type="button" className="btn secondary" onClick={closeChangePasswordModal} disabled={changePasswordSubmitting}>
                取消
              </button>
            </div>
            {changePasswordError && <p className="error">{changePasswordError}</p>}
            {changePasswordSuccess && <p className="success">{changePasswordSuccess}</p>}
          </form>
        </Modal>
      </main>
    </div>
  )
}
