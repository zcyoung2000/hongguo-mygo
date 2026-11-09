package uiapp

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestShimmerSystemUsesSharedBoundedTiming(t *testing.T) {
	if shimmerPeriod != 1600*time.Millisecond || shimmerSweep != 1100*time.Millisecond || shimmerBand != 260 {
		t.Fatal("shimmer timing diverged from the shared MyGo skeleton system")
	}
	if shimmerFrameRate != 30 {
		t.Fatal("shimmer must explicitly use a bounded 30fps paint schedule")
	}
}

func TestShimmerRespectsReducedMotion(t *testing.T) {
	app := NewTestApp()
	app.catalog.Loading = true
	tester := ui.NewTester(app.Render, 1000, 800)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	if !tester.HasText("正在加载目录") {
		t.Fatal("loading view missing")
	}
	tester.Advance(shimmerPeriod)
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("reduced-motion shimmer scheduled continuous paint frames")
	}
}

func TestLoadingShimmerAnimatesThenSettles(t *testing.T) {
	app := NewTestApp()
	app.view = "discover"
	app.catalog.Loading = true
	tester := ui.NewTester(app.Render, 1000, 800)
	tester.Frame()
	tester.Advance(shimmerSweep)
	tester.Frame()
	tester.Advance(shimmerPeriod - shimmerSweep)
	tester.Frame()
	tester.Advance(shimmerPeriod)
	tester.Frame()
	app.catalog.Loading = false
	app.catalog.Items = []CatalogItem{{ID: "123456", EpisodeID: "654321", Title: "加载完成样本"}}
	tester.Frame()
	tester.Advance(shimmerPeriod)
	tester.Frame()
	if !tester.HasText("加载完成样本") || tester.HasText("正在加载目录") {
		t.Fatal("loading state did not transition to content")
	}
	if tester.FramePending() {
		t.Fatal("completed loading view still requested continuous frames")
	}
}

func TestRepresentativeLoadingViewsShareShimmer(t *testing.T) {
	tests := []struct {
		name    string
		loading string
		prepare func(*App)
	}{
		{"discover", "正在加载目录", func(a *App) { a.catalog.Loading = true }},
		{"search", "正在搜索", func(a *App) { a.view = "search"; a.search.Loading = true }},
		{"library", "正在加载本地数据", func(a *App) { a.library.History.Loading = true; a.OpenView("history") }},
		{"rank", "正在加载排行榜", func(a *App) { a.rank.Loading = true; a.OpenView("rank") }},
		{"new", "正在加载新剧", func(a *App) { a.releases.Loading = true; a.OpenView("new") }},
		{"detail", "短剧详情", func(a *App) { a.detail.Loading = true; a.OpenDetail("123456") }},
		{"actor", "正在加载演员主页", func(a *App) { a.actor.Loading = true; a.OpenActor("actor-id") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewTestApp()
			test.prepare(app)
			tester := ui.NewTester(app.Render, 1000, 800)
			tester.Advance(shimmerSampleOffset)
			tester.Frame()
			if !tester.HasText(test.loading) {
				t.Fatalf("%s loading state missing: %v", test.name, tester.Texts())
			}
		})
	}
}

func TestHoverFeedbackCoversClickableNativeSurfaces(t *testing.T) {
	source := readFile(t, "view.go") + readFile(t, "feature_views.go") + readFile(t, "feed_chrome.go")
	for _, pattern := range []string{
		`hoverBackground[(]row,`,
		`hoverBackground[(]chip,`,
		`hoverBackground[(]card,`,
		`hoverBackground[(]actBtn,`,
		`hoverBackground[(]playBtn,`,
		`hoverBackground[(]backBtn,`,
		`hoverBackground[(]epBtn,`,
	} {
		if !containsRegexp(source, pattern) {
			t.Fatalf("hover feedback pattern missing: %s", pattern)
		}
	}
	if containsRegexp(source, `Transition[(]ui[.]ElementTransition[{]Duration: 150 [*] time[.]Millisecond`) {
		t.Fatal("legacy inline hover transition found")
	}
}

func TestSkeletonGeometryMatchesReadyContent(t *testing.T) {
	app := NewTestApp()
	app.view = "search"
	app.search.Loading = true
	tester := ui.NewTester(app.Render, 1600, 1000)
	tester.Frame()
	loadingBar, loadingBarOK := tester.Find("搜索短剧")
	loadingHeader, loadingHeaderOK := tester.Find("搜索")
	skeleton, skeletonOK := tester.AccessByRole(ui.RoleImage, "加载海报占位")
	if !skeletonOK {
		t.Fatal("search skeleton bone missing")
	}
	if !loadingBarOK || !loadingHeaderOK {
		t.Fatalf("search loading page missing: %v", tester.Texts())
	}
	if loadingHeader.Y > loadingBar.Y {
		t.Fatalf("search header is not rendered before the search bar: header=%+v bar=%+v", loadingHeader, loadingBar)
	}
	app.search.Loading = false
	app.search.Items = []CatalogItem{{ID: "123456", EpisodeID: "654321", Title: "几何稳定样本"}}
	tester.Frame()
	content, contentOK := tester.Find("几何稳定样本")
	readyBar, readyBarOK := tester.Find("搜索短剧")
	if !contentOK || !readyBarOK {
		t.Fatal("search ready content missing")
	}
	if math.Abs(float64(skeleton.Bounds.X-content.X)) > 2 {
		t.Fatalf("skeleton-to-content left edge shifted: skeleton=%+v content=%+v", skeleton.Bounds, content)
	}
	widthTolerance := max(2, content.W*0.03)
	if math.Abs(float64(loadingBar.X-readyBar.X)) > 2 || math.Abs(float64(loadingBar.Y-readyBar.Y)) > 2 {
		t.Fatalf("search bar shifted between loading and ready: loading=%+v ready=%+v", loadingBar, readyBar)
	}
	if math.Abs(float64(skeleton.Bounds.W-content.W)) > float64(widthTolerance) {
		t.Fatalf("skeleton bone does not use the visible grid width: bone=%+.2f card=%+.2f", skeleton.Bounds.W, content.W)
	}
}
