package main

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestNextPlayableSeason(t *testing.T) {
	collection := &SeriesCollectionDTO{Items: []SeriesCard{
		{ID: "s1", EpisodeCount: 10},
		{ID: "s2", Upcoming: true},
		{ID: "s3", EpisodeCount: 0},
		{ID: "s4", EpisodeCount: 12, Title: "第4季"},
	}}

	next := nextPlayableSeason("s1", collection)
	if next == nil || next.ID != "s4" {
		t.Fatalf("next = %+v, want s4", next)
	}
	if next := nextPlayableSeason("s4", collection); next != nil {
		t.Fatalf("next after last playable season = %+v, want nil", next)
	}
	if next := nextPlayableSeason("missing", collection); next != nil {
		t.Fatalf("next for unknown series = %+v, want nil", next)
	}
}

func TestParseAndCacheIDList(t *testing.T) {
	ids := parseIDList(" 100 , ,200,100 ")
	want := map[string]bool{"100": true, "200": true}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("parseIDList() = %v, want %v", ids, want)
	}
	if got := cacheIDList(ids); got != "100,200" {
		t.Fatalf("cacheIDList() = %q, want %q", got, "100,200")
	}
}

func TestSeasonsToCardsMarksUnreleasedSeasons(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	service := &HongguoService{store: store, media: &MediaServer{}}
	album := &model.Album{
		ID:    "album-1",
		Title: "Test Series",
		Seasons: []model.Season{
			{SeriesID: "100", Index: 1, EpisodeCount: 24, Status: model.SeriesUpdating},
			{SeriesID: "101", Index: 3, EpisodeCount: 0, Status: model.SeriesUpdating},
		},
	}

	cards := service.seasonsToCards(album)
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
	if cards[0].Upcoming {
		t.Errorf("released season marked upcoming: %+v", cards[0])
	}
	if cards[1].Heat != "待上线" || !cards[1].Upcoming || cards[1].Reserved {
		t.Errorf("unreleased season = %+v, want 待上线/upcoming/unreserved", cards[1])
	}

	if _, err := store.ToggleReservation(ReservationItem{SeriesID: "101", Title: "Test Series 3"}); err != nil {
		t.Fatalf("ToggleReservation err = %v", err)
	}
	cards = service.seasonsToCards(album)
	if !cards[1].Reserved {
		t.Errorf("unreleased season reserved = false, want true")
	}
}

func TestSourceNovelCardUsesSourceLabel(t *testing.T) {
	card := sourceNovelCard(&SourceNovelDTO{
		ID:       "book-1",
		Title:    "原著小说",
		Status:   "finished",
		Reads:    12300,
		CoverURL: "https://example.com/cover.jpg",
		Tags:     []string{"都市", "言情"},
	})
	if card == nil {
		t.Fatal("sourceNovelCard() = nil")
	}
	if card.Category != "原著剧本" || card.Kind != "source_novel" {
		t.Fatalf("card label = %q, kind = %q, want 原著剧本/source_novel", card.Category, card.Kind)
	}
	if card.BadgeText != "已完结" {
		t.Fatalf("card.BadgeText = %q, want 已完结", card.BadgeText)
	}
	if card.EpisodeCount != 0 {
		t.Fatalf("card.EpisodeCount = %d, want 0", card.EpisodeCount)
	}

	if sourceNovelCard(nil) != nil {
		t.Fatal("sourceNovelCard(nil) != nil")
	}
}

func TestSameSourceNovelRequiresBookID(t *testing.T) {
	source := &SourceNovelDTO{ID: "book-1", Title: "原著小说"}
	if isSameSourceNovel(nil, source) {
		t.Fatal("nil series matched source")
	}
	if isSameSourceNovel(&model.Series{SeriesSummary: model.SeriesSummary{ID: "video-2", Title: "原著小说"}}, source) {
		t.Fatal("title-only series matched source")
	}
	if !isSameSourceNovel(&model.Series{
		SeriesSummary:      model.SeriesSummary{ID: "video-2"},
		RelatedNovelBookID: "book-1",
	}, source) {
		t.Fatal("related book ID did not match source")
	}
}

func TestSeriesContentTypeLabel(t *testing.T) {
	if got := seriesContentTypeLabel(int(model.ContentMotionComic)); got != "漫剧" {
		t.Fatalf("seriesContentTypeLabel(motion comic) = %q, want 漫剧", got)
	}
	if got := seriesContentTypeLabel(int(model.ContentShortSeries)); got != "短剧" {
		t.Fatalf("seriesContentTypeLabel(short series) = %q, want 短剧", got)
	}
}

func TestNewSeriesDetailPageSanitizesUpstreamUTF8(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	service := &HongguoService{store: store, media: &MediaServer{}}
	detail := model.SeriesDetail{
		Series: &model.Series{
			SeriesSummary: model.SeriesSummary{ID: "100", Title: "Test Drama"},
			Rows: []model.InfoRow{
				{Kind: model.InfoTwoActors, Text: "张三\x80 · 李四\xff"},
			},
		},
		Episodes: []model.Episode{{ID: "episode-1", SeriesID: "100", Number: 1}},
	}
	detail.Series.Category = "悬疑"

	page := service.newSeriesDetailPage(detail, "100")
	if !utf8.ValidString(page.Actors[0].Name) || strings.ContainsRune(page.Actors[0].Name, utf8.RuneError) {
		t.Fatalf("actor name = %q, want valid UTF-8", page.Actors[0].Name)
	}
	for _, value := range append([]string{page.Title, page.Category}, page.Tags...) {
		if !utf8.ValidString(value) {
			t.Fatalf("page text = %q, want valid UTF-8", value)
		}
	}
}

func TestSeasonsToCardsFinishedAndUpcomingStatus(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	service := &HongguoService{store: store, media: &MediaServer{}}
	album := &model.Album{
		ID:    "album-cow",
		Title: "你让我当牛马我在荒岛成王",
		Seasons: []model.Season{
			{SeriesID: "s1", Index: 1, EpisodeCount: 215, Status: model.SeriesUpdating},
			{SeriesID: "s2", Index: 2, EpisodeCount: 154, Status: model.SeriesUpdating},
			{SeriesID: "s3", Index: 3, EpisodeCount: 152, Status: model.SeriesUpdating},
			{SeriesID: "s4", Index: 4, EpisodeCount: 123, Status: model.SeriesUpdating},
			{SeriesID: "s5", Index: 5, EpisodeCount: 0, Status: model.SeriesUpdating},
		},
	}

	detailMap := map[string]*model.Series{
		"s1": {
			SeriesSummary: model.SeriesSummary{ID: "s1", Title: "你让我当牛马我在荒岛成王第一季", EpisodeCount: 215},
			Status:        model.SeriesEnd,
			RightText:     "全215集",
		},
		"s2": {
			SeriesSummary: model.SeriesSummary{ID: "s2", Title: "你让我当牛马我在荒岛成王第二季", EpisodeCount: 143},
			Status:        model.SeriesEnd,
			RightText:     "全143集",
		},
		"s3": {
			SeriesSummary: model.SeriesSummary{ID: "s3", Title: "你让我当牛马我在荒岛成王第三季", EpisodeCount: 152},
			Status:        model.SeriesEnd,
			RightText:     "全152集",
		},
		"s4": {
			SeriesSummary: model.SeriesSummary{ID: "s4", Title: "你让我当牛马我在荒岛成王第四季", EpisodeCount: 123},
			Status:        model.SeriesEnd,
			RightText:     "全123集",
		},
		"s5": {
			SeriesSummary: model.SeriesSummary{ID: "s5", Title: "你让我当牛马我在荒岛成王第五季", EpisodeCount: 0},
			Status:        model.SeriesUpdating,
			RightText:     "待上线",
		},
	}

	cards := service.seasonsToCards(album, detailMap)
	if len(cards) != 5 {
		t.Fatalf("len(cards) = %d, want 5", len(cards))
	}

	// s1 to s4 must be finished ("已完结"), NOT "更新中"
	for i := 0; i < 4; i++ {
		if cards[i].Status != "finished" {
			t.Errorf("cards[%d].Status = %q, want 'finished'", i, cards[i].Status)
		}
		if cards[i].Heat != "已完结" {
			t.Errorf("cards[%d].Heat = %q, want '已完结'", i, cards[i].Heat)
		}
		if cards[i].Upcoming {
			t.Errorf("cards[%d].Upcoming = true, want false", i)
		}
	}

	// s5 is upcoming (待上线) with reservation support
	if !cards[4].Upcoming {
		t.Errorf("cards[4].Upcoming = false, want true")
	}
	if cards[4].Heat != "待上线" {
		t.Errorf("cards[4].Heat = %q, want '待上线'", cards[4].Heat)
	}
	if cards[4].Status != "upcoming" {
		t.Errorf("cards[4].Status = %q, want 'upcoming'", cards[4].Status)
	}
}

func TestNewSeriesDetailPageUpcomingAndFinishedStatus(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	service := &HongguoService{store: store, media: &MediaServer{}}

	// 1. Finished series
	finishedDetail := model.SeriesDetail{
		Series: &model.Series{
			SeriesSummary: model.SeriesSummary{ID: "fin-1", Title: "完结短剧", EpisodeCount: 80},
			Status:        model.SeriesEnd,
			RightText:     "全80集",
		},
	}
	finPage := service.newSeriesDetailPage(finishedDetail, "fin-1")
	if finPage.Upcoming {
		t.Errorf("finPage.Upcoming = true, want false")
	}
	if finPage.StatusText != "全80集" {
		t.Errorf("finPage.StatusText = %q, want '全80集'", finPage.StatusText)
	}
	if finPage.Status != "finished" {
		t.Errorf("finPage.Status = %q, want 'finished'", finPage.Status)
	}

	// 2. Upcoming series (EpisodeCount == 0)
	upcomingDetail := model.SeriesDetail{
		Series: &model.Series{
			SeriesSummary: model.SeriesSummary{ID: "up-1", Title: "待播短剧", EpisodeCount: 0},
			Status:        model.SeriesUpdating,
			RightText:     "全0集",
		},
	}
	upPage := service.newSeriesDetailPage(upcomingDetail, "up-1")
	if !upPage.Upcoming {
		t.Errorf("upPage.Upcoming = false, want true")
	}
	if upPage.StatusText != "待上线" {
		t.Errorf("upPage.StatusText = %q, want '待上线'", upPage.StatusText)
	}
	if upPage.Status != "upcoming" {
		t.Errorf("upPage.Status = %q, want 'upcoming'", upPage.Status)
	}
}
