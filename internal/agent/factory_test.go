package agent

import (
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func TestSystemPromptStaticPrefixStable(t *testing.T) {
	// Same factory config, different host counts → static block must be byte-identical
	f := newTestFactory(t)
	blocks1 := f.BuildSystemPrompt()
	staticPrefix1 := blocks1[0].Text

	// Add 5 cisco hosts
	for i := 0; i < 5; i++ {
		addTestHost(t, f.Hosts, fmt.Sprintf("cisco-%d", i), "cisco")
	}
	blocks2 := f.BuildSystemPrompt()
	staticPrefix2 := blocks2[0].Text

	// Add 3 huawei hosts
	for i := 0; i < 3; i++ {
		addTestHost(t, f.Hosts, fmt.Sprintf("huawei-%d", i), "huawei")
	}
	blocks3 := f.BuildSystemPrompt()
	staticPrefix3 := blocks3[0].Text

	if staticPrefix1 != staticPrefix2 {
		t.Fatalf("static prefix changed after adding cisco hosts:\n  len1=%d\n  len2=%d\n  first diff at byte %d",
			len(staticPrefix1), len(staticPrefix2), firstDiffOffset(staticPrefix1, staticPrefix2))
	}

	if staticPrefix2 != staticPrefix3 {
		t.Fatalf("static prefix changed after adding huawei hosts:\n  len2=%d\n  len3=%d\n  first diff at byte %d",
			len(staticPrefix2), len(staticPrefix3), firstDiffOffset(staticPrefix2, staticPrefix3))
	}
}

func firstDiffOffset(a, b string) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return minLen
}

func newTestFactory(t *testing.T) *Factory {
	// Create in-memory DB + empty factory
	sqldb := setupTestDB(t)
	hosts := store.NewHostStore(sqldb)
	return &Factory{
		Hosts: hosts,
		// Other fields zero-valued, test only cares about BuildSystemPrompt
	}
}

func addTestHost(t *testing.T, hosts *store.HostStore, hostname, vendor string) {
	_, err := hosts.Add(&models.AddHostRequest{
		Name:   hostname,
		IP:     "192.168.1.1",
		Vendor: vendor,
		Tags:   []string{"test"},
	})
	if err != nil {
		t.Fatalf("addTestHost failed: %v", err)
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })

	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return sqldb
}
