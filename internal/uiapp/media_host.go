package uiapp

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/egoist/mygo"
	"myguo/internal/native"
)

// MediaHost embeds the existing feature-complete playback scenes. Navigation and
// catalog UI remain native; source negotiation, MSE/HLS, buffering and controls
// share the same tested implementation as before the shell migration.
type MediaHost struct {
	mu       sync.Mutex
	view     *mygo.WebView
	ready    bool
	desired  map[string]any
	queue    []map[string]any
	draining bool
}

func (h *MediaHost) Attach(view *mygo.WebView) {
	h.mu.Lock()
	h.view = view
	h.startLocked()
	h.mu.Unlock()
	view.Page().OnRenderProcessGone(func(reason string) {
		h.mu.Lock()
		h.ready = false
		h.mu.Unlock()
		log.Printf("[media-host] renderer gone: %s", reason)
	})
}
func (h *MediaHost) WebView() *mygo.WebView { h.mu.Lock(); defer h.mu.Unlock(); return h.view }
func (h *MediaHost) scene(payload map[string]any) {
	h.sceneLocked(payload)
}
func (h *MediaHost) sceneLocked(payload map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.desired = payload
	h.queue = nil
	if h.ready {
		h.queue = append(h.queue, payload)
		h.startLocked()
	}
}
func (h *MediaHost) Recommend() { h.scene(map[string]any{"event": "recommend"}) }
func (h *MediaHost) RecommendChannel(channel string) {
	h.scene(map[string]any{"event": "recommend", "channel": channel})
}
func (h *MediaHost) Settings() { h.scene(map[string]any{"event": "settings"}) }
func (h *MediaHost) Stop()     { h.scene(map[string]any{"event": "stop"}) }
func (h *MediaHost) StopNow()  { h.stopWithoutWaiting() }
func (h *MediaHost) Detail(id string) {
	h.scene(map[string]any{"event": "detail", "seriesId": id})
}
func (h *MediaHost) Play(id string, num int, position float64, session int64) {
	// nil retains PlayEpisode's saved position for normal play/episode selection.
	var resume any
	if position >= 0 {
		resume = position
	}
	h.scene(map[string]any{"event": "play", "seriesId": id, "episodeNum": num, "resumeAt": resume, "session": session})
}
func (h *MediaHost) Command(p native.MediaCommandPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ready {
		h.queue = append(h.queue, map[string]any{"event": "command", "command": p.Command, "position": p.Position})
		h.startLocked()
	}
}
func (h *MediaHost) PictureInPicture() { h.Command(native.MediaCommandPayload{Command: "pip"}) }
func (h *MediaHost) PlayerControl(command string, value any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ready {
		h.queue = append(h.queue, map[string]any{"event": "command", "command": command, "value": value})
		h.startLocked()
	}
}
func (h *MediaHost) startLocked() {
	if !h.draining && h.view != nil {
		h.draining = true
		go h.drain()
	}
}
func (h *MediaHost) drain() {
	for {
		h.mu.Lock()
		if len(h.queue) == 0 || !h.ready {
			h.draining = false
			h.mu.Unlock()
			return
		}
		payload, view := h.queue[0], h.view
		h.queue = h.queue[1:]
		h.mu.Unlock()
		data, err := json.Marshal(payload)
		if err == nil && view != nil {
			_, err = view.Page().Eval("window.nativeMediaEvent && window.nativeMediaEvent(" + string(data) + ");void 0")
		}
		if err != nil {
			log.Printf("[media-host] command failed: %v", err)
		}
	}
}

// NativeShell exposes only callbacks needed by the media document.
type NativeShell struct{ App *App }

type AmbientGlowPayload struct {
	Active bool     `json:"active"`
	Left   [3]uint8 `json:"left"`
	Top    [3]uint8 `json:"top"`
}

func (s *NativeShell) AmbientGlow(payload AmbientGlowPayload) {
	s.App.SetAmbientGlow(payload.Active, payload.Left, payload.Top)
}

func (s *NativeShell) OpenDetail(id string)      { s.App.OpenDetail(id) }
func (s *NativeShell) OpenActor(id string)       { s.App.OpenActor(id) }
func (s *NativeShell) Closed(session int64)      { s.App.CloseMediaSession(session) }
func (s *NativeShell) ClosePlayer(session int64) { s.App.ClosePlayer(session) }
func (s *NativeShell) LibraryChanged()           { s.App.LibraryChanged() }
func (s *NativeShell) ToggleFullscreen() {
	if w := mygo.FocusedWindow(); w != nil {
		w.ToggleFullScreen()
		return
	}
	if windows := mygo.Windows(); len(windows) > 0 {
		windows[0].ToggleFullScreen()
		return
	}
	native.ToggleWindowFullscreen()
}
func (s *NativeShell) PlayerChromeChanged(value PlayerChrome) {
	s.App.UpdatePlayerChrome(value)
}

// StopNow uses the same ordered dispatcher as playback. An independent Eval
// could arrive after a newer Play and leave its visible WebView on native-idle.
func (h *MediaHost) stopWithoutWaiting() { h.Stop() }

// MediaReady is sent by native-media.js after its scene handler is installed.
// DOM ready alone is insufficient: a command must never be silently dropped
// because nativeMediaEvent hasn't been defined yet.
func (s *NativeShell) MediaReady() {
	s.App.mu.Lock()
	host := s.App.media
	s.App.mu.Unlock()
	if host != nil {
		host.markReady()
	}
}
func (h *MediaHost) markReady() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ready {
		return
	}
	h.ready = true
	if h.desired != nil {
		h.queue = append(h.queue, h.desired)
	}
	h.startLocked()
}
