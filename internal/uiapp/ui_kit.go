package uiapp

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	posterAspectRatio float32 = float32(4) / float32(3)
	dramaMinCardWidth float32 = 160
	dramaCardGap      float32 = 14
)

var (
	artworkFade             = ui.Animation{Duration: 320 * time.Millisecond, NonMotion: true}
	cardSpring              = ui.Snappy
	feedbackAnimationValues = ui.Animation{Duration: 120 * time.Millisecond}
)

func stableDramaKey(item CatalogItem, fallback int) any {
	if item.ID != "" || item.EpisodeID != "" {
		return fmt.Sprintf("%s|%s", item.ID, item.EpisodeID)
	}
	return fallback
}

func configureDramaGrid(state *ui.GridState, items []CatalogItem) *ui.GridState {
	if state == nil {
		return nil
	}
	state.Key = func(item int) any { return stableDramaKey(items[item], item) }
	state.Label = func(item int) string { return items[item].Title }
	return state
}

func dramaColumns(width float32) int {
	if width <= 0 {
		return 1
	}
	return max(2, int((width+dramaCardGap)/(dramaMinCardWidth+dramaCardGap)))
}

func dramaCardWidth(width float32) float32 {
	return (width - float32(dramaColumns(width)-1)*dramaCardGap) / float32(dramaColumns(width))
}

func gridContentWidth(c *ui.Context) float32 {
	width, _ := c.Size()
	return max(320, width-360)
}

// dramaSkeleton mirrors the real card geometry before any content arrives,
// so skeletons and loaded grids cannot shift the page.
func dramaSkeleton(c *ui.Context, count int, width float32) {
	cardWidth := dramaCardWidth(width)
	posterHeight := cardWidth * posterAspectRatio
	ui.Row(c).Wrap().Gap(dramaCardGap).Children(func() {
		for range count {
			ui.Column(c).Width(cardWidth).Gap(6).Children(func() {
				ui.Box(c).Width(cardWidth).Height(posterHeight).AspectRatio(posterAspectRatio).
					Radius(8).Background(c.Theme().SurfaceHover)
				ui.Box(c).Height(16).Radius(6).Background(c.Theme().SurfaceHover).FillWidth()
				ui.Box(c).WidthPercent(58).Height(12).Radius(6).Background(c.Theme().SurfaceHover)
			})
		}
	})
}

// pageHeader establishes one shared rhythm across catalog, rank, release,
// actor, search, library and detail pages.
func pageHeader(c *ui.Context, title, subtitle string) {
	ui.Column(c.Key("page-header:"+title)).Gap(2).Padding(0, 0, 4, 0).Children(func() {
		ui.Text(c, title).FontSize(20).Bold().LetterSpacing(-0.4).TextColor(c.Theme().Text).SingleLine()
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
		}
	})
}
