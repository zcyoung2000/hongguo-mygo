package main

import (
	"encoding/json"
	"os/exec"
	"strconv"
)

type OldSeriesJSON struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Cover        string `json:"cover"`
	EpisodeCount int    `json:"episode_count"`
}

func (s *Store) importOldRecords(dbPath string) {
	// Execute sqlite3 command to dump JSON from legacy hgplayer app
	cmd := exec.Command("/usr/bin/sqlite3", "-json", dbPath, "SELECT series_id, episode_index, position_sec, updated_at, series FROM progress ORDER BY updated_at DESC;")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	var rawRows []map[string]any
	if err := json.Unmarshal(out, &rawRows); err != nil {
		return
	}

	var items []HistoryItem
	for _, row := range rawRows {
		seriesID, _ := row["series_id"].(string)
		if seriesID == "" {
			continue
		}
		var epIdx int
		if v, ok := row["episode_index"].(float64); ok {
			epIdx = int(v)
		}
		var posSec float64
		if v, ok := row["position_sec"].(float64); ok {
			posSec = v
		}
		var updatedAt int64
		if v, ok := row["updated_at"].(float64); ok {
			updatedAt = int64(v)
		}

		seriesStr, _ := row["series"].(string)
		title := "短剧 " + seriesID
		cover := ""
		epCount := 80
		if seriesStr != "" {
			var parsed OldSeriesJSON
			if err := json.Unmarshal([]byte(seriesStr), &parsed); err == nil {
				if parsed.Title != "" {
					title = parsed.Title
				}
				if parsed.Cover != "" {
					cover = parsed.Cover
				}
				if parsed.EpisodeCount > 0 {
					epCount = parsed.EpisodeCount
				}
			}
		}

		cover = normalizeImageURL(cover)

		items = append(items, HistoryItem{
			SeriesID:      seriesID,
			Title:         title,
			CoverURL:      cover,
			EpisodeNum:    epIdx + 1,
			EpisodeTitle:  "第" + strconv.Itoa(epIdx+1) + "集",
			PositionSec:   posSec,
			DurationSec:   120.0,
			TotalEpisodes: epCount,
			UpdatedAt:     updatedAt,
			ContentType:   1004,
		})
	}

	if len(items) > 0 {
		_ = s.SaveHistoryBatch(items)
	}
}
