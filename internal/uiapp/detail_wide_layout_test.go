package uiapp

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"myguo/internal/ipc"
)

func detailWideFixture() Detail {
	episode := func(number int) CatalogItem {
		return CatalogItem{ID: "123456", EpisodeID: "episode", EpisodeCount: number}
	}
	episodes := make([]CatalogItem, 0, 24)
	for number := 1; number <= 24; number++ {
		episodes = append(episodes, episode(number))
	}
	return Detail{
		ID: "123456", Title: "宽屏详情", Description: "双栏布局验证",
		Source:      &ipc.SourceNovelDTO{ID: "source", Title: "原著小说"},
		Actors:      []ipc.ActorInfo{{Name: "演员甲", UserID: "actor-id"}, {Name: "演员乙"}},
		Episodes:    episodes,
		Seasons:     DetailCollection{Title: "同系列短剧", Items: []CatalogItem{{ID: "season", Title: "同系列短剧"}}},
		Adaptations: DetailCollection{Title: "同一剧本改编", Items: []CatalogItem{{ID: "adaptation", Title: "同一剧本改编"}}},
	}
}

func TestDetailWideLayoutPlacesEpisodesBesideHero(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = detailWideFixture()
	tt := renderTesterAndAdvance(t, app, 1600, 1000)
	hero, heroOK := tt.Find("宽屏详情")
	episodes, episodesOK := tt.Find("选集")
	if !heroOK || !episodesOK {
		t.Fatalf("wide detail sections missing: hero=%v episodes=%v", heroOK, episodesOK)
	}
	if episodes.X < hero.X+hero.W-8 {
		t.Fatalf("episodes did not move to right column: hero=%v episodes=%v", hero, episodes)
	}
	if episodes.Y < hero.Y || episodes.Y > hero.Y+hero.H+700 {
		t.Fatalf("episodes did not align to hero top: hero=%v episodes=%v", hero, episodes)
	}
	first, firstOK := tt.Find("第1集")
	if !firstOK {
		t.Fatal("large first episode button missing")
	}
	if first.W < 80 || first.H < 42 {
		t.Fatalf("wide episode buttons too small: %+v", first)
	}
	loading := NewTestApp()
	loading.detail = Detail{Loading: true}
	loading.OpenDetail("123456")
	loadingTester := ui.NewTester(loading.Render, 1600, 1000)
	loadingTester.Frame()
	loadingTester.Advance(shimmerSampleOffset)
	loadingTester.Frame()
	if _, ok := tt.Find("第1集"); !ok {
		t.Fatal("ready episode placeholder missing")
	}
	if _, ok := loadingTester.Find("同系列短剧"); !ok {
		t.Fatal("loading skeleton omits related collection placeholder")
	}
}

func TestDetailNarrowLayoutStaysSingleColumn(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = detailWideFixture()
	tt := renderTesterAndAdvance(t, app, 1200, 800)
	hero, _ := tt.Find("宽屏详情")
	episodes, episodesOK := tt.Find("选集")
	if !episodesOK {
		t.Fatal("episodes missing")
	}
	if episodes.Y <= hero.Y {
		t.Fatalf("narrow detail unexpectedly used two columns: hero=%v episodes=%v", hero, episodes)
	}
}

func absFloat(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
