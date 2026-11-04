package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCloseCancelsThenWaitsAndRejectsNewWork(t *testing.T) {
	var group Group
	canceled, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		group.Close()
	})
	group.Go(func(ctx context.Context) {
		<-ctx.Done()
		close(canceled)
		<-release
	})
	done := make(chan struct{})
	go func() { group.Close(); close(done) }()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("task did not receive cancellation")
	}
	select {
	case <-done:
		t.Fatal("close returned before the task drained")
	default:
	}
	if _, _, err := group.Acquire(); !errors.Is(err, context.Canceled) {
		t.Fatalf("work admitted after stop: %v", err)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not finish after task exit")
	}
	group.Close()
}
