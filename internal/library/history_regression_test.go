package library

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestRemoteFinalEpisodeUsesSDKDurationAndKeepsShortSeries(t *testing.T) {
	const id = "710000000000001"
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s", r.URL.Path)
		}
		writeJSON(w, map[string]any{"data": map[string]any{"data_list": []map[string]any{{"series_id": id, "vid_index": 3, "episode_cnt": 4, "current_play_time": 96000, "last_watch_time": time.Now().UnixMilli()}}}})
	}))
	service.episodes = func(context.Context, string) (*sdk.Page[model.Episode], error) {
		return &sdk.Page[model.Episode]{Items: []model.Episode{{Number: 4, Duration: 100 * time.Second}}}, nil
	}
	if err := store.SaveHistory(HistoryItem{SeriesID: id, Title: "Short series", CoverURL: "https://example.invalid/cover", ContentType: 1, EpisodeNum: 1, TotalEpisodes: 1, DurationSec: 50, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration()); err != nil {
		t.Fatal(err)
	}
	rows, err := store.GetHistory()
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	row := rows[0]
	if row.ContentType != 1 || row.EpisodeNum != 4 || row.TotalEpisodes != 4 || row.DurationSec != 100 || row.PositionSec != 96 {
		t.Fatalf("incorrect final episode: %+v", row)
	}
	pending, err := store.PendingHistoryDeletes(10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("short series queued for deletion: %+v %v", pending, err)
	}
}

func TestRemoteHistoryErrorCannotEraseLocalHistory(t *testing.T) {
	for _, response := range []map[string]any{{"code": 403, "data": map[string]any{}}, {"code": 0}, {"code": 0, "data": map[string]any{}}} {
		service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, response) }))
		if err := store.SaveHistory(HistoryItem{SeriesID: "710000000000001", Title: "Local", UpdatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		if err := service.syncRemoteHistory(context.Background(), service.currentHistoryGeneration()); err == nil {
			t.Fatal("invalid response accepted")
		}
		rows, err := store.GetHistory()
		if err != nil || len(rows) != 1 {
			t.Fatalf("lost local history: %+v %v", rows, err)
		}
	}
}
