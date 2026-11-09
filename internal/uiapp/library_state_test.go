package uiapp

import (
	"context"
	"sync"
	"testing"
)

type resultMutationProvider struct {
	mu sync.Mutex

	favorite    bool
	like        bool
	reservation bool
}

func (p *resultMutationProvider) ToggleFavorite(context.Context, string, string, string, string, int) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.favorite, nil
}

func (p *resultMutationProvider) ToggleLike(context.Context, string, string, string, string, int) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.like, nil
}

func (p *resultMutationProvider) ToggleReservation(context.Context, string, string, string, string, string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reservation, nil
}

func TestRemoveLibraryItemKeepsRowsWhenRemoteStillEnabled(t *testing.T) {
	tests := []struct {
		name string
		tab  LibraryTab
	}{
		{name: "favorite", tab: libraryFavorites},
		{name: "like", tab: libraryLikes},
		{name: "reservation", tab: libraryReservations},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewTestApp()
			app.libraryMutations = &resultMutationProvider{
				favorite:    true,
				like:        true,
				reservation: true,
			}
			other := CatalogItem{ID: "other-series", EpisodeID: "other-episode"}
			item := CatalogItem{ID: "series", EpisodeID: "episode"}
			switch test.tab {
			case libraryFavorites:
				app.library.Favorites = Library{Items: []CatalogItem{item, other}}
			case libraryLikes:
				app.library.Likes = Library{Items: []CatalogItem{item, other}}
			case libraryReservations:
				item.Reserved = true
				app.library.Reserved = Library{Items: []CatalogItem{item, other}}
			}
			app.RemoveLibraryItem(test.tab, item)
			state := app.LibraryState()
			library := map[LibraryTab]Library{libraryFavorites: state.Favorites, libraryLikes: state.Likes, libraryReservations: state.Reserved}[test.tab]
			if len(library.Items) != 2 {
				t.Fatalf("remote-enabled item was removed: %+v", library.Items)
			}
			if library.Items[0].ID != item.ID {
				t.Fatalf("remote-enabled item was reordered: %+v", library.Items)
			}
			if test.tab == libraryReservations && !library.Items[0].Reserved {
				t.Fatalf("remote-enabled reservation is unselected: %+v", library.Items[0])
			}
		})
	}
}

func TestRemoveLibraryItemRemovesOnlyRemoteDisabledRows(t *testing.T) {
	otherLike := CatalogItem{ID: "series", EpisodeID: "other-episode"}
	item := CatalogItem{ID: "series", EpisodeID: "episode"}
	app := NewTestApp()
	app.libraryMutations = &resultMutationProvider{}
	app.library.Favorites = Library{Items: []CatalogItem{item}}
	app.library.Likes = Library{Items: []CatalogItem{item, otherLike}}
	app.library.Reserved = Library{Items: []CatalogItem{item}}
	app.detail = Detail{ID: item.ID, Favorite: true, Reserved: true}

	app.RemoveLibraryItem(libraryFavorites, item)
	app.RemoveLibraryItem(libraryLikes, item)
	app.RemoveLibraryItem(libraryReservations, item)

	if len(app.library.Favorites.Items) != 0 {
		t.Fatalf("remote-disabled favorite was kept: %+v", app.library.Favorites.Items)
	}
	if len(app.library.Likes.Items) != 1 || app.library.Likes.Items[0].EpisodeID != otherLike.EpisodeID {
		t.Fatalf("remote-disabled like removed wrong episode: %+v", app.library.Likes.Items)
	}
	if len(app.library.Reserved.Items) != 0 {
		t.Fatalf("remote-disabled reservation was kept: %+v", app.library.Reserved.Items)
	}
	detail := app.DetailState()
	if detail.Favorite || detail.Reserved {
		t.Fatalf("remote-disabled detail state was kept: %+v", detail)
	}
}
