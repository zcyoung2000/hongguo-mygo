package acrylic

import "github.com/egoist/mygo/ui"

// —— 洗色阶梯 ——

// Kind 是控件静息时站在梯子的哪一级。
type Kind uint8

const (
	// Ghost 静息透明：工具按钮、× 键、列表行、分段格、标签芯片。
	Ghost Kind = iota
	// Filled 静息 WashChip：次级按钮、选择器、输入框、分段轨道、勾选框。
	Filled
)

// Wash 是控件这一帧的底色。状态只在一条梯子上往上走，不换几何：
//
//	          静息        悬停           按下 / 选中 / 进行中
//	Ghost     透明        WashHover      WashSelected
//	Filled    WashChip    WashSelected   WashSelected
func (p Palette) Wash(k Kind, hovered, active bool) ui.Color {
	switch {
	case active, hovered && k == Filled:
		return p.WashSelected
	case hovered:
		return p.WashHover
	case k == Filled:
		return p.WashChip
	}
	return ui.Transparent
}

// Edge 是实心控件的边：静息 Hairline，悬停 / 按下 HairlineStrong。边框一直
// 占着 1px、只换颜色 —— 状态变化时加减边框会让布局抖一下。
func (p Palette) Edge(hovered, active bool) ui.Color {
	if hovered || active {
		return p.HairlineStrong
	}
	return p.Hairline
}

// —— 图标 ——

// Icon 画一颗 size×size 的单色图标。
func Icon(c *ui.Context, svg *ui.SVG, size float32, color ui.Color) ui.Element {
	return ui.Box(c).Size(size, size).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Icon(svg, r, color)
	})
}

// —— 按钮 ——
//
// 四颗按钮都把 Label / Tooltip 写在 Disabled 前面：框架的 Tooltip 在构建时问
// 一次 Hovered()，而禁用元素不报悬停 —— 顺序反过来，「为什么点不了」那句
// 提示就永远浮不出来。返回元素，调用方问 Clicked()。

// IconButton 是 28×28 的幽灵图标键（工具栏上的刷新、设置）。
func IconButton(c *ui.Context, p Palette, label string, icon *ui.SVG, disabled bool) ui.Element {
	btn := ui.ButtonBase(c).Size(28, 28).Shrink(0).Radius(RadiusSM).Center().
		Cursor(ui.CursorPointer).Label(label).Tooltip(label).Disabled(disabled)
	fg := p.InkSubtle
	if disabled {
		fg = p.InkTertiary
		btn.Cursor(ui.CursorDefault)
	} else {
		hov, act := btn.Hovered(), btn.Pressed()
		btn.Background(p.Wash(Ghost, hov, act))
		if hov || act {
			fg = p.Ink
		}
	}
	Fade(btn)
	btn.Children(func() { Icon(c, icon, 15, fg) })
	return btn
}

// ToolButton 是图标 + 文字的幽灵键（工具栏上的「抓取」「推送」）。
func ToolButton(c *ui.Context, p Palette, label string, icon *ui.SVG, disabled bool) ui.Element {
	btn := ui.ButtonBase(c).Height(28).Shrink(0).Radius(RadiusSM).
		Padding(0, 9).Gap(6).AlignItems(ui.Center).
		Cursor(ui.CursorPointer).Label(label).Tooltip(label).Disabled(disabled)
	fg := p.InkSubtle
	if disabled {
		fg = p.InkTertiary
		btn.Cursor(ui.CursorDefault)
	} else {
		hov, act := btn.Hovered(), btn.Pressed()
		btn.Background(p.Wash(Ghost, hov, act))
		if hov || act {
			fg = p.Ink
		}
	}
	Fade(btn)
	btn.Children(func() {
		if icon != nil {
			Icon(c, icon, 15, fg)
		}
		ui.Text(c, label).FontSize(TextButton).FontWeight(WeightMedium).SingleLine().TextColor(fg)
	})
	return btn
}

// Button 是次级按钮：WashChip 底 + 发丝边 + InkMuted 字。「取消」「关闭」
// 「重试」都是它；主次靠「有没有实心底」分，不靠色相。
func Button(c *ui.Context, p Palette, label string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c).Shrink(0).Height(28).Radius(RadiusMD).
		Padding(0, 12).Gap(5).AlignItems(ui.Center).
		Label(label).Tooltip(label).Disabled(disabled).Cursor(ui.CursorPointer)
	bg, edge, fg := ui.Transparent, p.Hairline, p.InkTertiary
	if !disabled {
		hov, act := btn.Hovered(), btn.Pressed()
		bg, edge, fg = p.Wash(Filled, hov, act), p.Edge(hov, act), p.InkMuted
		if hov || act {
			fg = p.Ink
		}
	}
	btn.Background(bg).Border(1, edge)
	Fade(btn)
	btn.Children(func() {
		ui.Text(c, label).FontSize(TextButton).FontWeight(WeightMedium).TextColor(fg)
	})
	return btn
}

// PrimaryButton 是主按钮：实心强调色。一屏 / 一个对话框至多一颗 —— 并排两颗
// 蓝按钮，用户就得先读字才知道哪颗是要走的那一步。禁用时退成 WashChip。
func PrimaryButton(c *ui.Context, p Palette, label string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c).Shrink(0).Height(28).Radius(RadiusMD).
		Padding(0, 12).Gap(5).AlignItems(ui.Center).
		Label(label).Tooltip(label).Disabled(disabled).Cursor(ui.CursorPointer)
	bg, edge, fg := p.Accent, p.Accent, p.AccentText
	switch {
	case disabled:
		bg, edge, fg = p.WashChip, p.Hairline, p.InkTertiary
	case btn.Pressed():
		bg, edge = p.AccentPressed, p.AccentPressed
	case btn.Hovered():
		bg, edge = p.AccentHover, p.AccentHover
	}
	btn.Background(bg).Border(1, edge)
	Fade(btn)
	btn.Children(func() {
		ui.Text(c, label).FontSize(TextButton).FontWeight(WeightMedium).TextColor(fg)
	})
	return btn
}

// —— 芯片 ——

// Chip 是芯片 / 徽章：WashChip 底 + 发丝边 + InkMuted 字。语义只靠那颗 6px
// 圆点（dot 传 ui.Transparent 就不画）—— 一行四枚整块涂色的药丸，读的人
// 只看见色块，认不出字。
func Chip(c *ui.Context, p Palette, label string, dot ui.Color) ui.Element {
	chip := ui.Row(c).Height(18).Shrink(0).AlignItems(ui.Center).Gap(4).
		Padding(0, 6).Radius(RadiusXS).Background(p.WashChip).Border(1, p.Hairline).Label(label)
	chip.Children(func() {
		if dot.A > 0 {
			ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(dot)
		}
		ui.Text(c, label).FontSize(TextCaption).FontWeight(WeightMedium).
			SingleLine().TextColor(p.InkMuted)
	})
	return chip
}

// Kbd 是快捷键提示（⌘K）：WashKbd 底，字取最淡的一档。
func Kbd(c *ui.Context, p Palette, keys string) ui.Element {
	return ui.Box(c).Shrink(0).Radius(RadiusXS).Padding(1, 5).Background(p.WashKbd).Children(func() {
		ui.Text(c, keys).FontSize(TextCaption).TextColor(p.InkTertiary)
	})
}

// —— 输入 ——

// SearchField 是筛选框：WashChip 底、放大镜、无脸的输入框，返回里面那个输入
// 元素（问 Focused / Changed 用它）。不用 ui.SearchField：带脸的输入框会在
// 洗色上再叠一层自己的不透明底，placeholder 也写死成英文。
//
// 边框一直占 1px、只换颜色：静息 HairlineStrong（输入框要比分割线重一档才立
// 得起来），悬停 HairlineTertiary，聚焦 Accent。
func SearchField(c *ui.Context, p Palette, value *string, placeholder string) ui.Element {
	var in ui.Element
	box := ui.Row(c).Height(28).Shrink(0).AlignItems(ui.Center).
		Gap(6).Padding(0, 8).Radius(RadiusMD).Background(p.WashChip).
		Label(placeholder).Children(func() {
		Icon(c, IconSearch, 12, p.InkSubtle)
		in = ui.TextInputBase(c, value).Grow(1).MinWidth(0).
			Placeholder(placeholder).Label(placeholder)
		if value != nil && *value != "" {
			clear := ui.ButtonBase(c).Size(16, 16).Radius(RadiusPill).Center().
				Cursor(ui.CursorPointer).Label("Clear")
			if clear.Hovered() {
				clear.Background(p.WashSelected)
			}
			Fade(clear)
			clear.Children(func() { Icon(c, IconX, 10, p.InkSubtle) })
			if clear.Clicked() {
				*value = ""
			}
		}
	})
	edge := p.HairlineStrong
	switch {
	case in.Valid() && in.Focused():
		edge = p.Accent
	case box.Hovered():
		edge = p.HairlineTertiary
	}
	box.Border(1, edge)
	return in
}

// Checkbox 是落在亚克力层上的勾选框。不用 ui.Checkbox：框架那颗未勾选时
// 铺 t.Background —— 和卡片同色、却是不透明的，卡片一透就是一颗实心方块。
// 点击、Space、无障碍角色都还是 CheckboxBase 的，尺寸照框架那颗取。
func Checkbox(c *ui.Context, p Palette, checked *bool, label string) ui.Element {
	t := c.Theme()
	row := ui.CheckboxBase(c, checked).Gap(t.Space(2)).AlignItems(ui.Center).
		FocusRing(false).Label(label)
	on, hovered := *checked, row.Hovered()
	row.Children(func() {
		box := ui.Box(c).Size(t.Space(4), t.Space(4)).Radius(RadiusXS).Shrink(0)
		switch {
		case on:
			box.Background(p.Accent).Border(1, p.Accent)
		case hovered:
			box.Background(p.WashSelected).Border(1, p.HairlineTertiary)
		default:
			box.Background(p.WashChip).Border(1, p.HairlineStrong)
		}
		Fade(box)
		box.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			if on {
				pt.Icon(IconCheck, ui.Rect{X: r.X + 3, Y: r.Y + 3, W: r.W - 6, H: r.H - 6}, p.AccentText)
			}
			if row.FocusVisible() {
				pt.FocusRing(r, [4]float32{RadiusXS, RadiusXS, RadiusXS, RadiusXS})
			}
		})
		if label != "" {
			ui.Text(c, label).TextColor(p.Ink)
		}
	})
	return row
}

// —— 分段控件 ——

// SegmentedText 是文字分段控件：轨道 WashChip + 发丝边，格子是 Ghost 梯子，
// 选中的格子再描一道 HairlineStrong。每一格的边框一直占着 1px（未选中是透明
// 的），选中只换颜色 —— 加边框会把这一格撑宽 2px，右边的格子跟着挪。
//
// 轨道圆角 MD、格子 SM：格子四周缩 2px，8 − 2 = 6 正好同心。
func SegmentedText(c *ui.Context, p Palette, selected *int, labels ...string) ui.Element {
	parts := ui.SegmentedBase(c, selected, len(labels))
	g := parts.Track
	g.Height(28).AlignItems(ui.Center).Padding(2).Gap(2).Radius(RadiusMD).
		Background(p.WashChip).Border(1, p.Hairline)
	g.Children(func() {
		for i, label := range labels {
			s := parts.Segment(i).Height(22).Padding(0, 10).Radius(RadiusSM).
				Cursor(ui.CursorPointer).Label(label)
			segment(c, p, s, i == *selected, func(fg ui.Color) {
				ui.Text(c, label).FontSize(TextBodySM).FontWeight(WeightMedium).
					SingleLine().TextColor(fg)
			})
		}
	})
	// 框架的分段控件在 Changed 里应用这一帧的输入；不问的话调用方读到的
	// *selected 还是上一帧的。
	g.Changed()
	return g
}

// Segment 是图标分段控件的一格。
type Segment struct {
	Label string // 无障碍名字、悬停提示
	Icon  *ui.SVG
}

// SegmentedIcons 是图标分段控件（视图切换）。轨道不铺底、不描边：工具栏已经
// 坐在卡片上，再套一圈框就是「框里的框」。
func SegmentedIcons(c *ui.Context, p Palette, selected *int, items ...Segment) ui.Element {
	parts := ui.SegmentedBase(c, selected, len(items))
	g := parts.Track
	g.Height(28).AlignItems(ui.Center).Padding(2).Gap(2)
	g.Children(func() {
		for i, it := range items {
			s := parts.Segment(i).Size(26, 24).Radius(RadiusSM).Center().
				Cursor(ui.CursorPointer).Label(it.Label).Tooltip(it.Label)
			segment(c, p, s, i == *selected, func(fg ui.Color) { Icon(c, it.Icon, 14, fg) })
		}
	})
	g.Changed()
	return g
}

func segment(c *ui.Context, p Palette, s ui.Element, sel bool, content func(fg ui.Color)) {
	hov := s.Hovered()
	fg, edge := p.InkMuted, ui.Transparent
	if sel {
		fg, edge = p.Ink, p.HairlineStrong
	} else if hov {
		fg = p.Ink
	}
	s.Background(p.Wash(Ghost, hov, sel)).Border(1, edge)
	Fade(s)
	s.Children(func() { content(fg) })
}

// —— 列表与条带 ——

// Row 是列表 / 树里的一行：静息不铺底，悬停 WashHover，选中 Selection（强调
// 淡染的底带）。返回的行上接着 Children、问 Clicked。
func Row(c *ui.Context, p Palette, selected bool) ui.Element {
	row := ui.Row(c).Height(28).Shrink(0).FillWidth().Padding(0, 8).Gap(8).
		AlignItems(ui.Center).Radius(RadiusSM).Cursor(ui.CursorPointer)
	switch {
	case selected:
		row.Background(p.Selection)
	case row.Hovered():
		row.Background(p.WashHover)
	}
	Fade(row)
	return row
}

// HeaderStrip 是卡片里抬一档的栏头（文件名、状态、整文件操作）：WashChip 底，
// 下沿一道发丝线。
func HeaderStrip(c *ui.Context, p Palette) ui.Element {
	return ui.Row(c).Height(34).Shrink(0).FillWidth().Padding(0, 12).Gap(8).
		AlignItems(ui.Center).Background(p.WashChip).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Hairline)
}

// FooterStrip 是常驻的状态栏（总计、计数）：WashChip 底，上沿一道发丝线 ——
// 不画出来，这几个字就浮在底上，像谁忘了删的一行。
func FooterStrip(c *ui.Context, p Palette) ui.Element {
	return ui.Row(c).Shrink(0).FillWidth().Padding(7, 12).Gap(6).
		AlignItems(ui.Center).Background(p.WashChip).
		BorderWidth(1, 0, 0, 0).BorderColor(p.Hairline)
}

// SplitterGrip 是分隔条的命中宽度：一条发丝线是世界上最难点中的东西。
const SplitterGrip = 6

// Splitter 是两栏之间能拖的缝。屏上只有 1px 发丝线（悬停 / 按下提亮到
// HairlineStrong，不用强调色：它是边界，不是主操作），命中区两侧各探出
// (SplitterGrip−1)/2。不用 ui.Split：它拿 t.Border 画线，而 Theme 把 Border
// 设成了透明 —— 照搬的话这条缝在屏上一根线都没有。
//
// vertical 为真时线是竖的，size 是左栏宽度；否则线是横的，size 是上栏高度。
// 拖动夹在 [lo, hi]，双击回到 reset。把它摆在两栏之间（占 1px）。
func Splitter(c *ui.Context, p Palette, size *float32, lo, hi, reset float32, vertical bool) ui.Element {
	const off = (SplitterGrip - 1) / 2.0
	slot := ui.Box(c).Shrink(0)
	if vertical {
		slot.Width(1).FillHeight()
	} else {
		slot.Height(1).FillWidth()
	}
	slot.Children(func() {
		h := ui.Box(c).Absolute().Role(ui.RoleNone)
		if vertical {
			h.Top(0).Bottom(0).Left(-off).Width(SplitterGrip).Cursor(ui.CursorResizeEW)
		} else {
			h.Left(0).Right(0).Top(-off).Height(SplitterGrip).Cursor(ui.CursorResizeNS)
		}
		if h.DoubleClicked() {
			*size = reset
		}
		if dx, dy, held := h.Dragged(); held {
			d := dy
			if vertical {
				d = dx
			}
			*size = min(max(*size+d, lo), hi)
		}
		h.Draw(func(pt *ui.Painter, r ui.Rect) {
			line := p.Hairline
			if h.Hovered() || h.Pressed() {
				line = p.HairlineStrong
			}
			if vertical {
				pt.Fill(ui.Rect{X: r.X + off, Y: r.Y, W: 1, H: r.H}, line, 0)
			} else {
				pt.Fill(ui.Rect{X: r.X, Y: r.Y + off, W: r.W, H: 1}, line, 0)
			}
		})
	})
	return slot
}
