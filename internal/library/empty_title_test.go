package library

import (
	"context"
	"net/http"
	"testing"
)

func TestLibraryMutationsUseHistoryTitleFallback(t *testing.T) {
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	ctx := context.Background()

	if _, err := service.ToggleFavorite(ctx, "710000000000001", "", "", "", 0); err != nil {
		t.Fatalf("ToggleFavorite() error = %v", err)
	}
	favorites, err := store.GetFavorites()
	if err != nil || len(favorites) != 1 || favorites[0].Title != "未知短剧" {
		t.Fatalf("favorite title = %#v, err = %v", favorites, err)
	}

	if _, err := service.ToggleLike(ctx, "710000000000002", "710000000000102", "", "", 0); err != nil {
		t.Fatalf("ToggleLike() error = %v", err)
	}
	likes, err := store.GetLikes()
	if err != nil || len(likes) != 1 || likes[0].Title != "未知短剧" {
		t.Fatalf("like title = %#v, err = %v", likes, err)
	}
}
