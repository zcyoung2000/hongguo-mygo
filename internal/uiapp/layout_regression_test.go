package uiapp

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestPageFadeIsVisibleAndRapidNavigationDropsThePreviousPage(t *testing.T) {
	app := NewTestApp(nil)
	app.view = "search"
	app.catalog.Items = []CatalogItem{{ID: "123456", Title: "过渡中的短剧"}}
	tt := ui.NewTester(app.Render, 1200, 800)
	pixels := func() []byte {
		img := tt.Image()
		var out []byte
		for y := 0; y < 800; y++ {
			out = append(out, img.Pix[y*img.Stride+240*4:y*img.Stride+1200*4]...)
		}
		return out
	}
	app.OpenView("discover")
	tt.Frame()
	first := pixels()
	tt.Advance(60 * time.Millisecond)
	middle := pixels()
	tt.Advance(120 * time.Millisecond)
	last := pixels()
	if bytes.Equal(first, middle) || bytes.Equal(middle, last) {
		t.Fatal("page replacement has no visible fade between frames")
	}
	app.OpenView("search")
	tt.Frame()
	tt.Advance(30 * time.Millisecond)
	app.OpenView("discover")
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if !tt.HasText("过渡中的短剧") || tt.HasText("输入剧名、角色或演员") {
		t.Fatal("rapid navigation retained the outgoing page")
	}
	if tt.FramePending() {
		t.Fatal("page transition did not settle")
	}
}

func TestDiscoverSearchEntryAndCompactFilters(t *testing.T) {
	a := NewTestApp(nil)
	a.catalog.Filters = []FilterRow{{Title: "题材", Options: []FilterOption{{Label: "全部"}}}}
	a.catalog.Items = []CatalogItem{{ID: "123456", Title: "布局回归短剧"}}
	tt := ui.NewTester(a.Render, 1200, 800)
	filter, _ := tt.Find("题材")
	title, ok := tt.Find("布局回归短剧")
	// A single filter must not reserve the former 180px empty panel.
	if !ok || title.Y-filter.Y > 340 {
		t.Fatalf("single filter leaves excessive empty space: filter=%+v title=%+v", filter, title)
	}
	tt.ClickAt(300, 115)
	tt.Type("测试短剧")
	tt.Key(0, ui.KeyEnter)
	waitFor(time.Second, func() bool { return a.View() == "search" })
	if a.Query() != "测试短剧" || a.View() != "search" {
		t.Fatalf("discover search did not submit: query=%q view=%s", a.Query(), a.View())
	}
}

func TestMediaSurfaceFillsContentAndIsRemovedForDetail(t *testing.T) {
	for _, size := range [][2]int{{1000, 640}, {1600, 1000}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			view := &mygo.WebView{}
			app := NewTestApp(nil)
			app.sidebarWidth = 210
			app.SetMediaHost(&MediaHost{view: view})
			app.OpenView("recommend")
			tt := ui.NewTester(app.Render, size[0], size[1])
			assertFull := func() {
				t.Helper()
				for _, point := range [][2]float32{{220, 1}, {float32(size[0] - 1), 1}, {220, float32(size[1] - 1)}, {float32(size[0] - 1), float32(size[1] - 1)}} {
					if tt.WebViewAt(point[0], point[1]) != view {
						t.Fatalf("media leaves a gutter at %v", point)
					}
				}
			}
			assertFull()
			app.detailSource = &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "真实详情标题", EpisodeCount: 1}}}
			app.OpenDetail("123456")
			tt.Frame()
			if !tt.HasText("正在加载详情") || tt.WebViewAt(600, 300) != nil {
				t.Fatal("opening detail leaves the idle media layer over the loading page")
			}
			app.loadDetail("123456")
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			if !tt.HasText("真实详情标题") || !tt.HasText("播放 / 继续观看") || tt.WebViewAt(600, 300) != nil {
				t.Fatalf("detail hidden: web=%v texts=%v", tt.WebViewAt(600, 300), tt.Texts())
			}
			app.PlayDetail()
			tt.Frame()
			assertFull()
			app.ExitMedia()
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			if !tt.HasText("真实详情标题") || tt.WebViewAt(600, 300) != nil {
				t.Fatal("closing playback leaves a black media layer over detail")
			}
		})
	}
}

func TestMediaReadyKeepsLatestSceneAndDoesNotReplayItTwice(t *testing.T) {
	host := &MediaHost{}
	host.Recommend()
	host.StopNow()
	host.Play("123456", 2, -1, 7)
	if host.ready || len(host.queue) != 0 {
		t.Fatal("commands dispatched before the JavaScript scene handler was ready")
	}
	host.markReady()
	host.markReady()
	if len(host.queue) != 1 || host.queue[0]["event"] != "play" || host.queue[0]["session"] != int64(7) {
		t.Fatalf("readiness lost or replayed the latest scene: %+v", host.queue)
	}
}

func TestCollectionPostersFillColumnsAndPreserveAspectRatio(t *testing.T) {
	for _, width := range []int{1000, 1200, 1600} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := NewTestApp(nil)
			a.view = "search"
			for i := range 30 {
				a.search.Items = append(a.search.Items, CatalogItem{ID: fmt.Sprint(i), Title: fmt.Sprintf("海报%d", i)})
			}
			tt := ui.NewTester(a.Render, width, 800)
			first, ok := tt.Find("海报0")
			second, ok2 := tt.Find("海报1")
			if !ok || !ok2 {
				t.Fatal("first grid row missing")
			}
			posterWidth, posterHeight := float64(first.W), float64(first.W*4/3)
			if math.Abs(posterHeight/posterWidth-4.0/3) > .04 || math.Abs(float64(second.X-first.X)-posterWidth-20) > 4 {
				t.Fatalf("poster does not fill its column at 3:4: %.0fx%.0f column step=%.1f", posterWidth, posterHeight, second.X-first.X)
			}
		})
	}
}

func TestCollectionRowsFitRenderedMetadata(t *testing.T) {
	tests := []struct {
		view       string
		item       CatalogItem
		bottomText string
		nextGapMin float64
	}{
		{view: "history", item: CatalogItem{ID: "1", Title: "历史剧", ResumeEpisode: 3, EpisodeCount: 8, UpdatedAt: 1760000000}, bottomText: "3 / 8 集", nextGapMin: 4},
		{view: "favorites", item: CatalogItem{ID: "2", Title: "收藏剧", EpisodeCount: 8}, bottomText: "取消收藏", nextGapMin: 4},
		{view: "likes", item: CatalogItem{ID: "3", Title: "点赞剧", EpisodeCount: 9}, bottomText: "取消点赞", nextGapMin: 4},
		{view: "reservations", item: CatalogItem{ID: "4", Title: "预约剧", PublishAt: "2026-12-01"}, bottomText: "取消预约", nextGapMin: 4},
	}
	for _, test := range tests {
		t.Run(test.view, func(t *testing.T) {
			app := NewTestApp(nil)
			app.view = test.view
			library := Library{Items: []CatalogItem{test.item}}
			switch test.view {
			case "history":
				app.library.History = library
			case "favorites":
				app.library.Favorites = library
			case "likes":
				app.library.Likes = library
			case "reservations":
				app.library.Reserved = library
			}
			tt := ui.NewTester(app.Render, 1000, 1200)
			title, ok := tt.Find(test.item.Title)
			secondary, secondaryOK := tt.Find(test.bottomText)
			if !ok || !secondaryOK {
				t.Fatalf("rendered card content missing: title=%v secondary=%v texts=%v", ok, secondaryOK, tt.Texts())
			}
			gap := float64(secondary.Y + secondary.H - title.Y)
			if gap < test.nextGapMin {
				t.Fatalf("secondary content is clipped: gap=%.1f rects title=%+v secondary=%+v", gap, title, secondary)
			}
			if secondary.Y+secondary.H > 1200 {
				t.Fatalf("secondary content leaves viewport: %+v", secondary)
			}
		})
	}
}

func TestHistoryRowsFitMultiLineTitlesAcrossRows(t *testing.T) {
	app := NewTestApp(nil)
	app.view = "history"
	now := time.Now().Unix()
	app.library.Grids[0] = &ui.GridState{}
	app.library.History = Library{Title: "观看历史", Loaded: true, Items: []CatalogItem{
		{ID: "z1", Title: "谁说纨绔不能出状元：这一世我要亲手改命", EpisodeCount: 80, ResumeEpisode: 1, UpdatedAt: now - 180},
		{ID: "z6", Title: "深海来客：红色进化纪元", EpisodeCount: 88, ResumeEpisode: 23, UpdatedAt: now - 7200},
	}}
	tt := ui.NewTester(app.Render, 1280, 1200)
	tt.Frame()
	title, titleOK := tt.Find("谁说纨绔不能出状元：这一世我要亲手改命")
	progress, progressOK := tt.Find("1 / 80 集")
	timeText, timeOK := tt.Find("3分钟前")
	nextTitle, nextTitleOK := tt.Find("深海来客：红色进化纪元")
	nextProgress, nextProgressOK := tt.Find("23 / 88 集")
	if !titleOK || !progressOK || !timeOK || !nextTitleOK || !nextProgressOK {
		t.Fatalf("history metadata missing: title=%v progress=%v time=%v nextTitle=%v nextProgress=%v texts=%v", titleOK, progressOK, timeOK, nextTitleOK, nextProgressOK, tt.Texts())
	}
	if title.Y > progress.Y || progress.Y > timeText.Y {
		t.Fatalf("first history metadata overlaps: title=%+v progress=%+v time=%+v", title, progress, timeText)
	}
	if nextProgress.Y < nextTitle.Y {
		t.Fatalf("second history row incomplete: title=%+v progress=%+v viewport=1200", nextTitle, nextProgress)
	}
}

func TestSettingsUsesExistingInteractiveHost(t *testing.T) {
	a := NewTestApp(nil)
	a.SetMediaHost(&MediaHost{})
	a.OpenView("settings")
	if !a.hostedScene() {
		t.Fatal("settings command sent but its interactive document is hidden")
	}
	a.OpenView("discover")
	if a.hostedScene() {
		t.Fatal("settings document covers native discover")
	}
}

func TestNavigationCanStopMediaBeforeDocumentReady(t *testing.T) {
	host := &MediaHost{}
	host.Recommend()
	done := make(chan struct{})
	go func() { host.StopNow(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("early navigation deadlocked before the media document was ready")
	}
	if host.desired["event"] != "stop" {
		t.Fatal("early navigation did not cancel the recommendation scene")
	}
}

func TestSearchClearResetsResultsAndInitialPrompt(t *testing.T) {
	a := NewTestApp(nil)
	a.view, a.query = "search", "旧关键词"
	a.search.Items = []CatalogItem{{ID: "123456", Title: "旧搜索结果"}}
	tt := ui.NewTester(a.Render, 1200, 800)
	if err := tt.Click("清空搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Query() != "" || tt.HasText("旧搜索结果") || tt.HasText("暂无搜索结果") || !tt.HasText("输入剧名、角色或演员") {
		t.Fatalf("clear left stale results or misleading empty state: %v", tt.Texts())
	}
}
