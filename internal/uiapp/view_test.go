package uiapp

import (
	"strings"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo/ui"
)

func TestRenderDiscoverShowsAsyncLoadingState(t *testing.T) {
	app := NewTestApp(nil)
	app.catalog.Loading = true
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("正在加载目录...") {
		t.Fatalf("loading text missing: %v", tt.Texts())
	}
}

func TestRenderCatalogShowsItems(t *testing.T) {
	app := NewTestApp(nil)
	app.catalog.Items = []CatalogItem{
		{ID: "123456", Title: "第一剧", EpisodeCount: 12, Heat: "热播"},
		{ID: "223456", Title: "第二剧", EpisodeCount: 8, Heat: "新剧"},
	}
	app.catalog.HasMore = true
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("第一剧") || !tt.HasText("第二剧") {
		t.Fatalf("catalog texts = %v", tt.Texts())
	}
}

func TestRenderLibraryShowsRealLocalEmptyState(t *testing.T) {
	app := NewTestApp(nil)
	app.library.Grids = [4]*ui.GridState{{}, {}, {}, {}}
	app.library.History = Library{Empty: true}
	app.OpenView("history")
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("暂无记录") || !tt.HasText("浏览和播放后才会形成真实记录") {
		t.Fatalf("empty history texts = %v", tt.Texts())
	}
}

func TestRenderErrorAndRetryAreVisible(t *testing.T) {
	app := NewTestApp(nil)
	app.catalog.Error = "目录加载失败，请稍后重试"
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("目录加载失败") || !tt.HasText("重试") {
		t.Fatalf("error texts = %v", tt.Texts())
	}
}

func TestRenderSidebarUsesSVGNavigation(t *testing.T) {
	app := NewTestApp(nil)
	tt := ui.NewTester(app.Render, 1200, 800)
	texts := tt.Texts()
	for _, label := range []string{"推荐", "找剧", "排行榜", "新剧", "历史", "收藏", "预约", "设置"} {
		found := false
		for _, text := range texts {
			if strings.Contains(text, label) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("sidebar label %q missing from %v", label, texts)
		}
	}
}

func TestSidebarClickOpensHistory(t *testing.T) {
	app := NewTestApp(nil)
	tt := ui.NewTester(app.Render, 1200, 800)
	if err := tt.Click("历史"); err != nil {
		t.Fatalf("click history: %v", err)
	}
	waitFor(time.Second, func() bool { return app.View() == "history" })
	tt.Frame()
	if app.View() != "history" {
		t.Fatalf("view after click = %q", app.View())
	}
}

func TestRenderDetailShowsEpisodesAndRelatedState(t *testing.T) {
	app := NewTestApp(NewTestPlayerBridge())
	app.detail = Detail{
		ID: "123456", Title: "详情剧", StatusText: "完结", PlayCount: 321, EpisodeCount: 2,
		Episodes:     []CatalogItem{{ID: "654321", Title: "第1集", EpisodeCount: 1}},
		Related:      []CatalogItem{{ID: "223456", Title: "相关剧", EpisodeCount: 5}},
		RelatedTitle: "相关推荐",
	}
	app.playerOn = true
	app.view = "detail"
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("详情剧") || !tt.HasText("完结 · 321次播放") || !tt.HasText("第1集") || !tt.HasText("相关剧") || !tt.HasText("播放 / 继续观看") {
		t.Fatalf("detail texts = %v", tt.Texts())
	}
}

func TestSlidingTabsSelectedPillDoesNotCollapseWidth(t *testing.T) {
	app := NewTestApp(nil)
	app.rank.Contents = []model.RankContent{
		{ID: "all", Name: "全部"},
		{ID: "series", Name: "系列剧"},
		{ID: "real", Name: "真人剧"},
	}
	app.rank.Content = "real"
	app.view = "rank"
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	elem, ok := tt.Find("真人剧")
	if !ok {
		t.Fatal("expected 真人剧 tab to be found")
	}
	if elem.W < 50 {
		t.Fatalf("selected tab width collapsed: width=%v", elem.W)
	}
}
