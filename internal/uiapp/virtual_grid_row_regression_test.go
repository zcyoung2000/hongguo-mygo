package uiapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestReservationRowsHaveSpaceForAllMetadata(t *testing.T) {
	items := []CatalogItem{
		{ID: "one", EpisodeID: "e1", Title: "我能看到别人的情绪第三季", Upcoming: false, PublishAt: "2026-11-07"},
		{ID: "two", EpisodeID: "e2", Title: "我就是丧尸之王第二季", Upcoming: false, PublishAt: "2026-10-25"},
		{ID: "three", EpisodeID: "e3", Title: "十八岁的二次回响第二季", Upcoming: false, PublishAt: "2026-10-25"},
		{ID: "four", EpisodeID: "e4", Title: "下一行卡片标题", Upcoming: false, PublishAt: "2026-12-01"},
	}
	app := NewTestApp()
	app.view = "reservations"
	app.library.Reserved = Library{Loaded: true, Items: items}
	app.library.Grids[3] = &ui.GridState{}
	app.reservationsPending = false
	tester := ui.NewTester(app.Render, 560, 900)
	tester.Frame()
	first, firstOK := tester.Find("我能看到别人的情绪第三季")
	second, secondOK := tester.Find("我就是丧尸之王第二季")
	if !firstOK || !secondOK {
		t.Fatalf("reservation cards missing: %v", tester.Texts())
	}
	rowHeight := float64(second.Y - first.Y)
	if rowHeight <= float64(first.H)+12 {
		t.Fatalf("reservation grid rows overlap: rowHeight=%.1f cardHeight=%.1f first=%+v second=%+v", rowHeight, first.H, first, second)
	}
	cancel, cancelOK := tester.Find("取消预约")
	if !cancelOK {
		t.Fatal("reservation action missing")
	}
	if cancel.Y+cancel.H > second.Y {
		t.Fatalf("action overlaps next row: cancel=%+v nextRow=%.1f", cancel, second.Y)
	}
}
