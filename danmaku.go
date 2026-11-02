package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"myguo/internal/sign"
)

const (
	danmakuWindowMs   = 30_000
	danmakuMaxWindow  = 600_000
	danmakuMaxTextLen = 200
)

type Danmaku struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	Text       string `json:"text"`
	TimeMs     int64  `json:"timeMs"`
	Color      string `json:"color"`
	LikeCount  int64  `json:"likeCount"`
	Liked      bool   `json:"liked"`
	ShowDigg   bool   `json:"showDigg"`
	ShowAddOne bool   `json:"showAddOne"`
}

type DanmakuPage struct {
	Items  []Danmaku `json:"items"`
	NextMs int64     `json:"nextMs"`
}

type DanmakuSendResult struct {
	ID     string `json:"id,omitempty"`
	UserID string `json:"userId,omitempty"`
	Text   string `json:"text,omitempty"`
	TimeMs int64  `json:"timeMs,omitempty"`
}

type danmakuAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		CommentListInfo struct {
			NextQueryTime int64 `json:"next_query_danmaku_list_time"`
		} `json:"comment_list_info"`
		DataList []struct {
			Comment struct {
				CommentID string `json:"comment_id"`
				Common    struct {
					GroupID string `json:"group_id"`
					Content struct {
						Text string `json:"text"`
					} `json:"content"`
					UserInfo struct {
						BaseInfo struct {
							UserID   string `json:"user_id"`
							UserName string `json:"user_name"`
						} `json:"base_info"`
					} `json:"user_info"`
				} `json:"common"`
				Expand struct {
					BookID         string `json:"book_id"`
					OffsetTime     int64  `json:"offset_time"`
					ShowDiggButton bool   `json:"show_digg_button"`
					ShowAddOne     bool   `json:"show_add_one_button"`
					Color          string `json:"color"`
				} `json:"expand"`
				Stat struct {
					DiggCount  int64 `json:"digg_count"`
					UserAction struct {
						UserDigg bool `json:"user_digg"`
					} `json:"user_action"`
				} `json:"stat"`
			} `json:"comment"`
		} `json:"data_list"`
		Comment struct {
			CommentID string `json:"comment_id"`
		} `json:"comment"`
	} `json:"data"`
}

func parseDanmakuPage(raw []byte, seriesID, groupID string, startMs, durationMs int64) (*DanmakuPage, error) {
	if durationMs <= 0 {
		durationMs = danmakuWindowMs
	}
	var res danmakuAPIResponse
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&res); err != nil {
		return nil, fmt.Errorf("弹幕响应解析失败: %w", err)
	}
	if res.Code != 0 {
		message := strings.TrimSpace(res.Msg)
		if message == "" {
			message = fmt.Sprintf("%d", res.Code)
		}
		return nil, fmt.Errorf("弹幕接口返回错误: %s", message)
	}

	items := make([]Danmaku, 0, len(res.Data.DataList))
	seen := make(map[string]bool)
	for _, row := range res.Data.DataList {
		comment := row.Comment
		if comment.CommentID == "" || seen[comment.CommentID] {
			continue
		}
		if comment.Common.GroupID != "" && comment.Common.GroupID != groupID {
			continue
		}
		if comment.Expand.BookID != "" && comment.Expand.BookID != seriesID {
			continue
		}
		text := strings.TrimSpace(comment.Common.Content.Text)
		if text == "" || utf8.RuneCountInString(text) > 500 {
			continue
		}
		timeMs := comment.Expand.OffsetTime
		windowEnd := startMs + durationMs
		if timeMs < startMs || timeMs >= windowEnd {
			continue
		}
		color := strings.TrimSpace(comment.Expand.Color)
		if color == "" {
			color = "#ffffff"
		}
		seen[comment.CommentID] = true
		items = append(items, Danmaku{
			ID:         comment.CommentID,
			UserID:     strings.TrimSpace(comment.Common.UserInfo.BaseInfo.UserID),
			Text:       text,
			TimeMs:     timeMs,
			Color:      color,
			LikeCount:  comment.Stat.DiggCount,
			Liked:      comment.Stat.UserAction.UserDigg,
			ShowDigg:   comment.Expand.ShowDiggButton,
			ShowAddOne: comment.Expand.ShowAddOne,
		})
	}

	nextMs := res.Data.CommentListInfo.NextQueryTime
	if nextMs <= startMs {
		nextMs = startMs + durationMs
	}
	return &DanmakuPage{Items: items, NextMs: nextMs}, nil
}

func (s *HongguoService) postCommentAPI(ctx context.Context, path string, body any, source, channel string, requireLogin bool) ([]byte, error) {
	userCookies, userToken := s.authCredentials()
	if requireLogin && userCookies == "" {
		return nil, fmt.Errorf("请先登录后使用弹幕互动")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := defaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
	rawQuery, headers := sign.SignPOST(path, pairs, time.Now().UnixMilli(), deviceID, "7.3.9.32", payload, nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, commentAPIOrigin+path+"?"+rawQuery, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Comment-Source", source)
	req.Header.Set("Server-Channel", channel)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", s.userAgent)
	if userCookies != "" {
		req.Header.Set("Cookie", userCookies)
	}
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("弹幕接口返回 HTTP %d", resp.StatusCode)
	}
	return raw, nil
}

func (s *HongguoService) fetchTimedCommentFallback(ctx context.Context, seriesID, episodeID string, startMs, durationMs int64) (*DanmakuPage, error) {
	// The dedicated danmaku handler is not exposed for every playback group.
	// Ordinary comments carry the same playback offset and provide an
	// authentic timed stream when the dedicated handler rejects the group.
	raw, err := s.postCommentAPI(ctx, "/novel/commentapi/comment/list/"+episodeID+"/v1/", map[string]any{
		"comment_source":        4,
		"server_channel":        18,
		"group_id":              episodeID,
		"group_type":            30,
		"comment_type":          4,
		"sort":                  1,
		"need_count":            true,
		"start_offset_time":     startMs,
		"playlet_item_duration": durationMs,
		"count":                 90,
		"cursor":                "",
		"aid":                   8662,
		"compliance_status":     0,
		"business_param": map[string]any{
			"book_id":  seriesID,
			"req_type": 0,
		},
	}, "4", "18", false)
	if err != nil {
		return nil, err
	}
	return parseDanmakuPage(raw, seriesID, episodeID, startMs, durationMs)
}

func (s *HongguoService) GetDanmaku(ctx context.Context, seriesID, episodeID string, startMs, durationMs int64) (*DanmakuPage, error) {
	if !validContentID(seriesID) || !validContentID(episodeID) {
		return nil, fmt.Errorf("无效弹幕归属 ID")
	}
	if startMs < 0 {
		startMs = 0
	}
	if durationMs <= 0 {
		durationMs = danmakuWindowMs
	}
	if durationMs > danmakuMaxWindow {
		durationMs = danmakuMaxWindow
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	raw, err := s.postCommentAPI(ctx, "/novel/commentapi/comment/list/"+episodeID+"/v1/", map[string]any{
		"comment_source":          601,
		"server_channel":          1000,
		"group_id":                episodeID,
		"group_type":              30,
		"comment_type":            20,
		"sort":                    1,
		"book_id":                 seriesID,
		"start_offset_time":       startMs,
		"playlet_item_duration":   durationMs,
		"need_danmaku_guide_type": false,
		"business_param": map[string]any{
			"count":             90,
			"cursor":            "",
			"aid":               "8662",
			"compliance_status": 0,
		},
	}, "601", "1000", false)
	if err == nil {
		if page, parseErr := parseDanmakuPage(raw, seriesID, episodeID, startMs, durationMs); parseErr == nil {
			return sanitizeIPC(page), nil
		}
	}
	if page, err := s.fetchTimedCommentFallback(ctx, seriesID, episodeID, startMs, durationMs); err == nil {
		return sanitizeIPC(page), nil
	}
	return nil, fmt.Errorf("弹幕接口不可用")
}

func (s *HongguoService) SendDanmaku(ctx context.Context, seriesID, episodeID, text string, timeMs int64, replyTo string) (*DanmakuSendResult, error) {
	if !validContentID(seriesID) || !validContentID(episodeID) {
		return nil, fmt.Errorf("无效弹幕归属 ID")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("弹幕内容不能为空")
	}
	if utf8.RuneCountInString(text) > danmakuMaxTextLen {
		return nil, fmt.Errorf("弹幕内容最长 %d 个字符", danmakuMaxTextLen)
	}
	if timeMs < 0 {
		timeMs = 0
	}
	replyTo = strings.TrimSpace(replyTo)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	outer := map[string]any{
		"ignore_urge_rule": false,
		"book_id":          seriesID,
		"offset":           timeMs,
		"group_type":       1,
		"comment_type":     20,
		"sort":             1,
		"item_id":          episodeID,
		"text":             text,
		"business_param": map[string]any{
			"commit_source": 20,
			"group_id":      episodeID,
			"group_type":    1,
			"text":          text,
			"aid":           "8662",
		},
	}
	if replyTo != "" {
		outer["reply_to"] = replyTo
	}
	raw, err := s.postCommentAPI(ctx, "/novel/commentapi/comment/add/v1/", outer, "601", "1000", true)
	if err != nil {
		return nil, err
	}
	var res danmakuAPIResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("发送弹幕响应解析失败: %w", err)
	}
	if res.Code != 0 {
		message := strings.TrimSpace(res.Msg)
		if message == "" {
			message = fmt.Sprintf("%d", res.Code)
		}
		return nil, fmt.Errorf("发送弹幕失败: %s", message)
	}
	return sanitizeIPC(&DanmakuSendResult{
		ID:     res.Data.Comment.CommentID,
		UserID: s.store.GetProfile().UserID,
		Text:   text,
		TimeMs: timeMs,
	}), nil
}

func (s *HongguoService) LikeDanmaku(ctx context.Context, episodeID, danmakuID string, liked bool) (bool, error) {
	if !validContentID(episodeID) {
		return false, fmt.Errorf("无效分集 ID")
	}
	if danmakuID == "" || len(danmakuID) > 120 {
		return false, fmt.Errorf("无效弹幕 ID")
	}
	actionType := 1
	if !liked {
		actionType = 2
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	raw, err := s.postCommentAPI(ctx, "/novel/commentapi/comment/do_action/v1/", map[string]any{
		"object_id":    danmakuID,
		"object_type":  8,
		"action_type":  actionType,
		"comment_type": 20,
		"aid":          "8662",
		"enter_from":   "default_digg_source",
		"target":       episodeID,
	}, "601", "1000", true)
	if err != nil {
		return false, err
	}
	var res danmakuAPIResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return false, fmt.Errorf("弹幕点赞响应解析失败: %w", err)
	}
	if res.Code != 0 {
		message := strings.TrimSpace(res.Msg)
		if message == "" {
			message = fmt.Sprintf("%d", res.Code)
		}
		return false, fmt.Errorf("弹幕点赞失败: %s", message)
	}
	return liked, nil
}
