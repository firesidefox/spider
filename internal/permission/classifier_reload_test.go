package permission_test

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/permission"
)

func TestClassifier_Reload(t *testing.T) {
	c := permission.NewClassifier(nil)
	got := c.Classify(context.Background(), "docker rm abc")
	if got.Level != permission.L3Dangerous {
		t.Fatalf("before reload: got %s, want L3", got.Level)
	}
	c.Reload([]config.RuleConfig{
		{Pattern: `^docker\s+rm`, Level: "L2", Description: "docker remove"},
	})
	got = c.Classify(context.Background(), "docker rm abc")
	if got.Level != permission.L2Write {
		t.Fatalf("after reload: got %s, want L2", got.Level)
	}
	if got.Source != permission.SourceStatic {
		t.Fatalf("source = %s, want static", got.Source)
	}
	got = c.Classify(context.Background(), "ls -la")
	if got.Level != permission.L1Read {
		t.Fatalf("ls after reload: got %s, want L1", got.Level)
	}
}

func TestClassifier_ReloadInvalidPattern(t *testing.T) {
	c := permission.NewClassifier(nil)
	c.Reload([]config.RuleConfig{
		{Pattern: `^valid`, Level: "L2"},
		{Pattern: `[invalid`, Level: "L3"},
	})
	got := c.Classify(context.Background(), "valid cmd")
	if got.Level != permission.L2Write {
		t.Fatalf("got %s, want L2", got.Level)
	}
}
