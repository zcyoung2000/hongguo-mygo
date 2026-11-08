package store

import "testing"

func TestEmptyLibraryTitlesMigrateWithoutShowingEpisodeZero(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if _, err := store.db.Exec(`
		INSERT INTO history (series_id, episode_num) VALUES ('710000000000001', 0);
		INSERT INTO favorites (series_id) VALUES ('710000000000002');
		INSERT INTO likes (series_id, episode_id) VALUES ('710000000000003', '710000000000103');
		INSERT INTO reservations (series_id) VALUES ('710000000000004');
	`); err != nil {
		t.Fatalf("seed empty rows error = %v", err)
	}
	if err := store.migrateEmptyLibraryTitles(); err != nil {
		t.Fatalf("migrateEmptyLibraryTitles() error = %v", err)
	}

	history, err := store.GetHistory()
	if err != nil || len(history) != 1 || history[0].Title != "未知短剧" {
		t.Fatalf("history titles = %#v, err = %v", history, err)
	}
	favorites, err := store.GetFavorites()
	if err != nil || len(favorites) != 1 || favorites[0].Title != "未知短剧" {
		t.Fatalf("favorites = %#v, err = %v", favorites, err)
	}
	likes, err := store.GetLikes()
	if err != nil || len(likes) != 1 || likes[0].Title != "未知短剧" {
		t.Fatalf("likes = %#v, err = %v", likes, err)
	}
	reservations, err := store.GetReservations()
	if err != nil || len(reservations) != 1 || reservations[0].Title != "未知短剧" {
		t.Fatalf("reservations = %#v, err = %v", reservations, err)
	}
}
