package knowledge

import (
	"context"
	"time"
)

// KnowledgePlugin defines the interface for knowledge base operations.
type KnowledgePlugin interface {
	// KB management
	CreateKB(ctx context.Context, name string) (*KnowledgeBase, error)
	ListKBs(ctx context.Context) ([]KnowledgeBase, error)
	DeleteKB(ctx context.Context, kbID int) error

	// Group management
	CreateGroup(ctx context.Context, kbID int, name string) (*Group, error)
	ListGroups(ctx context.Context, kbID int) ([]Group, error)
	DeleteGroup(ctx context.Context, groupID int) error

	// Document management
	ListDocuments(ctx context.Context, groupID int) ([]Document, error)
	GetDocument(ctx context.Context, docID int) (*Document, error)
	DeleteDocuments(ctx context.Context, docIDs []int) error

	// Retrieval
	CatalogSections(ctx context.Context, scope Scope) ([]Section, error)
	CatalogEntries(ctx context.Context, sectionID int) ([]EntrySummary, error)
	FetchEntries(ctx context.Context, entryIDs []int) ([]Entry, error)
	Search(ctx context.Context, query string, scope Scope, topK int) ([]Entry, error)

	// Import
	ImportDocument(ctx context.Context, req ImportRequest) (*ImportResult, error)
}

// Scope defines the search/retrieval scope.
type Scope struct {
	Type string // "kb" | "group" | "document"
	ID   int
}

// KnowledgeBase represents a knowledge base.
type KnowledgeBase struct {
	ID        int
	Name      string
	CreatedAt time.Time
}

// Group represents a document group within a knowledge base.
type Group struct {
	ID        int
	KBID      int
	Name      string
	CreatedAt time.Time
}

// Document represents an imported document.
type Document struct {
	ID          int
	GroupID     int
	Name        string
	DocType     string // "openapi" | "markdown"
	RawContent  string
	Filename    string
	Status      string // "pending" | "indexing" | "ready" | "error"
	ErrorMsg    string
	EntryCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Section represents a logical section within a document.
type Section struct {
	ID         int
	DocumentID int
	Name       string
	Summary    string
	Position   int
	EntryCount int
}

// EntrySummary provides a lightweight view of an entry.
type EntrySummary struct {
	ID      int
	Title   string
	Summary string
}

// Entry represents a searchable knowledge entry.
type Entry struct {
	ID         int
	DocumentID int
	SectionID  *int
	Title      string
	Summary    string
	Content    string
	Embedding  []byte
	Position   int
}

// ImportRequest specifies parameters for importing a document.
type ImportRequest struct {
	GroupID  int
	Name     string
	Content  []byte
	Filename string
}

// ImportResult contains the outcome of a document import.
type ImportResult struct {
	DocumentID   int
	EntryCount   int
	SectionCount int
	Sections     []Section
}
