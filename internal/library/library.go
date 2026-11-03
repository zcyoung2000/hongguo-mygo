package library

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/beilunyang/openhongguo/sdk/model"
	"io"
	"log"
	"myguo/internal/mediautil"
	"myguo/internal/sanitize"
	"myguo/internal/sign"
	"myguo/internal/upstream"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (s *Service) SaveProgress(ctx context.Context, seriesID, episodeID string, title, coverURL string, episodeNum, totalEpisodes int, positionSec, durationSec float64) error {
	seriesID = strings.TrimSpace(seriesID)
	if !mediautil.ValidContentID(seriesID) {
		return fmt.Errorf("历史 ID 无效")
	}
	s.historyMu.Lock()
	s.cachedHistory = nil
	s.historyCachedAt = time.Time{}
	s.historyGeneration++
	s.historyMu.Unlock()
	item := HistoryItem{
		SeriesID:      seriesID,
		Title:         title,
		CoverURL:      coverURL,
		EpisodeNum:    episodeNum,
		EpisodeTitle:  fmt.Sprintf("第%d集", episodeNum),
		PositionSec:   positionSec,
		DurationSec:   durationSec,
		TotalEpisodes: totalEpisodes,
		ContentType:   1004,
	}
	if err := s.store.SaveHistory(item); err != nil {
		return err
	}
	stored, ok := s.store.GetHistoryItem(seriesID)
	if !ok {
		stored = item
	}
	if err := s.store.QueueHistoryUpload(stored, episodeID); err != nil {
		return err
	}
	s.triggerPendingHistorySync()
	return nil
}

func (s *Service) GetHistory(ctx context.Context) ([]HistoryItem, error) {
	s.historyMu.Lock()

	// 1. If in-memory cache is fresh (< 30s), return instantly
	if len(s.cachedHistory) > 0 && time.Since(s.historyCachedAt) < 30*time.Second {
		cached := make([]HistoryItem, len(s.cachedHistory))
		copy(cached, s.cachedHistory)
		s.historyMu.Unlock()
		return sanitize.Sanitize(cached), nil
	}

	// 2. Check local store (sub-millisecond instant return)
	localItems, err := s.store.GetHistory()
	if err != nil {
		s.historyMu.Unlock()
		return nil, err
	}
	if len(localItems) > 0 {
		for i := range localItems {
			localItems[i].CoverURL = s.imageURL(localItems[i].CoverURL)
		}
		localItems = sanitize.Sanitize(localItems)
		s.cachedHistory = localItems
		shouldSync := s.historyCachedAt.IsZero() || time.Since(s.historyCachedAt) >= 60*time.Second
		s.historyCachedAt = time.Now()
		generation := s.historyGeneration

		// Stale-While-Revalidate: trigger async background sync without blocking user
		if shouldSync && !s.historySyncing {
			s.historySyncing = true
			go func() {
				defer func() {
					s.historyMu.Lock()
					s.historySyncing = false
					s.historyMu.Unlock()
				}()
				s.syncRemoteHistory(context.Background(), generation)
			}()
		}
		s.historyMu.Unlock()
		return sanitize.Sanitize(localItems), nil
	}
	s.historyMu.Unlock()

	// 3. Local store is empty (e.g. first app launch with an existing account) -> sync synchronously
	s.syncRemoteHistory(ctx, s.currentHistoryGeneration())

	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	list, err := s.store.GetHistory()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].CoverURL = s.imageURL(list[i].CoverURL)
	}
	list = sanitize.Sanitize(list)
	s.cachedHistory = list
	s.historyCachedAt = time.Now()
	return sanitize.Sanitize(list), nil
}

type remoteHistoryRow struct {
	SeriesID        string
	VidIndex        int
	EpisodeCount    int
	CurrentPlayTime int64
	LastWatchTime   int64
}

type remoteHistoryPage struct {
	Items      []remoteHistoryRow
	NextOffset int64
	HasMore    bool
}

func (s *Service) fetchRemoteHistoryPage(ctx context.Context, offset int64) (*remoteHistoryPage, error) {
	userCookies, userToken := s.credentials()
	if userCookies == "" {
		return nil, nil
	}
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := append(upstream.DefaultDeviceQueryPairs(deviceID, iid, cdid, openudid),
		[2]string{"book_types", "2"},
		[2]string{"limit", "50"},
	)
	if offset > 0 {
		pairs = append(pairs, [2]string{"offset", strconv.FormatInt(offset, 10)})
	}
	ticket := time.Now().UnixMilli()
	rawQuery, sigHeaders := sign.SignGET(pairs, ticket, deviceID, "7.3.9.32", nil)
	reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/bookapi/read_progress/list/v1/?" + rawQuery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range sigHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("User-Agent", s.userAgentValue())
	req.Header.Set("Cookie", userCookies)
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("x-tt-store-region", s.storeRegionValue())
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("历史同步接口返回 HTTP %d", resp.StatusCode)
	}

	var histResp struct {
		Data struct {
			VideoData  []remoteHistoryRow `json:"data_list"`
			NextOffset int64              `json:"next_offset"`
			HasMore    bool               `json:"has_more"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &histResp); err != nil {
		return nil, err
	}
	page := &remoteHistoryPage{
		Items:      histResp.Data.VideoData,
		NextOffset: histResp.Data.NextOffset,
		HasMore:    histResp.Data.HasMore,
	}
	if page.HasMore && page.NextOffset <= offset {
		page.NextOffset = offset + int64(len(page.Items))
	}
	return page, nil
}

func (s *Service) syncRemoteHistory(ctx context.Context, generation uint64) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var remoteRows []remoteHistoryRow
	offset := int64(0)
	for page := 0; page < 10; page++ {
		result, err := s.fetchRemoteHistoryPage(ctx, offset)
		if err != nil {
			log.Printf("[history] remote history page %d failed: %v", page, err)
			return
		}
		if result == nil {
			return
		}
		remoteRows = append(remoteRows, result.Items...)
		if !result.HasMore || len(result.Items) == 0 || result.NextOffset <= offset {
			break
		}
		offset = result.NextOffset
	}
	if len(remoteRows) == 0 {
		s.triggerPendingHistorySync()
		return
	}

	// The local history model stores one row per series, while the remote
	// protocol can return several watched episodes for the same series.
	latestByID := make(map[string]remoteHistoryRow)
	for _, row := range remoteRows {
		if row.SeriesID == "" {
			continue
		}
		current, ok := latestByID[row.SeriesID]
		if !ok || row.LastWatchTime >= current.LastWatchTime {
			latestByID[row.SeriesID] = row
		}
	}

	var missingIDs []string
	for id := range latestByID {
		if s.store.HistoryDeleted(id) {
			continue
		}
		if exist, ok := s.store.GetHistoryItem(id); ok && exist.Title != "" && exist.CoverURL != "" && exist.TotalEpisodes > 0 {
			continue
		}
		missingIDs = append(missingIDs, id)
	}

	detailsMap := make(map[string]model.SeriesDetail)
	if len(missingIDs) > 0 {
		for i := 0; i < len(missingIDs); i += 100 {
			end := i + 100
			if end > len(missingIDs) {
				end = len(missingIDs)
			}
			bCtx, bCancel := context.WithTimeout(ctx, 8*time.Second)
			batch, bErr := s.client.GetSeriesBatch(bCtx, missingIDs[i:end])
			bCancel()
			if bErr == nil {
				for _, b := range batch {
					if b.Series != nil && b.Series.ID != "" {
						detailsMap[b.Series.ID] = b
					}
				}
			}
		}
	}

	var histBatch []HistoryItem
	for _, it := range latestByID {
		id := it.SeriesID
		title := ""
		cover := ""
		epCount := 0
		intro := ""
		contentType := 1004
		durationSec := 0.0

		if detail, ok := detailsMap[id]; ok {
			if detail.Series.ContentType == 1 {
				_ = s.store.DeleteHistory(id)
				continue
			}
			contentType = int(detail.Series.ContentType)
			if detail.Series.Title != "" {
				title = detail.Series.Title
			}
			cover = detail.Series.CoverURL
			epCount = detail.Series.EpisodeCount
			intro = detail.Series.Description
		} else if exist, ok := s.store.GetHistoryItem(id); ok {
			if exist.ContentType == 1 {
				_ = s.store.DeleteHistory(id)
				continue
			}
			contentType = exist.ContentType
			title = exist.Title
			cover = exist.CoverURL
			epCount = exist.TotalEpisodes
			intro = exist.Intro
			durationSec = exist.DurationSec
		} else {
			// Never synthesize placeholder metadata or progress values.
			continue
		}

		epNum := it.VidIndex + 1
		if epNum <= 0 {
			epNum = 1
		}
		posSec := float64(it.CurrentPlayTime) / 1000
		updatedAt := it.LastWatchTime / 1000
		if updatedAt <= 0 {
			updatedAt = time.Now().Unix()
		}
		if exist, ok := s.store.GetHistoryItem(id); ok && exist.UpdatedAt >= updatedAt {
			continue
		}

		histBatch = append(histBatch, HistoryItem{
			SeriesID:      id,
			Title:         title,
			CoverURL:      mediautil.NormalizeImageURL(cover),
			EpisodeNum:    epNum,
			EpisodeTitle:  fmt.Sprintf("第%d集", epNum),
			PositionSec:   posSec,
			DurationSec:   durationSec,
			TotalEpisodes: epCount,
			UpdatedAt:     updatedAt,
			Intro:         intro,
			ContentType:   contentType,
		})
	}

	if len(histBatch) > 0 {
		if s.currentHistoryGeneration() != generation {
			return
		}
		if err := s.store.SaveHistoryBatch(histBatch); err != nil {
			log.Printf("[history] failed to persist synced history: %v", err)
			return
		}
	}

	s.historyMu.Lock()
	if s.historyGeneration != generation {
		s.historyMu.Unlock()
		return
	}
	list, err := s.store.GetHistory()
	if err != nil {
		s.historyMu.Unlock()
		return
	}
	for i := range list {
		list[i].CoverURL = s.imageURL(list[i].CoverURL)
	}
	s.cachedHistory = list
	s.historyCachedAt = time.Now()
	s.historyMu.Unlock()
	s.triggerPendingHistorySync()
}
func (s *Service) DeleteHistory(ctx context.Context, seriesID string) error {
	s.historyMu.Lock()
	s.cachedHistory = nil
	s.historyCachedAt = time.Time{}
	s.historyGeneration++
	s.historyMu.Unlock()
	if err := s.store.DeleteHistory(seriesID); err != nil {
		return err
	}
	s.triggerPendingHistorySync()
	return nil
}

func (s *Service) ClearHistory(ctx context.Context) error {
	s.historyMu.Lock()
	s.cachedHistory = nil
	s.historyCachedAt = time.Time{}
	s.historyGeneration++
	s.historyMu.Unlock()
	if err := s.store.ClearHistory(); err != nil {
		return err
	}
	s.triggerPendingHistorySync()
	return nil
}

func (s *Service) ToggleFavorite(ctx context.Context, seriesID, title, coverURL, desc string, totalEpisodes int) (bool, error) {
	seriesID = strings.TrimSpace(seriesID)
	if !mediautil.ValidContentID(seriesID) {
		return false, fmt.Errorf("收藏 ID 无效")
	}
	s.favMutationMu.Lock()
	defer s.favMutationMu.Unlock()
	wasAdded, err := s.store.ToggleFavorite(FavoriteItem{
		SeriesID:      seriesID,
		Title:         title,
		CoverURL:      coverURL,
		Description:   desc,
		TotalEpisodes: totalEpisodes,
	})
	if err != nil {
		return false, err
	}
	s.favMu.Lock()
	s.cachedFavorites = nil
	s.favCachedAt = time.Time{}
	s.favGeneration++
	s.favMu.Unlock()
	if err := s.store.QueueFavoriteOp(seriesID, wasAdded); err != nil {
		return wasAdded, err
	}
	s.triggerPendingFavoriteSync()
	return wasAdded, nil
}

func (s *Service) IsFavorite(ctx context.Context, seriesID string) (bool, error) {
	return s.store.IsFavorite(seriesID), nil
}

func (s *Service) GetFavorites(ctx context.Context) ([]FavoriteItem, error) {
	s.favMu.Lock()

	// 1. If in-memory cache is fresh (< 30s), return instantly
	if len(s.cachedFavorites) > 0 && time.Since(s.favCachedAt) < 30*time.Second {
		cached := make([]FavoriteItem, len(s.cachedFavorites))
		copy(cached, s.cachedFavorites)
		s.favMu.Unlock()
		return sanitize.Sanitize(cached), nil
	}

	userCookies, _ := s.credentials()

	// 2. Check local store (sub-millisecond instant return)
	localItems, err := s.store.GetFavorites()
	if err != nil {
		s.favMu.Unlock()
		return nil, err
	}
	if len(localItems) > 0 {
		for i := range localItems {
			localItems[i].CoverURL = s.imageURL(localItems[i].CoverURL)
		}
		localItems = sanitize.Sanitize(localItems)
		s.cachedFavorites = localItems
		shouldSync := s.favCachedAt.IsZero() || time.Since(s.favCachedAt) >= 60*time.Second
		s.favCachedAt = time.Now()
		generation := s.favGeneration

		// Stale-While-Revalidate: trigger async background sync without blocking user
		if userCookies != "" && shouldSync && !s.favSyncing {
			s.favSyncing = true
			go func() {
				defer func() {
					s.favMu.Lock()
					s.favSyncing = false
					s.favMu.Unlock()
				}()
				s.syncRemoteFavorites(context.Background(), generation)
			}()
		}
		s.favMu.Unlock()
		return sanitize.Sanitize(localItems), nil
	}
	s.favMu.Unlock()

	if userCookies == "" {
		s.favMu.Lock()
		s.cachedFavorites = localItems
		s.favCachedAt = time.Now()
		s.favMu.Unlock()
		return sanitize.Sanitize(localItems), nil
	}

	// 3. Local store empty -> synchronous fetch
	s.syncRemoteFavorites(ctx, s.currentFavoriteGeneration())

	s.favMu.Lock()
	defer s.favMu.Unlock()
	list, err := s.store.GetFavorites()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].CoverURL = s.imageURL(list[i].CoverURL)
	}
	list = sanitize.Sanitize(list)
	s.cachedFavorites = list
	s.favCachedAt = time.Now()
	return sanitize.Sanitize(list), nil
}

func (s *Service) syncRemoteFavorites(ctx context.Context, generation uint64) {
	userCookies, _ := s.credentials()
	if userCookies == "" {
		return
	}
	processed, err := s.syncPendingFavoriteOps()
	if err != nil {
		log.Printf("[favorites] pending ops blocked remote sync: %v", err)
		return
	}
	if processed > 0 {
		// The shelf list can briefly lag a successful mutation. Avoid replacing
		// local state with a stale snapshot; the next periodic sync will refresh.
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	userCookies, userToken := s.credentials()
	deviceID, iid, _, _ := s.deviceIdentity()
	q := url.Values{
		"aid":             {"8662"},
		"app_name":        {"novelread"},
		"version_code":    {"73932"},
		"device_platform": {"android"},
		"device_id":       {deviceID},
		"iid":             {iid},
	}
	reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/bookapi/bookshelf/video/list/v?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", s.userAgentValue())
	if userCookies != "" {
		req.Header.Set("Cookie", userCookies)
	}
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("x-tt-store-region", s.storeRegionValue())
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		return
	}

	var shelfResp struct {
		Data struct {
			VideoShelfInfo []struct {
				SeriesID   string `json:"series_id"`
				ModifyTime int64  `json:"modify_time"`
			} `json:"video_shelf_info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &shelfResp); err != nil {
		return
	}

	// The shelf response is authoritative only after every remote ID can be
	// represented locally. If metadata lookup is partial, merge what we have
	// and retry later rather than treating unresolved entries as deletions.
	remoteIDs := make(map[string]bool)
	seen := make(map[string]bool)
	for _, it := range shelfResp.Data.VideoShelfInfo {
		if it.SeriesID != "" && !seen[it.SeriesID] {
			seen[it.SeriesID] = true
			remoteIDs[it.SeriesID] = true
		}
	}
	if len(remoteIDs) == 0 {
		if s.currentFavoriteGeneration() == generation {
			if err := s.store.ReplaceFavoriteBatch(nil); err != nil {
				return
			}
		}
		s.refreshFavoriteCache(generation)
		return
	}

	var missingIDs []string
	for id := range remoteIDs {
		if exist, ok := s.store.GetFavoriteItem(id); ok && exist.Title != "" && exist.CoverURL != "" {
			continue
		}
		missingIDs = append(missingIDs, id)
	}

	detailsMap := make(map[string]model.SeriesDetail)
	if len(missingIDs) > 0 {
		for i := 0; i < len(missingIDs); i += 100 {
			end := i + 100
			if end > len(missingIDs) {
				end = len(missingIDs)
			}
			bCtx, bCancel := context.WithTimeout(ctx, 8*time.Second)
			batch, bErr := s.client.GetSeriesBatch(bCtx, missingIDs[i:end])
			bCancel()
			if bErr == nil {
				for _, b := range batch {
					if b.Series != nil && b.Series.ID != "" {
						detailsMap[b.Series.ID] = b
					}
				}
			}
		}
	}

	var favBatch []FavoriteItem
	resolvedIDs := make(map[string]bool)
	for _, it := range shelfResp.Data.VideoShelfInfo {
		if it.SeriesID == "" || resolvedIDs[it.SeriesID] {
			continue
		}
		modTime := time.Now().Unix()
		if it.ModifyTime > 0 {
			modTime = it.ModifyTime / 1000
		}

		if b, ok := detailsMap[it.SeriesID]; ok {
			favBatch = append(favBatch, FavoriteItem{
				SeriesID:      b.Series.ID,
				Title:         b.Series.Title,
				CoverURL:      mediautil.NormalizeImageURL(b.Series.CoverURL),
				Description:   b.Series.Description,
				TotalEpisodes: b.Series.EpisodeCount,
				AddedAt:       modTime,
			})
			resolvedIDs[it.SeriesID] = true
		} else if exist, ok := s.store.GetFavoriteItem(it.SeriesID); ok {
			favBatch = append(favBatch, FavoriteItem{
				SeriesID:      exist.SeriesID,
				Title:         exist.Title,
				CoverURL:      exist.CoverURL,
				Description:   exist.Description,
				TotalEpisodes: exist.TotalEpisodes,
				AddedAt:       modTime,
			})
			resolvedIDs[it.SeriesID] = true
		}
	}

	if len(favBatch) > 0 && s.currentFavoriteGeneration() == generation {
		if err := s.store.SaveFavoriteBatch(favBatch); err != nil {
			return
		}
	}
	if len(resolvedIDs) != len(remoteIDs) || s.currentFavoriteGeneration() != generation {
		return
	}
	if err := s.store.ReplaceFavoriteBatch(favBatch); err != nil {
		return
	}
	s.refreshFavoriteCache(generation)
}

func (s *Service) refreshFavoriteCache(generation uint64) {
	s.favMu.Lock()
	if s.favGeneration != generation {
		s.favMu.Unlock()
		return
	}
	list, err := s.store.GetFavorites()
	if err != nil {
		s.favMu.Unlock()
		return
	}
	for i := range list {
		list[i].CoverURL = s.imageURL(list[i].CoverURL)
	}
	s.cachedFavorites = list
	s.favCachedAt = time.Now()
	s.favMu.Unlock()
}
func (s *Service) ToggleLike(ctx context.Context, seriesID, episodeID, title, coverURL string, episodeNum int) (bool, error) {
	userCookies, _ := s.credentials()
	seriesID = strings.TrimSpace(seriesID)
	episodeID = strings.TrimSpace(episodeID)
	if !mediautil.ValidContentID(seriesID) || !mediautil.ValidContentID(episodeID) {
		return false, fmt.Errorf("点赞 ID 无效")
	}
	liked := !s.store.IsLiked(seriesID, episodeID, episodeNum)
	if userCookies != "" {
		if err := s.requestEpisodeLikeMutation(ctx, seriesID, episodeID, liked); err != nil {
			return false, err
		}
	}
	wasAdded, err := s.store.ToggleLike(LikeItem{
		SeriesID:   seriesID,
		EpisodeID:  episodeID,
		Title:      title,
		CoverURL:   coverURL,
		EpisodeNum: episodeNum,
	})
	if err == nil {
		s.likeMu.Lock()
		s.cachedLikes = nil
		s.likeCachedAt = time.Time{}
		s.likeGeneration++
		s.likeMu.Unlock()
	}
	return wasAdded, err
}

func (s *Service) GetLikes(ctx context.Context) ([]LikeItem, error) {
	userCookies, _ := s.credentials()
	s.likeMu.Lock()

	if len(s.cachedLikes) > 0 && time.Since(s.likeCachedAt) < 30*time.Second {
		cached := make([]LikeItem, len(s.cachedLikes))
		copy(cached, s.cachedLikes)
		s.likeMu.Unlock()
		return sanitize.Sanitize(cached), nil
	}

	localItems, err := s.store.GetLikes()
	if err != nil {
		s.likeMu.Unlock()
		return nil, err
	}
	if len(localItems) > 0 {
		for i := range localItems {
			localItems[i].CoverURL = s.imageURL(localItems[i].CoverURL)
		}
		localItems = sanitize.Sanitize(localItems)
		s.cachedLikes = localItems
		shouldSync := s.likeCachedAt.IsZero() || time.Since(s.likeCachedAt) >= 60*time.Second
		s.likeCachedAt = time.Now()
		generation := s.likeGeneration

		if userCookies != "" && shouldSync && !s.likeSyncing {
			s.likeSyncing = true
			go func() {
				defer func() {
					s.likeMu.Lock()
					s.likeSyncing = false
					s.likeMu.Unlock()
				}()
				_ = s.syncRemoteLikes(context.Background(), generation)
			}()
		}
		s.likeMu.Unlock()
		return sanitize.Sanitize(localItems), nil
	}
	s.likeMu.Unlock()

	if userCookies == "" {
		return []LikeItem{}, nil
	}
	if err := s.syncRemoteLikes(ctx, s.currentLikeGeneration()); err != nil {
		return nil, err
	}

	s.likeMu.Lock()
	defer s.likeMu.Unlock()
	list, err := s.store.GetLikes()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].CoverURL = s.imageURL(list[i].CoverURL)
	}
	list = sanitize.Sanitize(list)
	s.cachedLikes = list
	s.likeCachedAt = time.Now()
	return sanitize.Sanitize(list), nil
}

func (s *Service) syncRemoteLikes(ctx context.Context, generation uint64) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	items, err := s.fetchAccountLikes(ctx)
	if err != nil {
		return err
	}
	if s.currentLikeGeneration() != generation {
		return nil
	}
	if err := s.store.ReplaceLikeBatch(items); err != nil {
		return err
	}
	if s.currentLikeGeneration() != generation {
		return nil
	}

	s.likeMu.Lock()
	list, err := s.store.GetLikes()
	if err != nil {
		s.likeMu.Unlock()
		return err
	}
	for i := range list {
		list[i].CoverURL = s.imageURL(list[i].CoverURL)
	}
	s.cachedLikes = sanitize.Sanitize(list)
	s.likeCachedAt = time.Now()
	s.likeMu.Unlock()
	return nil
}

func (s *Service) fetchAccountLikes(ctx context.Context) ([]LikeItem, error) {
	userCookies, userToken := s.credentials()
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := append(upstream.DefaultDeviceQueryPairs(deviceID, iid, cdid, openudid),
		[2]string{"action_type", "3"},
		[2]string{"count", "100"},
		[2]string{"offset", "0"},
		[2]string{"object_type_list", "6,15,10"},
	)
	ticket := time.Now().UnixMilli()
	rawQuery, sigHeaders := sign.SignGET(pairs, ticket, deviceID, "7.3.9.32", nil)
	reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/ugc/action/mget/v1/?" + rawQuery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range sigHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("Cookie", userCookies)
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("User-Agent", s.userAgentValue())
	req.Header.Set("x-ss-req-ticket", strconv.FormatInt(ticket, 10))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("点赞接口返回 HTTP %d", resp.StatusCode)
	}

	var res struct {
		Code int `json:"code"`
		Data struct {
			MixedDataList []struct {
				SeriesID       string `json:"series_id"`
				BookID         string `json:"book_id"`
				SeriesTitle    string `json:"series_title"`
				SeriesCoverURL string `json:"series_cover_url"`
				SeriesIntro    string `json:"series_intro"`
				VideoData      struct {
					SeriesID    string `json:"series_id"`
					Vid         string `json:"vid"`
					Title       string `json:"title"`
					Cover       string `json:"cover"`
					CoverURL    string `json:"cover_url"`
					ThumbURL    string `json:"thumb_url"`
					Description string `json:"video_desc"`
					EpisodeCnt  int    `json:"episode_cnt"`
					DiggTime    int64  `json:"user_digg_timestamp_ms"`
					CreateTime  int64  `json:"create_time"`
					Detail      struct {
						SeriesTitle string `json:"series_title"`
					} `json:"video_detail"`
				} `json:"video_data"`
			} `json:"mixed_data_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, fmt.Errorf("点赞接口返回 code %d", res.Code)
	}

	items := make([]LikeItem, 0, len(res.Data.MixedDataList))
	seen := make(map[string]bool)
	for _, row := range res.Data.MixedDataList {
		video := row.VideoData
		seriesID := firstNonEmpty(row.SeriesID, video.SeriesID, row.BookID)
		episodeID := firstNonEmpty(video.Vid, row.SeriesID, row.BookID)
		title := firstNonEmpty(row.SeriesTitle, video.Detail.SeriesTitle, video.Title)
		cover := firstNonEmpty(row.SeriesCoverURL, video.CoverURL, video.Cover, video.ThumbURL)
		addedAt := video.DiggTime / 1000
		if addedAt <= 0 {
			addedAt = video.CreateTime / 1000
		}
		if seriesID == "" || episodeID == "" || title == "" || seen[seriesID+"\x1f"+episodeID] {
			continue
		}
		seen[seriesID+"\x1f"+episodeID] = true
		items = append(items, LikeItem{
			SeriesID:      seriesID,
			EpisodeID:     episodeID,
			Title:         title,
			CoverURL:      mediautil.NormalizeImageURL(cover),
			EpisodeNum:    0,
			TotalEpisodes: video.EpisodeCnt,
			Description:   firstNonEmpty(row.SeriesIntro, video.Description),
			AddedAt:       addedAt,
		})
	}
	return items, nil
}

func (s *Service) requestEpisodeLikeMutation(ctx context.Context, seriesID, episodeID string, liked bool) error {
	userCookies, userToken := s.credentials()
	actionType := 3
	if !liked {
		actionType = 4
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	raw, err := json.Marshal(map[string]any{
		"action_category":      1,
		"action_reason_remark": "like_click",
		"action_type":          actionType,
		"business_param": map[string]any{
			"book_id":          0,
			"has_aigc_content": false,
			"modify_count":     0,
			"video_id":         seriesID,
			"shark_param": map[string]any{
				"enter_from":    "MainFragmentActivity",
				"page_list":     "MainFragmentActivity",
				"previous_page": "",
			},
		},
		"object_id":   episodeID,
		"object_type": 6,
	})
	if err != nil {
		return err
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	path := "/novel/articleapi/do_action/v1/"
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := upstream.DefaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
	ticket := time.Now().UnixMilli()
	rawQuery, sigHeaders := sign.SignPOST(path, pairs, ticket, deviceID, "7.3.9.32", compressed.Bytes(), nil)
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://api5-normal-lq.fqnovel.com"+path+"?"+rawQuery,
		bytes.NewReader(compressed.Bytes()),
	)
	if err != nil {
		return err
	}
	for key, value := range sigHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-SS-DP", "8662")
	req.Header.Set("Lc", "101")
	req.Header.Set("X-Reading-Request", fmt.Sprintf("%d-%d", ticket, binary.LittleEndian.Uint32(nonce[:])))
	req.Header.Set("Cookie", userCookies)
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("User-Agent", s.userAgentValue())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("点赞接口返回 HTTP %d", resp.StatusCode)
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("点赞响应解析失败: %w", err)
	}
	if result.Code != 0 {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "unknown error"
		}
		return fmt.Errorf("点赞失败: %s", message)
	}
	return nil
}

func (s *Service) ToggleReservation(ctx context.Context, seriesID, title, coverURL, note, publishAt string) (bool, error) {
	userCookies, _ := s.credentials()
	seriesID = strings.TrimSpace(seriesID)
	if !mediautil.ValidContentID(seriesID) {
		return false, fmt.Errorf("预约 ID 无效")
	}
	if userCookies == "" {
		return false, fmt.Errorf("请先登录后预约")
	}

	// The reservation list is the source of truth; mutations are confirmed by
	// reading the account list back before returning.
	s.invalidateReservationCaches()
	s.resMu.Lock()
	s.cachedReservations = nil
	s.resCachedAt = time.Time{}
	s.resGeneration++
	generation := s.resGeneration
	s.resMu.Unlock()
	wasReserved := s.store.HasReservation(seriesID)
	opType := 1
	if wasReserved {
		opType = 2
	}
	if err := s.requestReservationMutation(ctx, seriesID, opType); err != nil {
		return false, err
	}
	if !wasReserved {
		s.notify(ctx, Notification{
			Title:    "新剧预约成功",
			Body:     fmt.Sprintf("《%s》已加入预约列表，上线后将第一时间通知您！", title),
			SeriesID: seriesID,
		})
	}
	if _, err := s.syncRemoteReservations(ctx, generation); err != nil {
		// The write succeeded even when confirmation is slow or unavailable.
		return !wasReserved, nil
	}
	return !wasReserved, nil
}

func (s *Service) invalidateReservationCaches() {
	s.invalidateCaches("series_page:", "new_releases:")
}

func (s *Service) requestReservationMutation(ctx context.Context, seriesID string, opType int) error {
	userCookies, userToken := s.credentials()
	itemID, err := strconv.ParseInt(seriesID, 10, 64)
	if err != nil || itemID <= 0 {
		return fmt.Errorf("预约编号无效")
	}

	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	raw, err := json.Marshal(map[string]any{
		"item_id":   itemID,
		"item_type": 1,
		"op_type":   opType,
		"shark_param": map[string]any{
			"enter_from":    "BulletActivity",
			"previous_page": "MainFragmentActivity",
			"page_list":     "MainFragmentActivity,BulletActivity",
		},
		"wish_list_all_del": 0,
	})
	if err != nil {
		return err
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	path := "/reading/bookapi/search/uncover_subscribe/v"
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := upstream.DefaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
	ticket := time.Now().UnixMilli()
	rawQuery, headers := sign.SignPOST(path, pairs, ticket, deviceID, "7.3.9.32", compressed.Bytes(), nil)
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api5-normal-lq.fqnovel.com"+path+"?"+rawQuery,
		bytes.NewReader(compressed.Bytes()),
	)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-SS-DP", "8662")
	req.Header.Set("Lc", "101")
	req.Header.Set("X-Reading-Request", fmt.Sprintf("%d-%d", time.Now().UnixMilli(), binary.LittleEndian.Uint32(nonce[:])))
	req.Header.Set("Cookie", userCookies)
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("User-Agent", s.userAgentValue())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reservation API returned %d", resp.StatusCode)
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("reservation API returned invalid JSON: %w", err)
	}
	if result.Code != 0 {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "unknown error"
		}
		return fmt.Errorf("reservation API code %d: %s", result.Code, message)
	}
	return nil
}

func (s *Service) GetReservations(ctx context.Context) ([]ReservationItem, error) {
	s.resMu.Lock()

	// 1. If in-memory cache is fresh (< 60s), return instantly
	if len(s.cachedReservations) > 0 && time.Since(s.resCachedAt) < 60*time.Second {
		cached := make([]ReservationItem, len(s.cachedReservations))
		copy(cached, s.cachedReservations)
		s.resMu.Unlock()
		return sanitize.Sanitize(cached), nil
	}

	// 2. Check local store (sub-millisecond instant return)
	storeList, err := s.store.GetReservations()
	if err != nil {
		s.resMu.Unlock()
		return nil, err
	}
	for i := range storeList {
		storeList[i].CoverURL = s.imageURL(storeList[i].CoverURL)
	}
	storeList = sanitize.Sanitize(storeList)

	userCookies, _ := s.credentials()
	if userCookies == "" {
		s.cachedReservations = storeList
		s.resCachedAt = time.Now()
		s.resMu.Unlock()
		return sanitize.Sanitize(storeList), nil
	}

	if len(storeList) > 0 {
		s.cachedReservations = storeList
		shouldSync := s.resCachedAt.IsZero() || time.Since(s.resCachedAt) >= 60*time.Second
		s.resCachedAt = time.Now()
		generation := s.resGeneration
		// Stale-While-Revalidate: trigger async background sync without blocking user
		if shouldSync && !s.resSyncing {
			s.resSyncing = true
			go func() {
				defer func() {
					s.resMu.Lock()
					s.resSyncing = false
					s.resMu.Unlock()
				}()
				_, _ = s.syncRemoteReservations(context.Background(), generation)
			}()
		}
		s.resMu.Unlock()
		return sanitize.Sanitize(storeList), nil
	}
	s.resMu.Unlock()

	// 3. Local store empty -> synchronous fetch
	results, err := s.syncRemoteReservations(ctx, s.currentReservationGeneration())
	return sanitize.Sanitize(results), err
}

func (s *Service) syncRemoteReservations(ctx context.Context, generation uint64) ([]ReservationItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	var (
		online  []ReservationItem
		offline []ReservationItem
		onErr   error
		offErr  error
		wg      sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		online, onErr = s.fetchAccountReservations(ctx, true)
	}()
	go func() {
		defer wg.Done()
		offline, offErr = s.fetchAccountReservations(ctx, false)
	}()
	wg.Wait()

	if s.currentReservationGeneration() != generation {
		return nil, nil
	}
	if onErr != nil && offErr != nil {
		return nil, onErr
	}

	prevReservations, prevErr := s.store.GetReservations()
	if prevErr != nil {
		return nil, prevErr
	}
	prevOnline := make(map[string]bool)
	for _, r := range prevReservations {
		if r.IsOnline {
			prevOnline[r.SeriesID] = true
		}
	}

	for _, it := range online {
		if !prevOnline[it.SeriesID] && len(prevReservations) > 0 {
			s.notify(ctx, Notification{
				Title:    "新剧开播提醒",
				Body:     fmt.Sprintf("您预约的《%s》已正式上线，点击立即观看！", it.Title),
				SeriesID: it.SeriesID,
			})
		}
	}

	results := append(online, offline...)
	if s.currentReservationGeneration() != generation {
		return nil, nil
	}
	_ = s.store.ReplaceReservationBatch(results)

	s.resMu.Lock()
	if s.resGeneration != generation {
		s.resMu.Unlock()
		return nil, nil
	}
	storeList, storeErr := s.store.GetReservations()
	if storeErr != nil {
		s.resMu.Unlock()
		return nil, storeErr
	}
	for i := range storeList {
		storeList[i].CoverURL = s.imageURL(storeList[i].CoverURL)
	}
	storeList = sanitize.Sanitize(storeList)
	s.cachedReservations = storeList
	s.resCachedAt = time.Now()
	s.resMu.Unlock()

	return sanitize.Sanitize(storeList), nil
}

func (s *Service) fetchAccountReservations(ctx context.Context, online bool) ([]ReservationItem, error) {
	userCookies, userToken := s.credentials()
	var results []ReservationItem
	offset := 0

	for {
		deviceID, iid, cdid, openudid := s.deviceIdentity()
		pairs := append(upstream.DefaultDeviceQueryPairs(deviceID, iid, cdid, openudid),
			[2]string{"order_experiment", "descend"},
			[2]string{"offset", strconv.Itoa(offset)},
			// tab_type=13 is the account reservation tab; tab_type=5 is the public
			// release calendar that shares this endpoint.
			[2]string{"tab_type", "13"},
			[2]string{"is_online", strconv.FormatBool(online)},
		)

		ticket := time.Now().UnixMilli()
		rawQuery, sigHeaders := sign.SignGET(pairs, ticket, deviceID, "7.3.9.32", nil)
		reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/user/subscribe/list/v1/?" + rawQuery
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range sigHeaders {
			req.Header.Set(k, v)
		}
		req.Header.Set("Cookie", userCookies)
		if userToken != "" {
			req.Header.Set("x-tt-token", userToken)
		}
		req.Header.Set("User-Agent", s.userAgentValue())
		req.Header.Set("x-ss-req-ticket", strconv.FormatInt(ticket, 10))

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("reservation API returned %d", resp.StatusCode)
		}

		var res struct {
			Code int `json:"code"`
			Data struct {
				HasMore    bool `json:"has_more"`
				NextOffset int  `json:"next_offset"`
				Items      []struct {
					ItemID       string `json:"item_id"`
					Name         string `json:"name"`
					SubInfo      string `json:"sub_info"`
					Cover        string `json:"cover"`
					IsOnline     bool   `json:"is_online"`
					Expected     string `json:"expected_rec_text"`
					ScheduleTime int64  `json:"schedule_publish_time"`
					PlannedTime  int64  `json:"expected_publish_time"`
					Row          *struct {
						ID    string `json:"series_id"`
						IDStr string `json:"series_id_str"`
						Title string `json:"title"`
						Cover string `json:"cover"`
					} `json:"subscribe_data"`
				} `json:"subscribe_items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}
		if res.Code != 0 {
			return nil, fmt.Errorf("reservation API code %d", res.Code)
		}

		for _, it := range res.Data.Items {
			id := it.ItemID
			title := it.Name
			cover := it.Cover
			if it.Row != nil {
				// Reservation item_id is the scheduled notification, while
				// subscribe_data.series_id is the playable series after release.
				if it.Row.IDStr != "" {
					id = it.Row.IDStr
				} else if it.Row.ID != "" {
					id = it.Row.ID
				}
				if it.Row.Title != "" {
					title = it.Row.Title
				}
				if it.Row.Cover != "" {
					cover = it.Row.Cover
				}
			}
			if id == "" || id == "0" {
				continue
			}

			note := it.SubInfo
			if note == "" {
				note = it.Expected
			}
			if it.IsOnline {
				if note == "" {
					note = "已上线"
				}
			} else if note == "" {
				note = "预约中"
			}

			publishTime := it.ScheduleTime
			if publishTime == 0 {
				publishTime = it.PlannedTime
			}
			var publishAt string
			if publishTime > 0 {
				publishAt = time.Unix(publishTime, 0).Format(time.RFC3339)
			}

			results = append(results, ReservationItem{
				SeriesID:  id,
				Title:     title,
				CoverURL:  s.imageURL(cover),
				Note:      note,
				PublishAt: publishAt,
				IsOnline:  it.IsOnline,
				AddedAt:   time.Now().Unix(),
			})
		}

		if !res.Data.HasMore || res.Data.NextOffset <= offset {
			return results, nil
		}
		offset = res.Data.NextOffset
	}
}
