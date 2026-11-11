package upstream

import "database/sql"

type SQLiteStateStore struct {
	db *sql.DB
}

func NewSQLiteStateStore(db *sql.DB) *SQLiteStateStore {
	return &SQLiteStateStore{db: db}
}

func (s *SQLiteStateStore) LoadHostStates() ([]HostState, error) {
	rows, err := s.db.Query(`
		SELECT family, host, health, consecutive_failures, last_failure_at, last_success_at
		FROM upstream_host_state
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var states []HostState
	for rows.Next() {
		var state HostState
		if err := rows.Scan(
			&state.Family, &state.Host, &state.Health,
			&state.ConsecutiveFailures, &state.LastFailureAt, &state.LastSuccessAt,
		); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func (s *SQLiteStateStore) SaveHostStates(states []HostState) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM upstream_host_state`); err != nil {
		return err
	}
	for _, state := range states {
		if _, err := tx.Exec(`
			INSERT INTO upstream_host_state
				(family, host, health, consecutive_failures, last_failure_at, last_success_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, unixepoch())
		`, state.Family, state.Host, state.Health, state.ConsecutiveFailures,
			state.LastFailureAt, state.LastSuccessAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}
