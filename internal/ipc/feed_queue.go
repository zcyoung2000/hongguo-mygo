package ipc

import (
	"context"
	"encoding/json"
	"fmt"

	userstore "myguo/internal/store"
)

func feedChannelKey(channel string) string {
	if channel == "" {
		return "推荐"
	}
	return channel
}

// RestoreRecommendFeed only reads local state. Playback URLs are resolved by
// PlayEpisode against the current MediaServer, so restarts cannot reuse a dead
// localhost port or an expired upstream playback URL.
func (s *HongguoService) RestoreRecommendFeed(ctx context.Context, channel string) (*RecommendFeedResult, error) {
	s.feedMu.Lock()
	defer s.feedMu.Unlock()
	key := feedChannelKey(channel)
	entries, err := s.store.GetFeedQueue(key)
	if err != nil {
		return nil, err
	}
	progress, _ := s.store.GetFeedProgress(key)
	items := make([]SeriesCard, 0, len(entries))
	for _, entry := range entries {
		var card SeriesCard
		if err := json.Unmarshal(entry.Payload, &card); err != nil {
			return nil, fmt.Errorf("restore recommend queue: %w", err)
		}
		card.CoverURL = s.media.ImageURL(card.CoverURL)
		items = append(items, card)
	}
	return sanitizeIPC(&RecommendFeedResult{Items: items, Cursor: progress.Cursor, HasMore: true}), nil
}

func (s *HongguoService) ConsumeRecommendFeedItem(ctx context.Context, channel, seriesID string) error {
	// Store serializes this with queue writes. Do not wait for feedMu here:
	// a slow background Discover must not delay an on-screen acknowledgement.
	return s.store.ConsumeFeedQueue(feedChannelKey(channel), seriesID)
}

func (s *HongguoService) queueRecommendCards(channel string, cards []SeriesCard) error {
	entries := make([]userstore.FeedQueueEntry, 0, len(cards))
	for _, card := range cards {
		payload, err := json.Marshal(card)
		if err != nil {
			return err
		}
		entries = append(entries, userstore.FeedQueueEntry{SeriesID: card.ID, Payload: payload})
	}
	return s.store.AppendFeedQueue(channel, entries)
}
