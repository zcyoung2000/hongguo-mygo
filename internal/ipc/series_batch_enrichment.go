package ipc

import (
	"context"
	"github.com/beilunyang/openhongguo/sdk/model"
	"time"
)

const (
	// The SDK verified that 20 IDs are accepted and 30 are rejected, but keeps
	// each batch at 10 so a response stays under roughly 1 MB.
	seriesBatchSize    = 10
	seriesBatchTimeout = 8 * time.Second
)

func (s *HongguoService) enrichSeriesCards(ctx context.Context, cards []SeriesCard) []SeriesCard {
	if len(cards) == 0 {
		return cards
	}
	batchCtx, cancel := context.WithTimeout(ctx, seriesBatchTimeout)
	defer cancel()
	batch := s.client.GetSeriesBatch
	if s.seriesBatch != nil {
		batch = s.seriesBatch
	}
	details, err := batch(batchCtx, seriesBatchIDs(cards, seriesBatchSize))
	if err != nil || len(details) == 0 {
		return cards
	}
	byID := make(map[string]*model.Series, len(details))
	for i := range details {
		if details[i].Series != nil {
			byID[details[i].Series.ID] = details[i].Series
		}
	}
	for i := range cards {
		detail := byID[cards[i].ID]
		if detail == nil {
			continue
		}
		cards[i] = seriesCardFromDetail(cards[i], detail, s.imageURL)
	}
	return cards
}

func seriesBatchIDs(cards []SeriesCard, limit int) []string {
	ids := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for _, card := range cards {
		if len(ids) == limit {
			break
		}
		if card.ID == "" || seen[card.ID] {
			continue
		}
		seen[card.ID] = true
		ids = append(ids, card.ID)
	}
	return ids
}

func seriesCardFromDetail(card SeriesCard, detail *model.Series, imageURL func(string) string) SeriesCard {
	if detail == nil {
		return card
	}
	if detail.Title != "" {
		card.Title = detail.Title
	}
	if detail.Description != "" {
		card.Description = detail.Description
	}
	if detail.CoverURL != "" && time.Now().Before(detail.CoverExpiresAt) {
		card.CoverURL = imageURL(detail.CoverURL)
	}
	if detail.EpisodeCount > 0 {
		card.EpisodeCount = detail.EpisodeCount
	}
	if detail.Heat > 0 {
		card.Heat = formatHeatScore(detail.Heat)
	}
	return card
}
