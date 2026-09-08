# 01. 数据库

> 写于 2026-06-08

## TL;DR

- **DB 类型**: SQLite（用 `modernc.org/sqlite` 纯 Go 驱动，无 CGo）
- **DB 文件**: `./data/tiny-bot.db`（docker-compose 下挂到命名卷 `tiny-bot-cloud-agent-data`）
- **WAL 模式**: 开启（读写并发、崩溃安全）
- **迁移**: 单文件 `internal/store/schema.sql` 启动时全量应用；版本号记在 `schema_version` 表
- **表数量**: 5 张（users / devices / conversations / messages / memory_index）
- **连接池**: 写连接 1 个（避免 "database is locked"），读连接 4 个

## 为什么是 SQLite，不是 Postgres？

| 项 | SQLite | Postgres |
|---|---|---|
| 部署 | 单文件，零进程 | 独立服务，要起容器/进程 |
| 2c2g 内存占用 | ~25 MB（含 8MB page cache） | 至少 150-300 MB 起步 |
| 运维 | 备份 = 拷文件 | 需 pg_dump / WAL-G |
| 写并发 | 单写者（够我们用） | 强并发 |
| 工具链 | `sqlite3` CLI、DB Browser | psql、pgAdmin |

我们的并发模型是**每设备单 turn**（一个 turn 内串行：STT→LLM→TTS），全局并发最多几个活跃设备 → 单写者完全够用。SQLite 的「真香」是省掉了一个服务进程。

## 文件位置

| 部署方式 | 路径 |
|---|---|
| 本地开发 | `./data/tiny-bot.db` |
| docker-compose | `/opt/tiny-bot/data/tiny-bot.db`（命名卷 `agent-data` 内） |
| systemd | `/opt/tiny-bot/data/tiny-bot.db` |

环境变量 `TB_DB_PATH` 可覆盖。

## 表结构

> 完整 SQL 见 `internal/store/schema.sql`

### `schema_version`

记录当前 schema 版本。v1 = 1。

```sql
CREATE TABLE schema_version (version INTEGER PRIMARY KEY);
```

启动时执行 `schema.sql`（`CREATE IF NOT EXISTS`），再尝试 `ALTER TABLE users ADD COLUMN soul_md`（已存在则忽略），并写入 `schema_version=2`。

### `users` — 用户

一个用户可绑定多个设备。

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | TEXT PK | ulid/uuid（这里用 uuid v4） |
| `display_name` | TEXT NOT NULL | 显示名（"小明"） |
| `user_md` | TEXT | 可选：per-user 覆盖 USER.md 的 Markdown 文本 |
| `soul_md` | TEXT | 可选：per-user 覆盖 SOUL.md 的 Markdown 文本 |
| `created_at` / `updated_at` | INTEGER | 毫秒时间戳 |

### `devices` — 设备（= ESP32 一台）

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | TEXT PK | e.g. `tinypal-01` |
| `user_id` | TEXT FK→users | 所属用户 |
| `pairing_code` | TEXT UNIQUE | 配对码（status=unbound 时使用；签发后清空） |
| `token_hash` | TEXT | SHA-256 hex（token 本身只在签发时返回明文） |
| `status` | TEXT | `unbound`（未签发）/ `bound`（已签发） |
| `hardware_rev` | TEXT | 硬件版本（可选） |
| `last_seen_at` | INTEGER | 最近活跃时间（毫秒） |
| `created_at` | INTEGER | 创建时间 |

**索引**: `idx_devices_user ON devices(user_id)`

### `conversations` — 会话

一段连续对话的容器。客户端发 `end` 后我们开一个 conversation；WS 断开或长时间无活动时关闭。

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | TEXT PK | uuid |
| `device_id` | TEXT FK→devices | 所属设备 |
| `started_at` / `ended_at` | INTEGER | 毫秒；`ended_at=0` 表示进行中 |
| `turn_count` | INTEGER | 已发生的 turn 数（每次 `end` +1） |

**索引**: `idx_conv_device_time ON conversations(device_id, started_at DESC)`

### `messages` — 消息

会话里的每一条消息（user / assistant / tool）。

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | TEXT PK | uuid |
| `conversation_id` | TEXT FK→conversations | 所属会话 |
| `role` | TEXT | `user` / `assistant` / `tool` |
| `content` | TEXT | 文本（user/assistant 是文本，tool 是 JSON 结果） |
| `tool_name` | TEXT | 仅 tool 消息 |
| `tool_args` | TEXT | 仅 assistant 的 tool_use（JSON 字符串） |
| `latency_ms` | INTEGER | end → 首字节 TTS 的延迟（用户感知） |
| `tokens_in` / `tokens_out` | INTEGER | token 计数（v1 mock 不填，后续接 LLM 时填） |
| `created_at` | INTEGER | 毫秒 |

**索引**: `idx_msg_conv ON messages(conversation_id, created_at)`

### `memory_index` — 记忆倒排索引

每行对应一条记忆里的一行 Markdown（`workspace/memory/<device_id>/YYYY-MM-DD.md`）。

| 列 | 类型 | 说明 |
|---|---|---|
| `device_id` | TEXT | 设备 ID |
| `date` | TEXT | `YYYY-MM-DD` |
| `line_no` | INTEGER | 文件内行号（1-based） |
| `ts_ms` | INTEGER | 写入毫秒时间戳 |
| `kw` | TEXT | 空格拼接的小写 token（中文单字 + 英文/数字词） |

主键 `(device_id, date, line_no)`，额外 `idx_mem_kw ON memory_index(device_id, kw)` 用于按关键词过滤。`store.MemorySearchQuery` 在 lookback 内按 query 分词过滤；`MarkdownStore.Recall` 有命中时只打分这些行。

**为什么用 SQL 而不是纯文件 grep**？ 2c2g 单 VPS 上 grep 也行，但 7 天累计下来可能上千行；SQL 索引后 O(log n)，且能直接接 `BTree` 范围查询。

## 关键设计

### 写读连接分离

```go
// store.go
rw.SetMaxOpenConns(1)   // 单写者
ro.SetMaxOpenConns(4)   // 4 个读者
```

为什么？SQLite 写会持有 EXCLUSIVE 锁，多连接并发写会触发 "database is locked" 错误。我们所有写都走 RW 池，单连接就够。

### WAL + NORMAL

```go
?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)
```

- `journal_mode=WAL`：写不阻塞读
- `synchronous=NORMAL`：每次 commit fsync 一次（足够安全，写性能提升 2-3x）
- `foreign_keys=ON`：启用外键约束
- `busy_timeout=5000`：拿不到锁时等 5 秒

### 主键策略

全表用 TEXT 主键（UUID v4），而不是自增 INTEGER。理由：
- 跨服务迁移友好（不依赖本地 sequence）
- 分布式友好（未来多副本时可生成 ID 不冲突）
- 索引空间占用略大但可忽略

## 怎么改 schema？

1. 改 `internal/store/schema.sql`
2. `schema.sql` 是 `//go:embed` 进二进制的，所以**改完必须重新 build**
3. v1 没有版本化迁移；后续会改成 `migrations/001_init.sql`、`002_add_xxx.sql` 这种

## 怎么手动查数据？

```bash
# 进 sqlite3 CLI（如果装了）
sqlite3 data/tiny-bot.db

# 常用查询
sqlite> .schema
sqlite> SELECT * FROM devices;
sqlite> SELECT id, role, substr(content, 1, 30), created_at FROM messages ORDER BY created_at DESC LIMIT 10;
sqlite> SELECT COUNT(*) FROM conversations WHERE ended_at = 0;
```

## 备份

```bash
# 简单粗暴
cp data/tiny-bot.db data/tiny-bot.db.bak-$(date +%Y%m%d)

# 推荐：SQLite 官方 backup 命令（热备，WAL 模式安全）
sqlite3 data/tiny-bot.db ".backup data/tiny-bot.db.bak"
```

## 清理

```bash
# 清空全部（保留 schema）
sqlite3 data/tiny-bot.db "DELETE FROM messages; DELETE FROM conversations; DELETE FROM memory_index;"

# 完整重置（删除 DB 文件）
rm data/tiny-bot.db data/tiny-bot.db-shm data/tiny-bot.db-wal
./bin/seed -device-id tinypal-01 -pairing-code ABCD-1234
```

## 相关代码

| 文件 | 作用 |
|---|---|
| `internal/store/store.go` | 连接打开、WAL 配置、迁移入口 |
| `internal/store/schema.sql` | 全部 DDL（v1） |
| `internal/store/devices.go` | users / devices CRUD |
| `internal/store/conversations.go` | conversations / messages CRUD |
| `internal/store/memory_index.go` | 记忆索引读写 + 中文/英文分词 |
| `internal/store/models.go` | 行结构体 |
| `cmd/seed/main.go` | 预置用户/设备 CLI |
