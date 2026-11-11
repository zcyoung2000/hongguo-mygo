package library

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultProgressFlushWindow      = 5 * time.Second
	defaultProgressPositionDeltaSec = 5.0
)

type progressFlushRequest struct {
	seriesID       string
	episodeID      string
	title          string
	coverURL       string
	episodeNum     int
	totalEpisodes  int
	positionSec    float64
	durationSec    float64
	receivedAt     time.Time
	updatedAfter   time.Time
	force          bool
	scheduleCancel chan struct{}
}

type progressScheduler struct {
	mu      sync.Mutex
	entries map[string]*progressFlushRequest
	now     func() time.Time
	window  time.Duration
	delta   float64
}

type progressSchedulerConfig struct {
	now    func() time.Time
	window time.Duration
	delta  float64
}

func newProgressScheduler(config progressSchedulerConfig) *progressScheduler {
	if config.now == nil {
		config.now = time.Now
	}
	if config.window <= 0 {
		config.window = configurableDuration("MYGUO_PROGRESS_FLUSH_WINDOW", defaultProgressFlushWindow)
	}
	if config.delta < 0 {
		config.delta = configurableFloat("MYGUO_PROGRESS_DELTA_SEC", defaultProgressPositionDeltaSec)
	}
	return &progressScheduler{
		entries: make(map[string]*progressFlushRequest),
		now:     config.now,
		window:  config.window,
		delta:   config.delta,
	}
}

func configurableDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func configurableFloat(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func (scheduler *progressScheduler) add(request progressFlushRequest) bool {
	if request.seriesID == "" || request.episodeID == "" {
		return false
	}
	key := progressSchedulerKey(request.seriesID, request.episodeID)
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	if existing := scheduler.entries[key]; existing != nil && !request.force {
		positionAdvanced := request.positionSec-existing.positionSec >= scheduler.delta
		if !positionAdvanced && scheduler.now().Before(existing.updatedAfter) {
			return false
		}
		existing.cancel()
	}
	request.updatedAfter = scheduler.now().Add(scheduler.window)
	scheduler.entries[key] = &request
	return true
}

func (request *progressFlushRequest) window() time.Duration {
	if request.force {
		return 0
	}
	return 5 * time.Second
}

func progressSchedulerKey(seriesID, episodeID string) string {
	return seriesID + "\x00" + episodeID
}

func (request *progressFlushRequest) cancel() {
	if request == nil || request.scheduleCancel == nil {
		return
	}
	select {
	case <-request.scheduleCancel:
	default:
		close(request.scheduleCancel)
	}
}

func (scheduler *progressScheduler) take(seriesID, episodeID string) *progressFlushRequest {
	key := progressSchedulerKey(seriesID, episodeID)
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	request := scheduler.entries[key]
	if request != nil {
		delete(scheduler.entries, key)
		request.cancel()
	}
	return request
}

func (s *Service) scheduleProgressFlush(request progressFlushRequest) {
	if !s.progressScheduler.add(request) {
		return
	}
	if request.force {
		s.flushProgressRequest(context.Background(), request)
		return
	}
	if !s.background.Go(func(ctx context.Context) {
		timer := time.NewTimer(s.progressScheduler.window)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-request.scheduleCancel:
		case <-timer.C:
			if current := s.progressScheduler.take(request.seriesID, request.episodeID); current == &request {
				s.flushProgressRequest(ctx, *current)
			}
		}
	}) {
		if current := s.progressScheduler.take(request.seriesID, request.episodeID); current == &request {
			s.flushProgressRequest(context.Background(), *current)
		}
	}
}

func (s *Service) flushProgressRequest(ctx context.Context, request progressFlushRequest) {
	if current := s.progressScheduler.take(request.seriesID, request.episodeID); current != nil {
		request = *current
	}
	if err := s.SaveProgress(ctx,
		request.seriesID,
		request.episodeID,
		request.title,
		request.coverURL,
		request.episodeNum,
		request.totalEpisodes,
		request.positionSec,
		request.durationSec,
	); err != nil {
		log.Printf("[progress] deferred save failed: %v", err)
	}
}

func (s *Service) FlushPendingProgress() {
	s.progressScheduler.mu.Lock()
	requests := make([]progressFlushRequest, 0, len(s.progressScheduler.entries))
	for key, request := range s.progressScheduler.entries {
		requests = append(requests, *request)
		request.cancel()
		delete(s.progressScheduler.entries, key)
	}
	s.progressScheduler.mu.Unlock()
	for _, request := range requests {
		if err := s.SaveProgress(context.Background(),
			request.seriesID,
			request.episodeID,
			request.title,
			request.coverURL,
			request.episodeNum,
			request.totalEpisodes,
			request.positionSec,
			request.durationSec,
		); err != nil {
			log.Printf("[progress] exit save failed: %v", err)
		}
	}
	_, _ = s.syncPendingHistoryMutations()
	s.WaitForBackgroundSync()
}
