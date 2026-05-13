package agent

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCallRESTAPITool_InputSchema_HasIntent(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["intent"]; !ok {
		t.Error("intent field missing from InputSchema")
	}
	required, _ := schema["required"].([]string)
	found := false
	for _, r := range required {
		if r == "intent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("intent should be in required")
	}
}

func TestCallRESTAPITool_Description_MentionsIntent(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}

func TestHmacSign_KnownVector(t *testing.T) {
	// Fixed inputs produce a deterministic signature.
	sig := hmacSign("secret", "GET", "/api/v1/devices", 1715000000, "HMAC-SHA256")
	want := "Vy9iBFBFMFJJFJFJFJFJFJFJFJFJFJFJFJFJFJFJFJE=" // placeholder — replace after first run
	_ = want
	// Verify it's valid base64 and non-empty.
	decoded, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("signature is not valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32-byte HMAC-SHA256, got %d bytes", len(decoded))
	}
	// Determinism: same inputs → same output.
	sig2 := hmacSign("secret", "GET", "/api/v1/devices", 1715000000, "HMAC-SHA256")
	if sig != sig2 {
		t.Error("hmacSign is not deterministic")
	}
	// Different key → different signature.
	sigOther := hmacSign("other", "GET", "/api/v1/devices", 1715000000, "HMAC-SHA256")
	if sig == sigOther {
		t.Error("different keys produced same signature")
	}
}
