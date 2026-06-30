export type Status = 'pending' | 'queued' | 'processing' | 'reviewing' | 'completed'
export type TaskPriority = 'high' | 'medium' | 'low'
export type WorkRequestType = 'project' | 'task' | 'bug' | 'change'
export type WorkRequestStatus = 'submitted' | 'approved' | 'rejected' | 'converted' | 'applied'
export type AutomationTrigger = 'task_overdue' | 'task_status_changed' | 'task_progress_changed' | 'task_assignee_changed'
export type AssigneeChangeType = 'added' | 'removed'
export type AutomationExecutionStatus = 'success' | 'skipped' | 'failed'
export type SprintStatus = 'planned' | 'active' | 'closed'
export type WebhookEvent = 'task_status_changed'
export type WebhookDeliveryStatus = 'pending' | 'success' | 'failed'

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
  weeklyCapacityHours?: number
  roles?: Role[]
  departments?: Department[]
}

export interface Department {
  id: number
  name: string
  description: string
  users?: User[]
}

export interface Tag {
  id: number
  name: string
  taskCount?: number
}

export interface Project {
  id: number
  code: string
  name: string
  description: string
  startAt?: string
  endAt?: string
  attachment?: UploadAttachment
  attachments?: UploadAttachment[]
  createdAt?: string
  updatedAt?: string
  users?: User[]
  departments?: Department[]
  tasks?: Task[]
}

export interface Task {
  id: number
  taskNo: string
  title: string
  description: string
  customField1?: string
  customField2?: string
  customField3?: string
  status: Status
  priority?: TaskPriority
  isMilestone?: boolean
  progress: number
  estimatedHours?: number
  actualHours?: number
  remainingHours?: number
  startAt?: string
  endAt?: string
  attachment?: UploadAttachment
  attachments?: UploadAttachment[]
  projectId: number
  projectCode?: string
  projectName?: string
  parentId?: number
  children?: Task[]
  creatorId?: number
  creator?: User
  assignees?: User[]
  reviewers?: User[]
  tags?: Tag[]
  dependencies?: TaskDependency[]
  createdAt?: string
  updatedAt?: string
}

export interface TaskDependency {
  id?: number
  taskId: number
  dependsOnTaskId: number
  lagDays?: number
  type?: 'FS' | 'SS' | 'FF' | 'SF'
}

export interface TaskComment {
  id: number
  taskId: number
  authorId: number
  author?: User
  content: string
  attachments?: UploadAttachment[]
  mentions?: User[]
  isDeleted?: boolean
  createdAt: string
  updatedAt?: string
}

export interface TaskActivity {
  id: number
  taskId: number
  actorId?: number
  actor?: User
  type: string
  summary: string
  detail?: string
  commentId?: number
  comment?: TaskComment
  createdAt: string
  updatedAt?: string
}

export interface WorkRequest {
  id: number
  type: WorkRequestType
  title: string
  description: string
  priority: TaskPriority
  status: WorkRequestStatus
  projectId?: number
  project?: Project
  requesterId: number
  requester?: User
  reviewerId?: number
  reviewer?: User
  approvalNote?: string
  convertedTaskId?: number
  convertedTask?: Task
  targetTaskId?: number
  targetTask?: Task
  changePayload?: WorkRequestChangePayload
  appliedAt?: string
  appliedById?: number
  appliedBy?: User
  createdAt: string
  updatedAt?: string
}

export interface WorkRequestChangePayload {
  startAt?: string
  endAt?: string
  priority?: TaskPriority
  assigneeIds?: number[]
  scopeDescription?: string
}

export interface TemplateTaskDependency {
  dependsOnKey: string
  lagDays?: number
  type?: 'FS' | 'SS' | 'FF' | 'SF'
}

export interface TemplateTask {
  key: string
  title: string
  description?: string
  priority?: TaskPriority
  isMilestone?: boolean
  relativeStartDay?: number
  durationDays?: number
  dependencies?: TemplateTaskDependency[]
  children?: TemplateTask[]
}

export interface ProjectTemplate {
  id: number
  name: string
  description: string
  taskTree: TemplateTask[]
  createdAt?: string
  updatedAt?: string
}

export interface AutomationConditions {
  overdueDays: number
  projectIds?: number[]
  fromStatuses?: Status[]
  toStatuses?: Status[]
  fromProgressMin?: number
  fromProgressMax?: number
  toProgressMin?: number
  toProgressMax?: number
  assigneeChangeTypes?: AssigneeChangeType[]
}

export interface AutomationActions {
  notifyAssignees: boolean
  notifyProjectOwners: boolean
  addComment?: boolean
  commentContent?: string
  addTags?: boolean
  tagIds?: number[]
  assignAssignees?: boolean
  assigneeIds?: number[]
  callWebhook?: boolean
  webhookUrl?: string
}

export interface AutomationRule {
  id: number
  name: string
  trigger: AutomationTrigger
  isEnabled: boolean
  conditions: AutomationConditions
  actions: AutomationActions
  lastRunAt?: string
  createdById?: number
  createdBy?: User
  createdAt?: string
  updatedAt?: string
}

export interface AutomationExecutionLog {
  id: number
  ruleId: number
  rule?: AutomationRule
  trigger: AutomationTrigger
  status: AutomationExecutionStatus
  matchedCount: number
  actionCount: number
  message: string
  actorId?: number
  runSource: 'manual' | 'scheduled' | 'event'
  createdAt: string
  updatedAt?: string
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

export interface ApiToken {
  id: number
  name: string
  description: string
  tokenPrefix: string
  tokenLastFour: string
  permissionCodes: string[]
  isEnabled: boolean
  expiresAt?: string
  lastUsedAt?: string
  lastUsedIp?: string
  revokedAt?: string
  createdById: number
  serviceAccountId: number
  serviceAccount?: User
  createdAt: string
  updatedAt: string
}

export interface ApiTokenCreateResponse extends ApiToken {
  token: string
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

export interface TaskCalendarItem {
  id: number
  taskNo: string
  title: string
  status: Status
  priority?: TaskPriority
  isMilestone?: boolean
  progress: number
  startAt?: string
  endAt?: string
  projectId: number
  projectCode?: string
  projectName?: string
  assignees?: User[]
  reviewers?: User[]
  tags?: Tag[]
}

export interface TaskCalendarResponse {
  start?: string
  end?: string
  items?: TaskCalendarItem[]
}

export type SavedReportType = 'project_health' | 'member_workload' | 'task_status'

export interface SavedReportFilters {
  projectId?: number
  keyword?: string
  statuses?: Status[]
}

export interface SavedReportChartConfig {
  displayMode?: string
}

export interface SavedReport {
  id: number
  name: string
  description?: string
  type: SavedReportType
  filters?: SavedReportFilters
  chartConfig?: SavedReportChartConfig
  createdById: number
  createdBy?: User
  createdAt?: string
  updatedAt?: string
}

export interface Sprint {
  id: number
  name: string
  goal?: string
  status: SprintStatus
  startAt?: string
  endAt?: string
  capacityHours?: number
  createdById: number
  createdBy?: User
  taskCount?: number
  completedTaskCount?: number
  completionRate?: number
  tasks?: Task[]
  createdAt?: string
  updatedAt?: string
}

export interface WebhookSubscription {
  id: number
  name: string
  event: WebhookEvent
  url: string
  isEnabled: boolean
  lastDeliveryStatus?: WebhookDeliveryStatus
  lastDeliveredAt?: string
  lastError?: string
  createdById: number
  createdBy?: User
  createdAt?: string
  updatedAt?: string
}

export interface WebhookDelivery {
  id: number
  subscriptionId: number
  subscription?: WebhookSubscription
  event: WebhookEvent
  status: WebhookDeliveryStatus
  attempts: number
  payload?: string
  responseStatus?: number
  errorMessage?: string
  nextRetryAt?: string
  deliveredAt?: string
  createdAt?: string
  updatedAt?: string
}

export interface ProjectBaselineTaskSnapshot {
  taskId: number
  taskNo: string
  title: string
  status: Status
  progress: number
  isMilestone: boolean
  startAt?: string
  endAt?: string
  parentId?: number
}

export interface ProjectBaseline {
  id: number
  projectId: number
  project?: Project
  name: string
  description?: string
  taskCount: number
  completedTaskCount: number
  plannedStartAt?: string
  plannedEndAt?: string
  snapshot?: ProjectBaselineTaskSnapshot[]
  createdById: number
  createdBy?: User
  createdAt?: string
  updatedAt?: string
}

export interface ProjectBaselineTaskVariance {
  taskId: number
  taskNo: string
  title: string
  baselineStartAt?: string
  baselineEndAt?: string
  currentStartAt?: string
  currentEndAt?: string
  startVarianceDays: number
  endVarianceDays: number
  statusChanged: boolean
  progressChanged: boolean
  missingCurrentTask: boolean
}

export interface ProjectBaselineCompare {
  baselineTaskCount: number
  currentTaskCount: number
  baselineCompletedCount: number
  currentCompletedCount: number
  baselinePlannedEndAt?: string
  currentPlannedEndAt?: string
  endVarianceDays: number
  delayedTaskCount: number
  missingTaskCount: number
  changedTasks: ProjectBaselineTaskVariance[]
}

export interface ProjectBaselineDetail extends ProjectBaseline {
  compare: ProjectBaselineCompare
}

export interface CriticalPathTask {
  id: number
  taskNo: string
  title: string
  status: Status
  progress: number
  priority?: TaskPriority
  isMilestone: boolean
  startAt?: string
  endAt?: string
  durationDays: number
}

export interface CriticalPathResult {
  projectId: number
  projectEndAt?: string
  totalDurationDays: number
  criticalTaskIds: number[]
  tasks: CriticalPathTask[]
  hasCycle: boolean
}

export type ProjectHealthLevel = 'green' | 'yellow' | 'red'

export interface ProjectHealth {
  projectId: number
  projectCode: string
  projectName: string
  health: ProjectHealthLevel
  score: number
  completionRate: number
  totalTasks: number
  completedTasks: number
  overdueTasks: number
  milestoneOverdue: number
  unscheduledTasks: number
  reviewingTasks: number
  criticalOverdueTasks?: number
  reasons: string[]
}

export interface UploadAttachment {
  fileName: string
  filePath: string
  relativePath?: string
  fileSize: number
  mimeType: string
}

export const emptyUploadAttachment = (): UploadAttachment => ({
  fileName: '',
  filePath: '',
  relativePath: '',
  fileSize: 0,
  mimeType: ''
})

export const emptyUploadAttachments = (): UploadAttachment[] => []
