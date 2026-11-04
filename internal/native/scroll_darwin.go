//go:build darwin

package native

import (
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

var (
	scrollGestureMutex sync.Mutex
	scrollState        nativeScrollState
	scrollMonitor      objc.ID
	scrollMonitorBlock objc.Block
)

func StartScrollGestureMonitor() {
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
		scrollMonitorReady.Store(true)
	})
}

func handleNativeScrollEvent(phase, momentumPhase int, rawDeltaY float64, precise bool) {
	scrollGestureMutex.Lock()
	defer scrollGestureMutex.Unlock()

	if !feedScrollCapture.Load() {
		scrollState = nativeScrollState{}
		return
	}

	direction := scrollState.consume(phase, momentumPhase, rawDeltaY, precise, time.Now())
	if direction != "" && window != nil {
		_ = scrollGestureEvent.Emit(window, ScrollGesturePayload{Direction: direction})
	}
}
