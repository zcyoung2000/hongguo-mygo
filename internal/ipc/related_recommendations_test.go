package ipc

import (
	"context"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/resourcecache"
)

func TestRelatedFilterSelectionsMapsExactMetadata(t *testing.T) {
	service := &HongguoService{cache: resourcecache.New(metadataCacheCapacity)}
	page := &SeriesDetailPageDTO{
		Category:    "恋爱",
		ContentType: int(model.ContentMotionComic),
		Tags:        []string{"都市"},
	}
	panel := []model.FilterRow{
		{Key: "genre", Options: []model.FilterOption{{Name: "漫剧", Value: "comic_series"}}},
		{Key: "category_dim_theme", Options: []model.FilterOption{{Name: "奇幻", Value: "cate_6"}}},
		{Key: "category_dim_epoch", Options: []model.FilterOption{{Name: "都市", Value: "cate_1"}}},
		{Key: "sort", Options: []model.FilterOption{{Name: "最高热度", Value: "hot_score"}}},
	}
	service.cache.Set("series_related_panel:comic_series", resourcecache.Item{
		Value:     panel,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	selections := service.relatedFilterSelections(context.Background(), page)
	if len(selections["category_dim_epoch"]) != 1 || selections["category_dim_epoch"][0] != "cate_1" {
		t.Fatalf("epoch selections = %v, want [cate_1]", selections["category_dim_epoch"])
	}
	if len(selections["sort"]) != 1 || selections["sort"][0] != "hot_score" {
		t.Fatalf("sort selections = %v, want [hot_score]", selections["sort"])
	}
	if _, ok := selections["category_dim_theme"]; ok {
		t.Fatalf("theme selections = %v, want no unrelated theme", selections["category_dim_theme"])
	}
}

func TestRankRelatedCandidatesPrefersRelevanceThenPopularity(t *testing.T) {
	candidates := []relatedCandidate{
		{summary: model.SeriesSummary{ID: "tag-popular"}, relevance: relatedRelevanceTag},
		{summary: model.SeriesSummary{ID: "actor-very-popular"}, relevance: relatedRelevanceActor},
		{summary: model.SeriesSummary{ID: "filter-top"}, relevance: relatedRelevanceFilter},
		{summary: model.SeriesSummary{ID: "filter-second"}, relevance: relatedRelevanceFilter},
	}
	details := map[string]*model.Series{
		"tag-popular": {SeriesSummary: model.SeriesSummary{ID: "tag-popular"}, Heat: 100_000_000},
		"actor-very-popular": {
			SeriesSummary: model.SeriesSummary{ID: "actor-very-popular"},
			Heat:          99_000_000,
		},
		"filter-top": {SeriesSummary: model.SeriesSummary{ID: "filter-top"}, Heat: 5_000_000},
		"filter-second": {
			SeriesSummary: model.SeriesSummary{ID: "filter-second"},
			Heat:          1_000_000,
		},
	}

	ranked := rankRelatedCandidates(candidates, details)
	ids := make([]string, 0, len(ranked))
	for _, candidate := range ranked {
		ids = append(ids, candidate.summary.ID)
	}
	want := []string{"filter-top", "filter-second", "actor-very-popular", "tag-popular"}
	for i, id := range want {
		if ids[i] != id {
			t.Fatalf("ranked IDs = %v, want %v", ids, want)
		}
	}
}

func TestRelatedSeasonTitlesAreDistinguished(t *testing.T) {
	candidates := relatedSourceCandidates([]model.SeriesSummary{
		{ID: "season-1", Title: "天道余途：苍途问道"},
		{ID: "season-2", Title: "天道余途：苍途问道"},
		{ID: "season-3", Title: "天道余途：苍途问道 第二季"},
	}, relatedRelevanceOfficial)
	detailMap := map[string]*model.Series{
		"season-1": {SeriesSummary: model.SeriesSummary{ID: "season-1", Title: "天道余途：苍途问道"}},
		"season-2": {SeriesSummary: model.SeriesSummary{ID: "season-2", Title: "天道余途：苍途问道"}},
		"season-3": {SeriesSummary: model.SeriesSummary{ID: "season-3", Title: "天道余途：苍途问道 第二季"}},
	}
	numbers := map[string]int{"season-1": 1, "season-2": 2, "season-3": 3}

	applyRelatedSeasonTitles(candidates, detailMap, numbers)
	want := []string{
		"天道余途：苍途问道 第1季",
		"天道余途：苍途问道 第2季",
		"天道余途：苍途问道 第二季",
	}
	for i, candidate := range candidates {
		if candidate.summary.Title != want[i] {
			t.Fatalf("titles[%d] = %q, want %q", i, candidate.summary.Title, want[i])
		}
		if detailMap[candidate.summary.ID].Title != want[i] {
			t.Fatalf("detail titles[%d] = %q, want %q", i, detailMap[candidate.summary.ID].Title, want[i])
		}
	}
}

func TestOfficialRelatedCellRequiresSessionAndSpecificTitle(t *testing.T) {
	if isOfficialRelatedCell(nil) {
		t.Fatal("nil recommendation accepted")
	}
	generic := &model.Recommend{CellTitle: "更多好剧", Items: []model.SeriesSummary{{ID: "generic"}}}
	if isOfficialRelatedCell(generic) {
		t.Fatal("anonymous generic recommendation accepted")
	}
	official := &model.Recommend{
		CellTitle: "相关推荐",
		SessionID: "session-1",
		Items:     []model.SeriesSummary{{ID: "official"}},
	}
	if !isOfficialRelatedCell(official) {
		t.Fatal("session-bound related recommendation rejected")
	}
}

func TestRelatedCardUsesEnrichedDetail(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	service := &HongguoService{store: store, media: &MediaServer{}}
	summary := model.SeriesSummary{ID: "related", Title: "Fallback", Category: "短剧"}
	detail := &model.Series{
		SeriesSummary: model.SeriesSummary{
			ID: "related", Title: "Related Drama", Category: "漫剧", CoverURL: "https://example.com/cover.jpg",
			EpisodeCount: 66,
		},
		Status:      model.SeriesEnd,
		Heat:        12_300_000,
		ContentType: model.ContentMotionComic,
	}

	card := service.relatedCard(summary, detail)
	if card.Title != "Related Drama" || card.Category != "漫剧" || card.ContentType != int(model.ContentMotionComic) {
		t.Fatalf("card = %+v, want enriched comic card", card)
	}
	if card.Status != "finished" || card.Heat != "1230万热度" {
		t.Fatalf("card status/heat = %q/%q, want finished/1230万热度", card.Status, card.Heat)
	}
}
