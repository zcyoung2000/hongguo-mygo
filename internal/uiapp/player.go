package uiapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"myguo/internal/ipc"
	"myguo/internal/native"
)

type progressPayload struct {
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	Playing   bool    `json:"playing"`
	SeriesID  string  `json:"seriesId"`
	EpisodeID string  `json:"episodeId"`
	Session   int     `json:"session"`
}
type playbackProvider interface {
	PlayEpisode(context.Context, string, string, int, string, bool) (*ipc.PlaybackDTO, error)
}
type progressSaver interface {
	SaveProgress(context.Context, string, string, string, string, int, int, float64, float64) error
}
type seriesMetadata struct {
	ID, Title, CoverURL string
	TotalEpisodes       int
}
type PlaybackState struct {
	SeriesID, EpisodeID string
	EpisodeNum          int
	Quality             string
	Speed               float64
	Title, CoverURL     string
	Position, Duration  float64
	Playing             bool
	Session             int
	TotalEpisodes       int
	StreamURL           string
}
type playerCommand struct {
	session int
	payload map[string]any
}

type PlayerBridge struct {
	service      *ipc.HongguoService
	media        *ipc.MediaServer
	webview      *mygo.WebView
	page         *mygo.Page
	invalidateFn func()
	testMode     bool
	playback     playbackProvider
	saver        progressSaver

	mu       sync.RWMutex
	current  PlaybackState
	metadata seriesMetadata
	token    int
	sessions map[int]PlaybackState
	commands []playerCommand
	draining bool
	ready    bool
	saveMu   sync.Mutex
	saved    map[string]int
	artwork  *artworkLoader
}

func NewPlayerBridge(service *ipc.HongguoService, media *ipc.MediaServer) *PlayerBridge {
	return &PlayerBridge{service: service, media: media, sessions: map[int]PlaybackState{}, saved: map[string]int{}}
}
func NewTestPlayerBridge() *PlayerBridge {
	b := NewPlayerBridge(nil, nil)
	b.testMode = true
	return b
}

// Only JSON-safe callbacks are exposed to the player page.
type PlayerCallbacks struct{ bridge *PlayerBridge }

func (b *PlayerBridge) Callbacks() *PlayerCallbacks         { return &PlayerCallbacks{bridge: b} }
func (c *PlayerCallbacks) Progress(payload progressPayload) { c.bridge.ReportProgress(payload) }
func (c *PlayerCallbacks) SaveProgress(payload progressPayload) error {
	return c.bridge.SaveProgress(payload)
}
func (c *PlayerCallbacks) Next(payload progressPayload) {
	b := c.bridge
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.matches(payload) {
		b.current.Playing = true
		b.shiftLocked(1)
	}
}
func (b *PlayerBridge) SetInvalidator(fn func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.invalidateFn = fn
}
func (b *PlayerBridge) SetSeriesMetadata(id, title, cover string, total int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.metadata = seriesMetadata{id, title, cover, total}
}
func (b *PlayerBridge) Attach(_ *mygo.Window, view *mygo.WebView) {
	b.mu.Lock()
	b.webview, b.page = view, view.Page()
	b.mu.Unlock()
	view.Page().OnDOMReady(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.ready = true
		if b.current.StreamURL != "" {
			b.enqueueLocked(b.playPayload(b.current))
		} else {
			b.enqueueLocked(map[string]any{"event": "stop"})
		}
	})
	view.Page().OnRenderProcessGone(func(reason string) {
		b.mu.Lock()
		b.ready = false
		b.mu.Unlock()
		log.Printf("[native-player] renderer gone: %s", reason)
	})
}
func (b *PlayerBridge) Play(seriesID, episodeID string, episodeNum int, resumeAt float64, autoplay bool) {
	if b == nil || seriesID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.playLocked(seriesID, episodeID, episodeNum, resumeAt, autoplay)
}
func (b *PlayerBridge) playLocked(seriesID, episodeID string, num int, resumeAt float64, autoplay bool) {
	if num <= 0 {
		num = 1
	}
	b.token++
	token := b.token
	meta := b.metadata
	if meta.ID != seriesID {
		meta = seriesMetadata{ID: seriesID}
	}
	b.current = PlaybackState{SeriesID: seriesID, EpisodeID: episodeID, EpisodeNum: num, Position: resumeAt, Playing: autoplay, Session: token, Title: meta.Title, CoverURL: meta.CoverURL, TotalEpisodes: meta.TotalEpisodes}
	provider := b.playbackProvider()
	if provider == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		b.mu.Lock()
		quality := b.current.Quality
		b.mu.Unlock()
		playback, err := provider.PlayEpisode(ctx, seriesID, episodeID, num, quality, false)
		b.mu.Lock()
		defer b.mu.Unlock()
		if token != b.token {
			return
		}
		if err != nil {
			b.enqueueLocked(map[string]any{"event": "error", "message": err.Error()})
			return
		}
		if playback == nil {
			return
		}
		requested := b.current.Quality
		quality = playback.Quality
		if requested != "" {
			quality = requested
		} else if quality == "" {
			quality = b.current.Quality
		}
		state := b.current
		state.EpisodeID, state.EpisodeNum, state.StreamURL = playback.EpisodeID, playback.EpisodeNum, playback.StreamURL
		state.Position = playback.LastPosition
		if resumeAt > 0 {
			state.Position = resumeAt
		}
		state.Duration = float64(playback.DurationSec)
		state.Quality = quality
		if state.Title == "" {
			state.Title = playback.Title
		}
		if state.CoverURL == "" {
			state.CoverURL = playback.PosterURL
		}
		b.current = state
		b.sessions[token] = state
		for old := range b.sessions {
			if old < token-32 {
				delete(b.sessions, old)
			}
		}
		b.enqueueLocked(b.playPayload(state))
	}()
}
func (b *PlayerBridge) playbackProvider() playbackProvider {
	if b.playback != nil {
		return b.playback
	}
	if b.service != nil {
		return b.service
	}
	return nil
}
func (b *PlayerBridge) playPayload(s PlaybackState) map[string]any {
	return map[string]any{"event": "play", "seriesId": s.SeriesID, "episodeId": s.EpisodeID, "session": s.Session, "src": s.StreamURL, "resumeAt": s.Position, "autoplay": s.Playing, "poster": s.CoverURL}
}
func (b *PlayerBridge) Pause() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current.Playing = false
	b.enqueueLocked(map[string]any{"event": "pause"})
}
func (b *PlayerBridge) Resume() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current.Playing = true
	b.enqueueLocked(map[string]any{"event": "resume"})
}
func (b *PlayerBridge) Toggle() {
	if b.state().Playing {
		b.Pause()
	} else {
		b.Resume()
	}
}
func (b *PlayerBridge) Previous()       { b.shift(-1) }
func (b *PlayerBridge) Next()           { b.shift(1) }
func (b *PlayerBridge) shift(delta int) { b.mu.Lock(); defer b.mu.Unlock(); b.shiftLocked(delta) }
func (b *PlayerBridge) shiftLocked(delta int) {
	s := b.current
	next := s.EpisodeNum + delta
	if s.SeriesID == "" || next < 1 || (s.TotalEpisodes > 0 && next > s.TotalEpisodes) {
		return
	}
	b.playLocked(s.SeriesID, "", next, 0, s.Playing)
}
func (b *PlayerBridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.token++
	b.current = PlaybackState{}
	b.enqueueLocked(map[string]any{"event": "stop"})
}
func (b *PlayerBridge) Seek(position float64) {
	if !finite(position) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current.Position = math.Max(0, position)
	b.enqueueLocked(map[string]any{"event": "seek", "position": b.current.Position})
}

func (b *PlayerBridge) SelectQuality(quality string) {
	if quality == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.current
	if s.SeriesID == "" || s.Quality == quality {
		return
	}
	b.current.Quality = quality
	b.playLocked(s.SeriesID, s.EpisodeID, s.EpisodeNum, s.Position, s.Playing)
	b.current.Quality = quality
}

func (b *PlayerBridge) SelectSpeed(speed float64) {
	allowed := false
	for _, value := range []float64{.75, 1, 1.25, 1.5, 2} {
		if speed == value {
			allowed = true
			break
		}
	}
	if !allowed {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.current.Speed == speed {
		return
	}
	b.current.Speed = speed
	b.enqueueLocked(map[string]any{"event": "speed", "value": speed})
}
func (b *PlayerBridge) PictureInPicture() {
	b.mu.RLock()
	page := b.page
	b.mu.RUnlock()
	if page != nil {
		page.TogglePictureInPicture()
	}
}
func (b *PlayerBridge) Progress(position, duration float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current.Position = position
	b.current.Duration = duration
}
func (b *PlayerBridge) matches(p progressPayload) bool {
	s := b.current
	return p.Session == s.Session && p.SeriesID == s.SeriesID && p.EpisodeID == s.EpisodeID && s.StreamURL != ""
}
func (b *PlayerBridge) ReportProgress(p progressPayload) {
	if !finite(p.Position) || !finite(p.Duration) {
		return
	}
	b.mu.Lock()
	if !b.matches(p) {
		b.mu.Unlock()
		return
	}
	b.current.Position, b.current.Duration, b.current.Playing = p.Position, p.Duration, p.Playing
	b.enqueueLocked(nil)
	b.mu.Unlock()
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func (b *PlayerBridge) SaveProgress(p progressPayload) error {
	if p.SeriesID == "" || p.EpisodeID == "" || p.Duration <= 0 || !finite(p.Position) || !finite(p.Duration) {
		return nil
	}
	b.saveMu.Lock()
	defer b.saveMu.Unlock()
	b.mu.RLock()
	state, ok := b.sessions[p.Session]
	saver := b.saver
	if saver == nil && b.service != nil {
		saver = b.service
	}
	b.mu.RUnlock()
	if !ok || saver == nil || state.SeriesID != p.SeriesID || state.EpisodeID != p.EpisodeID || p.Session < b.saved[p.SeriesID] {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := saver.SaveProgress(ctx, p.SeriesID, p.EpisodeID, state.Title, state.CoverURL, state.EpisodeNum, state.TotalEpisodes, math.Max(0, p.Position), p.Duration)
	if err == nil {
		b.saved[p.SeriesID] = p.Session
	}
	return err
}
func (b *PlayerBridge) MediaCommand(command string, position float64) {
	switch command {
	case "play":
		b.Resume()
	case "pause":
		b.Pause()
	case "prev":
		b.Previous()
	case "next":
		b.Next()
	case "seek":
		b.Seek(position)
	case "pip":
		b.PictureInPicture()
	default:
		b.Toggle()
	}
}
func (b *PlayerBridge) MediaCommandPayload(p native.MediaCommandPayload) {
	b.MediaCommand(p.Command, p.Position)
}
func (b *PlayerBridge) state() PlaybackState { b.mu.RLock(); defer b.mu.RUnlock(); return b.current }

func (b *PlayerBridge) commandsForTest() []playerCommand {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]playerCommand(nil), b.commands...)
}

// Commands are queued in state order. Never hold a UI-state lock while waiting for Page.Eval.
func (b *PlayerBridge) enqueueLocked(payload map[string]any) {
	b.commands = append(b.commands, playerCommand{b.token, payload})
	if !b.draining {
		b.draining = true
		go b.drain()
	}
}
func playerScript(payload map[string]any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "window.nativePlayerEvent && window.nativePlayerEvent(" + string(data) + ")", nil
}
func (b *PlayerBridge) drain() {
	for {
		b.mu.Lock()
		if len(b.commands) == 0 {
			b.draining = false
			b.mu.Unlock()
			return
		}
		cmd := b.commands[0]
		b.commands = b.commands[1:]
		page, ready, current, fn := b.page, b.ready, b.current, b.invalidateFn
		valid := cmd.session == b.token
		b.mu.Unlock()
		if !valid {
			continue
		}
		if page != nil && ready && cmd.payload != nil {
			code, err := playerScript(cmd.payload)
			if err == nil {
				_, err = page.Eval(code)
			}
			if err != nil {
				log.Printf("[native-player] command failed: %v", err)
			}
		}
		if fn != nil {
			fn()
		}
		if !b.testMode {
			b.updateNowPlaying(current)
		}
	}
}
func (b *PlayerBridge) updateNowPlaying(s PlaybackState) {
	if s.SeriesID == "" {
		mygo.NowPlaying.Clear()
		return
	}
	state := mygo.PlaybackPaused
	if s.Playing {
		state = mygo.PlaybackPlaying
	}
	mygo.NowPlaying.Set(mygo.NowPlayingInfo{Title: fmt.Sprintf("%s · 第%d集", s.Title, s.EpisodeNum), Album: s.SeriesID, Duration: time.Duration(s.Duration * float64(time.Second)), Position: time.Duration(s.Position * float64(time.Second)), State: state, Commands: mygo.MediaPlay | mygo.MediaPause | mygo.MediaNext | mygo.MediaPrevious | mygo.MediaSeek})
}
