package uiapp

import "testing"

import "myguo/internal/ipc"

func TestConvertedLibraryTitlesUseHistoryFallback(t *testing.T) {
	const seriesID = "710000000000001"
	history, err := convertHistory([]ipc.HistoryItem{{SeriesID: seriesID, EpisodeNum: 0}}, nil)
	if err != nil {
		t.Fatalf("convertHistory() error = %v", err)
	}
	if len(history) != 1 || history[0].Title != "未知短剧" {
		t.Fatalf("history title = %#v", history)
	}

	favorites, err := convertFavorites([]ipc.FavoriteItem{{SeriesID: seriesID}}, nil)
	if err != nil {
		t.Fatalf("convertFavorites() error = %v", err)
	}
	if len(favorites) != 1 || favorites[0].Title != "未知短剧" {
		t.Fatalf("favorite title = %#v", favorites)
	}

	likes, err := convertLikes([]ipc.LikeItem{{SeriesID: seriesID, EpisodeID: "710000000000101"}}, nil)
	if err != nil {
		t.Fatalf("convertLikes() error = %v", err)
	}
	if len(likes) != 1 || likes[0].Title != "未知短剧" {
		t.Fatalf("like title = %#v", likes)
	}

	reservations, err := convertReservations([]ipc.ReservationItem{{SeriesID: seriesID}}, nil)
	if err != nil {
		t.Fatalf("convertReservations() error = %v", err)
	}
	if len(reservations) != 1 || reservations[0].Title != "未知短剧" {
		t.Fatalf("reservation title = %#v", reservations)
	}
}
