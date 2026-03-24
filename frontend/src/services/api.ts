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
