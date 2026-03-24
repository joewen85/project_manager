# Project Manager (SME)

基于中小企业项目管理场景的前后端分离系统：
- 后端：Go + Gin + GORM + JWT + MySQL
- 前端：Vite + React + TypeScript
- 部署：Docker Compose、Kubernetes Helm

## 功能模块
- RBAC 权限管理
- 用户管理
- 部门管理
- 项目列表 / 项目详情
- 任务列表
- 任务更新/删除
- 进度列表
- 统计分析
- 个人工作（我的任务 / 我的创建 / 我的参与）
- 甘特图
- 项目分解树结构（任务树）
- 操作审计日志

## 关键业务约束
- 创建任务时 `creatorId` 默认当前登录用户
- 用户体系由 RBAC 权限控制
- 任务编号 `taskNo` 唯一
- 任务开始/结束时间支持自定义日期时间
- 用户、项目、任务、部门通过多对多关系建模

## 分阶段开发计划（Plan）
1. 需求拆解与架构设计
2. 后端基础能力（鉴权、模型、RBAC）
3. 业务 API（用户/部门/项目/任务/统计）
4. 前端管理后台页面与可视化
5. API 文档与部署（compose / helm）
6. 联调与完善（分页、搜索、审计日志、通知）

## 快速启动（Docker Compose）
```bash
docker compose up -d --build
```
- MySQL: `localhost:3306`
- Backend: `http://localhost:8080`
- Frontend: `http://localhost:5173`

默认管理员：
- `admin / admin123`

若遇到 MySQL 镜像拉取失败（网络/TLS/EOF），可使用自动回退脚本：
```bash
bash scripts/compose-up.sh
```
或手动指定镜像：
```bash
MYSQL_IMAGE=mysql:8.0 docker compose up -d --build
```

## 本地开发
### 后端
```bash
cd backend
go run ./cmd/server
```

### 前端
```bash
cd frontend
npm install
npm run dev
```

## 项目计划与进度
- 计划：`PLAN.md`
- 进度：`PROGRESS.md`

## API 文档
- 简版文档：`docs/API.md`
- OpenAPI：`backend/docs/openapi.yaml`

## Helm 部署
```bash
helm upgrade --install project-manager ./deploy/helm/project-manager
```

## 一键验收
```bash
bash scripts/verify-deploy.sh
```
