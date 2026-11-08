package ipc

import (
	"testing"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestNewSeriesDetailPagePreservesActorSchemaUserID(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := &HongguoService{store: store, media: &MediaServer{}}
	series := &model.Series{
		SeriesSummary: model.SeriesSummary{ID: "123456", Title: "演员详情", EpisodeCount: 1},
		Rows: []model.InfoRow{
			{Kind: model.InfoTwoActors, Text: "演员·可跳转演员 无ID演员", Schema: "dragon8662://user/710000000000001"},
		},
	}
	page := service.newSeriesDetailPage(model.SeriesDetail{Series: series, Episodes: []model.Episode{{ID: "episode", SeriesID: "123456", Number: 1}}}, "123456")
	if len(page.Actors) != 2 || page.Actors[0].UserID != "710000000000001" || page.Actors[1].UserID != "" {
		t.Fatalf("unexpected actors: %+v", page.Actors)
	}
}
