package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func newHostKBTestApp(t *testing.T) (*mcppkg.App, string) {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &mcppkg.App{
		HostStore:       store.NewHostStore(database),
		AccessFaceStore: store.NewAccessFaceStore(database, cm),
		KnowledgeStore:  knowledge.NewStore(database),
	}
	group, err := app.KnowledgeStore.CreateGroup(context.Background(), "Ops")
	if err != nil {
		t.Fatal(err)
	}
	docID, err := insertKnowledgeDoc(t, database, group.ID, "Nginx")
	if err != nil {
		t.Fatal(err)
	}
	host, err := app.HostStore.Add(&models.AddHostRequest{Name: "gateway", IP: "10.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.AccessFaceStore.Add(host.ID, &models.AddAccessFaceRequest{
		Type:   models.FaceRESTAPI,
		IP:     "10.0.0.1",
		Port:   443,
		KBMode: "specific",
		KnowledgeSources: []models.KnowledgeSourceRef{
			{Type: "group", ID: group.ID},
			{Type: "doc", ID: docID},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return app, host.ID
}

func insertKnowledgeDoc(t *testing.T, database *sql.DB, groupID int, name string) (int, error) {
	t.Helper()
	res, err := database.Exec(`INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		groupID, name, "markdown", "content", "doc.md", "ready")
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func TestListHostsIncludesEnrichedAccessFaces(t *testing.T) {
	app, _ := newHostKBTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	w := httptest.NewRecorder()

	listHosts(app, w, req)

	assertHostResponseHasEnrichedFace(t, w)
}

func TestGetHostIncludesEnrichedAccessFaces(t *testing.T) {
	app, hostID := newHostKBTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/"+hostID, nil)
	w := httptest.NewRecorder()

	getHost(app, w, req, hostID)

	assertHostResponseHasEnrichedFace(t, w)
}

func TestListAccessFacesIncludesEnrichedKnowledgeSources(t *testing.T) {
	app, hostID := newHostKBTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/"+hostID+"/faces", nil)
	w := httptest.NewRecorder()

	listAccessFaces(app, w, req, hostID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var faces []struct {
		KBMode           string `json:"kb_mode"`
		KnowledgeSources []struct {
			Type      string `json:"type"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Title     string `json:"title"`
			GroupName string `json:"group_name"`
		} `json:"knowledge_sources"`
	}
	if err := json.NewDecoder(w.Body).Decode(&faces); err != nil {
		t.Fatal(err)
	}
	assertEnrichedSources(t, faces[0].KBMode, faces[0].KnowledgeSources)
}

func assertHostResponseHasEnrichedFace(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.Bytes()
	var hosts []struct {
		AccessFaces []struct {
			KBMode           string `json:"kb_mode"`
			KnowledgeSources []struct {
				Type      string `json:"type"`
				ID        int    `json:"id"`
				Name      string `json:"name"`
				Title     string `json:"title"`
				GroupName string `json:"group_name"`
			} `json:"knowledge_sources"`
		} `json:"access_faces"`
	}
	if err := json.Unmarshal(body, &hosts); err != nil {
		var host struct {
			AccessFaces []struct {
				KBMode           string `json:"kb_mode"`
				KnowledgeSources []struct {
					Type      string `json:"type"`
					ID        int    `json:"id"`
					Name      string `json:"name"`
					Title     string `json:"title"`
					GroupName string `json:"group_name"`
				} `json:"knowledge_sources"`
			} `json:"access_faces"`
		}
		if err := json.Unmarshal(body, &host); err != nil {
			t.Fatal(err)
		}
		if len(host.AccessFaces) == 0 {
			t.Fatalf("expected host response with access faces, got %s", string(body))
		}
		assertEnrichedSources(t, host.AccessFaces[0].KBMode, host.AccessFaces[0].KnowledgeSources)
		return
	}
	if len(hosts) == 0 || len(hosts[0].AccessFaces) == 0 {
		t.Fatalf("expected host response with access faces, got %+v", hosts)
	}
	assertEnrichedSources(t, hosts[0].AccessFaces[0].KBMode, hosts[0].AccessFaces[0].KnowledgeSources)
}

func assertEnrichedSources(t *testing.T, mode string, sources []struct {
	Type      string `json:"type"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	GroupName string `json:"group_name"`
}) {
	t.Helper()
	if mode != "specific" {
		t.Fatalf("expected kb_mode specific, got %q", mode)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %+v", sources)
	}
	if sources[0].Type != "group" || sources[0].Name != "Ops" {
		t.Fatalf("expected enriched group source, got %+v", sources[0])
	}
	if sources[1].Type != "doc" || sources[1].Title != "Nginx" || sources[1].GroupName != "Ops" {
		t.Fatalf("expected enriched doc source, got %+v", sources[1])
	}
}
