package ipc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/media"
	"myguo/internal/sparedevice"
)

func (s *HongguoService) UpdateFeedPreloadWindow(ctx context.Context, version int64, currentEpisodeID, currentQuality string, items []media.FeedPreloadItem) ([]media.FeedPreloadStatus, error) {
	return s.media.SetFeedPreloadWindow(version, currentEpisodeID, currentQuality, items), nil
}

func (s *HongguoService) PlayEpisode(ctx context.Context, seriesID, episodeID string, episodeNum int, quality string, preferHLS bool) (result *PlaybackDTO, playbackErr error) {
	started := time.Now()
	phase := "resolve-episode"
	if episodeNum <= 0 {
		episodeNum = 1
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	defer func() {
		log.Printf("[feed-playback] series=%q episode=%q phase=%s elapsedMs=%d cancelled=%t", seriesID, episodeID, phase, time.Since(started).Milliseconds(), ctx.Err() != nil)
		if playbackErr != nil {
			log.Printf("[feed-playback] series=%q phase=%s error=%q", seriesID, phase, playbackErr)
		}
	}()

	var episodePosterURL string
	var commentCount int64
	var playbackDurationSec int
	cachedStart := false
	var eps *sdk.Page[model.Episode]
	if episodeID == "" {
		// A full local cache can start instantly without a quality probe or
		// another episode-list round trip. Series details/comments still load
		// in the background on the frontend.
		candidates, err := s.store.GetCachedEpisodesAt(seriesID, normalizeVideoQuality(quality), episodeNum)
		if err != nil {
			return nil, err
		}
		for _, cached := range candidates {
			if !s.media.HasCachedEpisode(cached.EpisodeID, cached.Quality) {
				continue
			}
			episodeID = cached.EpisodeID
			episodeNum = cached.EpisodeNum
			quality = cached.Quality
			cachedStart = true
			break
		}
	}

	if episodeID == "" && !cachedStart {
		var err error
		eps, err = s.listEpisodes(ctx, seriesID)
		if err != nil {
			if errors.Is(err, sdk.ErrAnonymousLimited) {
				return nil, fmt.Errorf("分集访问受限，请稍后重试: %w", err)
			}
			return nil, fmt.Errorf("获取分集失败: %w", err)
		}
		if len(pageItems(eps)) == 0 {
			return nil, fmt.Errorf("暂无可播放分集")
		}
		targetNum := episodeNum
		found := false
		var selected model.Episode
		for _, ep := range pageItems(eps) {
			if ep.Number == targetNum {
				episodeID = ep.ID
				selected = ep
				playbackDurationSec = int(selected.Duration.Seconds())
				commentCount = ep.CommentCount
				found = true
				break
			}
		}
		if !found {
			items := pageItems(eps)
			episodeID = items[0].ID
			episodeNum = items[0].Number
			selected = items[0]
			commentCount = items[0].CommentCount
		}
		episodePosterURL = s.media.ImageURL(selected.CoverURL)
	}

	if eps == nil && s.client != nil && !cachedStart {
		pageCtx, pageCancel := context.WithTimeout(context.WithoutCancel(ctx), seriesDetailTimeout)
		defer pageCancel()
		if page, err := s.listEpisodes(pageCtx, seriesID); err == nil {
			eps = page
		}
	}
	s.media.PrefetchNextPlaybackModels(media.NextEpisodes(pageItems(eps), episodeNum))

	if episodeID == "" {
		return nil, fmt.Errorf("failed to resolve episode ID for %s", seriesID)
	}
	cachedStart = cachedStart || s.media.HasCachedEpisode(episodeID, quality)

	var qualityInfo QualitySelection
	resolvedAt := time.Now()
	log.Printf("[feed-playback] series=%q phase=episode-resolved elapsedMs=%d cached=%t", seriesID, resolvedAt.Sub(started).Milliseconds(), cachedStart)
	phase = "playback-info"
	if cachedStart {
		qualityInfo = QualitySelection{
			Options:     fallbackQualityOptions(quality),
			Selected:    quality,
			DurationSec: playbackDurationSec,
		}
	} else {
		playbackSession := sparedevice.RandomSession()
		defer s.media.ReleasePlaybackSession(playbackSession)
		info, err := s.media.PlaybackQualitiesForSession(ctx, episodeID, quality, playbackSession)
		if err != nil {
			// Playback can still succeed through MediaServer's own fallback.
			qualityInfo = QualitySelection{
				Options:     fallbackQualityOptions(normalizeVideoQuality(quality)),
				Selected:    normalizeVideoQuality(quality),
				DurationSec: playbackDurationSec,
			}
		} else {
			qualityInfo = info
		}
	}
	_ = s.store.RememberEpisode(seriesID, episodeID, episodeNum, qualityInfo.Selected)
	log.Printf("[feed-playback] series=%q phase=quality-resolved elapsedMs=%d phaseMs=%d", seriesID, time.Since(started).Milliseconds(), time.Since(resolvedAt).Milliseconds())
	phase = "local-state"
	isFullCached := fileExists(episodeCachePath(s.media.CacheDir(), episodeID, qualityInfo.Selected))
	fullStreamURL := s.media.VideoURL(episodeID, seriesID, episodeNum, qualityInfo.Selected)
	streamURL := fullStreamURL
	progressiveStreamURL := ""
	if !isFullCached {
		streamURL = s.media.ProgressiveVideoURL(episodeID, seriesID, episodeNum, qualityInfo.Selected)
		progressiveStreamURL = streamURL
	}
	hlsStreamURL := ""
	isHLSCached := s.media.HasCachedHLS(episodeID, qualityInfo.Selected)
	if !isFullCached {
		// Negotiation must not start a media job: a late IPC response could
		// otherwise restart a preload after the scheduler withdrew admission.
		// Return the alternative even for metadata-only lookahead so admission
		// can select HLS without repeating SDK negotiation or another IPC call.
		hlsStreamURL = s.media.HLSPlaylistURL(episodeID, seriesID, episodeNum, qualityInfo.Selected)
		if preferHLS || isHLSCached {
			streamURL = hlsStreamURL
		}
	}

	hist, err := s.store.GetHistory()
	if err != nil {
		return nil, err
	}
	var lastPos float64
	for _, h := range hist {
		if h.SeriesID == seriesID && h.EpisodeNum == episodeNum {
			lastPos = h.PositionSec
			break
		}
	}

	isLiked := s.store.IsLiked(seriesID, episodeID, episodeNum)
	phase = "complete"

	return sanitizeIPC(&PlaybackDTO{
		SeriesID:             seriesID,
		EpisodeID:            episodeID,
		EpisodeNum:           episodeNum,
		Title:                fmt.Sprintf("第%d集", episodeNum),
		StreamURL:            streamURL,
		FullStreamURL:        fullStreamURL,
		ProgressiveStreamURL: progressiveStreamURL,
		HLSStreamURL:         hlsStreamURL,
		PosterURL:            episodePosterURL,
		DurationSec:          firstPositiveInt(qualityInfo.DurationSec, playbackDurationSec),
		LastPosition:         lastPos,
		IsLiked:              isLiked,
		CommentCount:         commentCount,
		Quality:              qualityInfo.Selected,
		Qualities:            qualityInfo.Options,
		IsFullCached:         isFullCached,
		IsHLSCached:          isHLSCached,
	}), nil
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeNetworkProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "weak", "medium", "strong":
		return strings.ToLower(strings.TrimSpace(profile))
	default:
		return "unknown"
	}
}

func truncateString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func warmupWindowLimits(profile string) (next, previous int) {
	switch normalizeNetworkProfile(profile) {
	case "weak":
		// Preserve the immediate next progressive stream, but defer all full-file
		// work until playback proves that the connection can sustain it.
		return 1, 0
	case "medium":
		return 2, 1
	case "strong":
		return 3, 2
	default:
		return 2, 1
	}
}

func fallbackQualityOptions(selected string) []QualityOptionDTO {
	selected = normalizeVideoQuality(selected)
	options := make([]QualityOptionDTO, 0, len(videoQualities()))
	for _, quality := range videoQualities() {
		options = append(options, QualityOptionDTO{
			ID:        quality,
			Label:     qualityLabel(quality),
			Available: true,
			Active:    quality == selected,
		})
	}
	return options
}

// WarmupFeed performs bidirectional background preheating for adjacent dramas in the recommend feed.
func (s *HongguoService) WarmupFeed(ctx context.Context, centerSeriesID, centerEpisodeID string, prevSeriesIDs []string, nextSeriesIDs []string, quality, networkProfile string) error {
	quality = normalizeVideoQuality(quality)
	networkProfile = normalizeNetworkProfile(networkProfile)
	s.feedWarmupMu.Lock()
	s.feedWarmupGeneration++
	warmupGeneration := s.feedWarmupGeneration
	s.feedWarmupMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		type warmupItem struct {
			id           string
			priority     int
			deferred     bool
			prefetchOnly bool
		}
		items := make([]warmupItem, 0, 6)
		seen := make(map[string]bool, 6)
		appendSeries := func(id string, priority int, deferred, prefetchOnly bool) {
			if id != "" && !seen[id] {
				seen[id] = true
				items = append(items, warmupItem{id: id, priority: priority, deferred: deferred, prefetchOnly: prefetchOnly})
			}
		}
		nextLimit, previousLimit := warmupWindowLimits(networkProfile)
		appendSeries(centerSeriesID, 0, false, false)
		for i, id := range nextSeriesIDs {
			if i >= nextLimit {
				break
			}
			// The immediate next may fall back to a full file if its renderer
			// stream cannot start. Farther items are HLS-prefetch only so they
			// never compete with the handoff path.
			appendSeries(id, i+1, true, i > 0)
		}
		for i, id := range prevSeriesIDs {
			if i >= previousLimit {
				break
			}
			appendSeries(id, len(nextSeriesIDs)+i+1, true, true)
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].priority < items[j].priority })

		var tasksMu sync.Mutex
		tasks := make([]PrefetchTask, 0, len(items))
		publish := func(task PrefetchTask) {
			tasksMu.Lock()
			tasks = append(tasks, task)
			snapshot := make([]PrefetchTask, len(tasks))
			copy(snapshot, tasks)
			tasksMu.Unlock()

			// Serialize snapshots through the warmup lock. Otherwise a stale,
			// smaller snapshot could replace the window after a larger one and
			// cancel work that was just admitted.
			s.feedWarmupMu.Lock()
			defer s.feedWarmupMu.Unlock()
			if warmupGeneration != s.feedWarmupGeneration {
				return
			}
			s.media.ReplacePrefetchWindow(snapshot)
		}
		resolveItem := func(item warmupItem) {
			seriesID := item.id
			episodes, err := s.listEpisodes(ctx, seriesID)
			if err != nil || len(pageItems(episodes)) == 0 {
				return
			}
			episode := pageItems(episodes)[0]
			_ = s.store.RememberEpisode(seriesID, episode.ID, episode.Number, quality)
			publish(PrefetchTask{
				SeriesID:     seriesID,
				EpisodeID:    episode.ID,
				Number:       episode.Number,
				Quality:      quality,
				ProtectOnly:  seriesID == centerSeriesID,
				Deferred:     item.deferred,
				PrefetchOnly: item.prefetchOnly,
				Priority:     item.priority,
			})
		}

		// The renderer supplies the playing episode ID. Publishing it first gives
		// MediaServer an immediate protection key and cancels every stale feed
		// download without waiting for adjacent-series metadata requests.
		if centerEpisodeID != "" {
			publish(PrefetchTask{
				SeriesID:    centerSeriesID,
				EpisodeID:   centerEpisodeID,
				Number:      1,
				Quality:     quality,
				ProtectOnly: true,
			})
		}

		var workers sync.WaitGroup
		for _, item := range items {
			if item.id == centerSeriesID && centerEpisodeID != "" {
				continue
			}
			workers.Add(1)
			go func(item warmupItem) {
				defer workers.Done()
				resolveItem(item)
			}(item)
		}
		workers.Wait()
	}()

	return nil
}

func (s *HongguoService) GetMediaDiagnostics(ctx context.Context) (MediaDiagnosticsDTO, error) {
	return s.media.MediaDiagnostics(), nil
}

func (s *HongguoService) RecordFeedMetrics(ctx context.Context, metrics []FeedMetricDTO) error {
	if len(metrics) == 0 {
		return nil
	}
	if len(metrics) > 32 {
		metrics = metrics[len(metrics)-32:]
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range metrics {
		metric := &metrics[i]
		metric.Type = truncateString(metric.Type, 32)
		metric.Event = truncateString(metric.Event, 40)
		metric.Reason = truncateString(metric.Reason, 64)
		metric.SessionID = truncateString(metric.SessionID, 48)
		metric.SeriesID = truncateString(metric.SeriesID, 64)
		metric.EpisodeID = truncateString(metric.EpisodeID, 64)
		metric.InputSource = truncateString(metric.InputSource, 24)
		metric.NetworkProfile = truncateString(metric.NetworkProfile, 16)
		metric.StreamMode = truncateString(metric.StreamMode, 24)
		metric.Quality = truncateString(metric.Quality, 16)
		metric.FirstFrameSource = truncateString(metric.FirstFrameSource, 24)
		if metric.At == "" {
			metric.At = now
		} else {
			metric.At = truncateString(metric.At, 48)
		}
	}
	return s.store.AppendFeedMetrics(metrics)
}
