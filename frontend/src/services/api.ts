import axios from 'axios'
import type { AxiosError, AxiosRequestConfig } from 'axios'
import type { PageResult } from '../types'

declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
  }
}

const token = localStorage.getItem('token') || ''

export interface ApiErrorBody {
  code?: string
  message?: string
}

export type ApiPageResult<T> = PageResult<T>

export type QueryPrimitive = string | number | boolean | null | undefined
type PageFallback = { page: number; pageSize: number }

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

export const buildQuery = (params: Record<string, QueryPrimitive>) => {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return
    search.set(key, String(value))
  })
  const query = search.toString()
  return query ? `?${query}` : ''
}

export const toArray = <T>(data: unknown, listKey = 'list'): T[] => {
  if (Array.isArray(data)) {
    return data as T[]
  }
  if (data && typeof data === 'object') {
    const record = data as Record<string, unknown>
    const list = record[listKey]
    if (Array.isArray(list)) {
      return list as T[]
    }
  }
  return []
}

export const toPageResult = <T>(data: unknown, fallback: PageFallback): ApiPageResult<T> => {
  if (Array.isArray(data)) {
    return {
      list: data as T[],
      total: data.length,
      page: fallback.page,
      pageSize: fallback.pageSize
    }
  }

  if (data && typeof data === 'object') {
    const record = data as Record<string, unknown>
    const list = Array.isArray(record.list) ? (record.list as T[]) : []
    const total = Number(record.total)
    const page = Number(record.page)
    const pageSize = Number(record.pageSize)
    return {
      list,
      total: Number.isFinite(total) ? total : list.length,
      page: Number.isFinite(page) && page > 0 ? page : fallback.page,
      pageSize: Number.isFinite(pageSize) && pageSize > 0 ? pageSize : fallback.pageSize
    }
  }

  return {
    list: [],
    total: 0,
    page: fallback.page,
    pageSize: fallback.pageSize
  }
}

export const fetchArray = async <T>(
  path: string,
  params?: Record<string, QueryPrimitive>,
  config?: AxiosRequestConfig
) => {
  const query = params ? buildQuery(params) : ''
  const res = await api.get(`${path}${query}`, config)
  return toArray<T>(res.data)
}

export const fetchData = async <T>(
  path: string,
  params?: Record<string, QueryPrimitive>,
  config?: AxiosRequestConfig
) => {
  const query = params ? buildQuery(params) : ''
  const res = await api.get(`${path}${query}`, config)
  return res.data as T
}

export const fetchPage = async <T>(
  path: string,
  params: Record<string, QueryPrimitive>,
  fallback: PageFallback,
  config?: AxiosRequestConfig
) => {
  const query = buildQuery(params)
  const res = await api.get(`${path}${query}`, config)
  return toPageResult<T>(res.data, fallback)
}

export const readApiError = (error: unknown, fallback = '请求失败') => {
  const err = error as AxiosError<ApiErrorBody>
  return err?.response?.data?.message || err?.message || fallback
}

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status
    const message = error?.response?.data?.message || '请求失败'
    const silent = Boolean(error?.config?.silent)

    if (status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('permissions')
      window.location.href = '/login'
      return Promise.reject(error)
    }

    if (!silent) {
      console.error(message)
    }
    return Promise.reject(error)
  }
)
