package uiapp

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
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

func writeRelatedScreenshot(t *testing.T, tt *ui.Tester, name string) string {
	t.Helper()
	path := filepath.Join(os.TempDir(), name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create screenshot: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, tt.Image()); err != nil {
		t.Fatalf("encode screenshot: %v", err)
	}
	return path
}

func TestRenderDetailRelatedOmitsCategoryAndShowsCompleteMetadata(t *testing.T) {
	app := NewTestApp(nil)
	app.view = "detail"
	app.seriesID = "123456"
	app.detail = Detail{
		ID:           "123456",
		Title:        "详情页",
		RelatedTitle: "相关推荐",
		Related: clearRelatedCollectionCardFields([]CatalogItem{{
			ID:           "related-1",
			Title:        "相关推荐标题文本",
			EpisodeCount: 1000,
			Heat:         "9839.2万热度",
			Category:     "漫剧",
			BadgeText:    "置顶推荐",
		}}),
	}
	tt := ui.NewTester(app.Render, 1200, 900)
	if tt.FramePending() {
		tt.Advance(200)
	}

	if badge, badgeOK := tt.Find("置顶推荐"); badgeOK || badge.W > 0 {
		t.Fatalf("related card rendered badge: %+v", badge)
	}
	if !slices.Contains(tt.Texts(), "相关推荐标题文本") || !slices.Contains(tt.Texts(), "1000集 · 9839.2万热度") {
		t.Fatalf("related title or metadata missing: %v", tt.Texts())
	}
	poster, posterOK := tt.Find("poster:related-1|")
	meta, metaOK := tt.Find("1000集 · 9839.2万热度")
	if !posterOK || !metaOK {
		t.Fatalf("related poster or metadata missing: poster=%+v meta=%+v texts=%v", poster, meta, tt.Texts())
	}
	if ratio := poster.H / poster.W; ratio < 1.30 || ratio > 1.36 {
		t.Fatalf("related cover ratio = %.3f, poster=%+v", ratio, poster)
	}
	if meta.Y+meta.H > poster.Y+poster.H+18+16+16 {
		t.Fatalf("related metadata beyond poster plus two rows: poster=%+v meta=%+v", poster, meta)
	}
	if trailing := poster.Y + poster.H + 18 + 16 + 16 - (meta.Y + meta.H); trailing < 0 || trailing > 8 {
		t.Fatalf("related card leaves invalid trailing space: %.1f poster=%+v meta=%+v", trailing, poster, meta)
	}
	if meta.W != poster.W || meta.W != 150 || meta.Y+meta.H > 900 {
		t.Fatalf("related metadata clipped: poster=%+v meta=%+v", poster, meta)
	}
	screenshot := writeRelatedScreenshot(t, tt, "mygo-detail-related-fixed.png")
	t.Logf("related screenshot: %s", screenshot)
}

func TestRenderDetailRelatedPreservesCoverRatio(t *testing.T) {
	app := NewTestApp(nil)
	app.view = "detail"
	app.seriesID = "123456"
	app.detail = Detail{
		ID: "123456",
		Related: []CatalogItem{{
			ID:           "related-1",
			Title:        "相关剧标题",
			EpisodeCount: 220,
			Heat:         "9839.2万热度",
		},
		},
	}
	tt := ui.NewTester(app.Render, 1200, 900)
	if tt.FramePending() {
		tt.Advance(200)
	}

	if !slices.Contains(tt.Texts(), "相关剧标题") || !slices.Contains(tt.Texts(), "220集 · 9839.2万热度") {
		t.Fatalf("related title or metadata missing: %v", tt.Texts())
	}
	meta, metaOK := tt.Find("220集 · 9839.2万热度")
	if !metaOK {
		t.Fatalf("related metadata missing")
	}
	if meta.W != 150 || meta.Y+meta.H > 900 {
		t.Fatalf("related metadata clipped: meta=%+v", meta)
	}
	poster, posterOK := tt.Find("poster:related-1|")
	if !posterOK {
		t.Fatalf("related poster missing")
	}
	if ratio := poster.H / poster.W; ratio < 1.30 || ratio > 1.36 {
		t.Fatalf("related cover ratio = %.3f, poster=%+v", ratio, poster)
	}
	if poster.Y+poster.H > meta.Y-16 {
		t.Fatalf("related poster overlaps title: poster=%+v meta=%+v", poster, meta)
	}
}
