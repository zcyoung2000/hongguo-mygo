package acrylic

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 洗色阶梯：全部半透明，从轻到重排好；层全部不透明。任何一条坏了，要么
// 组件在磨砂上抠出实心，要么悬停比静息还轻。
func TestWashLadder(t *testing.T) {
	for _, p := range []Palette{Dark(), Light()} {
		ladder := []struct {
			name string
			c    ui.Color
		}{{"WashHover", p.WashHover}, {"WashChip", p.WashChip}, {"WashSelected", p.WashSelected}}
		for i, w := range ladder {
			if w.c.A == 0 || w.c.A == 255 {
				t.Errorf("dark=%v: %s 的 alpha=%d，洗色必须半透明", p.Dark, w.name, w.c.A)
			}
			if i > 0 && w.c.A < ladder[i-1].c.A {
				t.Errorf("dark=%v: %s 比 %s 轻，梯子倒了", p.Dark, w.name, ladder[i-1].name)
			}
		}
		for name, c := range map[string]ui.Color{"Ground": p.Ground, "Card": p.Card, "Popover": p.Popover, "Modal": p.Modal, "Code": p.Code} {
			if c.A != 255 {
				t.Errorf("dark=%v: 层 %s 必须不透明，alpha=%d", p.Dark, name, c.A)
			}
		}
		if got := p.Wash(Ghost, false, false); got != ui.Transparent {
			t.Errorf("dark=%v: 幽灵控件静息应透明，得到 %v", p.Dark, got)
		}
		if p.Wash(Filled, true, false) != p.WashSelected || p.Wash(Ghost, true, false) != p.WashHover {
			t.Errorf("dark=%v: 悬停没按梯子走", p.Dark)
		}
	}
}

// 窗口底：有材质时是薄纱，没有材质时实铺。深色压在最亮的材质上也得是暗的，
// 次要文字在上面不低于 4.5:1（材质约 #898c8d，是浅色系统上实测的灰度）。
func TestGroundVeil(t *testing.T) {
	p := Dark()
	if GroundColor(p, false) != p.Ground {
		t.Fatal("没有材质时窗口底应实铺 Ground")
	}
	veil := GroundColor(p, true)
	if veil != p.Ground.Alpha(GroundVeil) {
		t.Fatalf("有材质时窗口底应是 Ground × %.2f", GroundVeil)
	}
	ground := veil.Over(ui.Hex("#898c8d"))
	if r := contrast(ground, p.InkMuted); r < 4.5 {
		t.Errorf("深色窗口底压在亮材质上，次要文字对比度 %.1f:1，低于 4.5:1", r)
	}
}

func TestSettings(t *testing.T) {
	for in, want := range map[string]mygo.Vibrancy{
		"none": mygo.VibrancyNone, "subtle": mygo.VibrancySidebar,
		"medium": mygo.VibrancyPopover, "deep": mygo.VibrancyAcrylic, "bogus": mygo.VibrancySidebar,
	} {
		if got := NormalizeBlur(in).Vibrancy(); got != want {
			t.Errorf("Blur(%q).Vibrancy() = %q，应为 %q", in, got, want)
		}
	}
	for in, want := range map[float64]float64{0: OpacityDefault, -1: OpacityDefault, 0.1: OpacityMin, 2: OpacityMax, 0.5: 0.5} {
		if got := NormalizeOpacity(in); got != want {
			t.Errorf("NormalizeOpacity(%v) = %v，应为 %v", in, got, want)
		}
	}
	if got := CardFill(Light(), 1); got != Light().Card {
		t.Errorf("100%% 的卡片应与实色卡一致，得到 %v", got)
	}
	if CardRadius("darwin") != 20 || CardRadius("windows") != RadiusLG {
		t.Errorf("卡片圆角应与窗口同心、且不低于 RadiusLG：darwin=%v windows=%v",
			CardRadius("darwin"), CardRadius("windows"))
	}
}

// 亚克力兼容的硬指标：窗口底换成品红、卡片拧到 30%，每个组件身上没字的
// 地方都得透出品红。透不出来，就是铺了不透明的面。代码画布按设计是实的，
// 拿它当反向对照 —— 检查器要是连它也说「透」，就是检查器坏了。
func TestComponentsLetTheMaterialThrough(t *testing.T) {
	magenta := ui.Hex("#ff00ff")
	for _, dark := range []bool{false, true} {
		s := &GalleryState{Opacity: OpacityMin}
		tt := ui.NewTester(func(c *ui.Context) {
			Gallery(c, For(dark), s)
			c.Root().Background(magenta)
		}, 1200, 760)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		pinkAt := func(x, y float32) bool {
			r, g, b, _ := img.At(int(x), int(y)).RGBA()
			r, g, b = r>>8, g>>8, b>>8
			return int(r)-int(g) > 60 && int(b)-int(g) > 60
		}
		// 每个组件取一处没有字、没有图标的点：左内边距里、竖直居中。
		for _, name := range []string{
			"筛选分支", "筛选文件", "栏头", "页脚", "取消", "统一", "已 review", "修改上次提交",
			"main", "feature/acrylic", "acrylic.go", "controls.go", "HEAD",
		} {
			r, ok := tt.Find(name)
			if !ok {
				t.Fatalf("dark=%v: 样板间里找不到 %q", dark, name)
			}
			x, y := r.X+3, r.Y+r.H/2
			if name == "已 review" || name == "修改上次提交" {
				x = r.X + 8 // 勾选框方框的正中
			}
			if !pinkAt(x, y) {
				t.Errorf("dark=%v: %q 在 (%.0f,%.0f) 透不出材质 —— 它铺了不透明的面", dark, name, x, y)
			}
		}
		code, _ := tt.Find("代码画布")
		if pinkAt(code.X+code.W/2, code.Y+code.H-6) {
			t.Errorf("dark=%v: 代码画布也透了 —— 它应保持不透明，或者检查器失灵", dark)
		}
		if dir := os.Getenv("ACRYLIC_SHOT"); dir != "" {
			shot(t, tt, filepath.Join(dir, map[bool]string{false: "stress-light.png", true: "stress-dark.png"}[dark]))
		}
	}
}

// 样板间在两种明暗、实色窗口底与模态打开时都画得出来；ACRYLIC_SHOT=目录
// 时把截图写出去，改了组件就看一眼。
func TestGalleryShots(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := &GalleryState{}
		tt := ui.NewTester(func(c *ui.Context) { Gallery(c, For(dark), s) }, 1200, 760)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.SetDark(dark)
		tt.Frame()
		if err := tt.Click("提交"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !s.Dialog || !tt.HasText("对话框是实色浮层") {
			t.Fatalf("dark=%v: 点主按钮应打开对话框", dark)
		}
		name := map[bool]string{false: "light", true: "dark"}[dark]
		if dir := os.Getenv("ACRYLIC_SHOT"); dir != "" {
			shot(t, tt, filepath.Join(dir, "modal-"+name+".png"))
		}
		if err := tt.Click("关闭"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if dir := os.Getenv("ACRYLIC_SHOT"); dir != "" {
			shot(t, tt, filepath.Join(dir, "gallery-"+name+".png"))
		}
	}
}

func shot(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// contrast 是 WCAG 对比度（1~21）。
func contrast(a, b ui.Color) float64 {
	l := func(c ui.Color) float64 {
		ch := func(v uint8) float64 {
			s := float64(v) / 255
			if s <= 0.03928 {
				return s / 12.92
			}
			return math.Pow((s+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
	}
	la, lb := l(a), l(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
