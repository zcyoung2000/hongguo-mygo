package media

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestPlaybackLoadBroadcastsResultToEveryWaiter(t *testing.T) {
	for _, resolveErr := range []error{nil, errors.New("source unavailable")} {
		load := &playbackLoad{done: make(chan struct{})}
		ms := &MediaServer{pbLoads: map[string]*playbackLoad{"episode": load}}
		want := &model.Playback{Streams: []model.Stream{{URL: "https://media.invalid/video"}}}
		var waiters sync.WaitGroup
		for i := 0; i < 32; i++ {
			waiters.Add(1)
			go func() {
				defer waiters.Done()
				pb, err := ms.resolvePlayback(context.Background(), "episode")
				if pb != want || err != resolveErr {
					t.Errorf("shared resolve = %p/%v, want %p/%v", pb, err, want, resolveErr)
				}
			}()
		}
		load.result = playbackLoadResult{pb: want, err: resolveErr}
		close(load.done)
		waiters.Wait()
	}
}

func TestPlaybackLoadWaiterCanCancelIndependently(t *testing.T) {
	load := &playbackLoad{done: make(chan struct{})}
	ms := &MediaServer{pbLoads: map[string]*playbackLoad{"episode": load}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ms.resolvePlayback(ctx, "episode"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter = %v", err)
	}
	// Canceling this waiter must not consume or cancel the shared result.
	load.result.pb = &model.Playback{}
	close(load.done)
	if pb, err := ms.resolvePlayback(context.Background(), "episode"); pb != load.result.pb || err != nil {
		t.Fatalf("next waiter = %p/%v", pb, err)
	}
}
