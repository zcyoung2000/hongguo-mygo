package uiapp

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestNativeCardInteractionUsesSpringAndExplicitArtworkFade(t *testing.T) {
	if artworkFade.Duration != 320*time.Millisecond || !artworkFade.NonMotion {
		t.Fatal("artwork reveal must be paint-only and reduced-motion safe")
	}
}

func TestCatalogFrameBudget(t *testing.T) {
	items := make([]CatalogItem, 240)
	for index := range items {
		items[index] = CatalogItem{
			ID: fmt.Sprintf("series-%d", index), EpisodeID: "1",
			Title: fmt.Sprintf("性能样本 %d", index), CoverURL: fmt.Sprintf("perf:%d", index),
		}
	}
	app := NewTestApp()
	app.catalog.Items = items
	app.OpenView("discover")
	tester := ui.NewTester(app.Render, 1200, 800)
	tester.Frame()
	if tester.FramePending() {
		tester.Advance(320 * time.Millisecond)
		tester.Frame()
		if tester.FramePending() {
			t.Fatal("loaded catalog requested continuous rebuild frames")
		}
	}
}

func TestResponsiveDramaGeometryMatchesAcrossNativePages(t *testing.T) {
	items := []CatalogItem{
		{ID: "one", EpisodeID: "e1", Title: "几何一"},
		{ID: "two", EpisodeID: "e2", Title: "几何二"},
	}
	app := NewTestApp()
	app.view = "search"
	app.search.Items = items
	app.OpenView("search")
	tt := ui.NewTester(app.Render, 1000, 800)
	first, firstOK := tt.Find("几何一")
	second, secondOK := tt.Find("几何二")
	if !firstOK || !secondOK {
		t.Fatal("responsive search grid omitted cards")
	}
	step := float64(second.X - first.X)
	width := float64(first.W)
	if width < 150 || step-width < 20 || step-width > 22 {
		t.Fatalf("responsive cards do not share stable columns: width=%.1f step=%.1f", width, step)
	}
	if posterAspectRatio != float32(4)/float32(3) {
		t.Fatalf("poster aspect ratio changed: %v", posterAspectRatio)
	}
}

func TestNativePagesShareHeaderRhythm(t *testing.T) {
	tests := []struct {
		view    string
		title   string
		prepare func(*App)
	}{
		{"rank", "排行榜", func(a *App) { a.rank.Items = []ipc.RankItemDTO{{Position: 1, Title: "榜单剧", SeriesID: "123456"}} }},
		{"new", "新剧", func(a *App) { a.releases.Items = []CatalogItem{{ID: "123456", Title: "新剧样本"}} }},
		{"search", "搜索", func(a *App) { a.search.Items = []CatalogItem{{ID: "123456", Title: "搜索样本"}} }},
	}
	for _, test := range tests {
		t.Run(test.view, func(t *testing.T) {
			app := NewTestApp()
			test.prepare(app)
			app.OpenView(test.view)
			tt := ui.NewTester(app.Render, 1200, 800)
			header, headerOK := tt.AccessByRole(ui.RoleHeading, test.title)
			cardTitle := map[string]string{"rank": "榜单剧", "new": "新剧样本", "search": "搜索样本"}[test.view]
			card, cardOK := tt.Find(cardTitle)
			if !headerOK || !cardOK {
				t.Fatalf("page header or representative card missing: %v", tt.Texts())
			}
			if header.Bounds.Y > card.Y {
				t.Fatalf("header rhythm drifted: header=%+v card=%+v", header, card)
			}
		})
	}
}

func TestMotionTokensUseMyGoTransitionsWithExplicitChannels(t *testing.T) {
	if feedbackTransition.Duration > 120*1000*1000 || feedbackTransition.Position || feedbackTransition.Size {
		t.Fatal("feedback transition is not short and paint-only")
	}
	if selectionTransition.Duration != 160*1000*1000 || !selectionTransition.Colors || selectionTransition.Position || selectionTransition.Size {
		t.Fatal("selection transition has unstable timing or channels")
	}
	if !layoutTransition.Position || !layoutTransition.Size || !layoutTransition.Colors {
		t.Fatal("layout transition does not declare layout channels")
	}
	if drawerTransition.Duration != 240*1000*1000 || !drawerTransition.Position || drawerTransition.Size {
		t.Fatal("drawer transition uses the wrong surface timing")
	}
}

func TestVirtualGridsUseStableItemKeys(t *testing.T) {
	state := ui.GridState{}
	items := []CatalogItem{
		{ID: "series-1", EpisodeID: "episode-1", Title: "第一部"},
		{ID: "series-2", EpisodeID: "episode-2", Title: "第二部"},
	}
	app := NewTestApp()
	app.view = "search"
	ui.NewTester(app.Render, 1000, 800).Frame()
	configureDramaGrid(&state, items)
	for index, item := range items {
		if state.Key(index) != item.ID+"|"+item.EpisodeID || state.Label(index) != item.Title {
			t.Fatalf("grid key/label drifted at %d: %v %v", index, state.Key(index), state.Label(index))
		}
	}
}

func TestFrontendPerformanceAndHardRuleInvariants(t *testing.T) {
	css := readFile(t, "../../frontend/styles.css")
	liquid := readFile(t, "../../frontend/liquid-controls.css")
	if containsRegexp(css+liquid, `transition:\s*all`) {
		t.Fatal("frontend still animates every property")
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsRegexp(source, pattern string) bool {
	return regexp.MustCompile(pattern).MatchString(source)
}
