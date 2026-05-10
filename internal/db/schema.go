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
    username             TEXT NOT NULL DEFAULT '',
    auth_type            TEXT NOT NULL DEFAULT '',
    encrypted_credential TEXT NOT NULL DEFAULT '',
    encrypted_passphrase TEXT NOT NULL DEFAULT '',
    tags                 TEXT NOT NULL DEFAULT '[]',
    created_at           DATETIME NOT NULL,
    updated_at           DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS execution_logs (
    id              TEXT PRIMARY KEY,
    host_id         TEXT NOT NULL,
    command         TEXT NOT NULL,
    stdout          TEXT NOT NULL DEFAULT '',
    stderr          TEXT NOT NULL DEFAULT '',
    exit_code       INTEGER NOT NULL DEFAULT 0,
    duration_ms     INTEGER NOT NULL DEFAULT 0,
    triggered_by    TEXT NOT NULL DEFAULT 'mcp',
    risk_level      TEXT NOT NULL DEFAULT '',
    permission_mode TEXT NOT NULL DEFAULT '',
    approval_id     TEXT NOT NULL DEFAULT '',
    approved_by     TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL
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

CREATE TABLE IF NOT EXISTS ssh_keys (
    id                    TEXT PRIMARY KEY,
    user_id               TEXT NOT NULL,
    name                  TEXT NOT NULL,
    encrypted_private_key TEXT NOT NULL,
    encrypted_passphrase  TEXT NOT NULL DEFAULT '',
    fingerprint           TEXT NOT NULL DEFAULT '',
    created_at            DATETIME NOT NULL,
    updated_at            DATETIME NOT NULL,
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_ssh_keys_user_id ON ssh_keys(user_id);

-- Gateway chat tables
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_calls TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);

CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vendor TEXT NOT NULL DEFAULT '',
    cli_type TEXT NOT NULL DEFAULT '',
    doc_type TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    embedding BLOB,
    source_file TEXT NOT NULL DEFAULT '',
    chunk_index INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_documents_vendor_cli ON documents(vendor, cli_type);

CREATE TABLE IF NOT EXISTS pending_confirmations (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_input TEXT NOT NULL,
    risk_level TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL,
    resolved_at DATETIME,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS approvals (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL,
    command         TEXT NOT NULL,
    host            TEXT NOT NULL DEFAULT '',
    risk_level      TEXT NOT NULL,
    risk_reason     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending',
    approved_by     TEXT NOT NULL DEFAULT '',
    requested_at    DATETIME NOT NULL,
    resolved_at     DATETIME
);

CREATE INDEX IF NOT EXISTS idx_approvals_session_id ON approvals(session_id);
CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status);

CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    encrypted_api_key TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    selected_model TEXT NOT NULL DEFAULT '',
    is_active INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_provider_models_provider_id ON provider_models(provider_id);
`

// Migrate 导出版本，供测试使用。
func Migrate(db *sql.DB) error { return migrate(db) }

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
	_, err = db.Exec(`ALTER TABLE hosts ADD COLUMN ssh_key_id TEXT NOT NULL DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	alterStmts := []string{
		"ALTER TABLE hosts ADD COLUMN device_type TEXT DEFAULT ''",
		"ALTER TABLE hosts ADD COLUMN vendor TEXT DEFAULT ''",
		"ALTER TABLE hosts ADD COLUMN model TEXT DEFAULT ''",
		"ALTER TABLE hosts ADD COLUMN cli_type TEXT DEFAULT ''",
		"ALTER TABLE hosts ADD COLUMN firmware_version TEXT DEFAULT ''",
	}
	for _, stmt := range alterStmts {
		db.Exec(stmt) // ignore "duplicate column" errors
	}
	db.Exec("ALTER TABLE messages ADD COLUMN tool_calls TEXT NOT NULL DEFAULT ''")

	permCols := []string{
		"ALTER TABLE execution_logs ADD COLUMN risk_level TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE execution_logs ADD COLUMN permission_mode TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE execution_logs ADD COLUMN approval_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE execution_logs ADD COLUMN approved_by TEXT NOT NULL DEFAULT ''",
	}
	for _, stmt := range permCols {
		db.Exec(stmt)
	}
	db.Exec("ALTER TABLE conversations ADD COLUMN permission_mode TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE hosts ADD COLUMN ssh_legacy INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE documents ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'")
	db.Exec(`CREATE TABLE IF NOT EXISTS document_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)`)
	db.Exec("ALTER TABLE documents ADD COLUMN group_id INTEGER REFERENCES document_groups(id) ON DELETE SET NULL")
	db.Exec("ALTER TABLE providers ADD COLUMN embedding_model TEXT NOT NULL DEFAULT ''")
	// Single-row config table. Managed via DELETE + INSERT (no PK by design).
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS rag_config (
		type              TEXT NOT NULL DEFAULT 'openai',
		base_url          TEXT NOT NULL DEFAULT '',
		model             TEXT NOT NULL DEFAULT '',
		encrypted_api_key TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		return err
	}
	db.Exec("ALTER TABLE rag_config ADD COLUMN name TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE rag_config ADD COLUMN cached_models TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE rag_config ADD COLUMN validated_at TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE conversations ADD COLUMN status TEXT NOT NULL DEFAULT 'idle'")
	// Host redesign: new tables
	db.Exec(`CREATE TABLE IF NOT EXISTS access_faces (
		id TEXT PRIMARY KEY,
		host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		type TEXT NOT NULL CHECK(type IN ('ssh','restapi')),
		ip TEXT NOT NULL,
		port INTEGER NOT NULL,
		username TEXT NOT NULL DEFAULT '',
		auth_type TEXT NOT NULL DEFAULT '',
		encrypted_credential TEXT NOT NULL DEFAULT '',
		encrypted_passphrase TEXT NOT NULL DEFAULT '',
		ssh_key_id TEXT NOT NULL DEFAULT '',
		ssh_legacy INTEGER NOT NULL DEFAULT 0,
		base_url TEXT NOT NULL DEFAULT '',
		rest_auth_type TEXT NOT NULL DEFAULT '',
		rest_username TEXT NOT NULL DEFAULT '',
		header_name TEXT NOT NULL DEFAULT '',
		knowledge_sources TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS host_fingerprints (
		host_id TEXT PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
		ssh_host_key TEXT NOT NULL DEFAULT '',
		system_version TEXT NOT NULL DEFAULT '',
		hardware_id TEXT NOT NULL DEFAULT '',
		api_signature TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'unverified',
		snapshot_at DATETIME
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS host_memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		created_by TEXT NOT NULL CHECK(created_by IN ('user','agent')),
		created_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS host_knowledge_sources (
		host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		type TEXT NOT NULL CHECK(type IN ('group','doc')),
		ref_id INTEGER NOT NULL,
		PRIMARY KEY (host_id, type, ref_id)
	)`)
	// New hosts columns
	db.Exec("ALTER TABLE hosts ADD COLUMN notes TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE hosts ADD COLUMN product_name TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE hosts ADD COLUMN product_version TEXT NOT NULL DEFAULT ''")
	// Data migration: seed one SSH access_face per existing host.
	// Idempotent: WHERE clause skips hosts that already have an SSH face.
	db.Exec(`INSERT OR IGNORE INTO access_faces
		(id, host_id, type, ip, port, username, auth_type,
		 encrypted_credential, encrypted_passphrase, ssh_key_id, ssh_legacy,
		 base_url, rest_auth_type, rest_username, header_name,
		 knowledge_sources, created_at, updated_at)
		SELECT
			lower(hex(randomblob(16))), id, 'ssh', ip, port,
			COALESCE(username,''), COALESCE(auth_type,''),
			COALESCE(encrypted_credential,''), COALESCE(encrypted_passphrase,''),
			COALESCE(ssh_key_id,''), COALESCE(ssh_legacy,0),
			'', '', '', '[]', created_at, updated_at
		FROM hosts
		WHERE id NOT IN (SELECT host_id FROM access_faces)`)
	// Context compaction
	db.Exec(`CREATE TABLE IF NOT EXISTS conversation_summaries (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id  TEXT NOT NULL,
		up_to_message_id TEXT NOT NULL,
		chunks           TEXT NOT NULL,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(conversation_id)
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at)`)
	return nil
}
