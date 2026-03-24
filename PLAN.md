# Project Manager Implementation Plan

> 目标：基于中小企业场景，交付可上线的项目管理系统（Go + Gin + Gorm + JWT + MySQL + Vite + React + TS）。

## 阶段A：基础能力与规范
- [x] 统一分页返回：`list/total/page/pageSize`
- [x] 统一错误返回：`code/message`
- [x] 鉴权与权限中间件分层：`JWT + Permission`
- [x] 前端数据层：Axios 拦截器、401 自动跳转、全局错误提示
- [x] 完整接口响应结构一致性回归（全 API 覆盖）

## 阶段B：RBAC 权限管理
- [x] 权限 CRUD（列表/创建/修改/删除）
- [x] 角色 CRUD（列表/创建/修改/删除）
- [x] 角色-权限多对多绑定
- [x] 内置角色保护（admin 禁删）
- [x] 前端权限分配 UI（多选）

## 阶段C：用户与部门
- [x] 用户 CRUD
- [x] 用户启停（`isActive`）
- [x] 重置密码（更新时传入 password）
- [x] 用户-角色绑定
- [x] 用户-部门绑定
- [x] 部门 CRUD 与成员绑定

## 阶段D：项目与任务核心
- [x] 项目 CRUD
- [x] 项目-用户（负责人）多对多绑定
- [x] 项目-部门多对多绑定
- [x] 任务 CRUD
- [x] 任务状态机制（pending/queued/processing/completed）
- [x] 任务父子结构（parentId）
- [x] 任务多人指派（assigneeIds）
- [x] 创建任务默认创建人为当前用户
- [x] 任务编号唯一（可自动生成）
- [x] 任务时间按 RFC3339 存储

## 阶段E：进度、统计、个人工作
- [x] 进度列表聚合（按状态）
- [x] 仪表盘统计（用户/项目/任务/完成率）
- [x] 个人工作台（我的任务/我的创建/我的参与）

## 阶段F：甘特图与项目分解树
- [x] 甘特图数据接口 + 前端展示
- [x] 项目分解树接口 + 前端展示
- [x] 甘特图/任务树一致性自动化校验（测试）

## 阶段G：审计、文档、部署
- [x] 审计日志写入（关键创建/修改/删除）
- [x] 审计日志查询（支持分页与过滤）
- [x] API 文档维护（`docs/API.md` + `backend/docs/openapi.yaml`）
- [x] Docker Compose 部署
- [x] Helm 部署模板
- [x] 真机部署验收（compose 一次启动 + 健康检查 + 登录验证）
- [ ] k8s 真机部署验收（helm install 到真实集群）
- [x] 本地自动验收脚本（`scripts/verify-deploy.sh`）
- [x] Compose 启动回退脚本（`scripts/compose-up.sh`，自动选择可用 MySQL 镜像）

## 后续优化（下一迭代）
- [ ] 统一前端组件抽象（表单/列表/分页）
- [ ] 后端集成测试：登录->鉴权->CRUD->审计全链路
- [ ] 导出能力（任务/项目 CSV）
- [ ] 消息通知（站内通知）
