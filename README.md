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
- 任务列表 / Kanban 看板
- 迭代管理（Sprint/Cycle、任务范围、完成率、Kanban 按迭代筛选）
- 我的日程（月/周/日视图、iCal 导出）
- 请求入口（项目申请 / 任务请求 / 缺陷问题 / 变更申请）
- 项目模板（任务树 / 里程碑 / 相对排期 / 模板内依赖复制）
- 自动化规则（逾期任务定时提醒 / 状态、进度与执行人变更自动通知和评论 / 自动添加标签 / 自动指派执行人 / 调用 Webhook / 手动执行 / 执行日志）
- Webhook 订阅（任务状态变更事件、投递日志、失败重试）
- API Token / 服务账号（最小权限、独立禁用/撤销、审计）
- 任务更新/删除
- 任务评论 / @提及 / 活动流
- 进度列表
- 统计分析
- 项目健康度 Dashboard
- 资源容量与工时管理（任务估算/实际/剩余工时、用户周容量、本周成员负载）
- 报表中心（保存项目健康、成员负载、任务状态报表配置）
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
- 项目模板保存独立任务树 JSON，生成项目时映射为真实任务父子关系与依赖
- 自动化逾期提醒会按规则通知任务执行人与项目负责人，并记录每次执行日志
- 自动化状态/进度/执行人变更规则会在任务写事务内添加评论、通知执行人/项目负责人、追加缺失标签、追加缺失执行人，并记录事件执行日志
- 自动化 Webhook 会在规则事务提交后向每个匹配任务发送 JSON POST；失败会写入执行日志但不会回滚任务更新或其他自动化动作
- Webhook 订阅会在任务状态变更事务提交后向外部系统发送 JSON POST；失败会写入投递日志并支持手动重试
- API Token 以服务账号身份调用接口，权限由独立 Token 权限清单和服务账号角色共同约束；Token 只保存哈希，明文仅创建时返回一次
- 任务工时字段必须为非负数；用户默认周容量范围为 `0-168` 小时，Dashboard 按当前用户可见任务聚合本周成员负载并标记过载
- 迭代任务范围通过独立关联表维护，不写入任务主表；普通用户只能查看自己创建或包含自己可见任务的迭代

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
GOPROXY=https://goproxy.cn,direct \
NPM_REGISTRY=https://registry.npmmirror.com \
docker compose up -d --build
```
后端 Dockerfile 同时兼容旧变量 `GO_PROXY`，但直接执行 `docker build` 时需要显式传入构建参数，例如：
```bash
docker build --build-arg GOPROXY=https://goproxy.cn,direct -f backend/Dockerfile backend
# 兼容旧变量名：
docker build --build-arg GO_PROXY=https://goproxy.cn,direct -f backend/Dockerfile backend
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

### 5) 自动化规则 Webhook
规则动作中启用“调用 Webhook”后，系统会在规则事务提交后向 `webhookUrl` 发送 JSON POST，内容包含规则、触发器、任务和变更摘要。

安全默认值：
- 仅允许 `http` / `https` URL。
- 默认拒绝 localhost、内网和保留地址，降低 SSRF 风险。
- 如测试环境或内网部署确需调用私有地址，可在 `.env` 开启：
```bash
AUTOMATION_WEBHOOK_ALLOW_PRIVATE=true
```
- Webhook 失败会把对应执行日志标记为失败并记录原因，不会回滚任务更新、标签追加或执行人追加。

### 6) Webhook 订阅
“Webhook 订阅”模块用于让外部系统订阅项目管理事件。当前支持 `task_status_changed`，任务状态通过 `PUT /tasks/:id`、`PATCH /tasks/:id/status` 或 `PATCH /tasks/:id/complete` 变更后，会在任务事务提交后发送 JSON POST。

说明：
- 订阅 URL 使用与自动化 Webhook 相同的安全校验，默认禁止本机、内网和保留地址。
- 管理员创建的订阅接收全量任务状态事件；普通用户创建的订阅仅接收其作为创建人、执行人或审核人可见任务的事件。
- 每次投递都会写入投递日志，失败记录可在页面上手动重试。
- 如测试或内网部署确需回调私有地址，可配置：
```bash
AUTOMATION_WEBHOOK_ALLOW_PRIVATE=true
```

### 7) API Token / 服务账号
“API Token”模块用于外部系统以服务账号身份调用开放接口。Token 创建时会自动生成服务账号和独立角色，调用接口时仍走现有 RBAC、任务/项目可见范围和审计链路。

说明：
- Token 明文只在创建成功时返回一次，数据库仅保存哈希、前缀和后四位。
- Token 权限必须显式选择；实际权限由 Token 权限清单与服务账号角色权限共同约束。
- 支持独立停用、重新启用和撤销；停用或撤销后，Bearer Token 会返回未授权。
- 使用方式：
```bash
curl -H "Authorization: Bearer pmt_xxx" http://localhost:8080/api/v1/projects
```

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
- 数据库迁移：`backend/migrations/`（评论/活动流、请求/模板、自动化、资源容量与工时字段、保存报表已提供显式 SQL；本地测试仍通过 GORM AutoMigrate 初始化）

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
