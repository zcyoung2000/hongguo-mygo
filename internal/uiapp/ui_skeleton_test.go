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
	app := NewTestApp(nil)
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
	app := NewTestApp(nil)
	app.view = "discover"
	app.catalog.Loading = true
	tester := ui.NewTester(app.Render, 1000, 800)
	tester.Frame()
	tester.Advance(shimmerSweep)
	if !tester.FramePending() {
		t.Fatal("rest phase shimmer did not schedule its next bounded paint frame")
	}
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
		{"detail", "正在加载详情", func(a *App) { a.detail.Loading = true; a.OpenDetail("123456") }},
		{"actor", "正在加载演员主页", func(a *App) { a.actor.Loading = true; a.OpenActor("actor-id") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewTestApp(nil)
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
		`hoverBackground[(]toggle,`,
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
	app := NewTestApp(nil)
	app.view = "discover"
	app.catalog.Loading = true
	tester := ui.NewTester(app.Render, 1000, 800)
	tester.Frame()
	skeleton, skeletonOK := tester.Find("按体裁和筛选条件浏览真实目录")
	if !skeletonOK {
		t.Fatal("discover skeleton missing")
	}
	app.catalog.Loading = false
	app.catalog.Items = []CatalogItem{{ID: "123456", EpisodeID: "654321", Title: "几何稳定样本"}}
	tester.Frame()
	content, contentOK := tester.Find("几何稳定样本")
	if !contentOK {
		t.Fatal("ready content missing")
	}
	if math.Abs(float64(skeleton.X-content.X)) > 2 {
		t.Fatalf("skeleton-to-content left edge shifted: skeleton=%+v content=%+v", skeleton, content)
	}
	if skeleton.W < content.W {
		t.Fatalf("skeleton grid narrower than ready card column: skeleton=%+v content=%+v", skeleton, content)
	}
}
