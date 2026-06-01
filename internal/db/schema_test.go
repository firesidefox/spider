package db

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestPrometheusTables(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if _, err := sqldb.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(sqldb); err != nil {
		t.Fatal(err)
	}
	// Insert a host so FK on host_id is satisfied
	now := time.Now().UTC()
	if _, err := sqldb.Exec(`INSERT INTO hosts (id,name,ip,created_at,updated_at) VALUES ('host1','host1','127.0.0.1',?,?)`, now, now); err != nil {
		t.Fatalf("insert host: %v", err)
	}
	// Insert a topology so FK on topology_id is satisfied
	if _, err := sqldb.Exec(`INSERT INTO topologies (id,name,created_at,updated_at) VALUES ('topo1','topo1',datetime('now'),datetime('now'))`); err != nil {
		t.Fatalf("insert topology: %v", err)
	}
	// sources table
	_, err = sqldb.Exec(`INSERT INTO prometheus_sources
		(id,name,base_url,timeout_seconds,auth_type,username,
		 encrypted_password,encrypted_token,skip_tls_verify,created_at,updated_at)
		VALUES ('s1','test','http://localhost:9090',30,'none','','','',0,
				datetime('now'),datetime('now'))`)
	if err != nil {
		t.Fatalf("insert prometheus_sources: %v", err)
	}
	// bindings table — topology_layer scope
	_, err = sqldb.Exec(`INSERT INTO prometheus_bindings
		(id,source_id,scope_type,topology_id,layer,host_id,created_at)
		VALUES ('b1','s1','topology_layer','topo1','server',NULL,datetime('now'))`)
	if err != nil {
		t.Fatalf("insert prometheus_bindings (topology_layer): %v", err)
	}
	// bindings table — host scope
	_, err = sqldb.Exec(`INSERT INTO prometheus_bindings
		(id,source_id,scope_type,topology_id,layer,host_id,created_at)
		VALUES ('b2','s1','host',NULL,NULL,'host1',datetime('now'))`)
	if err != nil {
		t.Fatalf("insert prometheus_bindings (host): %v", err)
	}
	// unique constraint on (topology_id, layer)
	_, err = sqldb.Exec(`INSERT INTO prometheus_bindings
		(id,source_id,scope_type,topology_id,layer,host_id,created_at)
		VALUES ('b3','s1','topology_layer','topo1','server',NULL,datetime('now'))`)
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate (topology_id, layer)")
	}
	// unique constraint on host_id
	_, err = sqldb.Exec(`INSERT INTO prometheus_bindings
		(id,source_id,scope_type,topology_id,layer,host_id,created_at)
		VALUES ('b4','s1','host',NULL,NULL,'host1',datetime('now'))`)
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate host_id")
	}
}

func TestMigratePreservesHostKnowledgeSources(t *testing.T) {
	testMigratePreservesHostKnowledgeSources(t, true)
}

func TestMigratePreservesHostKnowledgeSourcesWhenHostHasNoAccessFace(t *testing.T) {
	testMigratePreservesHostKnowledgeSources(t, false)
}

func testMigratePreservesHostKnowledgeSources(t *testing.T, seedAccessFace bool) {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })

	if _, err := sqldb.Exec(`
		CREATE TABLE hosts (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 22,
			username TEXT NOT NULL DEFAULT '',
			auth_type TEXT NOT NULL DEFAULT '',
			encrypted_credential TEXT NOT NULL DEFAULT '',
			encrypted_passphrase TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE TABLE access_faces (
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
			probe_port INTEGER NOT NULL DEFAULT 0,
			probe_interval INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE TABLE host_knowledge_sources (
			host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
			type TEXT NOT NULL CHECK(type IN ('group','doc')),
			ref_id INTEGER NOT NULL,
			PRIMARY KEY (host_id, type, ref_id)
		);
	`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := sqldb.Exec(`INSERT INTO hosts (id, name, ip, created_at, updated_at) VALUES ('h1', 'host1', '10.0.0.1', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if seedAccessFace {
		if _, err := sqldb.Exec(`INSERT INTO access_faces (id, host_id, type, ip, port, created_at, updated_at) VALUES ('f1', 'h1', 'ssh', '10.0.0.1', 22, ?, ?)`, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sqldb.Exec(`INSERT INTO host_knowledge_sources (host_id, type, ref_id) VALUES ('h1', 'group', 7), ('h1', 'doc', 13)`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(sqldb); err != nil {
		t.Fatal(err)
	}

	var mode, raw string
	if err := sqldb.QueryRow(`SELECT kb_mode, knowledge_sources FROM access_faces WHERE host_id='h1' ORDER BY created_at LIMIT 1`).Scan(&mode, &raw); err != nil {
		t.Fatal(err)
	}
	if mode != "specific" {
		t.Fatalf("expected kb_mode specific, got %q", mode)
	}
	var sources []struct {
		Type string `json:"type"`
		ID   int    `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 migrated sources, got %s", raw)
	}
	seen := map[string]bool{}
	for _, src := range sources {
		seen[src.Type] = seen[src.Type] || (src.Type == "group" && src.ID == 7) || (src.Type == "doc" && src.ID == 13)
	}
	if !seen["group"] || !seen["doc"] {
		t.Fatalf("missing migrated refs: %s", raw)
	}
	var tableName string
	err = sqldb.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='host_knowledge_sources'`).Scan(&tableName)
	if err != sql.ErrNoRows {
		t.Fatalf("expected host_knowledge_sources table dropped, got name=%q err=%v", tableName, err)
	}
}

// TestFreshMigrate verifies that a brand-new in-memory database receives every
// migration entry in the registry and ends up with the expected core tables.
func TestFreshMigrate(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })

	if err := Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != len(migrations) {
		t.Fatalf("schema_migrations rows = %d, want %d", count, len(migrations))
	}

	// Spot-check that key tables across the registry exist.
	wantTables := []string{
		"hosts", "execution_logs", "users", "api_tokens", "ssh_keys",
		"conversations", "messages", "documents", "approvals",
		"providers", "provider_models",
		"document_groups", "rag_config",
		"access_faces", "host_fingerprints", "host_memories",
		"conversation_summaries", "todo_tasks",
		"topologies", "topology_nodes", "topology_edges",
		"tasks", "task_runs", "notify_channels",
		"knowledge_groups", "knowledge_documents", "knowledge_sections", "knowledge_entries",
		"prometheus_sources", "prometheus_bindings",
	}
	for _, name := range wantTables {
		var got string
		err := sqldb.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
		if err != nil {
			t.Errorf("expected table %s: %v", name, err)
		}
	}
}

// TestIdempotentMigrate verifies that running Migrate twice on the same DB is
// a no-op and does not duplicate schema_migrations rows.
func TestIdempotentMigrate(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })

	if err := Migrate(sqldb); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	var first int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(sqldb); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var second int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("schema_migrations row count changed: first=%d second=%d", first, second)
	}
	if second != len(migrations) {
		t.Fatalf("schema_migrations rows = %d, want %d", second, len(migrations))
	}
}

// TestOldDBMigrate simulates an existing pre-registry database (tables present,
// no schema_migrations row) and confirms Migrate runs to completion and
// records every migration ID.
func TestOldDBMigrate(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })

	// Pre-create the hosts table to mimic a database that already ran the
	// pre-registry monolithic migrate(). The full registry must still
	// converge on the same schema.
	if _, err := sqldb.Exec(`CREATE TABLE hosts (
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
	)`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(sqldb); err != nil {
		t.Fatalf("Migrate on old DB: %v", err)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != len(migrations) {
		t.Fatalf("schema_migrations rows = %d, want %d", count, len(migrations))
	}

	// Verify that a column added by a later migration exists.
	var hasUIPrefs int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='ui_prefs'`).Scan(&hasUIPrefs); err != nil {
		t.Fatal(err)
	}
	if hasUIPrefs != 1 {
		t.Fatalf("users.ui_prefs missing after Migrate")
	}
}
