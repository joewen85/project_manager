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

const protectedRoutes = [
  { path: '/', permission: 'stats.read' },
  { path: '/system/rbac', permission: 'system.rbac.read' },
  { path: '/system/users', permission: 'system.users.read' },
  { path: '/system/departments', permission: 'system.departments.read' },
  { path: '/system/audit', permission: 'system.audit.read' },
  { path: '/system/api-tokens', permission: 'system.api_tokens.read' },
  { path: '/tags', permission: 'tags.read' },
  { path: '/projects', permission: 'projects.read' },
  { path: '/project-templates', permission: 'templates.read' },
  { path: '/gantt', permission: 'projects.read' },
  { path: '/project-baselines', permission: 'baselines.read' },
  { path: '/registers', permission: 'registers.read' },
  { path: '/tasks', permission: 'tasks.read' },
  { path: '/sprints', permission: 'sprints.read' },
  { path: '/calendar', permission: 'tasks.read' },
  { path: '/reports', permission: 'reports.read' },
  { path: '/requests', permission: 'requests.read' },
  { path: '/automation-rules', permission: 'automations.read' },
  { path: '/webhooks', permission: 'webhooks.read' },
  { path: '/portals', permission: 'portal.read' },
  { path: '/assistant', permission: 'ai.read' },
  { path: '/notifications', permission: 'notifications.read' },
  { path: '/me', permission: 'tasks.read' }
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
          <Route index element={<PermissionGuard permission="stats.read"><DashboardPage /></PermissionGuard>} />
          <Route path="rbac" element={<Navigate to="/system/rbac" replace />} />
          <Route path="users" element={<Navigate to="/system/users" replace />} />
          <Route path="departments" element={<Navigate to="/system/departments" replace />} />
          <Route path="api-tokens" element={<Navigate to="/system/api-tokens" replace />} />
          <Route path="audit" element={<Navigate to="/system/audit" replace />} />
          <Route path="system/rbac" element={<PermissionGuard permission="system.rbac.read"><RbacPage /></PermissionGuard>} />
          <Route path="system/users" element={<PermissionGuard permission="system.users.read"><UsersPage /></PermissionGuard>} />
          <Route path="system/departments" element={<PermissionGuard permission="system.departments.read"><DepartmentsPage /></PermissionGuard>} />
          <Route path="system/api-tokens" element={<PermissionGuard permission="system.api_tokens.read"><ApiTokensPage /></PermissionGuard>} />
          <Route path="system/audit" element={<PermissionGuard permission="system.audit.read"><AuditPage /></PermissionGuard>} />
          <Route path="tags" element={<PermissionGuard permission="tags.read"><TagsPage /></PermissionGuard>} />
          <Route path="projects" element={<PermissionGuard permission="projects.read"><ProjectsPage /></PermissionGuard>} />
          <Route path="project-templates" element={<PermissionGuard permission="templates.read"><ProjectTemplatesPage /></PermissionGuard>} />
          <Route path="gantt" element={<PermissionGuard permission="projects.read"><GanttPage /></PermissionGuard>} />
          <Route path="project-baselines" element={<PermissionGuard permission="baselines.read"><ProjectBaselinesPage /></PermissionGuard>} />
          <Route path="registers" element={<PermissionGuard permission="registers.read"><ProjectRegistersPage /></PermissionGuard>} />
          <Route path="tasks" element={<PermissionGuard permission="tasks.read"><TasksPage /></PermissionGuard>} />
          <Route path="sprints" element={<PermissionGuard permission="sprints.read"><SprintsPage /></PermissionGuard>} />
          <Route path="calendar" element={<PermissionGuard permission="tasks.read"><CalendarPage /></PermissionGuard>} />
          <Route path="reports" element={<PermissionGuard permission="reports.read"><ReportsPage /></PermissionGuard>} />
          <Route path="requests" element={<PermissionGuard permission="requests.read"><RequestsPage /></PermissionGuard>} />
          <Route path="automation-rules" element={<PermissionGuard permission="automations.read"><AutomationRulesPage /></PermissionGuard>} />
          <Route path="webhooks" element={<PermissionGuard permission="webhooks.read"><WebhooksPage /></PermissionGuard>} />
          <Route path="portals" element={<PermissionGuard permission="portal.read"><PortalAdminPage /></PermissionGuard>} />
          <Route path="assistant" element={<PermissionGuard permission="ai.read"><AssistantPage /></PermissionGuard>} />
          <Route path="notifications" element={<PermissionGuard permission="notifications.read"><NotificationsPage /></PermissionGuard>} />
          <Route path="me" element={<PermissionGuard permission="tasks.read"><MyWorkPage /></PermissionGuard>} />
        </Route>
      </Routes>
    </Suspense>
  )
}
