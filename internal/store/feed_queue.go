package store

import "encoding/json"

// FeedQueueEntry contains catalog metadata, never process-local media URLs.
// Fetching an entry is not a view: it remains pending until its first frame.
type FeedQueueEntry struct {
	SeriesID string
	Payload  json.RawMessage
}

func (s *Store) AppendFeedQueue(channel string, entries []FeedQueueEntry) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, entry := range entries {
		if entry.SeriesID == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO feed_queue (channel, series_id, payload) VALUES (?, ?, ?)
			ON CONFLICT(channel, series_id) DO NOTHING
		`, channel, entry.SeriesID, []byte(entry.Payload)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetFeedQueue(channel string) ([]FeedQueueEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT series_id, payload FROM feed_queue WHERE channel = ? ORDER BY position`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]FeedQueueEntry, 0)
	for rows.Next() {
		var entry FeedQueueEntry
		if err := rows.Scan(&entry.SeriesID, &entry.Payload); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) ConsumeFeedQueue(channel, seriesID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A forward swipe also skips earlier pending entries. A late acknowledgement
	// for an already removed entry cannot delete any newer recommendations.
	_, err := s.db.Exec(`DELETE FROM feed_queue WHERE channel = ? AND position <= (
		SELECT position FROM feed_queue WHERE channel = ? AND series_id = ?
	)`, channel, channel, seriesID)
	return err
}
