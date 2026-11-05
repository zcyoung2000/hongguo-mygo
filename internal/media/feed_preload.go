package media

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FeedPreloadItem struct {
	SeriesID  string `json:"seriesId"`
	EpisodeID string `json:"episodeId"`
	Number    int    `json:"number"`
	Quality   string `json:"quality"`
}

type FeedPreloadStatus struct {
	EpisodeID     string  `json:"episodeId"`
	Quality       string  `json:"quality"`
	CachedSeconds float64 `json:"cachedSeconds"`
	Complete      bool    `json:"complete"`
	Active        bool    `json:"active"`
}

func (ms *MediaServer) stopFeedPreload(key string) {
	ms.hlsMu.Lock()
	if stream := ms.hlsStreams[key]; stream != nil {
		select {
		case <-stream.done:
		default:
			log.Printf("[feed-window] cancel transport=hls key=%q reason=window-replaced", key)
		}
		ms.retireHLSStreamLocked(key, stream)
	}
	ms.hlsMu.Unlock()
	ms.progressiveMu.Lock()
	if stream := ms.progressiveStreams[key]; stream != nil {
		log.Printf("[feed-window] cancel transport=progressive key=%q reason=window-replaced", key)
		stream.cancel()
	}
	ms.progressiveMu.Unlock()
}

func (ms *MediaServer) feedPreloadStatus(task PrefetchTask) FeedPreloadStatus {
	status := FeedPreloadStatus{EpisodeID: task.EpisodeID, Quality: task.Quality}
	if fileExists(episodeCachePath(ms.cacheDir, task.EpisodeID, task.Quality)) {
		status.Complete = true
		return status
	}
	ms.hlsMu.Lock()
	stream := ms.hlsStreams[PrefetchTaskKey(task)]
	if stream != nil && stream.retired == nil {
		select {
		case <-stream.done:
		default:
			status.Active = true
		}
	}
	ms.hlsMu.Unlock()
	playlist := filepath.Join(ms.hlsCacheDir(task.EpisodeID, task.Quality), "index.m3u8")
	if !hlsPlaylistReady(playlist) {
		return status
	}
	data, err := os.ReadFile(playlist)
	if err != nil {
		return status
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		seconds, _ := strconv.ParseFloat(strings.SplitN(strings.TrimPrefix(line, "#EXTINF:"), ",", 2)[0], 64)
		if seconds > 0 {
			status.CachedSeconds += seconds
		}
	}
	status.Complete = strings.Contains(string(data), "#EXT-X-ENDLIST")
	return status
}

// SetFeedPreloadWindow is an admission controller, not a second full-file
// downloader. It reuses the exact HLS rendition later opened by the video pool.
// Frontend revisions prevent an old swipe/slow IPC reply restoring stale work.
func (ms *MediaServer) SetFeedPreloadWindow(version int64, currentEpisodeID, currentQuality string, items []FeedPreloadItem) []FeedPreloadStatus {
	_, done, err := ms.tasks.Acquire()
	if err != nil {
		return []FeedPreloadStatus{}
	}
	defer done()
	ms.feedPreloadMu.Lock()
	defer ms.feedPreloadMu.Unlock()
	if version <= ms.feedPreloadVersion {
		return []FeedPreloadStatus{}
	}
	ms.feedPreloadVersion = version
	currentKey := ""
	if validContentID(currentEpisodeID) {
		currentKey = PrefetchTaskKey(PrefetchTask{EpisodeID: currentEpisodeID, Quality: currentQuality})
	}
	tasks := make([]PrefetchTask, 0, 2)
	desired := make(map[string]PrefetchTask, 2)
	for _, item := range items {
		if !validContentID(item.EpisodeID) || len(tasks) >= 2 {
			continue
		}
		task := normalizePrefetchTask(PrefetchTask{SeriesID: item.SeriesID, EpisodeID: item.EpisodeID, Number: item.Number, Quality: item.Quality})
		key := PrefetchTaskKey(task)
		if _, duplicate := desired[key]; duplicate || key == currentKey {
			continue
		}
		desired[key] = task
		tasks = append(tasks, task)
	}

	// Explicit speculative ownership lets backpressure cancel a background
	// video even when its offscreen decoder recently requested an HLS segment.
	// The protected current key is never canceled, including on promotion.
	for key := range ms.feedPreloadTasks {
		if _, keep := desired[key]; !keep && key != currentKey {
			ms.stopFeedPreload(key)
		}
	}
	if old := ms.feedPreloadCurrent; old != "" && old != currentKey {
		if _, keep := desired[old]; !keep {
			ms.stopFeedPreload(old)
		}
	}
	ms.feedPreloadCurrent = currentKey
	ms.feedPreloadTasks = desired
	if ms.feedPreloadTries == nil {
		ms.feedPreloadTries = make(map[string]time.Time)
	}
	for key := range ms.feedPreloadTries {
		if _, keep := desired[key]; !keep {
			delete(ms.feedPreloadTries, key)
		}
	}

	statuses := make([]FeedPreloadStatus, len(tasks))
	for i, task := range tasks {
		statuses[i] = ms.feedPreloadStatus(task)
	}
	// A reverse swipe can move an active download to second place. Give the
	// newly nearest episode priority rather than keeping both writers busy.
	if len(tasks) == 2 && !statuses[0].Complete && statuses[1].Active {
		ms.stopFeedPreload(PrefetchTaskKey(tasks[1]))
		statuses[1].Active = false
	}
	for i, task := range tasks {
		// Complete the nearest episode before spending bandwidth farther ahead.
		// Only complete renditions survive a restart, so a collection of short
		// prefixes cannot provide a useful local startup queue.
		if i > 0 && !statuses[i-1].Complete {
			break
		}
		if statuses[i].Complete || statuses[i].Active {
			continue
		}
		key := PrefetchTaskKey(task)
		if last := ms.feedPreloadTries[key]; !last.IsZero() && time.Since(last) < 5*time.Second {
			break
		}
		ms.feedPreloadTries[key] = time.Now()
		if _, err := ms.StartHLSStream(task.SeriesID, task.EpisodeID, task.Number, task.Quality); err != nil {
			break
		}
		statuses[i] = ms.feedPreloadStatus(task)
	}
	return statuses
}
