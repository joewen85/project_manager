# Project Progress Status

## 当前状态（截至本次）
- 总体进度：**约 98%（功能完成，compose 真机验收完成，k8s 真机验收待执行）**
- 分支：`main`
- 最近提交：
  - `f4f7908` feat(backend): standardize errors and complete rbac/task workflows
  - `a0a25fb` feat(frontend): add full CRUD forms, permission UI and auth interceptors
  - `3bdc896` docs(api): sync openapi and audit/rbac endpoints

## 已完成功能
- RBAC、用户、部门、项目、任务、进度统计、个人工作、甘特图、任务树、审计日志
- API 文档更新与 OpenAPI 同步
- Docker Compose / Helm 交付
- Compose 真机启动验收通过（健康检查 + 登录接口验证）
- 前端路由懒加载已完成（主包体积显著下降）
- 前端 UI/UX 阶段H已推进：完成 H1/H2/H3（设计变量、布局统一、导航高亮）
- 前端 UI/UX 阶段H继续推进：完成 H4（按钮/输入/表格/弹窗样式统一）
- 前端 UI/UX 阶段H继续推进：完成 H5（分页统一 + 列表加载/空态/错误态一致）
- 前端 UI/UX 阶段H继续推进：完成 H6（表单必填标记、提交态、错误反馈、时间校验）
- 前端 UI/UX 阶段H继续推进：完成 H7（图表、甘特图、任务树状态色与图例统一）
- 前端 UI/UX 阶段H继续推进：完成 H8（统一动效、active反馈、减少动画偏好适配）
- 前端 UI/UX 阶段H继续推进：完成 H9（键盘焦点、ARIA、多断点响应式适配）
- 前端 UI/UX 阶段H继续推进：完成 H10（按页面模块逐项收敛视觉与交互一致性）
- 前端 UI/UX 阶段H继续推进：完成 H11/H12（UI 验收清单与前端规范文档沉淀）
- 新增 CSV 导出能力（项目导出、任务导出，均按当前用户可见范围导出）
- 补强后端集成测试（可见范围与导出链路回归）
- 新增站内通知能力（通知列表、未读计数、单条/全部已读、任务/项目变更触发）

## 待完成事项（关键）
1. **k8s 真机部署验收**
   - 在目标 k8s 环境执行 `helm upgrade --install project-manager ./deploy/helm/project-manager`
   - 验证 `/health`、登录、核心 CRUD 链路
   - 若 compose 启动场景，使用：`bash scripts/compose-up.sh`

2. **测试完善**
   - 前端关键流程冒烟测试（登录、RBAC、任务创建、统计页）

3. **体验优化**
   - 通用组件升级（按钮/输入/表格/模态统一）
   - 表单交互优化（校验提示、成功提示、加载状态）
   - 数据可视化与可访问性优化（图表/对比度/键盘可达）

## 本轮执行记录
- 按要求跳过 `k8s 真机部署验收`
- 已开始执行 `PLAN.md` 阶段H（基于 `ui-ux-pro-max`）
- 完成：CSS 设计变量体系、顶层布局收敛、侧栏菜单 active 高亮与路由匹配优化
- 完成：业务页面容器统一为 `page-section`，组件样式统一（btn/input/table/modal/toolbar）
- 完成：新增 `DataState` 组件并接入 Projects/Tasks/Users/Departments/Audit/RBAC 列表页
- 完成：表单交互升级（`required-label`、提交中按钮禁用、统一成功/错误反馈、日期先后校验）
- 完成：新增状态设计令牌（`constants/status.ts`）并统一 Dashboard/Gantt/TaskTree 状态可视化
- 完成：统一过渡时长与缓动、按钮/导航/分页 active 反馈、Modal 入场动画与 `prefers-reduced-motion`
- 完成：分页与筛选组件 ARIA 语义增强、focus-visible 统一、320/768/1024/1440 断点优化
- 完成：Dashboard/MyWork 状态接入、各模块搜索筛选 ARIA 标签补齐、模块级页面结构收敛
- 完成：新增 `docs/UI_QA_CHECKLIST.md` 与 `docs/FRONTEND_UI_GUIDE.md`，并在 README 关联
- 完成：新增 `GET /projects/export`、`GET /tasks/export`，支持 CSV 导出
- 完成：新增后端集成测试，验证普通用户项目/任务/统计/导出范围收敛
- 完成：站内通知未读数字徽标（侧栏菜单），通知读/全部已读后自动刷新计数
- 完成：顶栏通知铃铛与最近 5 条下拉预览，支持直接标记已读并联动未读数
- 完成：通知模块筛选与关键词搜索，顶栏通知轮询优化（减少非必要列表请求）
- 完成：通知未授权提示增强（管理员可一键跳转 RBAC 授权）
- 完成：通知下拉键盘可访问性增强（↑/↓、Home/End、Enter/Space、ESC）
- 完成：通知下拉未读优先排序与“今天/更早”分组展示
- 完成：通知 API 降级策略拆分（列表接口与未读计数接口独立降级，避免单接口缺失导致 404 循环）
- 完成：Layout 轮询优化（`/auth/profile` 降为 60 秒轮询且仅前台页面触发，避免开发态高频请求）
- 完成：补充 favicon 声明，消除本地 `favicon.ico 404` 控制台噪音

## 下一步执行顺序（建议）
1. 先跑 Compose 全链路验收
2. 再跑 Helm 部署验收
3. 最后补自动化测试和体验优化
