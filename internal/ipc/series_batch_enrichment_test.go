package ipc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/resourcecache"
	userstore "myguo/internal/store"
)

func TestRecommendFeedEnrichesDetailBatch(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var batches [][]string
	svc := &HongguoService{
		store: store,
		media: &MediaServer{},
		discover: func(_ context.Context, _ sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{
					{ID: "1", Title: "raw one", EpisodeCount: 1},
					{ID: "2", Title: "raw two"},
					{ID: "3", Title: "raw three"},
					{ID: "4", Title: "raw four"},
					{ID: "11", Title: "raw eleven"},
				},
				Cursor:  "next",
				HasMore: true,
			}, nil
		},
		seriesBatch: func(_ context.Context, ids []string) ([]model.SeriesDetail, error) {
			batches = append(batches, append([]string(nil), ids...))
			out := make([]model.SeriesDetail, 0, len(ids))
			for _, id := range ids {
				out = append(out, model.SeriesDetail{Series: &model.Series{
					SeriesSummary: model.SeriesSummary{
						ID: id, Title: "batch " + id, Description: "batch description", EpisodeCount: 20,
						CoverURL: "https://images.example/" + id + ".jpg", CoverExpiresAt: time.Now().Add(time.Hour),
					},
					Heat: 123456,
				}})
			}
			return out, nil
		},
		cache: resourcecache.New(metadataCacheCapacity),
	}

	result, err := svc.GetRecommendFeedBatch(context.Background(), "推荐", "refresh", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0]) != 5 || batches[0][4] != "11" {
		t.Fatalf("detail batches = %v", batches)
	}
	for i := range result.Items {
		want := "batch " + result.Items[i].ID
		if result.Items[i].Title != want || result.Items[i].Description == "" || result.Items[i].EpisodeCount != 20 || result.Items[i].CoverURL == "" {
			t.Fatalf("card %d not enriched: %+v", i, result.Items[i])
		}
	}
	if result.Items[4].Title != "batch 11" {
		t.Fatalf("overflow card changed: %+v", result.Items[4])
	}
}

func TestRecommendFeedDetailBatchFailureKeepsCards(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	svc := &HongguoService{
		store: store,
		media: &MediaServer{},
		discover: func(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			return &sdk.Page[model.SeriesSummary]{
				Items:  []model.SeriesSummary{{ID: "1", Title: "raw one"}},
				Cursor: "next", HasMore: true,
			}, nil
		},
		seriesBatch: func(context.Context, []string) ([]model.SeriesDetail, error) {
			return nil, errors.New("multi detail unavailable")
		},
		cache: resourcecache.New(metadataCacheCapacity),
	}

	result, err := svc.GetRecommendFeedBatch(context.Background(), "推荐", "refresh", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Title != "raw one" {
		t.Fatalf("fallback cards = %+v", result.Items)
	}
}
