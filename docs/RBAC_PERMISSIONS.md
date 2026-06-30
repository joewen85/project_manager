# RBAC 权限清单与维护规范（CRUD 版）

> 目标：所有业务接口按“增删改查”粒度进行权限控制，不再采用“read/write”二元模型。

## 1. 命名规范

- 命名格式：`<module>.<action>`
- 标准动作：
  - `create`：新增
  - `read`：查询/查看
  - `update`：编辑/变更
  - `delete`：删除

## 2. 当前权限总览

| 权限码 | 说明 |
|---|---|
| `rbac.create` | 创建角色/权限 |
| `rbac.read` | 查看角色/权限 |
| `rbac.update` | 更新角色/权限 |
| `rbac.delete` | 删除角色/权限 |
| `users.create` | 创建用户 |
| `users.read` | 查看用户 |
| `users.update` | 更新用户 |
| `users.delete` | 删除用户 |
| `departments.create` | 创建部门 |
| `departments.read` | 查看部门 |
| `departments.update` | 更新部门 |
| `departments.delete` | 删除部门 |
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
| `api_tokens.create` | 创建服务账号 API Token |
| `api_tokens.read` | 查看服务账号 API Token 与权限候选 |
| `api_tokens.update` | 更新或禁用服务账号 API Token |
| `api_tokens.delete` | 撤销服务账号 API Token |
| `automations.create` | 创建自动化规则 |
| `automations.read` | 查看自动化规则与执行日志 |
| `automations.update` | 更新并执行自动化规则 |
| `automations.delete` | 删除自动化规则 |
| `notifications.read` | 查看通知 |
| `notifications.update` | 标记通知已读 |
| `stats.read` | 查看统计分析、项目健康度与成员负载 |
| `audit.read` | 查看审计日志 |
| `uploads.create` | 上传附件 |

## 3. 接口权限映射（后端已生效）

| 接口 | 权限 |
|---|---|
| `POST /uploads` | `uploads.create` |
| `GET /rbac/permissions` `GET /rbac/roles` | `rbac.read` |
| `POST /rbac/permissions` `POST /rbac/roles` | `rbac.create` |
| `PUT /rbac/permissions/:id` `PUT /rbac/roles/:id` | `rbac.update` |
| `DELETE /rbac/permissions/:id` `DELETE /rbac/roles/:id` | `rbac.delete` |
| `GET /users` | `users.read` |
| `POST /users` | `users.create`；可设置默认周容量 |
| `PUT /users/:id` | `users.update`；可更新默认周容量 |
| `DELETE /users/:id` | `users.delete` |
| `GET /departments` | `departments.read` |
| `POST /departments` | `departments.create` |
| `PUT /departments/:id` | `departments.update` |
| `DELETE /departments/:id` | `departments.delete` |
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
| `GET /reports` `GET /reports/:id` | `reports.read`；普通用户仅查看自己创建的保存报表 |
| `POST /reports` | `reports.create` |
| `PUT /reports/:id` | `reports.update`；普通用户仅更新自己创建的保存报表 |
| `DELETE /reports/:id` | `reports.delete`；普通用户仅删除自己创建的保存报表 |
| 报表中心预览卡片 | 复用 `/stats/project-health`、`/stats/member-workload`、`/tasks/progress-list`，仍分别要求 `stats.read`、`tasks.read` |
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
| `GET /api-tokens` `GET /api-tokens/:id` `GET /api-tokens/permission-options` | `api_tokens.read`；普通用户仅查看自己创建的 Token |
| `POST /api-tokens` | `api_tokens.create`；创建时自动生成服务账号和独立角色，明文 Token 仅返回一次 |
| `PUT /api-tokens/:id` | `api_tokens.update`；普通用户仅更新自己创建的 Token |
| `DELETE /api-tokens/:id` | `api_tokens.delete`；普通用户仅撤销自己创建的 Token |
| `GET /automation-rules` `GET /automation-rules/logs` | `automations.read` |
| `POST /automation-rules` | `automations.create` |
| `PUT /automation-rules/:id` `POST /automation-rules/:id/run` | `automations.update` |
| `DELETE /automation-rules/:id` | `automations.delete` |
| 自动化状态/进度/执行人变更触发的通知/评论/添加标签/指派执行人/Webhook | 通知、评论、添加标签和指派执行人由已启用规则在任务写事务内执行，Webhook 在事务提交后投递；触发人仍需通过任务接口原有权限与可见范围校验，不额外要求 `comments.create`、`notifications.update`、`tags.update` 或 `tasks.update`；指派动作会让新增执行人获得该任务可见性，Webhook 会调用外部地址，配置与手动运行受 `automations.create/update` 控制 |
| `GET /stats/dashboard` `GET /stats/project-health` `GET /stats/member-workload` | `stats.read`；普通用户按任务和项目可见范围聚合，成员负载按执行人周容量标记过载，关键路径逾期、未关闭高风险登记项和未解决问题会影响项目健康度 |
| `GET /notifications` `GET /notifications/unread-count` | `notifications.read` |
| `PATCH /notifications/:id/read` `PATCH /notifications/read-all` | `notifications.update` |
| `GET /audit/logs` | `audit.read` |

## 4. 初始化与升级策略

- `seed` 初始化时会自动创建/更新以上权限目录。
- 管理员角色 `admin` 每次初始化都会被覆盖为“拥有全部权限”。
- 旧权限会自动迁移后清理：
  - `*.write` => 对应模块的 `create/read/update/delete`
  - `rbac.manage` => `rbac.create/read/update/delete`
  - `notifications.write` => `notifications.read/update`

## 5. 默认角色策略

- `admin`：全量权限（自动同步）。
- `member`：默认具备 `notifications.read` + `notifications.update` + `requests.create` + `requests.read`，其余按业务分配。

## 6. 新功能接入规范

1. 在 `backend/internal/seed/seed.go` 维护新权限码。
2. 在 `backend/internal/router/router.go` 为每个接口绑定对应 CRUD 权限。
3. 同步更新本文件与 `docs/API.md`。
4. 用“有权限/无权限”两个角色验证 200/403 行为。
