//go:build darwin

package main

import (
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

type nativeScrollState struct {
	active    bool
	fired     bool
	sum       float64
	lastMouse time.Time
}

var (
	scrollGestureMutex sync.Mutex
	scrollState        nativeScrollState
	scrollMonitor      objc.ID
	scrollMonitorBlock objc.Block
)

const nativeScrollThreshold = 65.0

func startScrollGestureMonitor() {
	mygo.RunOnMain(func() {
		if scrollMonitor != 0 {
			return
		}

		nsevent := objc.ID(objc.GetClass("NSEvent"))
		selMonitor := objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:")
		selRetain := objc.RegisterName("retain")
		selPhase := objc.RegisterName("phase")
		selMomentumPhase := objc.RegisterName("momentumPhase")
		selDeltaY := objc.RegisterName("scrollingDeltaY")
		selPrecise := objc.RegisterName("hasPreciseScrollingDeltas")

		blk := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
			handleNativeScrollEvent(
				objc.Send[int](event, selPhase),
				objc.Send[int](event, selMomentumPhase),
				objc.Send[float64](event, selDeltaY),
				objc.Send[bool](event, selPrecise),
			)
			return event
		})

		monitor := objc.Send[objc.ID](
			nsevent,
			selMonitor,
			uint64(1<<22),
			uintptr(blk),
		)
		if monitor == 0 {
			return
		}

		scrollMonitorBlock = blk
		scrollMonitor = monitor.Send(selRetain)
	})
}

func handleNativeScrollEvent(phase, momentumPhase int, rawDeltaY float64, precise bool) {
	scrollGestureMutex.Lock()
	defer scrollGestureMutex.Unlock()

	if !feedScrollCapture.Load() {
		scrollState = nativeScrollState{}
		return
	}

	const (
		phaseBegan    = 1
		phaseChanged  = 2
		phaseEnded    = 4
		phaseCanceled = 8
		phaseMayBegin = 16
	)

	if phase&(phaseEnded|phaseCanceled) != 0 {
		scrollState = nativeScrollState{}
		return
	}

	// Momentum belongs to the gesture that already emitted its action.
	if momentumPhase != 0 {
		return
	}

	if phase&phaseBegan != 0 {
		scrollState = nativeScrollState{active: true}
	} else if phase&(phaseChanged|phaseMayBegin) != 0 {
		scrollState.active = true
	}

	// Browser deltaY is positive downward; AppKit has the opposite sign.
	deltaY := -rawDeltaY
	if !precise {
		deltaY *= 40
	}

	now := time.Now()
	if phase == 0 {
		// A conventional mouse wheel has no gesture phase. Separate notches by
		// their cadence so each notch is one action.
		if now.Sub(scrollState.lastMouse) > 120*time.Millisecond {
			scrollState.active = false
			scrollState.fired = false
			scrollState.sum = 0
		}
		scrollState.lastMouse = now
		scrollState.active = true
	}

	if !scrollState.active {
		return
	}

	scrollState.sum += deltaY
	if scrollState.fired || absFloat(scrollState.sum) < nativeScrollThreshold {
		return
	}

	scrollState.fired = true
	if mainWindow != nil {
		direction := "up"
		if scrollState.sum > 0 {
			direction = "down"
		}
		_ = scrollGestureEvent.Emit(mainWindow, ScrollGesturePayload{Direction: direction})
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
