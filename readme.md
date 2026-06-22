# HaoHaoAccounting

好好记账是一个面向个人用户的极简跨端记账 MVP：

- 后端：Go + Gin + GORM，支持 PostgreSQL / MySQL
- Web：Next.js + React
- Mobile：Expo + React Native
- 基础服务：PostgreSQL + Redis

## 功能

- 登录认证：当前仅开放固定账号密码登录，并通过 Bearer Token 访问受保护接口
- 手动记账：收支、金额、分类、账户、备注、标签、时间
- AI 对话记账：例如“今天午饭35”，解析后先确认再入账
- 分类管理：系统分类 + 自定义分类
- 账户管理：现金、银行卡、支付宝、微信、自定义账户
- 报表分析：分类占比、月度趋势、账户统计、周期对比
- 导入导出：CSV / XLSX
- Redis 缓存：报表汇总和 AI 解析缓存

## 目录

- `backend/`：Go API 服务
- `web/`：Next.js Web 端
- `mobile/`：Expo 移动端
- `docker-compose.local.yaml`：本地开发依赖服务
- `docker-compose.yaml`：生产部署示例

## 本地开发

本地开发推荐只用 Docker 跑 PostgreSQL 和 Redis，后端、Web、移动端在宿主机运行。
Node.js 版本以仓库根目录 `.nvmrc` 为准，当前为 22；根目录、Web、Mobile 的 `package.json` 通过 `engines` 声明同一主版本，CI 和 Docker 镜像也使用同一个主版本。

### 1. 启动数据库和缓存

```bash
docker compose -f docker-compose.local.yaml up -d postgres redis
```

默认连接信息：

- PostgreSQL：`127.0.0.1:55432`
- PostgreSQL 数据库：`haohaoaccounting`
- PostgreSQL 用户：`postgres`
- PostgreSQL 密码：`haohao123`
- Redis：`127.0.0.1:56379`
- Redis 密码：`haohao123`

如果你本机已经用旧密码启动过 `haohao_pg_data` volume，PostgreSQL 不会因为 compose 文件变化自动重置已有密码。最简单的方式是删除本地测试 volume 后重建：

```bash
docker compose -f docker-compose.local.yaml down -v
docker compose -f docker-compose.local.yaml up -d postgres redis
```

这会清空本地开发数据库。

### 2. 启动后端

```bash
cd backend
cp .env.example .env
go run ./cmd/server
```

后端默认监听：

- API：`http://localhost:8080/api/v1`
- 存活检查：`http://localhost:8080/livez`
- 就绪检查：`http://localhost:8080/readyz`
- 兼容健康检查：`http://localhost:8080/health`

`.env.example` 已经匹配本地 compose 的 PostgreSQL 和 Redis 密码。需要切换数据库时再修改 `DB_DRIVER` 和 `DB_DSN`。

如需显式执行数据库迁移：

```bash
cd backend
go run ./cmd/dbmigrate
```

API 合约维护在 `backend/api/openapi.yaml`。更新接口结构后，在仓库根目录生成 web/mobile 共用类型：

```bash
npm run generate:api-types
npm run verify:api-contract
```

提交前建议跑一遍和 CI 接近的检查：

```bash
npm run verify:compose
npm run verify:api-contract
npm run verify:backend
npm run verify:web
npm run verify:mobile
```

如果改动影响浏览器流程，额外运行：

```bash
npm run verify:web:e2e
```

### 3. 启动 Web

```bash
cd web
cp .env.example .env.local
npm ci
npm run dev
```

访问：

```text
http://localhost:3000/login
```

当前本地登录账号来自 `backend/.env.example`：

```text
username: admin
password: haohao123
```

### 4. 启动移动端

```bash
cd mobile
cp .env.example .env
npm ci
npm run start
```

用 Expo Go 或模拟器打开。默认移动端 API 地址是：

```text
http://127.0.0.1:8080/api/v1
```

如果使用真机调试，需要把 `EXPO_PUBLIC_API_BASE` 改成电脑在局域网内的 IP。

## 生产部署

生产部署示例在 `docker-compose.yaml`，默认包含：

- `web`：Next.js 生产服务，端口 `3000`
- `backend`：Go API 服务，端口 `8080`
- `postgres`：PostgreSQL，仅内部网络
- `redis`：Redis，仅内部网络并启用密码

### 镜像拆分

项目按 5 个镜像组织：

| 镜像 | Dockerfile | 说明 |
| --- | --- | --- |
| `haohaoaccounting-web:latest` | `web/Dockerfile` | Next.js Web 端 |
| `haohaoaccounting-backend:latest` | `backend/Dockerfile` | Go API 服务 |
| `haohaoaccounting-postgres:16` | `Dockerfile.postgres` | PostgreSQL 数据库 |
| `haohaoaccounting-redis:7` | `Dockerfile.redis` | Redis 缓存 |
| `haohaoaccounting-mysql:8.4` | `Dockerfile.mysql` | 可选 MySQL 数据库 |

默认生产组合是 `web + backend + postgres + redis`。`mysql` 通过 compose profile 启用，和 PostgreSQL 二选一使用。
生产 compose 还包含一个一次性 `dbmigrate` 服务，复用后端镜像中的 `/app/dbmigrate` 命令。`backend` 会等待 `dbmigrate` 成功退出后再启动，避免 API 在 schema 尚未迁移完成时接收请求。

独立构建 5 个镜像：

```bash
docker build -t haohaoaccounting-web:latest \
  --build-arg NEXT_PUBLIC_API_BASE=https://api.example.com/api/v1 \
  ./web

docker build -t haohaoaccounting-backend:latest ./backend

docker build -t haohaoaccounting-postgres:16 -f Dockerfile.postgres .

docker build -t haohaoaccounting-redis:7 -f Dockerfile.redis .

docker build -t haohaoaccounting-mysql:8.4 -f Dockerfile.mysql .
```

后端镜像同时包含 HTTP 服务 `/app/haohaoaccounting`、迁移命令 `/app/dbmigrate` 和 `migrations/` SQL 文件，因此可以用同一个镜像执行生产迁移：

```bash
docker run --rm \
  -e DB_DRIVER=postgres \
  -e DB_DSN="host=postgres user=haohao password=... dbname=haohaoaccounting port=5432 sslmode=disable TimeZone=Asia/Shanghai" \
  -e JWT_SECRET=jwt-secret-with-at-least-32-characters \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=admin-secret \
  -e CORS_ALLOW_ORIGINS=https://app.example.com \
  haohaoaccounting-backend:latest /app/dbmigrate
```

国内网络构建后端镜像时，可以指定 Go module 代理：

```bash
docker build -t haohaoaccounting-backend:latest \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  ./backend
```

这些镜像不内置生产密码。K8s、Compose 或其他运行平台需要通过 Secret/ConfigMap/环境变量注入 `DB_DSN`、`REDIS_PASSWORD`、`JWT_SECRET`、`ADMIN_PASSWORD`、`CORS_ALLOW_ORIGINS` 等配置。

Web 端使用 Next.js standalone 输出。`NEXT_PUBLIC_API_BASE` 会在构建时写入浏览器包，因此同一个 Web 镜像不能在不同公网 API 地址之间无损复用；切换 API 域名时需要带新的 `NEXT_PUBLIC_API_BASE` 重新构建镜像。

### 1. 设置生产环境变量

`docker-compose.yaml` 不内置可直接用于生产的默认密码。启动前必须在 shell 或 `.env` 中设置：

- `NEXT_PUBLIC_API_BASE`
- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `JWT_TTL`（可选，默认 `168h`）
- `JWT_CLOCK_SKEW`（可选，默认 `30s`）
- `JWT_ISSUER`、`JWT_AUDIENCE`（可选，默认 `haohao-accounting`、`haohao-accounting-api`）
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `CORS_ALLOW_ORIGINS`
- `GIN_MODE`（可选，默认 `release`，可选值 `debug`、`release`、`test`）
- `TRUSTED_PROXIES`（可选，部署在反向代理后面时填写代理 IP/CIDR；不要使用 unspecified 地址、`0.0.0.0/0`、`::/0`、域名或通配符）
- `DB_MAX_OPEN_CONNS`、`DB_MAX_IDLE_CONNS`、`DB_CONN_MAX_LIFETIME`、`DB_CONN_MAX_IDLE_TIME`（可选，数据库连接池配置）
- `HTTP_READ_TIMEOUT`、`HTTP_READ_HEADER_TIMEOUT`、`HTTP_WRITE_TIMEOUT`、`HTTP_IDLE_TIMEOUT`、`HTTP_SHUTDOWN_TIMEOUT`、`HTTP_REQUEST_TIMEOUT`（可选，Go duration 格式）
- `HTTP_MAX_HEADER_BYTES`（可选，默认 `1048576`，1 MiB）
- `HTTP_MAX_BODY_BYTES`（可选，默认 `6291456`，约 6 MiB）
- `HTTP_METRICS_ENABLED`（可选，默认 `false`；仅在 backend 端口受保护时开启 `/metrics`）
- `HTTP_METRICS_TOKEN`（可选；设置后 `/metrics` 需要 `Authorization: Bearer <token>`）
- `HTTP_HSTS_MAX_AGE_SECONDS`、`HTTP_HSTS_INCLUDE_SUBDOMAINS`、`HTTP_HSTS_PRELOAD`（可选，仅 HTTPS 部署启用）
- `BACKEND_STOP_GRACE_PERIOD`（可选，Compose 停止 backend 容器的宽限期，默认 `30s`，应大于 `HTTP_SHUTDOWN_TIMEOUT`）
- `MYSQL_ROOT_PASSWORD`（仅启用 MySQL profile 时需要）

这些值必须保持一致，否则后端会连不上数据库或 Redis。
后端会在连接数据库前校验 `DB_DRIVER`、`JWT_SECRET`、`ADMIN_USERNAME` 和 `ADMIN_PASSWORD`，缺失、不支持或不满足长度要求时会直接启动失败。`DB_DRIVER` 只支持 `postgres`、`pgsql`、`mysql`。
后端服务和 `dbmigrate` 会严格解析显式设置的整数、布尔值和 Go duration 环境变量；例如 `HTTP_READ_TIMEOUT=abc`、`HTTP_MAX_BODY_BYTES=-1` 或 `HTTP_HSTS_PRELOAD=maybe` 会让启动失败，而不是静默回退默认值。

示例：

```env
NEXT_PUBLIC_API_BASE=https://api.example.com/api/v1
POSTGRES_PASSWORD=replace-with-a-long-random-password
REDIS_PASSWORD=replace-with-a-long-random-password
JWT_SECRET=replace-with-at-least-32-random-characters
JWT_TTL=168h
JWT_CLOCK_SKEW=30s
JWT_ISSUER=https://api.example.com
JWT_AUDIENCE=haohao-accounting-api
ADMIN_USERNAME=admin
ADMIN_PASSWORD=replace-with-a-long-random-password
CORS_ALLOW_ORIGINS=https://app.example.com
GIN_MODE=release
TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=1h
DB_CONN_MAX_IDLE_TIME=30m
HTTP_READ_TIMEOUT=15s
HTTP_READ_HEADER_TIMEOUT=5s
HTTP_WRITE_TIMEOUT=30s
HTTP_IDLE_TIMEOUT=60s
HTTP_SHUTDOWN_TIMEOUT=10s
HTTP_REQUEST_TIMEOUT=0s
HTTP_MAX_HEADER_BYTES=1048576
HTTP_MAX_BODY_BYTES=6291456
HTTP_METRICS_ENABLED=false
HTTP_METRICS_TOKEN=
HTTP_HSTS_MAX_AGE_SECONDS=0
HTTP_HSTS_INCLUDE_SUBDOMAINS=false
HTTP_HSTS_PRELOAD=false
BACKEND_STOP_GRACE_PERIOD=30s
```

`CORS_ALLOW_ORIGINS` 使用英文逗号分隔多个完整 Web origin，必须包含 `http://` 或 `https://`，不要包含路径、query、fragment 或通配符。裸域名 `app.example.com`、带路径 `https://app.example.com/app`、通配符 `https://*.example.com` 都会在启动时被拒绝。例如：

```env
CORS_ALLOW_ORIGINS=https://app.example.com,https://admin.example.com
```

后端 CORS 允许浏览器发送 `Authorization` 和 `X-Request-ID`，并暴露 `X-Request-ID`、`Link`、`X-Total-Count`、`Content-Disposition`、`Allow`、`WWW-Authenticate`、`Retry-After`，方便前端读取追踪 ID、分页、导出文件名、鉴权、限流和 405 允许方法信息。

所有 `/api/v1` 业务接口及其框架级 404/405 错误都会返回 `Cache-Control: no-store`、`Pragma: no-cache` 和 `Expires: 0`，避免浏览器或中间代理缓存登录 token、用户数据、账单、报表响应或 API 错误。`/livez`、`/readyz`、`/health` 探针不套用该策略。

后端会统一返回基础安全响应头，包括 `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`、`Referrer-Policy: no-referrer`、`X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY` 和一个保守的 `Permissions-Policy`，默认关闭相机、定位、麦克风和支付能力。

`Strict-Transport-Security` 默认不发送，避免本地 HTTP 调试或尚未确认 HTTPS 覆盖面的域名被浏览器长期记住为强制 HTTPS。生产环境确认 API 域名只通过 HTTPS 访问后，可以设置 `HTTP_HSTS_MAX_AGE_SECONDS=31536000`；只有确认所有子域也都支持 HTTPS 时再开启 `HTTP_HSTS_INCLUDE_SUBDOMAINS=true`。准备加入浏览器 preload 列表时再开启 `HTTP_HSTS_PRELOAD=true`，此时启动校验会要求 `HTTP_HSTS_MAX_AGE_SECONDS>=31536000` 且 `HTTP_HSTS_INCLUDE_SUBDOMAINS=true`。

如果后端直接暴露给公网或本地直连调试，`TRUSTED_PROXIES` 可以留空。只有当服务位于 Nginx、Ingress、负载均衡等可信代理之后，并且需要读取 `X-Forwarded-*` 地址头时，才配置这些代理的具体 IP 或内网 CIDR。启动校验会拒绝 unspecified 地址、`0.0.0.0/0`、`::/0`、域名和非法 CIDR，避免任意客户端伪造 `X-Forwarded-For` 影响日志和登录限流。

后端默认以 Gin `release` 模式启动，生产环境应保持 `GIN_MODE=release`，避免输出调试路由和 debug 警告。只有本地排查框架行为时再临时设为 `debug`；测试代码可以使用 `test`。

后端签发和校验 JWT 时会要求 `iss` 与 `aud` 匹配 `JWT_ISSUER` 和 `JWT_AUDIENCE`，避免其他环境或其他服务签发的同密钥 token 被误用。生产环境建议把 `JWT_ISSUER` 设为当前 API 的稳定标识或域名，`JWT_AUDIENCE` 设为调用方/资源服务标识；切换这两个值会让旧 token 失效。
`JWT_CLOCK_SKEW` 用于 JWT `exp`/`iat` 等时间校验的少量时钟偏差容忍，默认 `30s`；只有在多节点时钟同步仍存在短暂偏差时才调整，设为 `0s` 可禁用该容忍。

后端使用 `net/http` server 启动并支持优雅关闭。`HTTP_*_TIMEOUT`、`BACKEND_STOP_GRACE_PERIOD` 和 `JWT_TTL` 使用 Go duration 格式，例如 `5s`、`30s`、`2m`、`168h`；一般不需要调整，只有在大文件导入、慢代理、平台关闭窗口或登录有效期有特殊要求时再覆盖。Compose 的 `BACKEND_STOP_GRACE_PERIOD` 应大于 `HTTP_SHUTDOWN_TIMEOUT`，否则 Docker 可能在应用优雅关闭完成前强制结束容器。

`HTTP_REQUEST_TIMEOUT` 默认 `0s`，表示不额外设置每个请求的 context deadline。生产环境可以按网关超时和接口耗时预算设置为例如 `30s`，下游数据库、Redis 或外部服务调用只要使用 `request.Context()` 就能收到取消信号；大文件导入等长耗时接口上线前应单独评估这个值。

`HTTP_MAX_HEADER_BYTES` 会写入 Go `http.Server.MaxHeaderBytes`，限制请求行和请求头总读取大小。默认 1 MiB，通常不需要调整；如果网关或浏览器携带异常大的 Cookie/认证头，应优先清理上游 header，而不是盲目放大该值。

后端使用 Go `database/sql` 连接池。`DB_MAX_OPEN_CONNS`、`DB_MAX_IDLE_CONNS` 控制连接数量，`DB_CONN_MAX_LIFETIME`、`DB_CONN_MAX_IDLE_TIME` 使用 Go duration 格式；默认分别是 `25`、`10`、`1h`、`30m`，HTTP 服务和 `dbmigrate` 会使用同一套配置。

`HTTP_MAX_BODY_BYTES` 是全局请求体上限，用于在 HTTP 层提前拒绝过大的 JSON 或 multipart 请求。默认约 6 MiB，略高于导入文件 5 MiB 的业务上限，用于容纳 multipart 边界和表单字段开销。
该上限同时覆盖已声明 `Content-Length` 的请求和 chunked/未知长度的流式请求；超限时接口返回结构化 `413 payload_too_large` 错误。

`HTTP_METRICS_ENABLED=false` 默认不暴露 `/metrics`。只有在 backend 端口位于可信内网、被反向代理认证保护，或采集器通过受控网络访问时再开启；设置 `HTTP_METRICS_TOKEN` 后采集器还必须发送 `Authorization: Bearer <token>`。该端点会包含 Go runtime、process 和低基数 HTTP 指标。

Compose 对无状态的 `web` 和 `backend` 启用了只读根文件系统、`no-new-privileges`、`cap_drop: [ALL]` 和 `init: true`。临时写入只允许进入 `tmpfs`，数据库和 Redis 仍通过 volume 持久化数据。若迁移到不支持这些 Compose 选项的平台，需要用等价的安全上下文配置替代。

### 2. 构建并启动

```bash
docker compose up -d --build
```

首次启动或镜像更新后，Compose 会先等待 PostgreSQL 健康检查通过，再运行 `dbmigrate`，最后启动 `backend` 和 `web`。

查看状态：

```bash
docker compose ps
docker compose logs -f backend
```

访问：

```text
http://localhost:3000/login
```

### 4. 停止服务

```bash
docker compose down
```

如果要连数据卷一起删除：

```bash
docker compose down -v
```

## 常用接口

- `POST /api/v1/auth/login`
- `GET /api/v1/me`
- `GET /api/v1/accounts`
- `POST /api/v1/accounts`
- `GET /api/v1/categories`
- `POST /api/v1/categories`
- `GET /api/v1/transactions`
- `POST /api/v1/transactions`
- `POST /api/v1/ai/parse`
- `GET /api/v1/reports/summary`
- `POST /api/v1/io/import`
- `GET /api/v1/io/export?format=csv|xlsx`

## 常见问题

### 后端提示 PostgreSQL 密码错误

本地开发时如果之前已经创建过旧的 PostgreSQL volume，改 `POSTGRES_PASSWORD` 不会更新已有数据库用户密码。可以删除本地测试 volume 重建：

```bash
docker compose -f docker-compose.local.yaml down -v
docker compose -f docker-compose.local.yaml up -d postgres redis
```

### Redis 缓存没有启用

确认后端环境变量和 Redis 服务密码一致：

```env
REDIS_ADDR=127.0.0.1:56379
REDIS_PASSWORD=haohao123
REDIS_DB=0
```

### Web 登录提示 Failed to fetch

通常是后端没启动，或 `NEXT_PUBLIC_API_BASE` 指向了错误地址。先确认：

```bash
curl http://localhost:8080/health
```

生产部署建议用 `/livez` 做 liveness probe，用 `/readyz` 做 readiness probe。`/readyz` 会检查数据库连接；Redis 未启用时显示 `disabled`，启用后会执行 ping。`/health` 保持兼容，返回和 `/readyz` 相同的结果。
