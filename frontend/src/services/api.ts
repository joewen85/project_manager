import axios from 'axios'
import type { AxiosError, AxiosRequestConfig } from 'axios'
import type { PageResult } from '../types'
import type { UploadAttachment } from '../types'

declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
  }
}

const token = localStorage.getItem('token') || ''
const permissionsEventName = 'permissions:changed'
let runtimePermissions: string[] = []

export interface ApiErrorBody {
  code?: string
  message?: string
}

export type ApiPageResult<T> = PageResult<T>

export type QueryPrimitive = string | number | boolean | null | undefined
type PageFallback = { page: number; pageSize: number }

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 15000,
  headers: token ? { Authorization: `Bearer ${token}` } : {}
})

export const setToken = (value: string) => {
  localStorage.setItem('token', value)
  api.defaults.headers.Authorization = `Bearer ${value}`
}

const normalizePermissionCodes = (values: string[]) => {
  const next = new Set(values.map((item) => String(item || '').trim()).filter(Boolean))
  for (const code of Array.from(next)) {
    if (code.endsWith('.write')) {
      const module = code.replace(/\.write$/, '')
      next.add(`${module}.create`)
      next.add(`${module}.read`)
      next.add(`${module}.update`)
      next.add(`${module}.delete`)
    }
    if (code === 'rbac.manage') {
      next.add('rbac.create')
      next.add('rbac.read')
      next.add('rbac.update')
      next.add('rbac.delete')
    }
    if (code === 'notifications.write') {
      next.add('notifications.update')
    }
  }
  return Array.from(next).sort()
}

export const clearPermissions = () => {
  runtimePermissions = []
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(permissionsEventName, { detail: [] }))
  }
}

export const setPermissions = (values: string[]) => {
  runtimePermissions = normalizePermissionCodes(values)
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(permissionsEventName, { detail: [...runtimePermissions] }))
  }
}

export const getPermissions = (): string[] => {
  return [...runtimePermissions]
}

export const onPermissionsChange = (listener: (permissions: string[]) => void) => {
  if (typeof window === 'undefined') return () => {}
  const handler = (event: Event) => {
    const detail = (event as CustomEvent<string[]>).detail
    listener(Array.isArray(detail) ? detail : getPermissions())
  }
  window.addEventListener(permissionsEventName, handler as EventListener)
  return () => window.removeEventListener(permissionsEventName, handler as EventListener)
}

export const hasPermission = (permission: string, permissions = getPermissions()) => permissions.includes(permission)

export const hasAnyPermission = (candidates: string[], permissions = getPermissions()) => {
  for (const candidate of candidates) {
    if (permissions.includes(candidate)) return true
  }
  return false
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

export interface UploadSourceFile {
  file: File
  relativePath?: string
}

export const uploadAttachments = async (files: UploadSourceFile[]) => {
  if (files.length === 0) return []
  const formData = new FormData()
  files.forEach((entry) => {
    const relativePath = (entry.relativePath || (entry.file as File & { webkitRelativePath?: string }).webkitRelativePath || entry.file.name).replaceAll('\\', '/')
    formData.append('files', entry.file)
    formData.append('relativePaths', relativePath)
  })
  const res = await api.post<{ attachments: UploadAttachment[] }>('/uploads', formData)
  return res.data.attachments || []
}

export const uploadAttachment = async (file: File) => {
  const attachments = await uploadAttachments([{ file }])
  return attachments[0]
}

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status
    const message = error?.response?.data?.message || '请求失败'
    const silent = Boolean(error?.config?.silent)

    if (status === 401) {
      localStorage.removeItem('token')
      clearPermissions()
      window.location.href = '/login'
      return Promise.reject(error)
    }

    if (!silent) {
      console.error(message)
    }
    return Promise.reject(error)
  }
)
