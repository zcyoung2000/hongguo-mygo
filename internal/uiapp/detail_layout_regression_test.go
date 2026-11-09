package uiapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestDetailVerticalRhythmAndCollectionSpacing(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.seriesID = "123456"
	app.detail = Detail{
		ID:          "123456",
		Title:       "紧凑详情页",
		Source:      &ipc.SourceNovelDTO{ID: "source", Title: "原著小说"},
		Actors:      []ipc.ActorInfo{{Name: "演员一"}},
		Seasons:     DetailCollection{Title: "同系列短剧", Items: []CatalogItem{{ID: "season-1", Title: "同系列短剧", Category: "短剧"}}},
		Adaptations: DetailCollection{Title: "同一剧本改编", Items: []CatalogItem{{ID: "adapt-1", Title: "同一剧本改编剧", Category: "漫剧"}}},
		Episodes:    []CatalogItem{{ID: "123456", Title: "紧凑详情页", EpisodeID: "1"}},
	}
	app.sidebarWidth = 150
	tt := ui.NewTester(app.Render, 1000, 800)
	if tt.FramePending() {
		tt.Advance(200)
	}

	title, ok := tt.Find("紧凑详情页")
	source, sourceOK := tt.Find("原著小说")
	seasons, seasonsOK := tt.Find("同系列短剧")
	collectionLabel, collectionOK := tt.Find("同一剧本改编")
	if !ok || !sourceOK || !seasonsOK || !collectionOK {
		t.Fatalf("detail sections missing: title=%+v source=%+v seasons=%+v collection=%+v", title, source, seasons, collectionLabel)
	}
	if title.Y+title.H > source.Y || source.Y+source.H > collectionLabel.Y {
		t.Fatalf("detail sections overlap or clip: title=%+v source=%+v collection=%+v", title, source, collectionLabel)
	}
	gap := float64(collectionLabel.Y - (seasons.Y + seasons.H))
	if gap < 6 || collectionLabel.Y+collectionLabel.H > 820 {
		t.Fatalf("detail collection spacing or viewport bounds wrong: gap=%.1f label=%+v seasons=%+v", gap, collectionLabel, seasons)
	}
}
