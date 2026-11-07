package uiapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestHoverFeedbackUsesAnimateColorAndSnappyPress(t *testing.T) {
	if feedbackAnimationValues.Duration != 120*time.Millisecond {
		t.Fatal("hover retargeting must use the 120ms feedback token")
	}
	if cardSpring.Spring.Bounce != ui.Snappy.Spring.Bounce || cardSpring.Spring.Duration != ui.Snappy.Spring.Duration {
		t.Fatal("press feedback must use the MyGo Snappy spring")
	}
	idle, hovered := ui.Transparent, ui.RGBA(255, 255, 255, 0.08)
	element := hoverBackground(ui.Box(nil), idle, hovered)
	if element.Hovered() {
		t.Fatal("a new hover helper must not synthesize hover state")
	}
}

func TestPlayerDrawerUsesSymmetricRightSideTransition(t *testing.T) {
	app := NewTestApp(nil)
	app.SetMediaHost(&MediaHost{})
	app.PlayDetail()
	app.mu.Lock()
	app.mediaOn = true
	app.playerChrome = PlayerChrome{
		Visible: true, ControlsVisible: true, Episode: 1, TotalEpisodes: 2,
		Quality: ControlFrame{X: 500, Y: 700, W: 52, H: 28},
		Speed:   ControlFrame{X: 560, Y: 700, W: 52, H: 28},
		Drawer:  ControlFrame{X: 760, Y: 700, W: 72, H: 28},
	}
	app.playerChromeKnown = true
	app.playerDrawerOpen = true
	app.mu.Unlock()

	tester := ui.NewTester(app.Render, 1000, 720)
	tester.Frame()
	drawer, visible := tester.Find("选集播放")
	if !visible {
		t.Fatal("drawer did not enter")
	}
	if drawer.X+drawer.W <= float32(1000-playerDrawerMaxWidth) {
		t.Fatalf("drawer lost its right-side geometry: %+v", drawer)
	}

	app.closePlayerDrawer()
	tester.Frame()
	tester.Advance(240 * time.Millisecond)
	tester.Frame()
	if tester.HasText("选集播放") {
		t.Fatal("drawer exit animation did not settle")
	}
	if tester.FramePending() {
		t.Fatal("closed drawer still requested rebuild frames")
	}
	if drawerTransition.Duration != 240*time.Millisecond || !drawerTransition.Position || drawerTransition.Size {
		t.Fatal("drawer transition must be symmetric 240ms position motion")
	}
}

func TestInteractionTransitionsSettleWithoutContinuousFrames(t *testing.T) {
	app := NewTestApp(nil)
	app.search.Items = []CatalogItem{{ID: "123456", EpisodeID: "654321", Title: "反馈稳定样本"}}
	tester := ui.NewTester(app.Render, 1000, 800)
	tester.Move(320, 240)
	tester.Frame()
	tester.Advance(300 * time.Millisecond)
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("hover and press feedback requested continuous rebuild frames")
	}
}

func TestArtworkRevealStaysPaintOnly(t *testing.T) {
	if artworkFade.Duration != 320*time.Millisecond || !artworkFade.NonMotion {
		t.Fatal("artwork reveal must remain a 320ms non-motion fade")
	}
}
