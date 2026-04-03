# RBAC 权限清单与维护规范

> 目的：统一项目内权限命名、授权边界与维护流程，确保后续新增功能可快速接入 RBAC。

## 1. 权限命名规范

- 命名格式：`<module>.<action>`
- 常用动作：
  - `read`：查询/查看
  - `write`：创建/修改/删除
  - `manage`：管理类高权限（如 RBAC）
- 兼容规则（后端已实现）：拥有 `xxx.write` 视为自动具备 `xxx.read`。

---

## 2. 当前权限总览（系统生效）

| 权限码 | 模块 | 说明 |
|---|---|---|
| `rbac.manage` | RBAC | 角色/权限管理 |
| `users.read` | 用户 | 查看用户列表 |
| `users.write` | 用户 | 新增/编辑/删除用户 |
| `departments.read` | 部门 | 查看部门列表 |
| `departments.write` | 部门 | 新增/编辑/删除部门 |
| `tags.read` | 标签 | 查看标签列表、标签统计 |
| `tags.write` | 标签 | 新增/编辑/删除标签 |
| `projects.read` | 项目 | 查看项目、项目详情、甘特图、任务树、项目导出 |
| `projects.write` | 项目 | 新增/编辑/删除项目 |
| `tasks.read` | 任务 | 查看任务、进度列表、我的任务、任务导出 |
| `tasks.write` | 任务 | 新增/编辑/删除任务、维护任务标签关联 |
| `notifications.read` | 通知 | 查看通知、未读数、标记已读 |
| `notifications.write` | 通知 | 预留写权限（当前核心接口可由 `read` 完成） |
| `stats.read` | 统计 | 查看统计分析 |
| `audit.read` | 审计 | 查看审计日志 |

---

## 3. 权限与后端路由映射

| 路由前缀 | 需要权限 |
|---|---|
| `/rbac/*` | `rbac.manage` |
| `/users/*` | `users.read`（写操作再校验 `users.write`） |
| `/departments/*` | `departments.read`（写操作再校验 `departments.write`） |
| `/tags/*` | `tags.read`（写操作再校验 `tags.write`） |
| `/projects/*` | `projects.read`（写操作再校验 `projects.write`） |
| `/tasks/*` | `tasks.read`（写操作再校验 `tasks.write`） |
| `/stats/*` | `stats.read` |
| `/notifications/*` | `notifications.read` |
| `/audit/*` | `audit.read` |

---

## 4. 前端菜单权限映射

| 菜单 | 权限 |
|---|---|
| 统计分析 | `stats.read` |
| RBAC 权限 | `rbac.manage` |
| 用户管理 | `users.read` |
| 部门管理 | `departments.read` |
| 标签管理 | `tags.read` |
| 项目列表 | `projects.read` |
| 任务列表 | `tasks.read` |
| 站内通知 | `notifications.read` |
| 审计日志 | `audit.read` |
| 个人工作 | `tasks.read` |

---

## 5. 默认角色策略

- `admin`：
  - 自动绑定全部权限（seed 初始化时覆盖为全量）
- `member`：
  - 默认至少包含 `notifications.read`（用于通知可见）
  - 其他业务权限按实际角色分配

---

## 6. 新功能接入 RBAC 的标准流程（必须执行）

新增任何功能模块时，按以下顺序维护：

1. **定义权限码**
   - 在 `backend/internal/seed/seed.go` 的 `permissionCodes` 中新增权限码。
2. **后端路由绑定权限**
   - 在 `backend/internal/router/router.go` 对新路由组/接口添加 `RequirePermission`。
3. **前端菜单与页面权限**
   - 在 `frontend/src/components/Layout.tsx` 菜单配置中绑定对应权限。
4. **文档同步**
   - 更新本文件 `docs/RBAC_PERMISSIONS.md`。
   - 更新 `docs/API.md`（接口说明）。
   - 必要时更新 `backend/docs/openapi.yaml`。
5. **联调验证**
   - 使用“有权限/无权限”两个角色验证页面可见性与接口 403 行为。

---

## 7. 常见排查

- 菜单不显示：
  - 检查角色是否有对应权限。
  - 退出重新登录，刷新前端本地权限缓存。
- 有菜单但接口 403：
  - 检查路由是否需要 `write` 权限。
- 权限刚保存未生效：
  - 等待前端 profile 轮询或手动刷新页面/重新登录。
