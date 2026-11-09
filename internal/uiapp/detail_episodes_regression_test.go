package uiapp

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func newDetailEpisodeTester(episodeCount int) (*App, *ui.Tester) {
	app := NewTestApp()
	app.view = "detail"
	app.seriesID = "123456"
	episodes := make([]CatalogItem, episodeCount)
	for index := range episodes {
		episodes[index] = CatalogItem{ID: "123456", EpisodeID: "episode", EpisodeCount: index + 1}
	}
	app.detail = Detail{ID: "123456", Title: "长剧选集", Episodes: episodes}
	return app, ui.NewTester(app.Render, 1200, 800)
}

func TestDetailEpisodes812RenderPerformance(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.seriesID = "123456"
	app.detail = longDetailEpisodesFixture(812)
	tt := ui.NewTester(app.Render, 1600, 1000)
	start := time.Now()
	tt.Frame()
	frameDuration := time.Since(start)
	if frameDuration > 500*time.Millisecond {
		t.Fatalf("812-episode first frame took too long: %s", frameDuration)
	}
	app.detailEpisodesList.ScrollToEnd()
	tt.Frame()
	if !tt.HasText("第812集") {
		t.Fatal("812-episode list cannot render the final row")
	}
	if os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR") != "" {
		fmt.Printf("812-episode first frame: %s\n", frameDuration)
	}
}

func longDetailEpisodesFixture(episodeCount int) Detail {
	detail := detailWideFixture()
	episode := func(number int) CatalogItem {
		return CatalogItem{ID: "123456", EpisodeID: "episode", EpisodeCount: number}
	}
	detail.Episodes = make([]CatalogItem, 0, episodeCount)
	for number := 1; number <= episodeCount; number++ {
		detail.Episodes = append(detail.Episodes, episode(number))
	}
	return detail
}

func TestDetailEpisodesRenderCompleteListWithoutExpansion(t *testing.T) {
	app, tt := newDetailEpisodeTester(220)
	tt.Frame()

	if _, found := tt.Find("展开全部"); found {
		t.Fatalf("detail episodes still collapse: %v", tt.Texts())
	}
	first, firstOK := tt.Find("第1集")
	if !firstOK {
		t.Fatalf("detail episodes do not render every episode: %v", tt.Texts())
	}
	if first.W != 52 || first.H != 36 {
		t.Fatalf("episode cell size changed: %+v", first)
	}
	app.detailEpisodesList.ScrollToEnd()
	tt.Frame()
	if !tt.HasText("第220集") {
		t.Fatalf("detail episodes omit the final episode after scrolling: %v", tt.Texts())
	}
}

func TestDetailEpisodesStayCompleteAfterEpisodeSelection(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.seriesID = "123456"
	app.SetMediaHost(&MediaHost{})
	app.detail = Detail{ID: "123456", Title: "长剧选集", Episodes: []CatalogItem{
		{ID: "123456", EpisodeCount: 1}, {ID: "123456", EpisodeCount: 2},
	}}
	app.ChooseEpisode(2)
	app.mediaOn = false
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	for _, text := range []string{"第1集", "第2集", "共 2 集"} {
		if !tt.HasText(text) {
			t.Fatalf("episode %q disappeared after selection: %v", text, tt.Texts())
		}
	}
}
