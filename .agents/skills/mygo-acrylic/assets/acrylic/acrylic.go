// Package acrylic 是 mygo 上的一套「亚克力」外观：窗口底是系统磨砂材质
// 盖一层主题色薄纱，工作卡按用户设的不透明度透出材质，卡片和侧栏上的控件
// 只用半透明的洗色。
//
// 三层，各有各的规矩（详见 SKILL.md）：
//
//	窗口底（Ground）   系统材质 + Palette.Ground × GroundVeil；没有材质时实铺 Ground
//	工作卡（Card）     Palette.Card × 用户不透明度（30%~100%，默认 80%）
//	浮层 / 模态 / 代码  实色：Popover、Modal、Code —— 它们本来就是实的
//
// 卡片和侧栏上的面板、列表不铺底；控件和条带只用洗色（Palette.Wash）。
// 透明度只有一个入口：用户设置里的不透明度，落在卡片这一层。
package acrylic

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// —— 用户设置：模糊档位与不透明度 ——

// Blur 是材质的模糊档位，存进配置的就是这个 id。
type Blur string

const (
	BlurNone   Blur = "none"   // 不挂材质：窗口底实铺
	BlurSubtle Blur = "subtle" // 侧栏材质（默认）
	BlurMedium Blur = "medium" // 浮层材质
	BlurDeep   Blur = "deep"   // Acrylic（Windows 11）/ HUD（macOS）
)

// Blurs 是设置里分段控件的顺序。
var Blurs = []Blur{BlurNone, BlurSubtle, BlurMedium, BlurDeep}

// NormalizeBlur 把不认识的档位（旧配置、手改的文件）收敛回 subtle：模糊档位
// 只影响观感，不值得为一条坏值让窗口没有材质。
func NormalizeBlur(id string) Blur {
	switch b := Blur(id); b {
	case BlurNone, BlurSubtle, BlurMedium, BlurDeep:
		return b
	}
	return BlurSubtle
}

// Vibrancy 把档位映射到 mygo 的窗口材质。
func (b Blur) Vibrancy() mygo.Vibrancy {
	switch NormalizeBlur(string(b)) {
	case BlurNone:
		return mygo.VibrancyNone
	case BlurMedium:
		return mygo.VibrancyPopover
	case BlurDeep:
		return mygo.VibrancyAcrylic
	}
	return mygo.VibrancySidebar
}

// 工作卡的不透明度：下限 30% —— 再低卡片上的字就没有稳定的对比度了。
const (
	OpacityMin     = 0.30
	OpacityMax     = 1.0
	OpacityDefault = 0.80
)

// NormalizeOpacity 把 0（没设过）当默认值，越界的收进 [OpacityMin, OpacityMax]。
func NormalizeOpacity(v float64) float64 {
	if v <= 0 {
		return OpacityDefault
	}
	return min(max(v, OpacityMin), OpacityMax)
}

// —— 窗口 ——

// Window 填好 mygo.WindowOptions 里亚克力那三项：透明底（合成器才能把背后
// 的内容磨砂进来）、第一帧之前露出的底色（取窗口底那支，避免先闪一下别的
// 颜色）、材质档位。
func Window(o *mygo.WindowOptions, dark bool, blur Blur) {
	o.Transparent = true
	o.BackgroundColor = hexOf(For(dark).Ground)
	o.Vibrancy = blur.Vibrancy()
}

// SetAppearance 让原生那一侧（材质、菜单、标题栏）跟着应用的明暗走。只在视图
// 里 c.SetTheme 不够：NSVisualEffectView 的明暗由原生决定，应用固定深色时
// 材质不能还是系统的浅色。mode 取 "light" / "dark" / 其余（跟随系统）。
func SetAppearance(mode string) {
	switch mode {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// 运行中改档位：win.SetVibrancy(blur.Vibrancy()); win.Invalidate()。
// 改不透明度：卡片颜色每帧现算，win.Invalidate() 就够。

// —— 层 ——

// GroundVeil 是窗口底那层主题色薄纱的不透明度。材质最亮（正午的白墙）也要
// 把侧栏的次要文字压在 4.5:1 以上：材质跟的是系统外观，不是应用里固定的
// 深 / 浅色，不能把底色整个交给它。
const GroundVeil = 0.65

// GroundColor 是窗口底这一帧的颜色：有材质时是薄纱，没有材质时实铺。
func GroundColor(p Palette, vibrancy bool) ui.Color {
	if vibrancy {
		return p.Ground.Alpha(GroundVeil)
	}
	return p.Ground
}

// Ground 把窗口底铺在根上。侧栏直接坐在它上面，不再自己铺色。
func Ground(c *ui.Context, p Palette) {
	c.Root().Background(GroundColor(p, c.Vibrancy()))
}

// CardFill 是工作卡的底：Card 乘用户设的不透明度（先 NormalizeOpacity）。
func CardFill(p Palette, opacity float64) ui.Color {
	return p.Card.Alpha(float32(NormalizeOpacity(opacity)))
}

// CardGap 是工作卡离窗口边的距离；卡片圆角按它和窗口圆角同心（CardRadius）。
const CardGap = 8

// Card 起一张工作卡：一列，铺 CardFill，圆角与窗口同心，Clip 住里面的条带，
// 外沿一圈抬升（Raised）。卡片里的面板、列表都不要再铺底。
func Card(c *ui.Context, p Palette, opacity float64) ui.Element {
	card := ui.Column(c).Grow(1).FillWidth().MinHeight(0).
		Radius(CardRadius(runtime.GOOS)).Background(CardFill(p, opacity)).Clip()
	return Raised(card, p)
}

// WindowRadius 是系统画的窗口圆角（不归我们画，只能跟）。
func WindowRadius(goos string) float32 {
	switch goos {
	case "darwin":
		return 28 // macOS 26；macOS 15 及更早是 10
	case "windows":
		return 8
	}
	return 12
}

// CardRadius 是工作卡的圆角：内 = 外 − CardGap，圆心落在同一点两圈弧才平行；
// 但不低于 RadiusLG —— Windows 上严格同心算出来是 0，那不是同心，是圆角丢了。
func CardRadius(goos string) float32 {
	return max(WindowRadius(goos)-CardGap, RadiusLG)
}

// Raised 把卡片从窗口底上抬起来：深色是上沿一道亮边 + 三层收紧的影子，浅色
// 是一条 0 模糊 / 1 扩散的硬边 + 两层软影。硬边用外阴影画、不用 Border ——
// Border 会像内边距一样把内容往里挤 1px。
func Raised(e ui.Element, p Palette) ui.Element {
	if p.Dark {
		return darkShadow(litEdge(e, p))
	}
	return e.Shadow(0, 0, 0, 1, p.CardBorder).
		Shadow(0, 1, 2, 0, p.CardShadow).
		Shadow(0, 1, 6, 0, p.CardShadow2)
}

// Floating 是浮层与对话框的抬升：比卡片多一层大投影。
func Floating(e ui.Element, p Palette) ui.Element {
	if p.Dark {
		return darkShadow(litEdge(e, p)).Shadow(0, 16, 40, 0, p.FloatShadow)
	}
	return e.Shadow(0, 0, 0, 1, p.CardBorder).
		Shadow(0, 1, 3, 0, p.CardShadow).
		Shadow(0, 16, 40, 0, p.FloatShadow)
}

// PopoverSurface 给下拉 / 右键浮层铺实色面。浮层里的条目可以用洗色，也可以
// 用不透明的阶梯 —— 底是实的，两种都成立。
func PopoverSurface(e ui.Element, p Palette) ui.Element {
	if p.Dark {
		return e.Background(p.Popover).
			InsetShadow(0, 0, 0, 1, ui.RGBA(255, 255, 255, 0.12)).
			Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.25)).
			Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, 0.40))
	}
	return e.Background(p.Popover).
		InsetShadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.30)).
		Shadow(0, 5, 10, 0, ui.RGBA(0, 0, 0, 0.22))
}

// Modal 是对话框的地基：压暗的背景 + 一块实色面板（Modal 色、圆角 XL）。
// Esc、点背景关闭、焦点锁在面板内都是 ui.DialogBase 的。
func Modal(c *ui.Context, p Palette, open *bool, fn func()) {
	t := c.Theme()
	w, h := c.Size()
	ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, 0.25))
		panel.Padding(t.Space(5)).Gap(t.Space(3)).Radius(RadiusXL).
			Background(p.Modal).Border(1, p.Hairline).
			MaxWidth(w-t.Space(10)).MaxHeight(h-t.Space(10)).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.24))
		fn()
	})
}

func litEdge(e ui.Element, p Palette) ui.Element {
	return e.Border(1, p.Ink.Alpha(0.12)).InsetShadow(0, 1, 0, 0, p.Ink.Alpha(0.18))
}

func darkShadow(e ui.Element) ui.Element {
	return e.Shadow(0, 1, 1, 0, ui.RGBA(0, 0, 0, 0.20)).
		Shadow(0, 2, 2, -1, ui.RGBA(0, 0, 0, 0.20)).
		Shadow(0, 4, 6, -3, ui.RGBA(0, 0, 0, 0.24))
}

// HoverMotion 是悬停 / 选中换色的过渡时长。
const HoverMotion = 120 * time.Millisecond

// Fade 给「平时没脸、悬停或选中才上色」的元素挂上颜色过渡。
func Fade(e ui.Element) ui.Element {
	return e.Transition(ui.ElementTransition{Colors: true, Duration: HoverMotion})
}
