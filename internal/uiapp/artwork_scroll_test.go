package uiapp

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func artworkTestServer(t *testing.T) (*httptest.Server, func(string) int) {
	t.Helper()
	var data bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{230, 30, 70, 255})
		}
	}
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		if r.URL.Path == "/failed" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(data.Bytes())
	}))
	t.Cleanup(server.Close)
	return server, func(path string) int { mu.Lock(); defer mu.Unlock(); return counts[path] }
}

func awaitArtwork(t *testing.T, loader *artworkLoader) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		loader.mu.Lock()
		pending := len(loader.pending)
		loader.mu.Unlock()
		if pending == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("artwork requests did not finish")
}

func TestArtworkVisiblePosterSurvivesCachePressureAndFailedRequests(t *testing.T) {
	server, requests := artworkTestServer(t)
	loader := newArtworkLoader("", func(s string) string { return server.URL + s }, nil)
	loader.beginFrame()
	loader.get("/visible")
	awaitArtwork(t, loader)
	want := loader.get("/visible")
	if want == nil {
		t.Fatal("test poster did not decode")
	}
	// Completions from older scroll positions must not displace the visible
	// poster, even while layout is still building the next frame's cells.
	for i := range 160 {
		loader.beginFrame()
		loader.get("/visible")
		loader.load(fmt.Sprintf("/old-%d", i))
		if got := loader.get("/visible"); got != want {
			t.Fatal("visible poster reverted to a placeholder")
		}
	}
	if requests("/visible") != 1 {
		t.Fatal("visible image was fetched repeatedly")
	}
	loader.mu.Lock()
	size := len(loader.entries)
	loader.mu.Unlock()
	if size > 128 {
		t.Fatalf("cache grew to %d", size)
	}
	loader.get("/failed")
	awaitArtwork(t, loader)
	for range 20 {
		loader.beginFrame()
		loader.get("/failed")
	}
	if requests("/failed") != 1 {
		t.Fatal("rendering retried a failed image before its cooldown")
	}
}

func TestCatalogArtworkIsLazyAndCompletionDoesNotMoveCards(t *testing.T) {
	server, requests := artworkTestServer(t)
	loader := newArtworkLoader("", func(s string) string { return server.URL + s }, nil)
	a := NewTestApp(nil)
	a.player = &PlayerBridge{artwork: loader}
	for i := range 240 {
		a.catalog.Items = append(a.catalog.Items, CatalogItem{ID: fmt.Sprint(i), Title: fmt.Sprintf("poster-%d", i), CoverURL: fmt.Sprintf("/poster-%d", i)})
	}
	tt := ui.NewTester(a.Render, 1200, 800)
	tt.Frame()
	awaitArtwork(t, loader)
	tt.Frame()
	before, ok := tt.Find("poster-0")
	if !ok {
		t.Fatal("first card missing")
	}
	loader.mu.Lock()
	loaded := len(loader.entries)
	loader.mu.Unlock()
	if loaded == 0 || loaded > 40 {
		t.Fatalf("initial viewport loaded %d of 240 posters", loaded)
	}
	for range 10 {
		tt.Frame()
	}
	after, _ := tt.Find("poster-0")
	if before != after {
		t.Fatalf("image completion moved card: %v -> %v", before, after)
	}
	if requests("/poster-0") != 1 || requests("/poster-239") != 0 {
		t.Fatal("visible poster reloaded or distant poster loaded eagerly")
	}
	// Paging appends rows without touching existing images or geometry.
	for i := 240; i < 280; i++ {
		a.catalog.Items = append(a.catalog.Items, CatalogItem{ID: fmt.Sprint(i), Title: fmt.Sprintf("poster-%d", i), CoverURL: fmt.Sprintf("/poster-%d", i)})
	}
	tt.Frame()
	after, _ = tt.Find("poster-0")
	if before != after || requests("/poster-0") != 1 {
		t.Fatal("pagination rebuilt or moved existing posters")
	}
	tt.Scroll(700, 600, 0, 600)
	tt.Frame()
	awaitArtwork(t, loader)
	tt.Frame()
	loader.mu.Lock()
	loadedAfterScroll := len(loader.entries)
	loader.mu.Unlock()
	if loadedAfterScroll <= loaded {
		t.Fatal("scroll did not lazily request the next posters")
	}
}
