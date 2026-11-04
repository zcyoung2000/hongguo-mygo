package library

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	userstore "myguo/internal/store"
)

type capturedRequest struct {
	path string
	body []byte
}

type handlerTransport struct{ handler http.Handler }

func (transport handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

type requestLog struct {
	mu      sync.Mutex
	request []capturedRequest
}

func (log *requestLog) add(request capturedRequest) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.request = append(log.request, request)
}

func (log *requestLog) matching(suffix string) []capturedRequest {
	log.mu.Lock()
	defer log.mu.Unlock()
	var matching []capturedRequest
	for _, request := range log.request {
		if strings.Contains(request.path, suffix) {
			matching = append(matching, request)
		}
	}
	return matching
}

func newSyncTestService(t *testing.T, handler http.Handler) (*Service, *userstore.Store) {
	t.Helper()
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("userstore.NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := NewService(Dependencies{
		Store:       store,
		Credentials: func() (string, string) { return "sessionid=account", "token" },
		DeviceIdentity: func() (string, string, string, string) {
			return "7100000000000001", "7100000000000002", "test-cdid", "test-openudid"
		},
		Runtime:    func() (string, string) { return "test-agent", "cn-gd" },
		HTTPClient: &http.Client{Transport: handlerTransport{handler: handler}},
	})
	t.Cleanup(service.WaitForBackgroundSync)
	return service, store
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func decodeGZIP(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer reader.Close()
	var payload map[string]any
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		t.Fatalf("decode sync payload error = %v", err)
	}
	return payload
}

func TestRemoteHistorySyncUpdatesNewerLocalRow(t *testing.T) {
	const seriesID = "710000000000001"
	updatedAt := time.Now().Add(-time.Hour).Unix()
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(capturedRequest{path: r.URL.Path})
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"data_list": []map[string]any{{
					"series_id":         seriesID,
					"vid_index":         2,
					"current_play_time": 22_000,
					"last_watch_time":   time.Now().UnixMilli(),
					"episode_cnt":       10,
				}},
				"has_more": false,
			},
		})
	}))
	if err := store.SaveHistory(HistoryItem{
		SeriesID:      seriesID,
		Title:         "Account Sync",
		CoverURL:      "https://media.invalid/cover.jpg",
		EpisodeNum:    2,
		EpisodeTitle:  "第2集",
		PositionSec:   10,
		DurationSec:   100,
		TotalEpisodes: 10,
		UpdatedAt:     updatedAt,
	}); err != nil {
		t.Fatalf("SaveHistory() error = %v", err)
	}

	service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration())

	items, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("history len = %d, want 1", len(items))
	}
	item := items[0]
	if item.EpisodeNum != 3 || item.EpisodeTitle != "第3集" || item.PositionSec != 22 || item.Title != "Account Sync" {
		t.Fatalf("unexpected synced history: %+v", item)
	}
	if item.UpdatedAt <= updatedAt {
		t.Fatalf("updated_at = %d, want newer than %d", item.UpdatedAt, updatedAt)
	}
	if len(log.matching("read_progress/list")) != 1 {
		t.Fatalf("remote history calls = %d, want 1", len(log.matching("read_progress/list")))
	}
}

func TestCompleteRemoteHistorySnapshotDeletesRemoteRemovedRow(t *testing.T) {
	const keptID, deletedID = "710000000000001", "710000000000002"
	seed := []HistoryItem{
		{SeriesID: keptID, Title: "Kept", CoverURL: "https://media.invalid/kept.jpg", EpisodeNum: 1, TotalEpisodes: 10, UpdatedAt: 100},
		{SeriesID: deletedID, Title: "Deleted remotely", CoverURL: "https://media.invalid/deleted.jpg", EpisodeNum: 1, TotalEpisodes: 10, UpdatedAt: 100},
	}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"data_list": []map[string]any{{
					"series_id":         keptID,
					"vid_index":         1,
					"current_play_time": 10_000,
					"last_watch_time":   time.Now().UnixMilli(),
				}},
				"has_more": false,
			},
		})
	}))
	for _, item := range seed {
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
	}

	if err := service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration()); err != nil {
		t.Fatalf("syncRemoteHistory() error = %v", err)
	}

	items, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
	if len(items) != 1 || items[0].SeriesID != keptID {
		t.Fatalf("complete snapshot = %+v, want only %s", items, keptID)
	}
}

func TestRecentHistoryMutationBlocksAuthoritativeRemoteSnapshot(t *testing.T) {
	const keptID, recentID = "710000000000001", "710000000000002"
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"data_list": []map[string]any{{
					"series_id":         keptID,
					"vid_index":         1,
					"current_play_time": 10_000,
					"last_watch_time":   time.Now().UnixMilli(),
				}},
				"has_more": false,
			},
		})
	}))
	for _, item := range []HistoryItem{
		{SeriesID: keptID, Title: "Kept", CoverURL: "https://media.invalid/kept.jpg", EpisodeNum: 1, TotalEpisodes: 10, UpdatedAt: 100},
		{SeriesID: recentID, Title: "Recent upload", CoverURL: "https://media.invalid/recent.jpg", EpisodeNum: 1, TotalEpisodes: 10, UpdatedAt: time.Now().Unix()},
	} {
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
	}
	service.touchHistoryMutation()

	if err := service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration()); err != nil {
		t.Fatalf("first syncRemoteHistory() error = %v", err)
	}
	items, err := store.GetHistory()
	if err != nil || len(items) != 2 {
		t.Fatalf("recent upload removed by unsettled snapshot = %v, %v", items, err)
	}

	service.historyMu.Lock()
	service.historyMutationAt = time.Now().Add(-time.Minute)
	service.historyMu.Unlock()
	if err := service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration()); err != nil {
		t.Fatalf("settled syncRemoteHistory() error = %v", err)
	}
	items, err = store.GetHistory()
	if err != nil || len(items) != 1 || items[0].SeriesID != keptID {
		t.Fatalf("settled snapshot = %v, %v", items, err)
	}
}

func TestIncompleteRemoteHistorySnapshotKeepsLocalRows(t *testing.T) {
	const keptID, absentID = "710000000000001", "710000000000002"
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"data_list": []map[string]any{{
					"series_id":         keptID,
					"vid_index":         1,
					"current_play_time": 10_000,
					"last_watch_time":   time.Now().UnixMilli(),
				}},
				"has_more": true,
			},
		})
	}))
	for _, item := range []HistoryItem{
		{SeriesID: keptID, Title: "Kept", CoverURL: "https://media.invalid/kept.jpg", EpisodeNum: 1, TotalEpisodes: 10, UpdatedAt: 100},
		{SeriesID: absentID, Title: "Local", CoverURL: "https://media.invalid/local.jpg", EpisodeNum: 1, TotalEpisodes: 10, UpdatedAt: 100},
	} {
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
	}

	if err := service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration()); err != nil {
		t.Fatalf("syncRemoteHistory() error = %v", err)
	}

	items, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("incomplete snapshot = %+v, want both local rows", items)
	}
}

func TestPendingHistoryUploadFlushesHistoryAndProgress(t *testing.T) {
	const seriesID = "710000000000001"
	const episodeID = "710000000000101"
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.add(capturedRequest{path: r.URL.Path, body: body})
		writeJSON(w, map[string]any{"code": 0})
	}))
	item := HistoryItem{
		SeriesID:      seriesID,
		Title:         "Pending",
		CoverURL:      "https://media.invalid/cover.jpg",
		EpisodeNum:    3,
		EpisodeTitle:  "第3集",
		PositionSec:   12.5,
		DurationSec:   100,
		TotalEpisodes: 10,
		UpdatedAt:     time.Now().Unix(),
	}
	if err := store.SaveHistory(item); err != nil {
		t.Fatalf("SaveHistory() error = %v", err)
	}
	if err := store.QueueHistoryUpload(item, episodeID); err != nil {
		t.Fatalf("QueueHistoryUpload() error = %v", err)
	}

	_, _ = service.syncPendingHistoryMutations()

	pending, err := store.PendingHistoryUploads(10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending uploads = %#v, err = %v", pending, err)
	}
	historyCalls := log.matching("read_history/update")
	progressCalls := log.matching("read_progress/upload")
	if len(historyCalls) != 1 || len(progressCalls) != 1 {
		t.Fatalf("history/progress calls = %d/%d, want 1/1", len(historyCalls), len(progressCalls))
	}
	historyPayload := decodeGZIP(t, historyCalls[0].body)
	progressPayload := decodeGZIP(t, progressCalls[0].body)
	if _, ok := historyPayload["update_datas"]; !ok {
		t.Fatalf("history payload missing update_datas: %#v", historyPayload)
	}
	if _, ok := progressPayload["books"]; !ok {
		t.Fatalf("progress payload missing books: %#v", progressPayload)
	}
}

func TestHistoryMutationBatchesSplitAfterUpstreamLimit(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var count int
		if strings.Contains(r.URL.Path, "read_progress/upload") {
			payload := decodeGZIP(t, raw)
			count = len(payload["books"].([]any))
		} else {
			payload := decodeGZIP(t, raw)
			count = len(payload["update_datas"].([]any))
		}
		log.add(capturedRequest{path: r.URL.Path, body: []byte(strconv.Itoa(count))})
		if count > 1 {
			writeJSON(w, map[string]any{"code": 100103, "message": "PARAM_INVALID"})
			return
		}
		writeJSON(w, map[string]any{"code": 0})
	}))
	for i, episodeID := range []string{"710000000000101", "710000000000102"} {
		item := HistoryItem{
			SeriesID:   fmt.Sprintf("71000000000000%d", i+1),
			EpisodeNum: 1,
			UpdatedAt:  100 + int64(i),
		}
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
		if err := store.QueueHistoryUpload(item, episodeID); err != nil {
			t.Fatalf("QueueHistoryUpload() error = %v", err)
		}
	}

	uploaded, err := service.processPendingHistoryUploads()
	if err != nil {
		t.Fatalf("processPendingHistoryUploads() error = %v", err)
	}
	if uploaded != 2 {
		t.Fatalf("uploaded = %d, want 2", uploaded)
	}
	if pending, err := store.PendingHistoryUploads(10); err != nil || len(pending) != 0 {
		t.Fatalf("pending uploads = %v, %v", pending, err)
	}
	historyCalls := log.matching("read_history/update")
	if len(historyCalls) != 3 || string(historyCalls[0].body) != "2" ||
		string(historyCalls[1].body) != "1" || string(historyCalls[2].body) != "1" {
		t.Fatalf("history calls = %v; want batch 2 then 1, 1", historyCalls)
	}
	progressCalls := log.matching("read_progress/upload")
	if len(progressCalls) != 2 || string(progressCalls[0].body) != "1" || string(progressCalls[1].body) != "1" {
		t.Fatalf("progress calls = %v; want single-item calls 1, 1", progressCalls)
	}
}

func TestHistoryDeleteBatchesSplitAfterUpstreamLimit(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		payload := decodeGZIP(t, raw)
		count := len(payload["update_datas"].([]any))
		log.add(capturedRequest{path: r.URL.Path, body: []byte(strconv.Itoa(count))})
		if count > 1 {
			writeJSON(w, map[string]any{"code": 100103, "message": "PARAM_INVALID"})
			return
		}
		writeJSON(w, map[string]any{"code": 0})
	}))
	for i, episodeID := range []string{"710000000000101", "710000000000102"} {
		seriesID := fmt.Sprintf("71000000000000%d", i+1)
		item := HistoryItem{SeriesID: seriesID, EpisodeNum: 1, UpdatedAt: 100}
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
		if err := store.RememberEpisode(seriesID, episodeID, 1, "720p"); err != nil {
			t.Fatalf("RememberEpisode() error = %v", err)
		}
		if err := store.DeleteHistory(seriesID); err != nil {
			t.Fatalf("DeleteHistory() error = %v", err)
		}
	}

	deleted, err := service.processPendingHistoryDeletes()
	if err != nil {
		t.Fatalf("processPendingHistoryDeletes() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if pending, err := store.PendingHistoryDeletes(10); err != nil || len(pending) != 0 {
		t.Fatalf("pending deletes = %v, %v", pending, err)
	}
	calls := log.matching("read_history/update")
	if len(calls) != 3 {
		t.Fatalf("delete calls = %d, want a rejected batch followed by single-item calls", len(calls))
	}
	if string(calls[0].body) != "2" || string(calls[1].body) != "1" || string(calls[2].body) != "1" {
		t.Fatalf("delete batch sizes = %s, %s, %s; want 2, 1, 1", calls[0].body, calls[1].body, calls[2].body)
	}
}

func TestHistoryMutationKeepsBatchWhenUpstreamAcceptsIt(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		payload := decodeGZIP(t, raw)
		var count int
		if strings.Contains(r.URL.Path, "read_progress/upload") {
			count = len(payload["books"].([]any))
		} else {
			count = len(payload["update_datas"].([]any))
		}
		log.add(capturedRequest{path: r.URL.Path, body: []byte(strconv.Itoa(count))})
		writeJSON(w, map[string]any{"code": 0})
	}))
	for i, episodeID := range []string{"710000000000101", "710000000000102"} {
		item := HistoryItem{SeriesID: fmt.Sprintf("71000000000000%d", i+1), EpisodeNum: 1, UpdatedAt: 100 + int64(i)}
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
		if err := store.QueueHistoryUpload(item, episodeID); err != nil {
			t.Fatalf("QueueHistoryUpload() error = %v", err)
		}
	}

	if uploaded, err := service.processPendingHistoryUploads(); err != nil || uploaded != 2 {
		t.Fatalf("processPendingHistoryUploads() = %d, %v; want 2, nil", uploaded, err)
	}
	for _, suffix := range []string{"read_history/update", "read_progress/upload"} {
		if calls := log.matching(suffix); len(calls) != 1 || string(calls[0].body) != "2" {
			t.Fatalf("%s calls = %v; want one two-item batch", suffix, calls)
		}
	}
}

func TestFavoritePendingOperationFlushesBeforeRemoteShelfSync(t *testing.T) {
	const seriesID = "710000000000001"
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.add(capturedRequest{path: r.URL.Path, body: body})
		if strings.HasSuffix(r.URL.Path, "bookshelf/video/update/v") {
			writeJSON(w, map[string]any{"code": 0})
			return
		}
		writeJSON(w, map[string]any{"data": map[string]any{"video_shelf_info": []map[string]any{{
			"series_id":   seriesID,
			"modify_time": time.Now().UnixMilli(),
		}}}})
	}))
	if _, err := store.ToggleFavorite(FavoriteItem{
		SeriesID:      seriesID,
		Title:         "Favorite",
		CoverURL:      "https://media.invalid/cover.jpg",
		TotalEpisodes: 10,
	}); err != nil {
		t.Fatalf("ToggleFavorite() error = %v", err)
	}
	if err := store.QueueFavoriteOp(seriesID, true); err != nil {
		t.Fatalf("QueueFavoriteOp() error = %v", err)
	}

	service.syncRemoteFavorites(context.Background(), service.currentFavoriteGeneration())

	mutationCalls := log.matching("bookshelf/video/update")
	shelfCalls := log.matching("bookshelf/video/list")
	if len(mutationCalls) != 1 || len(shelfCalls) != 0 {
		t.Fatalf("mutation/shelf calls = %d/%d, want 1/0", len(mutationCalls), len(shelfCalls))
	}
	ops, err := store.PendingFavoriteOps(10)
	if err != nil || len(ops) != 0 {
		t.Fatalf("pending favorites = %#v, err = %v", ops, err)
	}
	favorites, err := store.GetFavorites()
	if err != nil || len(favorites) != 1 || favorites[0].SeriesID != seriesID {
		t.Fatalf("favorites = %#v, err = %v", favorites, err)
	}
}

func TestRemoteLikesReplaceLocalSet(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(capturedRequest{path: r.URL.Path})
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"mixed_data_list": []map[string]any{{
			"series_id":        "710000000000001",
			"series_title":     "Liked Series",
			"series_cover_url": "https://media.invalid/cover.jpg",
			"video_data": map[string]any{
				"vid":                    "710000000000101",
				"title":                  "Episode",
				"episode_cnt":            10,
				"user_digg_timestamp_ms": time.Now().UnixMilli(),
			},
		}}}})
	}))
	if _, err := store.ToggleLike(LikeItem{SeriesID: "710000000000002", EpisodeID: "old"}); err != nil {
		t.Fatalf("seed local like error = %v", err)
	}

	if err := service.syncRemoteLikes(context.Background(), service.currentLikeGeneration()); err != nil {
		t.Fatalf("syncRemoteLikes() error = %v", err)
	}

	likes, err := store.GetLikes()
	if err != nil {
		t.Fatalf("GetLikes() error = %v", err)
	}
	if len(likes) != 1 || likes[0].SeriesID != "710000000000001" || likes[0].EpisodeID != "710000000000101" {
		t.Fatalf("remote likes = %+v", likes)
	}
	if len(log.matching("ugc/action/mget")) != 1 {
		t.Fatalf("remote like calls = %d, want 1", len(log.matching("ugc/action/mget")))
	}
}

func TestRemoteLikesFollowPagination(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		log.add(capturedRequest{path: r.URL.Path, body: []byte(strconv.Itoa(offset))})
		hasMore := offset == 0
		nextOffset := 0
		ids := []string{"710000000000001", "710000000000002"}
		if offset != 0 {
			ids = []string{"710000000000003"}
			nextOffset = 2
		}
		rows := make([]map[string]any, 0, len(ids))
		for i, seriesID := range ids {
			rows = append(rows, map[string]any{
				"series_id":    seriesID,
				"series_title": "Like " + strconv.Itoa(offset+i+1),
				"video_data": map[string]any{
					"vid":         fmt.Sprintf("7100000000001%02d", offset+i+1),
					"episode_cnt": 10,
				},
			})
		}
		writeJSON(w, map[string]any{
			"code":     0,
			"has_more": hasMore,
			"data": map[string]any{
				"has_more":        hasMore,
				"next_offset":     nextOffset,
				"mixed_data_list": rows,
			},
		})
	}))
	if _, err := store.ToggleLike(LikeItem{SeriesID: "710000000000004", EpisodeID: "old"}); err != nil {
		t.Fatalf("seed local like error = %v", err)
	}

	if err := service.syncRemoteLikes(context.Background(), service.currentLikeGeneration()); err != nil {
		t.Fatalf("syncRemoteLikes() error = %v", err)
	}

	likes, err := store.GetLikes()
	if err != nil {
		t.Fatalf("GetLikes() error = %v", err)
	}
	if len(likes) != 3 {
		t.Fatalf("remote likes = %+v, want three paginated rows", likes)
	}
	calls := log.matching("ugc/action/mget")
	if len(calls) != 2 || string(calls[0].body) != "0" || string(calls[1].body) != "2" {
		t.Fatalf("like page offsets = %v, want 0 then 2", calls)
	}
}

func TestRemoteReservationsUpgradeOnlineStateAndNotify(t *testing.T) {
	const seriesID = "710000000000001"
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(capturedRequest{path: r.URL.Path})
		online, _ := strconv.ParseBool(r.URL.Query().Get("is_online"))
		items := []map[string]any(nil)
		if online {
			items = []map[string]any{{
				"item_id":           "schedule-1",
				"name":              "Schedule title",
				"expected_rec_text": "already online",
				"is_online":         true,
				"subscribe_data": map[string]any{
					"series_id_str": seriesID,
					"title":         "Online Series",
					"cover":         "https://media.invalid/cover.jpg",
				},
			}}
		}
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"subscribe_items": items}})
	}))
	var notifications []Notification
	service.notify = func(_ context.Context, notification Notification) {
		notifications = append(notifications, notification)
	}
	if _, err := store.ToggleReservation(ReservationItem{
		SeriesID: seriesID,
		Title:    "Offline Series",
		Note:     "预约中",
	}); err != nil {
		t.Fatalf("seed reservation error = %v", err)
	}

	results, err := service.syncRemoteReservations(context.Background(), service.currentReservationGeneration())
	if err != nil {
		t.Fatalf("syncRemoteReservations() error = %v", err)
	}
	if len(results) != 1 || !results[0].IsOnline || results[0].SeriesID != seriesID || results[0].Title != "Online Series" {
		t.Fatalf("remote reservations = %+v", results)
	}
	if len(notifications) != 1 || notifications[0].SeriesID != seriesID {
		t.Fatalf("notifications = %+v", notifications)
	}
	if len(log.matching("subscribe/list")) != 2 {
		t.Fatalf("reservation calls = %d, want online and offline", len(log.matching("subscribe/list")))
	}
}
