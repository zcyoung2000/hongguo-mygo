package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// These definitions also migrate queues from versions with a series-only key.
// Unowned legacy intents stay local; they must never inherit a new login.
var syncQueues = []struct{ name, columns, fields, order string }{
	{"pending_history_uploads", "series_id, episode_id, title, cover_url, episode_num, episode_title, position_sec, duration_sec, total_episodes, updated_at, intro, content_type", `
		series_id TEXT NOT NULL, episode_id TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '', cover_url TEXT NOT NULL DEFAULT '',
		episode_num INTEGER NOT NULL DEFAULT 1, episode_title TEXT NOT NULL DEFAULT '',
		position_sec REAL NOT NULL DEFAULT 0, duration_sec REAL NOT NULL DEFAULT 0,
		total_episodes INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL DEFAULT 0,
		intro TEXT NOT NULL DEFAULT '', content_type INTEGER NOT NULL DEFAULT 1004`, "updated_at"},
	{"pending_history_deletes", "series_id, episode_id, episode_num, deleted_at", `
		series_id TEXT NOT NULL, episode_id TEXT NOT NULL DEFAULT '',
		episode_num INTEGER NOT NULL DEFAULT 1, deleted_at INTEGER NOT NULL DEFAULT 0`, "deleted_at"},
	{"pending_favorite_ops", "series_id, added, updated_at", `
		series_id TEXT NOT NULL, added INTEGER NOT NULL DEFAULT 1,
		updated_at INTEGER NOT NULL DEFAULT 0`, "updated_at"},
}

func (s *Store) initSyncQueues() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, queue := range syncQueues {
		var columns string
		if err := tx.QueryRow(`SELECT group_concat(name, ',') FROM pragma_table_info(?)`, queue.name).Scan(&columns); err != nil {
			return err
		}
		if strings.Contains(","+columns+",", ",scope,") {
			continue
		}
		if _, err := tx.Exec("ALTER TABLE " + queue.name + " RENAME TO " + queue.name + "_unowned"); err != nil {
			return err
		}
		if _, err := tx.Exec("CREATE TABLE " + queue.name + " (" + queue.fields + `,
			scope TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY(scope, series_id))`); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO " + queue.name + " (" + queue.columns + ") SELECT " + queue.columns + " FROM " + queue.name + "_unowned"); err != nil {
			return err
		}
		if _, err := tx.Exec("DROP TABLE " + queue.name + "_unowned"); err != nil {
			return err
		}
		if _, err := tx.Exec("CREATE INDEX idx_" + queue.name + "_" + strings.TrimSuffix(queue.order, "_at") + " ON " + queue.name + "(scope, " + queue.order + ")"); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS sync_revision_sequence (
		id INTEGER PRIMARY KEY CHECK (id = 1), revision INTEGER NOT NULL DEFAULT 0
	); INSERT OR IGNORE INTO sync_revision_sequence (id, revision) VALUES (1, 0)`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetSyncScope(scope string) {
	s.mu.Lock()
	s.syncScope = scope
	s.mu.Unlock()
}

// Caller holds s.mu. Explicit scopes let an in-flight task keep its original
// identity even if another request changes the service's active credentials.
func (s *Store) syncScopeLocked(scopes []string) string {
	if len(scopes) > 0 {
		return scopes[0]
	}
	return s.syncScope
}

func nextSyncRevision(tx *sql.Tx) (int64, error) {
	var revision int64
	err := tx.QueryRow(`UPDATE sync_revision_sequence SET revision = revision + 1 WHERE id = 1 RETURNING revision`).Scan(&revision)
	return revision, err
}

type syncIntent struct {
	scope, seriesID string
	revision        int64
}

func (s *Store) ackSyncIntents(table string, intents []syncIntent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, intent := range intents {
		if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE scope = ? AND series_id = ? AND revision = ?`, table), intent.scope, intent.seriesID, intent.revision); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AckHistoryUploads(items []PendingHistoryUpload) error {
	intents := make([]syncIntent, 0, len(items))
	for _, item := range items {
		intents = append(intents, syncIntent{item.Scope, item.SeriesID, item.Revision})
	}
	return s.ackSyncIntents("pending_history_uploads", intents)
}

func (s *Store) AckHistoryDeletes(items []PendingHistoryDelete) error {
	intents := make([]syncIntent, 0, len(items))
	for _, item := range items {
		intents = append(intents, syncIntent{item.Scope, item.SeriesID, item.Revision})
	}
	return s.ackSyncIntents("pending_history_deletes", intents)
}

func (s *Store) AckFavoriteOps(items []PendingFavoriteOp) error {
	intents := make([]syncIntent, 0, len(items))
	for _, item := range items {
		intents = append(intents, syncIntent{item.Scope, item.SeriesID, item.Revision})
	}
	return s.ackSyncIntents("pending_favorite_ops", intents)
}
