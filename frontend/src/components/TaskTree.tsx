import { ReactElement, useMemo, useState } from 'react'
import { Task, Status } from '../types'
import { STATUS_META } from '../constants/status'

interface Props {
  tasks?: Task[] | null
}

type FlatGroupMode = 'assignee' | 'status'

const renderNode = (task: Task, level = 0): ReactElement => (
  <li key={task.id} style={{ paddingLeft: level * 14 }}>
    <div className="tree-node-wrap">
      <span className="tree-node">
        {task.taskNo} - {task.title}
        <em className="status-dot" style={{ background: (STATUS_META[task.status] || STATUS_META.pending).color }}>
          {(STATUS_META[task.status] || STATUS_META.pending).label}
        </em>
      </span>
      <span className="tree-progress-line">
        <span className="tree-progress-bar" style={{ width: `${Math.max(0, Math.min(100, Number(task.progress || 0)))}%` }} />
      </span>
      <span className="tree-progress-text">{Math.max(0, Math.min(100, Number(task.progress || 0)))}%</span>
    </div>
    {task.children && task.children.length > 0 && (
      <ul>{task.children.map((child) => renderNode(child as Task, level + 1))}</ul>
    )}
  </li>
)

export function TaskTree({ tasks }: Props) {
  const safeTasks = Array.isArray(tasks) ? tasks : []
  const hasHierarchy = useMemo(
    () => safeTasks.some((task) => Array.isArray(task.children) && task.children.length > 0),
    [safeTasks]
  )

  const [groupMode, setGroupMode] = useState<FlatGroupMode>('assignee')
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})

  const groupedRows = useMemo(() => {
    if (hasHierarchy) return []
    const map = new Map<string, Task[]>()
    safeTasks.forEach((task) => {
      if (groupMode === 'status') {
        const key = task.status || 'pending'
        const list = map.get(key) || []
        list.push(task)
        map.set(key, list)
        return
      }
      const assignees = task.assignees && task.assignees.length > 0
        ? task.assignees.map((assignee) => assignee.name || assignee.username || '未知执行人')
        : ['未分配']
      assignees.forEach((name) => {
        const key = name.trim() || '未分配'
        const list = map.get(key) || []
        list.push(task)
        map.set(key, list)
      })
    })

    return Array.from(map.entries())
      .map(([key, list]) => {
        const uniqueTaskMap = new Map<number, Task>()
        list.forEach((task) => uniqueTaskMap.set(task.id, task))
        const uniqueTasks = Array.from(uniqueTaskMap.values())
        const completedCount = uniqueTasks.filter((task) => task.status === 'completed').length
        const avgProgress = uniqueTasks.length > 0
          ? Math.round(uniqueTasks.reduce((sum, task) => sum + Math.max(0, Math.min(100, Number(task.progress || 0))), 0) / uniqueTasks.length)
          : 0
        return { key, tasks: uniqueTasks, completedCount, avgProgress }
      })
      .sort((left, right) => right.tasks.length - left.tasks.length)
  }, [hasHierarchy, safeTasks, groupMode])

  const toggleGroup = (key: string) => setExpandedGroups((prev) => ({ ...prev, [key]: !prev[key] }))

  if (!hasHierarchy) {
    return (
      <div className="card">
        <h3>项目分解结构（分组视图）</h3>
        <p className="helper-text">当前项目无子任务结构，已切换为更清晰的分组展示。</p>
        <div className="flat-task-toolbar">
          <label htmlFor="flat-group-mode">分组方式</label>
          <select id="flat-group-mode" value={groupMode} onChange={(event) => setGroupMode(event.target.value as FlatGroupMode)}>
            <option value="assignee">按执行人</option>
            <option value="status">按状态</option>
          </select>
        </div>
        <div className="flat-task-groups">
          {groupedRows.length === 0 && <p className="helper-text">暂无任务</p>}
          {groupedRows.map((group, index) => {
            const isExpanded = expandedGroups[group.key] ?? index === 0
            return (
              <section key={group.key} className="flat-task-group">
                <button type="button" className="flat-task-group-header" onClick={() => toggleGroup(group.key)}>
                  <strong>{group.key}</strong>
                  <span>{group.tasks.length} 个任务</span>
                  <span>完成 {group.completedCount}</span>
                  <span>平均进度 {group.avgProgress}%</span>
                  <span>{isExpanded ? '收起' : '展开'}</span>
                </button>
                {isExpanded && (
                  <ul className="flat-task-list">
                    {group.tasks.map((task) => {
                      const progress = Math.max(0, Math.min(100, Number(task.progress || 0)))
                      const statusMeta = (STATUS_META[task.status as Status] || STATUS_META.pending)
                      return (
                        <li key={task.id} className="flat-task-item">
                          <span className="flat-task-title">{task.taskNo} - {task.title}</span>
                          <em className="status-dot" style={{ background: statusMeta.color }}>{statusMeta.label}</em>
                          <span className="tree-progress-line">
                            <span className="tree-progress-bar" style={{ width: `${progress}%` }} />
                          </span>
                          <span className="tree-progress-text">{progress}%</span>
                        </li>
                      )
                    })}
                  </ul>
                )}
              </section>
            )
          })}
        </div>
      </div>
    )
  }

  return (
    <div className="card">
      <h3>项目分解树结构</h3>
      <ul className="tree">{safeTasks.map((task) => renderNode(task))}</ul>
    </div>
  )
}
