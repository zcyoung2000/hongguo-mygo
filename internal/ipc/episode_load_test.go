package ipc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestEpisodeLoadBroadcastsToAllPlaybackAndDetailWaiters(t *testing.T) {
	for _, wantErr := range []error{nil, errors.New("unavailable")} {
		load := &episodeLoad{done: make(chan struct{})}
		svc := &HongguoService{episodeLoads: map[string]*episodeLoad{"series": load}}
		want := &sdk.Page[model.Episode]{Items: []model.Episode{{ID: "episode", Number: 1}}}
		var waiters sync.WaitGroup
		for i := 0; i < 32; i++ {
			waiters.Add(1)
			go func() {
				defer waiters.Done()
				page, err := svc.listEpisodesCached(context.Background(), "series")
				if page != want || err != wantErr {
					t.Errorf("shared episodes = %p/%v, want %p/%v", page, err, want, wantErr)
				}
			}()
		}
		load.result = episodeListResult{page: want, err: wantErr}
		close(load.done)
		waiters.Wait()
	}
}

func TestPlaybackEpisodeWaitCanTimeoutWithoutConsumingSharedResult(t *testing.T) {
	load := &episodeLoad{done: make(chan struct{})}
	svc := &HongguoService{episodeLoads: map[string]*episodeLoad{"series": load}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := svc.listEpisodesCached(ctx, "series")
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("canceled playback waiter = %v", err)
		}
	case <-time.After(time.Second):
		close(load.done)
		t.Fatal("playback waited past its own deadline for a background detail load")
	}
	load.result.page = &sdk.Page[model.Episode]{Items: []model.Episode{{ID: "episode"}}}
	close(load.done)
	if page, err := svc.listEpisodesCached(context.Background(), "series"); page != load.result.page || err != nil {
		t.Fatalf("later detail waiter lost shared result: %p/%v", page, err)
	}
}
