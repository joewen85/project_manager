import { Status } from '../types'

export const STATUS_META: Record<Status, { label: string; color: string }> = {
  pending: { label: '待处理', color: '#94a3b8' },
  queued: { label: '排队中', color: '#6366f1' },
  processing: { label: '处理中', color: '#0ea5e9' },
  completed: { label: '已完成', color: '#22c55e' }
}

export const STATUS_ORDER: Status[] = ['pending', 'queued', 'processing', 'completed']
