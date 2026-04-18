package db

import (
	"database/sql"
	"strings"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS hosts (
    id                   TEXT PRIMARY KEY,
    name                 TEXT UNIQUE NOT NULL,
    ip                   TEXT NOT NULL,
    port                 INTEGER NOT NULL DEFAULT 22,
    username             TEXT NOT NULL,
    auth_type            TEXT NOT NULL,
    encrypted_credential TEXT NOT NULL DEFAULT '',
    encrypted_passphrase TEXT NOT NULL DEFAULT '',
    tags                 TEXT NOT NULL DEFAULT '[]',
    created_at           DATETIME NOT NULL,
    updated_at           DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS execution_logs (
    id           TEXT PRIMARY KEY,
    host_id      TEXT NOT NULL,
    command      TEXT NOT NULL,
    stdout       TEXT NOT NULL DEFAULT '',
    stderr       TEXT NOT NULL DEFAULT '',
    exit_code    INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    triggered_by TEXT NOT NULL DEFAULT 'mcp',
    created_at   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_execution_logs_host_id ON execution_logs(host_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_created_at ON execution_logs(created_at);

-- Phase 2: 用户管理
CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    password     TEXT NOT NULL,
    role         TEXT NOT NULL,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL,
    last_login   DATETIME
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  DATETIME,
    created_at  DATETIME NOT NULL,
    last_used   DATETIME
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash);
`

// migrate 创建所有表（幂等）。
func migrate(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}
	// 幂等追加 user_id 列（SQLite ALTER TABLE 不支持 IF NOT EXISTS）
	_, err := db.Exec(`ALTER TABLE execution_logs ADD COLUMN user_id TEXT`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}
