package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFeedMetricTimingsAndDiagnosticsRoundTrip(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	want := FeedMetric{Type: "diagnostic", Event: "navigation-complete", InputID: 42,
		SeriesID: "710000000000001", EpisodeID: "710000000000002",
		SessionID: "session", InputSource: "keyboard", WaitingMS: 350, RenderedMS: 700,
		BufferedSec: 1.25, TimeToFirstFrameMS: 1617, PlaybackInfoMS: 1106, MediaStartupMS: 511,
		HandoffFirstFrameMS: 262, FirstFrameSource: "loaded-data", At: "2026-11-04T07:27:31Z"}
	if err := store.AppendFeedMetrics([]FeedMetric{want}); err != nil {
		t.Fatal(err)
	}
	var first, info, startup, handoff int64
	var source, payload string
	if err := store.db.QueryRow(`SELECT time_to_first_frame_ms, playback_info_ms, media_startup_ms,
		handoff_first_frame_ms, first_frame_source, diagnostic_json FROM feed_metrics`).Scan(
		&first, &info, &startup, &handoff, &source, &payload); err != nil {
		t.Fatal(err)
	}
	if first != 1617 || info != 1106 || startup != 511 || handoff != 262 || source != "loaded-data" {
		t.Fatalf("timing columns shifted: %d %d %d %d %q", first, info, startup, handoff, source)
	}
	var got FeedMetric
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("diagnostic fields lost: got %+v; want %+v", got, want)
	}
}

func TestFeedMetricDiagnosticMigrationPreservesExistingRows(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendFeedMetrics([]FeedMetric{{Type: "switch", DurationMS: 321}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`ALTER TABLE feed_metrics DROP COLUMN diagnostic_json`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.AppendFeedMetrics([]FeedMetric{{Type: "diagnostic", Event: "navigation-input", InputID: 7}}); err != nil {
		t.Fatal(err)
	}
	var duration int64
	if err := store.db.QueryRow(`SELECT duration_ms FROM feed_metrics WHERE type='switch'`).Scan(&duration); err != nil || duration != 321 {
		t.Fatalf("existing row changed during migration: %d %v", duration, err)
	}
	var payload string
	if err := store.db.QueryRow(`SELECT diagnostic_json FROM feed_metrics WHERE type='diagnostic'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var got FeedMetric
	if err := json.Unmarshal([]byte(payload), &got); err != nil || got.InputID != 7 {
		t.Fatalf("new diagnostics not persisted after migration: %+v %v", got, err)
	}
}

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
