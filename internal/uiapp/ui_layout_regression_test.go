package uiapp

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
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

func TestDiscoverPageScrollAndCollapsedFilters(t *testing.T) {
	rows := make([]FilterRow, 7)
	for index := range rows {
		rows[index] = FilterRow{
			Title:   fmt.Sprintf("维度%d", index+1),
			Options: []FilterOption{{Label: "全部", Value: ""}, {Label: fmt.Sprintf("条件%d", index+1), Value: fmt.Sprint(index + 1)}},
		}
	}
	items := make([]CatalogItem, 40)
	for index := range items {
		items[index] = CatalogItem{
			ID: fmt.Sprintf("discover-%d", index), EpisodeID: "1",
			Title: fmt.Sprintf("找剧结果%d", index+1), EpisodeCount: 32,
		}
	}
	app := NewTestApp()
	app.catalog.Filters = rows
	app.catalog.Items = items
	app.catalog.HasMore = true
	app.view = "discover"
	tt := ui.NewTester(app.Render, 1600, 1000)
	tt.Frame()
	_, ok := tt.Find("discover-page-scroll")
	if !ok {
		t.Fatal("discover page scroll missing")
	}
	_, cardOK := tt.Find("找剧结果1")
	if !cardOK {
		t.Fatalf("expanded discover missing card: %v", tt.Texts())
	}
	if app.discoverPageScroll.MaxY <= 0 {
		t.Fatal("discover grid does not contribute fixed page height")
	}

	// 初始态：展开，显示各筛选维度
	if !tt.HasText("维度1") {
		t.Fatal("initial discover should be expanded with filter rows")
	}

	// 1. 下滑越过阈值（> 48px）-> 自动收起
	app.discoverPageScroll.Y = 60
	tt.Frame()
	tt.Advance(260 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("筛选 · 真人剧") {
		t.Fatal("collapsed discover lacks selection summary")
	}
	for index := range rows {
		if tt.HasText(fmt.Sprintf("维度%d", index+1)) {
			t.Fatalf("collapsed filter row remained visible: %d", index+1)
		}
	}
	collapsedPage, _ := tt.Find("discover-page-scroll")
	if collapsedPage.H < 850 {
		t.Fatalf("collapsed page scroll is too short: %.1f", collapsedPage.H)
	}
	secondCard, secondOK := tt.Find("找剧结果9")
	thirdCard, thirdOK := tt.Find("找剧结果17")
	if !secondOK || !thirdOK || secondCard.Y+secondCard.H > collapsedPage.Y+collapsedPage.H || thirdCard.Y > collapsedPage.Y+collapsedPage.H {
		t.Fatalf("collapsed discover does not show two full rows: second=%+v third=%+v page=%+v", secondCard, thirdCard, collapsedPage)
	}

	// 2. 中段滚动防抖（4px < Y <= 48px，例如 20px）-> 保持当前收起态，不翻转
	app.discoverPageScroll.Y = 20
	tt.Frame()
	tt.Advance(260 * time.Millisecond)
	tt.Frame()
	if tt.HasText("维度1") {
		t.Fatal("intermediate scroll jitter: filters should stay collapsed at y=20")
	}
	if !tt.HasText("筛选 · 真人剧") {
		t.Fatal("intermediate scroll should keep collapsed summary")
	}

	// 3. 回到顶部（Y <= 4px，例如 2px）-> 自动展开
	app.discoverPageScroll.Y = 2
	tt.Frame()
	tt.Advance(260 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("维度1") {
		t.Fatal("scrolling to top (y<=4) did not restore expanded filter rows")
	}

	// 4. 从展开态进入中段（例如 30px，未超过 48px）-> 保持展开态，不翻转
	app.discoverPageScroll.Y = 30
	tt.Frame()
	tt.Advance(260 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("维度1") {
		t.Fatal("intermediate scroll jitter: filters should stay expanded at y=30")
	}

	// 导出截图到 MYGUO_NATIVE_SCREENSHOT_DIR, MYGUO_DISCOVER_SHOT_DIR 或 /Users/Henry/hg_shots_discover/
	shotDir := os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR")
	if shotDir == "" {
		shotDir = os.Getenv("MYGUO_DISCOVER_SHOT_DIR")
	}
	if shotDir == "" {
		shotDir = "/Users/Henry/hg_shots_discover"
	}
	if err := os.MkdirAll(shotDir, 0755); err == nil {
		saveShot := func(name string, y float32) {
			app.discoverPageScroll.Y = y
			tt.Frame()
			tt.Advance(260 * time.Millisecond)
			tt.Frame()
			if file, err := os.Create(filepath.Join(shotDir, name)); err == nil {
				defer file.Close()
				_ = png.Encode(file, tt.Image())
			}
		}
		// 展开态截图（y = 0）
		saveShot("discover-expanded.png", 0)
		// 收起态截图（y = 60，越过收起阈值且展示至少2整行卡片）
		saveShot("discover-collapsed.png", 60)
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
