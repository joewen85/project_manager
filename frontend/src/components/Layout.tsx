import { BarChart3, Building2, FolderKanban, ListChecks, NotebookTabs, Shield, UserCircle2, Users } from 'lucide-react'
import { Link, Outlet } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { getPermissions } from '../services/api'
import { api } from '../services/api'

const menus = [
  { to: '/', label: '统计分析', icon: BarChart3, permission: 'stats.read' },
  { to: '/rbac', label: 'RBAC权限', icon: Shield, permission: 'rbac.manage' },
  { to: '/users', label: '用户管理', icon: Users, permission: 'users.read' },
  { to: '/departments', label: '部门管理', icon: Building2, permission: 'departments.read' },
  { to: '/projects', label: '项目列表', icon: FolderKanban, permission: 'projects.read' },
  { to: '/tasks', label: '任务列表', icon: ListChecks, permission: 'tasks.read' },
  { to: '/audit', label: '审计日志', icon: NotebookTabs, permission: 'audit.read' },
  { to: '/me', label: '个人工作', icon: UserCircle2, permission: 'tasks.read' }
]

export function Layout() {
  const [profile, setProfile] = useState<{ name?: string; username?: string; email?: string }>({})
  const permissions = getPermissions()
  const visibleMenus = menus.filter((item) => permissions.includes(item.permission))

  useEffect(() => {
    void api.get('/auth/profile').then((res) => {
      setProfile({
        name: res.data?.name,
        username: res.data?.username,
        email: res.data?.email
      })
    })
  }, [])

  const logout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('permissions')
    window.location.href = '/login'
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <h1>Project Manager</h1>
        {visibleMenus.map((menu) => {
          const Icon = menu.icon
          return (
            <Link className="nav-item" to={menu.to} key={menu.to}>
              <Icon size={16} />
              <span>{menu.label}</span>
            </Link>
          )
        })}
      </aside>
      <main className="content" aria-live="polite">
        <header className="topbar card">
          <div>
            <strong>{profile.name || '当前用户'}</strong>
            <p>{profile.username || '-'} / {profile.email || '-'}</p>
          </div>
          <button className="btn danger" onClick={logout}>退出登录</button>
        </header>
        <Outlet />
      </main>
    </div>
  )
}
