package ipc

import (
	"context"

	"myguo/internal/library"
	"myguo/internal/native"
)

func CloseService(s *HongguoService) { s.libraryService().Close() }

func (s *HongguoService) libraryService() *library.Service {
	s.libraryOnce.Do(func() {
		s.library = library.NewService(library.Dependencies{
			Store:       s.store,
			Client:      s.client,
			ImageURL:    s.imageURL,
			Credentials: s.authCredentials,
			AuthSession: s.authSession,
			HTTPClient:  s.httpClient,
			Episodes:    s.listEpisodes,
			DeviceIdentity: func() (string, string, string, string) {
				return s.deviceIdentity()
			},
			Runtime: func() (string, string) {
				return s.runtimeIdentity()
			},
			InvalidateCaches: s.invalidateCatalogCaches,
			Notify: func(ctx context.Context, notification library.Notification) {
				_, _ = s.SendNativeNotification(ctx, native.NotificationArgs{
					Title:    notification.Title,
					Body:     notification.Body,
					SeriesID: notification.SeriesID,
				})
			},
		})
	})
	return s.library
}

func (s *HongguoService) invalidateCatalogCaches(prefixes ...string) {
	s.cache.InvalidatePrefixes(prefixes...)
	s.catalogService().InvalidatePrefixes(prefixes...)
}

func (s *HongguoService) SaveProgress(ctx context.Context, seriesID, episodeID string, title, coverURL string, episodeNum, totalEpisodes int, positionSec, durationSec float64) error {
	return s.libraryService().SaveProgress(ctx, seriesID, episodeID, title, coverURL, episodeNum, totalEpisodes, positionSec, durationSec)
}

func (s *HongguoService) GetHistory(ctx context.Context) ([]HistoryItem, error) {
	return s.libraryService().GetHistory(ctx)
}

func (s *HongguoService) RefreshHistory(ctx context.Context) ([]HistoryItem, error) {
	return s.libraryService().RefreshHistory(ctx)
}

func (s *HongguoService) ToggleFavorite(ctx context.Context, seriesID, title, coverURL, desc string, totalEpisodes int) (bool, error) {
	return s.libraryService().ToggleFavorite(ctx, seriesID, title, coverURL, desc, totalEpisodes)
}

func (s *HongguoService) IsFavorite(ctx context.Context, seriesID string) (bool, error) {
	return s.libraryService().IsFavorite(ctx, seriesID)
}

func (s *HongguoService) GetFavorites(ctx context.Context) ([]FavoriteItem, error) {
	return s.libraryService().GetFavorites(ctx)
}

func (s *HongguoService) RefreshFavorites(ctx context.Context) ([]FavoriteItem, error) {
	return s.libraryService().RefreshFavorites(ctx)
}

func (s *HongguoService) ToggleLike(ctx context.Context, seriesID, episodeID, title, coverURL string, episodeNum int) (bool, error) {
	return s.libraryService().ToggleLike(ctx, seriesID, episodeID, title, coverURL, episodeNum)
}

func (s *HongguoService) GetLikes(ctx context.Context) ([]LikeItem, error) {
	return s.libraryService().GetLikes(ctx)
}

func (s *HongguoService) RefreshLikes(ctx context.Context) ([]LikeItem, error) {
	return s.libraryService().RefreshLikes(ctx)
}

func (s *HongguoService) ToggleReservation(ctx context.Context, seriesID, title, coverURL, note, publishAt string) (bool, error) {
	return s.libraryService().ToggleReservation(ctx, seriesID, title, coverURL, note, publishAt)
}

func (s *HongguoService) GetReservations(ctx context.Context) ([]ReservationItem, error) {
	return s.libraryService().GetReservations(ctx)
}

func (s *HongguoService) RefreshReservations(ctx context.Context) ([]ReservationItem, error) {
	return s.libraryService().RefreshReservations(ctx)
}
