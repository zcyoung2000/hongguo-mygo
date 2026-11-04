package library

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	userstore "myguo/internal/store"
)

func TestConcurrentLikeIntentsStayConsistentWithRemote(t *testing.T) {
	firstStarted, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	var mu sync.Mutex
	var actions []float64
	cloudLiked := false
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		action := decodeGZIP(t, raw)["action_type"].(float64)
		mu.Lock()
		actions = append(actions, action)
		first := len(actions) == 1
		mu.Unlock()
		if first {
			close(firstStarted)
			<-release
		}
		mu.Lock()
		cloudLiked = action == 3
		mu.Unlock()
		writeJSON(w, map[string]any{"code": 0})
	}))
	done := make(chan error, 2)
	toggle := func() {
		_, err := service.ToggleLike(context.Background(), "710000000000001", "710000000000101", "Test", "", 1)
		done <- err
	}
	go toggle()
	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first mutation did not reach remote")
	}
	go toggle()
	// Keep the first request pending while the second intent is submitted.
	// The final remote action must be unlike, regardless of response timing.
	time.Sleep(25 * time.Millisecond)
	unblock.Do(func() { close(release) })
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("like mutation did not finish")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if cloudLiked || store.IsLiked("710000000000001", "710000000000101", 1) || !reflect.DeepEqual(actions, []float64{3, 4}) {
		t.Fatalf("diverged intent: cloud=%v actions=%v", cloudLiked, actions)
	}
}

func TestLikeAcknowledgmentSetsTargetAfterConcurrentRefresh(t *testing.T) {
	var store *userstore.Store
	service, db := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := store.SetLike(LikeItem{SeriesID: "710000000000001", EpisodeID: "710000000000101", EpisodeNum: 1}, true); err != nil {
			t.Error(err)
		}
		writeJSON(w, map[string]any{"code": 0})
	}))
	store = db
	liked, err := service.ToggleLike(context.Background(), "710000000000001", "710000000000101", "Test", "", 1)
	if err != nil || !liked || !store.IsLiked("710000000000001", "710000000000101", 1) {
		t.Fatalf("acknowledgment toggled a refreshed row: liked=%v err=%v", liked, err)
	}
}

func TestCloseCancelsBackgroundSyncAndKeepsPendingIntent(t *testing.T) {
	started := make(chan struct{})
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	if _, err := service.ToggleFavorite(context.Background(), "710000000000001", "Test", "", "", 1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background sync did not start")
	}
	done := make(chan struct{})
	go func() { service.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel and drain background sync")
	}
	items, err := store.PendingFavoriteOps(10)
	if err != nil || len(items) != 1 || !items[0].Added {
		t.Fatalf("canceled mutation was acknowledged: %v, %v", items, err)
	}
}
