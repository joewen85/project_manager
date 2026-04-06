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
const TagsPage = lazy(async () => import('./pages/TagsPage').then((module) => ({ default: module.TagsPage })))
const ProjectsPage = lazy(async () => import('./pages/ProjectsPage').then((module) => ({ default: module.ProjectsPage })))
const GanttPage = lazy(async () => import('./pages/GanttPage').then((module) => ({ default: module.GanttPage })))
const RbacPage = lazy(async () => import('./pages/RbacPage').then((module) => ({ default: module.RbacPage })))
const AuditPage = lazy(async () => import('./pages/AuditPage').then((module) => ({ default: module.AuditPage })))
const TasksPage = lazy(async () => import('./pages/TasksPage').then((module) => ({ default: module.TasksPage })))
const UsersPage = lazy(async () => import('./pages/UsersPage').then((module) => ({ default: module.UsersPage })))

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
  { path: '/rbac', permission: 'rbac.read' },
  { path: '/users', permission: 'users.read' },
  { path: '/departments', permission: 'departments.read' },
  { path: '/tags', permission: 'tags.read' },
  { path: '/projects', permission: 'projects.read' },
  { path: '/gantt', permission: 'projects.read' },
  { path: '/tasks', permission: 'tasks.read' },
  { path: '/notifications', permission: 'notifications.read' },
  { path: '/audit', permission: 'audit.read' },
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

    mediaQuery.addEventListener('change', handleSystemThemeChange)
    return () => mediaQuery.removeEventListener('change', handleSystemThemeChange)
  }, [])

  return (
    <Suspense fallback={<div className="card">页面加载中...</div>}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <Guard>
              <Layout />
            </Guard>
          }
        >
          <Route index element={<PermissionGuard permission="stats.read"><DashboardPage /></PermissionGuard>} />
          <Route path="rbac" element={<PermissionGuard permission="rbac.read"><RbacPage /></PermissionGuard>} />
          <Route path="users" element={<PermissionGuard permission="users.read"><UsersPage /></PermissionGuard>} />
          <Route path="departments" element={<PermissionGuard permission="departments.read"><DepartmentsPage /></PermissionGuard>} />
          <Route path="tags" element={<PermissionGuard permission="tags.read"><TagsPage /></PermissionGuard>} />
          <Route path="projects" element={<PermissionGuard permission="projects.read"><ProjectsPage /></PermissionGuard>} />
          <Route path="gantt" element={<PermissionGuard permission="projects.read"><GanttPage /></PermissionGuard>} />
          <Route path="tasks" element={<PermissionGuard permission="tasks.read"><TasksPage /></PermissionGuard>} />
          <Route path="notifications" element={<PermissionGuard permission="notifications.read"><NotificationsPage /></PermissionGuard>} />
          <Route path="audit" element={<PermissionGuard permission="audit.read"><AuditPage /></PermissionGuard>} />
          <Route path="me" element={<PermissionGuard permission="tasks.read"><MyWorkPage /></PermissionGuard>} />
        </Route>
      </Routes>
    </Suspense>
  )
}
