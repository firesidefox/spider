package auth

import (
	"os"
	"testing"
	"time"
)

func TestJWTSignAndVerify(t *testing.T) {
	dir := t.TempDir()
	m, err := NewJWTManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	token, err := m.Sign("user-1", "admin")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := m.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" {
		t.Errorf("claims mismatch: %+v", claims)
	}
}

func TestJWTKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	m1, _ := NewJWTManager(dir)
	token, _ := m1.Sign("u", "viewer")

	m2, _ := NewJWTManager(dir)
	if _, err := m2.Verify(token); err != nil {
		t.Errorf("key not persisted: %v", err)
	}
}

func TestJWTInvalidToken(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewJWTManager(dir)
	if _, err := m.Verify("invalid.token.here"); err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestTokenGenerate(t *testing.T) {
	tok, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !IsAPIToken(tok) {
		t.Errorf("expected spd_ prefix, got %q", tok)
	}
	if len(tok) != 4+64 { // "spd_" + 32 bytes hex
		t.Errorf("unexpected length %d", len(tok))
	}
}

func TestTokenHash(t *testing.T) {
	tok, _ := Generate()
	h1 := Hash(tok)
	h2 := Hash(tok)
	if h1 != h2 {
		t.Error("hash not deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(h1))
	}
}

// 确保 time 包被使用（避免 import 报错）
var _ = time.Now
var _ = os.TempDir
