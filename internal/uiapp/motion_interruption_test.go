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

func TestInteractionTransitionsSettleWithoutContinuousFrames(t *testing.T) {
	app := NewTestApp()
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
