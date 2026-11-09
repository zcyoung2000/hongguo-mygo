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
	episodes := make([]CatalogItem, 0, 100)
	for number := 1; number <= 100; number++ {
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

func emptyDetailFixture() Detail {
	fixture := detailWideFixture()
	fixture.Seasons = DetailCollection{}
	fixture.Adaptations = DetailCollection{}
	return fixture
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
	seasons, seasonsOK := tt.Find("同系列短剧")
	adaptations, adaptationsOK := tt.Find("同一剧本改编")
	if !seasonsOK || !adaptationsOK {
		t.Fatalf("wide detail collections missing: seasons=%v adaptations=%v", seasonsOK, adaptationsOK)
	}
	if seasons.X >= episodes.X || adaptations.X >= episodes.X {
		t.Fatalf("wide detail collections moved to the right column: seasons=%v adaptations=%v episodes=%v", seasons, adaptations, episodes)
	}
	first, firstOK := tt.Find("第1集")
	if !firstOK {
		t.Fatal("large first episode button missing")
	}
	if first.W < 52 || first.W > 54 || first.H != 36 {
		t.Fatalf("wide episode buttons have the wrong compact size: %+v", first)
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

func TestDetailWideColumnsScrollIndependently(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = detailWideFixture()
	app.sidebarWidth = 0
	tt := ui.NewTesterWith(app.Render, 1600, 1000, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true, TextScale: 1}})
	tt.Frame()
	tt.Frame()
	if app.detailLeftScroll.MaxY <= 0 || app.detailEpisodesScroll.MaxY <= 0 {
		t.Fatalf("wide detail columns cannot scroll: left=%v episodes=%v", app.detailLeftScroll.MaxY, app.detailEpisodesScroll.MaxY)
	}

	seasons, seasonsOK := tt.Find("同系列短剧")
	episodes, episodesOK := tt.Find("第1集")
	if !seasonsOK || !episodesOK {
		t.Fatalf("column landmarks missing: seasons=%v episodes=%v", seasonsOK, episodesOK)
	}
	leftCenterY := seasons.Y + seasons.H/2
	rightCenterY := episodes.Y + episodes.H/2
	seasonsTitleY := seasons.Y
	episodesTitleY := episodes.Y

	tt.Scroll(seasons.X+seasons.W/2, leftCenterY, 0, app.detailLeftScroll.MaxY)
	if app.detailLeftScroll.MaxY <= 0 {
		t.Fatal("left scroll has no travel")
	}
	if app.detailEpisodesScroll.Y != 0 {
		t.Fatalf("scrolling left changed right scroll state: right=%v", app.detailEpisodesScroll.Y)
	}
	movedSeasons, _ := tt.Find("同系列短剧")
	_, episodesStillVisible := tt.Find("选集")
	if movedSeasons.Y >= seasonsTitleY || !episodesStillVisible {
		t.Fatalf("independent left scroll failed: seasons=%v want<%v episodesVisible=%v", movedSeasons.Y, seasonsTitleY, episodesStillVisible)
	}

	tt.Scroll(episodes.X+episodes.W/2, rightCenterY, 0, app.detailEpisodesScroll.MaxY)
	if app.detailLeftScroll.Y != app.detailLeftScroll.MaxY {
		t.Fatalf("scrolling right changed left scroll state: left=%v want=%v", app.detailLeftScroll.Y, app.detailLeftScroll.MaxY)
	}
	_, seasonsStillVisible := tt.Find("同系列短剧")
	movedEpisodes, _ := tt.Find("第1集")
	if app.detailEpisodesScroll.Y <= 0 || movedEpisodes.Y >= episodesTitleY {
		t.Fatalf("independent right scroll failed: episodes=%v want<%v seasonsVisible=%v", movedEpisodes.Y, episodesTitleY, seasonsStillVisible)
	}
}

func TestDetailWideCollectionsHiddenWithoutData(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = emptyDetailFixture()
	app.detailLeftScroll.Y = 1
	tt := renderTesterAndAdvance(t, app, 1600, 1000)
	if tt.HasText("同系列短剧") || tt.HasText("同一剧本改编") {
		t.Fatal("empty collection sections must be hidden entirely")
	}
}

func TestDetailSkeletonAlignsToReadyHeaderAndCardShapes(t *testing.T) {
	loading := NewTestApp()
	loading.view = "detail"
	loading.detail = Detail{Loading: true}
	loadingTester := ui.NewTester(loading.Render, 1600, 1000)
	loadingTester.Frame()
	loadingTester.Advance(shimmerSampleOffset)
	loadingTester.Frame()
	skeletonBone, skeletonBoneOK := loadingTester.Find("短剧详情")
	if !skeletonBoneOK {
		t.Fatal("first detail skeleton bone missing")
	}
	loadingTester.Advance(shimmerSampleOffset)
	loadingTester.Frame()
	if loadingTester.HasText("正在加载详情") {
		t.Fatal("detail skeleton duplicated the standalone loading spinner")
	}
	if loadingTester.HasText("← 返回") {
		t.Fatal("detail skeleton included the ready back control")
	}

	ready := NewTestApp()
	ready.view = "detail"
	ready.detail = detailWideFixture()
	readyTester := renderTesterAndAdvance(t, ready, 1600, 1000)
	header, headerOK := readyTester.Find("短剧详情")
	if !headerOK {
		t.Fatal("ready detail header missing")
	}
	if absFloat(skeletonBone.Y-header.Y) > 8 {
		t.Fatalf("skeleton top does not match ready content top: bone=%v header=%v", skeletonBone, header)
	}

	if !loadingTester.HasText("同系列短剧") || !loadingTester.HasText("同一剧本改编") {
		t.Fatal("left column skeleton sections missing")
	}
}

func TestDetailNarrowLayoutStaysSingleColumn(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = detailWideFixture()
	app.sidebarWidth = 0
	tt := renderTesterAndAdvance(t, app, 1000, 800)
	if tt.FramePending() {
		tt.Advance(200)
	}
	_, episodesOK := tt.Find("选集")
	if !episodesOK {
		t.Fatal("episodes missing")
	}
	source, sourceOK := tt.Find("原著小说")
	seasons, seasonsOK := tt.Find("同系列短剧")
	adaptations, adaptationsOK := tt.Find("同一剧本改编")
	if !sourceOK || !seasonsOK || !adaptationsOK {
		t.Fatalf("narrow detail collections missing: source=%v seasons=%v adaptations=%v", sourceOK, seasonsOK, adaptationsOK)
	}
	if source.Y+source.H > seasons.Y || seasons.Y+seasons.H > adaptations.Y {
		t.Fatalf("narrow detail collection order changed: source=%+v seasons=%+v adaptations=%+v", source, seasons, adaptations)
	}
}

func absFloat(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
