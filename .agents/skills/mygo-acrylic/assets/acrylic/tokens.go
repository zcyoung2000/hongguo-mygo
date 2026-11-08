package acrylic

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// Palette 是亚克力外观的全部颜色。字段按「层 → 洗色 → 线 → 墨 → 强调与语义
// → 阴影」排，每一组的规矩写在组头上。
//
// 读法只有一条：**层**（Ground / Card / Popover / Modal / Code）是不透明的面，
// 只许当一层的底；**洗色**（Wash*）是半透明的，只许当控件和条带的底。
// 把一支层色铺进卡片里，就是在磨砂上抠出一块实心挡板（见 SKILL.md 铁律一）。
type Palette struct {
	Dark bool

	// —— 层（不透明）——
	//
	// Ground 是窗口底：有系统材质时乘 GroundVeil 当薄纱盖在材质上，没有材质
	// 时实铺。Card 是工作卡的实色，按用户设的不透明度乘出来（CardFill）。
	// Popover / Modal 是浮层与模态，Code 是代码 / diff 画布 —— 这三处本来
	// 就是实的，不跟着透。
	Ground  ui.Color
	Card    ui.Color
	Popover ui.Color
	Modal   ui.Color
	Code    ui.Color

	// —— 洗色阶梯（半透明，深色叠白、浅色叠黑）——
	//
	// 从轻到重：透明 → WashHover → WashChip → WashSelected。洗色是「在底下
	// 那层上叠一层明度」，不挑底：落在亚克力卡、实色弹窗、代码画布上都成立，
	// 所以共享控件一律只用它。WashKbd 是快捷键提示的底。
	WashHover    ui.Color
	WashChip     ui.Color
	WashSelected ui.Color
	WashKbd      ui.Color

	// —— 发丝线：1px，三档 ——
	Hairline         ui.Color // 分割、芯片边、静息控件边
	HairlineStrong   ui.Color // 输入框外框、悬停 / 选中控件边
	HairlineTertiary ui.Color // 输入框悬停、弹窗外围

	// —— 墨：四档灰阶 ——
	Ink         ui.Color // 标题、正文基准、选中项
	InkMuted    ui.Color // 常规正文、按钮文字
	InkSubtle   ui.Color // 元信息、未激活图标
	InkTertiary ui.Color // 占位符、禁用、快捷键

	// —— 强调与语义 ——
	//
	// 强调只有一支蓝，全屏同时亮着的实心强调不超过 1~2 处（主按钮、勾上的
	// 勾选框）。Selection 是列表选中带（强调淡染），Focus 是键盘焦点圈。
	Accent        ui.Color
	AccentHover   ui.Color
	AccentPressed ui.Color
	AccentText    ui.Color
	Selection     ui.Color
	Focus         ui.Color
	Success       ui.Color
	Warning       ui.Color
	Danger        ui.Color

	// —— 阴影与硬边 ——
	CardBorder  ui.Color // 浅色卡片 / 浮层外沿那条 1px 硬边
	CardShadow  ui.Color
	CardShadow2 ui.Color
	FloatShadow ui.Color
}

// Dark 是深色的一份。取值和 Gloss（v0.3.25 之后）逐值一致。
func Dark() Palette {
	return Palette{
		Dark:    true,
		Ground:  ui.Hex("#141418"),
		Card:    ui.Hex("#232328"),
		Popover: ui.Hex("#232327"),
		Modal:   ui.Hex("#2b2b31"),
		Code:    ui.Hex("#232328"),

		WashHover:    ui.RGBA(255, 255, 255, 0.05),
		WashChip:     ui.RGBA(255, 255, 255, 0.07),
		WashSelected: ui.RGBA(255, 255, 255, 0.10),
		WashKbd:      ui.RGBA(255, 255, 255, 0.06),

		Hairline:         ui.RGBA(255, 255, 255, 0.07),
		HairlineStrong:   ui.RGBA(255, 255, 255, 0.11),
		HairlineTertiary: ui.RGBA(255, 255, 255, 0.16),

		Ink:         ui.Hex("#ececf0"),
		InkMuted:    ui.Hex("#a9a9b4"),
		InkSubtle:   ui.Hex("#83838f"),
		InkTertiary: ui.Hex("#656570"),

		Accent:        ui.Hex("#6b93ff"),
		AccentHover:   ui.Hex("#82a3ff"),
		AccentPressed: ui.Hex("#5b82ee"),
		AccentText:    ui.Hex("#ffffff"),
		Selection:     ui.RGBA(107, 147, 255, 0.14),
		Focus:         ui.RGBA(107, 147, 255, 0.65),
		Success:       ui.Hex("#27a644"),
		Warning:       ui.Hex("#f0a33a"),
		Danger:        ui.Hex("#ff6961"),

		CardBorder:  ui.RGBA(255, 255, 255, 0.08),
		CardShadow:  ui.RGBA(0, 0, 0, 0.35),
		CardShadow2: ui.RGBA(0, 0, 0, 0.20),
		FloatShadow: ui.RGBA(0, 0, 0, 0.45),
	}
}

// Light 是浅色的一份：窗口是「灰的」，卡片才是「白的」。
func Light() Palette {
	return Palette{
		Ground:  ui.Hex("#eceef4"),
		Card:    ui.Hex("#ffffff"),
		Popover: ui.Hex("#ffffff"),
		Modal:   ui.Hex("#ffffff"),
		Code:    ui.Hex("#ffffff"),

		WashHover:    ui.RGBA(0, 0, 0, 0.04),
		WashChip:     ui.RGBA(0, 0, 0, 0.045),
		WashSelected: ui.RGBA(0, 0, 0, 0.07),
		WashKbd:      ui.RGBA(0, 0, 0, 0.04),

		Hairline:         ui.RGBA(0, 0, 0, 0.06),
		HairlineStrong:   ui.RGBA(0, 0, 0, 0.10),
		HairlineTertiary: ui.RGBA(0, 0, 0, 0.14),

		Ink:         ui.Hex("#1d1d1f"),
		InkMuted:    ui.Hex("#6e6e78"),
		InkSubtle:   ui.Hex("#8d8d97"),
		InkTertiary: ui.Hex("#a3a3ad"),

		Accent:        ui.Hex("#3b6cf6"),
		AccentHover:   ui.Hex("#527ef7"),
		AccentPressed: ui.Hex("#2f5ddb"),
		AccentText:    ui.Hex("#ffffff"),
		Selection:     ui.RGBA(59, 108, 246, 0.12),
		Focus:         ui.RGBA(59, 108, 246, 0.55),
		Success:       ui.Hex("#22863a"),
		Warning:       ui.Hex("#c27b0f"),
		Danger:        ui.Hex("#d93a35"),

		CardBorder:  ui.RGBA(0, 0, 0, 0.07),
		CardShadow:  ui.RGBA(0, 0, 0, 0.07),
		CardShadow2: ui.RGBA(0, 0, 0, 0.035),
		FloatShadow: ui.RGBA(20, 20, 40, 0.14),
	}
}

// For 按明暗取一份。
func For(dark bool) Palette {
	if dark {
		return Dark()
	}
	return Light()
}

// Theme 是和调色板配套的框架主题，给框架自绘的控件（输入框光标、滚动条、
// 工具提示、菜单）用。必须在建任何控件之前 c.SetTheme —— 它只作用于之后
// 建的控件。
//
// Border 设成透明：框架自绘的控件一律不描边，边由这套组件自己按发丝线画。
// 代价是 ui.Split 那条线也没了 —— 能拖的缝用 Splitter。
func Theme(p Palette) *ui.Theme {
	t := ui.LightTheme()
	if p.Dark {
		t = ui.DarkTheme()
	}
	t.Background = p.Card
	t.Border = ui.Transparent
	t.Text = p.Ink
	t.TextMuted = p.InkMuted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.Accent, p.AccentHover, p.AccentPressed, p.AccentText
	t.Danger, t.Warning, t.Success = p.Danger, p.Warning, p.Success
	t.Focus = p.Focus
	t.Radius = RadiusMD
	if p.Dark {
		t.Surface, t.SurfaceHover, t.SurfacePressed = ui.Hex("#404047"), ui.Hex("#4a4a52"), ui.Hex("#52525a")
		t.Selection = ui.RGBA(107, 147, 255, 0.25)
		t.Scrollbar = ui.RGBA(255, 255, 255, 0.16)
	} else {
		t.Surface, t.SurfaceHover, t.SurfacePressed = ui.Hex("#ffffff"), ui.Hex("#f7f7f9"), ui.Hex("#eeeef1")
		t.Selection = ui.RGBA(59, 108, 246, 0.25)
		t.Scrollbar = ui.RGBA(0, 0, 0, 0.18)
	}
	return t
}

// 形状与字号 token。圆角六档、各绑一类组件，别跨着用。
const (
	RadiusXS   = 4.0  // 勾选框、极小标签、快捷键提示
	RadiusSM   = 6.0  // 幽灵按钮、芯片、分段单元、列表选中带
	RadiusMD   = 8.0  // 通用按钮、输入框、搜索栏、分段轨道
	RadiusLG   = 12.0 // 卡片、浮层、Composer
	RadiusXL   = 16.0 // 模态
	RadiusPill = 9999.0

	TextButton  = 12.5
	TextCaption = 11.0
	TextBodySM  = 12.0

	WeightMedium   = 500
	WeightSemibold = 600
)

// hexOf 把颜色写成 "#rrggbb"（WindowOptions.BackgroundColor 要 CSS 语法）。
func hexOf(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
