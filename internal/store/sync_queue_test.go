package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func newQueueStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestHistoryAckKeepsNewerProgressWithSameTimestamp(t *testing.T) {
	s := newQueueStore(t)
	item := HistoryItem{SeriesID: "710000000000001", UpdatedAt: 100, PositionSec: 10}
	if err := s.QueueHistoryUpload(item, "710000000000101", "A"); err != nil {
		t.Fatal(err)
	}
	old, err := s.PendingHistoryUploads(10, "A")
	if err != nil || len(old) != 1 {
		t.Fatalf("snapshot: %v, %v", old, err)
	}
	item.PositionSec = 20
	if err := s.QueueHistoryUpload(item, "710000000000102", "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.AckHistoryUploads(old); err != nil {
		t.Fatal(err)
	}
	newer, err := s.PendingHistoryUploads(10, "A")
	if err != nil || len(newer) != 1 || newer[0].PositionSec != 20 || newer[0].EpisodeID != "710000000000102" {
		t.Fatalf("newer progress lost: %v, %v", newer, err)
	}
	if err := s.AckHistoryUploads(newer); err != nil {
		t.Fatal(err)
	}
	remaining, err := s.PendingHistoryUploads(10, "A")
	if err != nil || len(remaining) != 0 {
		t.Fatalf("confirmed progress not removed: %v, %v", remaining, err)
	}
}

func TestFavoriteAckDoesNotCrossOwnersOrReinsertedIntent(t *testing.T) {
	s := newQueueStore(t)
	const id = "710000000000001"
	if err := s.QueueFavoriteOp(id, true, "A"); err != nil {
		t.Fatal(err)
	}
	old, _ := s.PendingFavoriteOps(10, "A")
	if err := s.DeletePendingFavoriteOps([]string{id}, "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.QueueFavoriteOp(id, true, "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.QueueFavoriteOp(id, false, "B"); err != nil {
		t.Fatal(err)
	}
	if err := s.AckFavoriteOps(old); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"A", "B"} {
		items, err := s.PendingFavoriteOps(10, owner)
		if err != nil || len(items) != 1 || items[0].Scope != owner || items[0].Added != (owner == "A") {
			t.Fatalf("owner %s intent changed: %v, %v", owner, items, err)
		}
	}
}

func TestHistoryDeleteAckKeepsADeleteQueuedAfterReplay(t *testing.T) {
	s := newQueueStore(t)
	item := HistoryItem{SeriesID: "710000000000001", UpdatedAt: 100, EpisodeNum: 1}
	if err := s.SaveHistory(item, "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteHistory(item.SeriesID, "A"); err != nil {
		t.Fatal(err)
	}
	old, _ := s.PendingHistoryDeletes(10, "A")
	if err := s.SaveHistory(item, "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteHistory(item.SeriesID, "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.AckHistoryDeletes(old); err != nil {
		t.Fatal(err)
	}
	items, err := s.PendingHistoryDeletes(10, "A")
	if err != nil || len(items) != 1 || items[0].Revision <= old[0].Revision {
		t.Fatalf("new delete lost: %v, %v", items, err)
	}
}

func TestQueueMigrationKeepsLegacyDataUnownedAndRevisionSurvivesRestart(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToggleFavorite(FavoriteItem{SeriesID: "710000000000001", Title: "Local"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "userdata.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, queue := range syncQueues {
		fields := strings.Replace(queue.fields, "series_id TEXT NOT NULL", "series_id TEXT PRIMARY KEY", 1)
		if _, err := db.Exec("DROP TABLE " + queue.name + "; CREATE TABLE " + queue.name + " (" + fields + ")"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO pending_favorite_ops (series_id, added, updated_at) VALUES ('710000000000001', 1, 100)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	unowned, err := s.PendingFavoriteOps(10, "")
	if err != nil || len(unowned) != 1 {
		t.Fatalf("legacy intent lost: %v, %v", unowned, err)
	}
	other, err := s.PendingFavoriteOps(10, "B")
	if err != nil || len(other) != 0 || !s.IsFavorite("710000000000001") {
		t.Fatal("migration assigned old data to a new identity or removed local favorite")
	}
	if err := s.QueueFavoriteOp("710000000000001", true, "A"); err != nil {
		t.Fatal(err)
	}
	old, _ := s.PendingFavoriteOps(10, "A")
	if err := s.AckFavoriteOps(old); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.QueueFavoriteOp("710000000000001", true, "A"); err != nil {
		t.Fatal(err)
	}
	if err := s.AckFavoriteOps(old); err != nil {
		t.Fatal(err)
	}
	newer, err := s.PendingFavoriteOps(10, "A")
	if err != nil || len(newer) != 1 || newer[0].Revision <= old[0].Revision {
		t.Fatalf("restart reused acknowledged revision: %v, %v", newer, err)
	}
}
