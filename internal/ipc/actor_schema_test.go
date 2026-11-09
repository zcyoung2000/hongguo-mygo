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
			{Kind: model.InfoTwoActors, Text: "演员·可跳转演员", Schema: "dragon8662://user/710000000000001"},
			{Kind: model.InfoTwoActors, Text: "演员·搜索演员甲 搜索演员乙", Schema: "dragon8662://search?source=23&directQueryWord=%E6%90%9C%E7%B4%A2%E6%BC%94%E5%91%98%E7%94%B2%20%E6%90%9C%E7%B4%A2%E6%BC%94%E5%91%98%E4%B9%99&client_extra=%7B%22actor_ids%22%3A%5B%227100000000000201%22%2C%227100000000000202%22%5D%2C%22last_feed_bookid%22%3A%22123456%22%7D"},
			{Kind: model.InfoTwoActors, Text: "演员·无ID演员"},
		},
	}
	page := service.newSeriesDetailPage(model.SeriesDetail{Series: series, Episodes: []model.Episode{{ID: "episode", SeriesID: "123456", Number: 1}}}, "123456")
	if len(page.Actors) != 4 || page.Actors[0].ActorID != "710000000000001" || page.Actors[1].ActorID != "7100000000000201" || page.Actors[2].ActorID != "7100000000000202" || page.Actors[3].UserID != "" || page.Actors[3].ActorID != "" {
		t.Fatalf("unexpected actors: %+v", page.Actors)
	}
}
