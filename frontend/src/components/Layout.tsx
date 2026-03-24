import { BarChart3, Building2, FolderKanban, ListChecks, NotebookTabs, Shield, UserCircle2, Users } from 'lucide-react'
import { Link, Outlet } from 'react-router-dom'

const menus = [
  { to: '/', label: '统计分析', icon: BarChart3 },
  { to: '/rbac', label: 'RBAC权限', icon: Shield },
  { to: '/users', label: '用户管理', icon: Users },
  { to: '/departments', label: '部门管理', icon: Building2 },
  { to: '/projects', label: '项目列表', icon: FolderKanban },
  { to: '/tasks', label: '任务列表', icon: ListChecks },
  { to: '/audit', label: '审计日志', icon: NotebookTabs },
  { to: '/me', label: '个人工作', icon: UserCircle2 }
]

export function Layout() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <h1>Project Manager</h1>
        {menus.map((menu) => {
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
        <Outlet />
      </main>
    </div>
  )
}
