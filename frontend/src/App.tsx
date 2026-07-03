import { lazy, ReactElement, Suspense, useEffect, useMemo, useState } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { Layout } from './components/Layout'
import { fetchData, hasPermission, setPermissions } from './services/api'
import { usePermissions } from './hooks/usePermissions'

const DashboardPage = lazy(async () => import('./pages/DashboardPage').then((module) => ({ default: module.DashboardPage })))
const DepartmentsPage = lazy(async () => import('./pages/DepartmentsPage').then((module) => ({ default: module.DepartmentsPage })))
const LoginPage = lazy(async () => import('./pages/LoginPage').then((module) => ({ default: module.LoginPage })))
const MyWorkPage = lazy(async () => import('./pages/MyWorkPage').then((module) => ({ default: module.MyWorkPage })))
const NotificationsPage = lazy(async () => import('./pages/NotificationsPage').then((module) => ({ default: module.NotificationsPage })))
const ProjectTemplatesPage = lazy(async () => import('./pages/ProjectTemplatesPage').then((module) => ({ default: module.ProjectTemplatesPage })))
const PortalAdminPage = lazy(async () => import('./pages/PortalAdminPage').then((module) => ({ default: module.PortalAdminPage })))
const PortalPublicPage = lazy(async () => import('./pages/PortalPublicPage').then((module) => ({ default: module.PortalPublicPage })))
const ReportsPage = lazy(async () => import('./pages/ReportsPage').then((module) => ({ default: module.ReportsPage })))
const RequestsPage = lazy(async () => import('./pages/RequestsPage').then((module) => ({ default: module.RequestsPage })))
const SprintsPage = lazy(async () => import('./pages/SprintsPage').then((module) => ({ default: module.SprintsPage })))
const TagsPage = lazy(async () => import('./pages/TagsPage').then((module) => ({ default: module.TagsPage })))
const ProjectsPage = lazy(async () => import('./pages/ProjectsPage').then((module) => ({ default: module.ProjectsPage })))
const ProjectBaselinesPage = lazy(async () => import('./pages/ProjectBaselinesPage').then((module) => ({ default: module.ProjectBaselinesPage })))
const ProjectRegistersPage = lazy(async () => import('./pages/ProjectRegistersPage').then((module) => ({ default: module.ProjectRegistersPage })))
const GanttPage = lazy(async () => import('./pages/GanttPage').then((module) => ({ default: module.GanttPage })))
const RbacPage = lazy(async () => import('./pages/RbacPage').then((module) => ({ default: module.RbacPage })))
const AuditPage = lazy(async () => import('./pages/AuditPage').then((module) => ({ default: module.AuditPage })))
const ApiTokensPage = lazy(async () => import('./pages/ApiTokensPage').then((module) => ({ default: module.ApiTokensPage })))
const AssistantPage = lazy(async () => import('./pages/AssistantPage').then((module) => ({ default: module.AssistantPage })))
const AutomationRulesPage = lazy(async () => import('./pages/AutomationRulesPage').then((module) => ({ default: module.AutomationRulesPage })))
const CalendarPage = lazy(async () => import('./pages/CalendarPage').then((module) => ({ default: module.CalendarPage })))
const TasksPage = lazy(async () => import('./pages/TasksPage').then((module) => ({ default: module.TasksPage })))
const UsersPage = lazy(async () => import('./pages/UsersPage').then((module) => ({ default: module.UsersPage })))
const WebhooksPage = lazy(async () => import('./pages/WebhooksPage').then((module) => ({ default: module.WebhooksPage })))

interface ProfilePermission {
  code?: string
}

interface ProfileRole {
  permissions?: ProfilePermission[]
}

interface ProfileResponse {
  roles?: ProfileRole[]
}

function Guard({ children }: { children: ReactElement }) {
  const token = localStorage.getItem('token')
  return token ? children : <Navigate to="/login" replace />
}

function LegacyRedirect({ to }: { to: string }) {
  const location = useLocation()
  return <Navigate to={`${to}${location.search}${location.hash}`} replace />
}

const protectedRoutes = [
  { path: '/insights/dashboard', permission: 'stats.read' },
  { path: '/workbench/me', permission: 'tasks.read' },
  { path: '/workbench/notifications', permission: 'notifications.read' },
  { path: '/portfolio/projects', permission: 'projects.read' },
  { path: '/portfolio/templates', permission: 'templates.read' },
  { path: '/portfolio/gantt', permission: 'projects.read' },
  { path: '/portfolio/baselines', permission: 'baselines.read' },
  { path: '/portfolio/registers', permission: 'registers.read' },
  { path: '/delivery/tasks', permission: 'tasks.read' },
  { path: '/delivery/sprints', permission: 'sprints.read' },
  { path: '/delivery/calendar', permission: 'tasks.read' },
  { path: '/delivery/requests', permission: 'requests.read' },
  { path: '/insights/reports', permission: 'reports.read' },
  { path: '/insights/assistant', permission: 'ai.read' },
  { path: '/integrations/automations', permission: 'automations.read' },
  { path: '/integrations/webhooks', permission: 'webhooks.read' },
  { path: '/integrations/portals', permission: 'portal.read' },
  { path: '/settings/tags', permission: 'tags.read' },
  { path: '/system/rbac', permission: 'system.rbac.read' },
  { path: '/system/users', permission: 'system.users.read' },
  { path: '/system/departments', permission: 'system.departments.read' },
  { path: '/system/audit', permission: 'system.audit.read' },
  { path: '/system/api-tokens', permission: 'system.api_tokens.read' }
]

function PermissionGuard({ permission, children }: { permission: string; children: ReactElement }) {
  const location = useLocation()
  const permissions = usePermissions()
  const token = localStorage.getItem('token')
  const [checking, setChecking] = useState(() => Boolean(token) && permissions.length === 0)

  useEffect(() => {
    if (!token || permissions.length > 0) {
      setChecking(false)
      return
    }
    let cancelled = false
    setChecking(true)
    void fetchData<ProfileResponse>('/auth/profile', undefined, { silent: true })
      .then((profileData) => {
        const rolePermissions = (profileData?.roles || []).flatMap((role) => (role.permissions || []).map((permissionItem) => String(permissionItem.code || '')))
        setPermissions(rolePermissions)
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setChecking(false)
      })
    return () => {
      cancelled = true
    }
  }, [permissions.length, token])

  const fallbackPath = useMemo(
    () => protectedRoutes.find((item) => hasPermission(item.permission, permissions))?.path || '',
    [permissions]
  )

  if (checking) return <div className="card">权限校验中...</div>
  if (hasPermission(permission, permissions)) return children
  if (fallbackPath && fallbackPath !== location.pathname) return <Navigate to={fallbackPath} replace />
  return <div className="card">当前账号无访问权限：{permission}</div>
}

export default function App() {
  useEffect(() => {
    const resolveTheme = () => {
      const saved = localStorage.getItem('theme')
      if (saved === 'light' || saved === 'dark') return saved
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    }

    const applyTheme = () => {
      document.documentElement.setAttribute('data-theme', resolveTheme())
    }

    applyTheme()
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    const handleSystemThemeChange = () => {
      const saved = localStorage.getItem('theme')
      if (saved === 'light' || saved === 'dark') return
      applyTheme()
    }

    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handleSystemThemeChange)
      return () => mediaQuery.removeEventListener('change', handleSystemThemeChange)
    }

    mediaQuery.addListener(handleSystemThemeChange)
    return () => mediaQuery.removeListener(handleSystemThemeChange)
  }, [])

  return (
    <Suspense fallback={<div className="card">页面加载中...</div>}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/portal/:token" element={<PortalPublicPage />} />
        <Route
          path="/"
          element={
            <Guard>
              <Layout />
            </Guard>
          }
        >
          <Route index element={<Navigate to="/insights/dashboard" replace />} />
          <Route path="workbench/me" element={<PermissionGuard permission="tasks.read"><MyWorkPage /></PermissionGuard>} />
          <Route path="workbench/notifications" element={<PermissionGuard permission="notifications.read"><NotificationsPage /></PermissionGuard>} />
          <Route path="portfolio/projects" element={<PermissionGuard permission="projects.read"><ProjectsPage /></PermissionGuard>} />
          <Route path="portfolio/templates" element={<PermissionGuard permission="templates.read"><ProjectTemplatesPage /></PermissionGuard>} />
          <Route path="portfolio/gantt" element={<PermissionGuard permission="projects.read"><GanttPage /></PermissionGuard>} />
          <Route path="portfolio/baselines" element={<PermissionGuard permission="baselines.read"><ProjectBaselinesPage /></PermissionGuard>} />
          <Route path="portfolio/registers" element={<PermissionGuard permission="registers.read"><ProjectRegistersPage /></PermissionGuard>} />
          <Route path="delivery/tasks" element={<PermissionGuard permission="tasks.read"><TasksPage /></PermissionGuard>} />
          <Route path="delivery/sprints" element={<PermissionGuard permission="sprints.read"><SprintsPage /></PermissionGuard>} />
          <Route path="delivery/calendar" element={<PermissionGuard permission="tasks.read"><CalendarPage /></PermissionGuard>} />
          <Route path="delivery/requests" element={<PermissionGuard permission="requests.read"><RequestsPage /></PermissionGuard>} />
          <Route path="insights/dashboard" element={<PermissionGuard permission="stats.read"><DashboardPage /></PermissionGuard>} />
          <Route path="insights/reports" element={<PermissionGuard permission="reports.read"><ReportsPage /></PermissionGuard>} />
          <Route path="insights/assistant" element={<PermissionGuard permission="ai.read"><AssistantPage /></PermissionGuard>} />
          <Route path="integrations/automations" element={<PermissionGuard permission="automations.read"><AutomationRulesPage /></PermissionGuard>} />
          <Route path="integrations/webhooks" element={<PermissionGuard permission="webhooks.read"><WebhooksPage /></PermissionGuard>} />
          <Route path="integrations/portals" element={<PermissionGuard permission="portal.read"><PortalAdminPage /></PermissionGuard>} />
          <Route path="settings/tags" element={<PermissionGuard permission="tags.read"><TagsPage /></PermissionGuard>} />
          <Route path="rbac" element={<LegacyRedirect to="/system/rbac" />} />
          <Route path="users" element={<LegacyRedirect to="/system/users" />} />
          <Route path="departments" element={<LegacyRedirect to="/system/departments" />} />
          <Route path="api-tokens" element={<LegacyRedirect to="/system/api-tokens" />} />
          <Route path="audit" element={<LegacyRedirect to="/system/audit" />} />
          <Route path="tags" element={<LegacyRedirect to="/settings/tags" />} />
          <Route path="projects" element={<LegacyRedirect to="/portfolio/projects" />} />
          <Route path="project-templates" element={<LegacyRedirect to="/portfolio/templates" />} />
          <Route path="gantt" element={<LegacyRedirect to="/portfolio/gantt" />} />
          <Route path="project-baselines" element={<LegacyRedirect to="/portfolio/baselines" />} />
          <Route path="registers" element={<LegacyRedirect to="/portfolio/registers" />} />
          <Route path="tasks" element={<LegacyRedirect to="/delivery/tasks" />} />
          <Route path="sprints" element={<LegacyRedirect to="/delivery/sprints" />} />
          <Route path="calendar" element={<LegacyRedirect to="/delivery/calendar" />} />
          <Route path="requests" element={<LegacyRedirect to="/delivery/requests" />} />
          <Route path="reports" element={<LegacyRedirect to="/insights/reports" />} />
          <Route path="automation-rules" element={<LegacyRedirect to="/integrations/automations" />} />
          <Route path="webhooks" element={<LegacyRedirect to="/integrations/webhooks" />} />
          <Route path="portals" element={<LegacyRedirect to="/integrations/portals" />} />
          <Route path="assistant" element={<LegacyRedirect to="/insights/assistant" />} />
          <Route path="notifications" element={<LegacyRedirect to="/workbench/notifications" />} />
          <Route path="me" element={<LegacyRedirect to="/workbench/me" />} />
          <Route path="system/rbac" element={<PermissionGuard permission="system.rbac.read"><RbacPage /></PermissionGuard>} />
          <Route path="system/users" element={<PermissionGuard permission="system.users.read"><UsersPage /></PermissionGuard>} />
          <Route path="system/departments" element={<PermissionGuard permission="system.departments.read"><DepartmentsPage /></PermissionGuard>} />
          <Route path="system/api-tokens" element={<PermissionGuard permission="system.api_tokens.read"><ApiTokensPage /></PermissionGuard>} />
          <Route path="system/audit" element={<PermissionGuard permission="system.audit.read"><AuditPage /></PermissionGuard>} />
        </Route>
      </Routes>
    </Suspense>
  )
}
