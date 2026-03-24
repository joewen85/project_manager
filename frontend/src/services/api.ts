import axios from 'axios'

const token = localStorage.getItem('token') || ''

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1',
  timeout: 15000,
  headers: token ? { Authorization: `Bearer ${token}` } : {}
})

export const setToken = (value: string) => {
  localStorage.setItem('token', value)
  api.defaults.headers.Authorization = `Bearer ${value}`
}

export const setPermissions = (values: string[]) => {
  localStorage.setItem('permissions', JSON.stringify(values))
}

export const getPermissions = (): string[] => {
  try {
    return JSON.parse(localStorage.getItem('permissions') || '[]')
  } catch {
    return []
  }
}

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status
    const message = error?.response?.data?.message || '请求失败'
    const silent = Boolean((error?.config as any)?.silent)

    if (status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('permissions')
      window.location.href = '/login'
      return Promise.reject(error)
    }

    if (!silent) {
      window.alert(message)
    }
    return Promise.reject(error)
  }
)
