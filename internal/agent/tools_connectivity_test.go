package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func TestCheckConnectivityTool_Name(t *testing.T) {
	tool := &CheckConnectivityTool{}
	if tool.Name() != "CheckConnectivity" {
		t.Errorf("got %q, want CheckConnectivity", tool.Name())
	}
}

func TestCheckConnectivityTool_RiskLevel(t *testing.T) {
	tool := &CheckConnectivityTool{}
	if tool.DefaultRiskLevel() != RiskL1 {
		t.Error("CheckConnectivity must be L1")
	}
}

func TestCheckConnectivityTool_ConcurrencySafe(t *testing.T) {
	tool := &CheckConnectivityTool{}
	if !tool.IsConcurrencySafe(nil) {
		t.Error("CheckConnectivity must be concurrency safe")
	}
}

func TestCheckConnectivityTool_InputSchema(t *testing.T) {
	tool := &CheckConnectivityTool{}
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["host_ids"]; !ok {
		t.Error("host_ids missing from InputSchema")
	}
	required, _ := schema["required"].([]string)
	for _, r := range required {
		if r == "host_ids" {
			t.Error("host_ids must not be required")
		}
	}
}

func TestCheckConnectivityTool_SystemPromptSection(t *testing.T) {
	tool := &CheckConnectivityTool{}
	section := tool.SystemPromptSection()
	if section == "" {
		t.Error("SystemPromptSection must not be empty")
	}
	for _, keyword := range []string{"CheckConnectivity", "Explore", "unreachable"} {
		if !strings.Contains(section, keyword) {
			t.Errorf("SystemPromptSection missing keyword %q", keyword)
		}
	}
}

func TestCheckConnectivityTool_TCPFaceProbe_Reachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "test-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type: models.FaceSSH,
		IP:   "127.0.0.1",
		Port: port,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v — content: %s", err, result.Content)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if len(got[0].Faces) != 1 {
		t.Fatalf("expected 1 face, got %d", len(got[0].Faces))
	}
	if !got[0].Faces[0].Reachable {
		t.Errorf("face should be reachable, got error: %s", got[0].Faces[0].Error)
	}
}

func TestCheckConnectivityTool_TCPFaceProbe_Unreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "dead-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type: models.FaceSSH,
		IP:   "127.0.0.1",
		Port: port,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v — content: %s", err, result.Content)
	}
	if len(got) != 1 || len(got[0].Faces) != 1 {
		t.Fatalf("unexpected shape: %+v", got)
	}
	if got[0].Faces[0].Reachable {
		t.Error("face should be unreachable")
	}
}

func TestCheckConnectivityTool_ProbePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	probePort := ln.Addr().(*net.TCPAddr).Port

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "probe-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type:      models.FaceRESTAPI,
		IP:        "127.0.0.1",
		Port:      9999,
		ProbePort: probePort,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got[0].Faces[0].Reachable {
		t.Errorf("should use ProbePort and be reachable, got: %s", got[0].Faces[0].Error)
	}
	if got[0].Faces[0].Port != probePort {
		t.Errorf("reported port should be probePort %d, got %d", probePort, got[0].Faces[0].Port)
	}
}

func TestCheckConnectivityTool_EmptyHostIDs_AllHosts(t *testing.T) {
	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	for _, name := range []string{"h1", "h2", "h3"} {
		if _, err := hosts.Add(&models.AddHostRequest{Name: name, IP: "127.0.0.1", Tags: []string{}}); err != nil {
			t.Fatal(err)
		}
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 results, got %d", len(got))
	}
}

func TestCheckConnectivityTool_RESTAPIFaceReachable(t *testing.T) {
	srv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "api-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type: models.FaceRESTAPI,
		IP:   "127.0.0.1",
		Port: port,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got[0].Faces[0].Reachable {
		t.Errorf("REST API face should be reachable: %s", got[0].Faces[0].Error)
	}
}
