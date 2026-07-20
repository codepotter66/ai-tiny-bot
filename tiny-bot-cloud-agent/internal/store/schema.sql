-- ============================================================
-- tiny-bot-cloud-agent schema v1
-- ============================================================
-- 设计要点：
--   - SQLite + WAL 模式（store.go 里启用）
--   - 主键用 TEXT（ULID/UUID），跨服务迁移友好
--   - 时间戳一律毫秒（INTEGER）
--   - token_hash 存 SHA-256（hex），不存明文
-- ============================================================

CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER PRIMARY KEY
);

-- 用户（一个用户可绑定多个设备）
CREATE TABLE IF NOT EXISTS users (
  id           TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  user_md      TEXT,                                      -- per-user 覆盖 USER.md
  soul_md      TEXT,                                      -- per-user 覆盖 SOUL.md
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);

-- 设备
CREATE TABLE IF NOT EXISTS devices (
  id            TEXT PRIMARY KEY,                          -- e.g. tinypal-01
  user_id       TEXT NOT NULL REFERENCES users(id),
  pairing_code  TEXT UNIQUE,                              -- 配网用（status=unbound 时必填）
  token_hash    TEXT,                                     -- 长期 token 签发后填
  status        TEXT NOT NULL DEFAULT 'unbound',          -- unbound | bound
  hardware_rev  TEXT,
  last_seen_at  INTEGER,
  created_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_devices_user ON devices(user_id);

-- 会话（一段连续对话）
CREATE TABLE IF NOT EXISTS conversations (
  id          TEXT PRIMARY KEY,
  device_id   TEXT NOT NULL REFERENCES devices(id),
  started_at  INTEGER NOT NULL,
  ended_at    INTEGER,
  turn_count  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_conv_device_time ON conversations(device_id, started_at DESC);

-- 消息
CREATE TABLE IF NOT EXISTS messages (
  id              TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES conversations(id),
  role            TEXT NOT NULL,                          -- user | assistant | tool
  content         TEXT NOT NULL,
  tool_name       TEXT,
  tool_args       TEXT,                                   -- JSON 字符串
  latency_ms      INTEGER,
  tokens_in       INTEGER,
  tokens_out      INTEGER,
  created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_msg_conv ON messages(conversation_id, created_at);

-- 记忆索引（关键词检索用）
CREATE TABLE IF NOT EXISTS memory_index (
  device_id TEXT NOT NULL,
  date      TEXT NOT NULL,                                -- YYYY-MM-DD
  line_no   INTEGER NOT NULL,
  ts_ms     INTEGER NOT NULL,
  kw        TEXT NOT NULL,                                -- 空格拼接的小写 token
  PRIMARY KEY (device_id, date, line_no)
);
CREATE INDEX IF NOT EXISTS idx_mem_kw ON memory_index(device_id, kw);
