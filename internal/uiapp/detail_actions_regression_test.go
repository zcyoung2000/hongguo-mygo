package uiapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDetailPrimaryActionLabelStaysOnOneLine(t *testing.T) {
	tests := []struct {
		name   string
		detail Detail
		resume int
		label  string
	}{
		{
			name:   "all actions",
			detail: Detail{ID: "123456", Title: "三按钮详情", Favorite: true, Reserved: true},
			label:  "播放 / 继续观看",
		},
		{
			name:   "two actions",
			detail: Detail{ID: "123456", Title: "两按钮详情"},
			label:  "播放 / 继续观看",
		},
		{
			name:   "long resumed action",
			detail: Detail{ID: "123456", Title: "续看详情"},
			resume: 12,
			label:  "继续观看第 12 集",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewTestApp()
			app.view = "detail"
			app.seriesID = "123456"
			app.detail = test.detail
			if test.resume > 0 {
				app.resume = CatalogItem{ID: "123456", ResumeEpisode: test.resume}
			}
			tt := ui.NewTester(app.Render, 1200, 800)
			tt.Frame()
			button, ok := tt.Find(test.label)
			if !ok {
				t.Fatalf("primary action missing: %v", tt.Texts())
			}
			if button.H > 36 {
				t.Fatalf("primary action wrapped to multiple lines: %+v", button)
			}
		})
	}
}
