package uiapp

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// cardGrid gives posters the full column width and preserves their 3:4 ratio.
// Read the host's last layout, just as GridView does when fitting its columns.
func (a *App) cardGrid(c *ui.Context, key string, state *ui.GridState, scroll *ui.ScrollState, items []CatalogItem, extra float32, after func(CatalogItem)) ui.Element {
	host := ui.Column(c.Key(key)).MinWidth(0).MinHeight(0)
	width := host.Bounds().W
	baseTheme := *c.Theme()
	gridTheme := baseTheme
	gridTheme.Spacing = 8 // GridView derives its column and row gaps from Space(2).
	gap := gridTheme.Space(2)
	columns := max(1, int(math.Floor(float64((width+gap)/(160+gap)))))
	cardWidth := max(float32(150), (width-float32(columns-1)*gap)/float32(columns)-4)
	height := catalogRowHeight(items) + (cardWidth-150)*4/3 + extra
	host.Children(func() {
		c.SetTheme(&gridTheme)
		defer c.SetTheme(&baseTheme)
		ui.GridView(c, state, len(items), 160, height, func(index int) {
			c.SetTheme(&baseTheme)
			defer c.SetTheme(&gridTheme)
			item := items[index]
			a.catalogCardSized(c, item, cardWidth)
			if after != nil {
				after(item)
			}
		}).Grow(1).TrackScroll(scroll).ScrollbarInsets(0, 0, 12, 0)
	})
	return host
}
