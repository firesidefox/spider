package api

import (
	"net/http"
	"strings"
)

func registerKnowledgeRoutes(mux *http.ServeMux, d routeDeps) {
	// RAG config
	mux.HandleFunc("/api/v1/rag-config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getRagConfig(d.app, w, r)
		case http.MethodPut:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				putRagConfig(d.app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/rag-config/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				validateRagConfig(d.app, w, r)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/rag-config/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				listRagModels(d.app, w, r)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Documents
	mux.HandleFunc("/api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listDocuments(d.app, w, r)
		case http.MethodPost:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ingestDocument(d.app, w, r)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteBatchDocuments(d.app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/documents/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			searchDocuments(d.app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Document groups
	mux.HandleFunc("/api/v1/document-groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listGroups(d.app, w, r)
		case http.MethodPost:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createGroup(d.app, w, r)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteBatchGroups(d.app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/document-groups/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/document-groups/"):]
		if r.Method == http.MethodPost && strings.HasSuffix(rest, "/regenerate-description") {
			idStr := strings.TrimSuffix(rest, "/regenerate-description")
			regenerateGroupDescription(d.app, w, r, idStr)
			return
		}
		if r.Method == http.MethodPut {
			updateGroupDescription(d.app, w, r, rest)
			return
		}
		id := rest
		switch r.Method {
		case http.MethodPatch:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				renameGroup(d.app, w, r, id)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteGroup(d.app, w, r, id)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/documents/"):]
		if r.Method == http.MethodPost && strings.HasSuffix(rest, "/regenerate-description") {
			idStr := strings.TrimSuffix(rest, "/regenerate-description")
			regenerateDocDescription(d.app, w, r, idStr)
			return
		}
		if r.Method == http.MethodPut {
			updateDocDescription(d.app, w, r, rest)
			return
		}
		id := rest
		switch r.Method {
		case http.MethodDelete:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteDocument(d.app, w, r, id)
			})).ServeHTTP(w, r)
		case http.MethodPatch:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				moveDocumentToGroup(d.app, w, r, id)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Knowledge groups (top-level)
	mux.HandleFunc("/api/v1/knowledge-groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listKnowledgeGroups(d.app.KnowledgeStore, w, r)
		case http.MethodPost:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createKnowledgeGroup(d.app.KnowledgeStore, w, r)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteKnowledgeGroupsBatch(d.app.KnowledgeStore, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/knowledge-groups/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/knowledge-groups/"):]
		id := rest
		sub := ""
		if idx := indexOf(rest, '/'); idx >= 0 {
			id = rest[:idx]
			sub = rest[idx+1:]
		}
		if sub == "" {
			if r.Method == http.MethodDelete {
				d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					deleteKnowledgeGroup(d.app.KnowledgeStore, w, r, id)
				})).ServeHTTP(w, r)
				return
			}
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if sub == "documents" {
			if r.Method == http.MethodGet {
				listKnowledgeGroupDocuments(d.app.KnowledgeStore, w, r, id)
				return
			}
		}
		http.NotFound(w, r)
	})

	// Knowledge documents (with path suffix)
	mux.HandleFunc("/api/v1/knowledge-documents/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			rest := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge-documents/")
			if rest == "" {
				http.NotFound(w, r)
				return
			}
			if strings.HasSuffix(rest, "/sections") {
				docID := strings.TrimSuffix(rest, "/sections")
				getKnowledgeDocumentSections(d.app.KnowledgeStore, w, r, docID)
				return
			}
			getKnowledgeDocument(d.app.KnowledgeStore, w, r, rest)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/knowledge-sections/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			rest := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge-sections/")
			if rest == "" {
				http.NotFound(w, r)
				return
			}
			if strings.HasSuffix(rest, "/entries") {
				sectionID := strings.TrimSuffix(rest, "/entries")
				getKnowledgeSectionEntries(d.app.KnowledgeStore, w, r, sectionID)
				return
			}
			http.NotFound(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/knowledge-entries/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge-entries/")
		if r.Method == http.MethodGet {
			if rest == "" {
				http.NotFound(w, r)
				return
			}
			getKnowledgeEntry(d.app.KnowledgeStore, w, r, rest)
			return
		}
		if r.Method == http.MethodPost {
			if id, ok := strings.CutSuffix(rest, "/try"); ok {
				d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					tryKnowledgeEntry(d.app.KnowledgeStore, d.app.PrometheusSourceStore, w, r, id)
				})).ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Knowledge documents (batch operations)
	mux.HandleFunc("/api/v1/knowledge-documents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteKnowledgeDocuments(d.app.KnowledgeStore, w, r)
			})).ServeHTTP(w, r)
		case http.MethodPatch:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				moveKnowledgeDocuments(d.app.KnowledgeStore, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/knowledge-documents/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				importKnowledgeDocument(d.app.KnowledgeStore, d.app, d.kbEmbedder, w, r)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/knowledge-documents/reindex", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reindexKnowledgeDocuments(d.app.KnowledgeStore, d.app, d.kbEmbedder, w, r)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
}
