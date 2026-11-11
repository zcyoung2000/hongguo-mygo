package store

import "myguo/internal/upstream"

func (s *Store) UpstreamStateStore() *upstream.SQLiteStateStore {
	return upstream.NewSQLiteStateStore(s.db)
}

func (s *Store) InitUpstreamState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS upstream_host_state (
			family TEXT NOT NULL,
			host TEXT NOT NULL,
			health INTEGER NOT NULL DEFAULT 0,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			last_failure_at INTEGER NOT NULL DEFAULT 0,
			last_success_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (family, host)
		)
	`); err != nil {
		return err
	}
	return nil
}
