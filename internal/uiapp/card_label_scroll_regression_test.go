package uiapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestRemovedPageContextCardLabelsStayRemoved(t *testing.T) {
	online := CatalogItem{ID: "online", EpisodeID: "1", Title: "热度上线样本", Heat: "11578万热度"}

	t.Run("new page hot list", func(t *testing.T) {
		app := NewTestApp()
		app.view = "new"
		app.releases.Items = []CatalogItem{online}
		tester := ui.NewTester(func(c *ui.Context) {
			app.catalogCardSized(c, online, 180)
		}, 180, 420)
		tester.Frame()
		for _, removed := range []string{"已上线", "新剧", "收藏"} {
			if tester.HasText(removed) {
				t.Fatalf("removed card label %q is still rendered: %v", removed, tester.Texts())
			}
		}
		if !tester.HasText("11578万热度") {
			t.Fatalf("heat metadata was removed: %v", tester.Texts())
		}
	})

	t.Run("new page feature row", func(t *testing.T) {
		app := NewTestApp()
		app.view = "new"
		app.releases.Items = []CatalogItem{{ID: "feature", EpisodeID: "1", Title: "特写上线样本"}}
		item := app.releases.Items[0]
		tester := ui.NewTester(func(c *ui.Context) {
			app.releaseFeatureRow(c, item, 320)
		}, 1200, 700)
		tester.Frame()
		if tester.HasText("已上线") {
			t.Fatalf("feature row still renders the removed online badge: %v", tester.Texts())
		}
	})
}

func TestRemovedFavoritePageCardLabel(t *testing.T) {
	app := NewTestApp()
	app.view = "favorites"
	item := CatalogItem{ID: "favorite", EpisodeID: "1", Title: "剧集样本", EpisodeCount: 12}
	app.library.Favorites = Library{Items: []CatalogItem{item}}
	tester := ui.NewTester(func(c *ui.Context) {
		app.catalogCardSized(c, item, 180)
	}, 1000, 420)
	tester.Frame()
	for _, text := range tester.Texts() {
		if text == "收藏" {
			t.Fatalf("favorites page still renders the page-context favorite badge: %v", tester.Texts())
		}
	}
	if !tester.HasText("12集") || !tester.HasText("取消收藏") {
		t.Fatalf("episode badge or action missing: %v", tester.Texts())
	}
}

func TestSearchCardRemovesPageContextLabel(t *testing.T) {
	app := NewTestApp()
	app.view = "search"
	item := CatalogItem{ID: "search", EpisodeID: "1", Title: "搜索样本", EpisodeCount: 812, BadgeText: "搜索结果"}
	app.search.Items = []CatalogItem{item}
	tester := ui.NewTester(func(c *ui.Context) {
		app.catalogCardSized(c, item, 180)
	}, 1000, 420)
	tester.Frame()
	if tester.HasText("搜索结果") {
		t.Fatalf("search card still renders the page-context label: %v", tester.Texts())
	}
	if !tester.HasText("812集") {
		t.Fatalf("search episode metadata missing: %v", tester.Texts())
	}
}

func TestHistoryProgressBadgeSurvivesLabelCleanup(t *testing.T) {
	app := NewTestApp()
	app.view = "history"
	app.library.History = Library{Items: []CatalogItem{{
		ID: "history", EpisodeID: "1", Title: "历史进度样本",
		EpisodeCount: 80, ResumeEpisode: 3, UpdatedAt: 1_700_000_000,
	}}}
	tester := ui.NewTester(app.renderBody, 1200, 900)
	tester.Frame()
	if !tester.HasText("3 / 80集") {
		t.Fatalf("history progress badge missing: %v", tester.Texts())
	}
}

func TestNewCardRowHeightShrinksAfterLabelRemoval(t *testing.T) {
	app := NewTestApp()
	app.view = "new"
	item := CatalogItem{ID: "height", EpisodeID: "1", Title: "行高样本", Heat: "热度"}
	app.releases.Items = []CatalogItem{item}
	tester := ui.NewTester(func(c *ui.Context) {
		app.cardGrid(c, "releases-grid", app.releaseGrid, nil, app.releases.Items, 0, nil)
	}, 180, 420)
	tester.Frame()
	poster, posterOK := tester.Find("poster:height|1")
	heat, heatOK := tester.Find("热度")
	if !posterOK || !heatOK {
		t.Fatalf("new card content missing: poster=%v heat=%v", posterOK, heatOK)
	}
	contentHeight := heat.Y + heat.H - poster.Y
	expectedHeight := poster.W*posterAspectRatio + catalogRowGap + 2*catalogTextLineHeight
	if diff := contentHeight - expectedHeight; diff < -2 || diff > 2 {
		t.Fatalf("new card content did not shrink: content=%.1f expected=%.1f poster=%+.1f heat=%+.1f", contentHeight, expectedHeight, poster, heat)
	}
}
