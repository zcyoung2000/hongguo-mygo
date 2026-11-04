package ipc

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// Relevance buckets keep a same-theme/filter result ahead of a merely
// popular result. Upstream popularity only orders items inside a bucket.
const (
	relatedRelevanceOfficial = 5
	relatedRelevanceFilter   = 4
	relatedRelevanceActor    = 3
	relatedRelevanceTag      = 2
)

type relatedCandidate struct {
	summary   model.SeriesSummary
	relevance int
	order     int
}

var seasonTitleMarker = regexp.MustCompile(
	`(?i)(第\s*[0-9零一二三四五六七八九十百]+\s*季|season\s*[0-9]+|s[0-9]+\b)`,
)

func (s *HongguoService) GetSeriesRelated(ctx context.Context, seriesID string) (*SeriesCollectionDTO, error) {
	seriesID = strings.TrimSpace(seriesID)
	if seriesID == "" {
		return nil, fmt.Errorf("series ID cannot be empty")
	}

	cacheKey := "series_related:" + seriesID
	if collection, ok := s.cachedCollection(cacheKey); ok {
		return collection, nil
	}

	// The detail request is normally already cached by the page itself. It
	// supplies the metadata that replaces the anonymous endpoint's generic
	// "more good dramas" stream.
	page, err := s.GetSeriesPage(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch related series: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	candidates := s.relatedCandidates(ctx, page)
	seasonNumbers := s.relatedSeasonNumbers(ctx, page.ID)
	ids := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates)+1)
	for _, candidate := range candidates {
		id := candidate.summary.ID
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	detailMap := s.fetchSeriesBatchMap(ctx, ids)
	candidates = rankRelatedCandidates(candidates, detailMap)
	applyRelatedSeasonTitles(candidates, detailMap, seasonNumbers)
	cards := make([]SeriesCard, 0, len(candidates))
	for _, candidate := range candidates {
		cards = append(cards, s.relatedCard(candidate.summary, detailMap[candidate.summary.ID]))
		if len(cards) >= 18 {
			break
		}
	}

	collection := &SeriesCollectionDTO{Title: "相关推荐", Items: cards}
	if len(cards) == 0 {
		collection.Title = ""
	}
	collection = sanitizeIPC(collection)

	s.cacheCollection(cacheKey, collection)
	return collection, nil
}

func (s *HongguoService) relatedCandidates(ctx context.Context, page *SeriesDetailPageDTO) []relatedCandidate {
	var candidates []relatedCandidate
	// The SDK falls back to any non-empty plan cell, including the anonymous
	// "更多好剧" stream. Accept only a session-bound related-video cell; when
	// the server answers with that generic stream, metadata filters below run.
	if related, err := retryAnonymous(s, ctx, func(c context.Context) (*model.Recommend, error) {
		return s.client.SeriesRecommend(c, page.ID)
	}); err == nil && isOfficialRelatedCell(related) {
		candidates = append(candidates, relatedSourceCandidates(related.Items, relatedRelevanceOfficial)...)
	}
	if selections := s.relatedFilterSelections(ctx, page); len(selections) > 0 {
		if res, err := retryAnonymous(s, ctx, func(c context.Context) (*model.ShortFilterPage, error) {
			return s.client.ShortFilter(c, sdk.ShortFilterQuery{Selections: selections})
		}); err == nil {
			candidates = append(candidates, relatedSourceCandidates(shortFilterItems(res), relatedRelevanceFilter)...)
		}
	}

	// Exact metadata filters can be absent for niche details. Actor and tag
	// searches give the fallback a real connection to the current drama;
	// enrichment and ranking below prevent generic search rows from winning.
	if len(candidates) < 12 {
		for _, actor := range page.Actors {
			if actor.Name == "" {
				continue
			}
			if searchPage, err := s.relatedSearch(ctx, actor.Name, page.ContentType); err == nil {
				candidates = append(candidates, relatedSourceCandidates(pageItems(searchPage), relatedRelevanceActor)...)
			}
			if len(candidates) >= 30 {
				return candidates
			}
			break
		}
		for _, tag := range relatedSearchTerms(page) {
			if searchPage, err := s.relatedSearch(ctx, tag, page.ContentType); err == nil {
				candidates = append(candidates, relatedSourceCandidates(pageItems(searchPage), relatedRelevanceTag)...)
			}
			if len(candidates) >= 30 {
				break
			}
		}
	}
	return candidates
}

func isOfficialRelatedCell(related *model.Recommend) bool {
	return related != nil &&
		related.SessionID != "" &&
		strings.TrimSpace(related.CellTitle) != "" &&
		strings.TrimSpace(related.CellTitle) != "更多好剧"
}

func (s *HongguoService) relatedSearch(ctx context.Context, query string, contentType int) (*sdk.Page[model.SeriesSummary], error) {
	opts := sdk.SearchOptions{PageSize: 20}
	switch {
	case model.ContentType(contentType) == model.ContentMotionComic || strings.Contains(query, "漫剧"):
		opts.Filters = []string{sdk.SearchFilterComic}
	case strings.Contains(query, "AI剧"):
		opts.Filters = []string{sdk.SearchFilterAIComic}
	default:
		opts.Filters = []string{sdk.SearchFilterShortPlay}
	}
	return retryAnonymous(s, ctx, func(c context.Context) (*sdk.Page[model.SeriesSummary], error) {
		return s.client.Search(c, query, opts)
	})
}

func (s *HongguoService) relatedFilterPanel(ctx context.Context, genre string) []model.FilterRow {
	cacheKey := "series_related_panel:" + genre
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		rows, _ := entry.val.([]model.FilterRow)
		return rows
	}
	s.cacheMu.RUnlock()

	rows, err := retryAnonymous(s, ctx, func(c context.Context) ([]model.FilterRow, error) {
		return s.client.ShortFilterPanel(c, genre)
	})
	if err != nil {
		return nil
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: rows, expiresAt: time.Now().Add(time.Hour)}
	s.cacheMu.Unlock()
	return rows
}

// The detail rows contain user-facing category names. Map up to two exact
// matches to the official filter dimensions; more dimensions make the server
// query unnecessarily narrow. Sort by heat so the section is not a generic
// feed of unknown dramas.
func (s *HongguoService) relatedFilterSelections(ctx context.Context, page *SeriesDetailPageDTO) map[string][]string {
	if page == nil {
		return nil
	}

	terms := relatedTerms(page)
	if len(terms) == 0 {
		return nil
	}

	panel := s.relatedFilterPanel(ctx, relatedGenre(page))
	selections := make(map[string][]string)
	for _, row := range panel {
		if row.Key == "genre" || row.Key == "sort" {
			continue
		}
		var values []string
		for _, option := range row.Options {
			for _, term := range terms {
				if strings.EqualFold(strings.TrimSpace(option.Name), term) {
					values = append(values, option.Value)
					break
				}
			}
		}
		if len(values) > 0 {
			if len(values) > 2 {
				values = values[:2]
			}
			selections[row.Key] = values
			if len(selections) >= 2 {
				break
			}
		}
	}
	if len(selections) == 0 {
		return nil
	}

	for _, row := range panel {
		if row.Key != "sort" {
			continue
		}
		for _, option := range row.Options {
			if option.Value == "hot_score" {
				selections["sort"] = []string{option.Value}
				return selections
			}
		}
	}
	return selections
}

func relatedGenre(page *SeriesDetailPageDTO) string {
	if page == nil {
		return "short_play"
	}
	if page.ContentType == int(model.ContentMotionComic) {
		return "comic_series"
	}
	for _, term := range relatedTerms(page) {
		switch term {
		case "漫剧":
			return "comic_series"
		case "AI剧":
			return "ai_series"
		}
	}
	return "short_play"
}

func relatedTerms(page *SeriesDetailPageDTO) []string {
	if page == nil {
		return nil
	}
	terms := make([]string, 0, len(page.Tags)+1)
	seen := make(map[string]bool)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		terms = append(terms, value)
	}
	add(page.Category)
	for _, tag := range page.Tags {
		add(tag)
	}
	return terms
}

func relatedSearchTerms(page *SeriesDetailPageDTO) []string {
	var terms []string
	for _, term := range relatedTerms(page) {
		switch term {
		case "短剧", "真人剧", "漫剧", "AI剧", "电影", "电视剧", "场景剧":
			continue
		}
		if len([]rune(term)) < 2 {
			continue
		}
		terms = append(terms, term)
		if len(terms) >= 2 {
			return terms
		}
	}
	return terms
}

func relatedSourceCandidates(items []model.SeriesSummary, relevance int) []relatedCandidate {
	candidates := make([]relatedCandidate, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		candidates = append(candidates, relatedCandidate{
			summary:   item,
			relevance: relevance,
			order:     len(candidates),
		})
	}
	return candidates
}

func (s *HongguoService) relatedSeasonNumbers(ctx context.Context, seriesID string) map[string]int {
	album := s.cachedSeriesAlbum(seriesID)
	if album == nil {
		result, err := retryAnonymous(s, ctx, func(c context.Context) (*model.Album, error) {
			return s.client.Seasons(c, seriesID)
		})
		if err != nil {
			return nil
		}
		album = result
		s.cacheSeriesAlbum(seriesID, album)
	}
	return seasonNumbers(album)
}

func seasonNumbers(album *model.Album) map[string]int {
	if album == nil || len(album.Seasons) < 2 {
		return nil
	}

	numbers := make(map[string]int, len(album.Seasons))
	for i, season := range album.Seasons {
		number := season.Index
		if number == 0 {
			number = i + 1
		}
		if season.SeriesID != "" && number > 0 {
			numbers[season.SeriesID] = number
		}
	}
	return numbers
}

// Season rows often reuse the album title. Preserve every season as a real
// catalog item, but make the related card visually distinguishable.
func applyRelatedSeasonTitles(
	candidates []relatedCandidate,
	detailMap map[string]*model.Series,
	seasonNumbers map[string]int,
) {
	for i := range candidates {
		number, ok := seasonNumbers[candidates[i].summary.ID]
		if !ok {
			continue
		}

		candidate := &candidates[i]
		summary := candidate.summary
		detail := detailMap[summary.ID]
		if hasSeasonTitleMarker(summary.Title) || (detail != nil && hasSeasonTitleMarker(detail.Title)) {
			continue
		}
		title := summary.Title
		if detail != nil && detail.Title != "" {
			title = detail.Title
		}
		title = seasonDisplayTitle(title, number)
		if title == "" || title == summary.Title {
			continue
		}
		candidate.summary.Title = title
		if detail != nil {
			detail.SeriesSummary.Title = title
		}
	}
}

func hasSeasonTitleMarker(title string) bool {
	return seasonTitleMarker.MatchString(strings.TrimSpace(title))
}

func rankRelatedCandidates(candidates []relatedCandidate, detailMap map[string]*model.Series) []relatedCandidate {
	scored := make([]relatedCandidate, len(candidates))
	copy(scored, candidates)
	for i := range scored {
		detail := detailMap[scored[i].summary.ID]
		scored[i].order = i
		scored[i].relevance = scored[i].relevance*1_000_000_000 + int(seriesPopularity(detail))
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].relevance != scored[j].relevance {
			return scored[i].relevance > scored[j].relevance
		}
		return scored[i].order < scored[j].order
	})
	return scored
}

func seriesPopularity(series *model.Series) int64 {
	if series == nil {
		return 0
	}
	// Heat is the app's primary ranking signal. Play/follow/digg counts are
	// small tie-breakers and tolerate endpoints that omit heat.
	return series.Heat*8 + series.PlayCount/10_000 + series.FollowCount*4 + series.DiggCount*8
}

func (s *HongguoService) relatedCard(summary model.SeriesSummary, detail *model.Series) SeriesCard {
	d := summary
	status := "finished"
	heat := "热播"
	if detail != nil {
		d = detail.SeriesSummary
		upcoming := isUpcomingSeries(detail.EpisodeCount, detail.RightText)
		status = "finished"
		if upcoming {
			status = "upcoming"
			heat = "待上线"
		} else {
			switch detail.Status {
			case model.SeriesUpdating:
				status = "updating"
				heat = "更新中"
				if detail.RightText != "" && !strings.HasPrefix(detail.RightText, "全0") {
					heat = detail.RightText
				}
			case model.SeriesEnd:
				heat = "已完结"
			case model.SeriesUpdatedToday:
				heat = "今日更新"
			}
		}
		if detail.Heat > 0 {
			heat = formatHeatScore(detail.Heat) + "热度"
		}
	}

	if d.EpisodeCount == 0 && detail == nil {
		status = "upcoming"
		heat = "待上线"
	}

	tags := seriesSummaryTags(d)
	return SeriesCard{
		ID:           d.ID,
		Title:        d.Title,
		Description:  d.Description,
		CoverURL:     s.media.ImageURL(d.CoverURL),
		EpisodeCount: d.EpisodeCount,
		Category:     seriesContentTypeLabel(int(relatedContentType(detail, d))),
		ContentType:  int(relatedContentType(detail, d)),
		Author:       d.Author,
		Status:       status,
		Heat:         heat,
		Tags:         tags,
		Upcoming:     status == "upcoming",
		Reserved:     s.store.HasReservation(d.ID),
	}
}

func relatedContentType(detail *model.Series, summary model.SeriesSummary) model.ContentType {
	if detail != nil {
		return detail.ContentType
	}
	if summary.Category == "漫剧" {
		return model.ContentMotionComic
	}
	return model.ContentShortSeries
}
