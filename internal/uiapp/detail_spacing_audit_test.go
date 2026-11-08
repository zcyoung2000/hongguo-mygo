package uiapp

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

var SourceNovelForSpacingAudit = ipc.SourceNovelDTO{ID: "source-novel", Title: "重生：我远离后，她怎么慌了？"}

func TestDetailCollectionVerticalSpacingAudit(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.seriesID = "123456"
	app.detail = Detail{
		ID: "123456", Title: "系列间距审计短剧", Tags: []string{"都市", "逆袭"},
		Heat: "312万热度", StatusText: "全524集", PlayCount: 4846944,
		Source: &SourceNovelForSpacingAudit,
		Seasons: DetailCollection{
			Title: "十八岁的二次回响",
			Items: []CatalogItem{
				{ID: "season-2", Title: "十八岁的二次回响第二季", EpisodeCount: 119, Completed: true},
				{ID: "season-3", Title: "十八岁的二次回响第三季", Upcoming: true, Reserved: true},
			},
		},
		Adaptations: DetailCollection{
			Title: "同一剧本改编",
			Items: []CatalogItem{{ID: "adaptation-1", Title: "十八岁的二次回响漫画版", EpisodeCount: 80, Heat: "300万热度"}},
		},
		Episodes: []CatalogItem{{ID: "episode-1", EpisodeID: "episode-1", EpisodeCount: 1}},
	}
	tt := ui.NewTester(app.Render, 1200, 1080)
	tt.SetDark(true)
	if tt.FramePending() {
		tt.Advance(200)
	}

	seasonTitle, seasonTitleOK := tt.Find("十八岁的二次回响")
	seasonPoster, seasonPosterOK := tt.Find("poster:season-2|")
	adaptationMeta, adaptationMetaOK := tt.Find("80集 · 300万热度")
	adaptationTitle, adaptationTitleOK := tt.Find("同一剧本改编")
	adaptationPoster, adaptationPosterOK := tt.Find("poster:adaptation-1|")
	if !seasonTitleOK || !seasonPosterOK || !adaptationMetaOK || !adaptationTitleOK || !adaptationPosterOK {
		t.Fatalf("detail audit elements missing: title=%v poster=%v meta=%v adaptationTitle=%v adaptationPoster=%v",
			seasonTitleOK, seasonPosterOK, adaptationMetaOK, adaptationTitleOK, adaptationPosterOK)
	}

	titleGap := seasonPoster.Y - (seasonTitle.Y + seasonTitle.H)
	seasonCard, _ := tt.Find("十八岁的二次回响第二季")
	metaGap := adaptationTitle.Y - (seasonCard.Y + seasonCard.H)
	headingGap := adaptationPoster.Y - (adaptationTitle.Y + adaptationTitle.H)
	if titleGap < 6 || titleGap > 12 {
		t.Fatalf("season title to poster gap = %.1f, want 6-12: title=%v poster=%v", titleGap, seasonTitle, seasonPoster)
	}
	if metaGap < 6 || metaGap > 12 {
		t.Fatalf("season card bottom to adaptation gap = %.1f, want 6-12: card=%v adaptationTitle=%v", metaGap, seasonCard, adaptationTitle)
	}
	if headingGap < 6 || headingGap > 12 {
		t.Fatalf("adaptation title to poster gap = %.1f, want 6-12: title=%v poster=%v", headingGap, adaptationTitle, adaptationPoster)
	}
	if min(adaptationMeta.H, adaptationPoster.H, seasonPoster.H) <= 0 {
		t.Fatalf("audit elements have empty bounds: meta=%v poster=%v season=%v", adaptationMeta, adaptationPoster, seasonPoster)
	}
	if math.Abs(float64(seasonPoster.H/seasonPoster.W-4.0/3.0)) > 0.04 || math.Abs(float64(adaptationPoster.H/adaptationPoster.W-4.0/3.0)) > 0.04 {
		t.Fatalf("collection poster ratio wrong: season=%.3f adaptation=%.3f", seasonPoster.H/seasonPoster.W, adaptationPoster.H/adaptationPoster.W)
	}
	if seasonTitle.Y < 0 || seasonCard.Y+seasonCard.H > 1080 || adaptationTitle.Y+adaptationTitle.H > 1080 || adaptationPoster.Y+adaptationPoster.H > 1080 || adaptationMeta.Y+adaptationMeta.H > 1080 {
		t.Fatalf("collection content clipped: seasonCard=%v title=%v poster=%v meta=%v", seasonCard, adaptationTitle, adaptationPoster, adaptationMeta)
	}

	dir := os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGUO_NATIVE_SCREENSHOT_DIR to save detail spacing screenshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	frame := tt.Image()
	seasonAction, seasonActionOK := tt.Find("十八岁的二次回响第三季")
	if !seasonActionOK {
		t.Fatal("upcoming season card missing")
	}
	seasonRight := seasonAction.X + seasonAction.W + 8
	regionTop := int(min(seasonTitle.Y, adaptationTitle.Y))
	regionBottom := int(math.Ceil(math.Max(float64(adaptationMeta.Y+adaptationMeta.H), float64(seasonAction.Y+seasonAction.H))))
	regionLeft := int(min(seasonPoster.X, adaptationPoster.X))
	regionRight := int(math.Ceil(max(float64(seasonRight), float64(adaptationPoster.X+adaptationPoster.W))))
	region := image.NewRGBA(image.Rect(0, 0, regionRight-regionLeft, regionBottom-regionTop))
	for y := regionTop; y < regionBottom; y++ {
		for x := regionLeft; x < regionRight; x++ {
			region.Set(x-regionLeft, y-regionTop, frame.RGBAAt(x, y))
		}
	}
	for _, target := range []struct {
		name  string
		image *image.RGBA
	}{{"detail-spacing-full.png", frame}, {"detail-spacing-region.png", region}} {
		path := filepath.Join(dir, target.name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, target.image); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("saved screenshot: %s", path)
	}
}
