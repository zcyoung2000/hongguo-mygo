package ipc

import (
	"context"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestGetSeriesAdaptationsSecondCandidateCoverFallback(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	var requestedIDs []string
	service := &HongguoService{
		store: store,
		media: &MediaServer{},
		seriesBatch: func(_ context.Context, ids []string) ([]model.SeriesDetail, error) {
			requestedIDs = ids
			return []model.SeriesDetail{{
				Series: &model.Series{
					SeriesSummary:      model.SeriesSummary{ID: "adapted-1", Title: "同一剧本"},
					RelatedNovelBookID: "book-1",
				},
			}}, nil
		},
	}
	source := &SourceNovelDTO{ID: "book-1", Title: "原著小说"}

	cards := service.adaptationsForSource(context.Background(), "current", source, []model.SeriesSummary{
		{ID: "source-1", Title: "原著候选", CoverURL: "https://images.example/source.jpg"},
		{ID: "adapted-1", Title: "同一剧本", CoverURL: "https://images.example/search.jpg"},
	}, map[string]bool{"source-1": true})

	if len(cards) != 1 || cards[0].ID != "adapted-1" {
		t.Fatalf("cards = %+v, want the successful adapted candidate only", cards)
	}
	if want := service.media.ImageURL("https://images.example/search.jpg"); cards[0].CoverURL != want {
		t.Fatalf("empty batch cover did not preserve candidate cover: got %q, want %q", cards[0].CoverURL, want)
	}
	if len(requestedIDs) != 1 || requestedIDs[0] != "adapted-1" {
		t.Fatalf("batch IDs = %v, want only adapted-1", requestedIDs)
	}

	batchCover := "https://images.example/batch.jpg"
	service.seriesBatch = func(context.Context, []string) ([]model.SeriesDetail, error) {
		return []model.SeriesDetail{{
			Series: &model.Series{
				SeriesSummary:      model.SeriesSummary{ID: "adapted-1", Title: "同一剧本", CoverURL: batchCover},
				RelatedNovelBookID: "book-1",
			},
		}}, nil
	}
	cards = service.adaptationsForSource(context.Background(), "current", source, []model.SeriesSummary{
		{ID: "adapted-1", Title: "同一剧本", CoverURL: "https://images.example/search.jpg"},
	}, nil)
	if len(cards) != 1 || cards[0].CoverURL != service.media.ImageURL(batchCover) {
		t.Fatalf("present batch cover was not preferred: %+v", cards)
	}
}

func TestGetSeriesAdaptationsUsesFreshCandidateWhenBatchCoverExpired(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()

	expired := time.Now().Add(-time.Hour)
	service := &HongguoService{
		store: store,
		media: &MediaServer{},
		seriesBatch: func(context.Context, []string) ([]model.SeriesDetail, error) {
			return []model.SeriesDetail{{
				Series: &model.Series{
					SeriesSummary: model.SeriesSummary{
						ID:             "adapted-1",
						Title:          "同一剧本",
						CoverURL:       "https://images.example/expired-batch.jpg",
						CoverExpiresAt: expired,
					},
					RelatedNovelBookID: "book-1",
				},
			}}, nil
		},
	}
	source := &SourceNovelDTO{ID: "book-1", Title: "原著小说"}

	cards := service.adaptationsForSource(context.Background(), "current", source, []model.SeriesSummary{
		{
			ID:             "adapted-1",
			Title:          "同一剧本",
			CoverURL:       "https://images.example/fresh-search.jpg",
			CoverExpiresAt: time.Now().Add(time.Hour),
		},
	}, nil)
	if len(cards) != 1 || cards[0].CoverURL != service.media.ImageURL("https://images.example/fresh-search.jpg") {
		t.Fatalf("expired batch cover did not use fresh candidate cover: %+v", cards)
	}

	service.seriesBatch = func(context.Context, []string) ([]model.SeriesDetail, error) {
		return []model.SeriesDetail{{
			Series: &model.Series{
				SeriesSummary: model.SeriesSummary{
					ID:             "adapted-1",
					Title:          "同一剧本",
					CoverURL:       "https://images.example/expired-batch.jpg",
					CoverExpiresAt: expired,
				},
				RelatedNovelBookID: "book-1",
			},
		}}, nil
	}
	cards = service.adaptationsForSource(context.Background(), "current", source, []model.SeriesSummary{
		{
			ID:             "adapted-1",
			Title:          "同一剧本",
			CoverURL:       "https://images.example/expired-search.jpg",
			CoverExpiresAt: expired,
		},
	}, nil)
	if len(cards) != 1 || cards[0].CoverURL != "" {
		t.Fatalf("all expired covers should leave the card cover empty without placeholder art: %+v", cards)
	}
}
