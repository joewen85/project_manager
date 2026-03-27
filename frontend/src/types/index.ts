export type Status = 'pending' | 'queued' | 'processing' | 'completed'
export type TaskPriority = 'high' | 'medium' | 'low'

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}

export interface User {
  id: number
  username: string
  name: string
  email: string
  isActive?: boolean
  roles?: Role[]
  departments?: Department[]
}

export interface Department {
  id: number
  name: string
  description: string
  users?: User[]
}

export interface Project {
  id: number
  code: string
  name: string
  description: string
  startAt?: string
  endAt?: string
  createdAt?: string
  users?: User[]
  departments?: Department[]
  tasks?: Task[]
}

export interface Task {
  id: number
  taskNo: string
  title: string
  description: string
  status: Status
  priority?: TaskPriority
  isMilestone?: boolean
  progress: number
  startAt?: string
  endAt?: string
  projectId: number
  projectCode?: string
  projectName?: string
  parentId?: number
  children?: Task[]
  creatorId?: number
  creator?: User
  assignees?: User[]
  dependencies?: TaskDependency[]
  createdAt?: string
}

export interface TaskDependency {
  id?: number
  taskId: number
  dependsOnTaskId: number
  lagDays?: number
  type?: 'FS' | 'SS' | 'FF' | 'SF'
}

export interface Role {
  id: number
  name: string
  description: string
  permissions?: Permission[]
}

export interface Permission {
  id: number
  code: string
  name: string
  description: string
}

export interface Notification {
  id: number
  userId: number
  title: string
  content: string
  module: string
  targetId: number
  isRead: boolean
  createdAt: string
}

export interface AuditLog {
  id: number
  module: string
  action: string
  userId: number
  targetId: number
  success: boolean
  createdAt: string
}

export interface MyWorkData {
  myTasks: Task[]
  myCreated: Task[]
  myParticipate: Task[]
}
