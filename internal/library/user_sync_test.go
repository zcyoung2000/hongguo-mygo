package library

import (
	"encoding/json"
	"testing"
	"time"

	userstore "myguo/internal/store"
)

func TestProgressUploadPayloadUsesProtocolUnits(t *testing.T) {
	item := HistoryItem{
		SeriesID:      "710000000000001",
		EpisodeNum:    3,
		PositionSec:   12.5,
		DurationSec:   100,
		TotalEpisodes: 80,
	}
	payload := progressUploadPayload(item, "710000000000101")
	books, ok := payload["books"].([]any)
	if !ok || len(books) != 1 {
		t.Fatalf("expected one book, got %#v", payload["books"])
	}
	book, ok := books[0].(map[string]any)
	if !ok {
		t.Fatalf("expected book object, got %T", books[0])
	}
	if got := book["item_id"]; got != "710000000000101" {
		t.Fatalf("item_id = %#v", got)
	}
	if got := book["vid_index"]; got != 2 {
		t.Fatalf("vid_index = %#v, want 2", got)
	}
	if got := book["current_play_time"]; got != int64(12500) {
		t.Fatalf("current_play_time = %#v, want 12500", got)
	}
	if got := book["player_cumulative_total_duration"]; got != int64(12500) {
		t.Fatalf("cumulative duration = %#v, want 12500", got)
	}
	if got := book["progress_type"]; got != int64(12500) {
		t.Fatalf("progress type = %#v, want 12500", got)
	}
	if got := book["book_id"]; got != "710000000000001" {
		t.Fatalf("book_id = %#v, want string ID", got)
	}
}

func TestFavoriteMutationPayloadMarksShelfItem(t *testing.T) {
	for _, tc := range []struct {
		added bool
		want  bool
	}{{added: true, want: false}, {added: false, want: true}} {
		payload := favoriteMutationPayload("710000000000001", tc.added)
		items, ok := payload["update_bookshelf_video_list"].([]any)
		if !ok || len(items) != 1 {
			t.Fatalf("expected one shelf update, got %#v", payload["update_bookshelf_video_list"])
		}
		item, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("expected shelf object, got %T", items[0])
		}
		if got := item["is_cancelled"]; got != tc.want {
			t.Fatalf("is_cancelled = %#v, want %#v", got, tc.want)
		}
		if got := payload["book_type"]; got != favoriteBookType {
			t.Fatalf("book_type = %#v, want %d", got, favoriteBookType)
		}
		if _, err := json.Marshal(payload); err != nil {
			t.Fatalf("marshal favorite payload: %v", err)
		}
	}
}

func TestHistoryDeletePayloadMarksSoftDelete(t *testing.T) {
	item := HistoryItem{SeriesID: "710000000000001", EpisodeNum: 4, TotalEpisodes: 10}
	payload := historyDeletePayload([]HistoryItem{item}, []string{"710000000000104"})
	updates := payload["update_datas"].([]any)
	if len(updates) != 1 {
		t.Fatalf("updates = %#v, want one", updates)
	}
	update := updates[0].(map[string]any)
	if update["is_delete"] != true || update["use_soft_delete"] != true {
		t.Fatalf("delete flags = %#v", update)
	}
	if update["vid"] != int64(710000000000104) || update["vid_index"] != 3 {
		t.Fatalf("history ref = %#v", update)
	}
}

func TestReplaceFavoriteBatchReplacesCloudSet(t *testing.T) {
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("userstore.NewStore() error = %v", err)
	}
	defer store.Close()
	old := []FavoriteItem{{SeriesID: "710000000000001", Title: "old"}, {SeriesID: "710000000000002", Title: "remove"}}
	if err := store.ReplaceFavoriteBatch(old); err != nil {
		t.Fatalf("seed favorites: %v", err)
	}
	fresh := []FavoriteItem{{SeriesID: "710000000000001", Title: "fresh"}, {SeriesID: "710000000000003", Title: "new"}}
	if err := store.ReplaceFavoriteBatch(fresh); err != nil {
		t.Fatalf("ReplaceFavoriteBatch() error = %v", err)
	}
	items, err := store.GetFavorites()
	if err != nil {
		t.Fatalf("GetFavorites() error = %v", err)
	}
	byID := make(map[string]string, len(items))
	for _, item := range items {
		byID[item.SeriesID] = item.Title
	}
	if len(items) != 2 || byID["710000000000001"] != "fresh" || byID["710000000000003"] != "new" {
		t.Fatalf("favorites = %#v, want remote replacement", items)
	}
}

func TestHistoryPendingOperationsPreserveLatestIntent(t *testing.T) {
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("userstore.NewStore() error = %v", err)
	}
	defer store.Close()
	item := HistoryItem{
		SeriesID:      "710000000000001",
		Title:         "watched",
		EpisodeNum:    3,
		PositionSec:   12.5,
		DurationSec:   100,
		TotalEpisodes: 10,
		UpdatedAt:     time.Now().Unix(),
	}
	if err := store.SaveHistory(item); err != nil {
		t.Fatalf("SaveHistory() error = %v", err)
	}
	if err := store.RememberEpisode(item.SeriesID, "710000000000101", item.EpisodeNum, "720p"); err != nil {
		t.Fatalf("RememberEpisode() error = %v", err)
	}
	if err := store.QueueHistoryUpload(item, "710000000000101"); err != nil {
		t.Fatalf("QueueHistoryUpload() error = %v", err)
	}
	uploads, err := store.PendingHistoryUploads(1)
	if err != nil || len(uploads) != 1 || uploads[0].EpisodeID != "710000000000101" {
		t.Fatalf("uploads = %#v, err = %v", uploads, err)
	}
	if err := store.DeleteHistory(item.SeriesID); err != nil {
		t.Fatalf("DeleteHistory() error = %v", err)
	}
	deletes, err := store.PendingHistoryDeletes(1)
	if err != nil || len(deletes) != 1 || deletes[0].EpisodeID != "710000000000101" || deletes[0].EpisodeNum != 3 {
		t.Fatalf("deletes = %#v, err = %v", deletes, err)
	}
	// A later save is the newest intent and must cancel the queued deletion.
	item.UpdatedAt++
	if err := store.SaveHistory(item); err != nil {
		t.Fatalf("re-save history: %v", err)
	}
	deletes, err = store.PendingHistoryDeletes(1)
	if err != nil || len(deletes) != 0 {
		t.Fatalf("deletes after re-save = %#v, err = %v", deletes, err)
	}
}

func TestFavoritePendingOperationsCoalesceLatestIntent(t *testing.T) {
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("userstore.NewStore() error = %v", err)
	}
	defer store.Close()
	if err := store.QueueFavoriteOp("710000000000001", true); err != nil {
		t.Fatalf("QueueFavoriteOp() error = %v", err)
	}
	if err := store.QueueFavoriteOp("710000000000001", false); err != nil {
		t.Fatalf("QueueFavoriteOp() error = %v", err)
	}
	ops, err := store.PendingFavoriteOps(10)
	if err != nil || len(ops) != 1 || ops[0].Added {
		t.Fatalf("favorite ops = %#v, err = %v", ops, err)
	}
	if err := store.DeletePendingFavoriteOps([]string{ops[0].SeriesID}); err != nil {
		t.Fatalf("DeletePendingFavoriteOps() error = %v", err)
	}
}
