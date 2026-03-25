# 项目管理系统 API 文档

Base URL: `http://localhost:8080/api/v1`
统一错误返回: `{ "code": "...", "message": "..." }`

## 认证
### POST `/auth/login`
- 请求体: `{ "username": "admin", "password": "admin123" }`
- 响应: `{ token, user, permissions }`

### GET `/auth/profile`
- Header: `Authorization: Bearer <token>`

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

## 项目管理
### GET `/projects`
- Query: `page` `pageSize` `keyword`
### GET `/projects/editor-options`
- 用途: 项目编辑弹窗选项（负责人、参与部门）
- Query: `keyword` `userKeyword` `departmentKeyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }], departments: [{ id, name }] }`
### GET `/projects/:id`
### POST `/projects`
- 请求体: `{ code, name, description, startAt, endAt, userIds, departmentIds }`
### PUT `/projects/:id`
### DELETE `/projects/:id`

### GET `/projects/:projectId/gantt`
- 甘特图任务数据

### GET `/projects/:projectId/task-tree`
- 项目分解树结构（任务树）

## 任务管理
### GET `/tasks`
- Query: `projectId` `status` `page` `pageSize` `keyword`
### GET `/tasks/assignee-options`
- 用途: 任务编辑弹窗执行人选项
- Query: `keyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }] }`

### POST `/tasks`
- 请求体: `{ taskNo?, title, description, status, progress, startAt, endAt, projectId, parentId, assigneeIds }`
- 约束:
  - `creatorId` 默认使用当前登录用户
  - `taskNo` 唯一（为空自动生成）
  - `status` 支持 `pending|queued|processing|completed`

### GET `/tasks/progress-list`
- 进度列表统计

### GET `/tasks/me`
- 个人工作:
  - `myTasks`
  - `myCreated`
  - `myParticipate`
### PUT `/tasks/:id`
### DELETE `/tasks/:id`

## 统计分析
### GET `/stats/dashboard`
- 响应: 用户数、项目数、任务数、完成率

## 审计日志
### GET `/audit/logs`
- Query: `page` `pageSize` `module` `action`

## 默认种子账号
- 用户名: `admin`
- 密码: `admin123`
