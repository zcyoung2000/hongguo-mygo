package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"myguo/internal/mediautil"
)

type UserProfile struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	DeviceID string `json:"deviceId"`
	LoggedIn bool   `json:"loggedIn"`
}

type HistoryItem struct {
	SeriesID      string  `json:"seriesId"`
	Title         string  `json:"title"`
	CoverURL      string  `json:"coverUrl"`
	EpisodeNum    int     `json:"episodeNum"`
	EpisodeTitle  string  `json:"episodeTitle"`
	PositionSec   float64 `json:"positionSec"`
	DurationSec   float64 `json:"durationSec"`
	TotalEpisodes int     `json:"totalEpisodes"`
	UpdatedAt     int64   `json:"updatedAt"`
	Intro         string  `json:"intro,omitempty"`
	ContentType   int     `json:"contentType,omitempty"`
}

type FavoriteItem struct {
	SeriesID      string `json:"seriesId"`
	Title         string `json:"title"`
	CoverURL      string `json:"coverUrl"`
	Description   string `json:"description"`
	TotalEpisodes int    `json:"totalEpisodes"`
	AddedAt       int64  `json:"addedAt"`
}

type LikeItem struct {
	SeriesID      string `json:"seriesId"`
	EpisodeID     string `json:"episodeId"`
	Title         string `json:"title"`
	CoverURL      string `json:"coverUrl"`
	EpisodeNum    int    `json:"episodeNum"`
	TotalEpisodes int    `json:"totalEpisodes,omitempty"`
	Description   string `json:"description,omitempty"`
	AddedAt       int64  `json:"addedAt"`
}

type ReservationItem struct {
	SeriesID  string `json:"seriesId"`
	Title     string `json:"title"`
	CoverURL  string `json:"coverUrl"`
	Note      string `json:"note"`
	PublishAt string `json:"publishAt"`
	IsOnline  bool   `json:"isOnline"`
	AddedAt   int64  `json:"addedAt"`
}

type PendingHistoryUpload struct {
	HistoryItem
	EpisodeID string `json:"episodeId"`
	Scope     string `json:"-"`
	Revision  int64  `json:"-"`
}

type PendingHistoryDelete struct {
	HistoryItem
	EpisodeID string `json:"episodeId"`
	Scope     string `json:"-"`
	Revision  int64  `json:"-"`
}

type PendingFavoriteOp struct {
	SeriesID  string `json:"seriesId"`
	Added     bool   `json:"added"`
	UpdatedAt int64  `json:"updatedAt"`
	Scope     string `json:"-"`
	Revision  int64  `json:"-"`
}

// UserData holds legacy JSON schema for one-time auto-migration
type UserData struct {
	Profile      UserProfile                `json:"profile"`
	History      map[string]HistoryItem     `json:"history"`
	Favorites    map[string]FavoriteItem    `json:"favorites"`
	Likes        map[string]LikeItem        `json:"likes"`
	Reservations map[string]ReservationItem `json:"reservations"`
}

type Store struct {
	mu        sync.RWMutex
	db        *sql.DB
	dbPath    string
	syncScope string
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "userdata.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Optimize SQLite performance & concurrency
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA busy_timeout = 5000;
	`); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{
		db:     db,
		dbPath: dbPath,
	}

	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	// Versions before account-backed likes synthesized rows from history.
	// They used the synthetic "ep_<series>_<num>" ID convention.
	_, _ = s.db.Exec(`DELETE FROM likes WHERE episode_id LIKE 'ep\_%' ESCAPE '\'`)
	// A test once wrote "999" into production favorites. Those rows can never
	// resolve to an episode and make the followed feed stall on its first card.
	_, _ = s.db.Exec(`DELETE FROM favorites WHERE series_id GLOB '[0-9]*' AND length(trim(series_id)) < 6`)

	// Check if migration from userdata.json is needed
	jsonPath := filepath.Join(dir, "userdata.json")
	if _, err := os.Stat(jsonPath); err == nil {
		if migrationNeeded, err := s.jsonMigrationNeeded(); err != nil {
			log.Printf("[Store] Warning: could not inspect userdata.json migration state: %v", err)
		} else if migrationNeeded {
			if err := s.migrateFromJSON(jsonPath); err != nil {
				log.Printf("[Store] Warning: failed to migrate from userdata.json: %v", err)
			}
		}
	} else {
		// If neither db nor json has data, check old legacy app data
		var histCount int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM history`).Scan(&histCount)
		if histCount == 0 {
			s.importOldAppData()
		}
	}

	// Do this after legacy import too, because old JSON may contain the same
	// synthesized placeholders.
	_, _ = s.db.Exec(`DELETE FROM likes WHERE episode_id LIKE 'ep\_%' ESCAPE '\'`)
	_, _ = s.db.Exec(`DELETE FROM favorites WHERE series_id GLOB '[0-9]*' AND length(trim(series_id)) < 6`)

	return s, nil
}

func (s *Store) jsonMigrationNeeded() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var completed bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM migration_markers WHERE name = 'userdata.json')`).Scan(&completed)
	return !completed, err
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS migration_markers (
		name TEXT PRIMARY KEY,
		completed_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS profile (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		user_id TEXT NOT NULL DEFAULT '',
		nickname TEXT NOT NULL DEFAULT '',
		avatar TEXT NOT NULL DEFAULT '',
		device_id TEXT NOT NULL DEFAULT '',
		logged_in INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS history (
		series_id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		cover_url TEXT NOT NULL DEFAULT '',
		episode_num INTEGER NOT NULL DEFAULT 1,
		episode_title TEXT NOT NULL DEFAULT '',
		position_sec REAL NOT NULL DEFAULT 0.0,
		duration_sec REAL NOT NULL DEFAULT 0.0,
		total_episodes INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0,
		intro TEXT NOT NULL DEFAULT '',
		content_type INTEGER NOT NULL DEFAULT 1004
	);
	CREATE INDEX IF NOT EXISTS idx_history_updated ON history(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_history_content_type ON history(content_type);

	CREATE TABLE IF NOT EXISTS history_tombstones (
		series_id TEXT PRIMARY KEY,
		deleted_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS history_clear_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		cleared_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS pending_history_uploads (
		series_id TEXT PRIMARY KEY,
		episode_id TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		cover_url TEXT NOT NULL DEFAULT '',
		episode_num INTEGER NOT NULL DEFAULT 1,
		episode_title TEXT NOT NULL DEFAULT '',
		position_sec REAL NOT NULL DEFAULT 0.0,
		duration_sec REAL NOT NULL DEFAULT 0.0,
		total_episodes INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0,
		intro TEXT NOT NULL DEFAULT '',
		content_type INTEGER NOT NULL DEFAULT 1004
	);
	CREATE INDEX IF NOT EXISTS idx_pending_history_uploads_updated
		ON pending_history_uploads(updated_at);

	CREATE TABLE IF NOT EXISTS pending_history_deletes (
		series_id TEXT PRIMARY KEY,
		episode_id TEXT NOT NULL DEFAULT '',
		episode_num INTEGER NOT NULL DEFAULT 1,
		deleted_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_pending_history_deletes_deleted
		ON pending_history_deletes(deleted_at);

	CREATE TABLE IF NOT EXISTS favorites (
		series_id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		cover_url TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		total_episodes INTEGER NOT NULL DEFAULT 0,
		added_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_favorites_added ON favorites(added_at DESC);

	CREATE TABLE IF NOT EXISTS pending_favorite_ops (
		series_id TEXT PRIMARY KEY,
		added INTEGER NOT NULL DEFAULT 1,
		updated_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_pending_favorite_ops_updated
		ON pending_favorite_ops(updated_at);

	CREATE TABLE IF NOT EXISTS likes (
		series_id TEXT NOT NULL,
		episode_id TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		cover_url TEXT NOT NULL DEFAULT '',
		episode_num INTEGER NOT NULL DEFAULT 1,
		added_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (series_id, episode_id)
	);
	CREATE INDEX IF NOT EXISTS idx_likes_added ON likes(added_at DESC);
	CREATE INDEX IF NOT EXISTS idx_likes_series_ep ON likes(series_id, episode_num);

	CREATE TABLE IF NOT EXISTS reservations (
		series_id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		cover_url TEXT NOT NULL DEFAULT '',
		note TEXT NOT NULL DEFAULT '',
		publish_at TEXT NOT NULL DEFAULT '',
		is_online INTEGER NOT NULL DEFAULT 0,
		added_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_reservations_added ON reservations(added_at DESC);

	CREATE TABLE IF NOT EXISTS feed_progress (
		channel TEXT PRIMARY KEY,
		cursor TEXT NOT NULL DEFAULT '',
		generation INTEGER NOT NULL DEFAULT 1,
		updated_at INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS feed_seen (
		channel TEXT NOT NULL,
		generation INTEGER NOT NULL,
		series_id TEXT NOT NULL,
		seen_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (channel, generation, series_id)
	);
	CREATE INDEX IF NOT EXISTS idx_feed_seen_recent ON feed_seen(channel, generation, seen_at DESC);

	CREATE TABLE IF NOT EXISTS media_cache (
		series_id TEXT NOT NULL,
		episode_id TEXT NOT NULL,
		episode_num INTEGER NOT NULL DEFAULT 1,
		quality TEXT NOT NULL DEFAULT '720p',
		updated_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (series_id, episode_id, quality)
	);
	CREATE INDEX IF NOT EXISTS idx_media_cache_series ON media_cache(series_id, episode_num, updated_at DESC);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return s.initSyncQueues()
}

type CachedEpisode struct {
	SeriesID   string
	EpisodeID  string
	EpisodeNum int
	Quality    string
}

func (s *Store) RememberEpisode(seriesID, episodeID string, episodeNum int, quality string) error {
	if seriesID == "" || episodeID == "" {
		return nil
	}
	quality = mediautil.NormalizeVideoQuality(quality)
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO media_cache (series_id, episode_id, episode_num, quality, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(series_id, episode_id, quality) DO UPDATE SET
			episode_num = excluded.episode_num,
			updated_at = excluded.updated_at
	`, seriesID, episodeID, episodeNum, quality, time.Now().Unix())
	return err
}

func (s *Store) GetCachedEpisode(seriesID, quality string) (CachedEpisode, bool) {
	if seriesID == "" {
		return CachedEpisode{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Prefer the requested quality, then the most recently cached alternative.
	var item CachedEpisode
	err := s.db.QueryRow(`
		SELECT series_id, episode_id, episode_num, quality
		FROM media_cache
		WHERE series_id = ? AND quality = ?
		ORDER BY episode_num, updated_at DESC
		LIMIT 1
	`, seriesID, mediautil.NormalizeVideoQuality(quality)).Scan(
		&item.SeriesID, &item.EpisodeID, &item.EpisodeNum, &item.Quality,
	)
	if err == nil {
		return item, true
	}

	err = s.db.QueryRow(`
		SELECT series_id, episode_id, episode_num, quality
		FROM media_cache
		WHERE series_id = ?
		ORDER BY updated_at DESC, episode_num
		LIMIT 1
	`, seriesID).Scan(
		&item.SeriesID, &item.EpisodeID, &item.EpisodeNum, &item.Quality,
	)
	if err != nil {
		return CachedEpisode{}, false
	}
	return item, true
}

func (s *Store) GetCachedEpisodeAt(seriesID, quality string, episodeNum int) (CachedEpisode, bool) {
	if seriesID == "" || episodeNum <= 0 {
		return CachedEpisode{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var item CachedEpisode
	err := s.db.QueryRow(`
		SELECT series_id, episode_id, episode_num, quality
		FROM media_cache
		WHERE series_id = ? AND episode_num = ? AND quality = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, seriesID, episodeNum, mediautil.NormalizeVideoQuality(quality)).Scan(
		&item.SeriesID, &item.EpisodeID, &item.EpisodeNum, &item.Quality,
	)
	if err == nil {
		return item, true
	}

	err = s.db.QueryRow(`
		SELECT series_id, episode_id, episode_num, quality
		FROM media_cache
		WHERE series_id = ? AND episode_num = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, seriesID, episodeNum).Scan(
		&item.SeriesID, &item.EpisodeID, &item.EpisodeNum, &item.Quality,
	)
	if err != nil {
		return CachedEpisode{}, false
	}
	return item, true
}

func (s *Store) ClearCachedEpisodes() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM media_cache`)
	return err
}

// PruneCachedEpisodes retains the index for media intentionally kept in use
// during cache cleanup, including complete HLS renditions.
func (s *Store) PruneCachedEpisodes(keep func(CachedEpisode) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT series_id, episode_id, episode_num, quality FROM media_cache`)
	if err != nil {
		return err
	}
	var removed []CachedEpisode
	for rows.Next() {
		var item CachedEpisode
		if err := rows.Scan(&item.SeriesID, &item.EpisodeID, &item.EpisodeNum, &item.Quality); err != nil {
			rows.Close()
			return err
		}
		if !keep(item) {
			removed = append(removed, item)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range removed {
		if _, err := tx.Exec(`DELETE FROM media_cache WHERE series_id = ? AND episode_id = ? AND quality = ?`, item.SeriesID, item.EpisodeID, item.Quality); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type FeedProgress struct {
	Channel    string
	Cursor     string
	Generation int64
}

func (s *Store) GetFeedProgress(channel string) (FeedProgress, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var progress FeedProgress
	err := s.db.QueryRow(`
		SELECT channel, cursor, generation
		FROM feed_progress
		WHERE channel = ?
	`, channel).Scan(&progress.Channel, &progress.Cursor, &progress.Generation)
	if err != nil {
		return FeedProgress{Channel: channel, Generation: 1}, false
	}
	return progress, true
}

func (s *Store) SaveFeedProgress(channel, cursor string, generation int64) error {
	if generation <= 0 {
		generation = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO feed_progress (channel, cursor, generation, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(channel) DO UPDATE SET
			cursor = excluded.cursor,
			generation = excluded.generation,
			updated_at = excluded.updated_at
	`, channel, cursor, generation, time.Now().Unix())
	return err
}

func (s *Store) GetFeedSeenIDs(channel string, generation int64) map[string]bool {
	seen := make(map[string]bool)
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT series_id
		FROM feed_seen
		WHERE channel = ? AND generation = ?
	`, channel, generation)
	if err != nil {
		return seen
	}
	defer rows.Close()

	for rows.Next() {
		var seriesID string
		if err := rows.Scan(&seriesID); err == nil {
			seen[seriesID] = true
		}
	}
	return seen
}

func (s *Store) SaveFeedSeenIDs(channel string, generation int64, seriesIDs []string) error {
	if len(seriesIDs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO feed_seen (channel, generation, series_id, seen_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, seriesID := range seriesIDs {
		if seriesID == "" {
			continue
		}
		if _, err := stmt.Exec(channel, generation, seriesID, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) StartNextFeedGeneration(channel string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var generation int64
	err := s.db.QueryRow(`
		SELECT generation FROM feed_progress WHERE channel = ?
	`, channel).Scan(&generation)
	if err != nil {
		generation = 1
	}
	generation++
	_, err = s.db.Exec(`
		INSERT INTO feed_progress (channel, cursor, generation, updated_at)
		VALUES (?, '', ?, ?)
		ON CONFLICT(channel) DO UPDATE SET
			cursor = '',
			generation = excluded.generation,
			updated_at = excluded.updated_at
	`, channel, generation, time.Now().Unix())
	if err != nil {
		return generation, err
	}
	if _, err = s.db.Exec(`
		DELETE FROM feed_seen
		WHERE channel = ? AND generation < ?
	`, channel, generation); err != nil {
		return generation, err
	}
	return generation, nil
}

func (s *Store) migrateFromJSON(jsonPath string) error {
	dataBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var data UserData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return err
	}

	if err := s.importJSONTransaction(data); err != nil {
		return err
	}

	// Backup original userdata.json
	bakPath := jsonPath + ".bak"
	if err := os.Rename(jsonPath, bakPath); err != nil {
		return fmt.Errorf("数据已迁移，但归档旧 JSON 失败: %w", err)
	}
	log.Printf("[Store] Migrated and archived %s -> %s", jsonPath, bakPath)
	return nil
}

func (s *Store) importOldAppData() {
	if os.Getenv("MYGUO_TEST_NO_LEGACY_IMPORT") == "1" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldAccountPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	if accBytes, err := os.ReadFile(oldAccountPath); err == nil {
		var oldAcc struct {
			Profile struct {
				LoggedIn bool   `json:"loggedIn"`
				UserID   string `json:"userId"`
				Nickname string `json:"nickname"`
				Avatar   string `json:"avatar"`
			} `json:"profile"`
		}
		if json.Unmarshal(accBytes, &oldAcc) == nil && oldAcc.Profile.Nickname != "" {
			_ = s.UpdateProfile(UserProfile{
				UserID:   oldAcc.Profile.UserID,
				Nickname: oldAcc.Profile.Nickname,
				Avatar:   oldAcc.Profile.Avatar,
				LoggedIn: oldAcc.Profile.LoggedIn,
			})
		}
	}

	oldDbPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "hgplayer.db")
	if _, err := os.Stat(oldDbPath); err == nil {
		s.importOldRecords(oldDbPath)
	}
}

func (s *Store) SaveHistory(item HistoryItem, scopes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.UpdatedAt <= 0 {
		item.UpdatedAt = time.Now().Unix()
	}
	if item.ContentType == 0 {
		item.ContentType = 1004
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM history_tombstones WHERE series_id = ?`, item.SeriesID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM pending_history_deletes WHERE series_id = ? AND scope = ?`, item.SeriesID, s.syncScopeLocked(scopes)); err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO history (
			series_id, title, cover_url, episode_num, episode_title,
			position_sec, duration_sec, total_episodes, updated_at, intro, content_type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET
			title = CASE WHEN excluded.title != '' THEN excluded.title ELSE history.title END,
			cover_url = CASE WHEN excluded.cover_url != '' THEN excluded.cover_url ELSE history.cover_url END,
			episode_num = excluded.episode_num,
			episode_title = CASE WHEN excluded.episode_title != '' THEN excluded.episode_title ELSE history.episode_title END,
			position_sec = excluded.position_sec,
			duration_sec = CASE WHEN excluded.duration_sec > 0 THEN excluded.duration_sec ELSE history.duration_sec END,
			total_episodes = CASE WHEN excluded.total_episodes > 0 THEN excluded.total_episodes ELSE history.total_episodes END,
			updated_at = excluded.updated_at,
			intro = CASE WHEN excluded.intro != '' THEN excluded.intro ELSE history.intro END,
			content_type = CASE WHEN excluded.content_type != 0 THEN excluded.content_type ELSE history.content_type END
	`, item.SeriesID, item.Title, item.CoverURL, item.EpisodeNum, item.EpisodeTitle,
		item.PositionSec, item.DurationSec, item.TotalEpisodes, item.UpdatedAt, item.Intro, item.ContentType)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveHistoryBatch(items []HistoryItem, scopes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO history (
			series_id, title, cover_url, episode_num, episode_title,
			position_sec, duration_sec, total_episodes, updated_at, intro, content_type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET
			title = CASE WHEN excluded.title != '' THEN excluded.title ELSE history.title END,
			cover_url = CASE WHEN excluded.cover_url != '' THEN excluded.cover_url ELSE history.cover_url END,
			episode_num = excluded.episode_num,
			episode_title = CASE WHEN excluded.episode_title != '' THEN excluded.episode_title ELSE history.episode_title END,
			position_sec = excluded.position_sec,
			duration_sec = CASE WHEN excluded.duration_sec > 0 THEN excluded.duration_sec ELSE history.duration_sec END,
			total_episodes = CASE WHEN excluded.total_episodes > 0 THEN excluded.total_episodes ELSE history.total_episodes END,
			updated_at = excluded.updated_at,
			intro = CASE WHEN excluded.intro != '' THEN excluded.intro ELSE history.intro END,
			content_type = CASE WHEN excluded.content_type != 0 THEN excluded.content_type ELSE history.content_type END
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	clearAt, tombstones, err := s.historyExclusionsLocked(tx)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.ContentType == 1 {
			continue // Skip novels
		}
		if item.UpdatedAt <= 0 {
			item.UpdatedAt = time.Now().Unix()
		}
		if item.ContentType == 0 {
			item.ContentType = 1004
		}
		if item.UpdatedAt <= clearAt {
			continue
		}
		if tombstones[item.SeriesID] {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM pending_history_deletes WHERE series_id = ? AND scope = ?`, item.SeriesID, s.syncScopeLocked(scopes)); err != nil {
			return err
		}
		if _, err := stmt.Exec(
			item.SeriesID, item.Title, item.CoverURL, item.EpisodeNum, item.EpisodeTitle,
			item.PositionSec, item.DurationSec, item.TotalEpisodes, item.UpdatedAt, item.Intro, item.ContentType,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) historyExclusionsLocked(tx *sql.Tx) (int64, map[string]bool, error) {
	var clearAt int64
	if err := tx.QueryRow(`SELECT cleared_at FROM history_clear_state WHERE id = 1`).Scan(&clearAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, nil, err
	}
	rows, err := tx.Query(`SELECT series_id FROM history_tombstones`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	deleted := make(map[string]bool)
	for rows.Next() {
		var seriesID string
		if err := rows.Scan(&seriesID); err != nil {
			return 0, nil, err
		}
		deleted[seriesID] = true
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	return clearAt, deleted, nil
}

func (s *Store) GetHistory() ([]HistoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT series_id, title, cover_url, episode_num, episode_title,
		       position_sec, duration_sec, total_episodes, updated_at, intro, content_type
		FROM history
		WHERE content_type != 1
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []HistoryItem
	for rows.Next() {
		var it HistoryItem
		if err := rows.Scan(
			&it.SeriesID, &it.Title, &it.CoverURL, &it.EpisodeNum, &it.EpisodeTitle,
			&it.PositionSec, &it.DurationSec, &it.TotalEpisodes, &it.UpdatedAt, &it.Intro, &it.ContentType,
		); err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) GetHistoryItem(seriesID string) (HistoryItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var it HistoryItem
	err := s.db.QueryRow(`
		SELECT series_id, title, cover_url, episode_num, episode_title,
		       position_sec, duration_sec, total_episodes, updated_at, intro, content_type
		FROM history
		WHERE series_id = ?
	`, seriesID).Scan(
		&it.SeriesID, &it.Title, &it.CoverURL, &it.EpisodeNum, &it.EpisodeTitle,
		&it.PositionSec, &it.DurationSec, &it.TotalEpisodes, &it.UpdatedAt, &it.Intro, &it.ContentType,
	)
	if err != nil {
		return HistoryItem{}, false
	}
	return it, true
}

func (s *Store) DeleteHistory(seriesID string, scopes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var item HistoryItem
	var episodeID string
	err = tx.QueryRow(`
		SELECT series_id, title, cover_url, episode_num, episode_title,
		       position_sec, duration_sec, total_episodes, updated_at, intro, content_type
		       ,COALESCE((
		           SELECT episode_id FROM media_cache
		           WHERE media_cache.series_id = history.series_id
		             AND media_cache.episode_num = history.episode_num
		           ORDER BY updated_at DESC LIMIT 1
		       ), '')
		FROM history WHERE series_id = ?
	`, seriesID).Scan(
		&item.SeriesID, &item.Title, &item.CoverURL, &item.EpisodeNum, &item.EpisodeTitle,
		&item.PositionSec, &item.DurationSec, &item.TotalEpisodes, &item.UpdatedAt, &item.Intro, &item.ContentType,
		&episodeID,
	)
	if err == nil {
		revision, revErr := nextSyncRevision(tx)
		if revErr != nil {
			return revErr
		}
		if _, err := tx.Exec(`
			INSERT INTO pending_history_deletes (series_id, episode_id, episode_num, deleted_at, scope, revision)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(scope, series_id) DO UPDATE SET
				episode_id = excluded.episode_id,
				episode_num = excluded.episode_num,
				deleted_at = excluded.deleted_at, revision = excluded.revision
		`, seriesID, episodeID, item.EpisodeNum, time.Now().Unix(), s.syncScopeLocked(scopes), revision); err != nil {
			return err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM history WHERE series_id = ?`, seriesID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO history_tombstones (series_id, deleted_at) VALUES (?, ?)
		ON CONFLICT(series_id) DO UPDATE SET deleted_at = excluded.deleted_at
	`, seriesID, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ClearHistory(scopes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
		SELECT series_id,
		       COALESCE((
		           SELECT episode_id FROM media_cache
		           WHERE media_cache.series_id = history.series_id
		             AND media_cache.episode_num = history.episode_num
		           ORDER BY updated_at DESC LIMIT 1
		       ), ''),
		       episode_num
		FROM history
	`)
	if err != nil {
		return err
	}
	type pendingDelete struct {
		seriesID   string
		episodeID  string
		episodeNum int
	}
	var pending []pendingDelete
	for rows.Next() {
		var item pendingDelete
		if err := rows.Scan(&item.seriesID, &item.episodeID, &item.episodeNum); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	now := time.Now().Unix()
	revision, err := nextSyncRevision(tx)
	if err != nil {
		return err
	}
	for _, item := range pending {
		if _, err := tx.Exec(`
			INSERT INTO pending_history_deletes (series_id, episode_id, episode_num, deleted_at, scope, revision)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(scope, series_id) DO UPDATE SET
				episode_id = excluded.episode_id,
				episode_num = excluded.episode_num,
				deleted_at = excluded.deleted_at, revision = excluded.revision
		`, item.seriesID, item.episodeID, item.episodeNum, now, s.syncScopeLocked(scopes), revision); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM history`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO history_clear_state (id, cleared_at) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET cleared_at = excluded.cleared_at
	`, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) QueueHistoryUpload(item HistoryItem, episodeID string, scopes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.SeriesID == "" {
		return fmt.Errorf("待同步历史 ID 无效")
	}
	if item.UpdatedAt <= 0 {
		item.UpdatedAt = time.Now().Unix()
	}
	if item.ContentType == 0 {
		item.ContentType = 1004
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	revision, err := nextSyncRevision(tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO pending_history_uploads (
			series_id, episode_id, title, cover_url, episode_num, episode_title,
			position_sec, duration_sec, total_episodes, updated_at, intro, content_type, scope, revision
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, series_id) DO UPDATE SET
			episode_id = excluded.episode_id,
			title = excluded.title,
			cover_url = excluded.cover_url,
			episode_num = excluded.episode_num,
			episode_title = excluded.episode_title,
			position_sec = excluded.position_sec,
			duration_sec = excluded.duration_sec,
			total_episodes = excluded.total_episodes,
			updated_at = excluded.updated_at,
			intro = excluded.intro,
			content_type = excluded.content_type, revision = excluded.revision
	`, item.SeriesID, episodeID, item.Title, item.CoverURL, item.EpisodeNum, item.EpisodeTitle,
		item.PositionSec, item.DurationSec, item.TotalEpisodes, item.UpdatedAt, item.Intro, item.ContentType, s.syncScopeLocked(scopes), revision)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PendingHistoryUploads(limit int, scopes ...string) ([]PendingHistoryUpload, error) {
	if limit <= 0 {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT series_id, episode_id, title, cover_url, episode_num, episode_title,
		       position_sec, duration_sec, total_episodes, updated_at, intro, content_type, scope, revision
		FROM pending_history_uploads
		WHERE scope = ?
		ORDER BY updated_at
		LIMIT ?
	`, s.syncScopeLocked(scopes), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PendingHistoryUpload
	for rows.Next() {
		var item PendingHistoryUpload
		if err := rows.Scan(
			&item.SeriesID, &item.EpisodeID, &item.Title, &item.CoverURL, &item.EpisodeNum, &item.EpisodeTitle,
			&item.PositionSec, &item.DurationSec, &item.TotalEpisodes, &item.UpdatedAt, &item.Intro, &item.ContentType, &item.Scope, &item.Revision,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeletePendingHistoryUploads(seriesIDs []string, scopes ...string) error {
	if len(seriesIDs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, seriesID := range seriesIDs {
		if _, err := tx.Exec(`DELETE FROM pending_history_uploads WHERE series_id = ? AND scope = ?`, seriesID, s.syncScopeLocked(scopes)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PendingHistoryDeletes(limit int, scopes ...string) ([]PendingHistoryDelete, error) {
	if limit <= 0 {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT p.series_id, COALESCE(h.title, ''), COALESCE(h.cover_url, ''),
		       p.episode_num, COALESCE(h.episode_title, ''), 0, 0,
		       COALESCE(h.total_episodes, 0), p.deleted_at, COALESCE(h.intro, ''), 1004,
		       p.episode_id, p.scope, p.revision
		FROM pending_history_deletes p
		LEFT JOIN history h ON h.series_id = p.series_id
		WHERE p.scope = ?
		ORDER BY p.deleted_at
		LIMIT ?
	`, s.syncScopeLocked(scopes), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PendingHistoryDelete
	for rows.Next() {
		var item PendingHistoryDelete
		if err := rows.Scan(
			&item.SeriesID, &item.Title, &item.CoverURL, &item.EpisodeNum, &item.EpisodeTitle,
			&item.PositionSec, &item.DurationSec, &item.TotalEpisodes, &item.UpdatedAt, &item.Intro, &item.ContentType,
			&item.EpisodeID, &item.Scope, &item.Revision,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) DeletePendingHistoryDeletes(seriesIDs []string, scopes ...string) error {
	if len(seriesIDs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, seriesID := range seriesIDs {
		if _, err := tx.Exec(`DELETE FROM pending_history_deletes WHERE series_id = ? AND scope = ?`, seriesID, s.syncScopeLocked(scopes)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) HistoryDeleted(seriesID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var deletedAt int64
	err := s.db.QueryRow(`SELECT deleted_at FROM history_tombstones WHERE series_id = ?`, seriesID).Scan(&deletedAt)
	return err == nil
}

func (s *Store) ToggleFavorite(item FavoriteItem) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM favorites WHERE series_id = ?`, item.SeriesID).Scan(&exists)
	if err == nil && exists == 1 {
		_, err := s.db.Exec(`DELETE FROM favorites WHERE series_id = ?`, item.SeriesID)
		return false, err
	}

	if item.AddedAt <= 0 {
		item.AddedAt = time.Now().Unix()
	}
	_, err = s.db.Exec(`
		INSERT INTO favorites (series_id, title, cover_url, description, total_episodes, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET
			title = excluded.title,
			cover_url = excluded.cover_url,
			description = excluded.description,
			total_episodes = excluded.total_episodes,
			added_at = excluded.added_at
	`, item.SeriesID, item.Title, item.CoverURL, item.Description, item.TotalEpisodes, item.AddedAt)
	return true, err
}

func (s *Store) IsFavorite(seriesID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM favorites WHERE series_id = ?`, seriesID).Scan(&count)
	return err == nil && count > 0
}

func (s *Store) GetFavoriteItem(seriesID string) (FavoriteItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var it FavoriteItem
	err := s.db.QueryRow(`
		SELECT series_id, title, cover_url, description, total_episodes, added_at
		FROM favorites
		WHERE series_id = ?
	`, seriesID).Scan(&it.SeriesID, &it.Title, &it.CoverURL, &it.Description, &it.TotalEpisodes, &it.AddedAt)
	if err != nil {
		return FavoriteItem{}, false
	}
	return it, true
}

func (s *Store) GetFavorites() ([]FavoriteItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT series_id, title, cover_url, description, total_episodes, added_at
		FROM favorites
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []FavoriteItem
	for rows.Next() {
		var it FavoriteItem
		if err := rows.Scan(&it.SeriesID, &it.Title, &it.CoverURL, &it.Description, &it.TotalEpisodes, &it.AddedAt); err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) SaveFavoriteBatch(items []FavoriteItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO favorites (series_id, title, cover_url, description, total_episodes, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET
			title = excluded.title,
			cover_url = excluded.cover_url,
			description = excluded.description,
			total_episodes = excluded.total_episodes,
			added_at = excluded.added_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if item.AddedAt <= 0 {
			item.AddedAt = time.Now().Unix()
		}
		if _, err := stmt.Exec(item.SeriesID, item.Title, item.CoverURL, item.Description, item.TotalEpisodes, item.AddedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ReplaceFavoriteBatch(items []FavoriteItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM favorites`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO favorites (series_id, title, cover_url, description, total_episodes, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		if item.SeriesID == "" {
			continue
		}
		if item.AddedAt <= 0 {
			item.AddedAt = time.Now().Unix()
		}
		if _, err := stmt.Exec(
			item.SeriesID, item.Title, item.CoverURL, item.Description,
			item.TotalEpisodes, item.AddedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) QueueFavoriteOp(seriesID string, added bool, scopes ...string) error {
	if strings.TrimSpace(seriesID) == "" {
		return fmt.Errorf("待同步收藏 ID 无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	revision, err := nextSyncRevision(tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO pending_favorite_ops (series_id, added, updated_at, scope, revision)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(scope, series_id) DO UPDATE SET
			added = excluded.added,
			updated_at = excluded.updated_at, revision = excluded.revision
	`, seriesID, boolToInt(added), time.Now().Unix(), s.syncScopeLocked(scopes), revision)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) PendingFavoriteOps(limit int, scopes ...string) ([]PendingFavoriteOp, error) {
	if limit <= 0 {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT series_id, added, updated_at, scope, revision
		FROM pending_favorite_ops
		WHERE scope = ?
		ORDER BY updated_at
		LIMIT ?
	`, s.syncScopeLocked(scopes), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PendingFavoriteOp
	for rows.Next() {
		var item PendingFavoriteOp
		var added int
		if err := rows.Scan(&item.SeriesID, &added, &item.UpdatedAt, &item.Scope, &item.Revision); err != nil {
			return nil, err
		}
		item.Added = added != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeletePendingFavoriteOps(seriesIDs []string, scopes ...string) error {
	if len(seriesIDs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, seriesID := range seriesIDs {
		if _, err := tx.Exec(`DELETE FROM pending_favorite_ops WHERE series_id = ? AND scope = ?`, seriesID, s.syncScopeLocked(scopes)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ToggleLike(item LikeItem) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists int
	err := s.db.QueryRow(`
		SELECT 1 FROM likes WHERE series_id = ? AND episode_id = ?
	`, item.SeriesID, item.EpisodeID).Scan(&exists)
	if err == nil && exists == 1 {
		_, err := s.db.Exec(`
			DELETE FROM likes WHERE series_id = ? AND episode_id = ?
		`, item.SeriesID, item.EpisodeID)
		return false, err
	}

	if item.AddedAt <= 0 {
		item.AddedAt = time.Now().Unix()
	}
	_, err = s.db.Exec(`
		INSERT INTO likes (series_id, episode_id, title, cover_url, episode_num, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id, episode_id) DO UPDATE SET
			title = excluded.title,
			cover_url = excluded.cover_url,
			episode_num = excluded.episode_num,
			added_at = excluded.added_at
	`, item.SeriesID, item.EpisodeID, item.Title, item.CoverURL, item.EpisodeNum, item.AddedAt)
	return true, err
}

func (s *Store) IsLiked(seriesID, episodeID string, episodeNum int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM likes
		WHERE series_id = ? AND episode_id = ?
	`, seriesID, episodeID).Scan(&count)
	return err == nil && count > 0
}

// SetLike persists the acknowledged target state, even if a background refresh
// changed the local row while the mutation was in flight.
func (s *Store) SetLike(item LikeItem, liked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !liked {
		_, err := s.db.Exec(`DELETE FROM likes WHERE series_id = ? AND episode_id = ?`, item.SeriesID, item.EpisodeID)
		return err
	}
	if item.AddedAt <= 0 {
		item.AddedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`INSERT INTO likes (series_id, episode_id, title, cover_url, episode_num, added_at)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(series_id, episode_id) DO UPDATE SET
		title = excluded.title, cover_url = excluded.cover_url,
		episode_num = excluded.episode_num, added_at = excluded.added_at`,
		item.SeriesID, item.EpisodeID, item.Title, mediautil.NormalizeImageURL(item.CoverURL), item.EpisodeNum, item.AddedAt)
	return err
}

func (s *Store) ReplaceLikeBatch(items []LikeItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM likes`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO likes (series_id, episode_id, title, cover_url, episode_num, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if item.SeriesID == "" || item.EpisodeID == "" {
			continue
		}
		if item.AddedAt <= 0 {
			item.AddedAt = time.Now().Unix()
		}
		if _, err := stmt.Exec(
			item.SeriesID, item.EpisodeID, item.Title, item.CoverURL,
			item.EpisodeNum, item.AddedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetLikes() ([]LikeItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT series_id, episode_id, title, cover_url, episode_num, added_at
		FROM likes
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []LikeItem
	for rows.Next() {
		var it LikeItem
		if err := rows.Scan(&it.SeriesID, &it.EpisodeID, &it.Title, &it.CoverURL, &it.EpisodeNum, &it.AddedAt); err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) ToggleReservation(item ReservationItem) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM reservations WHERE series_id = ?`, item.SeriesID).Scan(&exists)
	if err == nil && exists == 1 {
		_, err := s.db.Exec(`DELETE FROM reservations WHERE series_id = ?`, item.SeriesID)
		return false, err
	}

	if item.AddedAt <= 0 {
		item.AddedAt = time.Now().Unix()
	}
	isOnline := 0
	if item.IsOnline {
		isOnline = 1
	}
	_, err = s.db.Exec(`
		INSERT INTO reservations (series_id, title, cover_url, note, publish_at, is_online, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET
			title = excluded.title,
			cover_url = excluded.cover_url,
			note = excluded.note,
			publish_at = excluded.publish_at,
			is_online = excluded.is_online,
			added_at = excluded.added_at
	`, item.SeriesID, item.Title, item.CoverURL, item.Note, item.PublishAt, isOnline, item.AddedAt)
	return true, err
}

func (s *Store) HasReservation(seriesID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists int
	return s.db.QueryRow(`SELECT 1 FROM reservations WHERE series_id = ?`, seriesID).Scan(&exists) == nil
}

func (s *Store) SaveReservationBatch(items []ReservationItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO reservations (series_id, title, cover_url, note, publish_at, is_online, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(series_id) DO UPDATE SET
			title = excluded.title,
			cover_url = excluded.cover_url,
			note = excluded.note,
			publish_at = excluded.publish_at,
			is_online = excluded.is_online,
			added_at = excluded.added_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if item.AddedAt <= 0 {
			item.AddedAt = time.Now().Unix()
		}
		isOnline := 0
		if item.IsOnline {
			isOnline = 1
		}
		if _, err := stmt.Exec(item.SeriesID, item.Title, item.CoverURL, item.Note, item.PublishAt, isOnline, item.AddedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ReplaceReservationBatch(items []ReservationItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM reservations`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO reservations (series_id, title, cover_url, note, publish_at, is_online, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	addedAt := time.Now().Unix()
	for _, item := range items {
		isOnline := 0
		if item.IsOnline {
			isOnline = 1
		}
		if _, err := stmt.Exec(item.SeriesID, item.Title, item.CoverURL, item.Note, item.PublishAt, isOnline, addedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetReservations() ([]ReservationItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT series_id, title, cover_url, note, publish_at, is_online, added_at
		FROM reservations
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReservationItem
	for rows.Next() {
		var it ReservationItem
		var isOnline int
		if err := rows.Scan(&it.SeriesID, &it.Title, &it.CoverURL, &it.Note, &it.PublishAt, &isOnline, &it.AddedAt); err != nil {
			return nil, err
		}
		it.IsOnline = isOnline == 1
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) GetProfile() UserProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var p UserProfile
	var loggedIn int
	err := s.db.QueryRow(`
		SELECT user_id, nickname, avatar, device_id, logged_in
		FROM profile WHERE id = 1
	`).Scan(&p.UserID, &p.Nickname, &p.Avatar, &p.DeviceID, &loggedIn)
	if err != nil {
		return UserProfile{
			UserID:   "anonymous",
			Nickname: "myguo用户",
			Avatar:   "",
			DeviceID: "",
			LoggedIn: false,
		}
	}
	p.LoggedIn = loggedIn == 1
	return p
}

func (s *Store) UpdateProfile(p UserProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	loggedIn := 0
	if p.LoggedIn {
		loggedIn = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO profile (id, user_id, nickname, avatar, device_id, logged_in)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			nickname = excluded.nickname,
			avatar = excluded.avatar,
			device_id = excluded.device_id,
			logged_in = excluded.logged_in
	`, p.UserID, p.Nickname, p.Avatar, p.DeviceID, loggedIn)
	return err
}

// saveLocked is maintained as a no-op for backward compatibility
func (s *Store) saveLocked() error {
	return nil
}
