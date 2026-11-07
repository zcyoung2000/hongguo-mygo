package uiapp

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/comment.svg
var commentSVG []byte

var commentIcon = ui.MustParseSVG(commentSVG)

// renderPlayerVisualFixture is used only by headless UI acceptance. The
// production player remains a WebView; this representation makes its fitted
// video, right rail and control row inspectable when the host cannot paint
// web content in tests.
func (a *App) renderPlayerVisualFixture(c *ui.Context) {
	ui.Column(c.Key("player-visual-fixture")).Fill().Background(ui.Hex("#000000")).Children(func() {
		ui.Row(c.Key("fixture-topbar")).Height(52).Padding(0, 18, 0, 14).
			AlignItems(ui.Center).Gap(10).Children(func() {
			back := ui.Row(c.Key("fixture-back")).Height(32).Padding(0, 12).
				AlignItems(ui.Center).Gap(6).Radius(7).
				Background(ui.RGBA(255, 255, 255, .08)).Border(1, ui.RGBA(255, 255, 255, .16)).
				Role(ui.RoleButton).Label("返回")
			back.Children(func() {
				ui.Text(c, "返回").FontSize(12).Bold().TextColor(ui.RGBA(255, 255, 255, .9))
			})
			ui.Text(c, "都市甜宠：重生恋人").FontSize(16).Bold().TextColor(ui.RGB(255, 255, 255))
			ui.Text(c, "第 22 集 / 共 812 集").FontSize(12).TextColor(ui.RGBA(255, 255, 255, .72))
		})

		ui.Row(c.Key("fixture-stage")).Fill().Padding(18, 18, 0, 18).Gap(18).AlignItems(ui.Center).Children(func() {
			ui.Column(c.Key("fixture-video")).Width(540).Height(390).Radius(12).
				LinearGradient(ui.LinearGradient{
					From:  ui.RGBA(24, 145, 112, 255),
					To:    ui.RGBA(250, 178, 68, 255),
					Angle: 135,
				}).
				Border(1, ui.RGBA(255, 255, 255, .22)).
				AlignItems(ui.Center).Justify(ui.Center).Gap(10).Children(func() {
				ui.Text(c, "视频内容 · 9:16").FontSize(22).Bold().TextColor(ui.RGB(0, 0, 0))
				ui.Text(c, "当前 22 / 812 集").FontSize(15).Bold().TextColor(ui.RGBA(0, 0, 0, .78))
			})

			ui.Column(c.Key("fixture-rail")).Width(74).Gap(22).AlignItems(ui.Center).Children(func() {
				railButton(c, "fixture-like", likeIcon, "点赞")
				railButton(c, "fixture-comment", commentIcon, "评论")
				railButton(c, "fixture-favorite", favoriteIcon, "收藏")
			})
		})

		ui.Column(c.Key("fixture-controls")).Height(72).Padding(8, 24, 10, 24).
			Background(ui.RGBA(12, 12, 16, .96)).Gap(6).Children(func() {
			ui.Box(c.Key("fixture-progress")).Height(4).Radius(2).
				Background(ui.RGBA(255, 255, 255, .16)).Children(func() {
				ui.Box(c.Key("fixture-progress-current")).Width(245).Height(4).Radius(2).
					Background(ui.Hex("#fe2c55"))
			})
			ui.Row(c.Key("fixture-control-row")).Fill().AlignItems(ui.Center).Children(func() {
				ui.Row(c.Key("fixture-controls-left")).Gap(12).AlignItems(ui.Center).Children(func() {
					controlText(c, "fixture-play", "播放", true)
					controlText(c, "fixture-prev", "上一集", false)
					controlText(c, "fixture-next", "下一集", false)
					ui.Text(c, "00:48 / 03:20").FontSize(12).TextColor(ui.RGBA(255, 255, 255, .72))
				})
				ui.Box(c.Key("fixture-controls-spacer")).Grow(1)
				ui.Row(c.Key("fixture-controls-right")).Gap(10).AlignItems(ui.Center).Children(func() {
					controlText(c, "fixture-quality", "720P", true)
					controlText(c, "fixture-speed", "1.0x", true)
					controlText(c, "fixture-danmaku", "弹幕", false)
					controlText(c, "fixture-settings", "设置", false)
					controlText(c, "fixture-episodes", "选集", true)
					controlText(c, "fixture-pip", "画中画", false)
					controlText(c, "fixture-fullscreen", "全屏", false)
				})
			})
		})
	})
}

func railButton(c *ui.Context, key string, icon *ui.SVG, label string) {
	ui.Column(c.Key(key)).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Row(c.Key(key+"-circle")).Width(46).Height(46).Radius(23).
			Background(ui.RGBA(18, 18, 24, .72)).Border(1, ui.RGBA(255, 255, 255, .18)).
			AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, icon).Size(24, 24).TextColor(ui.RGB(255, 255, 255))
		})
		ui.Text(c, label).FontSize(12).Bold().TextColor(ui.RGBA(255, 255, 255, .88))
	})
}

func controlText(c *ui.Context, key, label string, emphasized bool) {
	button := ui.Row(c.Key(key)).Height(30).Padding(0, 9).Radius(15).
		AlignItems(ui.Center).Role(ui.RoleButton).Label(label)
	if emphasized {
		button.Background(ui.RGBA(255, 255, 255, .12)).Border(1, ui.RGBA(255, 255, 255, .22))
	} else {
		button.Background(ui.RGBA(255, 255, 255, .06))
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(12).Bold().TextColor(ui.RGBA(255, 255, 255, .9))
	})
}
