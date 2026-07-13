# RBAC 权限清单与维护规范（CRUD 版）

> 目标：所有业务接口按“增删改查”粒度进行权限控制，不再采用“read/write”二元模型。

## 1. 命名规范

- 命名格式：`<module>.<action>`
- 标准动作：
  - `create`：新增
  - `read`：查询/查看
  - `update`：编辑/变更
  - `delete`：删除

## 2. 菜单与功能分类

菜单按业务功能分组，前端页面路径、API 分类前缀与权限模块保持一致。旧页面路径和旧 API 路径仍保留兼容，但新功能应优先接入分类后的路径。

| 主菜单 | 子菜单 | 前端路径 | API 分类前缀 | 权限模块 |
|---|---|---|---|---|
| 工作台 | 个人工作、站内通知 | `/workbench/*` | `/workbench/*` | `tasks.*` `notifications.*` |
| 项目管理 | 项目列表、模板管理、甘特模块、基线关键路径、风险问题决策 | `/portfolio/*` | `/portfolio/*` | `projects.*` `templates.*` `baselines.*` `registers.*` `finance.*` |
| 执行协作 | 任务列表、迭代管理、我的日程、请求入口 | `/delivery/*` | `/delivery/*` | `tasks.*` `comments.*` `sprints.*` `requests.*` |
| 洞察分析 | 统计分析、报表中心、AI 助理 | `/insights/*` | `/insights/*` | `stats.read` `reports.*` `ai.read` |
| 集成自动化 | 自动化规则、Webhook 订阅、外部门户 | `/integrations/*` | `/integrations/*` | `automations.*` `webhooks.*` `portal.*` |
| 基础配置 | 标签管理 | `/settings/tags` | `/settings/tags` | `tags.*` |
| 系统管理 | RBAC 权限、用户管理、部门管理、审计日志、API Token | `/system/*` | `/system/*` | `system.*` |

## 3. 当前权限总览

| 权限码 | 说明 |
|---|---|
| `system.rbac.create` | 创建角色/权限 |
| `system.rbac.read` | 查看角色/权限 |
| `system.rbac.update` | 更新角色/权限 |
| `system.rbac.delete` | 删除角色/权限 |
| `system.users.create` | 创建用户 |
| `system.users.read` | 查看用户 |
| `system.users.update` | 更新用户 |
| `system.users.delete` | 删除用户 |
| `system.departments.create` | 创建部门 |
| `system.departments.read` | 查看部门 |
| `system.departments.update` | 更新部门 |
| `system.departments.delete` | 删除部门 |
| `tags.create` | 创建标签 |
| `tags.read` | 查看标签 |
| `tags.update` | 更新标签 |
| `tags.delete` | 删除标签 |
| `projects.create` | 创建项目 |
| `projects.read` | 查看项目 |
| `projects.update` | 更新项目 |
| `projects.delete` | 删除项目 |
| `finance.read` | 查看项目预算、成本、收益、合同编号、合同附件与财务导出列 |
| `finance.update` | 维护项目预算、成本、收益、合同编号与合同附件 |
| `baselines.create` | 创建项目基线 |
| `baselines.read` | 查看项目基线与关键路径 |
| `baselines.delete` | 删除项目基线 |
| `registers.create` | 创建风险、问题与决策登记项 |
| `registers.read` | 查看风险、问题与决策登记册 |
| `registers.update` | 更新风险、问题与决策登记项 |
| `registers.delete` | 删除风险、问题与决策登记项 |
| `tasks.create` | 创建任务 |
| `tasks.read` | 查看任务 |
| `tasks.update` | 更新任务（含依赖/排期/工时） |
| `tasks.delete` | 删除任务 |
| `comments.create` | 创建任务评论与提及 |
| `comments.read` | 查看任务评论与活动流 |
| `comments.delete` | 删除自己的任务评论 |
| `requests.create` | 提交项目、任务、缺陷或变更请求 |
| `requests.read` | 查看请求入口与审批记录 |
| `requests.update` | 审批请求、转为任务并应用变更 |
| `templates.create` | 创建项目模板 |
| `templates.read` | 查看项目模板 |
| `templates.update` | 更新项目模板 |
| `templates.delete` | 删除项目模板 |
| `reports.create` | 创建保存报表 |
| `reports.read` | 查看报表中心与保存报表 |
| `reports.update` | 更新保存报表 |
| `reports.delete` | 删除保存报表 |
| `sprints.create` | 创建迭代周期 |
| `sprints.read` | 查看迭代周期与迭代任务 |
| `sprints.update` | 更新迭代周期与任务范围 |
| `sprints.delete` | 删除迭代周期 |
| `webhooks.create` | 创建外部 Webhook 订阅 |
| `webhooks.read` | 查看 Webhook 订阅与投递日志；普通用户仅查看自己的订阅与投递日志 |
| `webhooks.update` | 更新 Webhook 订阅并重试投递 |
| `webhooks.delete` | 删除 Webhook 订阅 |
| `system.api_tokens.create` | 创建服务账号 API Token |
| `system.api_tokens.read` | 查看服务账号 API Token 与权限候选 |
| `system.api_tokens.update` | 更新或禁用服务账号 API Token |
| `system.api_tokens.delete` | 撤销服务账号 API Token |
| `portal.create` | 创建客户或供应商项目门户邀请 |
| `portal.read` | 查看外部门户邀请、访问范围与外部协作记录 |
| `portal.update` | 更新或撤销外部门户邀请 |
| `portal.delete` | 删除外部门户邀请 |
| `automations.create` | 创建自动化规则 |
| `automations.read` | 查看自动化规则与执行日志 |
| `automations.update` | 更新并执行自动化规则 |
| `automations.delete` | 删除自动化规则 |
| `ai.read` | 使用 AI 助理生成只读草稿和建议 |
| `notifications.read` | 查看通知 |
| `notifications.update` | 标记通知已读 |
| `stats.read` | 查看统计分析、项目健康度与成员负载 |
| `system.audit.read` | 查看审计日志 |
| `uploads.create` | 上传附件 |

### 3.1 风险问题决策 CRUD 权限

“风险问题决策”统一使用 `registers` 权限模块。四个权限彼此独立，`read` 不会自动包含创建、更新或删除能力；需要进入该页面的角色至少应分配 `registers.read`。

| 权限码 | RBAC 显示名称 | 页面能力 | 主要接口 |
|---|---|---|---|
| `registers.create` | 风险问题决策-创建 | 显示并使用“新增登记项”入口 | `POST /project-registers` |
| `registers.read` | 风险问题决策-查看 | 进入页面，查看列表、详情、动态和概览 | `GET /project-registers`、`GET /project-registers/:id`、`GET /project-registers/:id/activities`、`GET /stats/register-overview` |
| `registers.update` | 风险问题决策-更新 | 编辑风险、问题或决策登记项 | `PUT /project-registers/:id` |
| `registers.delete` | 风险问题决策-删除 | 删除风险、问题或决策登记项 | `DELETE /project-registers/:id` |

推荐分配方式：只读角色仅分配 `registers.read`；登记项维护角色分配 `registers.create/read/update`；确需清理数据的管理角色再增加 `registers.delete`。

已有数据库升级时，后端启动会执行 `seed.Run`，按权限码自动新增缺失项并更新名称和描述。`admin` 角色会自动同步全部权限；其他已有角色不会被自动授予这四项，需要在“系统管理 > RBAC 权限”中按职责手工分配。

## 4. 接口权限映射（后端已生效）

分类 API 别名与旧路由复用相同权限和处理逻辑；例如 `/portfolio/projects` 等价于 `/projects`，`/delivery/tasks` 等价于 `/tasks`，`/integrations/webhooks` 等价于 `/webhooks`。

| 接口 | 权限 |
|---|---|
| `POST /uploads` | `uploads.create` |
| `GET /system/rbac/permissions` `GET /system/rbac/roles` | `system.rbac.read` |
| `POST /system/rbac/permissions` `POST /system/rbac/roles` | `system.rbac.create` |
| `PUT /system/rbac/permissions/:id` `PUT /system/rbac/roles/:id` | `system.rbac.update` |
| `DELETE /system/rbac/permissions/:id` `DELETE /system/rbac/roles/:id` | `system.rbac.delete` |
| `GET /system/users` | `system.users.read` |
| `POST /system/users` | `system.users.create`；可设置默认周容量 |
| `PUT /system/users/:id` | `system.users.update`；可更新默认周容量 |
| `DELETE /system/users/:id` | `system.users.delete` |
| `GET /system/departments` | `system.departments.read` |
| `POST /system/departments` | `system.departments.create` |
| `PUT /system/departments/:id` | `system.departments.update` |
| `DELETE /system/departments/:id` | `system.departments.delete` |
| `GET /tags` `GET /tags/:id` | `tags.read` |
| `POST /tags` | `tags.create` |
| `PUT /tags/:id` | `tags.update` |
| `DELETE /tags/:id` | `tags.delete` |
| `GET /projects` `GET /projects/:id` `GET /projects/export` `GET /projects/editor-options` `GET /projects/:id/gantt` `GET /projects/gantt-portfolio` `GET /projects/:id/task-tree` | `projects.read`；预算、成本、预计收益、合同编号、合同附件、预算使用率、超预算状态和项目导出财务列额外要求 `finance.read`、`finance.update` 或管理员 |
| `POST /projects` | `projects.create`；携带预算、成本、预计收益、合同编号或合同附件时额外要求 `finance.update` |
| `PUT /projects/:id` `POST /projects/:id/gantt/auto-resolve` | `projects.update`；`PUT /projects/:id` 携带任一财务字段时额外要求 `finance.update`，未携带财务字段可只更新基础项 |
| `DELETE /projects/:id` | `projects.delete` |
| `GET /projects/:id/critical-path` `GET /project-baselines` `GET /project-baselines/:id` | `baselines.read`；关键路径与基线比较只使用当前用户可见任务和依赖 |
| `POST /project-baselines` | `baselines.create`；项目必须当前用户可见，快照只包含当前用户可见任务 |
| `DELETE /project-baselines/:id` | `baselines.delete`；普通用户仅删除自己创建的基线 |
| `GET /project-registers` `GET /project-registers/:id` `GET /project-registers/:id/activities` | `registers.read`；普通用户仅查看当前用户可见项目内的登记项 |
| `POST /project-registers` | `registers.create`；项目必须当前用户可见，关联任务必须属于同项目且当前用户可见 |
| `PUT /project-registers/:id` | `registers.update`；更新会写登记项活动、审计和通知 |
| `DELETE /project-registers/:id` | `registers.delete`；项目仍需当前用户可见 |
| `GET /tasks*` `GET /tasks/calendar` `GET /tasks/calendar.ics` | `tasks.read`；日程和 iCal 导出仅返回当前用户可见任务 |
| `POST /tasks` | `tasks.create`；可写入估算/实际/剩余工时 |
| `PATCH /tasks/:id/progress` `PATCH /tasks/:id/status` `PATCH /tasks/:id/complete` | `tasks.read` + 任务执行人/审核人关系校验 |
| `PUT /tasks/:id` `PUT /tasks/:id/dependencies` `PATCH /tasks/:id/schedule` | `tasks.update`；`PUT /tasks/:id` 可更新估算/实际/剩余工时 |
| `DELETE /tasks/:id` | `tasks.delete` |
| `GET /tasks/:id/comments` `GET /tasks/:id/activities` | `comments.read` + 任务可见范围 |
| `POST /tasks/:id/comments` | `comments.create` + 任务可见范围 |
| `DELETE /tasks/:id/comments/:commentId` | `comments.delete` + 评论作者/管理员 |
| `GET /requests` | `requests.read`；普通用户仅查看自己提交的请求 |
| `POST /requests` | `requests.create` |
| `PATCH /requests/:id/review` `POST /requests/:id/convert-task` | `requests.update` |
| `POST /requests/:id/apply-change` | `requests.update`；仅审批通过的变更申请可应用，目标任务所属项目必须当前用户可见 |
| `GET /project-templates` | `templates.read` |
| `POST /project-templates` | `templates.create` |
| `PUT /project-templates/:id` | `templates.update` |
| `DELETE /project-templates/:id` | `templates.delete` |
| `POST /project-templates/:id/create-project` | `projects.create` + `templates.read` |
| `GET /reports` `GET /reports/:id` `GET /reports/:id/run` `GET /reports/:id/export.csv` | `reports.read`；普通用户仅查看、运行和导出自己创建的保存报表，报表数据仍按项目/任务可见范围过滤 |
| `POST /reports` | `reports.create` |
| `PUT /reports/:id` | `reports.update`；普通用户仅更新自己创建的保存报表 |
| `GET /reports/:id/subscription` | `reports.read`；查看自己的报表订阅 |
| `PUT /reports/:id/subscription` `DELETE /reports/:id/subscription` `POST /reports/:id/subscription/run` | `reports.update`；普通用户仅配置和触发自己创建报表的订阅 |
| `DELETE /reports/:id` | `reports.delete`；普通用户仅删除自己创建的保存报表 |
| `GET /sprints` `GET /sprints/:id` | `sprints.read`；普通用户仅查看自己创建或包含自己可见任务的迭代 |
| `POST /sprints` | `sprints.create` |
| `PUT /sprints/:id` | `sprints.update`；普通用户仅更新自己创建的迭代 |
| `POST /sprints/:id/tasks` `DELETE /sprints/:id/tasks/:taskId` | `sprints.update` + `tasks.read`；普通用户仅更新自己创建的迭代，加入/移除任务仍校验任务可见范围 |
| `DELETE /sprints/:id` | `sprints.delete`；普通用户仅删除自己创建的迭代 |
| `GET /webhooks` `GET /webhooks/:id` `GET /webhooks/deliveries` | `webhooks.read`；普通用户仅查看自己创建的订阅和投递日志 |
| `POST /webhooks` | `webhooks.create` |
| `PUT /webhooks/:id` `POST /webhooks/deliveries/:id/retry` | `webhooks.update`；普通用户仅更新自己创建的订阅并重试其投递 |
| `DELETE /webhooks/:id` | `webhooks.delete`；普通用户仅删除自己创建的订阅 |
| Webhook 订阅任务状态事件投递 | `webhooks.create/update` 控制订阅配置；管理员订阅接收全量事件，普通用户订阅仅接收订阅创建人可见任务事件 |
| `GET /system/api-tokens` `GET /system/api-tokens/:id` `GET /system/api-tokens/permission-options` | `system.api_tokens.read`；普通用户仅查看自己创建的 Token |
| `POST /system/api-tokens` | `system.api_tokens.create`；创建时自动生成服务账号和独立角色，明文 Token 仅返回一次 |
| `PUT /system/api-tokens/:id` | `system.api_tokens.update`；普通用户仅更新自己创建的 Token |
| `DELETE /system/api-tokens/:id` | `system.api_tokens.delete`；普通用户仅撤销自己创建的 Token |
| `GET /portal-invites` | `portal.read`；管理员查看全部，普通用户仅查看自己创建或当前可见项目下的邀请 |
| `POST /portal-invites` | `portal.create`；项目必须在当前用户可见范围内，明文邀请 Token 仅返回一次 |
| `PUT /portal-invites/:id` `PATCH /portal-invites/:id/revoke` | `portal.update`；可更新邀请范围、过期时间、附件白名单或撤销邀请 |
| `DELETE /portal-invites/:id` | `portal.delete`；删除邀请 |
| `GET /portal/:token` `POST /portal/:token/requests` `POST /portal/:token/tasks/:taskId/comments` `POST /portal/:token/uploads` | 公开 Token 校验；不走登录态 RBAC，只允许访问该邀请授权项目内 `externalVisible=true` 任务和邀请白名单附件 |
| `GET /automation-rules` `GET /automation-rules/logs` | `automations.read` |
| `POST /automation-rules` | `automations.create` |
| `PUT /automation-rules/:id` `POST /automation-rules/:id/run` | `automations.update` |
| `DELETE /automation-rules/:id` | `automations.delete` |
| 自动化状态/进度/执行人变更触发的通知/评论/添加标签/指派执行人/Webhook | 通知、评论、添加标签和指派执行人由已启用规则在任务写事务内执行，Webhook 在事务提交后投递；触发人仍需通过任务接口原有权限与可见范围校验，不额外要求 `comments.create`、`notifications.update`、`tags.update` 或 `tasks.update`；指派动作会让新增执行人获得该任务可见性，Webhook 会调用外部地址，配置与手动运行受 `automations.create/update` 控制 |
| `POST /ai/project-weekly-report` `POST /ai/project-risk-summary` `POST /ai/register-image-description` `POST /ai/task-breakdown` | `ai.read`；项目上下文额外要求 `projects.read` 并校验项目可见范围，任务上下文额外要求 `tasks.read`，评论/活动流额外要求 `comments.read`，登记册上下文额外要求 `registers.read`；AI 助理只返回草稿，不写项目/任务 |
| `GET /stats/dashboard` `GET /stats/project-health` `GET /stats/member-workload` | `stats.read`；普通用户按任务和项目可见范围聚合，成员负载按执行人周容量标记过载，关键路径逾期、未关闭高风险登记项和未解决问题会影响项目健康度 |
| `GET /notifications` `GET /notifications/unread-count` | `notifications.read` |
| `PATCH /notifications/:id/read` `PATCH /notifications/read-all` | `notifications.update` |
| `GET /system/audit/logs` | `system.audit.read` |

## 5. 初始化与升级策略

- `seed` 初始化时会自动创建/更新以上权限目录。
- 管理员角色 `admin` 每次初始化都会被覆盖为“拥有全部权限”，默认 `admin` 用户会绑定该角色，因此初始化后必须具备整个系统的所有权限。
- 旧权限会自动迁移后清理：
  - `*.write` => 对应模块的 `create/read/update/delete`
  - `rbac.manage` => `system.rbac.create/read/update/delete`
  - `rbac.*`、`users.*`、`departments.*`、`api_tokens.*`、`audit.read` => 对应 `system.*` 权限
  - `notifications.write` => `notifications.read/update`

## 6. 默认角色策略

- `admin`：全量权限（自动同步）。
- `member`：默认具备 `notifications.read` + `notifications.update` + `requests.create` + `requests.read`，其余按业务分配。

## 7. 新功能接入规范

1. 在 `backend/internal/seed/seed.go` 维护新权限码。
2. 在 `backend/internal/router/router.go` 为每个接口绑定对应 CRUD 权限。
3. 同步更新本文件与 `docs/API.md`。
4. 用“有权限/无权限”两个角色验证 200/403 行为。
