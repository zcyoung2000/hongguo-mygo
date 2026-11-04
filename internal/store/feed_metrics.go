package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const feedMetricRetentionRows = 5000

type FeedMetric struct {
	Type                string `json:"type"`
	FromIndex           int    `json:"fromIndex,omitempty"`
	ToIndex             int    `json:"toIndex,omitempty"`
	BufferedAtSwitch    bool   `json:"bufferedAtSwitch"`
	DurationMS          int64  `json:"durationMs"`
	TimeToFirstFrameMS  int64  `json:"timeToFirstFrameMs,omitempty"`
	PlaybackInfoMS      int64  `json:"playbackInfoMs,omitempty"`
	MediaStartupMS      int64  `json:"mediaStartupMs,omitempty"`
	HandoffFirstFrameMS int64  `json:"handoffFirstFrameMs,omitempty"`
	FirstFrameSource    string `json:"firstFrameSource,omitempty"`
	SwitchReadyMS       int64  `json:"switchReadyMs,omitempty"`
	RebufferCount       int    `json:"rebufferCount,omitempty"`
	RebufferMS          int64  `json:"rebufferMs,omitempty"`
	NetworkProfile      string `json:"networkProfile,omitempty"`
	NetworkBytesPerSec  int64  `json:"networkBytesPerSec,omitempty"`
	NetworkFailures     int64  `json:"networkFailures,omitempty"`
	PrefetchCompleted   int64  `json:"prefetchCompleted,omitempty"`
	PrefetchFailed      int64  `json:"prefetchFailed,omitempty"`
	PrefetchCanceled    int64  `json:"prefetchCanceled,omitempty"`
	PrefetchDropped     int64  `json:"prefetchDropped,omitempty"`
	HLSStarts           int64  `json:"hlsStarts,omitempty"`
	HLSHits             int64  `json:"hlsHits,omitempty"`
	StreamMode          string `json:"streamMode,omitempty"`
	CacheHit            bool   `json:"cacheHit"`
	FullFallback        bool   `json:"fullFallback"`
	Quality             string `json:"quality,omitempty"`
	At                  string `json:"at"`
}

const feedMetricColumns = `
	type, from_index, to_index, buffered_at_switch, duration_ms,
	time_to_first_frame_ms, handoff_first_frame_ms, first_frame_source,
	playback_info_ms, media_startup_ms,
	switch_ready_ms, rebuffer_count, rebuffer_ms, network_profile,
	network_bytes_per_sec, network_failures, prefetch_completed,
	prefetch_failed, prefetch_canceled, prefetch_dropped, hls_starts,
	hls_hits, stream_mode, cache_hit, full_fallback, quality, at
`

func feedMetricValues(metric FeedMetric) []any {
	boolInt := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	return []any{
		metric.Type, metric.FromIndex, metric.ToIndex, boolInt(metric.BufferedAtSwitch), metric.DurationMS,
		metric.TimeToFirstFrameMS, metric.PlaybackInfoMS, metric.MediaStartupMS,
		metric.HandoffFirstFrameMS, metric.FirstFrameSource,
		metric.SwitchReadyMS, metric.RebufferCount, metric.RebufferMS, metric.NetworkProfile,
		metric.NetworkBytesPerSec, metric.NetworkFailures, metric.PrefetchCompleted,
		metric.PrefetchFailed, metric.PrefetchCanceled, metric.PrefetchDropped, metric.HLSStarts,
		metric.HLSHits, metric.StreamMode, boolInt(metric.CacheHit), boolInt(metric.FullFallback), metric.Quality, metric.At,
	}
}

func (s *Store) AppendFeedMetrics(metrics []FeedMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := "INSERT INTO feed_metrics (" + feedMetricColumns + ") VALUES (" +
		strings.TrimSuffix(strings.Repeat("?,", 27), ",") + ")"
	statement, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, metric := range metrics {
		if metric.At == "" {
			metric.At = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if _, err := statement.Exec(feedMetricValues(metric)...); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		DELETE FROM feed_metrics WHERE id NOT IN (
			SELECT id FROM feed_metrics ORDER BY id DESC LIMIT ?
		)
	`, feedMetricRetentionRows); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) FeedMetricCount() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM feed_metrics`).Scan(&count)
	return count, err
}

func (s *Store) migrateFeedMetricColumns() error {
	var columns string
	if err := s.db.QueryRow(`SELECT group_concat(name, ',') FROM pragma_table_info('feed_metrics')`).Scan(&columns); err != nil {
		return err
	}
	for _, column := range []string{"playback_info_ms", "media_startup_ms"} {
		if !strings.Contains(","+columns+",", ","+column+",") {
			if _, err := s.db.Exec("ALTER TABLE feed_metrics ADD COLUMN " + column + " INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateFeedMetricsJSONL is a one-time import from the former JSONL
// diagnostics file. New samples are written only to SQLite.
func (s *Store) migrateFeedMetricsJSONL() error {
	var allMetrics []FeedMetric
	for _, name := range []string{"feed-metrics.jsonl", "feed-metrics.jsonl.1"} {
		path := filepath.Join(s.dir(), name)
		file, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var metrics []FeedMetric
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var metric FeedMetric
			if err := json.Unmarshal(line, &metric); err != nil {
				log.Printf("[metrics] skipped invalid legacy JSONL row: %v", err)
				continue
			}
			metrics = append(metrics, metric)
		}
		closeErr := file.Close()
		if err := scanner.Err(); err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		allMetrics = append(allMetrics, metrics...)
	}
	if len(allMetrics) > 0 {
		if err := s.AppendFeedMetrics(allMetrics); err != nil {
			return err
		}
	}
	for _, name := range []string{"feed-metrics.jsonl", "feed-metrics.jsonl.1"} {
		path := filepath.Join(s.dir(), name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
