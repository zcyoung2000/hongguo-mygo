package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestCloseDrainsConvertersPrefetchWorkersAndHTTP(t *testing.T) {
	processDir := t.TempDir()
	t.Setenv("MYGUO_TEST_PROCESS_DIR", processDir)
	executable := filepath.Join(t.TempDir(), "conversion")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' $$ > \"$MYGUO_TEST_PROCESS_DIR/$$\"\nexec /bin/sleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	previous := ffmpegCommand
	ffmpegCommand = func() (string, error) { return executable, nil }
	t.Cleanup(func() { ffmpegCommand = previous })
	ms, err := newMediaServer(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.Close)
	ids := []string{"710000000000101", "710000000000102", "710000000000103"}
	ms.pbCacheMu.Lock()
	for _, id := range ids {
		ms.pbCache[id] = playbackCacheItem{
			pb:        &model.Playback{Streams: []model.Stream{{Definition: "720p", URL: "https://media.invalid/test.mp4"}}},
			expiresAt: time.Now().Add(time.Minute),
		}
	}
	ms.pbCacheMu.Unlock()
	hls, err := ms.StartHLSStream("710000000000001", ids[0], 1, "720p")
	if err != nil {
		t.Fatal(err)
	}
	progressive, err := ms.startProgressiveStream("710000000000002", ids[1], 1, "720p", episodeCachePath(ms.cacheDir, ids[1], "720p"))
	if err != nil {
		t.Fatal(err)
	}
	ms.PrefetchEpisode("710000000000003", ids[2], 1, "720p")
	deadline := time.Now().Add(3 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(processDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 3 {
			for _, entry := range entries {
				pid, err := strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
				pids = append(pids, pid)
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pids) != 3 {
		t.Fatal("HLS, progressive and prefetch converters did not all start")
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", ms.port)
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	ms.Close()
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("converter %d survived Close: %v", pid, err)
		}
	}
	select {
	case <-hls.done:
	default:
		t.Fatal("HLS task did not drain")
	}
	if _, done, _ := progressive.snapshot(); !done {
		t.Fatal("progressive task did not drain")
	}
	if response, err := client.Get(url); err == nil {
		response.Body.Close()
		t.Fatal("HTTP server survived Close")
	}
}

func TestClosedMediaServerRejectsNewWork(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir(), prefetchQ: make(chan PrefetchTask, 4)}
	ms.Close()
	if _, err := ms.StartHLSStream("710000000000001", "710000000000101", 1, "720p"); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed HLS admission = %v", err)
	}
	if _, err := ms.startProgressiveStream("710000000000001", "710000000000101", 1, "720p", "unused"); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed progressive admission = %v", err)
	}
	ms.PrefetchEpisode("710000000000001", "710000000000101", 1, "720p")
	if len(ms.prefetchQ) != 0 {
		t.Fatal("closed server queued a new prefetch")
	}
	if _, err := ms.ClearCache(); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed cache admission = %v", err)
	}
	ms.Close()
}
