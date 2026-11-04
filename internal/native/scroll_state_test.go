package native

import (
	"testing"
	"time"
)

func TestTrackpadAccumulatesAppKitChangedPhaseOncePerGesture(t *testing.T) {
	var state nativeScrollState
	now := time.Now()
	// Literal values intentionally follow NSEvent.h, independently of the
	// implementation constants. Slow swipes contain many small Changed events.
	state.consume(32, 0, 0, true, now)
	state.consume(1, 0, -2, true, now)
	var directions []string
	for i := 1; i <= 100; i++ {
		if direction := state.consume(4, 0, -2, true, now.Add(time.Duration(i)*10*time.Millisecond)); direction != "" {
			directions = append(directions, direction)
		}
	}
	state.consume(8, 0, 0, true, now.Add(time.Second))
	for i := 1; i <= 20; i++ {
		if direction := state.consume(0, 4, -20, true, now.Add(time.Second+time.Duration(i)*10*time.Millisecond)); direction != "" {
			t.Fatalf("momentum generated navigation %s", direction)
		}
	}
	if len(directions) != 1 || directions[0] != "down" {
		t.Fatalf("one physical downward gesture produced %v", directions)
	}
	state.consume(1, 0, 0, true, now.Add(2*time.Second))
	if got := state.consume(4, 0, 70, true, now.Add(2*time.Second)); got != "up" {
		t.Fatalf("next physical gesture = %q", got)
	}
}

func TestTrackpadStationaryEndAndCancellation(t *testing.T) {
	now := time.Now()
	var state nativeScrollState
	state.consume(1, 0, -40, true, now)
	state.consume(2, 0, 0, true, now)
	if got := state.consume(8, 0, -30, true, now); got != "down" || state.active {
		t.Fatalf("final sample was lost or gesture was not reset: %q %+v", got, state)
	}
	state.consume(1, 0, -40, true, now)
	if got := state.consume(16, 0, -100, true, now); got != "" || state.active {
		t.Fatalf("cancelled gesture navigated: %q %+v", got, state)
	}
	state.consume(1, 0, -20, true, now)
	if got := state.consume(4, 0, -50, true, now); got != "down" {
		t.Fatalf("cancelled gesture prevented a new gesture: %q", got)
	}
}

func TestMouseWheelGroupsNotchesWithoutRepeatingMomentum(t *testing.T) {
	now := time.Now()
	var state nativeScrollState
	for i, want := range []string{"down", "", "down"} {
		if got := state.consume(0, 0, -3, false, now.Add(time.Duration(i*i)*100*time.Millisecond)); got != want {
			t.Fatalf("wheel sample %d = %q, want %q", i, got, want)
		}
	}
}
