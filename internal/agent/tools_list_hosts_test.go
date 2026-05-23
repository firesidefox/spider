package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func TestGetHostsIncludesKBMode(t *testing.T) {
	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)
	kb := knowledge.NewStore(database)
	group, err := kb.CreateGroup(context.Background(), "Ops")
	if err != nil {
		t.Fatal(err)
	}
	docID := insertAgentKnowledgeDoc(t, database, group.ID, "Nginx")
	host, err := hosts.Add(&models.AddHostRequest{Name: "gateway", IP: "10.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	specific := "specific"
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type:             models.FaceRESTAPI,
		IP:               "10.0.0.1",
		Port:             443,
		KBMode:           specific,
		KnowledgeSources: []models.KnowledgeSourceRef{{Type: "group", ID: group.ID}, {Type: "doc", ID: docID}},
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewGetHostsTool(hosts, faces)
	tool.knowledgeStore = kb
	result, err := tool.Execute(context.Background(), map[string]any{"name": "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	var got []struct {
		AccessFaces []struct {
			Type             string `json:"type"`
			KBMode           string `json:"kb_mode"`
			KnowledgeSources []struct {
				Type      string `json:"type"`
				Name      string `json:"name"`
				Title     string `json:"title"`
				GroupName string `json:"group_name"`
			} `json:"knowledge_sources"`
		} `json:"access_faces"`
	}
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, face := range got[0].AccessFaces {
		if face.Type == "restapi" {
			found = true
			if face.KBMode != "specific" {
				t.Fatalf("expected restapi face kb_mode specific, got %q", face.KBMode)
			}
			if len(face.KnowledgeSources) != 2 || face.KnowledgeSources[0].Name != "Ops" || face.KnowledgeSources[1].Title != "Nginx" || face.KnowledgeSources[1].GroupName != "Ops" {
				t.Fatalf("expected enriched knowledge sources, got %+v", face.KnowledgeSources)
			}
		}
	}
	if !found {
		t.Fatal("restapi face not found in GetHosts output")
	}
}

func insertAgentKnowledgeDoc(t *testing.T, database *sql.DB, groupID int, name string) int {
	t.Helper()
	res, err := database.Exec(`INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		groupID, name, "markdown", "content", "doc.md", "ready")
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}
