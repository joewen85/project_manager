import { ReactElement } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { DashboardPage } from './pages/DashboardPage'
import { DepartmentsPage } from './pages/DepartmentsPage'
import { LoginPage } from './pages/LoginPage'
import { MyWorkPage } from './pages/MyWorkPage'
import { ProjectsPage } from './pages/ProjectsPage'
import { RbacPage } from './pages/RbacPage'
import { AuditPage } from './pages/AuditPage'
import { TasksPage } from './pages/TasksPage'
import { UsersPage } from './pages/UsersPage'

function Guard({ children }: { children: ReactElement }) {
  const token = localStorage.getItem('token')
  return token ? children : <Navigate to="/login" replace />
}

export default function App() {
  return (
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
        <Route path="tasks" element={<TasksPage />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="me" element={<MyWorkPage />} />
      </Route>
    </Routes>
  )
}
