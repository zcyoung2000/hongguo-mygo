package acrylic

import "github.com/egoist/mygo/ui"

// Lucide 把 Lucide 图标的路径包成 24×24 viewBox 的描边 SVG（currentColor 由
// Painter.Icon 的颜色替换）。组件只自带用得到的几颗，其余在应用里照这样包。
func Lucide(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	IconSearch = Lucide(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.34-4.34"/>`)
	IconX      = Lucide(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)
	IconCheck  = Lucide(`<path d="M20 6 9 17l-5-5"/>`)
)
