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
	tt.ClickAt(300, 55)
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
			img := tt.Image()
			// With no artwork loader, the poster surface is the real fallback.
			x, y := int(first.X+20), int(first.Y+20)
			color := img.At(x, y)
			left, right, top, bottom := x, x, y, y
			for left > 0 && img.At(left-1, y) == color {
				left--
			}
			for right+1 < width && img.At(right+1, y) == color {
				right++
			}
			// Measure height away from rounded corners.
			x = (left + right) / 2
			for top > 0 && img.At(x, top-1) == color {
				top--
			}
			for bottom+1 < 800 && img.At(x, bottom+1) == color {
				bottom++
			}
			posterWidth, posterHeight := float64(right-left+1), float64(bottom-top+1)
			if math.Abs(posterHeight/posterWidth-4.0/3) > .04 || math.Abs(float64(second.X-first.X)-posterWidth-20) > 4 {
				t.Fatalf("poster does not fill its column at 3:4: %.0fx%.0f column step=%.1f", posterWidth, posterHeight, second.X-first.X)
			}
		})
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
