package ipc

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"myguo/internal/sign"
)

const (
	favoriteBookType = 2
	userSyncPageSize = 20
	userSyncMaxBody  = 1 << 20
)

func contentInt(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("内容 ID 无效")
	}
	return parsed, nil
}

func (s *HongguoService) postSignedMutation(ctx context.Context, operation, path string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s 编码失败: %w", operation, err)
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		return fmt.Errorf("%s 压缩失败: %w", operation, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("%s 压缩失败: %w", operation, err)
	}

	userCookies, userToken := s.authCredentials()
	deviceID, installID, cdid, openudid := s.deviceIdentity()
	pairs := defaultDeviceQueryPairs(deviceID, installID, cdid, openudid)
	ticket := time.Now().UnixMilli()
	rawQuery, headers := sign.SignPOST(path, pairs, ticket, deviceID, "7.3.9.32", compressed.Bytes(), nil)
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
	for key, value := range headers {
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
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s请求失败: %w", operation, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, userSyncMaxBody))
	if err != nil {
		return fmt.Errorf("%s响应读取失败: %w", operation, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s接口返回 HTTP %d", operation, resp.StatusCode)
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("%s接口返回无效 JSON: %w", operation, err)
	}
	if result.Code != 0 {
		message := result.Message
		if message == "" {
			message = "unknown error"
		}
		return fmt.Errorf("%s接口返回 code %d: %s", operation, result.Code, message)
	}
	return nil
}

func historyUpdatePayload(item HistoryItem, episodeID string) map[string]any {
	bookID, err := contentInt(item.SeriesID)
	if err != nil {
		return nil
	}
	episodeNum, err := contentInt(episodeID)
	if err != nil {
		return nil
	}
	index := item.EpisodeNum - 1
	if index < 0 {
		index = 0
	}
	now := time.Now().UnixMilli()
	positionMS := int64(item.PositionSec * 1000)
	inner := map[string]any{
		"book_id":                      bookID,
		"book_type":                    favoriteBookType,
		"vid":                          episodeNum,
		"vid_index":                    index,
		"chapter_index":                index,
		"current_play_position":        positionMS,
		"player_accumulate_total_time": positionMS,
		"read_timestamp_ms":            now,
		"update_timestamp_ms":          now,
		"is_delete":                    false,
		"is_interactive_game":          false,
		"is_listen_mode":               false,
		"is_multi_season":              false,
		"season_index":                 0,
		"digged_count":                 0,
		"user_digg":                    false,
		"duration":                     int64(item.DurationSec),
		"episode_cnt":                  item.TotalEpisodes,
		"origin_novel_book_id":         0,
		"recent_reads":                 0,
		"retain_video_play_time":       0,
		"series_play_cnt":              0,
		"tone_id":                      0,
		"use_soft_delete":              false,
		"meet_guide_comment_tag":       false,
		"user_playlet_comment_flag":    false,
	}
	return map[string]any{"update_datas": []any{inner}}
}

func historyUpdateBatchPayload(items []HistoryItem, episodeIDs []string) map[string]any {
	if len(items) != len(episodeIDs) {
		return nil
	}
	updates := make([]any, 0, len(items))
	for i, item := range items {
		payload := historyUpdatePayload(item, episodeIDs[i])
		if payload == nil {
			return nil
		}
		updates = append(updates, payload["update_datas"].([]any)[0])
	}
	return map[string]any{"update_datas": updates}
}

func progressUploadPayload(item HistoryItem, episodeID string) map[string]any {
	if _, err := contentInt(item.SeriesID); err != nil {
		return nil
	}
	if _, err := contentInt(episodeID); err != nil {
		return nil
	}
	index := item.EpisodeNum - 1
	if index < 0 {
		index = 0
	}
	now := time.Now().UnixMilli()
	positionMS := int64(item.PositionSec * 1000)
	inner := map[string]any{
		"book_id":                          item.SeriesID,
		"book_type":                        favoriteBookType,
		"item_id":                          episodeID,
		"vid_index":                        index,
		"current_play_time":                positionMS,
		"player_cumulative_total_duration": positionMS,
		"read_timestamp_ms":                now,
		"progress_type":                    positionMS,
		"channel_id":                       0,
		"cur_channel_id":                   0,
		"check_timestamp":                  now,
		"is_listen_mode":                   false,
		"is_local_book":                    false,
		"listen_and_read":                  false,
		"page_index":                       0,
		"page_progress_rate":               0,
		"paragraph_offset":                 0,
		"tone_id":                          0,
	}
	return map[string]any{"books": []any{inner}}
}

func progressUploadBatchPayload(items []HistoryItem, episodeIDs []string) map[string]any {
	if len(items) != len(episodeIDs) {
		return nil
	}
	books := make([]any, 0, len(items))
	for i, item := range items {
		payload := progressUploadPayload(item, episodeIDs[i])
		if payload == nil {
			return nil
		}
		books = append(books, payload["books"].([]any)[0])
	}
	return map[string]any{"books": books}
}

func favoriteMutationPayload(seriesID string, added bool) map[string]any {
	if _, err := contentInt(seriesID); err != nil {
		return nil
	}
	return map[string]any{
		"is_cancelled": false,
		"shark_param": map[string]any{
			"enter_from":         "MainFragmentActivity",
			"previous_page":      "",
			"page_list":          "MainFragmentActivity",
			"is_active_behavior": "true",
			"inactive_type":      "0",
		},
		"book_id":     seriesID,
		"book_type":   favoriteBookType,
		"modify_time": time.Now().UnixMilli(),
		"update_bookshelf_video_list": []any{map[string]any{
			"is_cancelled": !added,
		}},
	}
}

func historyDeletePayload(items []HistoryItem, episodeIDs []string) map[string]any {
	if len(items) != len(episodeIDs) {
		return nil
	}
	updates := make([]any, 0, len(items))
	now := time.Now().UnixMilli()
	for i, item := range items {
		bookID, err := contentInt(item.SeriesID)
		if err != nil {
			continue
		}
		episodeID, err := contentInt(episodeIDs[i])
		if err != nil {
			continue
		}
		index := item.EpisodeNum - 1
		if index < 0 {
			index = 0
		}
		updates = append(updates, map[string]any{
			"book_id":                      bookID,
			"book_type":                    favoriteBookType,
			"vid":                          episodeID,
			"vid_index":                    index,
			"is_delete":                    true,
			"use_soft_delete":              true,
			"update_timestamp_ms":          now,
			"chapter_index":                index,
			"current_play_position":        0,
			"player_accumulate_total_time": 0,
			"read_timestamp_ms":            now,
			"is_interactive_game":          false,
			"is_listen_mode":               false,
			"is_multi_season":              false,
			"season_index":                 0,
			"digged_count":                 0,
			"user_digg":                    false,
			"duration":                     0,
			"episode_cnt":                  item.TotalEpisodes,
			"origin_novel_book_id":         0,
			"recent_reads":                 0,
			"retain_video_play_time":       0,
			"series_play_cnt":              0,
			"tone_id":                      0,
			"meet_guide_comment_tag":       false,
			"user_playlet_comment_flag":    false,
		})
	}
	return map[string]any{"update_datas": updates}
}

func (s *HongguoService) resolveHistoryEpisodeID(item HistoryItem) string {
	if cached, ok := s.store.GetCachedEpisodeAt(item.SeriesID, "", item.EpisodeNum); ok {
		return cached.EpisodeID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	episodes, err := s.listEpisodes(ctx, item.SeriesID)
	if err != nil {
		return ""
	}
	for _, episode := range pageItems(episodes) {
		if episode.Number == item.EpisodeNum {
			return episode.ID
		}
	}
	return ""
}

func (s *HongguoService) requestHistoryDelete(items []HistoryItem, episodeIDs []string) error {
	userCookies, _ := s.authCredentials()
	if userCookies == "" || len(items) == 0 || len(items) != len(episodeIDs) {
		return nil
	}
	payload := historyDeletePayload(items, episodeIDs)
	updates, ok := payload["update_datas"].([]any)
	if !ok || len(updates) == 0 {
		return fmt.Errorf("历史删除缺少有效分集引用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.postSignedMutation(ctx, "历史删除", "/reading/bookapi/read_history/update/v", payload)
}

func (s *HongguoService) reportPlaybackProgress(items []HistoryItem, episodeIDs []string) error {
	userCookies, _ := s.authCredentials()
	if userCookies == "" || len(items) == 0 || len(items) != len(episodeIDs) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	historyPayload := historyUpdateBatchPayload(items, episodeIDs)
	if historyPayload == nil {
		return fmt.Errorf("历史上报缺少有效内容 ID")
	}
	if err := s.postSignedMutation(ctx, "历史上报", "/reading/bookapi/read_history/update/v", historyPayload); err != nil {
		return err
	}
	progressPayload := progressUploadBatchPayload(items, episodeIDs)
	if progressPayload == nil {
		return fmt.Errorf("进度上报缺少有效内容 ID")
	}
	return s.postSignedMutation(ctx, "进度上报", "/reading/bookapi/read_progress/upload/v", progressPayload)
}

func (s *HongguoService) triggerPendingHistorySync() {
	go s.syncPendingHistoryMutations()
}

func (s *HongguoService) syncPendingHistoryMutations() {
	s.historyMutationMu.Lock()
	if s.historyMutationSyncing {
		s.historyMutationWake = true
		s.historyMutationMu.Unlock()
		return
	}
	s.historyMutationSyncing = true
	s.historyMutationMu.Unlock()

	for {
		// Upload first, then delete. If a save is followed immediately by a
		// delete, this preserves the operation order on the server. A delete
		// followed by a save removes its delete intent while saving.
		if err := s.processPendingHistoryUploads(); err != nil {
			log.Printf("[history] pending upload sync failed: %v", err)
		}
		if err := s.processPendingHistoryDeletes(); err != nil {
			log.Printf("[history] pending delete sync failed: %v", err)
		}
		s.historyMutationMu.Lock()
		again := s.historyMutationWake
		s.historyMutationWake = false
		if !again {
			s.historyMutationSyncing = false
			s.historyMutationMu.Unlock()
			return
		}
		s.historyMutationMu.Unlock()
	}
}

func (s *HongguoService) processPendingHistoryUploads() error {
	userCookies, _ := s.authCredentials()
	if userCookies == "" {
		return nil
	}
	items, err := s.store.PendingHistoryUploads(userSyncPageSize)
	if err != nil || len(items) == 0 {
		return err
	}

	episodeIDs := make([]string, len(items))
	valid := make([]HistoryItem, 0, len(items))
	validIDs := make([]string, 0, len(items))
	validEpisodeIDs := make([]string, 0, len(items))
	for i, pending := range items {
		if s.store.HistoryDeleted(pending.SeriesID) {
			_ = s.store.DeletePendingHistoryUploads([]string{pending.SeriesID})
			continue
		}
		item := pending.HistoryItem
		if current, ok := s.store.GetHistoryItem(pending.SeriesID); ok && current.UpdatedAt >= pending.UpdatedAt {
			item = current
		}
		episodeID := pending.EpisodeID
		if episodeID == "" {
			episodeID = s.resolveHistoryEpisodeID(item)
		}
		if _, err := contentInt(item.SeriesID); err != nil {
			return err
		}
		if _, err := contentInt(episodeID); err != nil {
			return fmt.Errorf("历史 %s 缺少有效分集 ID", item.SeriesID)
		}
		episodeIDs[i] = episodeID
		valid = append(valid, item)
		validIDs = append(validIDs, pending.SeriesID)
		validEpisodeIDs = append(validEpisodeIDs, episodeID)
	}
	if len(valid) == 0 {
		return nil
	}
	if err := s.reportPlaybackProgress(valid, validEpisodeIDs); err != nil {
		return err
	}
	return s.store.DeletePendingHistoryUploads(validIDs)
}

func (s *HongguoService) processPendingHistoryDeletes() error {
	userCookies, _ := s.authCredentials()
	if userCookies == "" {
		return nil
	}
	pending, err := s.store.PendingHistoryDeletes(50)
	if err != nil || len(pending) == 0 {
		return err
	}
	items := make([]HistoryItem, 0, len(pending))
	episodeIDs := make([]string, 0, len(pending))
	for _, queued := range pending {
		items = append(items, queued.HistoryItem)
		episodeIDs = append(episodeIDs, queued.EpisodeID)
	}
	send := make([]HistoryItem, 0, len(items))
	sendIDs := make([]string, 0, len(items))
	sendEpisodeIDs := make([]string, 0, len(items))
	for i, item := range items {
		episodeID := pending[i].EpisodeID
		if episodeID == "" {
			episodeID = s.resolveHistoryEpisodeID(item)
		}
		if episodeID == "" {
			continue
		}
		send = append(send, item)
		sendIDs = append(sendIDs, item.SeriesID)
		sendEpisodeIDs = append(sendEpisodeIDs, episodeID)
	}
	if len(send) == 0 {
		return fmt.Errorf("待删除历史缺少可解析分集")
	}
	if err := s.requestHistoryDelete(send, sendEpisodeIDs); err != nil {
		return err
	}
	return s.store.DeletePendingHistoryDeletes(sendIDs)
}
func (s *HongguoService) triggerPendingFavoriteSync() {
	go func() {
		if _, err := s.syncPendingFavoriteOps(); err != nil {
			log.Printf("[favorites] pending sync failed: %v", err)
		}
	}()
}

func (s *HongguoService) syncPendingFavoriteOps() (int, error) {
	processed := 0
	for {
		s.favMutationMu.Lock()
		items, err := s.store.PendingFavoriteOps(userSyncPageSize)
		if err == nil && len(items) > 0 {
			userCookies, _ := s.authCredentials()
			if userCookies == "" {
				s.favMutationMu.Unlock()
				return processed, nil
			}
			for _, item := range items {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := s.postSignedMutation(ctx, "收藏同步", "/reading/bookapi/bookshelf/video/update/v", favoriteMutationPayload(item.SeriesID, item.Added))
				cancel()
				if err == nil {
					err = s.store.DeletePendingFavoriteOps([]string{item.SeriesID})
				}
				if err != nil {
					s.favMutationMu.Unlock()
					return processed, err
				}
				processed++
			}
		}
		s.favMutationMu.Unlock()
		if err != nil {
		}
		if len(items) < userSyncPageSize {
			return processed, nil
		}
	}
}
