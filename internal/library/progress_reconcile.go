package library

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"myguo/internal/mediautil"
	"myguo/internal/sign"
	"myguo/internal/upstream"
)

const seriesProgressTimeout = 8 * time.Second

func (s *Service) ReconcileSeriesProgress(ctx context.Context, seriesID string) (HistoryItem, error) {
	seriesID = strings.TrimSpace(seriesID)
	local, hasLocal := s.store.GetHistoryItem(seriesID)
	if !mediautil.ValidContentID(seriesID) {
		return local, fmt.Errorf("历史 ID 无效")
	}
	remote, err := s.fetchSeriesProgress(ctx, seriesID)
	if err != nil || remote == nil {
		return local, err
	}
	merged := mergeProgress(local, *remote)
	if hasLocal {
		merged.Title = local.Title
		merged.CoverURL = local.CoverURL
		merged.Intro = local.Intro
		merged.ContentType = local.ContentType
		if merged.DurationSec <= 0 || merged.EpisodeNum == local.EpisodeNum {
			merged.DurationSec = local.DurationSec
		}
	}
	if !hasLocal {
		merged = enrichProgress(merged, HistoryItem{})
	}
	if !hasLocal || !historyItemsEqual(local, merged) {
		if err := s.store.SaveHistory(merged, s.currentSyncSession().scope); err != nil {
			return local, err
		}
		s.historyMu.Lock()
		s.cachedHistory = nil
		s.historyCachedAt = time.Time{}
		s.historyGeneration++
		s.historyMu.Unlock()
	}
	return merged, nil
}

func historyItemsEqual(left, right HistoryItem) bool {
	return left.SeriesID == right.SeriesID &&
		left.EpisodeNum == right.EpisodeNum &&
		left.PositionSec == right.PositionSec &&
		left.DurationSec == right.DurationSec &&
		left.UpdatedAt == right.UpdatedAt
}

func mergeProgress(local, remote HistoryItem) HistoryItem {
	if local.SeriesID == "" {
		return remote
	}
	if remote.SeriesID == "" {
		return local
	}
	if local.UpdatedAt != remote.UpdatedAt {
		if local.UpdatedAt > remote.UpdatedAt {
			return enrichProgress(local, remote)
		}
		return enrichProgress(remote, local)
	}
	result := local
	if remote.EpisodeNum > local.EpisodeNum {
		result = remote
	} else if remote.EpisodeNum == local.EpisodeNum && remote.PositionSec > local.PositionSec {
		result = remote
	}
	return enrichProgress(result, olderProgress(local, remote))
}

func olderProgress(local, remote HistoryItem) HistoryItem {
	if local.UpdatedAt <= remote.UpdatedAt {
		return local
	}
	return remote
}

func enrichProgress(newer, older HistoryItem) HistoryItem {
	if newer.Title == "" {
		newer.Title = older.Title
	}
	if newer.CoverURL == "" {
		newer.CoverURL = older.CoverURL
	}
	if newer.TotalEpisodes <= 0 {
		newer.TotalEpisodes = older.TotalEpisodes
	}
	if newer.DurationSec <= 0 && newer.EpisodeNum == older.EpisodeNum {
		newer.DurationSec = older.DurationSec
	}
	if newer.Intro == "" {
		newer.Intro = older.Intro
	}
	if newer.ContentType == 0 {
		newer.ContentType = older.ContentType
	}
	return newer
}

func (s *Service) fetchSeriesProgress(ctx context.Context, seriesID string) (*HistoryItem, error) {
	ctx = s.bindSyncSession(ctx)
	session, err := s.sessionForRequest(ctx)
	if err != nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, seriesProgressTimeout)
	defer cancel()
	deviceID, installID, cdid, openudid := s.deviceIdentity()
	pairs := append(upstream.DefaultDeviceQueryPairs(deviceID, installID, cdid, openudid),
		[2]string{"book_types", "2"},
		[2]string{"series_ids", seriesID},
	)
	rawQuery, headers := sign.SignGET(pairs, time.Now().UnixMilli(), deviceID, "7.3.9.32", nil)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api5-normal-sinfonlinea.fqnovel.com/reading/bookapi/read_progress/list/v1/?"+rawQuery, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("User-Agent", s.userAgentValue())
	request.Header.Set("Cookie", session.cookies)
	if session.token != "" {
		request.Header.Set("x-tt-token", session.token)
	}
	request.Header.Set("x-tt-store-region", s.storeRegionValue())
	request.Header.Set("Accept", "application/json")
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("进度对账接口返回 HTTP %d", response.StatusCode)
	}
	var payload struct {
		Code int `json:"code"`
		Data *struct {
			Rows []remoteHistoryRow `json:"data_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Code != 0 || payload.Data == nil || payload.Data.Rows == nil {
		return nil, fmt.Errorf("进度对账响应无效 (code=%d)", payload.Code)
	}
	for _, row := range payload.Data.Rows {
		if row.SeriesID != seriesID {
			continue
		}
		item := HistoryItem{
			SeriesID:      row.SeriesID,
			Title:         "未知短剧",
			EpisodeNum:    row.VidIndex + 1,
			EpisodeTitle:  fmt.Sprintf("第%d集", row.VidIndex+1),
			PositionSec:   float64(row.CurrentPlayTime) / 1000,
			TotalEpisodes: row.EpisodeCount,
			UpdatedAt:     remoteProgressUpdatedAt(row),
			ContentType:   1004,
		}
		if item.EpisodeNum <= 0 {
			item.EpisodeNum = 1
			item.EpisodeTitle = "第1集"
		}
		return &item, nil
	}
	return nil, nil
}

func remoteProgressUpdatedAt(row remoteHistoryRow) int64 {
	if row.LastWatchTime <= 0 {
		return time.Now().Unix()
	}
	updatedAt := row.LastWatchTime / 1000
	if updatedAt <= 0 {
		return row.LastWatchTime
	}
	return updatedAt
}
