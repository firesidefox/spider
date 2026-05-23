package store

import (
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
)

func newAccessFaceTestStores(t *testing.T) (*HostStore, *AccessFaceStore) {
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
	return NewHostStore(database), NewAccessFaceStore(database, cm)
}

func addAccessFaceTestHost(t *testing.T, hosts *HostStore) string {
	t.Helper()
	h, err := hosts.Add(&models.AddHostRequest{Name: "gateway", IP: "10.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return h.ID
}

func TestAccessFaceKBModeValidation(t *testing.T) {
	hosts, faces := newAccessFaceTestStores(t)
	hostID := addAccessFaceTestHost(t, hosts)

	face, err := faces.Add(hostID, &models.AddAccessFaceRequest{
		Type: models.FaceSSH,
		IP:   "10.0.0.1",
		Port: 22,
	})
	if err != nil {
		t.Fatal(err)
	}
	if face.KBMode != "none" {
		t.Fatalf("expected default kb_mode none, got %q", face.KBMode)
	}
	if len(face.KnowledgeSources) != 0 {
		t.Fatalf("expected default empty knowledge_sources, got %+v", face.KnowledgeSources)
	}

	specific := "specific"
	if _, err := faces.Update(face.ID, &models.UpdateAccessFaceRequest{KBMode: &specific}); err == nil || !strings.Contains(err.Error(), "kb_mode=specific requires at least one knowledge_source") {
		t.Fatalf("expected specific empty sources error, got %v", err)
	}

	many := make([]models.KnowledgeSourceRef, 11)
	for i := range many {
		many[i] = models.KnowledgeSourceRef{Type: "group", ID: i + 1}
	}
	if _, err := faces.Update(face.ID, &models.UpdateAccessFaceRequest{KBMode: &specific, KnowledgeSources: many}); err == nil || !strings.Contains(err.Error(), "knowledge_sources exceeds limit of 10") {
		t.Fatalf("expected source limit error, got %v", err)
	}

	none := "none"
	updated, err := faces.Update(face.ID, &models.UpdateAccessFaceRequest{
		KBMode:           &none,
		KnowledgeSources: []models.KnowledgeSourceRef{{Type: "group", ID: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.KBMode != "none" || len(updated.KnowledgeSources) != 0 {
		t.Fatalf("expected none mode to clear sources, got mode=%q sources=%+v", updated.KBMode, updated.KnowledgeSources)
	}
}

func TestAccessFaceKBModeMergeSemantics(t *testing.T) {
	hosts, faces := newAccessFaceTestStores(t)
	hostID := addAccessFaceTestHost(t, hosts)
	specific := "specific"
	face, err := faces.Add(hostID, &models.AddAccessFaceRequest{
		Type:             models.FaceSSH,
		IP:               "10.0.0.1",
		Port:             22,
		KBMode:           specific,
		KnowledgeSources: []models.KnowledgeSourceRef{{Type: "group", ID: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	nextPort := 2222
	updated, err := faces.Update(face.ID, &models.UpdateAccessFaceRequest{Port: &nextPort})
	if err != nil {
		t.Fatal(err)
	}
	if updated.KBMode != "specific" {
		t.Fatalf("expected kb_mode preserved, got %q", updated.KBMode)
	}
	if len(updated.KnowledgeSources) != 1 || updated.KnowledgeSources[0].Type != "group" || updated.KnowledgeSources[0].ID != 1 {
		t.Fatalf("expected knowledge_sources preserved, got %+v", updated.KnowledgeSources)
	}

	none := "none"
	updated, err = faces.Update(face.ID, &models.UpdateAccessFaceRequest{KBMode: &none})
	if err != nil {
		t.Fatal(err)
	}
	if updated.KBMode != "none" || len(updated.KnowledgeSources) != 0 {
		t.Fatalf("expected none mode to clear sources, got mode=%q sources=%+v", updated.KBMode, updated.KnowledgeSources)
	}
}
