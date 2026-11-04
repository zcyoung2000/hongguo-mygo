package library

import (
	"context"
	"errors"
	"io"
	userstore "myguo/internal/store"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFavoriteSyncRequiresSuccessfulCompleteShelf(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"business error", `{"code":1001,"message":"login expired"}`, 2},
		{"unknown response", `{}`, 2},
		{"missing code", `{"data":{"video_shelf_info":[]}}`, 2},
		{"missing data", `{"code":0}`, 2},
		{"missing list", `{"code":0,"data":{}}`, 2},
		{"null list", `{"code":0,"data":{"video_shelf_info":null}}`, 2},
		{"invalid ID", `{"code":0,"data":{"video_shelf_info":[{"series_id":""}]}}`, 2},
		{"partial shelf", `{"code":0,"data":{"has_more":true,"video_shelf_info":[{"series_id":"710000000000001"}]}}`, 2},
		{"valid empty", `{"code":0,"data":{"video_shelf_info":[]}}`, 0},
		{"valid complete", `{"code":0,"data":{"video_shelf_info":[{"series_id":"710000000000001"}]}}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.body) }))
			for _, id := range []string{"710000000000001", "710000000000002"} {
				if _, err := store.ToggleFavorite(FavoriteItem{SeriesID: id, Title: "Local", CoverURL: "https://media.invalid/cover"}); err != nil {
					t.Fatal(err)
				}
			}
			service.syncRemoteFavorites(context.Background(), service.currentFavoriteGeneration())
			items, err := store.GetFavorites()
			if err != nil || len(items) != tc.want {
				t.Fatalf("local favorites = %v, %v; want %d", items, err, tc.want)
			}
		})
	}
}

func TestOlderUploadAckKeepsNewEpisodeProgress(t *testing.T) {
	const id, firstEpisode, nextEpisode = "710000000000001", "710000000000101", "710000000000102"
	var store *userstore.Store
	var once sync.Once
	var positions []float64
	service, db := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "read_history/update") {
			raw, _ := io.ReadAll(r.Body)
			row := decodeGZIP(t, raw)["update_datas"].([]any)[0].(map[string]any)
			positions = append(positions, row["current_play_position"].(float64))
			once.Do(func() {
				newer := HistoryItem{SeriesID: id, Title: "Local", EpisodeNum: 2, PositionSec: 20, DurationSec: 100, UpdatedAt: 100}
				if err := store.SaveHistory(newer); err != nil {
					t.Error(err)
				}
				if err := store.QueueHistoryUpload(newer, nextEpisode); err != nil {
					t.Error(err)
				}
			})
		}
		writeJSON(w, map[string]any{"code": 0})
	}))
	store = db
	item := HistoryItem{SeriesID: id, Title: "Local", EpisodeNum: 1, PositionSec: 10, DurationSec: 100, UpdatedAt: 100}
	if err := store.SaveHistory(item); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueHistoryUpload(item, firstEpisode); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryUploads(); err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingHistoryUploads(10)
	if err != nil || len(pending) != 1 || pending[0].EpisodeID != nextEpisode || pending[0].PositionSec != 20 {
		t.Fatalf("new intent lost: %v, %v", pending, err)
	}
	if _, err := service.processPendingHistoryUploads(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(positions, []float64{10000, 20000}) {
		t.Fatalf("uploaded positions = %v", positions)
	}
	pending, err = store.PendingHistoryUploads(10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("confirmed queue = %v, %v", pending, err)
	}
}

func TestPendingOperationsDoNotCrossCredentialChanges(t *testing.T) {
	var sentCookies []string
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentCookies = append(sentCookies, r.Header.Get("Cookie"))
		writeJSON(w, map[string]any{"code": 0})
	}))
	original := service.credentials
	originalScope := service.currentSyncSession().scope
	if err := store.QueueFavoriteOp("710000000000001", true); err != nil {
		t.Fatal(err)
	}
	item := HistoryItem{SeriesID: "710000000000002", EpisodeNum: 1, PositionSec: 10, DurationSec: 100}
	if err := store.SaveHistory(item); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueHistoryUpload(item, "710000000000102"); err != nil {
		t.Fatal(err)
	}
	deleted := HistoryItem{SeriesID: "710000000000003", EpisodeNum: 1}
	if err := store.SaveHistory(deleted); err != nil {
		t.Fatal(err)
	}
	if err := store.RememberEpisode(deleted.SeriesID, "710000000000103", 1, "720p"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteHistory(deleted.SeriesID); err != nil {
		t.Fatal(err)
	}
	service.credentials = func() (string, string) { return "sessionid=account-B", "B" }
	service.InvalidateUserData()
	if _, err := service.syncPendingFavoriteOps(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryUploads(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryDeletes(); err != nil {
		t.Fatal(err)
	}
	if len(sentCookies) != 0 {
		t.Fatalf("old intents used new credentials: %v", sentCookies)
	}
	if pending, err := store.PendingFavoriteOps(10, originalScope); err != nil || len(pending) != 1 {
		t.Fatal("old identity's queue was lost")
	}
	service.credentials = original
	service.InvalidateUserData()
	if _, err := service.syncPendingFavoriteOps(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryUploads(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryDeletes(); err != nil {
		t.Fatal(err)
	}
	if len(sentCookies) != 4 {
		t.Fatalf("original identity did not resume its intents: %v", sentCookies)
	}
	for _, cookie := range sentCookies {
		if cookie != "sessionid=account" {
			t.Fatalf("unexpected sender: %s", cookie)
		}
	}
}

func TestIdentityChangeBetweenHistoryAndProgressUploadStopsOldTransaction(t *testing.T) {
	var service *Service
	var cookies []string
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies = append(cookies, r.Header.Get("Cookie"))
		service.credentials = func() (string, string) { return "sessionid=account-B", "B" }
		service.InvalidateUserData()
		writeJSON(w, map[string]any{"code": 0})
	}))
	scope := service.currentSyncSession().scope
	item := HistoryItem{SeriesID: "710000000000001", EpisodeNum: 1, PositionSec: 10, DurationSec: 100}
	if err := store.QueueHistoryUpload(item, "710000000000101"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryUploads(); !errors.Is(err, errSyncSessionChanged) {
		t.Fatalf("identity change = %v", err)
	}
	if len(cookies) != 1 || cookies[0] != "sessionid=account" {
		t.Fatalf("mixed credentials within an upload: %v", cookies)
	}
	pending, err := store.PendingHistoryUploads(10, scope)
	if err != nil || len(pending) != 1 {
		t.Fatalf("interrupted intent was acknowledged: %v, %v", pending, err)
	}
}

func TestCredentialRefreshKeepsValidatedUserQueue(t *testing.T) {
	var gotCookie string
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		writeJSON(w, map[string]any{"code": 0})
	}))
	cookie := "sessionid=old"
	service.authSession = func() (string, string, string) { return "710000000000099", cookie, "" }
	service.InvalidateUserData()
	if err := store.QueueFavoriteOp("710000000000001", true); err != nil {
		t.Fatal(err)
	}
	cookie = "sessionid=renewed"
	service.InvalidateUserData()
	if processed, err := service.syncPendingFavoriteOps(); err != nil || processed != 1 {
		t.Fatalf("refresh lost pending intent: %d, %v", processed, err)
	}
	if gotCookie != cookie {
		t.Fatalf("renewed credentials not used: %s", gotCookie)
	}
}

func TestLocalFavoriteToggleDoesNotWaitForNetworkAndLatestIntentIsSent(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	var once sync.Once
	var cancelled []bool
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		row := decodeGZIP(t, raw)["update_bookshelf_video_list"].([]any)[0].(map[string]any)
		cancelled = append(cancelled, row["is_cancelled"].(bool))
		once.Do(func() { close(started); <-release })
		writeJSON(w, map[string]any{"code": 0})
	}))
	const id = "710000000000001"
	if _, err := store.ToggleFavorite(FavoriteItem{SeriesID: id, Title: "Local"}); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueFavoriteOp(id, true); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := service.syncPendingFavoriteOps(); done <- err }()
	<-started
	toggled := make(chan error, 1)
	go func() { _, err := service.ToggleFavorite(context.Background(), id, "Local", "", "", 1); toggled <- err }()
	select {
	case err := <-toggled:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		unblock.Do(func() { close(release) })
		<-done
		<-toggled
		t.Fatal("local toggle blocked behind a network request")
	}
	unblock.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	service.WaitForBackgroundSync()
	if !reflect.DeepEqual(cancelled, []bool{false, true}) {
		t.Fatalf("latest favorite intent was not sent: %v", cancelled)
	}
	if store.IsFavorite(id) {
		t.Fatal("cancelled favorite returned locally")
	}
}

func TestMutationWithoutBusinessStatusDoesNotAcknowledgeQueue(t *testing.T) {
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"message": "unknown response"})
	}))
	if err := store.QueueFavoriteOp("710000000000001", true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.syncPendingFavoriteOps(); err == nil {
		t.Fatal("unknown response was accepted")
	}
	pending, err := store.PendingFavoriteOps(10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("failed intent was deleted: %v, %v", pending, err)
	}
}

func TestPartialReservationSyncKeepsExistingLibrary(t *testing.T) {
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("is_online") == "true" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{}})
	}))
	if err := store.ReplaceReservationBatch([]ReservationItem{{SeriesID: "710000000000001", Title: "Local"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.syncRemoteReservations(context.Background(), service.currentReservationGeneration()); err == nil {
		t.Fatal("partial account response was accepted")
	}
	items, err := store.GetReservations()
	if err != nil || len(items) != 1 {
		t.Fatalf("partial response removed reservations: %v, %v", items, err)
	}
}
