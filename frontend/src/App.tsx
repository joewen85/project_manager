import { lazy, ReactElement, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'

const DashboardPage = lazy(async () => import('./pages/DashboardPage').then((module) => ({ default: module.DashboardPage })))
const DepartmentsPage = lazy(async () => import('./pages/DepartmentsPage').then((module) => ({ default: module.DepartmentsPage })))
const LoginPage = lazy(async () => import('./pages/LoginPage').then((module) => ({ default: module.LoginPage })))
const MyWorkPage = lazy(async () => import('./pages/MyWorkPage').then((module) => ({ default: module.MyWorkPage })))
const NotificationsPage = lazy(async () => import('./pages/NotificationsPage').then((module) => ({ default: module.NotificationsPage })))
const ProjectsPage = lazy(async () => import('./pages/ProjectsPage').then((module) => ({ default: module.ProjectsPage })))
const GanttPage = lazy(async () => import('./pages/GanttPage').then((module) => ({ default: module.GanttPage })))
const RbacPage = lazy(async () => import('./pages/RbacPage').then((module) => ({ default: module.RbacPage })))
const AuditPage = lazy(async () => import('./pages/AuditPage').then((module) => ({ default: module.AuditPage })))
const TasksPage = lazy(async () => import('./pages/TasksPage').then((module) => ({ default: module.TasksPage })))
const UsersPage = lazy(async () => import('./pages/UsersPage').then((module) => ({ default: module.UsersPage })))

function Guard({ children }: { children: ReactElement }) {
  const token = localStorage.getItem('token')
  return token ? children : <Navigate to="/login" replace />
}

export default function App() {
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
          <Route index element={<DashboardPage />} />
          <Route path="rbac" element={<RbacPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="departments" element={<DepartmentsPage />} />
          <Route path="projects" element={<ProjectsPage />} />
          <Route path="gantt" element={<GanttPage />} />
          <Route path="tasks" element={<TasksPage />} />
          <Route path="notifications" element={<NotificationsPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="me" element={<MyWorkPage />} />
        </Route>
      </Routes>
    </Suspense>
  )
}
