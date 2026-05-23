package knowledge

import (
	"context"
	"time"

	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/rag"
)

// KnowledgePlugin defines the interface for knowledge base operations.
type KnowledgePlugin interface {
	// Group management (top-level container)
	CreateGroup(ctx context.Context, name string) (*Group, error)
	ListGroups(ctx context.Context) ([]Group, error)
	GetGroupsByIDs(ctx context.Context, ids []int) ([]Group, error)
	DeleteGroup(ctx context.Context, groupID int) error
	DeleteGroups(ctx context.Context, groupIDs []int) error

	// Document management
	ListDocuments(ctx context.Context, groupID int) ([]Document, error)
	GetDocument(ctx context.Context, docID int) (*Document, error)
	GetDocumentsByIDs(ctx context.Context, ids []int) ([]Document, error)
	DeleteDocuments(ctx context.Context, docIDs []int) error
	MoveDocuments(ctx context.Context, docIDs []int, targetGroupID int) error

	// Retrieval
	CatalogSections(ctx context.Context, scope Scope) ([]Section, error)
	CatalogEntries(ctx context.Context, sectionID int) ([]EntrySummary, error)
	FetchEntries(ctx context.Context, entryIDs []int) ([]Entry, error)
	Search(ctx context.Context, query string, scope Scope, topK int, embedder rag.Embedder) ([]Entry, error)

	// Import
	ImportDocument(ctx context.Context, req ImportRequest) (*ImportResult, error)
	ReindexDocument(ctx context.Context, docID int, req ImportRequest) (*ImportResult, error)
}

// Scope defines the search/retrieval scope.
// Type "group" targets a top-level knowledge container (vendor/project).
// Type "document" targets a single uploaded file.
type Scope struct {
	Type string // "group" | "document"
	ID   int
}

// Group represents a top-level knowledge container (vendor/project).
type Group struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Document represents an imported document.
type Document struct {
	ID         int       `json:"id"`
	GroupID    int       `json:"group_id"`
	Name       string    `json:"name"`
	DocType    string    `json:"doc_type"` // "openapi" | "markdown"
	RawContent string    `json:"raw_content"`
	Filename   string    `json:"filename"`
	Status     string    `json:"status"` // "pending" | "indexing" | "ready" | "error"
	ErrorMsg   string    `json:"error_msg"`
	EntryCount int       `json:"entry_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Section represents a logical section within a document.
type Section struct {
	ID         int    `json:"id"`
	DocumentID int    `json:"document_id"`
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Position   int    `json:"position"`
	EntryCount int    `json:"entry_count"`
}

// EntrySummary provides a lightweight view of an entry.
type EntrySummary struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// Entry represents a searchable knowledge entry.
type Entry struct {
	ID         int    `json:"id"`
	DocumentID int    `json:"document_id"`
	SectionID  *int   `json:"section_id,omitempty"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Content    string `json:"content"`
	Embedding  []byte `json:"-"`
	Position   int    `json:"position"`
}

// ImportRequest specifies parameters for importing a document.
type ImportRequest struct {
	GroupID   int
	Name      string
	Content   []byte
	Filename  string
	DocType   string       // "openapi" | "markdown" — if empty, auto-detect
	LLMClient llm.Client   // required for markdown parsing and clustering
	Embedder  rag.Embedder // optional, for generating embeddings
}

// ImportResult contains the outcome of a document import.
type ImportResult struct {
	DocumentID   int
	EntryCount   int
	SectionCount int
	Sections     []Section
}
