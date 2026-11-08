package uiapp

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func responsivePageCases() []struct {
	name    string
	view    string
	header  string
	title   string
	prepare func(*App)
} {
	return []struct {
		name    string
		view    string
		header  string
		title   string
		prepare func(*App)
	}{
		{"discover", "discover", "发现短剧", "响应收敛发现", func(a *App) {
			a.catalog.Items = []CatalogItem{
				{ID: "discover-1", EpisodeID: "e1", Title: "响应收敛发现"},
				{ID: "discover-2", EpisodeID: "e2", Title: "响应收敛发现二"},
			}
		}},
		{"search", "search", "搜索", "响应收敛搜索", func(a *App) {
			a.query = "收敛"
			a.search.Items = []CatalogItem{
				{ID: "search-1", EpisodeID: "e1", Title: "响应收敛搜索"},
				{ID: "search-2", EpisodeID: "e2", Title: "响应收敛搜索二"},
			}
		}},
		{"rank", "rank", "排行榜", "响应收敛榜单", func(a *App) {
			a.rank.Items = []ipc.RankItemDTO{
				{Position: 1, SeriesID: "rank-1", Title: "响应收敛榜单", EpisodeCount: 20},
				{Position: 2, SeriesID: "rank-2", Title: "响应收敛榜单二", EpisodeCount: 18},
			}
		}},
		{"new", "new", "新剧", "响应收敛新剧", func(a *App) {
			a.releases.Items = []CatalogItem{
				{ID: "new-1", EpisodeID: "e1", Title: "响应收敛新剧"},
				{ID: "new-2", EpisodeID: "e2", Title: "响应收敛新剧二"},
			}
		}},
		{"history", "history", "观看历史", "响应收敛历史", func(a *App) {
			a.library.History = Library{Items: []CatalogItem{
				{ID: "history-1", EpisodeID: "e1", Title: "响应收敛历史", ResumeEpisode: 2, EpisodeCount: 10, UpdatedAt: 1760000000},
				{ID: "history-2", EpisodeID: "e2", Title: "响应收敛历史二", ResumeEpisode: 1, EpisodeCount: 10},
			}}
		}},
		{"favorites", "favorites", "我的收藏", "响应收敛收藏", func(a *App) {
			a.library.Favorites = Library{Items: []CatalogItem{
				{ID: "favorite-1", EpisodeID: "e1", Title: "响应收敛收藏", EpisodeCount: 10},
				{ID: "favorite-2", EpisodeID: "e2", Title: "响应收敛收藏二", EpisodeCount: 10},
			}}
		}},
		{"likes", "likes", "我的点赞", "响应收敛点赞", func(a *App) {
			a.library.Likes = Library{Items: []CatalogItem{
				{ID: "like-1", EpisodeID: "e1", Title: "响应收敛点赞", EpisodeCount: 10},
				{ID: "like-2", EpisodeID: "e2", Title: "响应收敛点赞二", EpisodeCount: 10},
			}}
		}},
		{"reservations", "reservations", "我的预约", "响应收敛预约", func(a *App) {
			a.library.Reserved = Library{Items: []CatalogItem{
				{ID: "reservation-1", EpisodeID: "e1", Title: "响应收敛预约", EpisodeCount: 10},
				{ID: "reservation-2", EpisodeID: "e2", Title: "响应收敛预约二", EpisodeCount: 10},
			}}
		}},
		{"actor", "actor", "演员主页", "响应收敛演员", func(a *App) {
			a.actor.Profile = &ipc.ActorProfileDTO{UserID: "actor-id", Name: "演员甲", Description: "简介"}
			a.actor.Works = []CatalogItem{
				{ID: "actor-work-1", EpisodeID: "e1", Title: "响应收敛演员", EpisodeCount: 20},
				{ID: "actor-work-2", EpisodeID: "e2", Title: "响应收敛演员二", EpisodeCount: 20},
			}
		}},
		{"detail", "detail", "短剧详情", "响应收敛详情", func(a *App) {
			a.detail = Detail{ID: "detail-1", Title: "响应收敛详情", Description: "剧情简介", EpisodeCount: 2}
		}},
	}
}

func TestRepresentativePagesResponsiveConvergence(t *testing.T) {
	sizes := [][2]int{{900, 700}, {1200, 800}, {1600, 1000}}
	expectedColumns := map[int]int{900: 2, 1200: 4, 1600: 6}
	for _, page := range responsivePageCases() {
		for _, size := range sizes {
			t.Run(fmt.Sprintf("%s/%dx%d", page.name, size[0], size[1]), func(t *testing.T) {
				app := NewTestApp()
				app.view = page.view
				page.prepare(app)
				tester := ui.NewTester(app.Render, size[0], size[1])
				tester.SetFocused(false)
				tester.Frame()

				if !tester.HasText(page.header) {
					t.Fatalf("%s page header missing: %v", page.name, tester.Texts())
				}
				content, contentOK := tester.Find(page.title)
				if !contentOK {
					t.Fatalf("%s representative content missing: %v", page.name, tester.Texts())
				}
				if content.X < 0 || content.Y < 0 || content.X+content.W > float32(size[0]) || content.Y+content.H > float32(size[1]) {
					t.Fatalf("%s content extends beyond %dx%d: %+v", page.name, size[0], size[1], content)
				}

				tester.Advance(500 * time.Millisecond)
				tester.Frame()
				if tester.FramePending() {
					t.Fatalf("%s at %dx%d requested continuous frames", page.name, size[0], size[1])
				}

				if page.name == "detail" || page.name == "rank" {
					return
				}

				if page.name == "new" {
					first, firstOK := tester.Find(page.title)
					if !firstOK {
						t.Fatalf("%s first card missing at %dx%d: %v", page.name, size[0], size[1], tester.Texts())
					}
					if first.W < 240 {
						t.Fatalf("%s sparse content does not use the large-card layout: %+v", page.name, first)
					}
					return
				}

				tester.ClickAt(float32(size[0]-20), float32(size[1]-10))
				tester.Frame()

				first, firstOK := tester.Find(page.title)
				second, secondOK := tester.Find(page.title + "二")
				if !firstOK || !secondOK {
					t.Fatalf("%s card grid incomplete at %dx%d: first=%v second=%v", page.name, size[0], size[1], firstOK, secondOK)
				}
				width := float64(first.W)
				step := float64(second.X - first.X)
				gap := step - width
				if width < float64(dramaMinCardWidth) {
					t.Fatalf("%s card width too small at %dx%d: %.1f", page.name, size[0], size[1], width)
				}
				if gap < 0 || gap > 24 {
					t.Fatalf("%s unstable column step at %dx%d: width=%.1f step=%.1f gap=%.1f", page.name, size[0], size[1], width, step, gap)
				}
				expectedWidth := float64(first.W)*float64(expectedColumns[size[0]]) + gap*float64(expectedColumns[size[0]]-1)
				if got := dramaColumns(float32(expectedWidth)); got != expectedColumns[size[0]] {
					t.Fatalf("%s unexpected responsive column model at %dx%d: got=%d want=%d", page.name, size[0], size[1], got, expectedColumns[size[0]])
				}
			})
		}
	}
}

func TestNewReleasesUseVisibleWidthWhenFewCards(t *testing.T) {
	sizes := [][2]int{{1600, 1000}, {1200, 800}}
	for _, size := range sizes {
		for _, count := range []int{1, 3} {
			name := fmt.Sprintf("%dx%d/%d-card", size[0], size[1], count)
			t.Run(name, func(t *testing.T) {
				app := NewTestApp()
				app.view = "new"
				app.releases.Calendar = true
				app.releases.Dates = []string{"20261109"}
				app.releases.Date = "20261109"
				for index := range count {
					app.releases.Items = append(app.releases.Items, CatalogItem{
						ID: fmt.Sprintf("new-%d", index), EpisodeID: "1",
						Title: fmt.Sprintf("少卡新剧%d", index), Upcoming: true,
					})
				}
				tester := ui.NewTester(app.Render, size[0], size[1])
				tester.Frame()
				contentWidth := max(float64(newReleaseSparsePosterWidth), float64(size[0])-float64(defaultSidebarWidth)-32)
				minRatio := float32(1)
				visibleCount := 0
				var narrowPanel ui.Rect
				for index := range count {
					panel, panelOK := tester.Find(fmt.Sprintf("release-feature-bounds:new-%d|1", index))
					if !panelOK {
						t.Fatalf("sparse release feature panel missing: %v", tester.Texts())
					}
					posterWidth := newReleasePosterWidth(float32(min(size[1], 700)))
					posterHeight := posterWidth * posterAspectRatio
					if math.Abs(float64(posterWidth-newReleaseSparsePosterWidth)) > 0.01 ||
						math.Abs(float64(posterAspectRatio-float32(4)/float32(3))) > 0.001 {
						t.Fatalf("sparse poster size or ratio changed at %dx%d: posterWidth=%.2f ratio=%.4f", size[0], size[1], posterWidth, posterAspectRatio)
					}
					if panel.H > posterHeight+0.01 {
						t.Fatalf("feature panel exceeds poster group height at %dx%d: panel=%+v posterHeight=%.2f", size[0], size[1], panel, posterHeight)
					}
					if panel.W <= 0 || panel.H <= 0 {
						continue
					}
					visibleCount++
					minRatio = min(minRatio, panel.W/float32(contentWidth))
					if index == 0 || panel.W < narrowPanel.W {
						narrowPanel = panel
					}
				}
				wantRatio := float64(newReleaseSparseContentRatio)
				if count == 3 {
					wantRatio = float64(newReleaseSparseGroup3Ratio)
				}
				ratio := float64(minRatio)
				if ratio < wantRatio {
					t.Fatalf("sparse release group too narrow at %dx%d: panel=%+v ratio=%.3f want>=%.3f", size[0], size[1], narrowPanel, ratio, wantRatio)
				}
				if visibleCount == 0 {
					t.Fatalf("no sparse release feature rows visible at %dx%d", size[0], size[1])
				}
				if ratio > float64(newReleaseSparseMaxGroupRatio) {
					t.Fatalf("sparse release group exceeds available width at %dx%d: panel=%+v ratio=%.3f want<=%.3f", size[0], size[1], narrowPanel, ratio, float64(newReleaseSparseMaxGroupRatio))
				}
			})
		}
	}
}

func TestNewReleasesKeepCatalogDensityWhenFull(t *testing.T) {
	app := NewTestApp()
	app.view = "new"
	app.releases.Calendar = true
	app.releases.Dates = []string{"20261109"}
	app.releases.Date = "20261109"
	for index := range 6 {
		app.releases.Items = append(app.releases.Items, CatalogItem{
			ID: fmt.Sprintf("full-%d", index), EpisodeID: "1",
			Title: fmt.Sprintf("满行新剧%d", index), Upcoming: true,
		})
	}
	tester := ui.NewTester(app.Render, 1600, 1000)
	tester.SetFocused(false)
	tester.Frame()
	first, firstOK := tester.Find("满行新剧0")
	third, thirdOK := tester.Find("满行新剧2")
	if !firstOK || !thirdOK {
		t.Fatalf("full release row incomplete: %v", tester.Texts())
	}
	if first.W > 230 {
		t.Fatalf("full releases changed catalog density: first=%+v", first)
	}
	if third.X < first.X+2*(first.W+18) || third.X > first.X+2*(first.W+24) {
		t.Fatalf("full releases did not use the normal column step: first=%+v third=%+v", first, third)
	}
}

func TestDiscoverFiveThousandItemsStayVirtualized(t *testing.T) {
	items := make([]CatalogItem, 5000)
	for index := range items {
		items[index] = CatalogItem{
			ID: fmt.Sprintf("catalog-%d", index), EpisodeID: "1",
			Title: fmt.Sprintf("五千米样本 %d", index), CoverURL: fmt.Sprintf("perf:%d", index),
		}
	}
	items[0].Title = "响应收敛首项"
	app := NewTestApp()
	app.catalog.Items = items
	app.view = "discover"
	tester := ui.NewTester(app.Render, 1200, 800)
	tester.Frame()
	if !tester.HasText("响应收敛首项") {
		t.Fatal("virtual catalog omitted first visible item")
	}
	texts := tester.Texts()
	visible := 0
	for _, text := range texts {
		if len(text) >= len("五千米样本 0") && text[:len("五千米样本")] == "五千米样本" {
			visible++
		}
	}
	if visible == 0 || visible >= 1000 {
		t.Fatalf("catalog viewport item count is not bounded: %d", visible)
	}
	tester.Advance(500 * time.Millisecond)
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("5000-item catalog requested continuous rebuild frames")
	}
	if app.catalogGrid == nil || app.catalogGrid.Key == nil {
		t.Fatal("discover catalog did not own a stable-keyed virtual grid state")
	}
	for index := 0; index < 3; index++ {
		if app.catalogGrid.Key(index) != fmt.Sprintf("catalog-%d|1", index) || app.catalogGrid.Label(index) != items[index].Title {
			t.Fatalf("catalog key/label drifted at %d", index)
		}
	}
}

func TestAllCatalogCollectionsUseSharedVirtualPaths(t *testing.T) {
	source := readFile(t, "view.go") + readFile(t, "feature_views.go")
	required := map[string][]string{
		"discover":    {"a.catalogGrid = &ui.GridState{}", "a.cardGrid(c, \"catalog-grid\", a.catalogGrid"},
		"search":      {"a.searchGrid = &ui.GridState{}", "a.cardGrid(c, \"search-grid\", a.searchGrid"},
		"new":         {"a.releaseGrid = &ui.GridState{}", "a.cardGrid(c, \"releases-grid\", a.releaseGrid"},
		"actor":       {"a.actorWorksGrid = &ui.GridState{}", "a.cardGrid(c, \"actor-grid\", a.actorWorksGrid"},
		"library all": {"a.library.Grids[grid]", "a.cardGrid(c, \"library-grid\", a.library.Grids[grid]"},
		"rank":        {"a.rank.List = &ui.ListState{}", "ui.List(c, a.rank.List"},
	}
	for page, patterns := range required {
		for _, pattern := range patterns {
			if !strings.Contains(source, pattern) {
				t.Fatalf("%s does not use the expected virtualized collection path: %s", page, pattern)
			}
		}
	}
	if strings.Contains(source, "ui.Grid(c.Key(\"catalog-grid-flow\"))") {
		t.Fatal("discover still renders an unbounded ui.Grid")
	}
}

func TestCompletionAuditHardRules(t *testing.T) {
	readme := readFile(t, "../../README.md")
	app := readFile(t, "../../main.go")
	if containsRegexp(app, `github.com/mattn/go-sqlite3|import "C"`) {
		t.Fatal("CGO-bound runtime dependency found")
	}
	if !containsRegexp(readme, `CGO_ENABLED=0`) {
		t.Fatal("CGO-free build guidance missing")
	}
	frontendApp := readFile(t, "../../frontend/app.js")
	playerChrome := readFile(t, "../../frontend/player-chrome.js")
	for _, source := range []string{frontendApp, playerChrome} {
		for _, match := range regexp.MustCompile(`(?s)<svg[^>]*>(.*?)</svg>`).FindAllString(source, -1) {
			if !containsRegexp(match, `currentColor`) {
				t.Fatalf("inline SVG lacks currentColor: %.160s", match)
			}
		}
	}
	for _, path := range []string{"frontend/app.js", "frontend/index.html"} {
		source := readFile(t, "../../"+path)
		for _, match := range regexp.MustCompile(`emoji|🎉|🚀`).FindAllString(source, -1) {
			t.Fatalf("emoji found in %s: %s", path, match)
		}
	}
	if !containsRegexp(frontendApp, `OpenHongguo|openhongguo|apiCall\(`) {
		t.Fatal("frontend does not use the exported apiCall/OpenHongguo boundary")
	}
	if !containsRegexp(readFile(t, "../../internal/store/store.go"), `WAL`) {
		t.Fatal("SQLite WAL mode invariant missing")
	}
	if !containsRegexp(readFile(t, "../../internal/store/store.go"), `userdata.db`) {
		t.Fatal("SQLite expected path invariant missing")
	}
	if !containsRegexp(readFile(t, "../../internal/media/server.go"), `MediaServer|media server|127.0.0.1`) {
		t.Fatal("local media server invariant missing")
	}
}
