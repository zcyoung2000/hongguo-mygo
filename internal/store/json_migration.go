package store

import (
	"time"

	"myguo/internal/mediautil"
)

// Data and its completion marker commit together. Existing SQLite rows win
// over legacy snapshots, including when an older release partially imported.
func (s *Store) importJSONTransaction(data UserData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var completed bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM migration_markers WHERE name = 'userdata.json')`).Scan(&completed); err != nil {
		return err
	}
	if completed {
		return nil
	}
	p := data.Profile
	if p.UserID != "" && p.UserID != "anonymous" {
		if _, err := tx.Exec(`INSERT INTO profile (id, user_id, nickname, avatar, device_id, logged_in)
			VALUES (1, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
			p.UserID, p.Nickname, p.Avatar, p.DeviceID, p.LoggedIn); err != nil {
			return err
		}
	}
	clearAt, tombstones, err := s.historyExclusionsLocked(tx)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, item := range data.History {
		if item.SeriesID == "" || item.ContentType == 1 || tombstones[item.SeriesID] {
			continue
		}
		if item.UpdatedAt <= 0 {
			if clearAt > 0 {
				continue
			}
			item.UpdatedAt = now
		}
		if item.UpdatedAt <= clearAt {
			continue
		}
		if item.ContentType == 0 {
			item.ContentType = 1004
		}
		if _, err := tx.Exec(`INSERT INTO history (
			series_id, title, cover_url, episode_num, episode_title, position_sec,
			duration_sec, total_episodes, updated_at, intro, content_type)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(series_id) DO NOTHING`,
			item.SeriesID, item.Title, mediautil.NormalizeImageURL(item.CoverURL), item.EpisodeNum,
			item.EpisodeTitle, item.PositionSec, item.DurationSec, item.TotalEpisodes,
			item.UpdatedAt, item.Intro, item.ContentType); err != nil {
			return err
		}
	}
	for _, item := range data.Favorites {
		if item.SeriesID == "" {
			continue
		}
		if item.AddedAt <= 0 {
			item.AddedAt = now
		}
		if _, err := tx.Exec(`INSERT INTO favorites (series_id, title, cover_url, description, total_episodes, added_at)
			VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(series_id) DO NOTHING`, item.SeriesID, item.Title,
			mediautil.NormalizeImageURL(item.CoverURL), item.Description, item.TotalEpisodes, item.AddedAt); err != nil {
			return err
		}
	}
	for _, item := range data.Likes {
		if item.SeriesID == "" || item.EpisodeID == "" {
			continue
		}
		if item.AddedAt <= 0 {
			item.AddedAt = now
		}
		if _, err := tx.Exec(`INSERT INTO likes (series_id, episode_id, title, cover_url, episode_num, added_at)
			VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(series_id, episode_id) DO NOTHING`, item.SeriesID,
			item.EpisodeID, item.Title, mediautil.NormalizeImageURL(item.CoverURL), item.EpisodeNum, item.AddedAt); err != nil {
			return err
		}
	}
	for _, item := range data.Reservations {
		if item.SeriesID == "" {
			continue
		}
		if item.AddedAt <= 0 {
			item.AddedAt = now
		}
		if _, err := tx.Exec(`INSERT INTO reservations (series_id, title, cover_url, note, publish_at, is_online, added_at)
			VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(series_id) DO NOTHING`, item.SeriesID, item.Title,
			mediautil.NormalizeImageURL(item.CoverURL), item.Note, item.PublishAt, item.IsOnline, item.AddedAt); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO migration_markers (name, completed_at) VALUES ('userdata.json', ?)`, now); err != nil {
		return err
	}
	return tx.Commit()
}
