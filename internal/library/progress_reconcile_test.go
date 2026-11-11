package library

import (
	"context"
	"net/http"
	"testing"
)

func TestReconcileSeriesProgressUsesBatchEndpointAndMergesRules(t *testing.T) {
	const seriesID = "710000000000001"
	remote := map[string]any{
		"series_id": seriesID, "vid_index": 1, "current_play_time": 10000,
		"last_watch_time": 4000, "episode_cnt": 10,
	}
	for _, local := range []HistoryItem{
		{SeriesID: seriesID, Title: "Local", EpisodeNum: 1, PositionSec: 6, DurationSec: 100, TotalEpisodes: 10, UpdatedAt: 2},
		{SeriesID: seriesID, Title: "Local", EpisodeNum: 2, PositionSec: 6, DurationSec: 100, TotalEpisodes: 10, UpdatedAt: 2},
		{SeriesID: seriesID, Title: "Local", EpisodeNum: 3, PositionSec: 20, DurationSec: 100, TotalEpisodes: 10, UpdatedAt: 2},
		{SeriesID: seriesID, Title: "Local", EpisodeNum: 2, PositionSec: 9, DurationSec: 100, TotalEpisodes: 10, UpdatedAt: 2},
	} {
		service, store := newSyncTestService(t, seriesProgressHandler([]map[string]any{remote}))
		if err := store.SaveHistory(local); err != nil {
			t.Fatal(err)
		}
		merged, err := service.ReconcileSeriesProgress(context.Background(), seriesID)
		if err != nil {
			t.Fatal(err)
		}
		if merged.EpisodeNum != 2 || merged.PositionSec != 10 || merged.Title != "Local" || merged.DurationSec != 100 {
			t.Fatalf("merged progress = %#v, want remote episode 2 at 10 seconds", merged)
		}
		_ = local
	}
}

func TestReconcileSeriesProgressKeepsNewerLocalProgress(t *testing.T) {
	const seriesID = "710000000000001"
	service, store := newSyncTestService(t, seriesProgressHandler([]map[string]any{{
		"series_id": seriesID, "vid_index": 1, "current_play_time": 10000,
		"last_watch_time": 1000, "episode_cnt": 10,
	}}))
	if err := store.SaveHistory(HistoryItem{
		SeriesID: seriesID, Title: "Local", EpisodeNum: 1, PositionSec: 12,
		DurationSec: 100, TotalEpisodes: 10, UpdatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := service.ReconcileSeriesProgress(context.Background(), seriesID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PositionSec != 12 || got.UpdatedAt != 2 {
		t.Fatalf("merged progress = %#v, want newer local row", got)
	}
}

func seriesProgressHandler(rows []map[string]any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/reading/bookapi/read_progress/list/v1/" || request.URL.Query().Get("series_ids") == "" {
			writeJSON(w, map[string]any{"code": 1001})
			return
		}
		writeJSON(w, map[string]any{"code": 0, "data": map[string]any{"data_list": rows}})
	})
}

func TestMergeProgressSameTimestampPrefersFartherPlayback(t *testing.T) {
	local := HistoryItem{SeriesID: "1", EpisodeNum: 2, PositionSec: 10, UpdatedAt: 10}
	remote := HistoryItem{SeriesID: "1", EpisodeNum: 1, PositionSec: 90, UpdatedAt: 10}
	if got := mergeProgress(local, remote); got.EpisodeNum != 2 || got.PositionSec != 10 {
		t.Fatalf("merge = %#v, want farther same-time progress", got)
	}
	if got := mergeProgress(remote, local); got.EpisodeNum != 2 || got.PositionSec != 10 {
		t.Fatalf("commutative merge = %#v", got)
	}
}
