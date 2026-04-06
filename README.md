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
- 站内通知（未读计数 / 已读管理）
- 甘特图独立模块（单项目/项目集、里程碑、依赖拖拽、自动顺延、冲突与缓冲分析）
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
先创建环境变量文件：
```bash
cp .env.template .env
```
如前后端端口不是默认值，记得在 `.env` 配置：
- `VITE_API_BASE_URL`（容器部署建议 `/api/v1`）
- `CORS_ALLOW_ORIGINS`（HTTPS 入口默认需包含 `https://localhost:5443`）
- `UPLOAD_DIR`（本地开发建议 `../static/uploads`，容器由 compose 固定到 `/app/static/uploads`）
- `UPLOAD_PUBLIC_BASE`（默认 `/static/uploads`）
- `FRONTEND_HTTPS_PORT`（线上建议 `443`）
- `FRONTEND_SSL_DIR`（证书目录，默认 `./deploy/ssl`）
- `FRONTEND_SSL_CERT_FILE` / `FRONTEND_SSL_KEY_FILE`（证书文件名）

```bash
docker compose up -d --build
```
- MySQL: `localhost:3306`
- Backend: `http://localhost:8080`
- Frontend HTTP: `http://localhost:5173`（自动跳转 HTTPS）
- Frontend HTTPS: `https://localhost:5443`

若登录接口返回 `POST 403`，请检查：
- `.env` 中 `VITE_API_BASE_URL` 建议为 `/api/v1`
- `.env` 中 `CORS_ALLOW_ORIGINS` 包含 `https://localhost:5443`

默认管理员：
- `admin / admin123`

若遇到 MySQL 镜像拉取失败（网络/TLS/EOF），可使用自动回退脚本：
```bash
bash scripts/compose-up.sh
```
或手动指定镜像：
```bash
MYSQL_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/mysql:lts docker compose up -d --build
```
如构建阶段访问 Docker Hub 受限，可一并指定构建基础镜像：
```bash
GO_BUILDER_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/golang:1.25-alpine \
APP_RUNTIME_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/alpine:latest \
NODE_BUILDER_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/node:24-alpine \
NGINX_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/nginx:alpine \
GO_PROXY=https://goproxy.cn,direct \
NPM_REGISTRY=https://registry.npmmirror.com \
docker compose up -d --build
```
如本机端口冲突，可指定主机端口：
```bash
MYSQL_PORT=3307 BACKEND_PORT=8081 FRONTEND_PORT=5174 FRONTEND_HTTPS_PORT=5444 docker compose up -d --build
```

## 线上 HTTPS 证书
将证书放到 `deploy/ssl`（或通过 `.env` 设置 `FRONTEND_SSL_DIR`）并使用文件名：
- `fullchain.pem`
- `privkey.pem`

线上一般建议：
```bash
FRONTEND_PORT=80 FRONTEND_HTTPS_PORT=443 FRONTEND_SSL_DIR=./deploy/ssl docker compose up -d --build
```

如果证书文件名不是上述默认值（例如 `www.yunstlm.com.pem` / `www.yunstlm.com.key`），可配置：
```bash
FRONTEND_SSL_CERT_FILE=www.yunstlm.com.pem
FRONTEND_SSL_KEY_FILE=www.yunstlm.com.key
```

## 本地开发
在项目根目录先准备环境文件：
```bash
cp .env.template .env
```
并确保上传目录存在（项目根目录）：
```bash
mkdir -p static/uploads
```

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
开发模式下，`/static/uploads` 会由 Vite 代理到后端；容器模式下 `./static/uploads` 会挂载到后端容器 `/app/static/uploads`。
项目与任务编辑表单支持：上传多个文件、上传文件夹、拖放上传；上传文件夹时会自动压缩为 ZIP 附件。

## 任务通知配置（企业微信 / 钉钉 / 邮件 / 飞书）
系统已支持任务通知多渠道推送：
- 邮件通知（SMTP）
- 企业微信任务通知（应用消息）
- 钉钉任务通知（机器人 Webhook）
- 飞书任务通知（服务端发送消息）

配置原则：
- 邮件通知可单独开启，也可与其它渠道同时开启。
- 企业微信 / 钉钉 / 飞书 三者只能配置一种（多配时将跳过非邮件渠道发送）。
- 任何渠道若所需环境变量缺失或为空，则该渠道不会发送通知。
- 可通过 `TASK_NOTIFY_PROVIDER` 显式指定非邮件渠道（推荐）。

推荐先配置：
```bash
TASK_NOTIFY_PROVIDER=wecom
# 可选值：wecom / dingtalk / feishu / none
# 留空时为自动识别（如同时配置多个非邮件渠道会跳过发送）
```

### 1) 邮件通知
在 `.env` 配置：
```bash
SMTP_HOST=smtp.example.com
SMTP_PORT=25
SMTP_USERNAME=your_user
SMTP_PASSWORD=your_password
SMTP_FROM=no-reply@example.com
```
说明：
- 仅当 `SMTP_HOST`、`SMTP_PORT`、`SMTP_FROM` 非空时启用。
- 默认按任务执行人的用户邮箱发送。

### 2) 企业微信任务通知（应用消息）
在 `.env` 配置：
```bash
WECOM_CORP_ID=wwxxxxxxxxxxxxxxxx
WECOM_CORP_SECRET=xxxxxxxxxxxxxxxx
WECOM_AGENT_ID=1000002
WECOM_TO_USER=@all
```
说明：
- `WECOM_TO_USER` 支持 `@all` 或 `userid1|userid2`。
- 仅当 `WECOM_CORP_ID`、`WECOM_CORP_SECRET`、`WECOM_AGENT_ID` 非空时启用。

### 3) 钉钉任务通知（机器人 Webhook）
在 `.env` 配置：
```bash
DINGTALK_WEBHOOK=https://oapi.dingtalk.com/robot/send?access_token=xxxx
DINGTALK_SECRET=SECxxxxxxxxxxxxxxxx
```
说明：
- `DINGTALK_SECRET` 可选；如果机器人开启加签，必须配置。
- 仅当 `DINGTALK_WEBHOOK` 非空时启用。

### 4) 飞书任务通知（服务端消息）
在 `.env` 配置：
```bash
FEISHU_APP_ID=cli_xxxxxxxxxxxxxxxx
FEISHU_APP_SECRET=xxxxxxxxxxxxxxxx
FEISHU_RECEIVE_ID_TYPE=email
FEISHU_RECEIVE_ID=
```
说明：
- 仅当 `FEISHU_APP_ID`、`FEISHU_APP_SECRET` 非空时启用。
- `FEISHU_RECEIVE_ID` 留空且 `FEISHU_RECEIVE_ID_TYPE=email` 时，按任务执行人邮箱逐个发送。
- 若需固定接收者（如群），可设置 `FEISHU_RECEIVE_ID`（并将 `FEISHU_RECEIVE_ID_TYPE` 设为对应类型，如 `chat_id`）。

### 通知自检（推荐）
配置完 `.env` 后，可直接执行：
```bash
bash scripts/test-task-notify.sh auto
```
或使用 Make 一键执行：
```bash
make notify-test
```
也可按渠道单测：
```bash
bash scripts/test-task-notify.sh email
bash scripts/test-task-notify.sh wecom
bash scripts/test-task-notify.sh dingtalk
bash scripts/test-task-notify.sh feishu
```
或：
```bash
make notify-test PROVIDER=email
make notify-test PROVIDER=wecom
make notify-test PROVIDER=dingtalk
make notify-test PROVIDER=feishu
```
说明：
- `auto` 会先测邮件（若已配置），再按 `TASK_NOTIFY_PROVIDER` 或自动识别测试一个非邮件渠道。
- 若邮件自检需要单独收件地址，可在 `.env` 配置 `SMTP_TEST_TO`（仅脚本使用，不影响业务发送）。

## 项目计划与进度
- 计划：`PLAN.md`
- 进度：`PROGRESS.md`

## API 文档
- 简版文档：`docs/API.md`
- OpenAPI：`backend/docs/openapi.yaml`
- RBAC 权限清单：`docs/RBAC_PERMISSIONS.md`
- 前端 UI 规范：`docs/FRONTEND_UI_GUIDE.md`
- 前端 UI 验收清单：`docs/UI_QA_CHECKLIST.md`

## Helm 部署
```bash
helm upgrade --install project-manager ./deploy/helm/project-manager
```

## 一键验收
```bash
bash scripts/verify-deploy.sh
```

## API 基准测试（可选）
采集当前环境 benchmark：
```bash
LABEL=after RUNS=5 OUT_FILE=docs/benchmark/after.md bash scripts/benchmark-api.sh
```

生成前后对比报告：
```bash
BEFORE_FILE=docs/benchmark/before.md AFTER_FILE=docs/benchmark/after.md OUT_FILE=docs/benchmark/compare.md bash scripts/benchmark-compare.sh
```
