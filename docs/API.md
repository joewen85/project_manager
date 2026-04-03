# 项目管理系统 API 文档

Base URL: `http://localhost:8080/api/v1`
统一错误返回: `{ "code": "...", "message": "..." }`

## 认证
### POST `/auth/login`
- 请求体: `{ "username": "admin", "password": "admin123" }`
- 响应: `{ token, user, permissions }`

### GET `/auth/profile`
- Header: `Authorization: Bearer <token>`

## 文件上传
### POST `/uploads`
- Header: `Authorization: Bearer <token>`
- Content-Type: `multipart/form-data`
- 表单字段: `files`（支持多个；兼容 `file`）
- 可选字段: `relativePaths`（与 `files` 顺序一一对应的相对路径）
- 支持：多文件、文件夹（前端可通过 `webkitdirectory` 或拖放目录上传）
- 规则：上传文件夹时，后端会按顶层目录自动压缩为 `zip` 附件返回
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
- 权限: `tags.read`
- 响应中的每个标签包含：`taskCount`（关联任务数量）
### GET `/tags/:id`
- 权限: `tags.read`
- 响应包含：`id` `name` `taskCount`
### POST `/tags`
- 请求体: `{ name }`
- 权限: `tags.write`
### PUT `/tags/:id`
- 请求体: `{ name }`
- 权限: `tags.write`
### DELETE `/tags/:id`
- 权限: `tags.write`

## 项目管理
### GET `/projects`
- Query: `page` `pageSize` `keyword`
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
- 约束:
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
- Query: `projectId` `status` `page` `pageSize` `keyword` `sortBy` `sortOrder`
  - `sortBy=priority` 时支持优先级排序
  - `sortOrder` 支持：`high|medium|low`（默认 `high`）
### GET `/tasks/export`
- 用途: 导出当前可见任务为 CSV
- Query: `projectId` `status` `keyword`
### GET `/tasks/assignee-options`
- 用途: 任务编辑弹窗执行人选项
- Query: `keyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }] }`

### POST `/tasks`
- 请求体: `{ taskNo?, title, description, customField1?, customField2?, customField3?, status, priority, isMilestone, progress, startAt, endAt, attachments?, projectId, parentId, assigneeIds, tagIds, dependencies? }`
- 权限: `tasks.write`
- `dependencies` 格式: `[{ dependsOnTaskId, lagDays, type }]`
- 约束:
  - `creatorId` 默认使用当前登录用户
  - `taskNo` 唯一（为空自动生成）
  - `status` 支持 `pending|queued|processing|completed`
  - `priority` 支持 `high|medium|low`（默认 `high`）
  - `tagIds` 为标签 ID 数组；返回任务详情/列表时会包含 `tags`
  - `customField1~3` 为三个可选自定义长文本字段
  - 维护任务标签关联时，无需额外 `tags.write`，沿用 `tasks.write`

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

## 审计日志
### GET `/audit/logs`
- Query: `page` `pageSize` `module` `action`

## 默认种子账号
- 用户名: `admin`
- 密码: `admin123`
