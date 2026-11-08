package acrylic

import "github.com/egoist/mygo/ui"

var (
	iconFiles   = Lucide(`<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>`)
	iconHistory = Lucide(`<circle cx="12" cy="12" r="3"/><line x1="3" x2="9" y1="12" y2="12"/><line x1="15" x2="21" y1="12" y2="12"/>`)
	iconTerm    = Lucide(`<polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/>`)
	iconRefresh = Lucide(`<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/>`)
	iconFetch   = Lucide(`<path d="M12 13v8l-4-4"/><path d="m12 21 4-4"/><path d="M4.393 15.269A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.436 8.284"/>`)
)

// GalleryState 是样板间里那些控件的状态。零值可用（宽度和不透明度取默认）。
type GalleryState struct {
	Opacity  float64 // 工作卡不透明度，0 = 默认 80%
	View     int     // 图标分段
	Mode     int     // 文字分段
	Reviewed bool
	Amend    bool
	Query    string
	Filter   string
	ListW    float32
	Selected int
	Dialog   bool
}

// Gallery 是整套组件的样板间：窗口底 + 侧栏 + 一张工作卡，每种组件摆一份，
// 外加一块按设计保持不透明的代码画布作对照。新项目照它搭外壳；改了组件，
// 跑 acrylic_test 的截图看一眼（ACRYLIC_SHOT=目录）。
func Gallery(c *ui.Context, p Palette, s *GalleryState) {
	c.SetTheme(Theme(p))
	if s.ListW == 0 {
		s.ListW = 220
	}
	Ground(c, p)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		// 侧栏：直接坐在窗口底上，不铺底。
		ui.Column(c).Width(240).Shrink(0).FillHeight().Padding(16, 10).Gap(6).Children(func() {
			ui.Text(c, "分支").FontSize(10.5).FontWeight(WeightSemibold).TextColor(p.InkSubtle)
			SearchField(c, p, &s.Filter, "筛选分支")
			for i, name := range []string{"main", "feature/acrylic", "release/v1"} {
				Row(c, p, i == s.Selected).Label(name).Children(func() {
					ui.Text(c, name).FontSize(TextBodySM).TextColor(p.InkMuted).Grow(1)
					if i == 0 {
						Chip(c, p, "HEAD", p.Ink)
					}
				})
			}
		})

		ui.Column(c).Grow(1).FillHeight().Padding(CardGap).Children(func() {
			Card(c, p, s.Opacity).Children(func() {
				// 工具栏：坐在卡片上，不铺底。
				ui.Row(c).Height(44).Shrink(0).FillWidth().Padding(0, 10).Gap(8).
					AlignItems(ui.Center).Children(func() {
					SegmentedIcons(c, p, &s.View,
						Segment{"改动", iconFiles}, Segment{"历史", iconHistory}, Segment{"终端", iconTerm})
					SegmentedText(c, p, &s.Mode, "统一", "并排")
					ui.Spacer(c)
					ToolButton(c, p, "抓取", iconFetch, false)
					IconButton(c, p, "刷新", iconRefresh, false)
				})
				HeaderStrip(c, p).Label("栏头").Children(func() {
					Chip(c, p, "M", p.Warning)
					ui.Text(c, "acrylic/controls.go").Font("monospace").FontSize(TextBodySM).TextColor(p.Ink)
					ui.Spacer(c)
					Checkbox(c, p, &s.Reviewed, "已 review")
				})
				ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
					// 列表栏：不铺底，和右边靠那条分隔条分家。
					ui.Column(c).Width(s.ListW).Shrink(0).FillHeight().Padding(8).Gap(4).Children(func() {
						SearchField(c, p, &s.Query, "筛选文件")
						for i, f := range []string{"acrylic.go", "tokens.go", "controls.go"} {
							Row(c, p, i == 2).Label(f).Children(func() {
								ui.Text(c, f).FontSize(TextBodySM).TextColor(p.Ink).Grow(1)
								ui.Text(c, "+12").FontSize(TextCaption).TextColor(p.Success)
							})
						}
					})
					Splitter(c, p, &s.ListW, 160, 360, 220, true)
					ui.Column(c).Grow(1).FillHeight().Padding(16).Gap(12).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							if PrimaryButton(c, p, "提交", false).Clicked() {
								s.Dialog = true
							}
							Button(c, p, "取消", false)
							Button(c, p, "不可用", true)
							Kbd(c, p, "⌘↵")
						})
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							Chip(c, p, "main", p.Accent)
							Chip(c, p, "origin/main", p.Success)
							Chip(c, p, "v1.0", p.Warning)
							Checkbox(c, p, &s.Amend, "修改上次提交")
						})
						// 代码画布：按设计保持不透明（增删淡染从背景色现算）。
						ui.Column(c).Grow(1).FillWidth().MinHeight(0).Radius(RadiusMD).
							Background(p.Code).Border(1, p.Hairline).Padding(12).Gap(4).
							Label("代码画布").Children(func() {
							for _, line := range []string{"func main() {", "\tacrylic.Gallery(c, p, &s)", "}"} {
								ui.Text(c, line).Font("monospace").FontSize(TextBodySM).TextColor(p.Ink)
							}
						})
					})
				})
				FooterStrip(c, p).Label("页脚").Children(func() {
					ui.Text(c, "总计").FontSize(TextCaption).TextColor(p.InkSubtle)
					ui.Text(c, "+36 −4").Font("monospace").FontSize(TextCaption).TextColor(p.InkMuted)
				})
			})
		})
	})

	Modal(c, p, &s.Dialog, func() {
		ui.Text(c, "对话框是实色浮层").FontSize(16).FontWeight(WeightSemibold).TextColor(p.Ink)
		ui.Text(c, "里面的控件照样用洗色：洗色不挑底。").FontSize(TextBodySM).TextColor(p.InkMuted)
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if Button(c, p, "关闭", false).Clicked() {
				s.Dialog = false
			}
		})
	})
}
