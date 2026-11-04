package native

import "time"

// NSEventPhase is a bit mask; Stationary occupies bit 1 (AppKit/NSEvent.h).
const (
	scrollPhaseBegan      = 1 << 0
	scrollPhaseStationary = 1 << 1
	scrollPhaseChanged    = 1 << 2
	scrollPhaseEnded      = 1 << 3
	scrollPhaseCanceled   = 1 << 4
	scrollPhaseMayBegin   = 1 << 5
	nativeScrollThreshold = 65.0
)

type nativeScrollState struct {
	active    bool
	fired     bool
	sum       float64
	lastEvent time.Time
}

func (s *nativeScrollState) consume(phase, momentumPhase int, rawDeltaY float64, precise bool, now time.Time) string {
	// An inertial tail never starts another navigation, including after Ended.
	if momentumPhase != 0 {
		return ""
	}
	if phase&scrollPhaseCanceled != 0 {
		*s = nativeScrollState{}
		return ""
	}
	if phase&scrollPhaseBegan != 0 || (phase == 0 && now.Sub(s.lastEvent) > 180*time.Millisecond) {
		*s = nativeScrollState{active: true}
	} else if phase&(scrollPhaseChanged|scrollPhaseMayBegin) != 0 || phase == 0 {
		s.active = true
	}
	s.lastEvent = now
	delta := -rawDeltaY
	if !precise {
		delta *= 40
	}
	direction := ""
	if s.active && !s.fired {
		s.sum += delta
		if s.sum >= nativeScrollThreshold {
			direction = "down"
		} else if s.sum <= -nativeScrollThreshold {
			direction = "up"
		}
		if direction != "" {
			s.fired = true
		}
	}
	if phase&(scrollPhaseEnded|scrollPhaseCanceled) != 0 {
		*s = nativeScrollState{}
	}
	return direction
}
