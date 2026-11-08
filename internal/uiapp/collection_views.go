package uiapp

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// cardGrid gives posters the full column width and preserves their 3:4 ratio.
// Read the host's last layout, just as GridView does when fitting its columns.
func (a *App) cardGrid(c *ui.Context, key string, state *ui.GridState, scroll *ui.ScrollState, items []CatalogItem, extra float32, after func(CatalogItem)) ui.Element {
	host := ui.Column(c.Key(key)).MinWidth(0).MinHeight(0)
	width := host.Bounds().W
	baseTheme := *c.Theme()
	gridTheme := baseTheme
	gridTheme.Spacing = 11 // Keep the visible card gap near 14 DIPs; GridView uses Space(2).
	gap := gridTheme.Space(2)
	columns := max(1, int((width+gap)/(dramaMinCardWidth+gap)))
	cardWidth := max(150, (width-float32(columns-1)*gap)/float32(columns)-4)
	configureDramaGrid(state, items)
	textHeight := a.collectionRowHeight(c, cardWidth, items)
	height := textHeight + cardWidth*posterAspectRatio + extra
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

const (
	catalogButtonHeight   float32 = 28
	catalogTextLineHeight float32 = 20
	catalogRowGap         float32 = 6
)

func (a *App) collectionRowHeight(c *ui.Context, cardWidth float32, items []CatalogItem) float32 {
	var tallest float32
	for _, item := range items {
		height := catalogTextLineHeight
		switch a.view {
		case "history", "reservations":
			height += 8
		case "new":
			if item.Upcoming {
				if item.PublishAt != "" {
					height += catalogTextLineHeight
				}
				height += catalogButtonHeight
			} else {
				height += catalogTextLineHeight
				if item.Heat != "" {
					_, heatHeight := c.MeasureText(cardWidth, ui.Span{Text: item.Heat, Size: 12, Weight: 700})
					height += max(catalogTextLineHeight, heatHeight)
				}
			}
		default:
			if item.Upcoming {
				height += measuredTextHeight(c, cardWidth, formatPublishDate(item.PublishAt), 13) + catalogButtonHeight
			} else {
				meta := item.Heat
				if item.EpisodeCount > 0 {
					meta = fmt.Sprintf("%d集", item.EpisodeCount)
					if item.Heat != "" {
						meta += " · " + item.Heat
					}
				}
				height += measuredTextHeight(c, cardWidth, meta, 13)
			}
		}
		tallest = max(tallest, height)
	}
	return tallest
}

func measuredTitleHeight(c *ui.Context, width float32, title string) float32 {
	_, height := c.MeasureText(width, ui.Span{Text: title, Size: 13, Weight: 700})
	return max(catalogTextLineHeight, height)
}

func measuredTextHeight(c *ui.Context, width float32, text string, size float32, weight ...int) float32 {
	if text == "" {
		return 0
	}
	spanWeight := 400
	if len(weight) > 0 {
		spanWeight = weight[0]
	}
	_, height := c.MeasureText(width, ui.Span{Text: text, Size: size, Weight: spanWeight})
	return max(catalogTextLineHeight, height)
}
