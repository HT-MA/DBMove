# DBMove — 数据库迁移 Web 平台

> Version 0.1.0 (MVP) · [PRD](./PRD.md)

DBMove 是一个给开发人员使用的 Web 数据库迁移工具：通过浏览器完成 **数据库 A → 数据库 B** 的全量数据迁移，不需要用户执行任何 `mysqldump` / `pg_dump` 之类的命令行。

```
开发人员 → Web UI → 选择源/目标 → Full Migration → 查看进度/日志 → 完成
```

DBMove 不重新实现数据库迁移算法，而是作为**统一 Web 管理层 + 任务调度层**，调用成熟的开源迁移工具（`mysqldump`/`mysql`、`pg_dump`/`pg_restore`）。

---

## 功能特性

- 连接管理：添加 / 编辑 / 删除 / 测试 MySQL、PostgreSQL 连接
- 密码安全：AES-256-GCM 加密存储，API 永不返回密码，日志自动脱敏
- 迁移任务：创建 / 启动 / 取消 / 重试，状态机 `PENDING → PREPARING → RUNNING → SUCCESS/FAILED/CANCELLED`
- 多数据库迁移：一次任务可迁移多个数据库（`source_db1 → target_db1`、`source_db2 → target_db2` …），进度跨库汇总
- 兼容性校验：任务创建时强制源/目标类型一致；Preflight 检查源/目标数据库版本兼容性（目标不得比源旧，pg_dump 客户端必须不低于源版本），不兼容则禁止迁移
- 异步执行：迁移 Worker 运行在独立容器 / Kubernetes Job 中，不阻塞 API
- 实时监控：SSE 实时日志 + 进度轮询（进度 / 表 / 行数 / 数据量 / 速度）
- 目标库策略：`error`（默认，存在即拒绝）/ `create`（缺失自动创建）/ `overwrite`（存在则覆盖）
- 并发控制：`MAX_CONCURRENT_MIGRATIONS`（默认 3），超出任务排队等待
- 深色 / 浅色主题

## 架构

```
Browser (React + TS + Ant Design)
    │  HTTP / SSE
    ▼
Go Backend (Gin + GORM + PostgreSQL)
    ├── Connection API / Migration API / Task Manager
    ├── Execution Mode: docker | kubernetes | local
    ▼
Migration Worker (独立进程/容器)
    ├── MySQL Engine   → mysqldump + mysql
    └── PostgreSQL Engine → pg_dump + pg_restore
```

真正耗时的迁移永远不在 API 进程里执行：

```
API → 创建 Worker 容器/Job → 立即返回 Task ID
Worker → 拉取任务配置 → Preflight → Dump → Restore → 上报状态/进度/日志
```

## 技术栈

| 层 | 技术 |
| --- | --- |
| Frontend | React 19 · TypeScript · Vite · Ant Design 5 · React Router · Axios |
| Backend | Go · Gin · GORM · PostgreSQL 16 |
| Worker | Go · `mysqldump`/`mysql` · `pg_dump`/`pg_restore` |
| Execution | Docker 容器 / Kubernetes Job（可选） |
| Realtime | Server-Sent Events (SSE) |
| Deploy | Docker Compose · Helm Chart |

## 目录结构

```text
dbmove/
├── backend/          # Go API 服务
│   ├── cmd/server/   # 入口
│   ├── internal/     # api / service / runner / dispatcher / model / repository / crypto
│   └── migrations/   # 参考 SQL
├── worker/           # 迁移 Worker（独立二进制）
│   ├── cmd/worker/
│   └── internal/     # engine (mysql/pg) / executor / reporter
├── frontend/         # React Web UI
├── deploy/           # docker-compose、demo 数据库、Helm Chart
├── scripts/verify.ps1
├── Makefile
└── README.md
```

## 环境要求

- Docker Desktop（含 docker compose）
- 可选：Kubernetes（`execution mode=kubernetes` 时）、Helm 3
- 本地开发：Go 1.24+、Node.js 20+、PostgreSQL 16（可选，compose 已提供）

## 快速开始（Docker Compose）

```bash
# 仅启动平台（Web + API + PostgreSQL）
docker compose up -d --build

# 含演示数据库（mysql/pg 的 source + target），用于验证迁移
docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d --build
```

启动后：

- Web UI: <http://localhost:3000>
- API: <http://localhost:8080>（healthz: <http://localhost:8080/healthz>）

> Docker 执行模式下，API 容器通过挂载的 `/var/run/docker.sock` 启动 `dbmove-worker:local` 容器来执行迁移，因此 API 容器必须能访问 Docker daemon。

## 端到端验证

```bash
docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d --build
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
```

验证脚本会自动：

1. 创建 4 个连接（mysql-source / mysql-target / pg-source / pg-target）
2. 测试连接
3. 执行 MySQL `source_db → target_db` 全量迁移
4. 执行 PostgreSQL `source_db → target_db` 全量迁移
5. 校验目标库行数与源库一致（users=3, orders=5）

多数据库迁移验证：

```bash
powershell -ExecutionPolicy Bypass -File scripts/verify-multi.ps1
```

该脚本会执行 MySQL（`source_db→target_db` + `sales_db→sales_db2`）与 PostgreSQL（`source_db→target_db` + `analytics_db→analytics_db2`）两组双库迁移，并校验所有目标库数据一致。

也可以通过 Web UI 手动验证：**Connections → New Connection → Test → Migrations → New Migration → Start → 查看实时日志与进度**。

> 注意：验证脚本面向全新环境（先 `docker compose -f docker-compose.yml -f docker-compose.demo.yml down -v` 清空平台库与演示库）。

## Kubernetes 部署（Helm）

前提：集群中已有 PostgreSQL（或在 `values.yaml` 指定外部数据库），迁移目标数据库在集群网络内可达。

```bash
# 构建并推送镜像（示例）
docker build -t dbmove-api:latest backend
docker build -t dbmove-web:latest frontend
docker build -t dbmove-worker:latest worker

# 配置
helm install dbmove deploy/helm/dbmove \
  --namespace dbmove --create-namespace \
  --set api.databaseUrl='postgres://dbmove:pass@pg-host:5432/dbmove?sslmode=disable' \
  --set api.encryptionKey='<base64 32字节密钥>' \
  --set api.internalToken='<内部通信令牌>' \
  --set image.repository=myregistry/dbmove
```

RBAC 遵循最小权限：只允许 `get/create/delete jobs`、`get/list/watch pods`、`get pods/log`、`create/delete secrets`，不授予 `cluster-admin`。

## 配置项（环境变量）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DBMOVE_HTTP_ADDR` | `:8080` | API 监听地址 |
| `DBMOVE_DATABASE_URL` | `postgres://...` | 平台自身 PostgreSQL DSN |
| `DBMOVE_ENCRYPTION_KEY` | 必填 | AES-256-GCM 密钥（32 字节，base64 或原始字符串） |
| `DBMOVE_EXECUTION_MODE` | `docker` | `docker` / `kubernetes` / `local` |
| `DBMOVE_WORKER_IMAGE` | `dbmove-worker:local` | Worker 镜像名 |
| `DBMOVE_API_URL` | `http://localhost:8080` | Worker 访问 API 的地址 |
| `DBMOVE_DOCKER_NETWORK` | 空 | docker 模式下 Worker 容器加入的 compose 网络 |
| `DBMOVE_MAX_CONCURRENT_MIGRATIONS` | `3` | 最大并发迁移数 |
| `DBMOVE_DATA_DIR` | `/data` | dump 文件目录 |
| `DBMOVE_INTERNAL_TOKEN` | 空 | Worker↔API 内部认证令牌（建议设置） |
| `DBMOVE_CORS_ORIGINS` | `*` | 允许的跨域来源 |
| `DBMOVE_K8S_NAMESPACE` | `dbmove` | k8s 模式下 Job 命名空间 |
| `DBMOVE_K8S_JOB_*` | — | Worker Job 资源 requests/limits/TTL |

## 平台数据库迁移

后端启动时通过 GORM `AutoMigrate` 自动建表/升级表结构；参考 SQL 见 [backend/migrations/001_init.sql](backend/migrations/001_init.sql)。

## 支持的数据库与迁移方式

### MVP（当前）

- MySQL 8.x → MySQL 8.x（`mysqldump` + `mysql`）
- PostgreSQL 14+ → PostgreSQL 14+（`pg_dump -Fc` + `pg_restore`）
- 迁移方式：`FULL`（全量）

### 后续阶段（PRD）

- DM8、Redis、MySQL↔PostgreSQL 跨库迁移
- `SCHEMA_ONLY` / `DATA_ONLY` / `FULL_INCREMENTAL`（CDC）

## API 文档

统一响应格式：

```json
{ "success": true, "data": {} }
{ "success": false, "error": { "code": "CONNECTION_FAILED", "message": "..." } }
```

### 连接管理

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/connections` | 创建连接 |
| GET | `/api/v1/connections` | 连接列表 |
| GET | `/api/v1/connections/:id` | 连接详情 |
| PUT | `/api/v1/connections/:id` | 更新（密码留空则不变） |
| DELETE | `/api/v1/connections/:id` | 删除（被任务引用时拒绝） |
| POST | `/api/v1/connections/test` | 测试未保存的连接配置 |
| POST | `/api/v1/connections/:id/test` | 测试已保存连接 |
| GET | `/api/v1/connections/:id/databases` | 列出该连接可见的数据库 |

### 迁移任务

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/migrations` | 创建任务（返回 `{id, status}`） |
| GET | `/api/v1/migrations?page&page_size&status` | 任务列表 |
| GET | `/api/v1/migrations/:id` | 任务详情 |
| POST | `/api/v1/migrations/:id/start` | 启动（排队执行） |
| POST | `/api/v1/migrations/:id/cancel` | 取消 |
| POST | `/api/v1/migrations/:id/retry` | 重试（仅 FAILED） |
| GET | `/api/v1/migrations/:id/logs` | 历史日志 |
| GET | `/api/v1/migrations/:id/logs/stream` | SSE 实时日志 |
| GET | `/api/v1/migrations/:id/progress` | 实时进度 |
| GET | `/api/v1/stats` | Dashboard 统计 |

### 内部接口（Worker 使用，需 `DBMOVE_INTERNAL_TOKEN`）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/internal/tasks/:id` | 拉取任务配置（不含密码） |
| POST | `/api/v1/internal/tasks/:id/status` | 上报状态 |
| POST | `/api/v1/internal/tasks/:id/progress` | 上报进度 |
| POST | `/api/v1/internal/tasks/:id/logs` | 批量上报日志 |

密码通过环境变量（Docker）或 Kubernetes Secret（k8s）注入 Worker，不经过内部 API。

创建迁移任务的数据库映射（单库兼容旧字段）：

```json
{
  "name": "multi-db-migration",
  "source_connection_id": 1,
  "target_connection_id": 2,
  "databases": [
    { "source": "source_db", "target": "target_db" },
    { "source": "sales_db", "target": "sales_db2" }
  ],
  "migration_type": "FULL",
  "target_db_policy": "create"
}
```

`databases` 省略时继续使用 `source_database` / `target_database` 单库字段；两者都提供时以 `databases` 为准。

## 错误码

`CONNECTION_FAILED` · `AUTH_FAILED` · `DATABASE_NOT_FOUND` · `PERMISSION_DENIED` · `DISK_SPACE_INSUFFICIENT` · `MIGRATION_FAILED` · `MIGRATION_NOT_FOUND` · `MIGRATION_ALREADY_RUNNING` · `MIGRATION_CANCELLED` · `ENGINE_NOT_FOUND` · `INVALID_DATABASE_TYPE` · `UNSUPPORTED_MIGRATION` · `INVALID_STATE` · `CONNECTION_IN_USE` · `INVALID_INPUT`

## 安全设计

- 密码 AES-256-GCM 加密存储（`DBMOVE_ENCRYPTION_KEY`），API 响应不含密码
- Worker 日志与后端日志对密码、DSN 中的凭据自动脱敏
- Worker 容器/K8s Secret 在任务结束后清理
- 前端密码输入框使用 password 类型

## 常见问题（Troubleshooting）

**Worker 容器无法启动 / 找不到镜像**
确认 `dbmove-worker:local` 已构建（`docker compose build worker` 或 `make build`），且 API 容器能访问 Docker daemon（挂载了 docker.sock）。

**Worker 连不上数据库**
确认 `docker-compose.demo.yml` 已启用（演示库在同一 compose 网络），或目标数据库对 Worker 容器网络可达；检查迁移日志中的连接错误。

**PostgreSQL 目标库报 "database does not exist"**
选择 `target_db_policy=create`（缺失自动创建）或 `overwrite`。

**目标库已存在但任务失败**
默认策略 `error` 会拒绝覆盖已存在的目标库；在新建迁移时选择 `overwrite` 并确认覆盖。

**MySQL dump 报权限错误**
`--routines/--triggers` 需要相应权限；生产账号权限不足时建议使用 root 或最小化 dump 选项。

**SSE 日志不刷新**
确认 nginx `proxy_buffering off` 已生效（`docker-compose.yml` 内 web 已内置该配置）。

**PostgreSQL restore 报 `unrecognized configuration parameter "transaction_timeout"`**
这是 pg_dump 17+ 客户端与 16 及以下服务器的已知兼容问题。DBMove 的 Worker 镜像固定使用 PostgreSQL 16 客户端；若源/目标为 PostgreSQL 17，请用对应版本的 `postgresql-client` 重新构建 Worker 镜像（参考 `worker/Dockerfile`）。

## 本地开发

```bash
# 1. 启动平台库与演示库
docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d postgres mysql-source mysql-target pg-source pg-target

# 2. 后端（本机运行，使用 local 执行模式时需先构建 worker 二进制）
cd backend && go run ./cmd/server

# 3. Worker（可选，local 模式）
cd worker && go build -o dbmove-worker ./cmd/worker

# 4. 前端
cd frontend && npm install && npm run dev
```

> 本地 `go run` 时默认 `DBMOVE_EXECUTION_MODE=docker`，需要配置 `DBMOVE_API_URL=http://localhost:8080` 等变量；开发期也可 `DBMOVE_EXECUTION_MODE=local` 直接跑 Worker 子进程。

## 测试

```bash
# 后端单测（crypto / redact / model / service）
cd backend && go test ./...

# Worker 单测（executor / engine / redact）
cd worker && go test ./...

# 前端单测（utils / components，Vitest）
cd frontend && npm test

# 或一键全部
make test
```

端到端验证（需要 Docker 与演示库）：

```bash
make demo-up
make verify        # 单库迁移
make verify-multi  # 多库迁移
```

## 开发指南

- 新增数据库类型：在 `worker/internal/engine` 实现 `Engine` 接口并 `Register`；在 backend `model` 中放开连接类型校验；backend `service` 的 `engineForType` 增加映射。
- 新增执行模式：实现 `runner.Runner` 接口，在 `cmd/server/main.go` 的 switch 中注册。
- 新增迁移方式：扩展 `MigrationType`、Worker `TaskConfig` 与前端表单。

## 路线图（PRD Phase）

1. ✅ Phase 1 基础框架（React + Go + PostgreSQL + Docker Compose）
2. ✅ Phase 2 Connection（CRUD + Test + 加密）
3. ✅ Phase 3 Migration（Full Migration + Worker + Docker/K8s Job + MySQL/PG Engine）
4. ✅ Phase 4 Monitoring（详情 / 进度 / SSE 日志 / Cancel / Retry）
5. ⏳ Phase 5 DM8 / Redis Engine
6. ⏳ Phase 6 高级功能（Schema Only / Data Only / CDC / 定时迁移等）
