package uiapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func newDetailEpisodeTester(episodeCount int) (*App, *ui.Tester) {
	app := NewTestApp(nil)
	app.view = "detail"
	app.seriesID = "123456"
	episodes := make([]CatalogItem, episodeCount)
	for index := range episodes {
		episodes[index] = CatalogItem{ID: "123456", EpisodeID: "episode", EpisodeCount: index + 1}
	}
	app.detail = Detail{ID: "123456", Title: "长剧选集", Episodes: episodes}
	return app, ui.NewTester(app.Render, 1200, 800)
}

func TestDetailEpisodesDefaultCollapseExpandAndCollapseAgain(t *testing.T) {
	app, tt := newDetailEpisodeTester(220)
	tt.Frame()

	_, found := tt.Find("展开全部")
	if !found {
		t.Fatalf("collapsed detail episodes missing expand control: %v", tt.Texts())
	}
	if tt.HasText("第48集") {
		t.Fatalf("collapsed detail episodes rendered beyond three rows: %v", tt.Texts())
	}

	if err := tt.Click("展开全部"); err != nil {
		t.Fatalf("click expand control: %v", err)
	}
	tt.Frame()
	app.mu.Lock()
	expanded := app.detailEpisodesExpanded
	app.mu.Unlock()
	if !expanded || !tt.HasText("第220集") {
		t.Fatalf("expand control did not reveal complete episode list: expanded=%v texts=%d", expanded, len(tt.Texts()))
	}
}

func TestDetailEpisodesExpandStateResetsAfterPlayback(t *testing.T) {
	app := NewTestApp(NewTestPlayerBridge())
	app.detail = Detail{ID: "123456", Title: "长剧选集"}
	app.detailEpisodesExpanded = true
	app.ChooseEpisode(2)
	reset := !app.detailEpisodesExpanded
	if !reset {
		t.Fatal("episode selection must reset detail expansion state")
	}
}
