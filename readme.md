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
- 健康检查：`http://localhost:8080/health`

`.env.example` 已经匹配本地 compose 的 PostgreSQL 和 Redis 密码。需要切换数据库时再修改 `DB_DRIVER` 和 `DB_DSN`。

### 3. 启动 Web

```bash
cd web
cp .env.example .env.local
npm install
npm run dev
```

访问：

```text
http://localhost:3000/login
```

当前登录账号：

```text
username: admin
password: haohao123
```

### 4. 启动移动端

```bash
cd mobile
cp .env.example .env
npm install
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

### 1. 修改公网 API 地址

打开 `docker-compose.yaml`，把两处 `NEXT_PUBLIC_API_BASE` 改成浏览器能访问的后端地址，例如：

```yaml
NEXT_PUBLIC_API_BASE: https://api.example.com/api/v1
```

如果 Web 和 API 放在同一台机器上，也可以先用：

```yaml
NEXT_PUBLIC_API_BASE: http://localhost:8080/api/v1
```

### 2. 设置生产密码

`docker-compose.yaml` 中的默认密码是示例值，部署前建议改掉：

- `backend.environment.DB_DSN` 里的 PostgreSQL 密码
- `backend.environment.REDIS_PASSWORD`
- `backend.environment.JWT_SECRET`
- `postgres.environment.POSTGRES_PASSWORD`
- `redis.command` 里的 `--requirepass`
- `redis.environment.REDIS_PASSWORD`

这些值必须保持一致，否则后端会连不上数据库或 Redis。

### 3. 构建并启动

```bash
docker compose up -d --build
```

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
