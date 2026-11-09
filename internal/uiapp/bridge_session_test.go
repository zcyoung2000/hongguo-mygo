package uiapp

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"myguo/internal/native"
)

func TestResumeBridgeAcceptsJavaScriptPayloadShape(t *testing.T) {
	const javascript = `{
		"seriesId": "123456",
		"episodeNum": 8,
		"positionSec": 725.25,
		"session": 7,
		"sequence": 1
	}`
	var payload ResumeProgress
	if err := json.Unmarshal([]byte(javascript), &payload); err != nil {
		t.Fatal(err)
	}

	app := NewTestApp()
	app.view = "detail"
	app.detail = Detail{ID: "123456"}
	app.mediaOn = true
	app.mediaSession = 7
	shell := NativeShell{App: app}
	shell.ResumeProgress(payload)

	if app.resume.ID != "123456" || app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 725.25 {
		t.Fatalf("JavaScript resume payload was not accepted: %+v", app.resume)
	}
}

func TestResumeBridgeRejectsStaleAndReplayedSequences(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = Detail{ID: "123456"}
	app.mediaOn = true
	app.mediaSession = 7
	shell := NativeShell{App: app}
	shell.ResumeProgress(ResumeProgress{SeriesID: "123456", EpisodeNum: 8, Position: 100, Session: 7, Sequence: 2})
	if app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 100 {
		t.Fatalf("newer resume sequence rejected: %+v", app.resume)
	}
	shell.ResumeProgress(ResumeProgress{SeriesID: "123456", EpisodeNum: 9, Position: 200, Session: 7, Sequence: 1})
	if app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 100 {
		t.Fatalf("out-of-order sequence changed resume: %+v", app.resume)
	}
	shell.ResumeProgress(ResumeProgress{SeriesID: "123456", EpisodeNum: 10, Position: 300, Session: 7, Sequence: 2})
	if app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 100 {
		t.Fatalf("stale sequence changed resume: %+v", app.resume)
	}
}

type recordingEval struct {
	mu      sync.Mutex
	scripts []string
	events  []string
}

func (r *recordingEval) eval(script string) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts = append(r.scripts, script)
	r.events = append(r.events, script)
	return nil, nil
}

func (r *recordingEval) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.scripts)
}

func (r *recordingEval) dispatched() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type failingEval struct {
	calls int
}

func (e *failingEval) eval(string) (any, error) {
	e.calls++
	return nil, errors.New("webview unavailable")
}

func TestMediaSceneReplacementCancelsFailedPayloadRetry(t *testing.T) {
	evaluator := &failingEval{}
	host := &MediaHost{ready: true, eval: evaluator.eval}
	host.Play("123456", 1, -1, 1)
	waitFor(250*time.Millisecond, func() bool { return evaluator.calls > 0 })
	host.Stop()
	host.Play("223456", 2, -1, 2)
	calls := evaluator.calls
	time.Sleep(250 * time.Millisecond)
	if evaluator.calls > calls {
		t.Fatalf("replaced scene was retried: before=%d after=%d", calls, evaluator.calls)
	}
	if host.desired["event"] != "play" || host.desired["seriesId"] != "223456" {
		t.Fatalf("new scene was lost: %+v", host.desired)
	}
}

func TestMediaCommandWaitsForReadyAndSurvivesSceneReplacement(t *testing.T) {
	evaluator := &recordingEval{}
	host := &MediaHost{view: &mygo.WebView{}, eval: evaluator.eval}
	host.PlayerControl("seek", 12.5)
	host.mu.Lock()
	pendingBeforeReady := len(host.pendingCommands)
	host.mu.Unlock()
	if pendingBeforeReady != 1 {
		t.Fatalf("command was not retained before ready: %d", pendingBeforeReady)
	}
	host.Stop()
	host.markReady()
	waitFor(250*time.Millisecond, func() bool { return evaluator.count() == 2 })
	if evaluator.count() != 2 {
		t.Fatalf("ready messages were not dispatched: %d", evaluator.count())
	}
	dispatched := strings.Join(evaluator.dispatched(), "\n")
	if !strings.Contains(dispatched, `"event":"stop"`) || !strings.Contains(dispatched, `"seek"`) {
		t.Fatalf("scene and command payload missing: %s", dispatched)
	}
}

func TestMediaCommandsDispatchExactlyOnce(t *testing.T) {
	evaluator := &recordingEval{}
	host := &MediaHost{ready: true, view: &mygo.WebView{}, eval: evaluator.eval}
	for range 5 {
		host.startLocked()
	}
	host.Command(native.MediaCommandPayload{Command: "pause"})
	waitFor(250*time.Millisecond, func() bool { return evaluator.count() == 1 })
	if evaluator.count() != 1 {
		t.Fatalf("command was not dispatched: %d", evaluator.count())
	}
	time.Sleep(30 * time.Millisecond)
	if evaluator.count() != 1 {
		t.Fatalf("command dispatched more than once: %d", evaluator.count())
	}
	host.mu.Lock()
	remaining := len(host.queue)
	host.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("successful command remained queued: %d", remaining)
	}
}
