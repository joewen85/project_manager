# `optimzie-plan.md` 执行计划（稳健提效版）

## 完成进展（2026-03-25）
- 已补齐事务回滚可验证能力：在后端事务链路加入测试专用 failpoint（默认关闭，不影响生产行为）。
- 已新增 2 条集成回滚用例：
  - `TestTaskCreateRollbackOnFailpoint`
  - `TestProjectUpdateRollbackOnFailpoint`
- 已新增 RBAC 回滚用例：
  - `TestRbacCreateRoleRollbackOnFailpoint`
  - `TestRbacCreatePermissionRollbackOnFailpoint`
- 已完成 RBAC 写链路事务化与关联 helper 接入（角色/权限的 create/update/delete）。
- 已完成错误与审计细节统一收敛：
  - 新增 `respondDBError` 统一 DB 错误响应
  - 新增 `auditDetailf` 统一审计 detail 模板
- 已扩展到查询型 handler：
  - `notifications` / `audit` / `stats` / `export` / `scope` / `tasks` / `projects` 查询错误均统一到 `respondDBError`
- 已完成前端 `any` 清理（核心页面与布局）：
  - `Layout` / `RbacPage` / `AuditPage` / `MyWorkPage` / `NotificationsPage`
  - 同步补齐 `types`（`Permission`/`AuditLog`/`MyWorkData`）与 Axios `silent` 配置类型声明
- 已完成后端验证错误收敛：
  - 新增 `respondValidationError` 并替换所有 `ShouldBindJSON` 入口，统一 `VALIDATION_ERROR` 响应格式
- 已完成前端分页/列表解析通用化：
  - 新增 `toPageResult` / `toArray`（`services/api.ts`）
  - `Users` / `Departments` / `Projects` / `Tasks` / `Audit` / `Notifications` / `Layout` 已接入，收敛分页解析重复代码
  - `Audit` / `Notifications` 查询参数拼接统一接入 `buildQuery`
- 已进一步收敛前端加载模板：
  - 新增 `fetchPage` / `fetchArray`（`services/api.ts`）
  - 核心列表页与通知下拉改为统一 loader 调用，减少页面内 `api.get + parse` 模板代码
- 已完成剩余页面数据层统一：
  - 新增 `fetchData`（对象响应统一读取）并在 `Dashboard` / `MyWork` / `Layout` / `Projects` / `Rbac` 接入
  - 页面层 `api.get` 直连已收敛为统一 helper（保留写操作 `post/put/delete/patch`）
- 已完成部署链路鲁棒性增强：
  - `backend/frontend Dockerfile` 支持构建镜像与代理参数（`GO_BUILDER_IMAGE` / `APP_RUNTIME_IMAGE` / `NODE_BUILDER_IMAGE` / `NGINX_IMAGE` / `GO_PROXY` / `NPM_REGISTRY`）
  - `scripts/compose-up.sh` 增加镜像自动回退（含 DaoCloud）与构建代理默认值，弱网环境可稳定拉起
- 已完成本轮 benchmark 实测输出：
  - `docs/benchmark/after.md`（`after-frontend-loader`）
  - `docs/benchmark/compare.md`（基于 `before.template` 的对比模板）
- 验证结果：`go test ./...` 与 `npm run build` 均通过。

## Summary
- 目标：在不改变业务语义前提下，优化后端查询效率、事务一致性、前端类型安全与分页性能。
- 原则：优先低风险高收益，所有改动保持向后兼容，可灰度回滚。
- 基线：后端 `go test ./...`、前端 `npm run build` 作为交付门槛。

## Step-by-step
1. **建立基线与守护线**
   - 新增 `scripts/benchmark-api.sh`，支持关键接口耗时采样（dashboard/tasks/projects）。
   - 保留并扩充现有后端单测，覆盖排序解析与权限缓存行为。
2. **后端鉴权查询优化**
   - 在 `RequirePermission` 中引入请求级权限缓存，避免同一请求重复查库。
   - 在 scope 逻辑中缓存 `isAdmin` 判定，减少重复角色存在性查询。
3. **后端事务化重构**
   - 用户/部门/项目/任务的 `create/update/delete` 改为事务边界。
   - 将主表写入、关联表同步、通知写入、审计写入纳入同一事务。
4. **查询与索引优化**
   - 新增任务、通知、审计高频查询索引（模型 tag 级别）。
   - `GET /projects`、`GET /tasks` 支持 `sortBy` + `sortOrder` 服务端排序。
5. **前端类型化 API 层**
   - 在 `src/services/api.ts` 增加统一 `buildQuery` 与 `readApiError`。
   - 在 `src/types/index.ts` 扩展通用分页与核心实体类型，减少 `any`。
6. **页面改造（服务端驱动）**
   - `UsersPage`、`DepartmentsPage` 改为服务端分页与统一错误处理。
   - `ProjectsPage`、`TasksPage` 改为服务端分页/排序/筛选驱动，保留原交互能力。
7. **体验与包体优化**
   - Dashboard 图表拆分为懒加载组件 `DashboardCharts`，降低路由初始负担。
   - 拦截器统一错误语义，减少全局阻断式提示。

## Public API Changes
- 兼容新增查询参数：
  - `GET /projects`: `sortBy`, `sortOrder`
  - `GET /tasks`: `sortBy`, `sortOrder`

## Acceptance
- 后端测试通过：`cd backend && go test ./...`
- 前端构建通过：`cd frontend && npm run build`
- 基准脚本可运行：`bash scripts/benchmark-api.sh`

## Rollback
- 后端：可先回退事务化提交（handler 层），保留索引与排序能力不影响兼容。
- 前端：可按页面维度回退（Users/Departments/Projects/Tasks 独立）。
- 数据库：仅新增索引，无破坏性结构迁移，可安全回滚代码。
