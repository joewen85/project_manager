# 项目管理系统 API 文档

Base URL: `http://localhost:8080/api/v1`
统一错误返回: `{ "code": "...", "message": "..." }`

## 认证

受保护接口统一使用 `Authorization: Bearer <token>`。`token` 可以是登录接口返回的 JWT，也可以是 API Token 模块生成的 `pmt_...` 服务账号 Token。

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
| GET | `/projects` `/projects/:id` `/projects/export` `/projects/editor-options` `/projects/:id/gantt` `/projects/gantt-portfolio` `/projects/:id/task-tree` | `projects.read` |
| POST | `/projects` | `projects.create` |
| PUT | `/projects/:id` | `projects.update` |
| POST | `/projects/:id/gantt/auto-resolve` | `projects.update` |
| DELETE | `/projects/:id` | `projects.delete` |
| GET | `/projects/:id/critical-path` `/project-baselines` `/project-baselines/:id` | `baselines.read` |
| POST | `/project-baselines` | `baselines.create` |
| DELETE | `/project-baselines/:id` | `baselines.delete` |
| GET | `/project-registers` `/project-registers/:id` `/project-registers/:id/activities` | `registers.read` |
| POST | `/project-registers` | `registers.create` |
| PUT | `/project-registers/:id` | `registers.update` |
| DELETE | `/project-registers/:id` | `registers.delete` |
| GET | `/tasks*` `/tasks/calendar` `/tasks/calendar.ics` | `tasks.read` |
| POST | `/tasks` | `tasks.create` |
| PUT | `/tasks/:id` `/tasks/:id/dependencies` | `tasks.update` |
| PATCH | `/tasks/:id/progress` `/tasks/:id/status` `/tasks/:id/complete` | `tasks.read` + 任务执行人/审核人关系校验 |
| PATCH | `/tasks/:id/schedule` | `tasks.update` |
| DELETE | `/tasks/:id` | `tasks.delete` |
| GET | `/tasks/:id/comments` `/tasks/:id/activities` | `comments.read` + 任务可见范围 |
| POST | `/tasks/:id/comments` | `comments.create` + 任务可见范围 |
| DELETE | `/tasks/:id/comments/:commentId` | `comments.delete` + 评论作者/管理员 |
| GET | `/requests` | `requests.read` |
| POST | `/requests` | `requests.create` |
| PATCH | `/requests/:id/review` `POST /requests/:id/convert-task` `POST /requests/:id/apply-change` | `requests.update` |
| GET | `/project-templates` | `templates.read` |
| POST | `/project-templates` | `templates.create` |
| PUT | `/project-templates/:id` | `templates.update` |
| DELETE | `/project-templates/:id` | `templates.delete` |
| POST | `/project-templates/:id/create-project` | `projects.create` + `templates.read` |
| GET | `/reports` `/reports/:id` | `reports.read` |
| POST | `/reports` | `reports.create` |
| PUT | `/reports/:id` | `reports.update` |
| DELETE | `/reports/:id` | `reports.delete` |
| GET | `/sprints` `/sprints/:id` | `sprints.read` |
| POST | `/sprints` | `sprints.create` |
| PUT | `/sprints/:id` `POST /sprints/:id/tasks` `DELETE /sprints/:id/tasks/:taskId` | `sprints.update` |
| DELETE | `/sprints/:id` | `sprints.delete` |
| GET | `/webhooks` `/webhooks/:id` `/webhooks/deliveries` | `webhooks.read` |
| POST | `/webhooks` | `webhooks.create` |
| PUT | `/webhooks/:id` `POST /webhooks/deliveries/:id/retry` | `webhooks.update` |
| DELETE | `/webhooks/:id` | `webhooks.delete` |
| GET | `/api-tokens` `/api-tokens/:id` `/api-tokens/permission-options` | `api_tokens.read` |
| POST | `/api-tokens` | `api_tokens.create` |
| PUT | `/api-tokens/:id` | `api_tokens.update` |
| DELETE | `/api-tokens/:id` | `api_tokens.delete` |
| GET | `/automation-rules` `/automation-rules/logs` | `automations.read` |
| POST | `/automation-rules` | `automations.create` |
| PUT | `/automation-rules/:id` `POST /automation-rules/:id/run` | `automations.update` |
| DELETE | `/automation-rules/:id` | `automations.delete` |
| POST | `/ai/project-weekly-report` `/ai/project-risk-summary` `/ai/task-breakdown` | `ai.read` + 对应源数据 read 权限 |
| GET | `/stats/dashboard` `/stats/project-health` `/stats/member-workload` | `stats.read` |
| GET | `/notifications` `/notifications/unread-count` | `notifications.read` |
| PATCH | `/notifications/:id/read` `/notifications/read-all` | `notifications.update` |
| GET | `/audit/logs` | `audit.read` |

字段级权限补充：项目预算、实际成本、预计收益、合同编号、合同附件、预算使用率和超预算状态需要 `finance.read`、`finance.update` 或管理员身份读取；写入这些字段需要 `finance.update`。

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
- 请求体: `{ username, name, email, password, weeklyCapacityHours?, roleIds, departmentIds }`
- `weeklyCapacityHours` 为默认周容量，范围 `0-168`，未传默认 `40`

### PUT `/users/:id`
- 请求体可更新 `weeklyCapacityHours`，范围 `0-168`
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
- 响应: `{ list, total, page, pageSize }`
- 财务脱敏: 没有 `finance.read` / `finance.update` 的用户不会看到 `budgetAmount`、`actualCostAmount`、`expectedRevenueAmount`、`contractNo`、`contractAttachments`、`budgetUsageRate`、`costOverBudget`

### GET `/projects/export`
- 用途: 导出当前可见项目为 CSV
- Query: `keyword`
- 财务列: 具备 `finance.read` / `finance.update` 的用户会追加 `预算`、`实际成本`、`预计收益`、`合同编号`、`预算使用率`、`是否超预算`；普通 `projects.read` 用户不导出这些列

### GET `/projects/editor-options`
- 用途: 项目编辑弹窗选项（负责人、参与部门）
- Query: `keyword` `userKeyword` `departmentKeyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }], departments: [{ id, name }] }`

### GET `/projects/:id`
- 财务脱敏规则同 `GET /projects`

### POST `/projects`
- 请求体: `{ code?, name, description, startAt, endAt, attachments?, userIds, departmentIds, budgetAmount?, actualCostAmount?, expectedRevenueAmount?, contractNo?, contractAttachments? }`
- `code` 为空时后端自动生成随机项目编码
- 写入 `budgetAmount`、`actualCostAmount`、`expectedRevenueAmount`、`contractNo` 或 `contractAttachments` 需要 `finance.update`
- 金额字段必须为非负数；`actualCostAmount > budgetAmount > 0` 时会向项目负责人发送 `项目成本超预算` 站内通知
- `contractAttachments`: `[{ fileName, filePath, relativePath, fileSize, mimeType, category, version, accessLevel, expiresAt }]`
  - `category`: `contract|invoice|acceptance|change|other`，空值默认 `contract`
  - `accessLevel`: `finance|internal|external`，空值默认 `finance`
  - `expiresAt`: 可空 RFC3339 时间

### PUT `/projects/:id`
- 请求体同 `POST /projects`；未携带财务字段时普通 `projects.update` 可继续更新基础项，携带任一财务字段则要求 `finance.update`
### DELETE `/projects/:id`

### GET `/projects/:projectId/gantt`
- 甘特图任务数据（含优先级、里程碑、执行人、依赖）
- 前端单项目甘特在具备 `baselines.read` 时会额外读取 `/projects/:projectId/critical-path`，并高亮关键路径任务；关键路径加载失败只影响高亮提示，不影响甘特主数据

### GET `/projects/gantt-portfolio`
- 项目集甘特数据（支持多项目统筹）
- Query: `projectIds`（可选，逗号分隔；为空则返回当前可见项目）

### POST `/projects/:projectId/gantt/auto-resolve`
- 自动同步依赖并顺延任务时间
- 响应: `{ updatedCount, projectId }`

### GET `/projects/:projectId/task-tree`
- 项目分解树结构（任务树）

## 项目基线与关键路径

### GET `/project-baselines`
- 权限: `baselines.read`
- Query: `page` `pageSize` `keyword` `projectId`
- 范围: 管理员查看全部；普通用户仅查看自己创建的项目基线
- 响应: `{ list, total, page, pageSize }`

### POST `/project-baselines`
- 权限: `baselines.create`
- 请求体: `{ projectId, name, description? }`
- 范围: `projectId` 必须是当前用户可见项目
- 效果: 对当前用户可见任务生成计划快照，记录任务数、完成数、计划开始/结束时间和任务状态/进度/排期

### GET `/project-baselines/:id`
- 权限: `baselines.read`
- 范围: 管理员或基线创建人，且基线项目仍需当前用户可见
- 响应: 基线详情 + `compare`
- `compare`: `{ baselineTaskCount, currentTaskCount, baselineCompletedCount, currentCompletedCount, baselinePlannedEndAt, currentPlannedEndAt, endVarianceDays, delayedTaskCount, missingTaskCount, changedTasks }`

### DELETE `/project-baselines/:id`
- 权限: `baselines.delete`
- 范围: 管理员或基线创建人

### GET `/projects/:projectId/critical-path`
- 权限: `baselines.read`
- 范围: 项目必须当前用户可见；只使用当前用户可见任务及其可见依赖计算
- 响应: `{ projectId, projectEndAt, totalDurationDays, criticalTaskIds, tasks, hasCycle }`
- 规则: 若可见依赖存在环，返回 `400` 与错误码 `CRITICAL_PATH_CYCLE`
- 健康度联动: 未完成的关键路径任务逾期时，`/stats/project-health` 的 `criticalOverdueTasks` 会计数，并将项目健康度置为 `red`

## 风险、问题、决策登记册

登记册用于沉淀项目级风险、执行问题和关键决策。所有接口都按当前用户项目可见范围收敛；创建和更新会写登记项活动、审计日志，并通过 `module=project_registers`、`targetId=登记项ID` 发送站内通知。

### GET `/project-registers`
- 权限: `registers.read`
- Query: `page` `pageSize` `keyword` `type` `status` `statuses` `severity` `severities` `projectId`
- `type`: `risk|issue|decision`
- `status`: `open|in_progress|resolved|closed`
- `severity`: `low|medium|high|critical`
- `statuses`、`severities` 支持逗号分隔多选
- 响应: `{ list, total, page, pageSize }`

### GET `/project-registers/:id`
- 权限: `registers.read`
- 范围: 登记项所属项目必须当前用户可见

### POST `/project-registers`
- 权限: `registers.create`
- 请求体: `{ type, projectId, taskId?, title, description?, status?, severity?, probability?, impact?, source?, responsePlan?, resolution?, decisionDetail?, background?, impactScope?, dueAt?, ownerId?, participantIds? }`
- `probability`: `low|medium|high`
- `impact`: `low|medium|high|critical`
- 规则: `projectId` 必须当前用户可见；`taskId` 如填写，任务必须当前用户可见且属于同项目；负责人和参与人必须是有效用户；`dueAt` 如填写必须是 RFC3339；状态默认 `open`，等级默认 `medium`

### PUT `/project-registers/:id`
- 权限: `registers.update`
- 请求体同创建
- 效果: 更新登记项，写入 `register.updated` 活动、审计日志并通知相关用户

### DELETE `/project-registers/:id`
- 权限: `registers.delete`
- 效果: 删除登记项并写审计日志

### GET `/project-registers/:id/activities`
- 权限: `registers.read`
- Query: `page` `pageSize`
- 响应: `{ list, total, page, pageSize }`
- 动态项: `{ id, registerId, actorId, actor, type, summary, detail, createdAt }`

## 任务管理

### GET `/tasks`
- Query: `projectId` `sprintId` `status` `statuses` `priorities` `assigneeIds` `searchFields` `page` `pageSize` `keyword` `sortBy` `sortOrder`
  - `sprintId` 按迭代筛选，仍只返回当前用户可见任务
  - `statuses` 支持逗号分隔的多状态筛选
  - `priorities` 支持逗号分隔的多优先级筛选
  - `assigneeIds` 支持逗号分隔的多人筛选
  - `searchFields` 支持逗号分隔：`taskNo,title,description,projectName,priority,status,customField1,customField2,customField3`
  - `sortBy=priority` 时支持优先级排序
  - `sortOrder` 支持：`asc|desc`；当 `sortBy=priority` 时，后端兼容 `high|medium|low`

### GET `/tasks/export`
- 用途: 导出当前可见任务为 CSV
- Query: `projectId` `status` `keyword`
- CSV 包含估算工时、实际工时、剩余工时列

### GET `/tasks/assignee-options`
- 用途: 任务编辑弹窗执行人选项
- Query: `keyword` `pageSize`
- 响应: `{ users: [{ id, name, username, email }] }`

### GET `/tasks/calendar`
- 用途: 我的日程月/周/日视图
- Query: `start` `end` `mine`
  - `start`/`end` 为 RFC3339 时间；未传时默认返回当前月视图范围
  - `mine=true` 时只返回当前用户创建、执行或审核相关任务；普通用户仍受任务可见范围限制
- 口径: 返回有开始或结束时间、且与查询范围有交集的任务
- 响应: `{ start, end, items: TaskCalendarItem[] }`
- `TaskCalendarItem`: `{ id, taskNo, title, status, priority, isMilestone, progress, startAt, endAt, projectId, projectCode, projectName, assignees, reviewers, tags }`

### GET `/tasks/calendar.ics`
- 用途: 导出当前可见日程为 iCal
- Query: 同 `/tasks/calendar`
- 响应: `text/calendar`

### POST `/tasks`
- 请求体: `{ taskNo?, title, description, customField1?, customField2?, customField3?, status, priority, isMilestone, progress, estimatedHours?, actualHours?, remainingHours?, startAt, endAt, attachments?, projectId, parentId, assigneeIds, tagIds, dependencies? }`
- `dependencies` 格式: `[{ dependsOnTaskId, lagDays, type }]`
- `creatorId` 默认使用当前登录用户
- `taskNo` 唯一（为空自动生成）
- `status` 支持 `pending|queued|processing|reviewing|completed`
- `priority` 支持 `high|medium|low`（默认 `high`）
- `estimatedHours`、`actualHours`、`remainingHours` 为非负数，未传默认 `0`
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
- 请求体同创建任务；工时字段为非负数
### PUT `/tasks/:id/dependencies`
- 请求体: `{ dependencies: [{ dependsOnTaskId, lagDays, type }] }`

### PATCH `/tasks/:id/status`
- 权限: `tasks.read`
- 范围: 只能更新当前账号可见任务；普通用户需为任务执行人或审核人，拥有 `tasks.update` 的用户可更新可见任务
- 请求体: `{ status: "pending|queued|processing|reviewing|completed" }`
- 规则: 更新到 `completed` 必须是任务审核人；完成时后端自动将 `progress` 补为 `100`
- 用途: 任务页 Kanban 拖拽轻量更新状态；成功后返回完整 `Task`

### PATCH `/tasks/:id/schedule`
- 请求体: `{ startAt, endAt }`
- Query: `autoResolve`（可选，默认 `true`）

### GET `/tasks/:id/comments`
- 权限: `comments.read`
- 范围: 只能查看当前账号可见任务的评论
- Query: `page` `pageSize`
- 响应: `{ list, total, page, pageSize }`
- 评论项: `{ id, taskId, authorId, author, content, attachments, mentions, createdAt, updatedAt }`

### POST `/tasks/:id/comments`
- 权限: `comments.create`
- 范围: 只能在当前账号可见任务下评论
- 请求体: `{ content, attachments?, mentionIds? }`
- `content` 支持 `@username`，后端会解析已存在用户名并合并 `mentionIds`
- 被提及用户会收到 `module=tasks`、`targetId=任务ID` 的站内通知

### DELETE `/tasks/:id/comments/:commentId`
- 权限: `comments.delete`
- 规则: 评论作者或管理员可删除
- 删除为软删除，默认评论列表不返回已删除评论

### GET `/tasks/:id/activities`
- 权限: `comments.read`
- 范围: 只能查看当前账号可见任务的动态
- Query: `page` `pageSize`
- 响应: `{ list, total, page, pageSize }`
- 动态项: `{ id, taskId, actorId, actor, type, summary, detail, commentId, comment, createdAt }`
- 当前记录: 任务创建、更新、进度更新、审核完成、依赖/排期调整、自动顺延、评论新增/删除

### DELETE `/tasks/:id`

## 请求入口

### GET `/requests`
- 权限: `requests.read`
- 范围: 管理员和具备 `requests.update` 的用户可查看全部请求；普通用户只查看自己提交的请求
- Query: `page` `pageSize` `keyword` `type` `status` `statuses` `projectId`
- `type`: `project|task|bug|change`
- `status`: `submitted|approved|rejected|converted|applied`

### POST `/requests`
- 权限: `requests.create`
- 普通请求体: `{ type, title, description?, priority?, projectId? }`
- 变更申请请求体: `{ type: "change", title, description?, priority?, targetTaskId, changePayload }`
- `changePayload`: `{ startAt?, endAt?, priority?, assigneeIds?, scopeDescription? }`，至少填写一个变更项；`priority` 取值 `high|medium|low`
- 规则: 变更申请必须选择当前用户可见任务，系统以目标任务项目作为请求项目；`assigneeIds` 必须是有效用户
- 用途: 业务用户提交项目申请、任务请求、缺陷/问题或变更申请；不要求项目/任务创建权限

### PATCH `/requests/:id/review`
- 权限: `requests.update`
- 请求体: `{ status: "approved|rejected", note? }`
- 效果: 记录审批人、审批意见，并通知请求提交人；变更申请审批通过后还需要调用应用接口才会修改任务

### POST `/requests/:id/convert-task`
- 权限: `requests.update`
- 请求体: `{ projectId, assigneeIds?, reviewerIds?, tagIds?, startAt?, endAt? }`
- 规则: 已拒绝、已转换、已应用或 `type=change` 的请求不能转任务
- 效果: 创建任务，复制请求标题/描述/优先级，回填 `convertedTaskId` 并将请求状态置为 `converted`

### POST `/requests/:id/apply-change`
- 权限: `requests.update`
- 规则: 仅 `type=change` 且状态为 `approved` 的请求可应用；目标任务项目必须当前用户可见
- 效果: 将 `changePayload` 中的排期、优先级和执行人应用到目标任务；写入 `change_request.applied` 任务活动、审计、申请人通知，执行人变化会触发对应通知和自动化规则；请求状态置为 `applied` 并记录 `appliedAt/appliedById`
- 响应: `{ request, task }`

## 项目模板

### GET `/project-templates`
- 权限: `templates.read`
- Query: `page` `pageSize` `keyword`
- 响应: `{ list, total, page, pageSize }`

### POST `/project-templates`
- 权限: `templates.create`
- 请求体: `{ name, description?, taskTree }`
- `taskTree` 节点: `{ key?, title, description?, priority?, isMilestone?, relativeStartDay?, durationDays?, dependencies?, children? }`
- `dependencies` 节点: `{ dependsOnKey, lagDays?, type? }`，`type` 支持 `FS|SS|FF|SF`
- 规则: 模板任务标题必填；`key` 全模板唯一；依赖只能引用同模板内任务 key；`durationDays <= 0` 时按 1 天处理

### PUT `/project-templates/:id`
- 权限: `templates.update`
- 请求体同创建模板

### DELETE `/project-templates/:id`
- 权限: `templates.delete`

### POST `/project-templates/:id/create-project`
- 权限: `projects.create`，并要求当前用户具备 `templates.read` 或管理员角色
- 请求体: `{ code?, name, description?, startAt?, endAt?, userIds?, departmentIds? }`
- 效果: 创建项目并按模板任务树生成真实任务；任务编号自动生成；父子关系、里程碑、相对排期和模板内依赖会映射为真实任务关系
- 响应: `{ templateId, project, tasks }`

## 报表中心

报表中心保存配置使用 `reports.*` 权限；页面预览卡片复用 `/stats/project-health`、`/stats/member-workload`、`/tasks/progress-list`，仍分别受 `stats.read`、`tasks.read` 控制。

### GET `/reports`
- 权限: `reports.read`
- Query: `page` `pageSize` `keyword` `type`
- 范围: 管理员可查看全部保存报表；普通用户仅查看自己创建的报表
- 响应: `{ list, total, page, pageSize }`

### GET `/reports/:id`
- 权限: `reports.read`
- 范围: 同列表

### POST `/reports`
- 权限: `reports.create`
- 请求体: `{ name, description?, type, filters?, chartConfig? }`
- `type`: `project_health|member_workload|task_status`
- `filters`: `{ projectId?, keyword?, statuses? }`；`projectId` 必须是当前用户可见项目
- `chartConfig`: `{ displayMode? }`

### PUT `/reports/:id`
- 权限: `reports.update`
- 范围: 管理员可更新全部保存报表；普通用户仅可更新自己创建的报表
- 请求体同创建

### DELETE `/reports/:id`
- 权限: `reports.delete`
- 范围: 同更新

## 迭代管理

### GET `/sprints`
- 权限: `sprints.read`
- Query: `page` `pageSize` `keyword` `status`
- 范围: 管理员可查看全部；普通用户仅查看自己创建或包含自己可见任务的迭代
- 响应: `{ list, total, page, pageSize }`，列表项包含 `taskCount`、`completedTaskCount`、`completionRate`

### GET `/sprints/:id`
- 权限: `sprints.read`
- 响应: 迭代详情与当前用户可见任务列表

### POST `/sprints`
- 权限: `sprints.create`
- 请求体: `{ name, goal?, status?, startAt?, endAt?, capacityHours? }`
- `status`: `planned|active|closed`

### PUT `/sprints/:id`
- 权限: `sprints.update`
- 范围: 管理员或迭代创建人
- 请求体同创建

### POST `/sprints/:id/tasks`
- 权限: `sprints.update` + `tasks.read`
- 范围: 管理员或迭代创建人；`taskIds` 必须全部为当前用户可见任务
- 请求体: `{ taskIds: number[] }`

### DELETE `/sprints/:id/tasks/:taskId`
- 权限: `sprints.update` + `tasks.read`
- 范围: 管理员或迭代创建人；任务也必须当前用户可见

### DELETE `/sprints/:id`
- 权限: `sprints.delete`
- 范围: 管理员或迭代创建人

## Webhook 订阅

Webhook 订阅用于让外部系统接收项目管理事件。当前支持 `task_status_changed`。URL 校验复用自动化 Webhook 安全策略：默认禁止本机、内网和保留地址；测试或内网部署可通过 `AUTOMATION_WEBHOOK_ALLOW_PRIVATE=true` 放开。

管理员创建的订阅接收全量任务状态事件；普通用户创建的订阅仅接收其作为创建人、执行人或审核人可见任务的事件。普通用户也只能查看和重试自己订阅产生的投递日志。

### GET `/webhooks`
- 权限: `webhooks.read`
- Query: `page` `pageSize` `keyword` `event` `isEnabled`
- 范围: 管理员查看全部；普通用户仅查看自己创建的订阅

### GET `/webhooks/:id`
- 权限: `webhooks.read`

### POST `/webhooks`
- 权限: `webhooks.create`
- 请求体: `{ name, event, url, isEnabled? }`
- `event`: `task_status_changed`

### PUT `/webhooks/:id`
- 权限: `webhooks.update`
- 范围: 管理员或订阅创建人
- 请求体同创建

### DELETE `/webhooks/:id`
- 权限: `webhooks.delete`
- 范围: 管理员或订阅创建人

### GET `/webhooks/deliveries`
- 权限: `webhooks.read`
- Query: `page` `pageSize` `subscriptionId` `event` `status`
- 范围: 管理员查看全部；普通用户仅查看自己订阅产生的投递日志
- `status`: `pending|success|failed`

### POST `/webhooks/deliveries/:id/retry`
- 权限: `webhooks.update`
- 范围: 管理员或订阅创建人；仅允许重试非成功投递记录，订阅停用时不可重试

## API Token / 服务账号

API Token 用于外部系统以服务账号身份调用接口。请求仍使用 `Authorization: Bearer <token>`；Token 明文只在创建时返回一次，后端仅保存哈希、前缀和后四位。调用时会按服务账号身份执行现有 RBAC、可见范围和审计逻辑。

### GET `/api-tokens`
- 权限: `api_tokens.read`
- Query: `page` `pageSize` `keyword` `isEnabled`
- 范围: 管理员查看全部；普通用户仅查看自己创建的 Token

### GET `/api-tokens/permission-options`
- 权限: `api_tokens.read`
- 效果: 返回可分配给 Token 的权限候选列表

### GET `/api-tokens/:id`
- 权限: `api_tokens.read`
- 范围: 管理员或 Token 创建人

### POST `/api-tokens`
- 权限: `api_tokens.create`
- 请求体: `{ name, description?, permissionIds, isEnabled?, expiresAt? }`
- `permissionIds` 至少 1 个；`expiresAt` 为 RFC3339 且必须晚于当前时间
- 响应: 返回 Token 元数据和一次性明文 `token`

### PUT `/api-tokens/:id`
- 权限: `api_tokens.update`
- 范围: 管理员或 Token 创建人
- 请求体同创建；不会重新生成明文 Token；已撤销 Token 不可更新

### DELETE `/api-tokens/:id`
- 权限: `api_tokens.delete`
- 范围: 管理员或 Token 创建人
- 效果: 撤销 Token 并停用对应服务账号，保留记录用于审计

## 自动化规则

### GET `/automation-rules`
- 权限: `automations.read`
- Query: `page` `pageSize` `keyword` `trigger` `isEnabled`
- 当前支持触发器: `task_overdue`、`task_status_changed`、`task_progress_changed`、`task_assignee_changed`

### POST `/automation-rules`
- 权限: `automations.create`
- 请求体: `{ name, trigger, isEnabled?, conditions, actions }`
- `task_overdue` 条件: `{ overdueDays?, projectIds? }`，`overdueDays` 默认 1，`projectIds` 为空表示不限制项目
- `task_status_changed` 条件: `{ projectIds?, fromStatuses?, toStatuses? }`；状态取值为 `pending`、`queued`、`processing`、`reviewing`、`completed`；至少配置一个变更前或变更后状态
- `task_progress_changed` 条件: `{ projectIds?, fromProgressMin?, fromProgressMax?, toProgressMin?, toProgressMax? }`；进度边界取值 0-100；至少配置一个变更前或变更后进度边界
- `task_assignee_changed` 条件: `{ projectIds?, assigneeChangeTypes }`；`assigneeChangeTypes` 取值为 `added`、`removed`，至少选择一种执行人变更类型
- `actions`: `{ notifyAssignees?, notifyProjectOwners?, addComment?, commentContent?, addTags?, tagIds?, assignAssignees?, assigneeIds?, callWebhook?, webhookUrl? }`；逾期规则至少启用通知、添加标签、指派执行人或调用 Webhook；状态/进度/执行人变更规则至少启用通知、添加评论、添加标签、指派执行人或调用 Webhook；`addTags=true` 时必须配置 `tagIds`，执行时只追加任务缺失标签；`assignAssignees=true` 时必须配置 `assigneeIds`，执行时只追加缺失执行人且不会递归触发执行人变更规则；`callWebhook=true` 时必须配置 `webhookUrl`，系统会向每个匹配任务发送 JSON POST；未传通知对象时默认同时通知执行人和项目负责人
- Webhook URL 默认只允许 `http`/`https` 且禁止本机、内网和保留地址；测试或内网部署如需允许私有地址，可通过后端环境变量 `AUTOMATION_WEBHOOK_ALLOW_PRIVATE=true` 开启。Webhook 在规则事务提交后投递，失败会把对应执行日志标记为 `failed` 并追加失败原因，但不会回滚已完成的任务更新、标签追加或执行人追加。
- 状态/进度变更规则会在 `PUT /tasks/:id`、`PATCH /tasks/:id/status`、`PATCH /tasks/:id/progress`、`PATCH /tasks/:id/complete` 改变状态或进度时执行；执行人变更规则会在 `PUT /tasks/:id` 改变执行人集合时执行；事件规则写入 `runSource=event` 的执行日志

### PUT `/automation-rules/:id`
- 权限: `automations.update`
- 请求体同创建规则

### POST `/automation-rules/:id/run`
- 权限: `automations.update`
- 效果: 手动执行规则；非管理员只处理当前用户可见项目内的任务；执行结果写入日志并触发站内通知或 Webhook；`task_status_changed`、`task_progress_changed` 和 `task_assignee_changed` 仅响应事件，手动执行会记录为跳过
- 响应: `{ id, ruleId, trigger, status, matchedCount, actionCount, message, actorId, runSource, createdAt }`

### GET `/automation-rules/logs`
- 权限: `automations.read`
- Query: `page` `pageSize` `ruleId` `status` `trigger`

### DELETE `/automation-rules/:id`
- 权限: `automations.delete`

### 后台执行
- 服务启动后会注册小时级后台任务，定时执行已启用的逾期规则；后台执行写入 `runSource=scheduled` 的执行日志。

## AI 助理

AI 助理当前为本地结构化草稿生成：不调用外部模型、不保存草稿、不直接创建或更新项目/任务。所有响应都包含 `requiresConfirmation=true` 和 `sourceRefs`，前端展示为可编辑草稿。

### POST `/ai/project-weekly-report`
- 权限: `ai.read` + `projects.read`；任务状态需 `tasks.read`，评论/活动需 `comments.read`，登记册需 `registers.read`
- 请求体: `{ projectId, weekStart?, weekEnd? }`
- `weekStart/weekEnd`: 可空；传入时必须是 RFC3339，空值默认当前周
- 响应: `{ mode, title, draft, highlights, recommendations, sourceRefs, requiresConfirmation, generatedAt }`

### POST `/ai/project-risk-summary`
- 权限: `ai.read` + `projects.read`；任务风险需 `tasks.read`，登记册风险需 `registers.read`
- 请求体: `{ projectId }`
- 响应: `{ mode, title, draft, highlights, recommendations, sourceRefs, requiresConfirmation, generatedAt }`

### POST `/ai/task-breakdown`
- 权限: `ai.read`；如传 `projectId` 则额外要求 `projects.read` 且项目当前用户可见
- 请求体: `{ projectId?, title?, description? }`
- 响应: `{ mode, title, summary, tasks, sourceRefs, requiresConfirmation, generatedAt }`
- `tasks`: `[{ title, description, priority, isMilestone, relativeStartDay, durationDays, sourceRefs }]`
- 安全边界: AI 助理不会读取财务字段、附件正文、密码、密钥或审计敏感数据；没有源权限时不会把对应源数据纳入草稿

## 统计分析

### GET `/stats/dashboard`
- 响应: 用户数、项目数、任务数、完成率
- 普通用户按任务可见范围统计：创建人、执行人、审核人相关任务。

### GET `/stats/project-health`
- 权限: `stats.read`
- 范围: 管理员查看全量任务和登记册聚合；普通用户仅按任务与项目可见范围聚合项目。
- 响应: `{ projects: ProjectHealth[] }`
- `ProjectHealth`: `{ projectId, projectCode, projectName, health, score, completionRate, totalTasks, completedTasks, overdueTasks, milestoneOverdue, unscheduledTasks, reviewingTasks, criticalOverdueTasks, highRiskRegisters, unresolvedIssues, reasons }`
- `score` 口径: 按任务“计划进度 - 实际进度”的滞后程度计算；实际进度为 `completed=100`，否则取 `progress`；计划进度按 `startAt/endAt` 和当前时间线性计算。
- 权重: `high=3`、`medium=2`、`low=1`，里程碑任务额外 `+1`。
- `health`: `green` 健康、`yellow` 关注、`red` 高风险；逾期、关键路径逾期、里程碑逾期、未排期、待审核堆积、未关闭高风险登记项和未解决问题会进入 `reasons`。
- 登记册联动: 未关闭高风险登记项会扣分并使项目置红；未解决问题会扣分并至少使项目进入关注状态。

### GET `/stats/member-workload`
- 权限: `stats.read`
- 范围: 管理员查看全量任务聚合；普通用户仅按任务可见范围聚合。
- 口径: 当前自然周内未完成任务，包含未排期任务和与本周有交集的排期任务；按执行人汇总估算/实际/剩余工时。
- 响应: `{ weekStart, weekEnd, members }`
- `members`: `[{ userId, name, username, email, taskCount, estimatedHours, actualHours, remainingHours, capacityHours, utilizationRate, overloaded }]`
- `overloaded`: `estimatedHours > capacityHours`；容量为 `0` 且估算工时大于 `0` 时也视为过载。

## 站内通知

### GET `/notifications`
- Query: `page` `pageSize` `isRead` `module` `keyword`
- 登记册通知使用 `module=project_registers`、`targetId=登记项ID`，前端会跳转到 `/registers?registerId=...`

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
