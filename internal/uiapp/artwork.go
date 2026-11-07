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
	used    uint64
}
type artworkLoader struct {
	mu         sync.Mutex
	entries    map[string]artworkEntry
	pending    map[string]bool
	visible    map[string]bool
	frame      map[string]bool
	clock      uint64
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
	l.clock++
	entry := l.entries[source]
	entry.used = l.clock
	if l.frame != nil {
		l.frame[source] = true
	}
	l.entries[source] = entry
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
	// Keep the access order recorded when the request began. Completion order
	// must not turn an old, offscreen request into the newest cache entry.
	entry := l.entries[source]
	entry.bitmap, entry.retryAt = bitmap, time.Now().Add(time.Minute)
	l.entries[source] = entry
	delete(l.pending, source)
	l.trimLocked()
	l.mu.Unlock()
	if l.invalidate != nil {
		l.invalidate()
	}
}

// The previous and current frame's artwork stays pinned while a frame is
// being built; asynchronous completions cannot evict posters still on screen.
func (l *artworkLoader) beginFrame() {
	l.mu.Lock()
	l.visible, l.frame = l.frame, make(map[string]bool)
	l.trimLocked()
	l.mu.Unlock()
}
func (l *artworkLoader) trimLocked() {
	for len(l.entries) > 128 {
		oldest := ""
		var used uint64 = ^uint64(0)
		for key, entry := range l.entries {
			if l.visible[key] || l.frame[key] || l.pending[key] {
				continue
			}
			if entry.used < used {
				oldest, used = key, entry.used
			}
		}
		if oldest == "" {
			return
		}
		delete(l.entries, oldest)
	}
}

// GridView builds visible cells during layout, after App.Render returns.
// Leave the frame open until the next render so these accesses are pinned too.
func (b *PlayerBridge) beginArtworkFrame() {
	b.mu.RLock()
	loader := b.artwork
	b.mu.RUnlock()
	if loader != nil {
		loader.beginFrame()
	}
}
