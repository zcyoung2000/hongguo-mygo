package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteMigrationAndStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hg_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mockJSON := `{
  "profile": {
    "userId": "12345",
    "nickname": "测试用户",
    "avatar": "http://example.com/avatar.jpg",
    "loggedIn": true
  },
  "history": {
    "drama_1": {
      "seriesId": "drama_1",
      "title": "测试短剧1",
      "coverUrl": "http://example.com/c1.jpg",
      "episodeNum": 3,
      "episodeTitle": "第3集",
      "positionSec": 45.5,
      "durationSec": 120.0,
      "totalEpisodes": 80,
      "updatedAt": 1700000000,
      "contentType": 1004
    },
    "short_series_1": {
      "seriesId": "short_series_1",
      "title": "测试真人短剧",
      "coverUrl": "http://example.com/n1.jpg",
      "episodeNum": 1,
      "contentType": 1
    }
  },
  "favorites": {
    "drama_1": {
      "seriesId": "drama_1",
      "title": "测试短剧1",
      "addedAt": 1700000000
    }
  },
  "likes": {
    "drama_1_ep3": {
      "seriesId": "drama_1",
      "episodeId": "ep3",
      "episodeNum": 3,
      "addedAt": 1700000000
    }
  },
  "reservations": {
    "res_1": {
      "seriesId": "res_1",
      "title": "预约短剧",
      "addedAt": 1700000000
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "userdata.json"), []byte(mockJSON), 0644); err != nil {
		t.Fatalf("failed to write mock json: %v", err)
	}

	// 1. Initialize store -> triggers migration
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	// 2. Check profile
	prof := store.GetProfile()
	if prof.Nickname != "测试用户" || !prof.LoggedIn {
		t.Errorf("Profile mismatch: %+v", prof)
	}

	// 3. Both motion comics and SDK content_type=1 short series survive migration.
	hist, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(hist) != 2 {
		t.Errorf("History mismatch: len=%d, %+v", len(hist), hist)
	}

	// 4. Check favorites
	if !store.IsFavorite("drama_1") {
		t.Errorf("Favorite drama_1 not found")
	}
	favItem, favOk := store.GetFavoriteItem("drama_1")
	if !favOk || favItem.Title != "测试短剧1" {
		t.Errorf("GetFavoriteItem drama_1 mismatch: ok=%v, item=%+v", favOk, favItem)
	}
	favs, err := store.GetFavorites()
	if err != nil {
		t.Fatalf("GetFavorites failed: %v", err)
	}
	if len(favs) != 1 {
		t.Errorf("Favorites len mismatch: %d", len(favs))
	}

	// 5. Check likes
	if !store.IsLiked("drama_1", "ep3", 3) {
		t.Errorf("Like drama_1 ep3 not found")
	}
	likes, err := store.GetLikes()
	if err != nil {
		t.Fatalf("GetLikes failed: %v", err)
	}
	if len(likes) != 1 {
		t.Errorf("Likes len mismatch: %d", len(likes))
	}

	// 6. Check reservations
	res, err := store.GetReservations()
	if err != nil {
		t.Fatalf("GetReservations failed: %v", err)
	}
	if len(res) != 1 || res[0].SeriesID != "res_1" {
		t.Errorf("Reservations mismatch: %+v", res)
	}

	// 7. Check legacy JSON was removed after durable migration
	if _, err := os.Stat(filepath.Join(tmpDir, "userdata.json")); !os.IsNotExist(err) {
		t.Errorf("userdata.json still exists, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "userdata.json.bak")); !os.IsNotExist(err) {
		t.Errorf("userdata.json.bak still exists, err = %v", err)
	}

	// 8. Test runtime updates (point progress update)
	err = store.SaveHistory(HistoryItem{
		SeriesID:    "drama_1",
		Title:       "测试短剧1",
		EpisodeNum:  4,
		PositionSec: 99.0,
		UpdatedAt:   1700000010,
	})
	if err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}
	item, ok := store.GetHistoryItem("drama_1")
	if !ok || item.EpisodeNum != 4 || item.PositionSec != 99.0 {
		t.Errorf("Updated item mismatch: %+v", item)
	}

	// 9. Test ToggleFavorite
	isFav, err := store.ToggleFavorite(FavoriteItem{SeriesID: "drama_1"})
	if err != nil || isFav {
		t.Errorf("Toggle off favorite failed: isFav=%v, err=%v", isFav, err)
	}
	if store.IsFavorite("drama_1") {
		t.Errorf("Favorite still exists after toggle off")
	}
}

func TestFeedProgressAndSeenGeneration(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	progress, _ := store.GetFeedProgress("推荐")
	if progress.Generation != 1 || progress.Cursor != "" {
		t.Fatalf("initial progress mismatch: %+v", progress)
	}

	if err := store.SaveFeedProgress("推荐", "cursor-1", 1); err != nil {
		t.Fatalf("SaveFeedProgress failed: %v", err)
	}
	if err := store.SaveFeedSeenIDs("推荐", 1, []string{"a", "b", "a"}); err != nil {
		t.Fatalf("SaveFeedSeenIDs failed: %v", err)
	}

	progress, _ = store.GetFeedProgress("推荐")
	seen := store.GetFeedSeenIDs("推荐", 1)
	if progress.Cursor != "cursor-1" || progress.Generation != 1 || len(seen) != 2 {
		t.Fatalf("saved feed progress mismatch: progress=%+v seen=%v", progress, seen)
	}

	next, err := store.StartNextFeedGeneration("推荐")
	if err != nil || next != 2 {
		t.Fatalf("StartNextFeedGeneration mismatch: generation=%d err=%v", next, err)
	}
	progress, _ = store.GetFeedProgress("推荐")
	if progress.Generation != 2 || progress.Cursor != "" {
		t.Fatalf("rotated progress mismatch: %+v", progress)
	}
	if len(store.GetFeedSeenIDs("推荐", 2)) != 0 {
		t.Fatal("new generation should start empty")
	}
}

func TestRecentFeedSeenIDsSuppressesImmediateRepeatsAcrossGenerations(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.SaveFeedSeenIDs("test", 1, []string{"a", "b"}); err != nil {
		t.Fatalf("SaveFeedSeenIDs generation 1 failed: %v", err)
	}
	seen := store.GetRecentFeedSeenIDs("test", 2)
	if !seen["a"] || !seen["b"] {
		t.Fatalf("generation 2 should inherit generation 1: %v", seen)
	}

	if err := store.SaveFeedSeenIDs("test", 2, []string{"c"}); err != nil {
		t.Fatalf("SaveFeedSeenIDs generation 2 failed: %v", err)
	}
	seen = store.GetRecentFeedSeenIDs("test", 3)
	if !seen["a"] || !seen["b"] || !seen["c"] {
		t.Fatalf("generation 3 should inherit the previous two generations: %v", seen)
	}

	if err := store.SaveFeedSeenIDs("test", 3, []string{"d"}); err != nil {
		t.Fatalf("SaveFeedSeenIDs generation 3 failed: %v", err)
	}
	seen = store.GetRecentFeedSeenIDs("test", 1)
	if !seen["a"] || !seen["b"] || !seen["c"] || !seen["d"] {
		t.Fatalf("generation 1 should inherit all three retained generations: %v", seen)
	}

	if err := store.SaveFeedProgress("test", "", 3); err != nil {
		t.Fatalf("SaveFeedProgress failed: %v", err)
	}
	if _, err := store.StartNextFeedGeneration("test"); err != nil {
		t.Fatalf("StartNextFeedGeneration failed: %v", err)
	}
	seen = store.GetRecentFeedSeenIDs("test", 4)
	progress, progressOK := store.GetFeedProgress("test")
	if !progressOK || progress.Generation != 4 {
		t.Fatalf("next progress mismatch: ok=%v progress=%+v", progressOK, progress)
	}
	if !seen["c"] || !seen["d"] || seen["a"] || seen["b"] {
		t.Fatalf("generation rotation should drop only the oldest generation: %v", seen)
	}
}

func TestCachedEpisodeIndex(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.RememberEpisode("series-1", "series-1-ep-3", 3, "1080p"); err != nil {
		t.Fatalf("RememberEpisode failed: %v", err)
	}
	if err := store.RememberEpisode("series-2", "series-2-ep-1", 1, "720p"); err != nil {
		t.Fatalf("RememberEpisode failed: %v", err)
	}

	item, ok := store.GetCachedEpisodeAt("series-1", "720p", 1)
	if ok {
		t.Fatalf("episode 1 should not exist: %+v", item)
	}
	item, ok = store.GetCachedEpisodeAt("series-1", "1080p", 3)
	if !ok || item.EpisodeID != "series-1-ep-3" || item.Quality != "1080p" {
		t.Fatalf("cached episode lookup mismatch: ok=%v item=%+v", ok, item)
	}
	item, ok = store.GetCachedEpisodeAt("series-2", "1080p", 1)
	if !ok || item.Quality != "720p" {
		t.Fatalf("quality fallback mismatch: ok=%v item=%+v", ok, item)
	}
}

func TestSQLiteMigratesFavoritesOnlyUser(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hg_store_fav_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "userdata.json")
	json := `{"favorites":{"fav_1":{"seriesId":"fav_1","title":"Favorites only","addedAt":1700000000}}}`
	if err := os.WriteFile(jsonPath, []byte(json), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	items, err := store.GetFavorites()
	if err != nil {
		t.Fatalf("GetFavorites failed: %v", err)
	}
	if len(items) != 1 || items[0].SeriesID != "fav_1" {
		t.Fatalf("favorites = %+v, want one fav_1", items)
	}
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Fatalf("legacy JSON still exists: %v", err)
	}
}

func TestToggleLikeUsesEpisodeID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hg_store_like_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	first := LikeItem{SeriesID: "710000000000001", EpisodeID: "710000000000101", EpisodeNum: 1}
	if added, err := store.ToggleLike(first); err != nil || !added {
		t.Fatalf("first ToggleLike = %v, %v", added, err)
	}
	second := LikeItem{SeriesID: "710000000000001", EpisodeID: "710000000000102", EpisodeNum: 1}
	added, err := store.ToggleLike(second)
	if err != nil || !added {
		t.Fatalf("second ToggleLike = %v, %v", added, err)
	}
	if !store.IsLiked("710000000000001", "710000000000101", 1) ||
		!store.IsLiked("710000000000001", "710000000000102", 1) {
		t.Fatal("same-number episode like was deleted by collision")
	}
	likes, err := store.GetLikes()
	if err != nil {
		t.Fatalf("GetLikes failed: %v", err)
	}
	if len(likes) != 2 {
		t.Fatalf("likes = %d, want 2", len(likes))
	}
}

func TestStoreRemovesShortFavoriteIDs(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if _, err := store.ToggleFavorite(FavoriteItem{SeriesID: "999", Title: "invalid test row"}); err != nil {
		t.Fatalf("ToggleFavorite failed: %v", err)
	}
	if !store.IsFavorite("999") {
		t.Fatal("precondition failed: invalid favorite was not inserted")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	store, err = NewStore(tmpDir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer store.Close()
	if store.IsFavorite("999") {
		t.Fatal("stale invalid favorite survived startup cleanup")
	}
}

func TestHistoryTombstoneSurvivesRemoteBatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hg_store_history_tomb_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.ClearHistory(); err != nil {
		t.Fatalf("ClearHistory failed: %v", err)
	}
	watchTime := time.Now().Add(10 * time.Second).Unix()
	if err := store.SaveHistory(HistoryItem{SeriesID: "710000000000001", Title: "Before delete"}); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}
	if err := store.DeleteHistory("710000000000001"); err != nil {
		t.Fatalf("DeleteHistory failed: %v", err)
	}
	err = store.SaveHistoryBatch([]HistoryItem{{SeriesID: "710000000000001", Title: "Remote restore", UpdatedAt: watchTime}})
	if err != nil {
		t.Fatalf("SaveHistoryBatch failed: %v", err)
	}
	items, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(items) != 0 || store.HistoryDeleted("710000000000001") == false {
		t.Fatalf("tombstoned item was restored: items=%v deleted=%v", items, store.HistoryDeleted("710000000000001"))
	}
	if err := store.SaveHistory(HistoryItem{SeriesID: "710000000000001", Title: "Watch again", UpdatedAt: watchTime + 1}); err != nil {
		t.Fatalf("SaveHistory after delete failed: %v", err)
	}
	if store.HistoryDeleted("710000000000001") {
		t.Fatal("watching again should clear the tombstone")
	}
}

func TestClearHistoryFiltersOldRemoteItems(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hg_store_history_clear_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()
	if err := store.ClearHistory(); err != nil {
		t.Fatalf("ClearHistory failed: %v", err)
	}
	err = store.SaveHistoryBatch([]HistoryItem{{SeriesID: "710000000000002", Title: "Old remote", UpdatedAt: time.Now().Add(-time.Minute).Unix()}})
	if err != nil {
		t.Fatalf("SaveHistoryBatch failed: %v", err)
	}
	items, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("cleared old item returned: %+v", items)
	}
}

func TestClearUserDataKeepsScopedIntents(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()
	if err := store.SaveHistory(HistoryItem{SeriesID: "710000000000001", EpisodeNum: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ToggleFavorite(FavoriteItem{SeriesID: "710000000000001", Title: "favorite"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ToggleLike(LikeItem{SeriesID: "710000000000001", EpisodeID: "710000000000101"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ToggleReservation(ReservationItem{SeriesID: "710000000000001", Title: "reservation"}); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueHistoryUpload(HistoryItem{SeriesID: "710000000000002", EpisodeNum: 1}, "710000000000102", "user:one"); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearUserData(); err != nil {
		t.Fatalf("ClearUserData failed: %v", err)
	}
	if items, err := store.GetHistory(); err != nil || len(items) != 0 {
		t.Fatalf("history survived ClearUserData: %v, %v", items, err)
	}
	if favorites, err := store.GetFavorites(); err != nil || len(favorites) != 0 {
		t.Fatalf("favorites survived ClearUserData: %v, %v", favorites, err)
	}
	if likes, err := store.GetLikes(); err != nil || len(likes) != 0 {
		t.Fatalf("likes survived ClearUserData: %v, %v", likes, err)
	}
	if reservations, err := store.GetReservations(); err != nil || len(reservations) != 0 {
		t.Fatalf("reservations survived ClearUserData: %v, %v", reservations, err)
	}
	if pending, err := store.PendingHistoryUploads(10, "user:one"); err != nil || len(pending) != 1 {
		t.Fatalf("scoped intent lost: %v, %v", pending, err)
	}
}

func TestClearCachedEpisodesRemovesIndex(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()
	if err := store.RememberEpisode("710000000000001", "710000000000101", 1, "720p"); err != nil {
		t.Fatalf("RememberEpisode failed: %v", err)
	}
	if _, ok := store.GetCachedEpisode("710000000000001", "720p"); !ok {
		t.Fatal("cached episode was not indexed")
	}
	if err := store.ClearCachedEpisodes(); err != nil {
		t.Fatalf("ClearCachedEpisodes failed: %v", err)
	}
	if _, ok := store.GetCachedEpisode("710000000000001", "720p"); ok {
		t.Fatal("cleared episode index still returned a row")
	}
}
