package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONMigrationRemovesSourceAndDoesNotReplayOnRestart(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	const id, episode = "710000000000001", "710000000000101"
	data, err := json.Marshal(UserData{Likes: map[string]LikeItem{episode: {SeriesID: id, EpisodeID: episode, EpisodeNum: 1, Title: "Migrated"}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "userdata.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for startup := 0; startup < 3; startup++ {
		store, err := NewStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := store.IsLiked(id, episode, 1), startup < 2; got != want {
			store.Close()
			t.Fatalf("startup %d: liked=%v, want %v", startup, got, want)
		}
		if needed, err := store.jsonMigrationNeeded(); err != nil || needed {
			store.Close()
			t.Fatalf("migration marker was not durable: needed=%v err=%v", needed, err)
		}
		if startup == 1 {
			if err := store.SetLike(LikeItem{SeriesID: id, EpisodeID: episode}, false); err != nil {
				store.Close()
				t.Fatal(err)
			}
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if startup == 0 {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("migrated JSON still exists: %v", err)
			}
		}
	}
}

func TestJSONMigrationRollsBackDataAndMarkerTogether(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`CREATE TRIGGER reject_migration BEFORE INSERT ON reservations BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	data := UserData{
		Profile:      UserProfile{UserID: "710000000000099", Nickname: "Legacy", LoggedIn: true},
		History:      map[string]HistoryItem{"history": {SeriesID: "710000000000001", Title: "History"}},
		Favorites:    map[string]FavoriteItem{"favorite": {SeriesID: "710000000000002", Title: "Favorite"}},
		Likes:        map[string]LikeItem{"like": {SeriesID: "710000000000003", EpisodeID: "710000000000103", Title: "Like"}},
		Reservations: map[string]ReservationItem{"reservation": {SeriesID: "710000000000004", Title: "Reservation"}},
	}
	if err := store.importJSONTransaction(data); err == nil {
		t.Fatal("injected migration failure was ignored")
	}
	for _, table := range []string{"history", "favorites", "likes", "reservations", "migration_markers"} {
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partially committed %s: count=%d err=%v", table, count, err)
		}
	}
	if store.GetProfile().LoggedIn {
		t.Fatal("failed migration committed the profile")
	}
	if _, err := store.db.Exec(`DROP TRIGGER reject_migration`); err != nil {
		t.Fatal(err)
	}
	if err := store.importJSONTransaction(data); err != nil {
		t.Fatal(err)
	}
	if needed, err := store.jsonMigrationNeeded(); err != nil || needed {
		t.Fatalf("successful retry missing marker: %v, %v", needed, err)
	}
}

func TestJSONMigrationPreservesNewerLocalRows(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const id = "710000000000001"
	if err := store.SaveHistory(HistoryItem{SeriesID: id, Title: "Current", PositionSec: 50, UpdatedAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := store.importJSONTransaction(UserData{History: map[string]HistoryItem{id: {SeriesID: id, Title: "Old", PositionSec: 10, UpdatedAt: 100}}}); err != nil {
		t.Fatal(err)
	}
	item, ok := store.GetHistoryItem(id)
	if !ok || item.Title != "Current" || item.PositionSec != 50 {
		t.Fatalf("legacy snapshot replaced current progress: %+v", item)
	}
}

func TestLegacyHistoryKeepsUnknownMetadataUnknown(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	legacyPath := filepath.Join(t.TempDir(), "old.sqlite")
	db, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE progress(series_id TEXT, episode_index INTEGER, position_sec REAL, updated_at INTEGER, series TEXT);
		INSERT INTO progress VALUES ('710000000000001', 0, 30, 100, '{"title":"Unknown duration"}');
		INSERT INTO progress VALUES ('710000000000002', 1, 15, 200, '{"title":"Known count","episode_count":12}')`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.importOldRecords(legacyPath)
	unknown, ok := store.GetHistoryItem("710000000000001")
	if !ok || unknown.TotalEpisodes != 0 || unknown.DurationSec != 0 || unknown.PositionSec != 30 {
		t.Fatalf("unknown metadata was fabricated: %+v", unknown)
	}
	known, ok := store.GetHistoryItem("710000000000002")
	if !ok || known.TotalEpisodes != 12 || known.DurationSec != 0 {
		t.Fatalf("authentic metadata was lost: %+v", known)
	}
}
