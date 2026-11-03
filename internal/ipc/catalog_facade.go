package ipc

import (
	"context"

	"myguo/internal/catalog"
)

func (s *HongguoService) imageURL(raw string) string {
	if s.media == nil {
		return raw
	}
	return s.media.ImageURL(raw)
}

func (s *HongguoService) catalogService() *catalog.Service {
	s.catalogOnce.Do(func() {
		s.catalog = catalog.NewService(s.client, s.imageURL, s.httpClient, catalog.Dependencies{
			Credentials: s.authCredentials,
			DeviceIdentity: func() (string, string, string, string) {
				return s.deviceIdentity()
			},
			Runtime: func() (string, string) {
				return s.userAgent, s.storeRegion
			},
			ReservedSeriesIDs: func(ctx context.Context) ([]string, error) {
				items, err := s.GetReservations(ctx)
				if err != nil {
					return nil, err
				}
				ids := make([]string, 0, len(items))
				for _, item := range items {
					ids = append(ids, item.SeriesID)
				}
				return ids, nil
			},
			Episodes:   s.listEpisodes,
			IsFavorite: func(seriesID string) bool { return s.store.IsFavorite(seriesID) },
		})
	})
	return s.catalog
}

func (s *HongguoService) GetFilterPanel(ctx context.Context, genre string) ([]FilterRowDTO, error) {
	value, err := s.catalogService().GetFilterPanel(ctx, genre)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetDiscover(ctx context.Context, genre string, selections map[string]string, cursor string) (DiscoverResponse, error) {
	value, err := s.catalogService().GetDiscover(ctx, genre, selections, cursor)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetRankings(ctx context.Context, contentID, boardID, cursor string) (RankResponse, error) {
	value, err := s.catalogService().GetRankings(ctx, contentID, boardID, cursor)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetActorProfile(ctx context.Context, userID string) (*ActorProfileDTO, error) {
	value, err := s.catalogService().GetActorProfile(ctx, userID)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetActorWorks(ctx context.Context, userID, cursor string) (*ActorWorksResponse, error) {
	value, err := s.catalogService().GetActorWorks(ctx, userID, cursor)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetNewReleases(ctx context.Context, date string) (NewReleaseResponse, error) {
	value, err := s.catalogService().GetNewReleases(ctx, date)
	return sanitizeIPC(value), err
}

func (s *HongguoService) Search(ctx context.Context, query string) ([]SeriesCard, error) {
	value, err := s.catalogService().Search(ctx, query)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetSuggestions(ctx context.Context, prefix string) ([]string, error) {
	value, err := s.catalogService().GetSuggestions(ctx, prefix)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetHotSearch(ctx context.Context) (HotSearchDTO, error) {
	value, err := s.catalogService().GetHotSearch(ctx)
	return sanitizeIPC(value), err
}

func (s *HongguoService) GetSeriesDetail(ctx context.Context, seriesID string) (*SeriesDetailDTO, error) {
	value, err := s.catalogService().GetSeriesDetail(ctx, seriesID)
	return sanitizeIPC(value), err
}
