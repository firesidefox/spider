package db

import (
	"database/sql"
	"encoding/json"
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

type kbSourceRef struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

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
		kb_mode TEXT NOT NULL DEFAULT 'none',
		knowledge_sources TEXT NOT NULL DEFAULT '[]',
		probe_port INTEGER NOT NULL DEFAULT 0,
		probe_interval INTEGER NOT NULL DEFAULT 0,
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
	// New hosts columns
	db.Exec("ALTER TABLE hosts ADD COLUMN notes TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE access_faces ADD COLUMN ssh_login_input TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE access_faces ADD COLUMN hmac_algo TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE access_faces ADD COLUMN probe_port INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE access_faces ADD COLUMN probe_interval INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE access_faces ADD COLUMN rest_scheme TEXT NOT NULL DEFAULT 'http'")
	db.Exec("ALTER TABLE access_faces ADD COLUMN kb_mode TEXT NOT NULL DEFAULT 'none'")
	db.Exec("ALTER TABLE hosts ADD COLUMN product_name TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE hosts ADD COLUMN product_version TEXT NOT NULL DEFAULT ''")
	// Data migration: seed one SSH access_face per existing host.
	// Idempotent: WHERE clause skips hosts that already have an SSH face.
	if _, err := db.Exec(`INSERT OR IGNORE INTO access_faces
		(id, host_id, type, ip, port, username, auth_type,
		 encrypted_credential, encrypted_passphrase, ssh_key_id, ssh_legacy,
		 base_url, rest_auth_type, rest_username, header_name,
		 kb_mode, knowledge_sources, created_at, updated_at)
		SELECT
			lower(hex(randomblob(16))), id, 'ssh', ip, port,
			COALESCE(username,''), COALESCE(auth_type,''),
			COALESCE(encrypted_credential,''), COALESCE(encrypted_passphrase,''),
			COALESCE(ssh_key_id,''), COALESCE(ssh_legacy,0),
			'', '', '', '', 'none', '[]', created_at, updated_at
		FROM hosts
		WHERE id NOT IN (SELECT host_id FROM access_faces)`); err != nil {
		return err
	}
	if err := migrateAccessFaceKBMode(db); err != nil {
		return err
	}
	if err := migrateHostKnowledgeSources(db); err != nil {
		return err
	}
	db.Exec(`DROP TABLE IF EXISTS host_knowledge_sources`)
	// Context compaction
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS conversation_summaries (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id  TEXT NOT NULL,
		up_to_message_id TEXT NOT NULL,
		chunks           TEXT NOT NULL,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(conversation_id)
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS todo_tasks (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT    NOT NULL,
		subject         TEXT    NOT NULL,
		active_form     TEXT    NOT NULL DEFAULT '',
		description     TEXT    NOT NULL DEFAULT '',
		status          TEXT    NOT NULL DEFAULT 'pending',
		owner           TEXT    NOT NULL DEFAULT '',
		created_at      DATETIME NOT NULL,
		updated_at      DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topologies (
		id         TEXT PRIMARY KEY,
		name       TEXT UNIQUE NOT NULL,
		notes      TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topology_nodes (
		id          TEXT PRIMARY KEY,
		topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
		layer       TEXT NOT NULL DEFAULT '',
		name        TEXT NOT NULL,
		role        TEXT NOT NULL DEFAULT '',
		host_id     TEXT REFERENCES hosts(id) ON DELETE SET NULL,
		notes       TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topology_edges (
		id          TEXT PRIMARY KEY,
		topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
		from_node   TEXT NOT NULL REFERENCES topology_nodes(id) ON DELETE CASCADE,
		to_node     TEXT NOT NULL REFERENCES topology_nodes(id) ON DELETE CASCADE,
		created_at  DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_topology_edges_unique ON topology_edges(from_node, to_node)`); err != nil {
		return err
	}
	// migrate existing topology_nodes: add layer, drop group_id
	db.Exec("ALTER TABLE topology_nodes ADD COLUMN layer TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE topology_nodes DROP COLUMN group_id")
	db.Exec("ALTER TABLE topology_nodes ADD COLUMN pos_x REAL NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE topology_nodes ADD COLUMN pos_y REAL NOT NULL DEFAULT 0")
	// Task automation tables
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		id                   TEXT PRIMARY KEY,
		name                 TEXT NOT NULL,
		goal                 TEXT NOT NULL,
		host_ids             TEXT NOT NULL DEFAULT '[]',
		schedule             TEXT NOT NULL DEFAULT '',
		notify_mode          TEXT NOT NULL DEFAULT 'none',
		run_retention_days   INTEGER NOT NULL DEFAULT 30,
		timeout_minutes      INTEGER NOT NULL DEFAULT 30,
		status               TEXT NOT NULL DEFAULT 'active',
		created_at           DATETIME NOT NULL,
		updated_at           DATETIME NOT NULL,
		source_conv_id       TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS task_runs (
		id          TEXT PRIMARY KEY,
		task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		started_at  DATETIME NOT NULL,
		finished_at DATETIME,
		status      TEXT NOT NULL DEFAULT 'running',
		raw_output  TEXT NOT NULL DEFAULT '',
		summary     TEXT NOT NULL DEFAULT '',
		alerted     INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_runs_task_id ON task_runs(task_id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS notify_channels (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		name              TEXT NOT NULL,
		type              TEXT NOT NULL DEFAULT 'dingtalk',
		encrypted_config  TEXT NOT NULL DEFAULT '',
		enabled           INTEGER NOT NULL DEFAULT 1,
		created_at        DATETIME NOT NULL,
		updated_at        DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_task_runs_one_running ON task_runs(task_id) WHERE status='running'`); err != nil {
		return err
	}
	// Migrate todo_tasks: drop turn_id and blocked_by columns
	var hasTurnID int
	db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('todo_tasks') WHERE name='turn_id'`).Scan(&hasTurnID)
	if hasTurnID > 0 {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS todo_tasks_new (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT    NOT NULL,
			subject         TEXT    NOT NULL,
			active_form     TEXT    NOT NULL DEFAULT '',
			description     TEXT    NOT NULL DEFAULT '',
			status          TEXT    NOT NULL DEFAULT 'pending',
			owner           TEXT    NOT NULL DEFAULT '',
			created_at      DATETIME NOT NULL,
			updated_at      DATETIME NOT NULL,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		)`); err != nil {
			return err
		}
		if _, err := db.Exec(`INSERT INTO todo_tasks_new
			SELECT id, conversation_id, subject, active_form, description, status, owner, created_at, updated_at
			FROM todo_tasks`); err != nil {
			return err
		}
		if _, err := db.Exec(`DROP TABLE todo_tasks`); err != nil {
			return err
		}
		if _, err := db.Exec(`ALTER TABLE todo_tasks_new RENAME TO todo_tasks`); err != nil {
			return err
		}
	} else {
		// Fresh install: active_form may already exist; ignore duplicate column error
		db.Exec(`ALTER TABLE todo_tasks ADD COLUMN active_form TEXT NOT NULL DEFAULT ''`)
	}
	// Idempotent: ignore duplicate column error
	db.Exec(`ALTER TABLE users ADD COLUMN ui_prefs TEXT NOT NULL DEFAULT '{}'`)
	db.Exec(`ALTER TABLE todo_tasks ADD COLUMN seq INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_todo_tasks_conv ON todo_tasks(conversation_id)`)

	// Knowledge base tables (groups are the top-level container)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_groups (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_documents (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id    INTEGER NOT NULL REFERENCES knowledge_groups(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		doc_type    TEXT NOT NULL CHECK(doc_type IN ('openapi','markdown')),
		raw_content TEXT NOT NULL DEFAULT '',
		filename    TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','indexing','ready','error')),
		error_msg   TEXT NOT NULL DEFAULT '',
		entry_count INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_sections (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		summary     TEXT NOT NULL DEFAULT '',
		position    INTEGER NOT NULL DEFAULT 0
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_entries (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
		section_id  INTEGER REFERENCES knowledge_sections(id) ON DELETE SET NULL,
		title       TEXT NOT NULL,
		summary     TEXT NOT NULL DEFAULT '',
		content     TEXT NOT NULL,
		embedding   BLOB,
		position    INTEGER NOT NULL DEFAULT 0
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_docs_group_id ON knowledge_documents(group_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_sections_doc_id ON knowledge_sections(document_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_entries_doc_id ON knowledge_entries(document_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_entries_section_id ON knowledge_entries(section_id)`)

	// Migrate knowledge_groups: remove kb_id column if it exists
	var kbIDExists int
	db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('knowledge_groups') WHERE name='kb_id'`).Scan(&kbIDExists)
	if kbIDExists > 0 {
		db.Exec(`
			CREATE TABLE knowledge_groups_new (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				name       TEXT NOT NULL,
				created_at DATETIME NOT NULL
			)`)
		db.Exec(`INSERT INTO knowledge_groups_new (id, name, created_at) SELECT id, name, created_at FROM knowledge_groups`)
		db.Exec(`DROP TABLE knowledge_groups`)
		db.Exec(`ALTER TABLE knowledge_groups_new RENAME TO knowledge_groups`)
	}

	db.Exec("ALTER TABLE documents ADD COLUMN description TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE document_groups ADD COLUMN description TEXT NOT NULL DEFAULT ''")
	return nil
}

func migrateAccessFaceKBMode(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, kb_mode, knowledge_sources FROM access_faces`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type update struct {
		id      string
		mode    string
		sources []kbSourceRef
	}
	var updates []update
	for rows.Next() {
		var id, mode, raw string
		if err := rows.Scan(&id, &mode, &raw); err != nil {
			return err
		}
		var sources []kbSourceRef
		if err := json.Unmarshal([]byte(raw), &sources); err != nil {
			continue
		}
		hasSentinel := false
		hasValid := false
		filtered := make([]kbSourceRef, 0, len(sources))
		for _, src := range sources {
			if src.Type == "none" && src.ID == 0 {
				hasSentinel = true
				continue
			}
			if (src.Type == "group" || src.Type == "doc") && src.ID > 0 {
				hasValid = true
				filtered = append(filtered, src)
			}
		}
		switch {
		case hasSentinel:
			updates = append(updates, update{id: id, mode: "none", sources: []kbSourceRef{}})
		case mode == "none" && hasValid:
			updates = append(updates, update{id: id, mode: "specific", sources: filtered})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, u := range updates {
		raw, err := json.Marshal(u.sources)
		if err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE access_faces SET kb_mode=?, knowledge_sources=? WHERE id=?`, u.mode, string(raw), u.id); err != nil {
			return err
		}
	}
	return nil
}

func migrateHostKnowledgeSources(db *sql.DB) error {
	var legacyTableExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='host_knowledge_sources'`).Scan(&legacyTableExists); err != nil {
		return err
	}
	if legacyTableExists == 0 {
		return nil
	}
	rows, err := db.Query(`
		SELECT host_id, type, ref_id
		FROM host_knowledge_sources
		ORDER BY host_id, type, ref_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	byHost := map[string][]kbSourceRef{}
	for rows.Next() {
		var hostID, refType string
		var refID int
		if err := rows.Scan(&hostID, &refType, &refID); err != nil {
			return err
		}
		if (refType == "group" || refType == "doc") && refID > 0 {
			byHost[hostID] = append(byHost[hostID], kbSourceRef{Type: refType, ID: refID})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for hostID, legacySources := range byHost {
		var faceID, mode, raw string
		err := db.QueryRow(`
			SELECT id, kb_mode, knowledge_sources
			FROM access_faces
			WHERE host_id = ?
			ORDER BY CASE WHEN type='ssh' THEN 0 ELSE 1 END, created_at
			LIMIT 1`, hostID).Scan(&faceID, &mode, &raw)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		var sources []kbSourceRef
		if err := json.Unmarshal([]byte(raw), &sources); err != nil {
			sources = nil
		}
		merged := mergeKBSources(sources, legacySources)
		if len(merged) == 0 {
			continue
		}
		out, err := json.Marshal(merged)
		if err != nil {
			return err
		}
		if mode == "none" {
			mode = "specific"
		}
		if _, err := db.Exec(`UPDATE access_faces SET kb_mode=?, knowledge_sources=? WHERE id=?`, mode, string(out), faceID); err != nil {
			return err
		}
	}
	return nil
}

func mergeKBSources(existing, legacy []kbSourceRef) []kbSourceRef {
	seen := map[kbSourceRef]struct{}{}
	out := make([]kbSourceRef, 0, len(existing)+len(legacy))
	for _, src := range existing {
		if (src.Type != "group" && src.Type != "doc") || src.ID <= 0 {
			continue
		}
		if _, ok := seen[src]; ok {
			continue
		}
		seen[src] = struct{}{}
		out = append(out, src)
	}
	for _, src := range legacy {
		if (src.Type != "group" && src.Type != "doc") || src.ID <= 0 {
			continue
		}
		if _, ok := seen[src]; ok {
			continue
		}
		seen[src] = struct{}{}
		out = append(out, src)
	}
	return out
}
