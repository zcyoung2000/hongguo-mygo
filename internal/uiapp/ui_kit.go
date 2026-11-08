package uiapp

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	posterAspectRatio    float32 = float32(4) / float32(3)
	dramaMinCardWidth    float32 = 160
	dramaCardGap         float32 = 14
	catalogFooterHeight  float32 = 40
	pageContentTopSpace  float32 = 14
	pageContentBottomPad float32 = 12
)

const (
	newReleaseMaxCardWidth        float32 = 340
	newReleaseSparsePosterWidth   float32 = 340
	newReleaseSparseContentRatio  float32 = 0.60
	newReleaseSparseGroup3Ratio   float32 = 0.85
	newReleaseSparseMaxGroupRatio float32 = 1.02
	newReleaseSparseChromeHeight  float32 = 320
)

func newReleasePosterWidth(height float32) float32 {
	available := max(newReleaseSparsePosterWidth*posterAspectRatio, height-newReleaseSparseChromeHeight)
	return min(newReleaseSparsePosterWidth, available/posterAspectRatio)
}

var (
	artworkFade             = ui.Animation{Duration: 320 * time.Millisecond, NonMotion: true}
	cardSpring              = ui.Snappy
	feedbackAnimationValues = ui.Animation{Duration: 120 * time.Millisecond}
	drawerTransition        = ui.ElementTransition{
		Duration: 240 * time.Millisecond,
		Ease:     ui.CubicBezier(0.32, 0.72, 0, 1),
		Position: true,
		Colors:   true,
	}
)

// hoverBackground always names the idle target first, so a pointer leaving
// interrupts toward the correct color instead of freezing at the hovered one.
func hoverBackground(element ui.Element, idle, hovered ui.Color) ui.Element {
	background := element.AnimateColor("background", idle, feedbackAnimationValues)
	if element.Hovered() {
		background = element.AnimateColor("background", hovered, feedbackAnimationValues)
	}
	element.Background(background)
	pose := ui.Pose{}
	if element.Pressed() {
		pose.Scale = 0.98
	}
	element.AnimatePose(pose, cardSpring)
	return element
}

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

func (a *App) gridContentWidth(c *ui.Context) float32 {
	sidebarWidth := a.sidebarWidth
	if sidebarWidth == 0 {
		sidebarWidth = defaultSidebarWidth
	}
	width, _ := c.Size()
	return max(320, width-sidebarWidth-32)
}

// dramaSkeleton mirrors the real card geometry before any content arrives,
// so skeletons and loaded grids cannot shift the page.
func dramaSkeleton(c *ui.Context, count int, width float32) {
	skeleton := pageShimmer(c)
	cardWidth := dramaCardWidth(width)
	posterHeight := cardWidth * posterAspectRatio
	ui.Row(c.Key("drama-skeleton")).Wrap().Gap(dramaCardGap).Children(func() {
		for index := range count {
			ui.Column(c.Key(index)).Width(cardWidth).Gap(6).Children(func() {
				skeleton.bone(c.Key("poster"), 8).Width(cardWidth).Height(posterHeight).
					AspectRatio(posterAspectRatio).Role(ui.RoleImage).Label("加载海报占位")
				skeleton.text(c.Key("title"), 13, 1.3).WidthPercent(92)
				skeleton.text(c.Key("metadata"), 12, 1.3).WidthPercent(boneWidth(index))
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

func sectionHeader(c *ui.Context, title, detail string) {
	ui.Row(c.Key("section-header:" + title)).AlignItems(ui.Baseline).Gap(10).Children(func() {
		ui.Text(c, title).FontSize(17).Bold().TextColor(c.Theme().Text)
		if detail != "" {
			ui.Text(c, detail).FontSize(12).TextColor(c.Theme().TextMuted)
		}
	})
}
