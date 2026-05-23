package db

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

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
