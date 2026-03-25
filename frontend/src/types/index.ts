export type Status = 'pending' | 'queued' | 'processing' | 'completed'

export interface User {
  id: number
  username: string
  name: string
  email: string
}

export interface Department {
  id: number
  name: string
  description: string
}

export interface Project {
  id: number
  code: string
  name: string
  description: string
  startAt?: string
  endAt?: string
}

export interface Task {
  id: number
  taskNo: string
  title: string
  description: string
  status: Status
  progress: number
  startAt?: string
  endAt?: string
  projectId: number
  parentId?: number
  children?: Task[]
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
