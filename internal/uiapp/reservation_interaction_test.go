package uiapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestReservationCardsFollowUpcomingActivationRules(t *testing.T) {
	pending := ipc.ReservationItem{SeriesID: "pending-1", Title: "待上线预约剧", IsOnline: false, PublishAt: "2099-01-02"}
	released := ipc.ReservationItem{SeriesID: "released-1", Title: "已上线预约剧", IsOnline: true, PublishAt: "2020-01-02"}

	t.Run("pending click", func(t *testing.T) {
		app, tt := newReservationTester(pending, false)
		if err := tt.Click("待上线预约剧"); err != nil {
			t.Fatal(err)
		}
		waitFor(time.Second, func() bool { return app.View() == "reservations" })
		if app.View() != "reservations" || app.SeriesID() != "" {
			t.Fatalf("upcoming reservation opened detail: view=%q series=%q", app.View(), app.SeriesID())
		}
	})

	t.Run("pending keyboard", func(t *testing.T) {
		app, tt := newReservationTester(pending, false)
		for !tt.Focused("待上线预约剧") {
			tt.Key(0, ui.KeyTab)
		}
		tt.Key(0, ui.KeyEnter)
		waitFor(time.Second, func() bool { return app.View() == "reservations" })
		if app.View() != "reservations" || app.SeriesID() != "" {
			t.Fatalf("pending reservation opened detail on Enter: view=%q series=%q", app.View(), app.SeriesID())
		}
		tt.Key(0, ui.KeySpace)
		waitFor(time.Second, func() bool { return app.View() == "reservations" })
		if app.View() != "reservations" || app.SeriesID() != "" {
			t.Fatalf("pending reservation opened detail on Space: view=%q series=%q", app.View(), app.SeriesID())
		}
	})

	t.Run("released click", func(t *testing.T) {
		app, tt := newReservationTester(released, true)
		if err := tt.Click("已上线预约剧"); err != nil {
			t.Fatal(err)
		}
		waitFor(time.Second, func() bool { return app.View() == "detail" && app.SeriesID() == "released-1" })
		if app.View() != "detail" || app.SeriesID() != "released-1" {
			t.Fatalf("released reservation did not open detail: view=%q series=%q", app.View(), app.SeriesID())
		}
	})
}

func newReservationTester(item ipc.ReservationItem, online bool) (*App, *ui.Tester) {
	app := NewTestApp(nil)
	app.SetInvalidator(func() {})
	item.IsOnline = online
	app.libraryData = &fakeLibrary{reservations: []ipc.ReservationItem{item}}
	app.loadLibrary(libraryReservations)
	app.reservationsPending = !online
	app.view = "reservations"
	return app, ui.NewTester(app.Render, 1200, 800)
}
