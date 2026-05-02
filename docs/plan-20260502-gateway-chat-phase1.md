# Gateway Chat Phase 1: 基础层 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation layer — database migrations, config extension, LLM client, and store layer — so Phase 2 (Agent Engine) has all dependencies ready.

**Architecture:** Extend existing Config with multi-model LLM/Embedding configs. Add new SQLite tables (conversations, messages, documents, pending_confirmations) and extend hosts table. Implement LLM client interface with Claude provider. All following existing spider.ai patterns (store pattern, parameterized queries, idempotent migrations).

**Tech Stack:** Go 1.23, modernc.org/sqlite, Claude API (HTTP), existing spider.ai patterns

**Spec Reference:** `docs/spec-20260502-gateway-chat.md`

---

### Task 1: Config 扩展 — LLM 与 Embedding 多模型配置

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for LLM config parsing**

```go
func TestLoadLLMConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: /tmp/test
llm:
  active: claude-sonnet
  models:
    - id: claude-sonnet
      provider: claude
      api_key: sk-ant-test
      model: claude-sonnet-4-6
      max_tokens: 4096
    - id: gpt4o
      provider: openai
      api_key: sk-test
      model: gpt-4o
      max_tokens: 4096
embedding:
  active: openai-small
  models:
    - id: openai-small
      provider: openai
      api_key: sk-test
      model: text-embedding-3-small
      dimensions: 1536
`
	os.WriteFile(cfgPath, []byte(content), 0600)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LLM.Active != "claude-sonnet" {
		t.Errorf("LLM.Active = %q, want %q", cfg.LLM.Active, "claude-sonnet")
	}
	if len(cfg.LLM.Models) != 2 {
		t.Fatalf("LLM.Models len = %d, want 2", len(cfg.LLM.Models))
	}
	if cfg.LLM.Models[0].Provider != "claude" {
		t.Errorf("Models[0].Provider = %q, want %q", cfg.LLM.Models[0].Provider, "claude")
	}
	if cfg.Embedding.Active != "openai-small" {
		t.Errorf("Embedding.Active = %q, want %q", cfg.Embedding.Active, "openai-small")
	}
	if cfg.Embedding.Models[0].Dimensions != 1536 {
		t.Errorf("Dimensions = %d, want 1536", cfg.Embedding.Models[0].Dimensions)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/config/ -run TestLoadLLMConfig -v`
Expected: FAIL — `cfg.LLM` fields don't exist yet

- [ ] **Step 3: Add LLM and Embedding config structs**

Add to `internal/config/config.go`:

```go
type LLMModelConfig struct {
	ID        string `yaml:"id"`
	Provider  string `yaml:"provider"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

type LLMConfig struct {
	Active string           `yaml:"active"`
	Models []LLMModelConfig `yaml:"models"`
}

type EmbeddingModelConfig struct {
	ID         string `yaml:"id"`
	Provider   string `yaml:"provider"`
	APIKey     string `yaml:"api_key"`
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions"`
}

type EmbeddingConfig struct {
	Active string                `yaml:"active"`
	Models []EmbeddingModelConfig `yaml:"models"`
}
```

Add fields to existing `Config` struct:

```go
LLM       LLMConfig       `yaml:"llm"`
Embedding EmbeddingConfig `yaml:"embedding"`
```

- [ ] **Step 4: Add helper methods to resolve active model**

Add to `internal/config/config.go`:

```go
func (c *LLMConfig) ActiveModel() *LLMModelConfig {
	for i := range c.Models {
		if c.Models[i].ID == c.Active {
			return &c.Models[i]
		}
	}
	return nil
}

func (c *EmbeddingConfig) ActiveModel() *EmbeddingModelConfig {
	for i := range c.Models {
		if c.Models[i].ID == c.Active {
			return &c.Models[i]
		}
	}
	return nil
}

func (m *LLMModelConfig) ResolveAPIKey() string {
	envKey := os.Getenv("SPIDER_LLM_APIKEY_" + m.ID)
	if envKey != "" {
		return envKey
	}
	return m.APIKey
}

func (m *EmbeddingModelConfig) ResolveAPIKey() string {
	envKey := os.Getenv("SPIDER_EMBEDDING_APIKEY_" + m.ID)
	if envKey != "" {
		return envKey
	}
	return m.APIKey
}
```

- [ ] **Step 5: Write test for ActiveModel and ResolveAPIKey**

```go
func TestActiveModel(t *testing.T) {
	cfg := &LLMConfig{
		Active: "gpt4o",
		Models: []LLMModelConfig{
			{ID: "claude-sonnet", Provider: "claude"},
			{ID: "gpt4o", Provider: "openai"},
		},
	}
	m := cfg.ActiveModel()
	if m == nil || m.ID != "gpt4o" {
		t.Errorf("ActiveModel = %v, want gpt4o", m)
	}
	cfg.Active = "nonexistent"
	if cfg.ActiveModel() != nil {
		t.Error("ActiveModel should return nil for nonexistent")
	}
}

func TestResolveAPIKey(t *testing.T) {
	m := &LLMModelConfig{ID: "test", APIKey: "from-config"}
	if m.ResolveAPIKey() != "from-config" {
		t.Errorf("ResolveAPIKey = %q, want from-config", m.ResolveAPIKey())
	}
	t.Setenv("SPIDER_LLM_APIKEY_test", "from-env")
	if m.ResolveAPIKey() != "from-env" {
		t.Errorf("ResolveAPIKey = %q, want from-env", m.ResolveAPIKey())
	}
}
```

- [ ] **Step 6: Run all config tests**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/config/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add multi-model LLM and embedding configuration"
```

---

### Task 2: Database 迁移 — Host 扩展 + 新表

**Files:**
- Modify: `internal/db/schema.go`
- Modify: `internal/models/host.go`

- [ ] **Step 1: Add Host device fields to model**

In `internal/models/host.go`, add to Host struct:

```go
DeviceType      string `json:"device_type,omitempty"`
Vendor          string `json:"vendor,omitempty"`
Model           string `json:"model,omitempty"`
CLIType         string `json:"cli_type,omitempty"`
FirmwareVersion string `json:"firmware_version,omitempty"`
```

- [ ] **Step 2: Add migration SQL to schema.go**

In `internal/db/schema.go`, add to the `migrate()` function following existing ALTER TABLE pattern (suppress duplicate column errors):

```go
alterStmts := []string{
    "ALTER TABLE hosts ADD COLUMN device_type TEXT",
    "ALTER TABLE hosts ADD COLUMN vendor TEXT",
    "ALTER TABLE hosts ADD COLUMN model TEXT",
    "ALTER TABLE hosts ADD COLUMN cli_type TEXT",
    "ALTER TABLE hosts ADD COLUMN firmware_version TEXT",
}
for _, stmt := range alterStmts {
    db.Exec(stmt) // ignore "duplicate column" errors
}
```

- [ ] **Step 3: Add new tables to schema**

Add CREATE TABLE statements to schema constant:

```sql
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
```

- [ ] **Step 4: Run server to verify migration**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./cmd/spider/ && echo "build ok"`
Expected: build ok

- [ ] **Step 5: Commit**

```bash
git add internal/db/schema.go internal/models/host.go
git commit -m "feat(db): add gateway chat tables and host device fields"
```

---

### Task 3: Models — Conversation, Message, Document, PendingConfirmation

**Files:**
- Create: `internal/models/conversation.go`
- Create: `internal/models/document.go`

- [ ] **Step 1: Create conversation.go**

```go
package models

import "time"

type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type PendingConfirmation struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	ToolName       string     `json:"tool_name"`
	ToolInput      string     `json:"tool_input"`
	RiskLevel      string     `json:"risk_level"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}
```

- [ ] **Step 2: Create document.go**

```go
package models

import "time"

type Document struct {
	ID         int       `json:"id"`
	Vendor     string    `json:"vendor"`
	CLIType    string    `json:"cli_type"`
	DocType    string    `json:"doc_type"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Embedding  []byte    `json:"-"`
	SourceFile string    `json:"source_file"`
	ChunkIndex int       `json:"chunk_index"`
	CreatedAt  time.Time `json:"created_at"`
}
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/models/conversation.go internal/models/document.go
git commit -m "feat(models): add conversation, message, document, pending_confirmation models"
```

---

### Task 4: Store — ConversationStore

**Files:**
- Create: `internal/store/conversation.go`
- Create: `internal/store/conversation_test.go`

- [ ] **Step 1: Write failing tests**

```go
package store

import (
	"database/sql"
	"testing"

	"github.com/spiderai/spider/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestConversationStore_CreateAndGet(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	conv, err := s.Create("user-1", "test title")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if conv.ID == "" || conv.Title != "test title" {
		t.Errorf("unexpected conv: %+v", conv)
	}

	got, err := s.GetByID(conv.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "test title" {
		t.Errorf("Title = %q, want %q", got.Title, "test title")
	}
}

func TestConversationStore_ListByUser(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	s.Create("user-1", "conv A")
	s.Create("user-1", "conv B")
	s.Create("user-2", "conv C")

	list, err := s.ListByUser("user-1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestConversationStore_Delete(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	conv, _ := s.Create("user-1", "to delete")
	err := s.Delete(conv.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.GetByID(conv.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/ -run TestConversationStore -v`
Expected: FAIL — ConversationStore not defined

- [ ] **Step 3: Implement ConversationStore**

```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

type ConversationStore struct {
	db *sql.DB
}

func NewConversationStore(db *sql.DB) *ConversationStore {
	return &ConversationStore{db: db}
}

func (s *ConversationStore) Create(userID, title string) (*models.Conversation, error) {
	conv := &models.Conversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     title,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_, err := s.db.Exec(
		"INSERT INTO conversations (id, user_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		conv.ID, conv.UserID, conv.Title, conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert conversation: %w", err)
	}
	return conv, nil
}

func (s *ConversationStore) GetByID(id string) (*models.Conversation, error) {
	row := s.db.QueryRow(
		"SELECT id, user_id, title, created_at, updated_at FROM conversations WHERE id = ?", id,
	)
	var c models.Conversation
	err := row.Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("scan conversation: %w", err)
	}
	return &c, nil
}

func (s *ConversationStore) ListByUser(userID string) ([]*models.Conversation, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, title, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	var list []*models.Conversation
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan conversation row: %w", err)
		}
		list = append(list, &c)
	}
	return list, nil
}

func (s *ConversationStore) UpdateTitle(id, title string) error {
	_, err := s.db.Exec(
		"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?",
		title, time.Now().UTC(), id,
	)
	return err
}

func (s *ConversationStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	return err
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/ -run TestConversationStore -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/conversation.go internal/store/conversation_test.go
git commit -m "feat(store): add ConversationStore with CRUD operations"
```

---

### Task 5: Store — MessageStore

**Files:**
- Create: `internal/store/message.go`
- Create: `internal/store/message_test.go`

- [ ] **Step 1: Write failing tests**

```go
package store

import "testing"

func TestMessageStore_SaveAndList(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "test")

	err := ms.Save(conv.ID, "user", `{"type":"text","text":"hello"}`)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	err = ms.Save(conv.ID, "assistant", `{"type":"text","text":"hi"}`)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	msgs, err := ms.ListByConversation(conv.ID)
	if err != nil {
		t.Fatalf("ListByConversation: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("msgs[0].Role = %q, want user", msgs[0].Role)
	}
}

func TestMessageStore_DeleteByConversation(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "test")
	ms.Save(conv.ID, "user", `{"text":"hello"}`)

	err := ms.DeleteByConversation(conv.ID)
	if err != nil {
		t.Fatalf("DeleteByConversation: %v", err)
	}
	msgs, _ := ms.ListByConversation(conv.ID)
	if len(msgs) != 0 {
		t.Errorf("len = %d, want 0", len(msgs))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/ -run TestMessageStore -v`
Expected: FAIL

- [ ] **Step 3: Implement MessageStore**

```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

type MessageStore struct {
	db *sql.DB
}

func NewMessageStore(db *sql.DB) *MessageStore {
	return &MessageStore{db: db}
}

func (s *MessageStore) Save(conversationID, role, content string) error {
	_, err := s.db.Exec(
		"INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
		uuid.New().String(), conversationID, role, content, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

func (s *MessageStore) ListByConversation(conversationID string) ([]*models.Message, error) {
	rows, err := s.db.Query(
		"SELECT id, conversation_id, role, content, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	var list []*models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		list = append(list, &m)
	}
	return list, nil
}

func (s *MessageStore) DeleteByConversation(conversationID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE conversation_id = ?", conversationID)
	return err
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/ -run TestMessageStore -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/message.go internal/store/message_test.go
git commit -m "feat(store): add MessageStore for conversation messages"
```

---

### Task 6: Store — DocumentStore

**Files:**
- Create: `internal/store/document.go`
- Create: `internal/store/document_test.go`

- [ ] **Step 1: Write failing tests**

```go
package store

import "testing"

func TestDocumentStore_SaveAndSearch(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	err := ds.Save("huawei", "vrp", "cli_ref", "display interface", "display interface [type] [number]", nil, "cli-ref.md", 0)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	err = ds.Save("cisco", "ios", "cli_ref", "show interface", "show interface [type] [number]", nil, "cli-ref.md", 1)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	docs, err := ds.ListByVendor("huawei", "vrp")
	if err != nil {
		t.Fatalf("ListByVendor: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("len = %d, want 1", len(docs))
	}
	if docs[0].Title != "display interface" {
		t.Errorf("Title = %q, want %q", docs[0].Title, "display interface")
	}
}

func TestDocumentStore_DeleteBySource(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	ds.Save("huawei", "vrp", "cli_ref", "cmd1", "content1", nil, "file-a.md", 0)
	ds.Save("huawei", "vrp", "cli_ref", "cmd2", "content2", nil, "file-a.md", 1)
	ds.Save("cisco", "ios", "cli_ref", "cmd3", "content3", nil, "file-b.md", 0)

	err := ds.DeleteBySource("file-a.md")
	if err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}
	all, _ := ds.List()
	if len(all) != 1 {
		t.Errorf("len = %d, want 1", len(all))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/ -run TestDocumentStore -v`
Expected: FAIL

- [ ] **Step 3: Implement DocumentStore**

```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type DocumentStore struct {
	db *sql.DB
}

func NewDocumentStore(db *sql.DB) *DocumentStore {
	return &DocumentStore{db: db}
}

func (s *DocumentStore) Save(vendor, cliType, docType, title, content string, embedding []byte, sourceFile string, chunkIndex int) error {
	_, err := s.db.Exec(
		"INSERT INTO documents (vendor, cli_type, doc_type, title, content, embedding, source_file, chunk_index, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		vendor, cliType, docType, title, content, embedding, sourceFile, chunkIndex, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (s *DocumentStore) List() ([]*models.Document, error) {
	rows, err := s.db.Query("SELECT id, vendor, cli_type, doc_type, title, content, source_file, chunk_index, created_at FROM documents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) ListByVendor(vendor, cliType string) ([]*models.Document, error) {
	rows, err := s.db.Query(
		"SELECT id, vendor, cli_type, doc_type, title, content, source_file, chunk_index, created_at FROM documents WHERE vendor = ? AND cli_type = ? ORDER BY id",
		vendor, cliType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) DeleteBySource(sourceFile string) error {
	_, err := s.db.Exec("DELETE FROM documents WHERE source_file = ?", sourceFile)
	return err
}

func (s *DocumentStore) Delete(id int) error {
	_, err := s.db.Exec("DELETE FROM documents WHERE id = ?", id)
	return err
}

func scanDocumentRows(rows *sql.Rows) ([]*models.Document, error) {
	var list []*models.Document
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.Vendor, &d.CLIType, &d.DocType, &d.Title, &d.Content, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		list = append(list, &d)
	}
	return list, nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/ -run TestDocumentStore -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/document.go internal/store/document_test.go
git commit -m "feat(store): add DocumentStore for RAG document chunks"
```

---

### Task 7: LLM Client — 接口 + Claude Provider

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/claude.go`
- Create: `internal/llm/claude_test.go`

- [ ] **Step 1: Create LLM client interface**

```go
package llm

import "context"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type StreamEvent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

type ChatRequest struct {
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens"`
}

type Client interface {
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}
```

- [ ] **Step 2: Implement Claude provider**

```go
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spiderai/spider/internal/config"
)

type ClaudeClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewClaudeClient(cfg *config.LLMModelConfig) *ClaudeClient {
	return &ClaudeClient{
		apiKey: cfg.ResolveAPIKey(),
		model:  cfg.Model,
		http:   &http.Client{},
	}
}

func (c *ClaudeClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": req.MaxTokens,
		"system":     req.System,
		"messages":   req.Messages,
		"stream":     true,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamEvent, 32)
	go c.readSSE(resp.Body, ch)
	return ch, nil
}

func (c *ClaudeClient) readSSE(body io.ReadCloser, ch chan<- StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 || line[:6] != "data: " {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			return
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}

		eventType, _ := raw["type"].(string)
		switch eventType {
		case "content_block_delta":
			delta, _ := raw["delta"].(map[string]any)
			deltaType, _ := delta["type"].(string)
			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				ch <- StreamEvent{Type: "text_delta", Text: text}
			} else if deltaType == "input_json_delta" {
				// tool input streaming — accumulate externally
				text, _ := delta["partial_json"].(string)
				ch <- StreamEvent{Type: "tool_input_delta", Text: text}
			}
		case "content_block_start":
			cb, _ := raw["content_block"].(map[string]any)
			cbType, _ := cb["type"].(string)
			if cbType == "tool_use" {
				name, _ := cb["name"].(string)
				id, _ := cb["id"].(string)
				ch <- StreamEvent{
					Type:     "tool_start",
					ToolCall: &ToolCall{ID: id, Name: name},
				}
			}
		case "message_stop":
			ch <- StreamEvent{Type: "message_stop"}
			return
		}
	}
}
```

- [ ] **Step 3: Write test for NewClaudeClient construction**

```go
package llm

import (
	"testing"

	"github.com/spiderai/spider/internal/config"
)

func TestNewClaudeClient(t *testing.T) {
	cfg := &config.LLMModelConfig{
		ID:       "test",
		Provider: "claude",
		APIKey:   "sk-test-key",
		Model:    "claude-sonnet-4-6",
	}
	client := NewClaudeClient(cfg)
	if client.model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", client.model)
	}
	if client.apiKey != "sk-test-key" {
		t.Errorf("apiKey = %q, want sk-test-key", client.apiKey)
	}
}
```

- [ ] **Step 4: Add factory function**

Add to `internal/llm/client.go`:

```go
func NewClient(cfg *config.LLMModelConfig) (Client, error) {
	switch cfg.Provider {
	case "claude":
		return NewClaudeClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
```

- [ ] **Step 5: Run tests**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./internal/llm/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/llm/
git commit -m "feat(llm): add LLM client interface and Claude streaming provider"
```

---

### Task 8: Host Store 扩展 — 设备字段读写

**Files:**
- Modify: `internal/store/host.go`

- [ ] **Step 1: Update host store queries to include new fields**

Find all SELECT/INSERT/UPDATE queries in `internal/store/host.go` and add the 5 new columns: `device_type`, `vendor`, `model`, `cli_type`, `firmware_version`.

Update scan functions to read new fields. Update Create/Update methods to write new fields.

Follow existing pattern — the new fields are nullable, so use `sql.NullString` for scanning or handle empty strings.

- [ ] **Step 2: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/store/host.go
git commit -m "feat(store): extend host store with device_type/vendor/model/cli_type/firmware fields"
```

---

### Task 9: Settings API 扩展 — LLM 配置读写

**Files:**
- Modify: `internal/api/settings.go`

- [ ] **Step 1: Extend settingsResponse with LLM fields**

Add to `settingsResponse` struct:

```go
LLM       config.LLMConfig       `json:"llm"`
Embedding config.EmbeddingConfig `json:"embedding"`
```

- [ ] **Step 2: Update getSettings to return LLM config**

Add to `getSettings` response:

```go
LLM:       app.Config.LLM,
Embedding: app.Config.Embedding,
```

- [ ] **Step 3: Update updateSettings to accept LLM config**

Add handling in `updateSettings`:

```go
if req.LLM.Active != "" {
    app.Config.LLM = req.LLM
}
if req.Embedding.Active != "" {
    app.Config.Embedding = req.Embedding
}
```

Mask API keys in response (show only last 4 chars).

- [ ] **Step 4: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/api/settings.go
git commit -m "feat(api): extend settings endpoint with LLM and embedding config"
```

---

### Task 10: 全量构建验证

- [ ] **Step 1: Run full build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: success

- [ ] **Step 2: Run all tests**

Run: `cd /Users/cw/fty.ai/spider.ai && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 3: Verify new tables exist**

Start server briefly, check SQLite schema has all new tables and columns.

Phase 1 complete. All foundation pieces ready for Phase 2 (Agent Engine).
