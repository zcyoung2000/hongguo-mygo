package ipc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/sanitize"
)

type ActorInfo struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

type SourceNovelDTO struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Reads    int64    `json:"reads"`
	CoverURL string   `json:"coverUrl"`
	Tags     []string `json:"tags"`
}

type SeriesDetailPageDTO struct {
	SeriesDetailDTO
	Category      string          `json:"category"`
	ContentType   int             `json:"contentType,omitempty"`
	Tags          []string        `json:"tags"`
	Subtitles     []string        `json:"subtitles"`
	Actors        []ActorInfo     `json:"actors"`
	PlayCount     int64           `json:"playCount"`
	FollowCount   int64           `json:"followCount"`
	LikeCount     int64           `json:"likeCount"`
	Source        *SourceNovelDTO `json:"source,omitempty"`
	EpisodeTitle  string          `json:"episodeTitle"`
	EpisodesTitle string          `json:"episodesTitle"`
	StatusText    string          `json:"statusText"`
	PublishText   string          `json:"publishText,omitempty"`
	Upcoming      bool            `json:"upcoming,omitempty"`
	Reserved      bool            `json:"reserved,omitempty"`
}

type SeriesCollectionDTO struct {
	Title string       `json:"title"`
	Items []SeriesCard `json:"items"`
}

func retryAnonymous[T any](s *HongguoService, ctx context.Context, op func(context.Context) (T, error)) (T, error) {
	val, err := op(ctx)
	if err == nil {
		return val, nil
	}
	if errorsIsAnonymous(err) {
		_ = s.client.ResetDevice(ctx)
		return op(ctx)
	}
	return val, err
}

func errorsIsAnonymous(err error) bool {
	if errors.Is(err, sdk.ErrAnonymousLimited) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "anonymous")
}

const (
	seriesDetailTimeout    = 8 * time.Second
	seriesPageDetailBudget = 12 * time.Second
)

func (s *HongguoService) GetSeriesPage(ctx context.Context, seriesID string) (*SeriesDetailPageDTO, error) {
	seriesID = strings.TrimSpace(seriesID)
	if seriesID == "" {
		return nil, fmt.Errorf("series ID cannot be empty")
	}

	cacheKey := "series_page:" + seriesID
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return s.withSeriesUserState(entry.val.(*SeriesDetailPageDTO)), nil
	}
	s.cacheMu.RUnlock()

	// The batch and single-detail endpoints must not share one deadline.
	// Otherwise a slow batch consumes the budget and guarantees that the
	// fallback fails with the same context error, turning a transient
	// endpoint delay into a failed detail page.
	batchCtx, batchCancel := context.WithTimeout(ctx, seriesDetailTimeout)
	batch, batchErr := retryAnonymous(s, batchCtx, func(c context.Context) ([]model.SeriesDetail, error) {
		return s.client.GetSeriesBatch(c, []string{seriesID})
	})
	batchCancel()

	var detail model.SeriesDetail
	switch {
	case batchErr == nil && len(batch) > 0 && batch[0].Series != nil:
		detail = batch[0]
	default:
		// Some live-action cards resolve through the player batch endpoint but
		// are missing (or arrive without episodes) from that response. The
		// single-series detail endpoint is the detail page's fallback.
		seriesCtx, seriesCancel := context.WithTimeout(ctx, seriesDetailTimeout)
		series, seriesErr := retryAnonymous(s, seriesCtx, func(c context.Context) (*model.Series, error) {
			return s.client.GetSeries(c, seriesID)
		})
		seriesCancel()
		if seriesErr != nil || series == nil {
			if seriesErr == nil {
				seriesErr = fmt.Errorf("series not found")
			}
			if batchErr != nil {
				seriesErr = fmt.Errorf("%v (batch: %v)", seriesErr, batchErr)
			}
			return nil, fmt.Errorf("failed to fetch series detail: %w", seriesErr)
		}
		detail = model.SeriesDetail{Series: series}
	}

	if len(detail.Episodes) == 0 {
		episodesCtx, episodesCancel := context.WithTimeout(ctx, seriesDetailTimeout)
		if episodes, episodeErr := s.client.ListEpisodes(episodesCtx, seriesID, sdk.EpisodeOptions{}); episodeErr == nil && episodes != nil {
			detail.Episodes = episodes.Items
		}
		episodesCancel()
	}

	page := s.newSeriesDetailPage(detail, seriesID)
	page.EpisodeTitle = "选集"
	page.EpisodesTitle = fmt.Sprintf("共 %d 集", page.EpisodeCount)
	page = sanitizeIPC(page)

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: page, expiresAt: time.Now().Add(5 * time.Minute)}
	s.cacheMu.Unlock()

	return s.withSeriesUserState(page), nil
}

// Only catalog metadata may be reused. User flags are read from the local
// library for every response, without mutating the shared cached DTO.
func (s *HongguoService) withSeriesUserState(page *SeriesDetailPageDTO) *SeriesDetailPageDTO {
	result := *page
	result.IsFavorite = s.store.IsFavorite(result.ID)
	result.Reserved = s.store.HasReservation(result.ID)
	return &result
}

func (s *HongguoService) GetSeriesSeasons(ctx context.Context, seriesID string) (*SeriesCollectionDTO, error) {
	return s.getSeasonCollection(ctx, seriesID, false)
}

// GetNextSeason returns the first playable season after the current series.
// Upcoming seasons are skipped because their episodes cannot be resolved yet.
func (s *HongguoService) GetNextSeason(ctx context.Context, seriesID string) (*SeriesCard, error) {
	collection, err := s.getSeasonCollection(ctx, seriesID, true)
	if err != nil {
		return nil, err
	}
	return nextPlayableSeason(seriesID, collection), nil
}

func (s *HongguoService) getSeasonCollection(ctx context.Context, seriesID string, includeCurrent bool) (*SeriesCollectionDTO, error) {
	seriesID = strings.TrimSpace(seriesID)
	if seriesID == "" {
		return nil, fmt.Errorf("series ID cannot be empty")
	}

	cacheKey := "series_seasons:" + seriesID
	if includeCurrent {
		cacheKey += ":all"
	}
	if collection, ok := s.cachedCollection(cacheKey); ok {
		return collection, nil
	}

	ctx, cancel := context.WithTimeout(ctx, seriesPageDetailBudget)
	defer cancel()

	album, err := retryAnonymous(s, ctx, func(c context.Context) (*model.Album, error) {
		return s.client.Seasons(c, seriesID)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch series seasons: %w", err)
	}

	var detailMap map[string]*model.Series
	if album != nil && len(album.Seasons) > 0 {
		ids := make([]string, 0, len(album.Seasons))
		for _, season := range album.Seasons {
			if season.SeriesID != "" {
				ids = append(ids, season.SeriesID)
			}
		}
		detailMap = s.fetchSeriesBatchMap(ctx, ids)
	}

	cards := s.seasonsToCards(album, detailMap)
	if !includeCurrent {
		cards = excludeSeriesCard(cards, seriesID)
	}
	collection := &SeriesCollectionDTO{Items: cards}
	if len(cards) > 0 {
		collection.Title = firstNonEmpty(albumTitle(album), "同系列短剧")
	}
	collection = sanitizeIPC(collection)
	s.cacheCollection(cacheKey, collection)
	return collection, nil
}

func nextPlayableSeason(seriesID string, collection *SeriesCollectionDTO) *SeriesCard {
	if collection == nil || seriesID == "" {
		return nil
	}

	currentIndex := -1
	for i, item := range collection.Items {
		if item.ID == seriesID {
			currentIndex = i
			break
		}
	}
	if currentIndex < 0 {
		return nil
	}

	for _, item := range collection.Items[currentIndex+1:] {
		if item.ID == "" || item.ID == seriesID || item.Upcoming || item.EpisodeCount <= 0 {
			continue
		}
		season := item
		return &season
	}
	return nil
}

func (s *HongguoService) GetSeriesAdaptations(ctx context.Context, seriesID, sourceID, sourceTitle, excludeIDs string) (*SeriesCollectionDTO, error) {
	seriesID = strings.TrimSpace(seriesID)
	sourceID = strings.TrimSpace(sourceID)
	sourceTitle = strings.TrimSpace(sourceTitle)
	if seriesID == "" {
		return nil, fmt.Errorf("series ID cannot be empty")
	}
	if sourceID == "" && sourceTitle == "" {
		return &SeriesCollectionDTO{}, nil
	}

	exclude := parseIDList(excludeIDs)
	exclude[seriesID] = true
	cacheKey := "series_adaptations:" + seriesID + ":" + sourceID + ":" + normalizeTitle(sourceTitle) + ":" + cacheIDList(exclude)
	if collection, ok := s.cachedCollection(cacheKey); ok {
		return collection, nil
	}

	ctx, cancel := context.WithTimeout(ctx, seriesPageDetailBudget)
	defer cancel()

	sourceData, err := retryAnonymous(s, ctx, func(c context.Context) (*sdk.Page[model.SeriesSummary], error) {
		return s.client.Search(c, sourceTitle, sdk.SearchOptions{PageSize: 30})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch adapted series: %w", err)
	}

	source := &SourceNovelDTO{ID: sourceID, Title: sourceTitle}
	cards := s.adaptationsForSource(ctx, seriesID, source, pageItems(sourceData), exclude)
	if sourceCard := sourceNovelCard(source); sourceCard != nil {
		cards = append([]SeriesCard{*sourceCard}, cards...)
	}
	collection := &SeriesCollectionDTO{Items: cards}
	if len(cards) > 0 {
		collection.Title = "同一剧本改编"
	}
	collection = sanitizeIPC(collection)
	s.cacheCollection(cacheKey, collection)
	return collection, nil
}

func (s *HongguoService) cachedCollection(cacheKey string) (*SeriesCollectionDTO, bool) {
	s.cacheMu.RLock()
	entry, ok := s.cache[cacheKey]
	s.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	collection, ok := entry.val.(*SeriesCollectionDTO)
	return collection, ok
}

func (s *HongguoService) cacheCollection(cacheKey string, collection *SeriesCollectionDTO) {
	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: collection, expiresAt: time.Now().Add(5 * time.Minute)}
	s.cacheMu.Unlock()
}

func albumTitle(album *model.Album) string {
	if album == nil {
		return ""
	}
	return firstNonEmpty(album.Title, "同系列短剧")
}

func parseIDList(value string) map[string]bool {
	ids := make(map[string]bool)
	for _, id := range strings.Split(value, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids[id] = true
		}
	}
	return ids
}

func cacheIDList(ids map[string]bool) string {
	values := make([]string, 0, len(ids))
	for id := range ids {
		values = append(values, id)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func (s *HongguoService) newSeriesDetailPage(detail model.SeriesDetail, seriesID string) *SeriesDetailPageDTO {
	d := detail.Series
	upcoming := isUpcomingSeries(d.EpisodeCount, d.RightText)
	statusStr := d.Status.String()
	if upcoming {
		statusStr = "upcoming"
	}
	page := &SeriesDetailPageDTO{
		SeriesDetailDTO: SeriesDetailDTO{
			ID:           sanitizeUTF8(d.ID),
			Title:        sanitizeUTF8(d.Title),
			Description:  sanitizeUTF8(d.Description),
			CoverURL:     s.media.ImageURL(d.CoverURL),
			EpisodeCount: d.EpisodeCount,
			Status:       statusStr,
			Heat:         formatHeatScore(d.Heat) + "热度",
			IsFavorite:   s.store.IsFavorite(seriesID),
		},
		Subtitles:   sanitizeUTF8Strings(d.Subtitles),
		ContentType: int(d.ContentType),
		PlayCount:   d.PlayCount,
		FollowCount: d.FollowCount,
		LikeCount:   d.DiggCount,
		StatusText:  seriesStatusText(d),
		Upcoming:    upcoming,
		Reserved:    s.store.HasReservation(seriesID),
	}
	if !d.CreatedAt.IsZero() && d.CreatedAt.Year() > 2000 {
		page.PublishText = d.CreatedAt.Format("2006-01-02")
	}

	var tags []string
	seenTags := make(map[string]bool)
	addTag := func(value string) {
		value = sanitizeUTF8(value)
		for _, piece := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == '/' || unicode.IsSpace(r)
		}) {
			if piece == "" || seenTags[piece] {
				continue
			}
			seenTags[piece] = true
			tags = append(tags, piece)
		}
	}

	var actorNames []string
	seenActors := make(map[string]bool)
	for _, row := range d.Rows {
		switch row.Kind {
		case model.InfoCategory, model.InfoTags, model.InfoPlotTag:
			addTag(row.Text)
		case model.InfoTwoActors:
			text := sanitizeUTF8(row.Text)
			if idx := strings.Index(text, "·"); idx >= 0 {
				text = text[idx+1:]
			}
			for _, name := range strings.FieldsFunc(text, func(r rune) bool {
				return r == ',' || r == '，' || r == '、' || r == '/' || unicode.IsSpace(r)
			}) {
				name = sanitizeUTF8(name)
				key := strings.ToLower(name)
				if key == "" || seenActors[key] {
					continue
				}
				seenActors[key] = true
				actorNames = append(actorNames, name)
			}
		}
	}
	for _, name := range actorNames {
		page.Actors = append(page.Actors, ActorInfo{Name: name, Role: "演员"})
	}
	if len(tags) == 0 && d.Category != "" {
		addTag(d.Category)
	}
	page.Tags = sanitizeUTF8Strings(tags)
	page.Category = sanitizeUTF8(firstNonEmpty(d.Category, "短剧"))

	if d.Novel != nil && d.Novel.BookID != "" && d.Novel.BookID != d.ID {
		page.Source = &SourceNovelDTO{
			ID:       d.Novel.BookID,
			Title:    sanitizeUTF8(firstNonEmpty(d.Novel.Title, d.Novel.ShortTitle)),
			Status:   d.Novel.Status.String(),
			Reads:    d.Novel.Reads,
			CoverURL: s.media.ImageURL(d.Novel.CoverURL),
			Tags:     sanitizeUTF8Strings(d.Novel.Tags),
		}
	}

	var episodes []EpisodeDTO
	var commentCount int64
	for _, ep := range detail.Episodes {
		episodes = append(episodes, EpisodeDTO{
			ID:           ep.ID,
			SeriesID:     seriesID,
			Number:       ep.Number,
			Title:        sanitizeUTF8(firstNonEmpty(ep.Title, fmt.Sprintf("第%d集", ep.Number))),
			Playable:     ep.Playable,
			PosterURL:    s.media.ImageURL(ep.CoverURL),
			CommentCount: ep.CommentCount,
		})
		commentCount += ep.CommentCount
	}
	page.Episodes = episodes
	page.CommentCount = commentCount
	return page
}

// Upstream rows occasionally contain malformed UTF-8. MyGo's strict JSON
// encoder rejects them, so drop invalid bytes before crossing the IPC
// boundary while preserving surrounding valid text.
func sanitizeUTF8(value string) string { return sanitize.UTF8(value) }

func sanitizeUTF8Strings(values []string) []string { return sanitize.UTF8Strings(values) }

func (s *HongguoService) fetchSeriesBatchMap(ctx context.Context, ids []string) map[string]*model.Series {
	if len(ids) == 0 {
		return nil
	}
	details, err := retryAnonymous(s, ctx, func(c context.Context) ([]model.SeriesDetail, error) {
		return s.client.GetSeriesBatch(c, ids)
	})
	if err != nil || len(details) == 0 {
		return nil
	}
	res := make(map[string]*model.Series, len(details))
	for _, d := range details {
		if d.Series != nil && d.Series.ID != "" {
			res[d.Series.ID] = d.Series
		}
	}
	return res
}

func (s *HongguoService) seriesCards(items []model.SeriesSummary) []SeriesCard {
	cards := make([]SeriesCard, 0, len(items))
	for _, it := range items {
		if it.ID == "" {
			continue
		}
		upcoming := it.EpisodeCount == 0
		heat := "热播"
		status := "finished"
		if upcoming {
			heat = "待上线"
			status = "upcoming"
		}
		cards = append(cards, SeriesCard{
			ID:           it.ID,
			Title:        it.Title,
			Description:  it.Description,
			CoverURL:     s.media.ImageURL(it.CoverURL),
			EpisodeCount: it.EpisodeCount,
			Category:     firstNonEmpty(it.Category, "短剧"),
			Author:       it.Author,
			Status:       status,
			Heat:         heat,
			Tags:         seriesSummaryTags(it),
			Upcoming:     upcoming,
			Reserved:     s.store.HasReservation(it.ID),
		})
	}
	return cards
}

func (s *HongguoService) seasonsToCards(album *model.Album, details ...map[string]*model.Series) []SeriesCard {
	if album == nil {
		return nil
	}

	var detailMap map[string]*model.Series
	if len(details) > 0 {
		detailMap = details[0]
	}

	cards := make([]SeriesCard, 0, len(album.Seasons))
	for _, season := range album.Seasons {
		if season.SeriesID == "" {
			continue
		}
		coverURL := season.CoverURL
		if coverURL == "" {
			coverURL = album.CoverURL
		}
		epCount := season.EpisodeCount
		status := season.Status
		title := firstNonEmpty(season.Title, album.Title)
		rightText := ""

		if detailMap != nil {
			if d, ok := detailMap[season.SeriesID]; ok && d != nil {
				if d.CoverURL != "" {
					coverURL = d.CoverURL
				}
				if d.Title != "" {
					title = d.Title
				}
				if d.EpisodeCount > 0 || epCount == 0 {
					epCount = d.EpisodeCount
				}
				status = d.Status
				rightText = d.RightText
			}
		}

		upcoming := isUpcomingSeries(epCount, rightText)

		var (
			heatVal   string
			statusStr string
		)

		if upcoming {
			heatVal = "待上线"
			statusStr = "upcoming"
		} else {
			switch status {
			case model.SeriesEnd:
				heatVal = "已完结"
				statusStr = "finished"
			case model.SeriesUpdating:
				if rightText != "" && !strings.HasPrefix(rightText, "全0") {
					heatVal = rightText
				} else {
					heatVal = "更新中"
				}
				statusStr = "updating"
			case model.SeriesUpdatedToday:
				heatVal = "今日更新"
				statusStr = "updating"
			case model.SeriesUpdateStopped:
				heatVal = "停止更新"
				statusStr = "finished"
			default:
				heatVal = "热播中"
				statusStr = "finished"
			}
		}

		cards = append(cards, SeriesCard{
			ID:           season.SeriesID,
			Title:        title,
			CoverURL:     s.media.ImageURL(coverURL),
			EpisodeCount: epCount,
			Category:     "",
			Status:       statusStr,
			Heat:         heatVal,
			Upcoming:     upcoming,
			Reserved:     s.store.HasReservation(season.SeriesID),
		})
	}
	return cards
}

func (s *HongguoService) adaptationsForSource(ctx context.Context, currentID string, source *SourceNovelDTO, candidates []model.SeriesSummary, excludeIDs map[string]bool) []SeriesCard {
	seasonIDs := make(map[string]bool, len(excludeIDs)+1)
	for id := range excludeIDs {
		seasonIDs[id] = true
	}
	seasonIDs[currentID] = true

	var ids []string
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.ID == "" || seasonIDs[candidate.ID] || seen[candidate.ID] {
			continue
		}
		if candidate.ID == source.ID {
			continue
		}
		seen[candidate.ID] = true
		ids = append(ids, candidate.ID)
	}
	if len(ids) == 0 {
		return nil
	}

	details, err := retryAnonymous(s, ctx, func(c context.Context) ([]model.SeriesDetail, error) {
		return s.client.GetSeriesBatch(c, ids)
	})
	if err != nil {
		return nil
	}

	out := make([]SeriesCard, 0, len(details))
	for _, detail := range details {
		if detail.Series == nil || detail.Series.ID == currentID {
			continue
		}
		if !isSameSourceNovel(detail.Series, source) {
			continue
		}

		d := detail.Series
		upcoming := isUpcomingSeries(d.EpisodeCount, d.RightText)

		var (
			heatVal   string
			statusStr string
		)

		if upcoming {
			heatVal = "待上线"
			statusStr = "upcoming"
		} else {
			statusStr = d.Status.String()
			if d.Heat > 0 {
				heatVal = formatHeatScore(d.Heat) + "热度"
			} else {
				switch d.Status {
				case model.SeriesEnd:
					heatVal = "已完结"
					statusStr = "finished"
				case model.SeriesUpdating:
					if d.RightText != "" && !strings.HasPrefix(d.RightText, "全0") {
						heatVal = d.RightText
					} else {
						heatVal = "更新中"
					}
					statusStr = "updating"
				default:
					heatVal = "热播"
				}
			}
		}

		out = append(out, SeriesCard{
			ID:           d.ID,
			Title:        d.Title,
			Description:  d.Description,
			CoverURL:     s.media.ImageURL(d.CoverURL),
			EpisodeCount: d.EpisodeCount,
			Category:     seriesContentTypeLabel(int(d.ContentType)),
			ContentType:  int(d.ContentType),
			Status:       statusStr,
			Heat:         heatVal,
			Upcoming:     upcoming,
			Reserved:     s.store.HasReservation(d.ID),
		})
		if len(out) >= 18 {
			break
		}
	}
	return out
}

func isSameSourceNovel(series *model.Series, source *SourceNovelDTO) bool {
	if series == nil || source == nil || source.ID == "" {
		return false
	}
	if series.Novel != nil && series.Novel.BookID == source.ID {
		return true
	}
	return series.RelatedNovelBookID == source.ID
}

func seriesContentTypeLabel(contentType int) string {
	switch model.ContentType(contentType) {
	case model.ContentMotionComic:
		return "漫剧"
	case model.ContentMovie:
		return "电影"
	case model.ContentTelePlay:
		return "电视剧"
	case model.ContentScenePlay:
		return "场景剧"
	default:
		return "短剧"
	}
}

func sourceNovelCard(source *SourceNovelDTO) *SeriesCard {
	if source == nil || source.ID == "" || source.Title == "" {
		return nil
	}

	badgeText := "小说"
	switch source.Status {
	case "finished":
		badgeText = "已完结"
	case "ongoing":
		badgeText = "连载中"
	case "updated today":
		badgeText = "今日更新"
	}

	heat := "原著剧本"
	if source.Reads > 0 {
		heat = formatHeatScore(source.Reads) + "阅读"
	}

	return &SeriesCard{
		ID:          source.ID,
		Title:       source.Title,
		Description: strings.Join(source.Tags, " · "),
		CoverURL:    source.CoverURL,
		Category:    "原著剧本",
		Kind:        "source_novel",
		BadgeText:   badgeText,
		Heat:        heat,
	}
}

func seriesSummaryTags(item model.SeriesSummary) []string {
	var tags []string
	if item.Category != "" {
		tags = append(tags, item.Category)
	}
	if item.EpisodeCount > 0 {
		tags = append(tags, fmt.Sprintf("%d集", item.EpisodeCount))
	}
	return tags
}

func isUpcomingRightText(text string) bool {
	if text == "" {
		return false
	}
	text = strings.TrimSpace(text)
	if text == "全0集" {
		return true
	}
	return strings.Contains(text, "待上线") ||
		strings.Contains(text, "即将上线") ||
		strings.Contains(text, "即将推出") ||
		strings.Contains(text, "火速预约") ||
		strings.Contains(text, "未上线") ||
		strings.Contains(text, "敬请期待")
}

func isUpcomingSeries(episodeCount int, rightText string) bool {
	return episodeCount == 0 || isUpcomingRightText(rightText)
}

func seriesStatusText(d *model.Series) string {
	if isUpcomingSeries(d.EpisodeCount, d.RightText) {
		return "待上线"
	}
	if d.RightText != "" && d.RightText != "全0集" {
		return d.RightText
	}
	return seriesStatusTextFromModel(d.Status)
}

func seriesStatusTextFromModel(status model.SeriesStatus) string {
	switch status {
	case model.SeriesUpdating:
		return "更新中"
	case model.SeriesEnd:
		return "已完结"
	case model.SeriesUpdatedToday:
		return "今日更新"
	case model.SeriesUpdateStopped:
		return "停止更新"
	default:
		return "热播中"
	}
}

func excludeSeriesCard(cards []SeriesCard, id string) []SeriesCard {
	out := make([]SeriesCard, 0, len(cards))
	for _, card := range cards {
		if card.ID != id {
			out = append(out, card)
		}
	}
	return out
}

func normalizeTitle(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("：", ":", "，", ",", "！", "!", "？", "?").Replace(value)
	return strings.ToLower(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
