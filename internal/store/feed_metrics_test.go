package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyFeedMetricsJSONLMigratesToSQLite(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	path := filepath.Join(dir, "feed-metrics.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"switch","networkProfile":"medium","fullFallback":true}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	count, err := store.FeedMetricCount()
	if err != nil || count != 1 {
		t.Fatalf("migrated metric count = %d, %v; want 1, nil", count, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("feed-metrics.jsonl still exists, err = %v", err)
	}
}
