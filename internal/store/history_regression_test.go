package store

import "testing"

func TestHistoryDurationBelongsToWatchedEpisode(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	item := HistoryItem{SeriesID: "710000000000001", Title: "Short series", EpisodeNum: 1, ContentType: 1, TotalEpisodes: 2, DurationSec: 100, PositionSec: 96}
	if err := s.SaveHistory(item); err != nil {
		t.Fatal(err)
	}
	item.EpisodeNum = 2
	item.DurationSec = 0
	item.PositionSec = 96
	if err := s.SaveHistoryBatch([]HistoryItem{item}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.GetHistory()
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if rows[0].DurationSec != 0 {
		t.Fatalf("borrowed previous episode duration: %+v", rows[0])
	}
	item.DurationSec = 120
	if err := s.SaveHistory(item); err != nil {
		t.Fatal(err)
	}
	item.DurationSec = 0
	if err := s.SaveHistory(item); err != nil {
		t.Fatal(err)
	}
	row, _ := s.GetHistoryItem(item.SeriesID)
	if row.DurationSec != 120 {
		t.Fatalf("lost same-episode duration: %+v", row)
	}
}
