package uiapp

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// FeedChrome mirrors presentation only. The existing document remains the
// authority for recommendation requests, playback and media state.
type FeedChrome struct {
	Visible    bool    `json:"visible"`
	RightInset float32 `json:"rightInset"`
}

var feedChannels = [...]struct{ value, label string }{
	{"关注", "关注"}, {"", "推荐"}, {"漫剧", "漫剧"}, {"真人剧", "真人剧"},
}

func validFeedChannel(value string) bool {
	for _, channel := range feedChannels {
		if channel.value == value {
			return true
		}
	}
	return false
}

func (a *App) SelectFeedChannel(channel string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !validFeedChannel(channel) || a.view != "recommend" || a.mediaOn || a.feedChannel == channel {
		return
	}
	a.feedChannel = channel
	if a.media != nil {
		a.media.RecommendChannel(channel)
	}
	a.invalidate()
}

func (s *NativeShell) FeedChromeChanged(value FeedChrome) {
	if math.IsNaN(float64(value.RightInset)) || math.IsInf(float64(value.RightInset), 0) {
		return
	}
	value.RightInset = max(0, min(value.RightInset, 2000))
	a := s.App
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.feedChromeKnown && a.feedChrome == value {
		return
	}
	a.feedChrome, a.feedChromeKnown = value, true
	a.invalidate()
}

func (a *App) focusMediaAfterPointer() {
	a.mu.Lock()
	host, test := a.media, a.testMode
	a.mu.Unlock()
	if host != nil && !test {
		if view := host.WebView(); view != nil {
			view.Focus()
		}
	}
}

func (a *App) renderFeedChannels(c *ui.Context) {
	if a.feedChromeKnown && !a.feedChrome.Visible {
		return
	}
	selected := 1
	for index, channel := range feedChannels {
		if channel.value == a.feedChannel {
			selected = index
		}
	}
	// A transparent absolute row leaves the full WebView in place. Only the
	// actual segmented control intercepts input, including over the native view.
	ui.Row(c.Key("feed-channel-overlay")).Absolute().Top(14).Left(0).Right(a.feedChrome.RightInset).Justify(ui.Center).Children(func() {
		parts := ui.SegmentedBase(c.Key("feed-channels"), &selected, len(feedChannels))
		track := parts.Track.Label("推荐频道").Padding(4, 6).Gap(4).Radius(24).
			Material(glass.Glass{Style: glass.Regular}).
			OnChange(func() { go a.SelectFeedChannel(feedChannels[selected].value) })

		if a.ambientActive {
			track.Border(1, a.ambientTop.Alpha(0.40)).
				Shadow(0, 8, 24, 0, a.ambientTop.Alpha(0.25))
		} else {
			track.Border(1, ui.RGBA(255, 255, 255, 0.12)).
				Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.32))
		}

		track.Children(func() {
			for index, channel := range feedChannels {
				idx := index
				ch := channel
				isSelected := idx == selected
				segment := parts.Segment(idx).Label(ch.label).Padding(0, 18).Height(32).Radius(18).
					Transition(ui.ElementTransition{Duration: 160 * time.Millisecond, Colors: true})

				if !isSelected {
					hoverBackground(segment, ui.Transparent, ui.RGBA(255, 255, 255, 0.08))
				} else {
					segment.Background(ui.Transparent)
				}

				segment.OnClick(func() {
					// Mouse users retain video shortcuts after choosing a channel;
					// keyboard users keep focus in the native radio group.
					if !segment.FocusVisible() {
						go a.focusMediaAfterPointer()
					}
				})

				segment.Children(func() {
					// 1. In-flow sizer: guarantees segment width is stable and never collapses
					ui.Row(c.Key("feed-sizer:" + ch.value)).Opacity(0).Children(func() {
						ui.Text(c, ch.label).FontSize(13).Bold().SingleLine()
					})
					// 2. Pure background sliding glider: slides across channels without moving the text
					if isSelected {
						ui.Box(c.Key("feed-pill-active")).Absolute().Fill().Radius(18).
							LinearGradient(ui.LinearGradient{
								From:  ui.Hex("#d91b42"),
								To:    ui.Hex("#b81235"),
								Angle: 135,
							}).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.35)).
							Transition(ui.ElementTransition{
								Duration: 240 * time.Millisecond,
								Ease:     ui.CubicBezier(0.25, 1, 0.5, 1),
								Match:    slidingTabMatch{group: "feed-channels"},
							})
					}
					// 3. Stationary text label: stays in place over the glider, only cross-fading color
					ui.Row(c.Key("feed-label:" + ch.value)).Absolute().Fill().
						AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						txt := ui.Text(c, ch.label).FontSize(13).SingleLine().
							Transition(ui.ElementTransition{Duration: 180 * time.Millisecond, Colors: true})
						if isSelected {
							txt.Bold().TextColor(ui.Hex("#ffffff"))
						} else {
							txt.TextColor(ui.RGBA(255, 255, 255, 0.75))
						}
					})
				})
			}
		})
	})
}
