//go:build darwin

package native

import (
	"log"
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
		selType := objc.RegisterName("type")
		selKeyCode := objc.RegisterName("keyCode")
		selRepeat := objc.RegisterName("isARepeat")

		blk := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
			if objc.Send[uint64](event, selType) == 10 { // NSEventTypeKeyDown
				// Observe only feed navigation keys; leave the native event intact.
				// This distinguishes AppKit receipt from WebView/JS handling.
				code := objc.Send[uint16](event, selKeyCode)
				if feedScrollCapture.Load() && (code == 125 || code == 126) && !objc.Send[bool](event, selRepeat) {
					log.Printf("[feed-input] native-key keyCode=%d repeat=false", code)
				}
				return event
			}
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
			uint64(1<<22|1<<10), // scroll wheel and key down
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
		emitScrollGesture(ScrollGesturePayload{Direction: direction})
	}
}
