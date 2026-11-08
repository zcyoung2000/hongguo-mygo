package uiapp

import (
	"context"
	"testing"

	"myguo/internal/ipc"
)

type relatedDetailProvider struct{}

func (relatedDetailProvider) GetSeriesPage(context.Context, string) (*ipc.SeriesDetailPageDTO, error) {
	return &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "详情页"}}, nil
}

func (relatedDetailProvider) GetSeriesRelated(context.Context, string) (*ipc.SeriesCollectionDTO, error) {
	return &ipc.SeriesCollectionDTO{Title: "相关推荐", Items: []ipc.SeriesCard{{
		ID: "related-1", Title: "相关剧", Category: "漫剧", BadgeText: "置顶推荐",
		EpisodeCount: 1000, Heat: "9839.2万热度",
	}}}, nil
}

func (relatedDetailProvider) GetFilterPanel(context.Context, string) ([]ipc.FilterRowDTO, error) {
	return nil, nil
}

func (relatedDetailProvider) GetDiscover(context.Context, string, map[string]string, string) (ipc.DiscoverResponse, error) {
	return ipc.DiscoverResponse{}, nil
}

func TestRelatedLoadingRemovesCategoryAndBadge(t *testing.T) {
	app := NewTestApp(nil)
	app.detailSource = relatedDetailProvider{}
	app.loadRelatedAt("123456", app.generation)

	related := app.DetailState().Related
	if len(related) != 1 {
		t.Fatalf("related count = %d, want 1", len(related))
	}
	if related[0].Category != "" || related[0].BadgeText != "" {
		t.Fatalf("related preserved page-only fields: category=%q badge=%q", related[0].Category, related[0].BadgeText)
	}
	if related[0].EpisodeCount != 1000 || related[0].Heat != "9839.2万热度" {
		t.Fatalf("related metadata lost: %+v", related[0])
	}
}
