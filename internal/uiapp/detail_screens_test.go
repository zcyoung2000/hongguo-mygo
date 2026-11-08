package uiapp

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestExportDetailLoadingAndReadyScreens(t *testing.T) {
	dir := os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGUO_NATIVE_SCREENSHOT_DIR to export detail screens")
	}
	states := []struct {
		name   string
		detail Detail
	}{
		{"detail-loading", Detail{Loading: true}},
		{"detail-ready", detailWideFixture()},
	}
	for _, state := range states {
		for _, width := range []int{1200, 1600} {
			name := state.name + "-" + itoa(width)
			t.Run(name, func(t *testing.T) {
				app := NewTestApp()
				app.view = "detail"
				app.detail = state.detail
				tt := ui.NewTester(app.Render, width, 1000)
				tt.SetDark(true)
				tt.Frame()
				if state.detail.Loading {
					tt.Advance(shimmerSampleOffset)
					tt.Frame()
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				file, err := os.Create(filepath.Join(dir, name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					_ = file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
