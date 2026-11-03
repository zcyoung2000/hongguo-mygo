package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
)

func TestProtectedFullCacheDownloadUsesContentKeyAndDecrypts(t *testing.T) {
	const playbackPath = "/novel/player/video_model/v1/"
	const registerPath = "/service/2/device_register/"
	const sourceURL = "https://protected.invalid/720.mp4"
	videoModel := map[string]any{
		"status":     10,
		"video_id":   "protected-video",
		"media_type": "video",
		"video_list": []map[string]any{{
			"main_url": sourceURL,
			"encrypt_info": map[string]any{
				"encrypt":           true,
				"encryption_method": "cenc-aes-ctr",
				"kid":               "test-kid",
				"spade_a":           "nLwSxG6JI85zhT//c7AK4HauO+lzrg7TdZwO0XWrP+J0pDypqQ==",
			},
			"video_meta": map[string]any{
				"definition": "720p",
				"codec_type": "bytevc1",
				"vwidth":     720,
				"vheight":    1280,
				"bitrate":    1_000_000,
				"size":       1_000_000,
			},
		}},
	}
	encodedModel, err := json.Marshal(videoModel)
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case registerPath:
			_, _ = w.Write([]byte(`{"new_user":1,"device_id_str":"7100000000000001","install_id_str":"7100000000000002"}`))
		case playbackPath:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data":    map[string]any{"video_model": string(encodedModel)},
			})
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	client, err := adapters.NewClient(profilePath, sdk.Options{
		ProfileStore:   memstore.New(),
		RegisterOrigin: server.URL,
		APIOrigin:      server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	pb, err := client.ResolvePlayback(context.Background(), "7100000000000201", sdk.PlaybackOptions{})
	if !errors.Is(err, sdk.ErrDRMProtected) || pb == nil {
		t.Fatalf("ResolvePlayback() error = %v, playback = %+v", err, pb)
	}
	key, err := sdk.ContentKey(pb)
	if err != nil {
		t.Fatalf("ContentKey() error = %v", err)
	}
	ffmpegPath := filepath.Join(t.TempDir(), "ffmpeg")
	script := fmt.Sprintf(`#!/bin/bash
key=
url=
previous=
for argument; do
  if [ "$previous" = "-decryption_key" ]; then key="$argument"; fi
  if [ "$previous" = "-i" ]; then url="$argument"; fi
  previous="$argument"
done
if [ "$key" != "%s" ] || [ "$url" != "%s" ]; then
  exit 97
fi
dd if=/dev/zero of="${@: -1}" bs=1024 count=2 status=none
`, key.Hex, sourceURL)
	if err := os.WriteFile(ffmpegPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HONGGUO_FFMPEG", ffmpegPath)

	ms := &MediaServer{
		client:   client,
		cacheDir: t.TempDir(),
		pbCache:  make(map[string]playbackCacheItem),
		pbLoads:  make(map[string]*playbackLoad),
	}
	destPath := episodeCachePath(ms.cacheDir, "7100000000000201", "720p")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ms.downloadAndDecrypt(context.Background(), "7100000000000201", "720p", destPath); err != nil {
		t.Fatalf("downloadWithQualityFallback() error = %v", err)
	}
	info, err := os.Stat(destPath)
	if err != nil || info.Size() != 2048 {
		t.Fatalf("decrypted cache stat = %v, err = %v", info, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(destPath)); len(entries) != 1 {
		t.Fatalf("temporary files were not cleaned: %v", entries)
	}
	if !FileExists(destPath) {
		t.Fatal("decrypted file is not considered playable")
	}
}
