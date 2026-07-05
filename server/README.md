# Server

Go API lives here.

当前实现前后端联调闭环：

- `GET /healthz`
- `GET /api/content/bootstrap`
- `POST /api/users/reserve`
- `POST /api/runs`
- `GET /api/leaderboard`

## PostgreSQL

配置 `DATABASE_URL` 后，后端会在启动时自动执行 `server/db/schema.sql` 和 `server/db/seed.sql`。数据库路径负责：

- 商品卡、场景、事件、状态和音轨入口。
- 用户名唯一占用。
- 成绩提交。
- 排行榜排序。

本机 Homebrew PostgreSQL 示例：

```sh
brew services start postgresql@15
/opt/homebrew/opt/postgresql@15/bin/createdb crazy_fuckwit_250
DATABASE_URL='postgres://localhost/crazy_fuckwit_250?sslmode=disable' npm run dev:api
```

如果不配置 `DATABASE_URL`，后端会退回内存排行榜和静态内容包。这个兜底只用于本地开发，不是正式存储。
