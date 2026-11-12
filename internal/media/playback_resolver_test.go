package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestPlaybackRacePrimarySuccessCancelsSpare(t *testing.T) {
	spareStarted := make(chan struct{})
	spareCanceled := make(chan struct{})
	primary := fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		return playbackResult("primary"), nil
	}}
	spare := fakePlaybackResolver{func(ctx context.Context, _ string) (*playbackResolveResult, error) {
		close(spareStarted)
		<-ctx.Done()
		close(spareCanceled)
		return nil, ctx.Err()
	}}

	result, err := newPlaybackRaceResolver(primary, spare).ResolvePlayback(context.Background(), "71001")
	if err != nil {
		t.Fatalf("ResolvePlayback err = %v", err)
	}
	if result.Playback.VideoID != "primary" {
		t.Fatalf("winner = %s", result.Playback.VideoID)
	}
	select {
	case <-spareCanceled:
	case <-time.After(time.Second):
		t.Fatal("spare source was not canceled")
	}
}

func TestPlaybackRaceSlowPrimaryLosesToSpare(t *testing.T) {
	primary := fakePlaybackResolver{func(ctx context.Context, _ string) (*playbackResolveResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return playbackResult("primary"), nil
		}
	}}
	spare := fakePlaybackResolver{func(ctx context.Context, _ string) (*playbackResolveResult, error) {
		return playbackResult("spare"), nil
	}}

	result, err := newPlaybackRaceResolver(primary, spare).ResolvePlayback(context.Background(), "71001")
	if err != nil {
		t.Fatalf("ResolvePlayback err = %v", err)
	}
	if result.Playback.VideoID != "spare" {
		t.Fatalf("winner = %s", result.Playback.VideoID)
	}
}

func TestPlaybackRaceEmptyPrimaryAllowsSwitchAndAggregatesAllFailures(t *testing.T) {
	primary := fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		return nil, sdk.ErrAnonymousLimited
	}}
	spare := fakePlaybackResolver{func(ctx context.Context, _ string) (*playbackResolveResult, error) {
		select {
		case <-time.After(50 * time.Millisecond):
			return nil, fmt.Errorf("spare unavailable")
		case <-ctx.Done():
			return nil, fmt.Errorf("spare canceled")
		}
	}}

	_, err := newPlaybackRaceResolver(primary, spare).ResolvePlayback(context.Background(), "71001")
	if err == nil {
		t.Fatal("ResolvePlayback err = nil")
	}
	if !strings.Contains(err.Error(), "video_model") || !strings.Contains(err.Error(), "spare unavailable") {
		t.Fatalf("aggregate error = %v", err)
	}
}

func TestPlaybackRaceParamErrorIsNotSwitched(t *testing.T) {
	paramErr := fmt.Errorf("bad body: %w", sdk.ErrParamInvalid)
	primary := fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		return nil, paramErr
	}}
	spare := fakePlaybackResolver{func(ctx context.Context, _ string) (*playbackResolveResult, error) {
		<-ctx.Done()
		return nil, nil
	}}

	_, err := newPlaybackRaceResolver(primary, spare).ResolvePlayback(context.Background(), "71001")
	if !errors.Is(err, sdk.ErrParamInvalid) {
		t.Fatalf("ResolvePlayback err = %v", err)
	}
}

func TestPlaybackFallbackResolverFollowsFallbackAPI(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/fallback" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"video_model": map[string]any{
				"video_id": "fallback-video",
				"video_list": map[string]any{
					"720p": map[string]any{"main_url": serverURL(r) + "/media.mp4", "definition": "720p"},
				},
			}},
		})
	}))
	defer server.Close()

	fallback := fmt.Sprintf("%s/fallback?video_id=71001", server.URL)
	inner := fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		return &playbackResolveResult{Fallback: fallback}, nil
	}}
	resolver := playbackFallbackResolver{inner: inner, fetcher: sdkPlaybackResolver{http: server.Client()}.fetchFallback}

	result, err := resolver.ResolvePlayback(context.Background(), "71001")
	if err != nil {
		t.Fatalf("ResolvePlayback err = %v", err)
	}
	if requests != 1 || result.Playback.VideoID != "fallback-video" || result.Playback.Streams[0].URL == "" {
		t.Fatalf("requests=%d result=%+v", requests, result.Playback)
	}
}

func TestPlaybackFallbackResolverHandlesNilResultWithSwitchableError(t *testing.T) {
	inner := fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		return nil, sdk.ErrAnonymousLimited
	}}
	resolver := playbackFallbackResolver{inner: inner, fetcher: func(context.Context, string) (*playbackResolveResult, error) {
		t.Fatal("fetcher should not be called without a fallback URL")
		return nil, nil
	}}

	result, err := resolver.ResolvePlayback(context.Background(), "71001")
	if result != nil {
		t.Fatalf("result = %+v, want nil", result)
	}
	if !errors.Is(err, sdk.ErrAnonymousLimited) {
		t.Fatalf("ResolvePlayback err = %v", err)
	}
}

func TestParseFallbackAPI(t *testing.T) {
	encoded, err := json.Marshal(map[string]string{"fallback_api": " https://fallback.invalid/api "})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"data": map[string]any{"video_model": string(encoded)}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := parseFallbackAPI(raw)
	if err != nil || got != "https://fallback.invalid/api" {
		t.Fatalf("parseFallbackAPI got %q err %v", got, err)
	}
}

func TestResolvePlaybackDeduplicatesConcurrentEpisodeLoads(t *testing.T) {
	ms := &MediaServer{pbLoads: make(map[string]*playbackLoad)}
	release := make(chan struct{})
	calls := make(chan struct{}, 1)
	ms.playbackResolver = fakePlaybackResolver{func(ctx context.Context, _ string) (*playbackResolveResult, error) {
		calls <- struct{}{}
		select {
		case <-release:
			return playbackResult("merged"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}

	const waiters = 8
	results := make(chan *model.Playback, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			pb, err := ms.resolvePlayback(context.Background(), "71001")
			if err != nil {
				t.Errorf("resolvePlayback err = %v", err)
			}
			results <- pb
		}()
	}
	<-calls
	close(release)
	for i := 0; i < waiters; i++ {
		pb := <-results
		if pb == nil || pb.VideoID != "merged" {
			t.Fatalf("merged playback = %+v", pb)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("resolvePlayback called source %d times", 1+len(calls))
	}
}

func playbackResult(source string) *playbackResolveResult {
	return &playbackResolveResult{Playback: &model.Playback{EpisodeID: "71001", VideoID: source, Streams: []model.Stream{{URL: "https://media.invalid/" + source + ".mp4"}}}}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
