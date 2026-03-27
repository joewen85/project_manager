import dayjs from 'dayjs'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import Gantt, { GanttTask } from 'frappe-gantt'
import '../../vendor/frappe-gantt.css'
import { api, fetchArray, fetchPage, readApiError } from '../../services/api'
import { STATUS_META } from '../../constants/status'
import { Project, Task, TaskDependency } from '../../types'
import { DataState } from '../../components/DataState'

interface Props {
  initialProjectId?: number
}

type ScopeMode = 'single' | 'portfolio'
type TimelineMode = 'Day' | 'Week' | 'Month'

const timelineModes: TimelineMode[] = ['Day', 'Week', 'Month']

interface RiskItem {
  taskId: number
  taskNo: string
  title: string
  projectName: string
  slackDays: number
}

const priorityLabel: Record<string, string> = {
  high: '高',
  medium: '中',
  low: '低'
}

const priorityOrder: Record<string, number> = {
  high: 0,
  medium: 1,
  low: 2
}

const calcPlannedProgress = (task: Task) => {
  if (!task.startAt || !task.endAt) return 0
  const start = dayjs(task.startAt)
  const end = dayjs(task.endAt)
  const now = dayjs()
  if (!start.isValid() || !end.isValid() || !end.isAfter(start)) return 0
  if (now.isBefore(start)) return 0
  if (now.isAfter(end)) return 100
  const ratio = now.diff(start) / end.diff(start)
  return Math.max(0, Math.min(100, Math.round(ratio * 100)))
}

const byPriorityAndStartAt = (left: Task, right: Task) => {
  const leftPriority = priorityOrder[left.priority || 'high'] ?? 9
  const rightPriority = priorityOrder[right.priority || 'high'] ?? 9
  if (leftPriority !== rightPriority) return leftPriority - rightPriority
  const leftAt = dayjs(left.startAt).valueOf()
  const rightAt = dayjs(right.startAt).valueOf()
  return leftAt - rightAt
}

const toDependencyPayload = (dependencies: TaskDependency[] | undefined, fallbackTaskId: number) => {
  return (dependencies || []).map((dependency) => ({
    dependsOnTaskId: dependency.dependsOnTaskId,
    lagDays: Number(dependency.lagDays || 0),
    type: dependency.type || 'FS',
    taskId: dependency.taskId || fallbackTaskId
  }))
}

const formatTaskName = (task: Task) => {
  if (!task.startAt || !task.endAt) return `${task.taskNo} ${task.title}`
  const durationDays = Math.max(dayjs(task.endAt).diff(dayjs(task.startAt), 'day'), 1)
  if (durationDays <= 2) return task.taskNo
  if (durationDays <= 4) {
    const shortTitle = (task.title || '').trim()
    const compact = shortTitle.length > 8 ? `${shortTitle.slice(0, 8)}…` : shortTitle
    return compact ? `${task.taskNo} ${compact}` : task.taskNo
  }
  return `${task.taskNo} ${task.title}`
}

export function GanttModule({ initialProjectId }: Props) {
  const [scopeMode, setScopeMode] = useState<ScopeMode>('single')
  const [timelineMode, setTimelineMode] = useState<TimelineMode>('Week')
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProjectIds, setSelectedProjectIds] = useState<number[]>(initialProjectId ? [initialProjectId] : [])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [actionSuccess, setActionSuccess] = useState('')
  const [selectedTaskId, setSelectedTaskId] = useState<number>()
  const [dependencyTargetId, setDependencyTargetId] = useState<number>()
  const [draggingTaskId, setDraggingTaskId] = useState<number>()
  const [syncing, setSyncing] = useState(false)
  const ganttWrapRef = useRef<HTMLDivElement | null>(null)
  const ganttRef = useRef<Gantt | null>(null)
  const taskMapRef = useRef<Map<number, Task>>(new Map())

  const activeProjectId = selectedProjectIds[0]

  const loadProjects = useCallback(async () => {
    const page = await fetchPage<Project>('/projects', { page: 1, pageSize: 200 }, { page: 1, pageSize: 200 }, { silent: true })
    setProjects(page.list)
    if (!selectedProjectIds.length && page.list.length > 0) {
      const fallbackProjectId = initialProjectId && page.list.some((project) => project.id === initialProjectId)
        ? initialProjectId
        : page.list[0].id
      setSelectedProjectIds([fallbackProjectId])
    }
  }, [initialProjectId, selectedProjectIds.length])

  const loadGanttTasks = useCallback(async () => {
    if (scopeMode === 'single' && !activeProjectId) {
      setTasks([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const data = scopeMode === 'single'
        ? await fetchArray<Task>(`/projects/${activeProjectId}/gantt`, undefined, { silent: true })
        : await fetchArray<Task>(
          '/projects/gantt-portfolio',
          selectedProjectIds.length > 0 ? { projectIds: selectedProjectIds.join(',') } : undefined,
          { silent: true }
        )
      setTasks(data.sort(byPriorityAndStartAt))
      if (data.length > 0) {
        setSelectedTaskId((prev) => prev && data.some((task) => task.id === prev) ? prev : data[0].id)
        setDependencyTargetId((prev) => prev && data.some((task) => task.id === prev) ? prev : data[0].id)
      } else {
        setSelectedTaskId(undefined)
        setDependencyTargetId(undefined)
      }
    } catch (loadError) {
      setTasks([])
      setError(readApiError(loadError, '甘特图数据加载失败'))
    } finally {
      setLoading(false)
    }
  }, [scopeMode, activeProjectId, selectedProjectIds])

  useEffect(() => {
    void loadProjects().catch(() => {})
  }, [loadProjects])

  useEffect(() => {
    void loadGanttTasks()
  }, [loadGanttTasks])

  useEffect(() => {
    taskMapRef.current = new Map(tasks.map((task) => [task.id, task]))
  }, [tasks])

  const scheduledTasks = useMemo(
    () => tasks.filter((task) => task.startAt && task.endAt && dayjs(task.endAt).isAfter(dayjs(task.startAt))),
    [tasks]
  )
  const unscheduledCount = tasks.length - scheduledTasks.length

  const frappeTasks = useMemo<GanttTask[]>(() => {
    return scheduledTasks.map((task) => ({
      id: String(task.id),
      name: formatTaskName(task),
      start: dayjs(task.startAt).toISOString(),
      end: dayjs(task.endAt).toISOString(),
      progress: Number(task.progress || 0),
      dependencies: (task.dependencies || []).map((dependency) => String(dependency.dependsOnTaskId)).join(','),
      custom_class: `pm-gantt-bar--priority-${task.priority || 'high'}--status-${task.status}--milestone-${task.isMilestone ? '1' : '0'}`,
      description: task.description || ''
    }))
  }, [scheduledTasks])

  const selectedTask = useMemo(
    () => tasks.find((task) => task.id === selectedTaskId),
    [tasks, selectedTaskId]
  )

  const riskItems = useMemo<RiskItem[]>(() => {
    const taskMap = new Map(tasks.map((task) => [task.id, task]))
    return tasks.flatMap((task) => {
      if (!task.startAt || !Array.isArray(task.dependencies) || task.dependencies.length === 0) return []
      const startAt = dayjs(task.startAt)
      if (!startAt.isValid()) return []
      let maxDependencyEnd = startAt.subtract(30, 'day')
      task.dependencies.forEach((dependency) => {
        const predecessor = taskMap.get(dependency.dependsOnTaskId)
        if (!predecessor?.endAt) return
        const dependencyEnd = dayjs(predecessor.endAt).add(Number(dependency.lagDays || 0), 'day')
        if (dependencyEnd.isAfter(maxDependencyEnd)) maxDependencyEnd = dependencyEnd
      })
      const slackDays = startAt.diff(maxDependencyEnd, 'day')
      if (Number.isNaN(slackDays) || slackDays > 2) return []
      return [{
        taskId: task.id,
        taskNo: task.taskNo,
        title: task.title,
        projectName: task.projectName || `项目#${task.projectId}`,
        slackDays
      }]
    })
  }, [tasks])

  const overlapConflicts = useMemo(() => {
    const userTaskMap = new Map<number, { userName: string; tasks: Task[] }>()
    scheduledTasks.forEach((task) => {
      ;(task.assignees || []).forEach((assignee) => {
        const current = userTaskMap.get(assignee.id) || { userName: `${assignee.name}(${assignee.username})`, tasks: [] }
        current.tasks.push(task)
        userTaskMap.set(assignee.id, current)
      })
    })

    const conflicts: Array<{ userName: string; left: Task; right: Task }> = []
    userTaskMap.forEach((entry) => {
      const sortedTasks = [...entry.tasks].sort((left, right) => dayjs(left.startAt).valueOf() - dayjs(right.startAt).valueOf())
      for (let index = 1; index < sortedTasks.length; index += 1) {
        const previous = sortedTasks[index - 1]
        const current = sortedTasks[index]
        if (dayjs(current.startAt).isBefore(dayjs(previous.endAt))) {
          conflicts.push({ userName: entry.userName, left: previous, right: current })
        }
      }
    })
    return conflicts
  }, [scheduledTasks])

  const progressStats = useMemo(() => {
    if (scheduledTasks.length === 0) {
      return { planned: 0, actual: 0, delayedCount: 0 }
    }
    const planned = Math.round(scheduledTasks.reduce((sum, task) => sum + calcPlannedProgress(task), 0) / scheduledTasks.length)
    const actual = Math.round(scheduledTasks.reduce((sum, task) => sum + Number(task.progress || 0), 0) / scheduledTasks.length)
    const delayedCount = scheduledTasks.filter((task) => Number(task.progress || 0) + 10 < calcPlannedProgress(task)).length
    return { planned, actual, delayedCount }
  }, [scheduledTasks])

  const milestones = useMemo(() => tasks.filter((task) => task.isMilestone), [tasks])
  const milestoneDone = milestones.filter((task) => task.status === 'completed' || Number(task.progress || 0) >= 100).length
  const milestoneOverdue = milestones.filter((task) => task.endAt && dayjs(task.endAt).isBefore(dayjs()) && task.status !== 'completed').length

  const assigneeReport = useMemo(() => {
    const map = new Map<string, { total: number; completed: number }>()
    tasks.forEach((task) => {
      const assignees = task.assignees || []
      assignees.forEach((assignee) => {
        const key = `${assignee.name}(${assignee.username})`
        const current = map.get(key) || { total: 0, completed: 0 }
        current.total += 1
        if (task.status === 'completed') current.completed += 1
        map.set(key, current)
      })
    })
    return Array.from(map.entries())
      .map(([name, value]) => ({ name, ...value }))
      .sort((left, right) => right.total - left.total)
      .slice(0, 8)
  }, [tasks])

  const projectReport = useMemo(() => {
    const map = new Map<number, { name: string; total: number; completed: number; delayed: number; milestone: number }>()
    const delayedSet = new Set<number>(scheduledTasks.filter((task) => Number(task.progress || 0) + 10 < calcPlannedProgress(task)).map((task) => task.id))
    tasks.forEach((task) => {
      const row = map.get(task.projectId) || {
        name: task.projectName || `${task.projectCode || 'PROJ'} #${task.projectId}`,
        total: 0,
        completed: 0,
        delayed: 0,
        milestone: 0
      }
      row.total += 1
      if (task.status === 'completed') row.completed += 1
      if (task.isMilestone) row.milestone += 1
      if (delayedSet.has(task.id)) row.delayed += 1
      map.set(task.projectId, row)
    })
    return Array.from(map.entries()).map(([projectId, value]) => ({ projectId, ...value }))
  }, [tasks, scheduledTasks])

  const handleDateChange = useCallback((task: GanttTask, start: Date, end: Date) => {
    const taskId = Number(task.id)
    if (!Number.isFinite(taskId)) return
    setActionError('')
    setActionSuccess('')
    setSyncing(true)
    const endAt = dayjs(end).add(1, 'second').toISOString()
    void api.patch(`/tasks/${taskId}/schedule`, { startAt: start.toISOString(), endAt }, { silent: true })
      .then((response) => {
        const updatedCount = Number(response.data?.updatedCount || 0)
        setActionSuccess(`任务已改期，系统自动顺延 ${updatedCount} 个关联任务`)
        void loadGanttTasks()
      })
      .catch((updateError) => setActionError(readApiError(updateError, '任务改期失败')))
      .finally(() => setSyncing(false))
  }, [loadGanttTasks])

  const handleAutoResolve = useCallback(async () => {
    const projectIds = scopeMode === 'single'
      ? (activeProjectId ? [activeProjectId] : [])
      : Array.from(new Set(tasks.map((task) => task.projectId)))
    if (projectIds.length === 0) return
    setSyncing(true)
    setActionError('')
    setActionSuccess('')
    try {
      const results = await Promise.all(projectIds.map(async (projectId) => {
        const response = await api.post(`/projects/${projectId}/gantt/auto-resolve`, undefined, { silent: true })
        return Number(response.data?.updatedCount || 0)
      }))
      const updatedCount = results.reduce((sum, value) => sum + value, 0)
      setActionSuccess(`依赖同步完成，自动调整 ${updatedCount} 个任务`)
      await loadGanttTasks()
    } catch (resolveError) {
      setActionError(readApiError(resolveError, '依赖同步失败'))
    } finally {
      setSyncing(false)
    }
  }, [scopeMode, activeProjectId, tasks, loadGanttTasks])

  const handleDropDependency = useCallback(async (targetTaskId: number, sourceTaskId: number) => {
    if (!targetTaskId || !sourceTaskId || targetTaskId === sourceTaskId) return
    const target = taskMapRef.current.get(targetTaskId)
    if (!target) return
    const nextDependencies = toDependencyPayload(target.dependencies, targetTaskId)
    if (nextDependencies.some((dependency) => dependency.dependsOnTaskId === sourceTaskId)) return
    nextDependencies.push({
      taskId: targetTaskId,
      dependsOnTaskId: sourceTaskId,
      lagDays: 0,
      type: 'FS'
    })

    setSyncing(true)
    setActionError('')
    setActionSuccess('')
    try {
      await api.put(`/tasks/${targetTaskId}/dependencies`, { dependencies: nextDependencies }, { silent: true })
      await api.post(`/projects/${target.projectId}/gantt/auto-resolve`, undefined, { silent: true })
      setActionSuccess(`已建立依赖：任务 #${targetTaskId} 依赖 #${sourceTaskId}`)
      await loadGanttTasks()
    } catch (updateError) {
      setActionError(readApiError(updateError, '依赖设置失败'))
    } finally {
      setSyncing(false)
      setDraggingTaskId(undefined)
    }
  }, [loadGanttTasks])

  const handleRemoveDependency = useCallback(async (targetTaskId: number, dependsOnTaskId: number) => {
    const target = taskMapRef.current.get(targetTaskId)
    if (!target) return
    const nextDependencies = toDependencyPayload(target.dependencies, targetTaskId)
      .filter((dependency) => dependency.dependsOnTaskId !== dependsOnTaskId)
    setSyncing(true)
    setActionError('')
    setActionSuccess('')
    try {
      await api.put(`/tasks/${targetTaskId}/dependencies`, { dependencies: nextDependencies }, { silent: true })
      await api.post(`/projects/${target.projectId}/gantt/auto-resolve`, undefined, { silent: true })
      setActionSuccess(`已解除依赖：任务 #${targetTaskId} 不再依赖 #${dependsOnTaskId}`)
      await loadGanttTasks()
    } catch (updateError) {
      setActionError(readApiError(updateError, '依赖解除失败'))
    } finally {
      setSyncing(false)
    }
  }, [loadGanttTasks])

  useEffect(() => {
    const wrapper = ganttWrapRef.current
    if (!wrapper) return

    if (frappeTasks.length === 0) {
      wrapper.innerHTML = ''
      ganttRef.current = null
      return
    }

    wrapper.innerHTML = ''
    const instance = new Gantt(wrapper, frappeTasks, {
      view_mode: timelineMode,
      view_modes: timelineModes,
      language: 'zh',
      move_dependencies: true,
      readonly_progress: true,
      on_click: (task) => setSelectedTaskId(Number(task.id)),
      on_date_change: handleDateChange
    })
    ganttRef.current = instance
    return () => {
      wrapper.innerHTML = ''
      ganttRef.current = null
    }
  }, [frappeTasks, timelineMode, handleDateChange])

  const dependencyTarget = tasks.find((task) => task.id === dependencyTargetId)

  return (
    <section className="page-section gantt-module-page">
      <div className="card gantt-control-grid">
        <select
          aria-label="甘特范围模式"
          value={scopeMode}
          onChange={(event) => setScopeMode(event.target.value as ScopeMode)}
        >
          <option value="single">单项目甘特</option>
          <option value="portfolio">项目集甘特</option>
        </select>

        {scopeMode === 'single' && (
          <select
            aria-label="选择项目"
            value={activeProjectId || ''}
            onChange={(event) => setSelectedProjectIds(event.target.value ? [Number(event.target.value)] : [])}
          >
            {projects.map((project) => (
              <option key={project.id} value={project.id}>{project.code} - {project.name}</option>
            ))}
          </select>
        )}

        {scopeMode === 'portfolio' && (
          <div className="multi-checklist compact">
            {projects.map((project) => (
              <label key={project.id} className="multi-check-item">
                <input
                  type="checkbox"
                  checked={selectedProjectIds.includes(project.id)}
                  onChange={() => {
                    setSelectedProjectIds((prev) => prev.includes(project.id) ? prev.filter((item) => item !== project.id) : [...prev, project.id])
                  }}
                />
                <span>{project.code}</span>
              </label>
            ))}
          </div>
        )}

        <select
          aria-label="甘特时间粒度"
          value={timelineMode}
          onChange={(event) => setTimelineMode(event.target.value as TimelineMode)}
        >
          <option value="Day">按天</option>
          <option value="Week">按周</option>
          <option value="Month">按月</option>
        </select>

        <button className="btn secondary" disabled={syncing} onClick={() => { void loadGanttTasks() }}>刷新数据</button>
        <button className="btn" disabled={syncing} onClick={() => { void handleAutoResolve() }}>
          {syncing ? '处理中...' : '自动同步依赖'}
        </button>
      </div>

      {(actionError || actionSuccess) && (
        <div className="card">
          {actionError && <p className="error">{actionError}</p>}
          {actionSuccess && <p className="success">{actionSuccess}</p>}
        </div>
      )}

      <div className="cards">
        <article className="card metric-card">
          <h4>计划 vs 实际</h4>
          <p className="metric-value">{progressStats.actual}% / {progressStats.planned}%</p>
          <small>实际平均进度 / 计划平均进度</small>
        </article>
        <article className="card metric-card">
          <h4>延期风险任务</h4>
          <p className="metric-value">{progressStats.delayedCount}</p>
          <small>实际进度比计划进度落后超过 10%</small>
        </article>
        <article className="card metric-card">
          <h4>里程碑完成</h4>
          <p className="metric-value">{milestoneDone} / {milestones.length}</p>
          <small>逾期里程碑 {milestoneOverdue} 个</small>
        </article>
        <article className="card metric-card">
          <h4>资源冲突</h4>
          <p className="metric-value">{overlapConflicts.length}</p>
          <small>同执行人任务时间重叠</small>
        </article>
        <article className="card metric-card">
          <h4>依赖缓冲不足</h4>
          <p className="metric-value">{riskItems.length}</p>
          <small>依赖链缓冲小于等于 2 天</small>
        </article>
        <article className="card metric-card">
          <h4>未排期任务</h4>
          <p className="metric-value">{unscheduledCount}</p>
          <small>建议补齐开始/结束时间</small>
        </article>
      </div>

      <div className="card gantt-main-card">
        <h3>工程项目甘特图</h3>
        <DataState loading={loading} error={error} empty={!loading && !error && tasks.length === 0} emptyText="暂无甘特图任务" onRetry={() => { void loadGanttTasks() }} />
        {!loading && !error && tasks.length > 0 && <div className="pm-gantt-shell" ref={ganttWrapRef} />}
      </div>

      {selectedTask && (
        <div className="card">
          <h3>任务详情与阶段目标</h3>
          <div className="detail-columns">
            <div><strong>任务：</strong>{selectedTask.taskNo} {selectedTask.title}</div>
            <div><strong>项目：</strong>{selectedTask.projectName || selectedTask.projectCode || `#${selectedTask.projectId}`}</div>
            <div><strong>优先级：</strong>{priorityLabel[selectedTask.priority || 'high']}</div>
            <div><strong>状态：</strong>{STATUS_META[selectedTask.status].label}</div>
            <div><strong>计划进度：</strong>{calcPlannedProgress(selectedTask)}%</div>
            <div><strong>实际进度：</strong>{Number(selectedTask.progress || 0)}%</div>
            <div><strong>执行人：</strong>{(selectedTask.assignees || []).map((user) => user.name).join('、') || '-'}</div>
            <div><strong>里程碑：</strong>{selectedTask.isMilestone ? '是' : '否'}</div>
          </div>
        </div>
      )}

      <div className="card dependency-board">
        <h3>拖拽设置任务依赖</h3>
        <p className="helper-text">把“前置任务”拖到“目标任务”卡片即可建立依赖，系统会自动顺延并消解冲突。</p>

        <select
          aria-label="依赖目标任务"
          value={dependencyTargetId || ''}
          onChange={(event) => setDependencyTargetId(Number(event.target.value))}
        >
          {tasks.map((task) => (
            <option key={task.id} value={task.id}>
              {task.taskNo} {task.title}
            </option>
          ))}
        </select>

        <div
          className="dependency-target"
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault()
            if (!dependencyTargetId || !draggingTaskId) return
            void handleDropDependency(dependencyTargetId, draggingTaskId)
          }}
        >
          <strong>目标任务：</strong>
          <span>{dependencyTarget ? `${dependencyTarget.taskNo} ${dependencyTarget.title}` : '请选择目标任务'}</span>
        </div>

        <div className="dependency-source-grid">
          {tasks
            .filter((task) => task.id !== dependencyTargetId)
            .map((task) => (
              <button
                key={task.id}
                type="button"
                className="btn secondary dependency-source"
                draggable
                onDragStart={() => setDraggingTaskId(task.id)}
                onClick={() => setSelectedTaskId(task.id)}
              >
                {task.taskNo}
              </button>
            ))}
        </div>

        {dependencyTarget && (
          <div className="dependency-list">
            <strong>当前依赖：</strong>
            {(dependencyTarget.dependencies || []).length === 0 && <span className="helper-text">暂无依赖</span>}
            {(dependencyTarget.dependencies || []).map((dependency) => {
              const dependencyTask = tasks.find((task) => task.id === dependency.dependsOnTaskId)
              return (
                <button
                  type="button"
                  key={`${dependencyTarget.id}-${dependency.dependsOnTaskId}`}
                  className="btn danger"
                  onClick={() => { void handleRemoveDependency(dependencyTarget.id, dependency.dependsOnTaskId) }}
                >
                  解除 {dependencyTask ? `${dependencyTask.taskNo} ${dependencyTask.title}` : `#${dependency.dependsOnTaskId}`}
                </button>
              )
            })}
          </div>
        )}
      </div>

      <div className="card">
        <h3>多项目统筹报表</h3>
        <table>
          <thead>
            <tr>
              <th>项目</th>
              <th>任务总数</th>
              <th>完成数</th>
              <th>延期风险</th>
              <th>里程碑</th>
              <th>完成率</th>
            </tr>
          </thead>
          <tbody>
            {projectReport.map((row) => (
              <tr key={row.projectId}>
                <td>{row.name}</td>
                <td>{row.total}</td>
                <td>{row.completed}</td>
                <td>{row.delayed}</td>
                <td>{row.milestone}</td>
                <td>{row.total ? `${Math.round((row.completed / row.total) * 100)}%` : '0%'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>团队协作负载</h3>
        <table>
          <thead>
            <tr>
              <th>成员</th>
              <th>任务总数</th>
              <th>已完成</th>
              <th>完成率</th>
            </tr>
          </thead>
          <tbody>
            {assigneeReport.length === 0 && (
              <tr><td colSpan={4}>暂无执行人数据</td></tr>
            )}
            {assigneeReport.map((row) => (
              <tr key={row.name}>
                <td>{row.name}</td>
                <td>{row.total}</td>
                <td>{row.completed}</td>
                <td>{row.total ? `${Math.round((row.completed / row.total) * 100)}%` : '0%'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>冲突与缓冲明细</h3>
        <div className="gantt-risk-grid">
          <section>
            <h4>人员时间冲突</h4>
            <ul className="gantt-risk-list">
              {overlapConflicts.length === 0 && <li>暂无冲突</li>}
              {overlapConflicts.slice(0, 10).map((item) => (
                <li key={`${item.userName}-${item.left.id}-${item.right.id}`}>
                  {item.userName}：{item.left.taskNo} 与 {item.right.taskNo} 存在重叠
                </li>
              ))}
            </ul>
          </section>
          <section>
            <h4>依赖缓冲不足</h4>
            <ul className="gantt-risk-list">
              {riskItems.length === 0 && <li>缓冲健康</li>}
              {riskItems.slice(0, 10).map((item) => (
                <li key={item.taskId}>
                  {item.projectName} / {item.taskNo}（{item.title}）缓冲 {item.slackDays} 天
                </li>
              ))}
            </ul>
          </section>
        </div>
      </div>
    </section>
  )
}
