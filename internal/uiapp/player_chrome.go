package uiapp

import (
	"math"

	"github.com/egoist/mygo/ui"
)

type ControlFrame struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
}

type PlayerQualityOption struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
}

type PlayerChrome struct {
	Visible         bool                  `json:"visible"`
	ControlsVisible bool                  `json:"controlsVisible"`
	Quality         ControlFrame          `json:"quality"`
	Speed           ControlFrame          `json:"speed"`
	Drawer          ControlFrame          `json:"drawer"`
	QualityLabel    string                `json:"qualityLabel"`
	QualityOptions  []PlayerQualityOption `json:"qualityOptions"`
	SpeedValue      float64               `json:"speedValue"`
	Episode         int                   `json:"episode"`
	TotalEpisodes   int                   `json:"totalEpisodes"`
}

func (f ControlFrame) valid() bool {
	for _, value := range []float32{f.X, f.Y, f.W, f.H} {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || value < -1 {
			return false
		}
	}
	return f.W <= 1000 && f.H <= 4000
}

func (s *NativeShell) SelectPlayerQuality(id string)   { s.App.playerControl("quality", id) }
func (s *NativeShell) SelectPlayerSpeed(value float64) { s.App.playerControl("speed", value) }
func (s *NativeShell) SelectPlayerEpisode(value int)   { s.App.playerControl("episode", value) }

func (a *App) playerControl(command string, value any) {
	a.mu.Lock()
	host := a.media
	a.mu.Unlock()
	if host != nil {
		host.PlayerControl(command, value)
	}
}

func (a *App) UpdatePlayerChrome(value PlayerChrome) {
	if !(value.Quality.valid() && value.Speed.valid() && value.Drawer.valid()) {
		return
	}
	value.Quality.X = max(0, min(value.Quality.X, 5000))
	value.Quality.Y = max(0, min(value.Quality.Y, 4000))
	value.Quality.W = max(0, min(value.Quality.W, 1000))
	value.Quality.H = max(0, min(value.Quality.H, 200))
	value.Speed.X = max(0, min(value.Speed.X, 5000))
	value.Speed.Y = max(0, min(value.Speed.Y, 4000))
	value.Speed.W = max(0, min(value.Speed.W, 1000))
	value.Speed.H = max(0, min(value.Speed.H, 200))
	value.Drawer.X = max(0, min(value.Drawer.X, 5000))
	value.Drawer.Y = max(0, min(value.Drawer.Y, 4000))
	value.Drawer.W = max(0, min(value.Drawer.W, 1000))
	value.Drawer.H = max(0, min(value.Drawer.H, 4000))
	value.Episode = max(0, value.Episode)
	value.TotalEpisodes = max(0, value.TotalEpisodes)
	value.SpeedValue = max(0, value.SpeedValue)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playerChromeKnown && playerChromeEqual(a.playerChrome, value) {
		return
	}
	if !value.Visible {
		a.playerDrawerOpen = false
	}
	a.playerChrome, a.playerChromeKnown = value, true
	a.invalidate()
}

func playerChromeEqual(a, b PlayerChrome) bool {
	if a.Visible != b.Visible || a.ControlsVisible != b.ControlsVisible ||
		a.Quality != b.Quality || a.Speed != b.Speed || a.Drawer != b.Drawer ||
		a.QualityLabel != b.QualityLabel || a.SpeedValue != b.SpeedValue ||
		a.Episode != b.Episode || a.TotalEpisodes != b.TotalEpisodes ||
		len(a.QualityOptions) != len(b.QualityOptions) {
		return false
	}
	for index := range a.QualityOptions {
		if a.QualityOptions[index] != b.QualityOptions[index] {
			return false
		}
	}
	return true
}

func (a *App) renderNativePlayerControls(c *ui.Context) {
	chrome := a.playerChrome
	if !chrome.Visible {
		return
	}
	if chrome.ControlsVisible {
		a.renderNativeQualityControl(c, chrome)
		a.renderNativeSpeedControl(c, chrome)
		a.renderNativeEpisodeButton(c, chrome)
	}
	if a.playerDrawerOpen {
		a.renderNativeEpisodeDrawer(c)
		a.renderPlayerDrawerScrim(c)
	}
}

const playerDrawerMaxWidth = 248

func (a *App) renderPlayerDrawerScrim(c *ui.Context) {
	// The drawer opens on top of the media WebView. A transparent native
	// layer is the only way to see a click that would otherwise be sent to
	// the web page, as MyGo routes unclaimed WebView areas to the host.
	scrim := ui.ButtonBase(c).Label("关闭选集背景").Absolute().Top(0).Left(0).Right(playerDrawerMaxWidth).Bottom(0)
	scrimFade := drawerTransition
	scrimFade.Position = false
	scrimFade.Size = false
	scrimFade.Colors = true
	scrimFade.Enter = &ui.Motion{Opacity: 0}
	scrimFade.Exit = &ui.Motion{Opacity: 0}
	scrim.Transition(scrimFade)
	scrim.Background(ui.Transparent).OnClick(a.closePlayerDrawer)
}

func (a *App) nativeControlBox(c *ui.Context, key string, frame ControlFrame) ui.Element {
	return ui.Box(c.Key(key)).Absolute().Top(frame.Y).Left(frame.X).Width(frame.W).Height(frame.H)
}

func (a *App) renderNativeQualityControl(c *ui.Context, chrome PlayerChrome) {
	a.nativeControlBox(c, "native-player-quality", chrome.Quality).Children(func() {
		button := ui.ButtonBase(c).Label("清晰度").Fill().TextColor(c.Theme().Text).
			Background(ui.RGBA(30, 32, 42, .58)).Border(1, ui.RGBA(255, 255, 255, .16)).Radius(7).Shadow(0, 6, 18, 0, ui.RGBA(0, 0, 0, .28))
		button.PopoverMenuButton(func(m *ui.Menu) {
			for _, option := range chrome.QualityOptions {
				item := m.Item(option.Label).Disabled(!option.Available).Radio(option.Active)
				if item.Chosen() {
					go a.playerControl("quality", option.ID)
				}
			}
		})
		button.Children(func() { ui.Text(c, chrome.QualityLabel).FontSize(12).Bold().SingleLine() })
	})
}

func (a *App) renderNativeSpeedControl(c *ui.Context, chrome PlayerChrome) {
	a.nativeControlBox(c, "native-player-speed", chrome.Speed).Children(func() {
		button := ui.ButtonBase(c).Label("倍速").Fill().TextColor(c.Theme().Text).
			Background(ui.RGBA(30, 32, 42, .58)).Border(1, ui.RGBA(255, 255, 255, .16)).Radius(7).Shadow(0, 6, 18, 0, ui.RGBA(0, 0, 0, .28))
		button.PopoverMenuButton(func(m *ui.Menu) {
			for _, speed := range []float64{.75, 1, 1.25, 1.5, 2} {
				item := m.Item(formatPlayerSpeed(speed)).Radio(speed == chrome.SpeedValue)
				if item.Chosen() {
					go a.playerControl("speed", speed)
				}
			}
		})
		button.Children(func() { ui.Text(c, formatPlayerSpeed(chrome.SpeedValue)).FontSize(12).Bold().SingleLine() })
	})
}

func (a *App) renderNativeEpisodeButton(c *ui.Context, chrome PlayerChrome) {
	a.nativeControlBox(c, "native-player-episodes", chrome.Drawer).Children(func() {
		button := ui.ButtonBase(c).Label("选集").Fill().TextColor(c.Theme().Text).
			Background(ui.RGBA(30, 32, 42, .58)).Border(1, ui.RGBA(255, 255, 255, .16)).Radius(7).Shadow(0, 6, 18, 0, ui.RGBA(0, 0, 0, .28))
		button.OnClick(func() {
			a.mu.Lock()
			a.playerDrawerOpen = !a.playerDrawerOpen
			a.mu.Unlock()
			a.focusMediaAfterPointer()
			a.invalidate()
		})
		button.Children(func() {
			ui.Icon(c, episodesIcon).Size(18, 18)
			ui.Text(c, "选集").FontSize(12).Bold().SingleLine()
		})
	})
}

func (a *App) renderNativeEpisodeDrawer(c *ui.Context) {
	chrome := a.playerChrome
	windowWidth, windowHeight := c.Size()
	drawerWidth := min(float32(playerDrawerMaxWidth), max(220, windowWidth*.28))
	drawer := ui.Box(c.Key("native-player-drawer")).Absolute().Top(0).Right(0).
		Width(drawerWidth).Height(windowHeight).
		Background(ui.RGBA(28, 30, 40, .72)).Border(1, ui.RGBA(255, 255, 255, .16)).Shadow(-8, 0, 32, 0, ui.RGBA(0, 0, 0, .35)).
		OnClick(func() {})
	slide := drawerTransition
	slide.Enter = &ui.Motion{X: drawerWidth}
	slide.Exit = &ui.Motion{X: drawerWidth}
	drawer.Transition(slide)
	drawer.Children(func() {
		ui.Column(c).Fill().Padding(14, 16).Gap(12).Children(func() {
			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "选集播放").Bold()
					ui.Textf(c, "共 %d 集", chrome.TotalEpisodes).TextColor(c.Theme().TextMuted)
				})
				close := ui.ButtonBase(c).Label("关闭选集").Focusable().Radius(14).Padding(3, 3).
					Background(ui.RGBA(255, 255, 255, .10)).TextColor(ui.RGBA(255, 255, 255, .78))
				close.OnClick(a.closePlayerDrawer).Children(func() { ui.Icon(c, closeIcon).Size(18, 18) })
			})
			if chrome.TotalEpisodes == 0 {
				ui.Text(c, "暂无剧集").TextColor(c.Theme().TextMuted)
			} else {
				a.playerDrawerGrid.Key = func(index int) any { return index + 1 }
				a.playerDrawerGrid.Label = func(index int) string { return formatEpisode(index + 1) }
				ui.GridView(c, &a.playerDrawerGrid, min(chrome.TotalEpisodes, 12), 55, 28, func(index int) {
					number := index + 1
					if chrome.Episode > 12 && index == 11 {
						number = chrome.Episode
					}
					button := ui.ButtonBase(c).Label(formatEpisode(number)).Fill().Radius(8)
					if number == chrome.Episode {
						button.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
					} else {
						button.Background(ui.RGBA(255, 255, 255, .07)).TextColor(ui.RGBA(255, 255, 255, .72))
					}
					button.OnClick(func() {
						go a.playerControl("episode", number)
						a.closePlayerDrawer()
					})
					button.Children(func() { ui.Textf(c, "%d", number).Bold() })
				}).Grow(1)
			}
		})
	})
}

func (a *App) closePlayerDrawer() {
	a.mu.Lock()
	a.playerDrawerOpen = false
	a.mu.Unlock()
	a.focusMediaAfterPointer()
	a.invalidate()
}

func formatPlayerSpeed(value float64) string {
	switch value {
	case .75:
		return "0.75x"
	case 1:
		return "1.0x"
	case 1.25:
		return "1.25x"
	case 1.5:
		return "1.5x"
	case 2:
		return "2.0x"
	default:
		return "1.0x"
	}
}
