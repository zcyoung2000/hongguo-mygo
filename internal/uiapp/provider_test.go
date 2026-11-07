package uiapp

import (
	"context"
	"sync/atomic"

	"myguo/internal/ipc"
)

type fakeDetail struct {
	page      *ipc.SeriesDetailPageDTO
	delay     chan struct{}
	pages     atomic.Int64
	filters   atomic.Int64
	discovers atomic.Int64
}

func (f *fakeDetail) GetSeriesPage(context.Context, string) (*ipc.SeriesDetailPageDTO, error) {
	f.pages.Add(1)
	return f.page, nil
}
func (f *fakeDetail) GetSeriesRelated(context.Context, string) (*ipc.SeriesCollectionDTO, error) {
	return &ipc.SeriesCollectionDTO{}, nil
}
func (f *fakeDetail) GetFilterPanel(context.Context, string) ([]ipc.FilterRowDTO, error) {
	f.filters.Add(1)
	return nil, nil
}
func (f *fakeDetail) GetDiscover(ctx context.Context, genre string, values map[string]string, cursor string) (ipc.DiscoverResponse, error) {
	f.discovers.Add(1)
	if f.delay != nil {
		select {
		case <-f.delay:
		case <-ctx.Done():
		}
	}
	return ipc.DiscoverResponse{}, nil
}

type fakeFavorite struct {
	favorite         bool
	reserved         bool
	favoriteCalls    atomic.Int64
	reservationCalls atomic.Int64
}

func (f *fakeFavorite) ToggleFavorite(context.Context, string, string, string, string, int) (bool, error) {
	f.favoriteCalls.Add(1)
	return f.favorite, nil
}
func (f *fakeFavorite) ToggleReservation(context.Context, string, string, string, string, string) (bool, error) {
	f.reservationCalls.Add(1)
	return f.reserved, nil
}

type fakePlayback struct {
	playback *ipc.PlaybackDTO
	calls    atomic.Int64
	quality  string
}

func (f *fakePlayback) PlayEpisode(_ context.Context, _, _ string, num int, quality string, _ bool) (*ipc.PlaybackDTO, error) {
	f.calls.Add(1)
	f.quality = quality
	if f.playback == nil {
		return nil, nil
	}
	result := *f.playback
	result.EpisodeNum = num
	return &result, nil
}
