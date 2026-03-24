import { BarChart3, Building2, FolderKanban, ListChecks, NotebookTabs, Shield, UserCircle2, Users } from 'lucide-react'
import { Link, Outlet } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { getPermissions, setPermissions } from '../services/api'
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
  const [permissions, setPermissionState] = useState<string[]>(getPermissions())
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const visibleMenus = menus.filter((item) => permissions.includes(item.permission))

  useEffect(() => {
    const expandPermissions = (codes: string[]) => {
      const next = new Set(codes)
      for (const code of codes) {
        if (code.endsWith('.write')) {
          next.add(code.replace(/\\.write$/, '.read'))
        }
      }
      return Array.from(next)
    }

    const refreshProfile = async () => {
      const res = await api.get('/auth/profile')
      setProfile({
        name: res.data?.name,
        username: res.data?.username,
        email: res.data?.email
      })
      const rolePermissions = (res.data?.roles || []).flatMap((role: any) =>
        (role.permissions || []).map((permission: any) => String(permission.code))
      )
      const merged = expandPermissions(rolePermissions)
      setPermissionState(merged)
      setPermissions(merged)
    }

    void refreshProfile()
    const timer = window.setInterval(() => {
      void refreshProfile()
    }, 15000)
    return () => window.clearInterval(timer)
  }, [])

  const logout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('permissions')
    window.location.href = '/login'
  }

  const initials = (profile.name || profile.username || 'U').slice(0, 2).toUpperCase()

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
          <div className="user-anchor">
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
                <button className="logout-item" onClick={logout}>↪ 退出登录</button>
              </div>
            )}
          </div>
        </header>
        <Outlet />
      </main>
    </div>
  )
}
