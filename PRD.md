# DBMove 数据库迁移 Web 平台

> Version: 1.0
> Status: MVP Development
> Project Name: DBMove
> Purpose: 为开发人员提供一个简单易用的 Web 数据库迁移工具，通过浏览器完成数据库 A → 数据库 B 的数据迁移，不要求用户使用命令行。

---

# 1. 项目背景

公司测试环境存在大量 Kubernetes / K3s 集群，每个集群中通过 Namespace 运行大量数据库服务，包括：

* MySQL
* PostgreSQL
* DM8
* Redis
* 其他数据库

当前数据库迁移主要依赖命令行工具，例如：

* mysqldump
* mydumper / myloader
* pg_dump / pg_restore
* DM8 官方工具
* Redis 工具

开发人员通常不熟悉这些命令，或者不希望通过命令行执行数据库迁移。

因此开发一个统一的 Web 数据库迁移工具：

```text
开发人员
    ↓
Web UI
    ↓
选择源数据库
    ↓
选择目标数据库
    ↓
选择迁移方式
    ↓
开始迁移
    ↓
查看进度 / 日志
    ↓
迁移完成
```

DBMove 不负责重新实现数据库迁移算法，而是作为统一的 Web 管理层和任务调度层，调用成熟的开源数据库迁移工具。

---

# 2. 项目目标

## 2.1 核心目标

实现一个简单、稳定、易用的 Web 数据库迁移平台。

用户不需要执行任何数据库迁移命令，只需要：

1. 添加数据库连接
2. 测试数据库连接
3. 创建迁移任务
4. 选择 Source Database
5. 选择 Target Database
6. 选择迁移模式
7. 点击开始
8. 查看迁移进度
9. 查看日志
10. 查看最终结果

---

# 3. MVP 范围

第一版本只实现以下功能。

## 3.1 数据库类型

MVP 优先支持：

* MySQL
* PostgreSQL

第二阶段支持：

* DM8
* Redis

不要在 MVP 阶段实现其他数据库。

---

# 4. MVP 支持的迁移方式

第一版本：

```text
Full Migration
```

第二阶段：

```text
Schema Only
Data Only
Full + Incremental
```

MVP 不需要实现复杂 CDC。

---

# 5. 非目标

第一版本明确不要实现：

* 数据库备份平台
* 数据库监控平台
* 数据库管理平台
* SQL 在线编辑器
* SQL 执行平台
* 数据库资产管理
* Kubernetes 集群管理
* GitLab 管理
* Argo CD 管理
* 环境 Clone
* 数据脱敏
* 数据同步拓扑
* 数据库高可用
* 自动数据库部署

DBMove 只负责：

> Database A → Database B

---

# 6. 核心使用场景

## 场景 1：MySQL → MySQL

```text
MySQL A
    ↓
DBMove
    ↓
MySQL B
```

例如：

```text
test-a / mysql
        ↓
test-b / mysql
```

---

## 场景 2：PostgreSQL → PostgreSQL

```text
PostgreSQL A
    ↓
DBMove
    ↓
PostgreSQL B
```

---

## 场景 3：MySQL → PostgreSQL

如果底层迁移引擎支持，则允许：

```text
MySQL
  ↓
PostgreSQL
```

但 MVP 优先保证同类型数据库迁移稳定。

---

# 7. 总体架构

```text
                         Browser
                            │
                            ▼
                  ┌──────────────────┐
                  │ React + TypeScript│
                  │    Ant Design    │
                  └────────┬─────────┘
                           │
                         HTTP
                           │
                           ▼
                  ┌──────────────────┐
                  │    Go Backend     │
                  ├──────────────────┤
                  │ Connection API    │
                  │ Migration API     │
                  │ Task Manager      │
                  │ Engine Manager    │
                  │ Log Manager       │
                  └────────┬─────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
        PostgreSQL      Kubernetes   Migration
        DBMove DB          Job        Engines
                                         │
                              ┌──────────┼──────────┐
                              │          │          │
                           SeaTunnel  mydumper   pg_dump
```

---

# 8. 技术栈

## 8.1 Frontend

使用：

```text
React
TypeScript
Vite
Ant Design
React Router
Axios
```

推荐：

```text
React 19+
TypeScript
Vite
Ant Design 5+
```

---

# 9. Backend

使用：

```text
Go
Gin
GORM
PostgreSQL
```

推荐：

```text
Go 1.24+
Gin
GORM
PostgreSQL 16+
```

后端负责：

* REST API
* 数据库连接管理
* 迁移任务管理
* 任务状态管理
* Migration Engine 调度
* Kubernetes Job 创建
* 日志获取
* 任务取消
* 任务重试

---

# 10. 为什么 Backend 使用 Go

Go 适合这个项目，因为：

1. 项目本质是 DevOps 工具
2. 需要调用外部进程
3. 需要操作 Kubernetes API
4. 需要创建 Kubernetes Job
5. 并发任务处理简单
6. 部署非常简单
7. 最终可以编译成一个二进制文件

---

# 11. 为什么不自己开发迁移引擎

DBMove 不负责实现：

* MySQL Binlog 解析
* PostgreSQL WAL 解析
* 数据类型转换算法
* 数据复制算法
* 数据 Dump 算法

这些交给成熟工具。

DBMove 的定位：

```text
Web UI
    ↓
Task Management
    ↓
Migration Engine
```

---

# 12. Migration Engine 设计

定义统一接口：

```go
type MigrationEngine interface {
    TestConnection(ctx context.Context, conn *Connection) error

    Preflight(ctx context.Context, task *MigrationTask) error

    Migrate(ctx context.Context, task *MigrationTask) error

    Cancel(ctx context.Context, task *MigrationTask) error

    GetProgress(ctx context.Context, task *MigrationTask) (*Progress, error)
}
```

不同数据库实现不同 Engine。

```text
MigrationEngine
       │
       ├── MySQLMigrationEngine
       │
       ├── PostgreSQLMigrationEngine
       │
       ├── DM8MigrationEngine
       │
       └── RedisMigrationEngine
```

---

# 13. MySQL Migration Engine

MySQL 优先使用：

```text
mydumper
myloader
```

对于简单场景可以支持：

```text
mysqldump
mysql
```

推荐默认：

```text
mydumper → myloader
```

原因：

* 支持并行
* 大数据库性能更好
* 迁移速度更高
* 更适合生产测试环境大数据量迁移

流程：

```text
Source MySQL
     │
     ▼
mydumper
     │
     ▼
Migration Data
     │
     ▼
myloader
     │
     ▼
Target MySQL
```

---

# 14. PostgreSQL Migration Engine

使用：

```text
pg_dump
pg_restore
```

流程：

```text
Source PostgreSQL
       │
       ▼
    pg_dump
       │
       ▼
Migration File
       │
       ▼
   pg_restore
       │
       ▼
Target PostgreSQL
```

对于大数据库可以进一步支持：

```text
pg_dump -Fd
```

实现并行 Dump / Restore。

---

# 15. DM8 Migration Engine

第二阶段实现。

使用 DM8 官方提供的：

* DM Export
* DM Import

不要在 DBMove 中自行实现 DM8 数据格式解析。

DBMove 只负责：

```text
Generate Command
    ↓
Execute Job
    ↓
Collect Logs
    ↓
Report Status
```

---

# 16. Redis Migration Engine

第二阶段实现。

优先支持：

```text
Redis RDB
```

基本流程：

```text
Source Redis
    ↓
RDB
    ↓
Transfer
    ↓
Target Redis
```

暂不实现 Redis Cluster 数据迁移。

---

# 17. Kubernetes Job

所有真正耗时的迁移任务不要直接运行在 Go Backend 进程中。

推荐：

```text
Browser
   ↓
Go Backend
   ↓
Create Kubernetes Job
   ↓
Migration Worker Pod
   ↓
Execute Migration
```

例如：

```text
dbmove-migration-1024
        │
        └── Pod
             │
             └── dbmove-worker
```

---

# 18. 为什么使用 Kubernetes Job

优点：

* 任务隔离
* 资源限制
* 任务失败可以重试
* 日志独立
* 可以同时执行多个迁移任务
* Backend 不被长时间迁移任务阻塞
* 适合 K3s / Kubernetes 环境

---

# 19. Migration Worker

Worker 可以是一个独立 Go 程序：

```text
dbmove-worker
```

职责：

1. 接收任务配置
2. 初始化 Migration Engine
3. 执行迁移
4. 输出结构化日志
5. 更新任务状态
6. 更新进度
7. 迁移完成后退出

---

# 20. Worker 配置

Worker 不应该直接从前端获得数据库密码。

推荐：

```text
Backend
   ↓
Kubernetes Secret
   ↓
Migration Job
   ↓
Worker
```

数据库密码通过 Kubernetes Secret 注入。

---

# 21. 数据库连接模型

数据库连接包含：

```text
id
name
type
host
port
username
password
database
ssl_mode
description
created_at
updated_at
```

数据库类型：

```text
mysql
postgresql
dm8
redis
```

---

# 22. 密码安全

禁止明文保存密码。

推荐：

```text
AES-256-GCM
```

或者：

```text
Kubernetes Secret
```

MVP 可以：

```text
数据库密码加密存储在 PostgreSQL
```

同时在 API 返回时：

```text
password: 不返回
```

---

# 23. Connection API

## 创建连接

```http
POST /api/v1/connections
```

请求：

```json
{
  "name": "test-mysql",
  "type": "mysql",
  "host": "10.0.0.10",
  "port": 3306,
  "username": "root",
  "password": "password",
  "database": "order_db"
}
```

---

## 获取连接列表

```http
GET /api/v1/connections
```

---

## 获取连接详情

```http
GET /api/v1/connections/:id
```

---

## 删除连接

```http
DELETE /api/v1/connections/:id
```

---

## 测试连接

```http
POST /api/v1/connections/:id/test
```

返回：

```json
{
  "success": true,
  "version": "MySQL 8.0.36",
  "latency_ms": 12
}
```

---

# 24. Migration Task 模型

字段：

```text
id
name
source_connection_id
target_connection_id
source_database
target_database
migration_type
status
engine
progress
tables_total
tables_completed
rows_total
rows_completed
bytes_total
bytes_transferred
error_message
started_at
finished_at
created_by
created_at
updated_at
```

---

# 25. Migration Type

第一版：

```text
FULL
```

第二阶段：

```text
SCHEMA_ONLY
DATA_ONLY
FULL_INCREMENTAL
```

---

# 26. Migration Status

```text
PENDING
PREPARING
RUNNING
SUCCESS
FAILED
CANCELLED
```

状态转换：

```text
PENDING
   ↓
PREPARING
   ↓
RUNNING
   ├── SUCCESS
   ├── FAILED
   └── CANCELLED
```

---

# 27. 创建迁移任务 API

```http
POST /api/v1/migrations
```

请求：

```json
{
  "name": "order-db-migration",
  "source_connection_id": 1,
  "target_connection_id": 2,
  "source_database": "order_db",
  "target_database": "order_db",
  "migration_type": "FULL"
}
```

返回：

```json
{
  "id": 1024,
  "status": "PENDING"
}
```

---

# 28. 启动迁移

```http
POST /api/v1/migrations/:id/start
```

Backend：

```text
Migration Task
      ↓
Preflight
      ↓
Create Kubernetes Job
      ↓
Worker
      ↓
Running
```

---

# 29. 取消迁移

```http
POST /api/v1/migrations/:id/cancel
```

Backend 删除对应 Kubernetes Job。

---

# 30. 重试迁移

```http
POST /api/v1/migrations/:id/retry
```

只允许：

```text
FAILED
```

状态任务重试。

---

# 31. 获取迁移任务

```http
GET /api/v1/migrations
```

支持：

```text
page
page_size
status
source
target
created_by
```

---

# 32. 获取任务详情

```http
GET /api/v1/migrations/:id
```

---

# 33. 获取任务日志

```http
GET /api/v1/migrations/:id/logs
```

---

# 34. 实时日志

使用：

```text
Server-Sent Events
```

接口：

```http
GET /api/v1/migrations/:id/logs/stream
```

前端：

```text
Browser
   ↓
SSE
   ↓
Go Backend
   ↓
Kubernetes Pod Logs
```

---

# 35. 实时进度

接口：

```http
GET /api/v1/migrations/:id/progress
```

实时更新：

```json
{
  "status": "RUNNING",
  "progress": 68,
  "tables_total": 183,
  "tables_completed": 124,
  "bytes_total": 19327352832,
  "bytes_transferred": 13124710400,
  "speed": "35 MB/s"
}
```

---

# 36. 前端页面结构

```text
/
├── dashboard
├── connections
│   ├── list
│   ├── create
│   └── edit
│
├── migrations
│   ├── list
│   ├── create
│   └── detail
│
└── settings
```

---

# 37. Dashboard 页面

展示：

```text
Connections
    MySQL
    PostgreSQL
    DM8
    Redis

Migration Statistics
    Running
    Success
    Failed

Recent Migrations
```

不要做复杂图表。

---

# 38. Connections 页面

表格：

```text
Name
Type
Host
Port
Database
Status
Updated
Actions
```

Actions：

```text
Test
Edit
Delete
```

---

# 39. New Connection 页面

表单：

```text
Name
Database Type
Host
Port
Username
Password
Database
SSL
Description
```

按钮：

```text
Test Connection
Save
Cancel
```

---

# 40. New Migration 页面

采用左右布局。

```text
Source                  Target

Connection              Connection
Database                Database

       ↓ Migration ↓

Migration Type
[ Full Migration ]

[ Start Migration ]
```

Source 和 Target 必须明显区分。

---

# 41. Migration Detail 页面

顶部：

```text
Migration #1024

MySQL
test-mysql
    ↓
MySQL
test-mysql-02
```

显示：

```text
Status
Progress
Duration
Tables
Rows
Data Size
Speed
```

下面：

```text
Logs
```

---

# 42. UI 设计风格

整体使用现代 DevOps 工具风格。

参考：

* Vercel
* Argo CD
* Rancher
* GitLab

要求：

* 简洁
* 深色 / 浅色主题可选
* 大量留白
* 卡片少而精
* 状态颜色明确
* 不使用复杂渐变
* 不使用花哨动画

状态：

```text
Running   → 蓝色
Success   → 绿色
Failed    → 红色
Pending   → 灰色
Cancelled → 橙色
```

---

# 43. 数据库结构

平台自身使用 PostgreSQL。

## connections

```sql
CREATE TABLE connections (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(30) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL,
    username VARCHAR(255),
    password_encrypted TEXT,
    database_name VARCHAR(255),
    ssl_mode VARCHAR(50),
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

## migration_tasks

```sql
CREATE TABLE migration_tasks (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,

    source_connection_id BIGINT NOT NULL,
    target_connection_id BIGINT NOT NULL,

    source_database VARCHAR(255),
    target_database VARCHAR(255),

    migration_type VARCHAR(50) NOT NULL,
    engine VARCHAR(50),

    status VARCHAR(30) NOT NULL,

    progress INTEGER DEFAULT 0,

    tables_total BIGINT DEFAULT 0,
    tables_completed BIGINT DEFAULT 0,

    rows_total BIGINT DEFAULT 0,
    rows_completed BIGINT DEFAULT 0,

    bytes_total BIGINT DEFAULT 0,
    bytes_transferred BIGINT DEFAULT 0,

    speed BIGINT DEFAULT 0,

    error_message TEXT,

    started_at TIMESTAMP,
    finished_at TIMESTAMP,

    created_by VARCHAR(100),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

## migration_logs

```sql
CREATE TABLE migration_logs (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL,
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

# 44. Kubernetes Job 模板

Worker Job 示例：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: dbmove-migration-1024
spec:
  backoffLimit: 0

  template:
    spec:
      restartPolicy: Never

      containers:
        - name: migration-worker
          image: dbmove-worker:latest

          env:
            - name: TASK_ID
              value: "1024"

            - name: DBMOVE_API
              value: "http://dbmove-api"

          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"

            limits:
              cpu: "4"
              memory: "4Gi"
```

实际生产环境需要根据迁移任务动态设置资源。

---

# 45. Worker 与 Backend 通信

Worker 启动：

```text
TASK_ID=1024
```

Worker 调用：

```http
GET /internal/tasks/1024
```

获取：

```text
Source Connection
Target Connection
Migration Type
Engine
Configuration
```

然后执行迁移。

---

# 46. Worker 日志

Worker 输出标准日志：

```text
2026-08-24 21:30:01 INFO migration started
2026-08-24 21:30:02 INFO connecting source
2026-08-24 21:30:02 INFO source connected
2026-08-24 21:30:03 INFO connecting target
2026-08-24 21:30:03 INFO target connected
2026-08-24 21:30:04 INFO starting dump
2026-08-24 21:31:02 INFO table users completed
2026-08-24 21:32:12 INFO table orders completed
2026-08-24 21:35:30 INFO restore completed
2026-08-24 21:35:31 INFO migration completed
```

---

# 47. Preflight Check

迁移开始之前必须执行检查。

至少检查：

```text
Source connection
Target connection
Source database
Target database
Target permissions
Target disk space
Network connectivity
Database version
Migration engine availability
```

结果：

```text
✓ Source connection
✓ Target connection
✓ Database exists
✓ Permission check
✓ Disk space
✓ Migration engine

Ready to migrate
```

如果失败：

```text
✗ Target disk space insufficient
```

禁止开始迁移。

---

# 48. 目标数据库策略

第一版：

如果 Target Database 不存在：

```text
允许用户选择：

[ Create Database ]
```

如果存在：

```text
警告：

Target database already exists.

[ Cancel ]
[ Continue and overwrite ]
```

MVP 默认禁止覆盖已有数据库，必须明确确认。

---

# 49. 数据安全

必须做到：

1. Password 不出现在普通 API Response
2. Password 不写入日志
3. Migration Logs 自动过滤 password
4. Kubernetes Secret 不打印
5. 前端密码输入框使用 password 类型
6. 删除连接时删除对应 Secret
7. Worker 完成后清理临时 Secret

---

# 50. 日志安全

禁止：

```text
mysql://root:password@10.0.0.1
```

直接出现在日志。

必须脱敏：

```text
mysql://root:******@10.0.0.1
```

---

# 51. 任务并发

MVP：

```text
max_concurrent_migrations = 3
```

超过 3 个任务：

```text
PENDING
```

等待执行。

后续可以改成配置：

```text
MAX_CONCURRENT_MIGRATIONS
```

---

# 52. 失败处理

迁移失败：

```text
RUNNING
   ↓
FAILED
```

记录：

```text
error_message
worker logs
exit code
```

前端显示：

```text
Migration Failed

Reason:
Target database permission denied.

[ View Logs ]
[ Retry ]
```

---

# 53. 取消任务

用户点击：

```text
Cancel Migration
```

Backend：

```text
Find Kubernetes Job
      ↓
Delete Job
      ↓
Status = CANCELLED
```

---

# 54. Docker 部署

至少提供：

```text
Dockerfile
docker-compose.yml
```

本地开发：

```text
docker compose up -d
```

启动：

```text
dbmove-api
dbmove-web
postgresql
```

Worker 镜像单独：

```text
dbmove-worker
```

---

# 55. Kubernetes 部署

提供 Helm Chart：

```text
deploy/
└── helm/
    └── dbmove/
        ├── Chart.yaml
        ├── values.yaml
        └── templates/
            ├── deployment.yaml
            ├── service.yaml
            ├── ingress.yaml
            ├── serviceaccount.yaml
            └── rbac.yaml
```

---

# 56. RBAC

Backend 需要：

```text
get jobs
create jobs
delete jobs
get pods
get pods/log
```

最小权限原则。

不要给：

```text
cluster-admin
```

---

# 57. 项目目录

最终：

```text
dbmove/
│
├── frontend/
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   ├── services/
│   │   ├── hooks/
│   │   ├── stores/
│   │   └── types/
│   ├── package.json
│   └── vite.config.ts
│
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   │
│   ├── internal/
│   │   ├── api/
│   │   ├── service/
│   │   ├── engine/
│   │   ├── model/
│   │   ├── repository/
│   │   ├── kubernetes/
│   │   └── config/
│   │
│   ├── migrations/
│   ├── go.mod
│   └── Dockerfile
│
├── worker/
│   ├── cmd/
│   │   └── worker/
│   │       └── main.go
│   ├── internal/
│   │   ├── engine/
│   │   ├── executor/
│   │   └── reporter/
│   ├── go.mod
│   └── Dockerfile
│
├── deploy/
│   ├── docker-compose.yml
│   └── helm/
│
├── docs/
│
├── Makefile
└── README.md
```

---

# 58. 开发阶段

## Phase 1：基础框架

实现：

* React
* Go
* PostgreSQL
* Docker Compose
* 基础 Layout
* API Framework
* Database Migration

验收：

```text
docker compose up
```

能够启动：

```text
Web
API
PostgreSQL
```

---

# 59. Phase 2：Connection

实现：

* Connection CRUD
* MySQL
* PostgreSQL
* Test Connection
* Password 加密
* Connection List

验收：

能够通过 Web UI：

```text
添加 MySQL
添加 PostgreSQL
测试连接
编辑
删除
```

---

# 60. Phase 3：Migration

实现：

* Create Migration
* Full Migration
* Migration Task
* Worker
* Kubernetes Job
* MySQL Engine
* PostgreSQL Engine

验收：

```text
MySQL A
    ↓
DBMove
    ↓
MySQL B
```

能够成功迁移。

以及：

```text
PostgreSQL A
    ↓
DBMove
    ↓
PostgreSQL B
```

能够成功迁移。

---

# 61. Phase 4：Monitoring

实现：

* Task Detail
* Progress
* Logs
* SSE
* Cancel
* Retry

验收：

用户可以实时看到：

```text
Running
Progress
Logs
Speed
```

---

# 62. Phase 5：DM8 / Redis

增加：

```text
DM8 Engine
Redis Engine
```

---

# 63. Phase 6：高级功能

后续再做：

```text
Schema Only
Data Only
Table Selection
Incremental
CDC
Scheduled Migration
Migration Templates
Data Validation
```

---

# 64. MVP 验收标准

必须满足：

## Connection

* [ ] 可以添加 MySQL
* [ ] 可以添加 PostgreSQL
* [ ] 可以测试连接
* [ ] 可以编辑连接
* [ ] 可以删除连接
* [ ] 密码不会出现在 API 返回中

## Migration

* [ ] 可以创建迁移任务
* [ ] 可以选择 Source
* [ ] 可以选择 Target
* [ ] 可以选择数据库
* [ ] 可以启动迁移
* [ ] 可以取消迁移
* [ ] 可以重试失败任务

## Worker

* [ ] Worker 可以执行 MySQL 迁移
* [ ] Worker 可以执行 PostgreSQL 迁移
* [ ] Worker 运行在 Kubernetes Job
* [ ] Job 失败后状态正确
* [ ] Job 成功后状态正确

## UI

* [ ] Dashboard
* [ ] Connections
* [ ] New Migration
* [ ] Migration Detail
* [ ] Migration Logs
* [ ] Migration History

## Deployment

* [ ] Docker Compose 可以运行
* [ ] Helm Chart 可以部署
* [ ] README 包含完整部署说明

---

# 65. Agent 开发要求

AI Agent 必须遵守以下原则：

## 原则 1：先完成 MVP

不要一开始实现：

* CDC
* DM8
* Redis
* RBAC
* 多租户
* 数据脱敏
* 复杂监控

先让：

```text
MySQL → MySQL
PostgreSQL → PostgreSQL
```

跑通。

---

## 原则 2：Migration Engine 必须抽象

不要把数据库迁移代码写死在 API Handler 中。

必须：

```text
API
 ↓
Service
 ↓
Migration Engine
 ↓
Worker
```

---

## 原则 3：耗时任务不能阻塞 API

禁止：

```go
// HTTP Request 中直接执行 30 分钟的 dump
```

必须：

```text
API
 ↓
Create Job
 ↓
Return Task ID
```

---

## 原则 4：所有迁移任务异步执行

API：

```http
POST /migrations
```

应该快速返回：

```json
{
  "id": 1024,
  "status": "PENDING"
}
```

---

## 原则 5：前端不能直接连接数据库

架构必须：

```text
Browser
   ↓
Backend
   ↓
Database
```

禁止：

```text
Browser
   ↓
Database
```

---

# 66. API Response 统一格式

成功：

```json
{
  "success": true,
  "data": {}
}
```

失败：

```json
{
  "success": false,
  "error": {
    "code": "CONNECTION_FAILED",
    "message": "Failed to connect database"
  }
}
```

---

# 67. Error Code

至少定义：

```text
CONNECTION_FAILED
AUTH_FAILED
DATABASE_NOT_FOUND
PERMISSION_DENIED
DISK_SPACE_INSUFFICIENT
MIGRATION_FAILED
MIGRATION_NOT_FOUND
MIGRATION_ALREADY_RUNNING
MIGRATION_CANCELLED
ENGINE_NOT_FOUND
INVALID_DATABASE_TYPE
```

---

# 68. README 必须包含

README 必须说明：

```text
Project Introduction
Features
Architecture
Requirements
Local Development
Docker Deployment
Kubernetes Deployment
Configuration
Database Migration
Supported Databases
API Documentation
Troubleshooting
Development Guide
```

---

# 69. 最终产品体验

用户最终看到的流程应该非常简单：

```text
登录 DBMove
    ↓
Connections
    ↓
选择 MySQL A
    ↓
选择 MySQL B
    ↓
New Migration
    ↓
Full Migration
    ↓
Start
    ↓
Preflight Check
    ↓
Running
    ↓
查看实时日志
    ↓
Migration Completed
```

用户完全不需要知道：

```text
mysqldump
mydumper
myloader
pg_dump
pg_restore
SeaTunnel
Kubernetes Job
```

这些全部属于 DBMove 内部实现。

---

# 70. 最终技术决策

最终采用：

```text
Frontend:
React + TypeScript + Vite + Ant Design

Backend:
Go + Gin + GORM

Platform Database:
PostgreSQL

Task Execution:
Kubernetes Job

Migration Engine:
SeaTunnel / Native Database Tools

Realtime:
SSE

Deployment:
Docker + Helm

Architecture:
Web UI
    ↓
Go API
    ↓
Migration Manager
    ↓
Kubernetes Job
    ↓
Migration Worker
    ↓
Database Migration Engine
```

---

# 71. 第一版绝对优先级

实现顺序必须是：

```text
1. Project Skeleton
       ↓
2. Connection Management
       ↓
3. MySQL Connection Test
       ↓
4. PostgreSQL Connection Test
       ↓
5. Migration Task
       ↓
6. Kubernetes Job
       ↓
7. MySQL → MySQL
       ↓
8. PostgreSQL → PostgreSQL
       ↓
9. Logs
       ↓
10. Progress
       ↓
11. Cancel
       ↓
12. Retry
       ↓
13. Docker
       ↓
14. Helm
```

完成以上内容以后再考虑其他数据库和高级功能。

---

# 72. AI Agent 执行要求

开始开发之前：

1. 阅读整个设计文档。
2. 检查当前代码仓库是否已经存在项目代码。
3. 如果项目为空，从零初始化。
4. 如果已有代码，优先复用已有结构。
5. 不要擅自改变技术栈。
6. 不要增加 MVP 范围之外的大型功能。
7. 每完成一个 Phase 必须进行测试。
8. 所有 API 必须有基本错误处理。
9. 所有数据库操作必须使用参数化查询。
10. 所有敏感信息必须脱敏。
11. 所有长任务必须异步执行。
12. 所有迁移任务必须具有唯一 Task ID。
13. 所有任务必须能够查看日志。
14. 所有失败任务必须能够看到明确失败原因。

---

# 73. 最终验收 Demo

开发完成后，必须能够演示：

```text
打开 DBMove
    ↓
添加 MySQL Source
    ↓
Test Connection ✓
    ↓
添加 MySQL Target
    ↓
Test Connection ✓
    ↓
New Migration
    ↓
Source:
MySQL A / test_db

Target:
MySQL B / test_db

Mode:
Full Migration

    ↓
Start
    ↓
Preflight ✓
    ↓
Migration Running
    ↓
实时显示日志
    ↓
显示 Progress
    ↓
Migration Completed ✓
```

然后验证：

```text
Source MySQL 数据
       =
Target MySQL 数据
```

再完成：

```text
PostgreSQL A
       ↓
DBMove
       ↓
PostgreSQL B
```

整个流程成功后，才认为 MVP 完成。

---

# 74. 一句话总结

DBMove 不是数据库管理系统，也不是重新开发一个数据库同步引擎。

它是：

```text
一个简单的 Web UI
        +
一个迁移任务管理器
        +
Kubernetes Job
        +
成熟的数据库迁移工具
```

核心目标只有一个：

> **让开发人员不用敲命令，通过 Web 页面就可以安全、直观地把一个数据库迁移到另一个数据库。**
