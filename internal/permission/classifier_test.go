package permission_test

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/permission"
)

func TestClassifier_StaticRules(t *testing.T) {
	c := permission.NewClassifier(nil)

	tests := []struct {
		cmd       string
		wantLevel permission.RiskLevel
	}{
		{"ls -la /tmp", permission.L1Read},
		{"cat /etc/hosts", permission.L1Read},
		{"ps aux", permission.L1Read},
		{"df -h", permission.L1Read},
		{"grep -r foo /var/log", permission.L1Read},
		{"echo hello > /tmp/test.txt", permission.L2Write},
		{"cp /tmp/a /tmp/b", permission.L2Write},
		{"systemctl restart nginx", permission.L2Write},
		{"rm /tmp/test.txt", permission.L3Dangerous},
		{"systemctl stop nginx", permission.L3Dangerous},
		{"kill 1234", permission.L3Dangerous},
		{"rm -rf /tmp/old", permission.L4Destroy},
		{"dd if=/dev/zero of=/dev/sda", permission.L4Destroy},
		{"unknown-custom-tool --flag", permission.L3Dangerous},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := c.Classify(context.Background(), tt.cmd)
			if got.Level != tt.wantLevel {
				t.Errorf("Classify(%q) = %v, want %v (reason: %s)", tt.cmd, got.Level, tt.wantLevel, got.Reason)
			}
		})
	}
}
