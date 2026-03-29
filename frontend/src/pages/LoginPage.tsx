import { FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, setPermissions, setToken } from '../services/api'

export function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const res = await api.post('/auth/login', { username, password })
      setToken(res.data.token)
      setPermissions(res.data.permissions || [])
      navigate('/')
    } catch {
      setError('登录失败，请检查用户名和密码')
    }
  }

  return (
    <div className="login-wrap">
      <form className="card login" onSubmit={onSubmit}>
        <h2>登录项目管理系统</h2>
        <label htmlFor="username">用户名</label>
        <input id="username" value={username} onChange={(e) => setUsername(e.target.value)} />
        <label htmlFor="password">密码</label>
        <input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        <button type="submit" className="btn">登录</button>
        {error && <p className="error">{error}</p>}
      </form>
    </div>
  )
}
