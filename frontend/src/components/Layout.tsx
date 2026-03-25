import { BarChart3, Building2, FolderKanban, ListChecks, Moon, NotebookTabs, Shield, Sun, UserCircle2, Users } from 'lucide-react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
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
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const saved = localStorage.getItem('theme')
    if (saved === 'light' || saved === 'dark') return saved
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  })
  const location = useLocation()
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

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  const logout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('permissions')
    window.location.href = '/login'
  }

  const initials = (profile.name || profile.username || 'U').slice(0, 2).toUpperCase()
  const titleEntries: Array<[string, string]> = [
    ['/', '统计分析'],
    ['/rbac', 'RBAC 权限管理'],
    ['/users', '用户管理'],
    ['/departments', '部门管理'],
    ['/projects', '项目列表'],
    ['/tasks', '任务列表'],
    ['/audit', '审计日志'],
    ['/me', '个人工作']
  ]
  const currentTitle = titleEntries.find(([path]) => path === '/' ? location.pathname === '/' : location.pathname.startsWith(path))?.[1] || '项目管理系统'

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <h1 className="sidebar-title">Project Manager</h1>
        {visibleMenus.map((menu) => {
          const Icon = menu.icon
          return (
            <NavLink className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`} to={menu.to} key={menu.to} end={menu.to === '/'}>
              <Icon size={16} />
              <span>{menu.label}</span>
            </NavLink>
          )
        })}
      </aside>
      <main className="content" aria-live="polite">
        <header className="topbar card">
          <h2 className="page-title">{currentTitle}</h2>
          <div className="user-anchor">
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
