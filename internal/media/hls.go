package media

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/mediautil"
)

func validContentID(id string) bool { return mediautil.ValidContentID(id) }

func pageItems[T any](page *sdk.Page[T]) []T { return mediautil.PageItems(page) }

const (
	hlsWaitTimeout             = 12 * time.Second
	activeHLSMediaRequestGrace = 8 * time.Second
)

type hlsStream struct {
	key          string
	dir          string
	playlistPath string
	cancel       context.CancelFunc
	done         chan struct{}
	startedAt    time.Time
	// rendererOwned marks streams created for a video element. Window
	// replacement may retain the newest one while resolving adjacent metadata.
	rendererOwned bool
	// Guarded by MediaServer.hlsMu. A new writer must wait for the canceled
	// writer and its directory cleanup before reusing this rendition path.
	retired chan struct{}

	mu                 sync.Mutex
	err                error
	lastMediaRequestAt time.Time
}

func (stream *hlsStream) touchMediaRequest() {
	stream.mu.Lock()
	stream.lastMediaRequestAt = time.Now()
	stream.mu.Unlock()
}

func (stream *hlsStream) hasRecentMediaRequest(now time.Time) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return !stream.lastMediaRequestAt.IsZero() && now.Sub(stream.lastMediaRequestAt) <= activeHLSMediaRequestGrace
}

func (stream *hlsStream) wait(ctx context.Context) error {
	select {
	case <-stream.done:
		stream.mu.Lock()
		err := stream.err
		stream.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (stream *hlsStream) finish(err error) {
	stream.mu.Lock()
	if stream.err == nil {
		stream.err = err
	}
	stream.mu.Unlock()
	close(stream.done)
}

func (ms *MediaServer) HLSMasterURL(episodeID, seriesID string, num int, quality string) string {
	values := urlValuesForEpisode(seriesID, num)
	return fmt.Sprintf(
		"http://127.0.0.1:%d/hls/%s/%s/master.m3u8?%s",
		ms.port,
		urlPathEscape(episodeID),
		normalizeVideoQuality(quality),
		values.Encode(),
	)
}

func (ms *MediaServer) HLSPlaylistURL(episodeID, seriesID string, num int, quality string) string {
	return fmt.Sprintf(
		"http://127.0.0.1:%d/hls/%s/%s/index.m3u8?%s",
		ms.port,
		urlPathEscape(episodeID),
		normalizeVideoQuality(quality),
		urlValuesForEpisode(seriesID, num).Encode(),
	)
}

func urlValuesForEpisode(seriesID string, num int) url.Values {
	values := url.Values{}
	values.Set("series_id", seriesID)
	values.Set("num", strconv.Itoa(num))
	return values
}

func urlPathEscape(value string) string {
	return url.PathEscape(value)
}

func (ms *MediaServer) StartHLSStream(seriesID, episodeID string, number int, quality string) (*hlsStream, error) {
	root, taskDone, err := ms.tasks.Acquire()
	if err != nil {
		return nil, err
	}
	async := false
	defer func() {
		if !async {
			taskDone()
		}
	}()
	if !validContentID(episodeID) {
		return nil, errors.New("invalid episode id")
	}
	quality = normalizeVideoQuality(quality)
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: episodeID, Quality: quality})

	ms.hlsMu.Lock()
	if stream, active := ms.hlsStreams[key]; active {
		if retired := stream.retired; retired != nil {
			ms.hlsMu.Unlock()
			<-retired
			return ms.StartHLSStream(seriesID, episodeID, number, quality)
		}
		completed := false
		select {
		case <-stream.done:
			completed = true
		default:
		}
		if !completed || hlsPlaylistComplete(stream.playlistPath) {
			ms.hlsMu.Unlock()
			return stream, nil
		}
		// External cleanup or a damaged cache must not leave an immortal done
		// task whose missing output can never be regenerated.
		delete(ms.hlsStreams, key)
	}
	if ms.hlsStreams == nil {
		ms.hlsStreams = make(map[string]*hlsStream)
	}
	dir := ms.hlsCacheDir(episodeID, quality)
	playlistPath := filepath.Join(dir, "index.m3u8")
	if hlsPlaylistComplete(playlistPath) {
		stream := &hlsStream{
			key:           key,
			dir:           dir,
			playlistPath:  playlistPath,
			cancel:        func() {},
			done:          make(chan struct{}),
			startedAt:     time.Now(),
			rendererOwned: true,
		}
		stream.finish(nil)
		ms.hlsStreams[key] = stream
		ms.hlsMu.Unlock()
		return stream, nil
	}

	// Clear interrupted files before publishing the new stream. A queued writer
	// may not run immediately, and HTTP requests must not see its predecessor's
	// partial playlist while waiting for a conversion slot.
	if err := os.RemoveAll(dir); err != nil {
		ms.hlsMu.Unlock()
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		ms.hlsMu.Unlock()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(root, progressiveStreamTimeout)
	stream := &hlsStream{
		key:          key,
		dir:          dir,
		playlistPath: playlistPath,
		cancel:       cancel,
		done:         make(chan struct{}),
		startedAt:    time.Now(),
		// This package currently creates HLS jobs only in response to
		// PlayEpisode or an HLS media request, both owned by the renderer.
		rendererOwned: true,
	}
	ms.hlsStreams[key] = stream
	ms.hlsMu.Unlock()
	ms.hlsStarts.Add(1)

	async = true
	go func() {
		defer taskDone()
		ms.runHLSStream(ctx, stream, seriesID, episodeID, number, quality)
	}()
	return stream, nil
}

func (ms *MediaServer) hasHLSStream(episodeID, quality string) bool {
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: episodeID, Quality: quality})
	ms.hlsMu.Lock()
	defer ms.hlsMu.Unlock()
	_, active := ms.hlsStreams[key]
	return active
}

func (ms *MediaServer) touchHLSStream(episodeID, quality string) {
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: episodeID, Quality: quality})
	ms.hlsMu.Lock()
	if stream := ms.hlsStreams[key]; stream != nil {
		stream.touchMediaRequest()
	}
	ms.hlsMu.Unlock()
}

func (ms *MediaServer) retainHLSStreams(desired map[string]PrefetchTask, preservedRendererKey string) {
	ms.hlsMu.Lock()
	defer ms.hlsMu.Unlock()
	now := time.Now()
	for key, stream := range ms.hlsStreams {
		if _, keep := desired[key]; keep {
			continue
		}
		if stream.rendererOwned && (key == preservedRendererKey || stream.hasRecentMediaRequest(now)) {
			continue
		}
		ms.retireHLSStreamLocked(key, stream)
	}
}

// Caller holds hlsMu. Keep completed renditions, and serialize interrupted
// writer cleanup with a later request that revisits the same episode.
func (ms *MediaServer) retireHLSStreamLocked(key string, stream *hlsStream) {
	if stream.retired != nil {
		return
	}
	stream.cancel()
	select {
	case <-stream.done:
		delete(ms.hlsStreams, key)
		return
	default:
	}
	stream.retired = make(chan struct{})
	go func() {
		<-stream.done
		ms.hlsMu.Lock()
		defer ms.hlsMu.Unlock()
		// ENDLIST can be committed just before done is closed. Retirement
		// must preserve that completed rendition for a later backward swipe.
		if !hlsPlaylistComplete(filepath.Join(stream.dir, "index.m3u8")) {
			_ = os.RemoveAll(stream.dir)
		}
		if ms.hlsStreams[stream.key] == stream {
			delete(ms.hlsStreams, stream.key)
		}
		close(stream.retired)
	}()
}

func (ms *MediaServer) hlsCacheDir(episodeID, quality string) string {
	if !validContentID(episodeID) {
		return ""
	}
	return filepath.Join(ms.cacheDir, "episodes", "hls", episodeID+"_"+normalizeVideoQuality(quality))
}

func hlsPlaylistReady(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "#EXTINF") {
		return false
	}
	dir := filepath.Dir(path)
	if strings.Contains(string(data), `URI="init.mp4"`) && !mediaFileExists(filepath.Join(dir, "init.mp4")) {
		return false
	}
	ready := true
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !mediaFileExists(filepath.Join(dir, line)) {
			ready = false
			break
		}
	}
	return ready
}

func hlsPlaylistComplete(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), "#EXT-X-ENDLIST") && hlsPlaylistReady(path)
}

func mediaFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func (ms *MediaServer) runHLSStream(ctx context.Context, stream *hlsStream, seriesID, episodeID string, number int, quality string) {
	defer stream.cancel()
	defer func() { stream.finish(ctx.Err()) }()

	select {
	case hlsSem <- struct{}{}:
		defer func() { <-hlsSem }()
	case <-ctx.Done():
		return
	}
	select {
	case downloadSem <- struct{}{}:
		defer func() { <-downloadSem }()
	case <-ctx.Done():
		return
	}
	if ctx.Err() != nil {
		return
	}
	ms.triggerCacheEviction()

	pb, err := ms.resolvePlayback(ctx, episodeID)
	if err != nil {
		ms.failHLSStream(stream, err)
		return
	}
	source, err := selectVideoStream(pb, quality)
	if err != nil {
		ms.failHLSStream(stream, err)
		return
	}
	keyHex := ""
	if pb.Protected {
		contentKey, keyErr := sdk.ContentKey(pb)
		if keyErr != nil {
			ms.failHLSStream(stream, fmt.Errorf("failed to extract CENC key: %w", keyErr))
			return
		}
		keyHex = contentKey.Hex
	}
	ffmpegPath, err := ffmpegCommand()
	if err != nil {
		ms.failHLSStream(stream, err)
		return
	}
	if !hasFreeDiskSpace(stream.dir, MinFreeCacheDiskBytes) {
		ms.failHLSStream(stream, errors.New("insufficient disk space for HLS media"))
		return
	}

	monitorDone := make(chan struct{})
	monitorExited := make(chan struct{})
	go func() {
		defer close(monitorExited)
		ms.monitorHLSOutput(ctx, stream, monitorDone)
	}()
	command := exec.CommandContext(ctx, ffmpegPath, ffmpegHLSArgs(source.URL, pb.AudioURL, keyHex, stream.dir)...)
	runErr := command.Run()
	close(monitorDone)
	<-monitorExited
	if runErr != nil {
		if ctx.Err() != nil {
			return
		}
		ms.failHLSStream(stream, runErr)
		return
	}
	ms.flushMediaNetworkSample()
	// A successful converter has produced the entire episode, even if the
	// preload window changes during bookkeeping immediately after it exits.
	if err := appendHLSEndList(stream.playlistPath); err != nil {
		ms.failHLSStream(stream, err)
		return
	}
	ms.rememberCachedEpisode(seriesID, episodeID, number, quality)
}

func (ms *MediaServer) failHLSStream(stream *hlsStream, err error) {
	stream.mu.Lock()
	stream.err = err
	stream.mu.Unlock()
	ms.observeMediaFailure()

	ms.hlsMu.Lock()
	if ms.hlsStreams[stream.key] == stream && stream.retired == nil {
		_ = os.RemoveAll(stream.dir)
		delete(ms.hlsStreams, stream.key)
	}
	ms.hlsMu.Unlock()
	log.Printf("[hls] stream failed for %s: %v", stream.key, err)
}

func (ms *MediaServer) monitorHLSOutput(ctx context.Context, stream *hlsStream, done chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastSize := int64(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
		}
		size := hlsDirectorySize(stream.dir)
		if size > lastSize {
			ms.observeMediaBytes(int(size - lastSize))
			lastSize = size
		}
	}
}

func hlsDirectorySize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func evictHLSCache(root string, maxBytes, targetBytes int64, pinnedPaths map[string]struct{}) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	type rendition struct {
		path    string
		size    int64
		modTime time.Time
	}
	var renditions []rendition
	var totalSize int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if _, pinned := pinnedPaths[path]; pinned {
			// Active writers remain excluded from the evictable total as well;
			// otherwise a long movie could repeatedly exceed the ceiling.
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		size := hlsDirectorySize(path)
		totalSize += size
		renditions = append(renditions, rendition{path: path, size: size, modTime: info.ModTime()})
	}
	if totalSize <= maxBytes {
		return
	}
	sort.Slice(renditions, func(i, j int) bool { return renditions[i].modTime.Before(renditions[j].modTime) })
	for _, item := range renditions {
		if totalSize <= targetBytes {
			break
		}
		if err := os.RemoveAll(item.path); err == nil {
			totalSize -= item.size
		}
	}
}

func ffmpegHLSArgs(streamURL, audioURL, keyHex, outputDir string) []string {
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-nostdin"}
	input := func(mediaURL string) {
		// MP4/DASH already carries stream metadata. Bound probing so a slow
		// source does not consume several megabytes before publishing segment 1.
		args = append(args, "-probesize", "131072", "-analyzeduration", "1000000", "-rw_timeout", "15000000")
		if keyHex != "" {
			args = append(args, "-decryption_key", keyHex)
		}
		args = append(args, "-i", mediaURL)
	}
	input(streamURL)
	if audioURL = strings.TrimSpace(audioURL); audioURL != "" {
		input(audioURL)
		args = append(args, "-map", "0:v:0", "-map", "1:a:0")
	}
	return append(args,
		"-c", "copy",
		"-max_interleave_delta", "1000000",
		"-flush_packets", "1",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_playlist_type", "event",
		"-hls_flags", "independent_segments+omit_endlist+temp_file",
		"-hls_segment_type", "fmp4",
		"-hls_segment_filename", filepath.Join(outputDir, "seg%03d.m4s"),
		filepath.Join(outputDir, "index.m3u8"),
	)
}

func appendHLSEndList(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "#EXT-X-ENDLIST") {
		return nil
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, []byte("#EXT-X-ENDLIST\n")...), 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (ms *MediaServer) handleHLS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	episodeID, quality, resource, ok := parseHLSPath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid HLS path", http.StatusBadRequest)
		return
	}
	seriesID := r.URL.Query().Get("series_id")
	number, _ := strconv.Atoi(r.URL.Query().Get("num"))
	if number <= 0 {
		number = 1
	}

	switch resource {
	case "master.m3u8":
		ms.serveHLSMaster(w, r, seriesID, number, episodeID, quality)
	case "index.m3u8":
		if !ms.serveHLSPlaylist(w, r, seriesID, episodeID, number, quality) {
			return
		}
	default:
		ms.serveHLSSegment(w, r, episodeID, quality, resource)
	}
}

func parseHLSPath(path string) (episodeID, quality, resource string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "hls" || !validContentID(parts[1]) || parts[2] == "" || parts[3] == "" {
		return "", "", "", false
	}
	for _, known := range videoQualities {
		if parts[2] == known {
			quality = known
			return parts[1], quality, parts[3], true
		}
	}
	return "", "", "", false
}

func (ms *MediaServer) serveHLSMaster(w http.ResponseWriter, r *http.Request, seriesID string, number int, episodeID, requestedQuality string) {
	pb, resolveErr := ms.resolvePlayback(r.Context(), episodeID)
	lines := []string{"#EXTM3U", "#EXT-X-VERSION:7", "#EXT-X-INDEPENDENT-SEGMENTS"}
	seen := make(map[string]bool)
	variantValues := urlValuesForEpisode(seriesID, number)
	requestedDefinition := definitionNumber(normalizeVideoQuality(requestedQuality))
	if resolveErr == nil && pb != nil {
		streams := append([]model.Stream(nil), pb.Streams...)
		sort.SliceStable(streams, func(i, j int) bool { return streamQualityRank(streams[i]) > streamQualityRank(streams[j]) })
		for _, stream := range streams {
			if strings.TrimSpace(stream.URL) == "" {
				continue
			}
			quality := qualityForStreamRank(streamQualityRank(stream))
			if seen[quality] {
				continue
			}
			// Even when this endpoint is used directly, never offer a rendition
			// above the feed's requested/network-safe ceiling.
			if requestedDefinition > 0 && definitionNumber(quality) > requestedDefinition {
				continue
			}
			seen[quality] = true
			bandwidth := estimatedHLSBandwidth(stream.Bitrate, streamQualityRank(stream), pb.AudioURL != "")
			resolution := ""
			if stream.Width > 0 && stream.Height > 0 {
				resolution = fmt.Sprintf(",RESOLUTION=%d:%d", stream.Width, stream.Height)
			}
			codecs := hlsCodecAttribute(stream.Codec, pb.AudioURL != "")
			if codecs != "" {
				resolution += ",CODECS=" + codecs
			}
			lines = append(lines, fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d%s", bandwidth, resolution))
			variantValues := cloneURLValues(variantValues)
			lines = append(lines, "../"+quality+"/index.m3u8?"+variantValues.Encode())
		}
	}
	if len(lines) == 3 {
		quality := normalizeVideoQuality(requestedQuality)
		lines = append(lines,
			"#EXT-X-STREAM-INF:BANDWIDTH=1500000",
			"../"+quality+"/index.m3u8?"+cloneURLValues(variantValues).Encode())
	}
	lines = append(lines, "")
	writeHLSResponse(w, http.StatusOK, []byte(strings.Join(lines, "\n")))
}

func cloneURLValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

func estimatedHLSBandwidth(videoBitrate int64, definition int, hasAudio bool) int64 {
	bandwidth := videoBitrate
	if bandwidth <= 0 {
		bandwidth = int64(definition) * 100000
	}
	if bandwidth <= 0 {
		bandwidth = 1000000
	}
	if hasAudio {
		// Separate DASH audio is muxed into the rendition; include conservative
		// AAC overhead so players do not overestimate available capacity.
		bandwidth += 128000
	}
	return bandwidth
}

func hlsCodecAttribute(codec string, hasSeparateAudio bool) string {
	codec = strings.ToLower(strings.TrimSpace(codec))
	// Only emit RFC 6381 codecs already supplied by the source. Mapping generic
	// labels such as "h264" to a guessed profile would make ABR selection wrong.
	// If a separate audio track is muxed in and its codec is unknown, omit the
	// attribute instead of claiming a video-only rendition.
	if hasSeparateAudio {
		return ""
	}
	if strings.HasPrefix(codec, "avc1.") || strings.HasPrefix(codec, "hvc1.") || strings.HasPrefix(codec, "hev1.") {
		return `"` + codec + `"`
	}
	return ""
}

func pruneIncompleteHLSCache(root string) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() && hlsPlaylistComplete(filepath.Join(path, "index.m3u8")) {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func (ms *MediaServer) HasCachedEpisode(episodeID, quality string) bool {
	return validContentID(episodeID) && (fileExists(episodeCachePath(ms.cacheDir, episodeID, normalizeVideoQuality(quality))) ||
		ms.HasCachedHLS(episodeID, quality))
}

func (ms *MediaServer) HasCachedHLS(episodeID, quality string) bool {
	return validContentID(episodeID) && hlsPlaylistComplete(filepath.Join(ms.hlsCacheDir(episodeID, quality), "index.m3u8"))
}

func (ms *MediaServer) serveHLSPlaylist(w http.ResponseWriter, r *http.Request, seriesID, episodeID string, number int, quality string) bool {
	release := ms.pinCacheFile(ms.hlsCacheDir(episodeID, quality))
	defer release()
	ms.hlsHits.Add(1)
	stream, err := ms.StartHLSStream(seriesID, episodeID, number, quality)
	if err != nil {
		http.Error(w, "HLS media is unavailable", http.StatusServiceUnavailable)
		return false
	}
	stream.touchMediaRequest()

	waitCtx, cancel := context.WithTimeout(r.Context(), hlsWaitTimeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for !hlsPlaylistReady(stream.playlistPath) {
		select {
		case <-stream.done:
			if !hlsPlaylistReady(stream.playlistPath) {
				http.Error(w, "HLS media ended before a segment was ready", http.StatusServiceUnavailable)
				return false
			}
		case <-waitCtx.Done():
			if streamErr := stream.wait(waitCtx); streamErr != nil && !errors.Is(streamErr, context.DeadlineExceeded) {
				log.Printf("[hls] playlist unavailable for %s: %v", stream.key, streamErr)
			}
			http.Error(w, "HLS playlist is not ready", http.StatusServiceUnavailable)
			return false
		case <-ticker.C:
		}
	}

	data, err := os.ReadFile(stream.playlistPath)
	if err != nil {
		http.Error(w, "HLS playlist is unavailable", http.StatusServiceUnavailable)
		return false
	}
	// EVENT streams are generated ahead of playback. Start at the episode's
	// beginning, not at a live-edge offset chosen from the growing playlist.
	data = []byte(strings.Replace(string(data), "#EXTM3U\n", "#EXTM3U\n#EXT-X-START:TIME-OFFSET=0,PRECISE=YES\n", 1))
	writeHLSResponse(w, http.StatusOK, data)
	return true
}

func (ms *MediaServer) serveHLSSegment(w http.ResponseWriter, r *http.Request, episodeID, quality, name string) {
	if name != "init.mp4" && (!strings.HasPrefix(name, "seg") || !strings.HasSuffix(name, ".m4s")) {
		http.Error(w, "invalid HLS segment", http.StatusBadRequest)
		return
	}
	release := ms.pinCacheFile(ms.hlsCacheDir(episodeID, quality))
	defer release()
	path := filepath.Join(ms.hlsCacheDir(episodeID, quality), name)
	if !strings.HasPrefix(path, filepath.Clean(ms.hlsCacheDir(episodeID, quality))+string(os.PathSeparator)) {
		http.Error(w, "invalid HLS segment", http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	ms.touchHLSStream(episodeID, quality)
	http.ServeFile(w, r, path)
}

func writeHLSResponse(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
