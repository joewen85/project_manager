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
| `tasks.create` | 创建任务 |
| `tasks.read` | 查看任务 |
| `tasks.update` | 更新任务（含依赖/排期） |
| `tasks.delete` | 删除任务 |
| `notifications.read` | 查看通知 |
| `notifications.update` | 标记通知已读 |
| `stats.read` | 查看统计分析 |
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
| `POST /users` | `users.create` |
| `PUT /users/:id` | `users.update` |
| `DELETE /users/:id` | `users.delete` |
| `GET /departments` | `departments.read` |
| `POST /departments` | `departments.create` |
| `PUT /departments/:id` | `departments.update` |
| `DELETE /departments/:id` | `departments.delete` |
| `GET /tags` `GET /tags/:id` | `tags.read` |
| `POST /tags` | `tags.create` |
| `PUT /tags/:id` | `tags.update` |
| `DELETE /tags/:id` | `tags.delete` |
| `GET /projects*` | `projects.read` |
| `POST /projects` | `projects.create` |
| `PUT /projects/:id` `POST /projects/:id/gantt/auto-resolve` | `projects.update` |
| `DELETE /projects/:id` | `projects.delete` |
| `GET /tasks*` | `tasks.read` |
| `POST /tasks` | `tasks.create` |
| `PUT /tasks/:id` `PUT /tasks/:id/dependencies` `PATCH /tasks/:id/schedule` | `tasks.update` |
| `DELETE /tasks/:id` | `tasks.delete` |
| `GET /stats/dashboard` | `stats.read` |
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
- `member`：默认具备 `notifications.read` + `notifications.update`，其余按业务分配。

## 6. 新功能接入规范

1. 在 `backend/internal/seed/seed.go` 维护新权限码。
2. 在 `backend/internal/router/router.go` 为每个接口绑定对应 CRUD 权限。
3. 同步更新本文件与 `docs/API.md`。
4. 用“有权限/无权限”两个角色验证 200/403 行为。
