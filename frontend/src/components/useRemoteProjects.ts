import { UIEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { fetchData, fetchPage, readApiError } from '../services/api'
import { Project } from '../types'

const PAGE_SIZE = 20
const SEARCH_DEBOUNCE_MS = 300
const SCROLL_THROTTLE_MS = 200

const mergeUniqueProjects = (current: Project[], incoming: Project[]) => {
  const projectMap = new Map<number, Project>()
  current.forEach((project) => projectMap.set(project.id, project))
  incoming.forEach((project) => projectMap.set(project.id, project))
  return Array.from(projectMap.values())
}

export function useRemoteProjects(selectedValues: string[]) {
  const cacheRef = useRef(new Map<string, Project>())
  const requestIdRef = useRef(0)
  const throttleAtRef = useRef(0)
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [projects, setProjects] = useState<Project[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [query])

  const loadPage = useCallback(async (targetPage: number, append: boolean) => {
    const requestId = requestIdRef.current + 1
    requestIdRef.current = requestId
    if (append) setLoadingMore(true)
    else {
      setLoading(true)
      setError('')
    }

    try {
      const response = await fetchPage<Project>(
        '/projects',
        { page: targetPage, pageSize: PAGE_SIZE, keyword: debouncedQuery },
        { page: targetPage, pageSize: PAGE_SIZE },
        { silent: true }
      )
      if (requestIdRef.current !== requestId) return
      response.list.forEach((project) => cacheRef.current.set(String(project.id), project))
      setProjects((prev) => append ? mergeUniqueProjects(prev, response.list) : response.list)
      setPage(response.page)
      setTotal(response.total)
    } catch (loadError) {
      if (requestIdRef.current !== requestId) return
      if (!append) {
        setProjects([])
        setPage(1)
        setTotal(0)
      }
      setError(readApiError(loadError, '项目加载失败'))
    } finally {
      if (requestIdRef.current === requestId) {
        setLoading(false)
        setLoadingMore(false)
      }
    }
  }, [debouncedQuery])

  useEffect(() => {
    void loadPage(1, false)
  }, [loadPage])

  useEffect(() => {
    const missingValues = selectedValues.filter((value) => value && !cacheRef.current.has(value))
    if (missingValues.length === 0) return
    let cancelled = false
    void Promise.all(
      missingValues.map(async (value) => {
        const project = await fetchData<Project>(`/projects/${Number(value)}`, undefined, { silent: true })
        return { value, project }
      })
    ).then((results) => {
      if (cancelled) return
      results.forEach(({ value, project }) => {
        if (!project?.id) return
        cacheRef.current.set(value, project)
      })
      setProjects((prev) => [...prev])
    }).catch(() => {})
    return () => {
      cancelled = true
    }
  }, [selectedValues])

  const loadMore = useCallback(() => {
    if (loading || loadingMore || projects.length >= total) return
    void loadPage(page + 1, true)
  }, [loading, loadingMore, page, projects.length, total, loadPage])

  const handleListScroll = useCallback((event: UIEvent<HTMLElement>) => {
    const now = Date.now()
    if (now - throttleAtRef.current < SCROLL_THROTTLE_MS) return
    throttleAtRef.current = now
    const target = event.currentTarget
    const distanceToBottom = target.scrollHeight - target.scrollTop - target.clientHeight
    if (distanceToBottom <= 32) {
      loadMore()
    }
  }, [loadMore])

  const selectedProjects = useMemo(() => (
    selectedValues
      .map((value) => cacheRef.current.get(value))
      .filter((project): project is Project => Boolean(project))
  ), [selectedValues, projects])

  return {
    projects,
    query,
    setQuery,
    resetQuery: () => setQuery(''),
    loading,
    loadingMore,
    error,
    total,
    hasMore: projects.length < total,
    handleListScroll,
    selectedProjects
  }
}
