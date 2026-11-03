package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
)

const progressiveStreamTimeout = 10 * time.Minute

type progressiveStream struct {
	key       string
	seriesID  string
	episodeID string
	number    int
	quality   string
	tempPath  string
	finalPath string
	file      *os.File
	command   *exec.Cmd
	cancel    context.CancelFunc
	startedAt time.Time
	// rendererOwned is true for streams opened by a video element rather than
	// a background full-file prefetch.
	rendererOwned bool

	mu      sync.Mutex
	cond    *sync.Cond
	written int64
	done    bool
	err     error
	waiters map[chan struct{}]struct{}
	readers int

	// Written by the stream goroutine before finish() reads it.
	stderrTail string
}

func (stream *progressiveStream) retainReader() {
	stream.mu.Lock()
	stream.readers++
	stream.mu.Unlock()
}

func (stream *progressiveStream) releaseReader() {
	stream.mu.Lock()
	stream.readers--
	canFinalize := stream.done && stream.err == nil && stream.readers <= 0
	stream.mu.Unlock()
	if canFinalize {
		stream.finalize()
	}
}

func (stream *progressiveStream) hasRequestReader() bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.readers > 1
}

func (stream *progressiveStream) append(chunk []byte) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.done {
		return stream.err
	}
	if len(chunk) == 0 {
		return nil
	}
	if _, err := stream.file.Write(chunk); err != nil {
		stream.done = true
		stream.err = err
		stream.cond.Broadcast()
		return err
	}
	stream.written += int64(len(chunk))
	stream.cond.Broadcast()
	stream.notifyWaitersLocked()
	return nil
}

func (stream *progressiveStream) snapshot() (written int64, done bool, err error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.written, stream.done, stream.err
}

func (stream *progressiveStream) waitUntil(offset int64) (int64, bool, error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	for stream.written <= offset && !stream.done {
		stream.cond.Wait()
	}
	return stream.written, stream.done, stream.err
}

func (stream *progressiveStream) waitUntilContext(offset int64, ctx context.Context) error {
	for {
		stream.mu.Lock()
		if stream.written > offset || stream.done {
			stream.mu.Unlock()
			return nil
		}
		notify := make(chan struct{}, 1)
		if stream.waiters == nil {
			stream.waiters = make(map[chan struct{}]struct{})
		}
		stream.waiters[notify] = struct{}{}
		stream.mu.Unlock()

		select {
		case <-ctx.Done():
			stream.mu.Lock()
			delete(stream.waiters, notify)
			stream.mu.Unlock()
			return ctx.Err()
		case <-notify:
		}
	}
}

func (stream *progressiveStream) notifyWaitersLocked() {
	for waiter := range stream.waiters {
		close(waiter)
		delete(stream.waiters, waiter)
	}
}

func (ms *MediaServer) startProgressiveStream(seriesID, episodeID string, number int, quality, finalPath string) (*progressiveStream, error) {
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: episodeID, Quality: quality})

	ms.progressiveMu.Lock()
	if stream, active := ms.progressiveStreams[key]; active {
		ms.progressiveMu.Unlock()
		return stream, nil
	}
	if ms.progressiveStreams == nil {
		ms.progressiveStreams = make(map[string]*progressiveStream)
	}
	ms.progressiveMu.Unlock()

	pb, err := ms.resolvePlayback(context.Background(), episodeID)
	if err != nil {
		return nil, err
	}
	source, err := selectVideoStream(pb, quality)
	if err != nil {
		return nil, err
	}
	keyHex := ""
	if pb.Protected {
		contentKey, keyErr := sdk.ContentKey(pb)
		if keyErr != nil {
			return nil, fmt.Errorf("failed to extract CENC key: %w", keyErr)
		}
		keyHex = contentKey.Hex
	}

	ffmpegPath, err := ffmpegCommand()
	if err != nil {
		return nil, err
	}
	ms.triggerCacheEviction()
	if !hasFreeDiskSpace(filepath.Dir(finalPath), MinFreeCacheDiskBytes) {
		return nil, fmt.Errorf("insufficient disk space for progressive media")
	}

	downloadSem <- struct{}{}
	ms.progressiveMu.Lock()
	if stream, active := ms.progressiveStreams[key]; active {
		ms.progressiveMu.Unlock()
		<-downloadSem
		return stream, nil
	}
	if fileExists(finalPath) {
		ms.progressiveMu.Unlock()
		<-downloadSem
		return nil, os.ErrExist
	}

	tempPath := filepath.Join(filepath.Dir(finalPath), "tmp_stream_"+filepath.Base(finalPath))
	file, err := os.Create(tempPath)
	if err != nil {
		ms.progressiveMu.Unlock()
		<-downloadSem
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), progressiveStreamTimeout)
	command := exec.CommandContext(ctx, ffmpegPath, ffmpegProgressiveArgs(source.URL, pb.AudioURL, keyHex)...)
	var stderrBuf bytes.Buffer
	command.Stderr = &stderrBuf
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		_ = file.Close()
		_ = os.Remove(tempPath)
		ms.progressiveMu.Unlock()
		<-downloadSem
		return nil, err
	}
	if err := command.Start(); err != nil {
		cancel()
		_ = file.Close()
		_ = os.Remove(tempPath)
		ms.progressiveMu.Unlock()
		<-downloadSem
		return nil, err
	}

	stream := &progressiveStream{
		key:       key,
		seriesID:  seriesID,
		episodeID: episodeID,
		number:    number,
		quality:   quality,
		tempPath:  tempPath,
		finalPath: finalPath,
		file:      file,
		command:   command,
		cancel:    cancel,
		startedAt: time.Now(),
		// This path is reached only from a renderer media request.
		rendererOwned: true,
	}
	stream.cond = sync.NewCond(&stream.mu)
	stream.retainReader()
	ms.progressiveStreams[key] = stream
	ms.progressiveMu.Unlock()
	ms.progressiveStarts.Add(1)

	go ms.runProgressiveStream(ctx, stream, stdout, &stderrBuf)
	return stream, nil
}

func (ms *MediaServer) runProgressiveStream(ctx context.Context, stream *progressiveStream, stdout io.ReadCloser, stderrBuf *bytes.Buffer) {
	defer stream.cancel()
	defer func() { <-downloadSem }()

	go func() {
		<-ctx.Done()
		_ = stream.command.Process.Kill()
	}()

	buffer := make([]byte, 64*1024)
	for {
		read, readErr := stdout.Read(buffer)
		if read > 0 {
			ms.observeMediaBytes(read)
			if err := stream.append(buffer[:read]); err != nil {
				ms.observeMediaFailure()
				_ = stream.command.Process.Kill()
				_ = stdout.Close()
				waitErr := stream.command.Wait()
				stream.stderrTail = strings.TrimSpace(stderrBuf.String())
				stream.finish(waitErr)
				ms.rememberProgressiveStream(stream)
				break
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				ms.observeMediaFailure()
			} else {
				ms.flushMediaNetworkSample()
			}
			_ = stdout.Close()
			waitErr := stream.command.Wait()
			if waitErr != nil && ctx.Err() == nil {
				ms.observeMediaFailure()
			}
			stream.stderrTail = strings.TrimSpace(stderrBuf.String())
			stream.finish(waitErr)
			ms.rememberProgressiveStream(stream)
			break
		}
	}

	ms.progressiveMu.Lock()
	if ms.progressiveStreams[stream.key] == stream {
		delete(ms.progressiveStreams, stream.key)
	}
	ms.progressiveMu.Unlock()
	stream.releaseReader()
}

func (ms *MediaServer) rememberProgressiveStream(stream *progressiveStream) {
	_, _, streamErr := stream.snapshot()
	if streamErr != nil {
		return
	}
	ms.rememberCachedEpisode(stream.seriesID, stream.episodeID, stream.number, stream.quality)
}

func (stream *progressiveStream) finish(commandErr error) {
	written, _, _ := stream.snapshot()
	finalErr := commandErr
	if finalErr == nil && written <= 1024 {
		finalErr = fmt.Errorf("progressive stream produced %d bytes", written)
	}

	stream.mu.Lock()
	stream.done = true
	stream.err = finalErr
	stream.cond.Broadcast()
	stream.notifyWaitersLocked()
	stream.mu.Unlock()
	_ = stream.file.Close()
	if finalErr != nil && finalErr != context.Canceled {
		tail := ""
		if stream.stderrTail != "" {
			tail = ": " + stream.stderrTail
		}
		log.Printf("[progressive stream] failed for %s: %v%s", stream.key, finalErr, tail)
		_ = os.Remove(stream.tempPath)
	}
}

func (stream *progressiveStream) finalize() {
	stream.mu.Lock()
	alreadyFinalized := stream.done && stream.err != nil
	stream.mu.Unlock()
	if alreadyFinalized {
		return
	}
	if err := os.Rename(stream.tempPath, stream.finalPath); err != nil {
		log.Printf("[progressive stream] finalize failed for %s: %v", stream.key, err)
		_ = os.Remove(stream.tempPath)
	}
}

func ffmpegProgressiveArgs(streamURL, audioURL, keyHex string) []string {
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-nostdin"}
	input := func(mediaURL string) {
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
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-frag_duration", "1000000",
		"pipe:1",
	)
}

type httpByteRange struct {
	start    int64
	end      int64
	hasEnd   bool
	fullSize bool
}

func parseHTTPByteRange(header string, fileSize int64, growing bool) (httpByteRange, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return httpByteRange{start: 0, fullSize: true}, true
	}
	if !strings.HasPrefix(header, "bytes=") || strings.Contains(header, ",") {
		return httpByteRange{}, false
	}
	parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
	if len(parts) != 2 {
		return httpByteRange{}, false
	}

	if strings.TrimSpace(parts[0]) == "" {
		length, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || length <= 0 || length > fileSize {
			return httpByteRange{}, false
		}
		return httpByteRange{start: fileSize - length, end: fileSize - 1, hasEnd: true}, true
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || (start >= fileSize && !growing) {
		return httpByteRange{}, false
	}
	if strings.TrimSpace(parts[1]) == "" {
		return httpByteRange{start: start, fullSize: true}, true
	}
	end, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || end < start {
		return httpByteRange{}, false
	}
	return httpByteRange{start: start, end: end, hasEnd: true}, true
}

func (ms *MediaServer) serveProgressiveVideo(w http.ResponseWriter, r *http.Request, seriesID, episodeID string, number int, quality, finalPath string) {
	if fileExists(finalPath) {
		ms.progressiveHits.Add(1)
		ms.serveEpisodeFile(w, r, finalPath)
		return
	}

	releaseFinalPin := ms.pinCacheFile(finalPath)
	defer releaseFinalPin()
	stream, err := ms.startProgressiveStream(seriesID, episodeID, number, quality, finalPath)
	if err != nil {
		if err == os.ErrExist {
			ms.serveEpisodeFile(w, r, finalPath)
			return
		}
		log.Printf("[progressive stream] start failed for %s/%s: %v", episodeID, quality, err)
		http.Error(w, "progressive media is unavailable", http.StatusServiceUnavailable)
		return
	}

	fileSize, done, _ := stream.snapshot()
	if done && fileSize <= 1024 {
		http.Error(w, "progressive media ended early", http.StatusServiceUnavailable)
		return
	}

	byteRange, valid := parseHTTPByteRange(r.Header.Get("Range"), fileSize, !done)
	if !valid {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if done && byteRange.start >= fileSize {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "requested range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Feed-Stream", "progressive")

	status := http.StatusOK
	if byteRange.hasEnd {
		status = http.StatusPartialContent
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%s", byteRange.start, byteRange.end, streamSizeLabel(stream)))
	}

	if byteRange.hasEnd {
		length := byteRange.end - byteRange.start + 1
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(status)
		stream.retainReader()
		defer stream.releaseReader()
		_ = copyProgressiveRange(w, r, stream, byteRange.start, byteRange.end)
		return
	}

	if done {
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize-byteRange.start, 10))
	}
	w.WriteHeader(status)
	stream.retainReader()
	defer stream.releaseReader()
	_ = copyProgressiveRange(w, r, stream, byteRange.start, -1)
}

func streamSizeLabel(stream *progressiveStream) string {
	written, done, _ := stream.snapshot()
	if done {
		return strconv.FormatInt(written, 10)
	}
	return "*"
}

func copyProgressiveRange(w io.Writer, request *http.Request, stream *progressiveStream, start, end int64) error {
	offset := start
	for {
		written, done, streamErr := stream.snapshot()
		if written <= offset && !done {
			if err := stream.waitUntilContext(offset, request.Context()); err != nil {
				return err
			}
			continue
		}
		if streamErr != nil && written <= offset {
			return streamErr
		}
		if written > offset {
			available := written - offset
			if end >= 0 && available > end-offset+1 {
				available = end - offset + 1
			}
			file, err := os.Open(stream.tempPath)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(w, io.NewSectionReader(file, offset, available))
			_ = file.Close()
			if copyErr != nil {
				return copyErr
			}
			offset += available
		}
		if end >= 0 && offset > end {
			return nil
		}
		if done {
			return nil
		}
		if flusher, canFlush := w.(http.Flusher); canFlush {
			flusher.Flush()
		}
	}
}
