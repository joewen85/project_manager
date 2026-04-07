# 项目管理系统 API 文档

Base URL: `http://localhost:8080/api/v1`
统一错误返回: `{ "code": "...", "message": "..." }`

## 认证

### POST `/auth/login`
- 请求体: `{ "username": "admin", "password": "admin123" }`
- 响应: `{ token, user, permissions }`

### GET `/auth/profile`
- Header: `Authorization: Bearer <token>`

### POST `/auth/change-password`
- Header: `Authorization: Bearer <token>`

## 权限总览（接口级）

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/uploads` | `uploads.create` |
| GET | `/rbac/permissions` `/rbac/roles` | `rbac.read` |
| POST | `/rbac/permissions` `/rbac/roles` | `rbac.create` |
| PUT | `/rbac/permissions/:id` `/rbac/roles/:id` | `rbac.update` |
| DELETE | `/rbac/permissions/:id` `/rbac/roles/:id` | `rbac.delete` |
| GET | `/users` | `users.read` |
| POST | `/users` | `users.create` |
| PUT | `/users/:id` | `users.update` |
| DELETE | `/users/:id` | `users.delete` |
| GET | `/departments` | `departments.read` |
| POST | `/departments` | `departments.create` |
| PUT | `/departments/:id` | `departments.update` |
| DELETE | `/departments/:id` | `departments.delete` |
| GET | `/tags` `/tags/:id` | `tags.read` |
| POST | `/tags` | `tags.create` |
| PUT | `/tags/:id` | `tags.update` |
| DELETE | `/tags/:id` | `tags.delete` |
| GET | `/projects*` | `projects.read` |
| POST | `/projects` | `projects.create` |
| PUT | `/projects/:id` | `projects.update` |
| POST | `/projects/:id/gantt/auto-resolve` | `projects.update` |
| DELETE | `/projects/:id` | `projects.delete` |
| GET | `/tasks*` | `tasks.read` |
| POST | `/tasks` | `tasks.create` |
| PUT | `/tasks/:id` `/tasks/:id/dependencies` | `tasks.update` |
| PATCH | `/tasks/:id/schedule` | `tasks.update` |
| DELETE | `/tasks/:id` | `tasks.delete` |
| GET | `/stats/dashboard` | `stats.read` |
| GET | `/notifications` `/notifications/unread-count` | `notifications.read` |
| PATCH | `/notifications/:id/read` `/notifications/read-all` | `notifications.update` |
| GET | `/audit/logs` | `audit.read` |

## 文件上传

### POST `/uploads`
- Header: `Authorization: Bearer <token>`
- 权限: `uploads.create`
- Content-Type: `multipart/form-data`
- 表单字段: `files`（支持多个；兼容 `file`）
- 可选字段: `relativePaths`（与 `files` 顺序一一对应的相对路径）
- 响应: `{ attachments: [{ fileName, filePath, relativePath, fileSize, mimeType }] }`
- 存储目录: `static/uploads/YYYY/MM/DD/`

## RBAC

### GET `/rbac/permissions`
### POST `/rbac/permissions`
### PUT `/rbac/permissions/:id`
### DELETE `/rbac/permissions/:id`
### GET `/rbac/roles`
### POST `/rbac/roles`
- 请求体: `{ name, description, permissionIds: number[] }`
### PUT `/rbac/roles/:id`
### DELETE `/rbac/roles/:id`

## 用户管理

### GET `/users`
- Query: `page` `pageSize` `keyword`

### POST `/users`
- 请求体: `{ username, name, email, password, roleIds, departmentIds }`

### PUT `/users/:id`
### DELETE `/users/:id`

## 部门管理

### GET `/departments`
- Query: `page` `pageSize` `keyword`

### POST `/departments`
- 请求体: `{ name, description, userIds }`

### PUT `/departments/:id`
### DELETE `/departments/:id`

## 标签管理

### GET `/tags`
- Query: `page` `pageSize` `keyword`
- 响应中的每个标签包含：`taskCount`（关联任务数量）

### GET `/tags/:id`
- 响应包含：`id` `name` `taskCount`

### POST `/tags`
- 请求体: `{ name }`

### PUT `/tags/:id`
- 请求体: `{ name }`

### DELETE `/tags/:id`

## 项目管理

### GET `/projects`
- Query: `page` `pageSize` `keyword` `searchFields`
  - `searchFields` 支持逗号分隔：`code,name,description`

### GET `/projects/export`
- 用途: 导出当前可见项目为 CSV
- Query: `keyword`

### GET `/projects/editor-options`
- 用途: 项目编辑弹窗选项（负责人、参与部门）
- Query: `keyword` `userKeyword` `departmentKeyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }], departments: [{ id, name }] }`

### GET `/projects/:id`

### POST `/projects`
- 请求体: `{ code?, name, description, startAt, endAt, attachments?, userIds, departmentIds }`
- `code` 为空时后端自动生成随机项目编码

### PUT `/projects/:id`
### DELETE `/projects/:id`

### GET `/projects/:projectId/gantt`
- 甘特图任务数据（含优先级、里程碑、执行人、依赖）

### GET `/projects/gantt-portfolio`
- 项目集甘特数据（支持多项目统筹）
- Query: `projectIds`（可选，逗号分隔；为空则返回当前可见项目）

### POST `/projects/:projectId/gantt/auto-resolve`
- 自动同步依赖并顺延任务时间
- 响应: `{ updatedCount, projectId }`

### GET `/projects/:projectId/task-tree`
- 项目分解树结构（任务树）

## 任务管理

### GET `/tasks`
- Query: `projectId` `status` `statuses` `priorities` `assigneeIds` `searchFields` `page` `pageSize` `keyword` `sortBy` `sortOrder`
  - `statuses` 支持逗号分隔的多状态筛选
  - `priorities` 支持逗号分隔的多优先级筛选
  - `assigneeIds` 支持逗号分隔的多人筛选
  - `searchFields` 支持逗号分隔：`taskNo,title,description,projectName,priority,status,customField1,customField2,customField3`
  - `sortBy=priority` 时支持优先级排序
  - `sortOrder` 支持：`asc|desc`；当 `sortBy=priority` 时，后端兼容 `high|medium|low`

### GET `/tasks/export`
- 用途: 导出当前可见任务为 CSV
- Query: `projectId` `status` `keyword`

### GET `/tasks/assignee-options`
- 用途: 任务编辑弹窗执行人选项
- Query: `keyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }] }`

### POST `/tasks`
- 请求体: `{ taskNo?, title, description, customField1?, customField2?, customField3?, status, priority, isMilestone, progress, startAt, endAt, attachments?, projectId, parentId, assigneeIds, tagIds, dependencies? }`
- `dependencies` 格式: `[{ dependsOnTaskId, lagDays, type }]`
- `creatorId` 默认使用当前登录用户
- `taskNo` 唯一（为空自动生成）
- `status` 支持 `pending|queued|processing|completed`
- `priority` 支持 `high|medium|low`（默认 `high`）
- `tagIds` 为标签 ID 数组；返回任务详情/列表时会包含 `tags`
- `customField1~3` 为三个可选自定义长文本字段

### GET `/tasks/progress-list`
- 进度列表统计

### GET `/tasks/me`
- 个人工作:
  - `myTasks`
  - `myCreated`
  - `myParticipate`

### PUT `/tasks/:id`
### PUT `/tasks/:id/dependencies`
- 请求体: `{ dependencies: [{ dependsOnTaskId, lagDays, type }] }`

### PATCH `/tasks/:id/schedule`
- 请求体: `{ startAt, endAt }`
- Query: `autoResolve`（可选，默认 `true`）

### DELETE `/tasks/:id`

## 统计分析

### GET `/stats/dashboard`
- 响应: 用户数、项目数、任务数、完成率

## 站内通知

### GET `/notifications`
- Query: `page` `pageSize` `isRead` `module` `keyword`

### GET `/notifications/unread-count`

### PATCH `/notifications/:id/read`

### PATCH `/notifications/read-all`

### GET `/notifications/ws`
- WebSocket 实时通知通道（不再依赖前端定时轮询）
- 认证：`?token=<JWT>`（或 `Authorization: Bearer <JWT>`）
- 事件：`{"type":"notifications.updated","at":"<RFC3339Nano>"}`

## 审计日志

### GET `/audit/logs`
- Query: `page` `pageSize` `module` `action`

## 默认种子账号

- 用户名: `admin`
- 密码: `admin123`
- 默认角色策略：
  - `admin` 每次初始化自动同步为“全权限”
  - `member` 默认包含 `notifications.read`、`notifications.update`
