package uiapp

import (
	"time"

	"github.com/egoist/mygo/ui"
)

var (
	artworkFade             = ui.Animation{Duration: 320 * time.Millisecond, NonMotion: true}
	cardSpring              = ui.Snappy
	feedbackAnimationValues = ui.Animation{Duration: 120 * time.Millisecond}
)

// hoverBackground always names the idle target first, so a pointer leaving
// interrupts toward the correct color instead of freezing at the hovered one.
func hoverBackground(element ui.Element, idle, hovered ui.Color) ui.Element {
	background := element.AnimateColor("background", idle, feedbackAnimationValues)
	if element.Hovered() {
		background = element.AnimateColor("background", hovered, feedbackAnimationValues)
	}
	element.Background(background)
	pose := ui.Pose{}
	if element.Pressed() {
		pose.Scale = 0.98
	}
	element.AnimatePose(pose, cardSpring)
	return element
}
