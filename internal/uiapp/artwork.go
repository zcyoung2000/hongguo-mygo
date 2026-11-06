package uiapp

import (
	"context"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/mediautil"
)

type artworkEntry struct {
	bitmap  ui.ImageSource
	retryAt time.Time
}
type artworkLoader struct {
	mu         sync.Mutex
	entries    map[string]artworkEntry
	pending    map[string]bool
	slots      chan struct{}
	cacheDir   string
	proxyURL   func(string) string
	invalidate func()
}

func newArtworkLoader(dir string, proxy func(string) string, invalidate func()) *artworkLoader {
	return &artworkLoader{entries: map[string]artworkEntry{}, pending: map[string]bool{}, slots: make(chan struct{}, 4), cacheDir: dir, proxyURL: proxy, invalidate: invalidate}
}

// Artwork never does disk IO, decoding or HTTP on the render thread. Only the local MediaServer proxy is fetched.
func (b *PlayerBridge) Artwork(source string) ui.ImageSource {
	b.mu.Lock()
	if b.artwork == nil && b.media != nil {
		b.artwork = newArtworkLoader(b.media.CacheDir(), b.media.ImageURL, func() {
			b.mu.RLock()
			fn := b.invalidateFn
			b.mu.RUnlock()
			if fn != nil {
				fn()
			}
		})
	}
	loader := b.artwork
	b.mu.Unlock()
	if loader == nil || source == "" {
		return nil
	}
	return loader.get(source)
}
func (l *artworkLoader) get(source string) ui.ImageSource {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[source]
	if entry.bitmap != nil {
		return entry.bitmap
	}
	if !l.pending[source] && !time.Now().Before(entry.retryAt) {
		l.pending[source] = true
		go l.load(source)
	}
	return nil
}
func (l *artworkLoader) load(source string) {
	l.slots <- struct{}{}
	defer func() { <-l.slots }()
	var bitmap ui.ImageSource
	if l.cacheDir != "" {
		if data, err := os.ReadFile(mediautil.ImageCachePath(l.cacheDir, source)); err == nil {
			bitmap, _ = ui.DecodeImage(data)
		}
	}
	if bitmap == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.proxyURL(source), nil)
		if err == nil {
			// Do not follow a redirect out of the local media pipeline.
			client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			if resp, err := client.Do(req); err == nil {
				if resp.StatusCode == http.StatusOK {
					if data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)); err == nil {
						bitmap, _ = ui.DecodeImage(data)
					}
				}
				resp.Body.Close()
			}
		}
	}
	l.mu.Lock()
	// Bound decoded memory; evicted images can be reloaded from the disk cache.
	if len(l.entries) >= 128 {
		for key := range l.entries {
			delete(l.entries, key)
			break
		}
	}
	l.entries[source] = artworkEntry{bitmap: bitmap, retryAt: time.Now().Add(time.Minute)}
	delete(l.pending, source)
	l.mu.Unlock()
	if l.invalidate != nil {
		l.invalidate()
	}
}
