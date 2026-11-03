package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/appdirs"
	"myguo/internal/mediautil"
)

const (
	DefaultVideoQuality     = "720p"
	MaxEpisodeCacheBytes    = int64(2 * 1024 * 1024 * 1024) // 2 GB limit for episodes
	TargetEpisodeCacheBytes = int64(1536 * 1024 * 1024)     // 1.5 GB target after eviction
	MaxImageCacheBytes      = int64(500 * 1024 * 1024)      // 500 MB limit for images
	TargetImageCacheBytes   = int64(350 * 1024 * 1024)      // 350 MB target after eviction
	MaxHLSCacheBytes        = int64(1024 * 1024 * 1024)     // 1 GB limit for local HLS renditions
	TargetHLSCacheBytes     = int64(768 * 1024 * 1024)      // 768 MB target after eviction
	MinFreeCacheDiskBytes   = uint64(512 * 1024 * 1024)     // Reserve space for the OS and in-progress MP4 writes
)

var videoQualities = []string{"1080p", "720p", "540p", "480p", "360p"}

type MediaDiagnosticsDTO struct {
	NetworkProfile     string `json:"networkProfile"`
	NetworkBytesPerSec int64  `json:"networkBytesPerSec"`
	NetworkFailures    int64  `json:"networkFailures"`
	PlaybackCacheHits  int64  `json:"playbackCacheHits"`
	PlaybackCacheMiss  int64  `json:"playbackCacheMiss"`
	ProgressiveStarts  int64  `json:"progressiveStarts"`
	ProgressiveHits    int64  `json:"progressiveHits"`
	HLSStarts          int64  `json:"hlsStarts"`
	HLSHits            int64  `json:"hlsHits"`
	FullDownloads      int64  `json:"fullDownloads"`
	PrefetchCompleted  int64  `json:"prefetchCompleted"`
	PrefetchFailed     int64  `json:"prefetchFailed"`
	PrefetchCanceled   int64  `json:"prefetchCanceled"`
	PrefetchDropped    int64  `json:"prefetchDropped"`
}

type QualityOptionDTO struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Codec     string `json:"codec,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Bitrate   int64  `json:"bitrate,omitempty"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
}

type QualitySelection struct {
	Options     []QualityOptionDTO `json:"options"`
	Selected    string             `json:"selected"`
	DurationSec int                `json:"-"`
}

type MediaServer struct {
	client             *sdk.Client
	cacheDir           string
	port               int
	server             *http.Server
	lockMap            sync.Map // episodeID -> *sync.Mutex
	imgLocks           sync.Map // hexHash -> *sync.Mutex
	pbCacheMu          sync.RWMutex
	pbCache            map[string]playbackCacheItem
	pbLoadMu           sync.Mutex
	pbLoads            map[string]*playbackLoad
	prefetchQ          chan PrefetchTask
	prefetchMu         sync.Mutex
	prefetchQueued     map[string]PrefetchTask
	prefetchActive     map[string]*PrefetchExecution
	progressiveMu      sync.Mutex
	progressiveStreams map[string]*progressiveStream
	hlsMu              sync.Mutex
	hlsStreams         map[string]*hlsStream
	cachePinMu         sync.RWMutex
	cachePins          map[string]int
	onEpisodeCacheFn   atomic.Pointer[func(seriesID, episodeID string, num int, quality string) error]
	onDeviceResetFn    atomic.Pointer[func(ctx context.Context) error]
	evicting           atomic.Bool
	lastEvictionAt     atomic.Int64
	networkMu          sync.Mutex
	networkSampleBytes int64
	networkSampleAt    time.Time
	networkBytesPerSec float64
	networkSamples     uint64
	networkErrors      uint64
	networkLastErrorAt time.Time
	playbackCacheHits  atomic.Int64
	playbackCacheMiss  atomic.Int64
	progressiveStarts  atomic.Int64
	progressiveHits    atomic.Int64
	hlsStarts          atomic.Int64
	hlsHits            atomic.Int64
	fullDownloads      atomic.Int64
	prefetchCompleted  atomic.Int64
	prefetchFailed     atomic.Int64
	prefetchCanceled   atomic.Int64
	prefetchDropped    atomic.Int64
}

type playbackCacheItem struct {
	pb        *model.Playback
	expiresAt time.Time
}

type playbackLoad struct {
	done   chan struct{}
	result playbackLoadResult
}

type playbackLoadResult struct {
	pb  *model.Playback
	err error
}

type PrefetchExecution struct {
	cancel context.CancelFunc
}

func (ms *MediaServer) CacheDir() string { return ms.cacheDir }

func VideoQualities() []string { return append([]string(nil), videoQualities...) }

func QualityLabel(quality string) string { return qualityLabel(quality) }

type rendererStreamCandidate struct {
	key       string
	startedAt time.Time
}

// Package-bundled GUI processes often receive a minimal PATH, while Homebrew
// installs ffmpeg outside that default search path.
var ffmpegCommand = sync.OnceValues(func() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("HONGGUO_FFMPEG")); custom != "" {
		if path, err := exec.LookPath(custom); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("HONGGUO_FFMPEG is not executable: %s", custom)
	}

	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		bundleDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(bundleDir, "ffmpeg"),
			filepath.Join(filepath.Dir(bundleDir), "Resources", "ffmpeg"),
		)
	}
	candidates = append(candidates,
		"/opt/homebrew/bin/ffmpeg",
		"/usr/local/bin/ffmpeg",
		"/usr/bin/ffmpeg",
		"/bin/ffmpeg",
	)
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", "ffmpeg"),
			filepath.Join(home, "homebrew", "bin", "ffmpeg"),
		)
	}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}
	return "", errors.New("ffmpeg executable not found; install ffmpeg or set HONGGUO_FFMPEG")
})

type PrefetchTask struct {
	SeriesID  string
	EpisodeID string
	Number    int
	Quality   string
	// protectOnly keeps an active progressive stream in the feed window without
	// starting a duplicate full-file download for the episode being played.
	ProtectOnly bool
	// deferred lets the feed's immediate-next task wait briefly for the video
	// element to open its progressive stream before falling back to a full cache.
	Deferred bool
	// prefetchOnly is used for i+2/i+3: their HLS jobs are useful, but a
	// duplicate full-file download would compete with the immediate next video.
	PrefetchOnly bool
	// Lower values are dispatched first. The feed must never let i+2 compete
	// with i+1 for upstream bandwidth.
	Priority int
}

func normalizeVideoQuality(quality string) string {
	return mediautil.NormalizeVideoQuality(quality)
}

func FFmpegCommand() (string, error) { return ffmpegCommand() }

func episodeCachePath(cacheDir, episodeID, quality string) string {
	return EpisodeCachePath(cacheDir, episodeID, quality)
}

func qualityLabel(quality string) string {
	return strings.ToUpper(quality)
}

func EpisodeCachePath(cacheDir, episodeID, quality string) string {
	if !validContentID(episodeID) {
		return ""
	}
	name := episodeID
	if quality != DefaultVideoQuality {
		name += "_" + quality
	}
	// v2 muxes a separate DASH audio track into the MP4. The suffix also
	// prevents playback from reusing files written by the old video-only path.
	name += "_v2"
	return filepath.Join(cacheDir, "episodes", name+".mp4")
}

func definitionNumber(definition string) int {
	number := 0
	for _, char := range definition {
		if char < '0' || char > '9' {
			break
		}
		number = number*10 + int(char-'0')
	}
	return number
}

func streamQualityRank(stream model.Stream) int {
	if number := definitionNumber(stream.Definition); number > 0 {
		return number
	}
	if stream.Height > 0 {
		return stream.Height
	}
	return stream.Width
}

func nearestQuality(requested string, candidates []string) string {
	if len(candidates) == 0 {
		return normalizeVideoQuality(requested)
	}

	requestedRank := definitionNumber(normalizeVideoQuality(requested))
	selected := candidates[0]
	selectedGap := abs(definitionNumber(selected) - requestedRank)
	for _, candidate := range candidates[1:] {
		gap := abs(definitionNumber(candidate) - requestedRank)
		// Prefer the higher quality when two candidates are equally distant.
		if gap < selectedGap || (gap == selectedGap && definitionNumber(candidate) > definitionNumber(selected)) {
			selected = candidate
			selectedGap = gap
		}
	}
	return selected
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func qualitySelectionFromPlayback(pb *model.Playback, requested string) QualitySelection {
	requested = normalizeVideoQuality(requested)
	available := make(map[string]bool)
	streamDetails := make(map[string]model.Stream)
	if pb != nil {
		for _, stream := range pb.Streams {
			if strings.TrimSpace(stream.URL) == "" {
				continue
			}
			rank := streamQualityRank(stream)
			if rank <= 0 {
				continue
			}
			mappedQuality := qualityForStreamRank(rank)
			available[mappedQuality] = true
			if _, exists := streamDetails[mappedQuality]; !exists || stream.Bitrate > streamDetails[mappedQuality].Bitrate {
				streamDetails[mappedQuality] = stream
			}
		}
	}

	candidates := make([]string, 0, len(available))
	for _, quality := range videoQualities {
		if available[quality] {
			candidates = append(candidates, quality)
		}
	}
	selected := nearestQuality(requested, candidates)

	options := make([]QualityOptionDTO, 0, len(videoQualities))
	for _, quality := range videoQualities {
		option := QualityOptionDTO{
			ID:        quality,
			Label:     qualityLabel(quality),
			Available: available[quality],
			Active:    quality == selected,
		}
		if stream, ok := streamDetails[quality]; ok {
			option.Codec = stream.Codec
			option.Width = stream.Width
			option.Height = stream.Height
			option.Bitrate = stream.Bitrate
		}
		options = append(options, option)
	}
	durationSec := 0
	if pb != nil && pb.DurationSeconds > 0 {
		durationSec = pb.DurationSeconds
	}
	return QualitySelection{Options: options, Selected: selected, DurationSec: durationSec}
}

func qualityForStreamRank(rank int) string {
	selected := DefaultVideoQuality
	selectedGap := abs(rank - definitionNumber(selected))
	for _, quality := range videoQualities {
		gap := abs(rank - definitionNumber(quality))
		if gap < selectedGap || (gap == selectedGap && definitionNumber(quality) > definitionNumber(selected)) {
			selected = quality
			selectedGap = gap
		}
	}
	return selected
}

func selectVideoStream(pb *model.Playback, requested string) (model.Stream, error) {
	if pb == nil {
		return model.Stream{}, fmt.Errorf("playback is unavailable")
	}
	quality := normalizeVideoQuality(requested)
	stream, err := pb.Select(quality)
	if err == nil {
		return stream, nil
	}

	type rankedStream struct {
		rank   int
		stream model.Stream
	}
	candidates := make([]rankedStream, 0, len(pb.Streams))
	for _, candidate := range pb.Streams {
		if strings.TrimSpace(candidate.URL) == "" {
			continue
		}
		candidates = append(candidates, rankedStream{
			rank:   streamQualityRank(candidate),
			stream: candidate,
		})
	}
	if len(candidates) == 0 {
		return model.Stream{}, err
	}

	requestedRank := definitionNumber(quality)
	selected := candidates[0]
	selectedGap := abs(selected.rank - requestedRank)
	for _, candidate := range candidates[1:] {
		gap := abs(candidate.rank - requestedRank)
		if gap < selectedGap || (gap == selectedGap && candidate.rank > selected.rank) {
			selected = candidate
			selectedGap = gap
		}
	}
	return selected.stream, nil
}

func (ms *MediaServer) resolvePlayback(ctx context.Context, episodeID string) (*model.Playback, error) {
	if episodeID == "" {
		return nil, fmt.Errorf("empty episode id")
	}

	ms.pbCacheMu.RLock()
	if item, ok := ms.pbCache[episodeID]; ok && time.Now().Before(item.expiresAt) {
		ms.pbCacheMu.RUnlock()
		ms.playbackCacheHits.Add(1)
		return item.pb, nil
	}
	ms.pbCacheMu.RUnlock()
	ms.playbackCacheMiss.Add(1)

	ms.pbLoadMu.Lock()
	if ms.pbLoads == nil {
		ms.pbLoads = make(map[string]*playbackLoad)
	}
	if load, active := ms.pbLoads[episodeID]; active {
		ms.pbLoadMu.Unlock()
		select {
		case <-load.done:
			return load.result.pb, load.result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	// A request can fill the cache between our first lookup and this lock.
	ms.pbCacheMu.RLock()
	item, cached := ms.pbCache[episodeID]
	ms.pbCacheMu.RUnlock()
	if cached && time.Now().Before(item.expiresAt) {
		ms.pbLoadMu.Unlock()
		return item.pb, nil
	}
	load := &playbackLoad{done: make(chan struct{})}
	ms.pbLoads[episodeID] = load
	ms.pbLoadMu.Unlock()

	pb, err := ms.client.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
	if errors.Is(err, sdk.ErrAnonymousLimited) {
		if resetErr := ms.resetDevice(ctx); resetErr != nil {
			log.Printf("[media] device reset failed: %v", resetErr)
		}
		ms.clearPlaybackCache()
		pb, err = ms.client.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
	}
	if errors.Is(err, sdk.ErrDRMProtected) && pb != nil {
		err = nil
	}
	if pb == nil {
		if err == nil {
			err = fmt.Errorf("empty playback model")
		}
		pb = nil
	}

	if err == nil && pb != nil {
		ms.pbCacheMu.Lock()
		ms.rememberPlaybackLocked(episodeID, pb)
		ms.pbCacheMu.Unlock()
	}

	ms.pbLoadMu.Lock()
	load.result = playbackLoadResult{pb: pb, err: err}
	close(load.done)
	delete(ms.pbLoads, episodeID)
	ms.pbLoadMu.Unlock()

	return pb, err
}

func (ms *MediaServer) clearPlaybackCache() {
	ms.pbCacheMu.Lock()
	clear(ms.pbCache)
	ms.pbCacheMu.Unlock()
}

func (ms *MediaServer) ClearPlaybackCache() {
	ms.clearPlaybackCache()
}

func (ms *MediaServer) rememberPlaybackLocked(episodeID string, pb *model.Playback) {
	if ms.pbCache == nil {
		ms.pbCache = make(map[string]playbackCacheItem)
	}
	if len(ms.pbCache) >= 100 {
		oldestID := ""
		var oldestExpiry time.Time
		for id, item := range ms.pbCache {
			if oldestID == "" || item.expiresAt.Before(oldestExpiry) {
				oldestID = id
				oldestExpiry = item.expiresAt
			}
		}
		delete(ms.pbCache, oldestID)
	}
	ms.pbCache[episodeID] = playbackCacheItem{
		pb:        pb,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
}

func (ms *MediaServer) PlaybackQualities(ctx context.Context, episodeID, requested string) (QualitySelection, error) {
	pb, err := ms.resolvePlayback(ctx, episodeID)
	if err != nil {
		return QualitySelection{}, err
	}
	if pb == nil {
		return QualitySelection{}, fmt.Errorf("no playback available for episode %s", episodeID)
	}
	if _, err := selectVideoStream(pb, requested); err != nil {
		return QualitySelection{}, err
	}
	return qualitySelectionFromPlayback(pb, requested), nil
}

func (ms *MediaServer) lockFor(episodeID, quality string) *sync.Mutex {
	actualLock, _ := ms.lockMap.LoadOrStore(episodeID+"\x00"+normalizeVideoQuality(quality), &sync.Mutex{})
	return actualLock.(*sync.Mutex)
}

func (ms *MediaServer) observeMediaBytes(size int) {
	if size <= 0 {
		return
	}

	now := time.Now()
	ms.networkMu.Lock()
	defer ms.networkMu.Unlock()
	if ms.networkSampleAt.IsZero() {
		ms.networkSampleAt = now
		ms.networkSampleBytes = int64(size)
		return
	}

	ms.networkSampleBytes += int64(size)
	elapsed := now.Sub(ms.networkSampleAt)
	if elapsed < 250*time.Millisecond {
		return
	}
	instant := float64(ms.networkSampleBytes) / elapsed.Seconds()
	if ms.networkBytesPerSec == 0 {
		ms.networkBytesPerSec = instant
	} else {
		ms.networkBytesPerSec = 0.75*ms.networkBytesPerSec + 0.25*instant
	}
	ms.networkSamples++
	ms.networkSampleAt = now
	ms.networkSampleBytes = 0
}

func (ms *MediaServer) observeMediaFailure() {
	ms.networkMu.Lock()
	ms.networkErrors++
	ms.networkLastErrorAt = time.Now()
	ms.networkMu.Unlock()
}

func (ms *MediaServer) flushMediaNetworkSample() {
	ms.networkMu.Lock()
	defer ms.networkMu.Unlock()
	if ms.networkSampleAt.IsZero() || ms.networkSampleBytes <= 0 {
		return
	}
	elapsed := time.Since(ms.networkSampleAt)
	if elapsed < 100*time.Millisecond {
		return
	}
	instant := float64(ms.networkSampleBytes) / elapsed.Seconds()
	if ms.networkBytesPerSec == 0 {
		ms.networkBytesPerSec = instant
	} else {
		ms.networkBytesPerSec = 0.75*ms.networkBytesPerSec + 0.25*instant
	}
	ms.networkSamples++
	ms.networkSampleAt = time.Now()
	ms.networkSampleBytes = 0
}

func (ms *MediaServer) mediaNetworkSnapshot() (profile string, bytesPerSecond float64, errors uint64) {
	ms.networkMu.Lock()
	ema := ms.networkBytesPerSec
	samples := ms.networkSamples
	failureCount := ms.networkErrors
	lastFailure := ms.networkLastErrorAt
	ms.networkMu.Unlock()

	profile = "unknown"
	switch {
	case samples == 0:
	case ema >= 1.5*1024*1024:
		profile = "strong"
	case ema >= 500*1024:
		profile = "medium"
	default:
		profile = "weak"
	}
	if failureCount > 0 && time.Since(lastFailure) < 2*time.Minute {
		switch profile {
		case "strong":
			profile = "medium"
		case "medium":
			profile = "weak"
		}
	}
	return profile, ema, failureCount
}

func NewMediaServer(client *sdk.Client) (*MediaServer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cacheDir := appdirs.AppCacheDir(home)
	if err := os.MkdirAll(filepath.Join(cacheDir, "episodes"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "images"), 0755); err != nil {
		return nil, err
	}
	removeLegacyPreviews(filepath.Join(cacheDir, "episodes"))
	removeHLSCache(filepath.Join(cacheDir, "episodes", "hls"))

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := l.Addr().(*net.TCPAddr).Port

	ms := &MediaServer{
		client:             client,
		cacheDir:           cacheDir,
		port:               port,
		pbCache:            make(map[string]playbackCacheItem),
		pbLoads:            make(map[string]*playbackLoad),
		prefetchQ:          make(chan PrefetchTask, 64),
		prefetchQueued:     make(map[string]PrefetchTask),
		prefetchActive:     make(map[string]*PrefetchExecution),
		progressiveStreams: make(map[string]*progressiveStream),
		hlsStreams:         make(map[string]*hlsStream),
		cachePins:          make(map[string]int),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/video", ms.handleVideo)
	mux.HandleFunc("/hls/", ms.handleHLS)
	mux.HandleFunc("/img", ms.handleImage)
	mux.HandleFunc("/poster", ms.handlePoster)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("OK"))
	})

	ms.server = &http.Server{Handler: mux}
	go func() {
		if err := ms.server.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Printf("media server error: %v", err)
		}
	}()

	// Start concurrent prefetch workers
	for i := 0; i < 3; i++ {
		go ms.prefetchWorker()
	}

	// Trigger initial cache check to clean up obsolete cache from previous runs
	ms.triggerCacheEviction()

	return ms, nil
}

// removeLegacyPreviews deletes obsolete preview and one-second warmup files.
func removeLegacyPreviews(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.Contains(name, "_preview") || strings.Contains(name, "_warm_") || strings.HasPrefix(name, "tmp_") {
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("[media cache] failed to remove legacy file %s: %v", path, err)
			}
		}
	}
}

func (ms *MediaServer) Port() int {
	return ms.port
}

func (ms *MediaServer) SetDeviceResetter(resetter func(ctx context.Context) error) {
	ms.onDeviceResetFn.Store(&resetter)
}

func (ms *MediaServer) resetDevice(ctx context.Context) error {
	if resetter := ms.onDeviceResetFn.Load(); resetter != nil {
		return (*resetter)(ctx)
	}
	return ms.client.ResetDevice(ctx)
}

func (ms *MediaServer) SetEpisodeCacheCallback(callback func(seriesID, episodeID string, num int, quality string) error) {
	ms.onEpisodeCacheFn.Store(&callback)
}

func (ms *MediaServer) VideoURL(episodeID, seriesID string, num int, quality string) string {
	return ms.videoURL(episodeID, seriesID, num, quality, "")
}

func (ms *MediaServer) ProgressiveVideoURL(episodeID, seriesID string, num int, quality string) string {
	return ms.videoURL(episodeID, seriesID, num, quality, "progressive")
}

func (ms *MediaServer) videoURL(episodeID, seriesID string, num int, quality, mode string) string {
	values := url.Values{}
	values.Set("id", episodeID)
	values.Set("series_id", seriesID)
	values.Set("num", strconv.Itoa(num))
	values.Set("quality", normalizeVideoQuality(quality))
	if mode != "" {
		values.Set("mode", mode)
	}
	return fmt.Sprintf("http://127.0.0.1:%d/video?%s", ms.port, values.Encode())
}

func (ms *MediaServer) PosterURL(episodeID string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/poster?id=%s", ms.port, url.QueryEscape(episodeID))
}

func normalizeImageURL(raw string) string {
	return mediautil.NormalizeImageURL(raw)
}

func (ms *MediaServer) ImageURL(origURL string) string {
	norm := normalizeImageURL(origURL)
	if norm == "" {
		return ""
	}
	enc := base64.URLEncoding.EncodeToString([]byte(norm))
	return fmt.Sprintf("http://127.0.0.1:%d/img?u=%s", ms.port, enc)
}

func (ms *MediaServer) handleVideo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Accept-Ranges, Content-Type")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	episodeID := r.URL.Query().Get("id")
	if !validContentID(episodeID) {
		http.Error(w, "invalid episode id", http.StatusBadRequest)
		return
	}
	seriesID := r.URL.Query().Get("series_id")
	numStr := r.URL.Query().Get("num")
	num, _ := strconv.Atoi(numStr)
	quality := normalizeVideoQuality(r.URL.Query().Get("quality"))
	if r.URL.Query().Get("mode") == "progressive" {
		if path := episodeCachePath(ms.cacheDir, episodeID, quality); path != "" {
			ms.serveProgressiveVideo(w, r, seriesID, episodeID, num, quality, path)
			return
		}
	}

	destPath := episodeCachePath(ms.cacheDir, episodeID, quality)
	if destPath == "" {
		http.Error(w, "invalid episode id", http.StatusBadRequest)
		return
	}
	if fileExists(destPath) {
		ms.serveEpisodeFile(w, r, destPath)
		return
	}

	actualQuality, actualPath, err := ms.downloadWithQualityFallback(r.Context(), episodeID, quality)
	if err != nil {
		log.Printf("download/decrypt failed for %s: %v", episodeID, err)
		http.Error(w, fmt.Sprintf("failed to decrypt media: %v", err), http.StatusInternalServerError)
		return
	}
	ms.rememberCachedEpisode(seriesID, episodeID, num, actualQuality)

	ms.serveEpisodeFile(w, r, actualPath)
}
func (ms *MediaServer) pinCacheFile(path string) func() {
	ms.cachePinMu.Lock()
	if ms.cachePins == nil {
		ms.cachePins = make(map[string]int)
	}
	ms.cachePins[path]++
	ms.cachePinMu.Unlock()

	return func() {
		ms.cachePinMu.Lock()
		if ms.cachePins[path] <= 1 {
			delete(ms.cachePins, path)
		} else {
			ms.cachePins[path]--
		}
		ms.cachePinMu.Unlock()
	}
}

func (ms *MediaServer) serveEpisodeFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "cached media is no longer available", http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "cached media is unavailable", http.StatusNotFound)
		return
	}
	_ = os.Chtimes(path, time.Now(), time.Now())
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func ffmpegDownloadArgs(streamURL, audioURL, keyHex string, maxSeconds int) []string {
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-nostdin"}

	input := func(mediaURL string) {
		if keyHex != "" {
			args = append(args, "-decryption_key", keyHex)
		}
		args = append(args, "-i", mediaURL)
	}
	input(streamURL)
	if audioURL = strings.TrimSpace(audioURL); audioURL != "" {
		// Segmented titles carry encrypted audio separately from video.
		input(audioURL)
		args = append(args, "-map", "0:v:0", "-map", "1:a:0")
	}
	if maxSeconds > 0 {
		args = append(args, "-t", strconv.Itoa(maxSeconds))
	}
	return append(args, "-c", "copy", "-f", "mp4", "-movflags", "+faststart")
}

func extractFirstFrame(ctx context.Context, sourcePath, destPath string) error {
	tmpPath := filepath.Join(filepath.Dir(destPath), "tmp_"+filepath.Base(destPath))
	defer os.Remove(tmpPath)

	args := []string{
		"-y", "-hide_banner", "-loglevel", "error", "-nostdin",
		"-ss", "0", "-i", sourcePath, "-frames:v", "1",
		"-vf", "scale=540:-2", "-q:v", "3", tmpPath,
	}
	ffmpegPath, err := ffmpegCommand()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg poster error (%v): %s", err, string(out))
	}
	return os.Rename(tmpPath, destPath)
}

// EnsureEpisodePoster returns a stable JPEG extracted from the first video frame if available.
func (ms *MediaServer) EnsureEpisodePoster(ctx context.Context, episodeID string) (string, error) {
	if !validContentID(episodeID) {
		return "", fmt.Errorf("missing episode id")
	}

	posterPath := filepath.Join(ms.cacheDir, "images", "frame_"+episodeID+".jpg")
	if fileExists(posterPath) {
		return ms.PosterURL(episodeID), nil
	}

	fullPath := episodeCachePath(ms.cacheDir, episodeID, DefaultVideoQuality)
	if !fileExists(fullPath) {
		return "", nil
	}

	mu := ms.lockFor(episodeID, DefaultVideoQuality)
	mu.Lock()
	defer mu.Unlock()

	if !fileExists(posterPath) && fileExists(fullPath) {
		if err := extractFirstFrame(ctx, fullPath, posterPath); err != nil && !fileExists(posterPath) {
			return "", err
		}
	}
	if fileExists(posterPath) {
		return ms.PosterURL(episodeID), nil
	}
	return "", nil
}

func (ms *MediaServer) handlePoster(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	episodeID := r.URL.Query().Get("id")
	if !validContentID(episodeID) {
		serveFallback(w)
		return
	}
	posterPath := filepath.Join(ms.cacheDir, "images", "frame_"+episodeID+".jpg")
	releaseCachePin := ms.pinCacheFile(posterPath)
	defer releaseCachePin()

	if _, err := ms.EnsureEpisodePoster(r.Context(), episodeID); err != nil {
		log.Printf("[poster] failed for %s: %v", episodeID, err)
		serveFallback(w)
		return
	}
	if fileExists(posterPath) {
		http.ServeFile(w, r, posterPath)
		return
	}
	serveFallback(w)
}

func (ms *MediaServer) rememberCachedEpisode(seriesID, episodeID string, num int, quality string) {
	callback := ms.onEpisodeCacheFn.Load()
	if callback == nil || seriesID == "" || episodeID == "" || num <= 0 {
		return
	}
	if err := (*callback)(seriesID, episodeID, num, quality); err != nil {
		log.Printf("[media cache index] failed for %s/%s: %v", seriesID, episodeID, err)
	}
	ms.triggerCacheEviction()
}

func (ms *MediaServer) downloadAndDecrypt(ctx context.Context, episodeID, quality, destPath string) error {
	ms.triggerCacheEviction()
	if !hasFreeDiskSpace(filepath.Dir(destPath), MinFreeCacheDiskBytes) {
		return fmt.Errorf("insufficient disk space for media cache")
	}
	downloadSem <- struct{}{}
	defer func() { <-downloadSem }()

	tmpPath := filepath.Join(filepath.Dir(destPath), "tmp_"+filepath.Base(destPath))
	defer os.Remove(tmpPath)

	pb, err := ms.resolvePlayback(ctx, episodeID)
	if err != nil {
		return err
	}
	stream, err := selectVideoStream(pb, quality)
	if err != nil {
		return err
	}

	keyHex := ""
	if pb.Protected {
		k, err := sdk.ContentKey(pb)
		if err != nil {
			return fmt.Errorf("failed to extract CENC key: %w", err)
		}
		keyHex = k.Hex
	}

	// Build ffmpeg command with stream copy (CENC decryption without re-encoding).
	args := append(ffmpegDownloadArgs(stream.URL, pb.AudioURL, keyHex, 0), tmpPath)

	ffmpegPath, err := ffmpegCommand()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error (%v): %s", err, string(out))
	}

	return os.Rename(tmpPath, destPath)
}

func videoQualityFallbackOrder(requested string) []string {
	requested = normalizeVideoQuality(requested)
	order := make([]string, 0, len(videoQualities))
	for _, quality := range videoQualities {
		if quality == requested {
			order = append(order, quality)
		}
	}
	// Prefer lower definitions before higher ones: a failed 720p download is
	// more likely to remain playable quickly at 540p than at 1080p.
	for _, quality := range videoQualities {
		if quality != requested && definitionNumber(quality) < definitionNumber(requested) {
			order = append(order, quality)
		}
	}
	for _, quality := range videoQualities {
		if quality != requested && definitionNumber(quality) > definitionNumber(requested) {
			order = append(order, quality)
		}
	}
	return order
}

func (ms *MediaServer) downloadWithQualityFallback(ctx context.Context, episodeID, requestedQuality string) (string, string, error) {
	for _, quality := range videoQualityFallbackOrder(requestedQuality) {
		destPath := episodeCachePath(ms.cacheDir, episodeID, quality)
		if fileExists(destPath) {
			return quality, destPath, nil
		}

		releaseCachePin := ms.pinCacheFile(destPath)
		mu := ms.lockFor(episodeID, quality)
		mu.Lock()
		if !fileExists(destPath) {
			err := ms.downloadAndDecrypt(ctx, episodeID, quality, destPath)
			if err != nil {
				mu.Unlock()
				releaseCachePin()
				if ctx.Err() != nil {
					return quality, destPath, err
				}
				continue
			}
		}
		mu.Unlock()
		releaseCachePin()
		return quality, destPath, nil
	}
	return normalizeVideoQuality(requestedQuality), episodeCachePath(ms.cacheDir, episodeID, requestedQuality), fmt.Errorf("all video qualities failed for %s", episodeID)
}

func (ms *MediaServer) prefetchWorker() {
	for task := range ms.prefetchQ {
		task = normalizePrefetchTask(task)
		key := PrefetchTaskKey(task)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

		ms.prefetchMu.Lock()
		if ms.prefetchQueued == nil {
			ms.prefetchQueued = make(map[string]PrefetchTask)
		}
		if ms.prefetchActive == nil {
			ms.prefetchActive = make(map[string]*PrefetchExecution)
		}
		delete(ms.prefetchQueued, key)
		execution := &PrefetchExecution{cancel: cancel}
		_, alreadyActive := ms.prefetchActive[key]
		if !alreadyActive {
			ms.prefetchActive[key] = execution
		}
		ms.prefetchMu.Unlock()

		if !alreadyActive {
			ms.runPrefetchTask(ctx, task, key)
		}
		cancel()

		ms.prefetchMu.Lock()
		if ms.prefetchActive[key] == execution {
			delete(ms.prefetchActive, key)
		}
		ms.prefetchMu.Unlock()
	}
}

func (ms *MediaServer) runPrefetchTask(ctx context.Context, task PrefetchTask, taskKey string) {
	if task.PrefetchOnly {
		return
	}
	episodeID := task.EpisodeID
	if episodeID == "" {
		episodes, err := ms.client.ListEpisodes(ctx, task.SeriesID, sdk.EpisodeOptions{})
		if err != nil {
			log.Printf("[prefetch] episode list failed for %s episode %d: %v", task.SeriesID, task.Number, err)
			return
		}
		for _, episode := range pageItems(episodes) {
			if episode.Number == task.Number {
				episodeID = episode.ID
				break
			}
		}
		if episodeID == "" {
			log.Printf("[prefetch] episode %d not found for %s", task.Number, task.SeriesID)
			return
		}
	}

	destPath := episodeCachePath(ms.cacheDir, episodeID, task.Quality)
	if fileExists(destPath) {
		return
	}
	if ms.hasProgressiveStream(episodeID, task.Quality) || ms.hasHLSStream(episodeID, task.Quality) {
		return
	}
	if task.Deferred {
		// PlayEpisode alone may use its full eight-second budget before the
		// video element opens the local URL. Cover that deadline plus IPC and
		// decoder startup before choosing a duplicate full-file fallback.
		waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		for {
			if ms.hasProgressiveStream(episodeID, task.Quality) || ms.hasHLSStream(episodeID, task.Quality) {
				cancel()
				return
			}
			if waitCtx.Err() != nil {
				break
			}
			select {
			case <-waitCtx.Done():
			case <-time.After(100 * time.Millisecond):
			}
		}
		cancel()
	}

	prefetchSem <- struct{}{}
	// A video element may open its progressive stream while episode metadata was
	// being resolved. Recheck under the prefetch semaphore before allocating a
	// second upstream connection.
	if fileExists(destPath) || ms.hasProgressiveStream(episodeID, task.Quality) || ms.hasHLSStream(episodeID, task.Quality) {
		<-prefetchSem
		return
	}
	ms.fullDownloads.Add(1)
	actualQuality, actualPath, err := ms.downloadWithQualityFallback(ctx, episodeID, task.Quality)
	if err != nil {
		ms.prefetchFailed.Add(1)
		ms.observeMediaFailure()
		log.Printf("[prefetch] download failed for %s: %v", taskKey, err)
	} else {
		ms.prefetchCompleted.Add(1)
		ms.rememberCachedEpisode(task.SeriesID, episodeID, task.Number, actualQuality)
		_ = actualPath
	}
	<-prefetchSem
}
func normalizePrefetchTask(task PrefetchTask) PrefetchTask {
	task.Quality = normalizeVideoQuality(task.Quality)
	if task.Number <= 0 {
		task.Number = 1
	}
	return task
}

func sortPrefetchTasks(tasks []PrefetchTask) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority < tasks[j].Priority
		}
		return PrefetchTaskKey(tasks[i]) < PrefetchTaskKey(tasks[j])
	})
}

func PrefetchTaskKey(task PrefetchTask) string {
	quality := normalizeVideoQuality(task.Quality)
	if task.EpisodeID != "" {
		return "episode\x00" + task.EpisodeID + "\x00" + quality
	}
	return "series\x00" + task.SeriesID + "\x00" + strconv.Itoa(task.Number) + "\x00" + quality
}

func (ms *MediaServer) enqueuePrefetchTask(task PrefetchTask) bool {
	if ms.prefetchQ == nil {
		return false
	}
	key := PrefetchTaskKey(task)
	if _, queued := ms.prefetchQueued[key]; queued {
		return true
	}
	if _, active := ms.prefetchActive[key]; active {
		return true
	}

	if cap(ms.prefetchQ) == 0 || len(ms.prefetchQ) >= cap(ms.prefetchQ) {
		return false
	}

	// Keep the queue priority-ordered. Workers consume from the head, while
	// replacement can reorder the active feed window as the user swipes.
	items := make([]PrefetchTask, 0, len(ms.prefetchQ)+1)
drain:
	for {
		select {
		case queued := <-ms.prefetchQ:
			items = append(items, queued)
		default:
			break drain
		}
	}
	items = append(items, task)
	sortPrefetchTasks(items)
	queuedNow := make(map[string]PrefetchTask, len(items))
	for _, ordered := range items {
		if len(ms.prefetchQ) >= cap(ms.prefetchQ) {
			ms.prefetchDropped.Add(1)
			continue
		}
		ms.prefetchQ <- ordered
		queuedNow[PrefetchTaskKey(ordered)] = ordered
	}
	clear(ms.prefetchQueued)
	for queuedKey, queuedTask := range queuedNow {
		ms.prefetchQueued[queuedKey] = queuedTask
	}
	return true
}

func (ms *MediaServer) PrefetchEpisode(seriesID, episodeID string, number int, quality string) {
	if seriesID == "" && episodeID == "" {
		return
	}
	if episodeID != "" && fileExists(episodeCachePath(ms.cacheDir, episodeID, normalizeVideoQuality(quality))) {
		return
	}

	task := normalizePrefetchTask(PrefetchTask{
		SeriesID:  seriesID,
		EpisodeID: episodeID,
		Number:    number,
		Quality:   quality,
	})
	ms.prefetchMu.Lock()
	if ms.prefetchQueued == nil {
		ms.prefetchQueued = make(map[string]PrefetchTask)
	}
	if ms.prefetchActive == nil {
		ms.prefetchActive = make(map[string]*PrefetchExecution)
	}
	queued := ms.enqueuePrefetchTask(task)
	ms.prefetchMu.Unlock()
	if !queued {
		log.Printf("[prefetch] queue full; dropped series=%s episodeID=%s number=%d", seriesID, episodeID, number)
	}
}

// ReplacePrefetchWindow replaces queued background work with the active feed
// window. Active downloads are cancelled unless their key remains desired.
func (ms *MediaServer) ReplacePrefetchWindow(tasks []PrefetchTask) {
	orderedTasks := make([]PrefetchTask, 0, len(tasks))
	desired := make(map[string]PrefetchTask, len(tasks))
	for _, task := range tasks {
		if task.SeriesID == "" && task.EpisodeID == "" {
			continue
		}
		task = normalizePrefetchTask(task)
		key := PrefetchTaskKey(task)
		if _, exists := desired[key]; !exists {
			desired[key] = task
			orderedTasks = append(orderedTasks, task)
		}
	}
	sortPrefetchTasks(orderedTasks)
	preservedRendererKey := ms.newestRendererStreamKey()

	ms.prefetchMu.Lock()
	defer ms.prefetchMu.Unlock()
	if ms.prefetchQueued == nil {
		ms.prefetchQueued = make(map[string]PrefetchTask)
	}
	if ms.prefetchActive == nil {
		ms.prefetchActive = make(map[string]*PrefetchExecution)
	}

	retained := make([]PrefetchTask, 0, len(orderedTasks))
drain:
	for {
		select {
		case queued := <-ms.prefetchQ:
			if _, keep := desired[PrefetchTaskKey(queued)]; keep {
				retained = append(retained, queued)
			}
		default:
			break drain
		}
	}

	for key, execution := range ms.prefetchActive {
		if _, keep := desired[key]; !keep {
			execution.cancel()
			ms.prefetchCanceled.Add(1)
		}
	}
	ms.progressiveMu.Lock()
	for key, stream := range ms.progressiveStreams {
		if _, keep := desired[key]; !keep {
			// A live media response is the renderer's current source even when a
			// newer adjacent HLS stream has just started. Canceling it makes the
			// video element fall back to a full-file download and rebuffer.
			if stream.rendererOwned && (key == preservedRendererKey || stream.hasRequestReader()) {
				continue
			}
			stream.cancel()
			ms.prefetchCanceled.Add(1)
		}
	}
	ms.progressiveMu.Unlock()
	ms.retainHLSStreams(desired, preservedRendererKey)
	clear(ms.prefetchQueued)
	for _, task := range retained {
		if !task.ProtectOnly && !task.PrefetchOnly {
			ms.enqueuePrefetchTask(task)
		}
	}
	for _, task := range orderedTasks {
		if !task.ProtectOnly && !task.PrefetchOnly {
			ms.enqueuePrefetchTask(task)
		}
	}
}

func (ms *MediaServer) newestRendererStreamKey() string {
	var newest rendererStreamCandidate
	consider := func(key string, startedAt time.Time, rendererOwned bool) {
		if !rendererOwned || startedAt.IsZero() || startedAt.Before(newest.startedAt) {
			return
		}
		newest = rendererStreamCandidate{key: key, startedAt: startedAt}
	}

	ms.progressiveMu.Lock()
	for key, stream := range ms.progressiveStreams {
		consider(key, stream.startedAt, stream.rendererOwned)
	}
	ms.progressiveMu.Unlock()

	ms.hlsMu.Lock()
	for key, stream := range ms.hlsStreams {
		consider(key, stream.startedAt, stream.rendererOwned)
	}
	ms.hlsMu.Unlock()
	return newest.key
}

func (ms *MediaServer) MediaDiagnostics() MediaDiagnosticsDTO {
	profile, bytesPerSecond, failures := ms.mediaNetworkSnapshot()
	return MediaDiagnosticsDTO{
		NetworkProfile:     profile,
		NetworkBytesPerSec: int64(bytesPerSecond),
		NetworkFailures:    int64(failures),
		PlaybackCacheHits:  ms.playbackCacheHits.Load(),
		PlaybackCacheMiss:  ms.playbackCacheMiss.Load(),
		ProgressiveStarts:  ms.progressiveStarts.Load(),
		ProgressiveHits:    ms.progressiveHits.Load(),
		HLSStarts:          ms.hlsStarts.Load(),
		HLSHits:            ms.hlsHits.Load(),
		FullDownloads:      ms.fullDownloads.Load(),
		PrefetchCompleted:  ms.prefetchCompleted.Load(),
		PrefetchFailed:     ms.prefetchFailed.Load(),
		PrefetchCanceled:   ms.prefetchCanceled.Load(),
		PrefetchDropped:    ms.prefetchDropped.Load(),
	}
}

func (ms *MediaServer) hasProgressiveStream(episodeID, quality string) bool {
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: episodeID, Quality: quality})
	ms.progressiveMu.Lock()
	defer ms.progressiveMu.Unlock()
	_, active := ms.progressiveStreams[key]
	return active
}

var (
	mediaHTTPClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
	sipsSem     = make(chan struct{}, 16)
	downloadSem = make(chan struct{}, 6)
	// Current playback, the immediate next video, and one look-ahead job may
	// remux concurrently. A single slot makes every swipe wait for the entire
	// preceding episode to download, even though its first segments are ready.
	hlsSem = make(chan struct{}, 3)
	// Feed cache warming is deliberately serialized: the immediate next video's
	// progressive stream always has first claim on upstream bandwidth.
	prefetchSem = make(chan struct{}, 1)
)

const fallbackSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400"><rect width="300" height="400" fill="#181920"/><circle cx="150" cy="160" r="36" fill="#252733"/><polygon points="142,146 166,160 142,174" fill="#666"/><text x="150" y="230" fill="#777" font-size="14" font-family="-apple-system,BlinkMacSystemFont,PingFang SC,sans-serif" text-anchor="middle">myguo</text></svg>`

func serveFallback(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fallbackSVG))
}

func (ms *MediaServer) handleImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rawParam := r.URL.Query().Get("u")
	if rawParam == "" {
		serveFallback(w)
		return
	}

	targetURL := rawParam
	if dec, err := base64.URLEncoding.DecodeString(rawParam); err == nil && len(dec) > 0 && strings.HasPrefix(string(dec), "http") {
		targetURL = string(dec)
	} else if decStd, err := base64.StdEncoding.DecodeString(rawParam); err == nil && len(decStd) > 0 && strings.HasPrefix(string(decStd), "http") {
		targetURL = string(decStd)
	} else {
		targetURL = normalizeImageURL(rawParam)
	}

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		serveFallback(w)
		return
	}
	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.User != nil || parsed.Port() != "" || !isAllowedImageHost(parsed.Hostname()) {
		log.Printf("[img-proxy] rejected untrusted image URL %s", targetURL)
		serveFallback(w)
		return
	}

	hash := md5.Sum([]byte(targetURL))
	hexHash := hex.EncodeToString(hash[:])
	cacheJpg := filepath.Join(ms.cacheDir, "images", hexHash+".jpg")
	releaseCachePin := ms.pinCacheFile(cacheJpg)
	defer releaseCachePin()

	// If already cached and valid, serve immediately
	if fi, err := os.Stat(cacheJpg); err == nil && fi.Size() > 500 {
		ms.triggerCacheEviction()
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		http.ServeFile(w, r, cacheJpg)
		return
	}

	// Lock per image to avoid concurrent duplicate downloads & conversion races
	lockVal, _ := ms.imgLocks.LoadOrStore(hexHash, &sync.Mutex{})
	mu := lockVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Recheck cache after acquiring lock
	if fi, err := os.Stat(cacheJpg); err == nil && fi.Size() > 500 {
		ms.triggerCacheEviction()
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		http.ServeFile(w, r, cacheJpg)
		return
	}

	// Fetch from upstream ByteDance CDN
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		serveFallback(w)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Referer", "https://novel.snssdk.com/")

	resp, err := mediaHTTPClient.Do(req)
	if err != nil {
		log.Printf("[img-proxy] download failed for %s: %v", targetURL, err)
		serveFallback(w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[img-proxy] upstream returned %d for %s", resp.StatusCode, targetURL)
		serveFallback(w)
		return
	}

	tmpRaw := filepath.Join(ms.cacheDir, "images", hexHash+".raw")
	f, err := os.Create(tmpRaw)
	if err != nil {
		serveFallback(w)
		return
	}
	_, err = io.Copy(f, resp.Body)
	_ = f.Close()
	if err != nil {
		_ = os.Remove(tmpRaw)
		serveFallback(w)
		return
	}
	defer os.Remove(tmpRaw)

	// Check if raw image is already standard browser-native format (JPEG, PNG, WebP, GIF)
	// to avoid expensive and slow /usr/bin/sips subprocess spawning.
	rawHeader := make([]byte, 16)
	if hf, hErr := os.Open(tmpRaw); hErr == nil {
		_, _ = io.ReadFull(hf, rawHeader)
		_ = hf.Close()
	}

	isJPEG := len(rawHeader) >= 3 && rawHeader[0] == 0xFF && rawHeader[1] == 0xD8 && rawHeader[2] == 0xFF
	isPNG := len(rawHeader) >= 8 && bytes.Equal(rawHeader[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	isWebP := len(rawHeader) >= 12 && string(rawHeader[:4]) == "RIFF" && string(rawHeader[8:12]) == "WEBP"
	isGIF := len(rawHeader) >= 6 && (string(rawHeader[:6]) == "GIF87a" || string(rawHeader[:6]) == "GIF89a")

	if isJPEG || isPNG || isWebP || isGIF {
		// Native web image format - rename directly with zero CPU conversion overhead!
		_ = os.Rename(tmpRaw, cacheJpg)
	} else {
		// Only invoke Apple sips CLI for HEIC or other non-native formats
		sipsSem <- struct{}{}
		cmd := exec.Command("/usr/bin/sips", "-s", "format", "jpeg", tmpRaw, "--out", cacheJpg)
		sipsErr := cmd.Run()
		<-sipsSem
		if sipsErr != nil {
			// Fallback: rename raw to cacheJpg if sips failed
			_ = os.Rename(tmpRaw, cacheJpg)
		}
	}

	if fi, err := os.Stat(cacheJpg); err == nil && fi.Size() > 500 {
		ms.triggerCacheEviction()
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		http.ServeFile(w, r, cacheJpg)
		return
	}

	serveFallback(w)
}

func isAllowedImageHost(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))
	if hostname == "" {
		return false
	}
	allowed := []string{
		"bytedance.com",
		"byteimg.com",
		"douyin.com",
		"douyinpic.com",
		"fqnovel.com",
		"fqnovelpic.com",
		"hongguo.com",
		"ixigua.com",
		"pstatp.com",
		"snssdk.com",
		"toutiao.com",
		"zjcdn.com",
	}
	for _, suffix := range allowed {
		if hostname == suffix || strings.HasSuffix(hostname, "."+suffix) {
			return true
		}
	}
	return false
}

func (ms *MediaServer) GetCacheSize() int64 {
	var totalSize int64
	_ = filepath.Walk(ms.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize
}

func (ms *MediaServer) ClearCache() (int64, error) {
	var totalSize int64
	_ = filepath.Walk(ms.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if strings.HasPrefix(info.Name(), "tmp_") {
				return nil
			}
			totalSize += info.Size()
			_ = os.Remove(path)
		}
		return nil
	})
	_ = os.MkdirAll(filepath.Join(ms.cacheDir, "episodes"), 0755)
	_ = os.MkdirAll(filepath.Join(ms.cacheDir, "images"), 0755)
	return totalSize, nil
}

func (ms *MediaServer) Close() {
	if ms.server != nil {
		_ = ms.server.Close()
	}
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 1024
}

func fileExists(path string) bool { return FileExists(path) }

func hasFreeDiskSpace(path string, minBytes uint64) bool {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return true
	}
	return stats.Bavail*uint64(stats.Bsize) >= minBytes
}

type cacheFileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

func evictDirLRU(dir string, maxBytes, targetBytes int64, pinnedPaths map[string]struct{}) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var files []cacheFileEntry
	var totalSize int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
		// Temporary files count toward capacity but belong to active writers.
		if strings.HasPrefix(name, "tmp_") {
			continue
		}
		path := filepath.Join(dir, name)
		if _, pinned := pinnedPaths[path]; pinned {
			continue
		}
		files = append(files, cacheFileEntry{
			path:    path,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	if totalSize <= maxBytes {
		return
	}

	// Sort files by ModTime ascending (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	for _, f := range files {
		if totalSize <= targetBytes {
			break
		}
		if err := os.Remove(f.path); err == nil {
			totalSize -= f.size
		}
	}
}

func (ms *MediaServer) triggerCacheEviction() {
	if time.Since(time.UnixMilli(ms.lastEvictionAt.Load())) < 30*time.Second {
		return
	}
	if !ms.evicting.CompareAndSwap(false, true) {
		return
	}
	ms.lastEvictionAt.Store(time.Now().UnixMilli())
	go func() {
		defer ms.evicting.Store(false)
		ms.cachePinMu.RLock()
		pinnedPaths := make(map[string]struct{}, len(ms.cachePins))
		for path := range ms.cachePins {
			pinnedPaths[path] = struct{}{}
		}
		evictDirLRU(filepath.Join(ms.cacheDir, "episodes"), MaxEpisodeCacheBytes, TargetEpisodeCacheBytes, pinnedPaths)
		evictDirLRU(filepath.Join(ms.cacheDir, "images"), MaxImageCacheBytes, TargetImageCacheBytes, pinnedPaths)
		ms.cachePinMu.RUnlock()

		ms.hlsMu.Lock()
		activeHLS := make(map[string]struct{}, len(ms.hlsStreams))
		for _, stream := range ms.hlsStreams {
			activeHLS[stream.dir] = struct{}{}
		}
		ms.hlsMu.Unlock()
		evictHLSCache(filepath.Join(ms.cacheDir, "episodes", "hls"), MaxHLSCacheBytes, TargetHLSCacheBytes, activeHLS)
	}()
}
