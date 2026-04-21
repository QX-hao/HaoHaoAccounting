# HaoHaoAccounting

面向个人用户的极简跨端记账 MVP 系统：

- 后端：Go + Gin + GORM（支持 PostgreSQL / MySQL）
- Web：React + Next.js
- Mobile：React Native（Expo）

## 已实现功能（MVP）

- 登录：手机号 / 邮箱 / 微信（MVP 免密）
- 记账：手动记一笔（收支、分类、账户、备注、标签）
- AI 对话记账：如“今天午饭35”解析成待确认表单
- 分类管理：系统分类 + 自定义分类
- 账户管理：账户新增与余额联动
- 账单明细：筛选查询、分页返回
- 报表分析：分类占比、月度趋势、按账户统计、时间段对比
- 导入导出：CSV / XLSX
- Redis 缓存：报表汇总缓存、AI 解析缓存

## 目录结构

- `backend/` Go-Gin API
- `web/` Next.js Web 端
- `mobile/` Expo 移动端
- `stitch_/` 你提供的 Web UI 稿
- `stitch_elegant_mobile_ledger_ui/` 你提供的移动端 UI 稿

## 快速启动

`<repo-root>` 表示你本机上 clone 下来的仓库根目录（包含 `backend/`、`web/`、`mobile/`）。

### 1) 启动基础服务

默认启动：`PostgreSQL + Redis`

```bash
cd <repo-root>
docker compose up -d
```

默认映射端口（已避开常见占用）：
- PostgreSQL: `55432`
- Redis: `56379`

如果你想改用 MySQL（而不是 PostgreSQL）：

```bash
docker compose --profile mysql up -d mysql redis
```

说明：`MySQL` 和 `PostgreSQL` 是二选一，不是按业务拆库。

### 2) 启动后端

```bash
cd <repo-root>/backend
cp .env.example .env
# 按需修改 DB_DRIVER / DB_DSN / REDIS_ADDR
go run ./cmd/server
```

默认监听：`http://localhost:8080`

### 3) 启动 Web

```bash
cd <repo-root>/web
cp .env.example .env.local
npm install
npm run dev
```

访问：`http://localhost:3000/login`

### 4) 启动移动端

```bash
cd <repo-root>/mobile
cp .env.example .env
npm install
npm run start
```

然后用 Expo Go 或模拟器运行。

## 后端关键接口

- `POST /api/v1/auth/login`
- `GET /api/v1/me`
- `GET/POST/PUT/DELETE /api/v1/accounts`
- `GET/POST/PUT/DELETE /api/v1/categories`
- `GET/POST/PUT/DELETE /api/v1/transactions`
- `POST /api/v1/ai/parse`
- `GET /api/v1/reports/summary`
- `POST /api/v1/io/import`
- `GET /api/v1/io/export?format=csv|xlsx`

## 说明

当前实现采用 Go + Gin + GORM（后端）与 Next.js / React Native（前端），支持 PostgreSQL 或 MySQL（二选一）以及 Redis 缓存。
