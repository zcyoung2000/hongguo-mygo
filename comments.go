package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"myguo/internal/sign"
)

const commentAPIOrigin = "https://api5-normal-sinfonlineb.fqnovel.com"

type CommentDTO struct {
	ID         string `json:"id"`
	Author     string `json:"author"`
	AvatarURL  string `json:"avatarUrl,omitempty"`
	Content    string `json:"content"`
	CreatedAt  int64  `json:"createdAt"`
	Likes      int64  `json:"likes"`
	ReplyCount int64  `json:"replyCount"`
	ReplyTo    string `json:"replyTo,omitempty"`
}

type CommentPageDTO struct {
	Items      []CommentDTO `json:"items"`
	Total      int64        `json:"total"`
	Cursor     string       `json:"cursor,omitempty"`
	HasMore    bool         `json:"hasMore"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type commentAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		CommonListInfo struct {
			Total   int64  `json:"total"`
			HasMore bool   `json:"has_more"`
			Cursor  string `json:"cursor"`
		} `json:"common_list_info"`
		DataList []struct {
			Comment struct {
				CommentID string `json:"comment_id"`
				Common    struct {
					GroupID string `json:"group_id"`
					Status  int    `json:"status"`
					Content struct {
						Text string `json:"text"`
					} `json:"content"`
					UserInfo struct {
						BaseInfo struct {
							UserName   string `json:"user_name"`
							UserAvatar string `json:"user_avatar"`
						} `json:"base_info"`
					} `json:"user_info"`
					CreateTimestamp int64 `json:"create_timestamp"`
				} `json:"common"`
				Expand struct {
					BookID string `json:"book_id"`
				} `json:"expand"`
				Stat struct {
					DiggCount  int64 `json:"digg_count"`
					ReplyCount int64 `json:"reply_count"`
				} `json:"stat"`
			} `json:"comment"`
		} `json:"data_list"`
	} `json:"data"`
}

type commentReplyAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Comment *struct {
			Common struct {
				GroupID string `json:"group_id"`
			} `json:"common"`
			Expand struct {
				BookID string `json:"book_id"`
			} `json:"expand"`
		} `json:"comment"`
		CommentListInfo struct {
			Total   int64  `json:"total"`
			HasMore bool   `json:"has_more"`
			Cursor  string `json:"cursor"`
		} `json:"comment_list_info"`
		ReplyList []struct {
			ReplyID string `json:"reply_id"`
			Common  struct {
				Content struct {
					Text string `json:"text"`
				} `json:"content"`
				UserInfo struct {
					BaseInfo struct {
						UserName   string `json:"user_name"`
						UserAvatar string `json:"user_avatar"`
					} `json:"base_info"`
				} `json:"user_info"`
				CreateTimestamp int64 `json:"create_timestamp"`
			} `json:"common"`
			Stat struct {
				DiggCount  int64 `json:"digg_count"`
				ReplyCount int64 `json:"reply_count"`
			} `json:"stat"`
			ReplyToUserInfo *struct {
				BaseInfo struct {
					UserName string `json:"user_name"`
				} `json:"base_info"`
			} `json:"reply_to_user_info"`
		} `json:"reply_list"`
	} `json:"data"`
}

func validContentID(id string) bool {
	if len(id) < 6 || len(id) > 30 {
		return false
	}
	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validCommentCursor(cursor string) error {
	if cursor == "" {
		return nil
	}
	if len(cursor) > 8192 {
		return fmt.Errorf("评论游标过长")
	}
	var value struct {
		SessionID string `json:"session_id"`
		Offset    int64  `json:"offset"`
	}
	if err := json.Unmarshal([]byte(cursor), &value); err != nil {
		return fmt.Errorf("评论游标无效")
	}
	if value.SessionID == "" || len(value.SessionID) > 200 || value.Offset < 0 || value.Offset > 10000000 {
		return fmt.Errorf("评论游标无效")
	}
	return nil
}

func validReplyCursor(cursor string) error {
	if cursor == "" {
		return nil
	}
	if len(cursor) > 32 {
		return fmt.Errorf("回复游标过长")
	}
	offset, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || offset < 0 || offset > 10000000 {
		return fmt.Errorf("回复游标无效")
	}
	return nil
}

func parseCommentPage(raw []byte, seriesID, groupID, cursor string) (*CommentPageDTO, error) {
	var res commentAPIResponse
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&res); err != nil {
		return nil, fmt.Errorf("评论响应解析失败: %w", err)
	}
	if res.Code != 0 {
		message := strings.TrimSpace(res.Msg)
		if message == "" {
			message = strconv.Itoa(res.Code)
		}
		return nil, fmt.Errorf("评论接口返回错误: %s", message)
	}

	previousOffset := int64(0)
	if cursor != "" {
		var value struct {
			Offset int64 `json:"offset"`
		}
		if json.Unmarshal([]byte(cursor), &value) == nil {
			previousOffset = value.Offset
		}
	}
	info := res.Data.CommonListInfo
	next := strings.TrimSpace(info.Cursor)
	if info.HasMore && (next == cursor || len(next) > 8192) {
		return nil, fmt.Errorf("评论分页游标无效")
	}
	if info.HasMore && next != "" {
		var value struct {
			Offset int64 `json:"offset"`
		}
		if err := json.Unmarshal([]byte(next), &value); err != nil || value.Offset <= previousOffset {
			return nil, fmt.Errorf("评论分页游标无效")
		}
	}

	items := make([]CommentDTO, 0, len(res.Data.DataList))
	seen := make(map[string]bool)
	for _, row := range res.Data.DataList {
		comment := row.Comment
		common := comment.Common
		if comment.CommentID == "" || seen[comment.CommentID] || common.Status != 1 {
			continue
		}
		if common.GroupID != groupID || (comment.Expand.BookID != "" && comment.Expand.BookID != seriesID) {
			continue
		}
		content := strings.TrimSpace(common.Content.Text)
		if content == "" || len(content) > 20000 {
			continue
		}
		if len(comment.CommentID) > 120 {
			continue
		}
		seen[comment.CommentID] = true
		items = append(items, CommentDTO{
			ID:         comment.CommentID,
			Author:     strings.TrimSpace(common.UserInfo.BaseInfo.UserName),
			AvatarURL:  strings.TrimSpace(common.UserInfo.BaseInfo.UserAvatar),
			Content:    content,
			CreatedAt:  common.CreateTimestamp,
			Likes:      comment.Stat.DiggCount,
			ReplyCount: comment.Stat.ReplyCount,
		})
		if len(items) >= 100 {
			break
		}
	}

	return &CommentPageDTO{
		Items:      items,
		Total:      info.Total,
		Cursor:     cursor,
		HasMore:    info.HasMore,
		NextCursor: next,
	}, nil
}

func parseReplyPage(raw []byte, seriesID, groupID, commentID, cursor string) (*CommentPageDTO, error) {
	var res commentReplyAPIResponse
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&res); err != nil {
		return nil, fmt.Errorf("回复响应解析失败: %w", err)
	}
	if res.Code != 0 {
		message := strings.TrimSpace(res.Msg)
		if message == "" {
			message = strconv.Itoa(res.Code)
		}
		return nil, fmt.Errorf("回复接口返回错误: %s", message)
	}

	previousOffset := int64(0)
	if cursor != "" {
		if value, err := strconv.ParseInt(cursor, 10, 64); err == nil {
			previousOffset = value
		}
	}
	info := res.Data.CommentListInfo
	next := strings.TrimSpace(info.Cursor)
	if info.HasMore && (next == cursor || len(next) > 32) {
		return nil, fmt.Errorf("回复分页游标无效")
	}
	if info.HasMore && next != "" {
		offset, err := strconv.ParseInt(next, 10, 64)
		if err != nil || offset <= previousOffset {
			return nil, fmt.Errorf("回复分页游标无效")
		}
	}
	if res.Data.Comment != nil {
		common := res.Data.Comment.Common
		expand := res.Data.Comment.Expand
		if common.GroupID != "" && common.GroupID != groupID {
			return nil, fmt.Errorf("回复归属校验失败")
		}
		if expand.BookID != "" && expand.BookID != seriesID {
			return nil, fmt.Errorf("回复归属校验失败")
		}
	}

	items := make([]CommentDTO, 0, len(res.Data.ReplyList))
	seen := make(map[string]bool)
	for _, row := range res.Data.ReplyList {
		if row.ReplyID == "" || len(row.ReplyID) > 120 || seen[row.ReplyID] {
			continue
		}
		content := strings.TrimSpace(row.Common.Content.Text)
		if content == "" || len(content) > 20000 {
			continue
		}
		seen[row.ReplyID] = true
		replyTo := ""
		if row.ReplyToUserInfo != nil {
			replyTo = strings.TrimSpace(row.ReplyToUserInfo.BaseInfo.UserName)
		}
		items = append(items, CommentDTO{
			ID:        row.ReplyID,
			Author:    strings.TrimSpace(row.Common.UserInfo.BaseInfo.UserName),
			AvatarURL: strings.TrimSpace(row.Common.UserInfo.BaseInfo.UserAvatar),
			Content:   content,
			CreatedAt: row.Common.CreateTimestamp,
			Likes:     row.Stat.DiggCount,
			ReplyTo:   replyTo,
		})
		if len(items) >= 100 {
			break
		}
	}

	return &CommentPageDTO{
		Items:      items,
		Total:      info.Total,
		Cursor:     cursor,
		HasMore:    info.HasMore,
		NextCursor: next,
	}, nil
}

func (s *HongguoService) fetchEpisodeComments(ctx context.Context, seriesID, episodeID, cursor string) (*CommentPageDTO, error) {
	if !validContentID(seriesID) {
		return nil, fmt.Errorf("无效短剧 ID")
	}
	if !validContentID(episodeID) {
		return nil, fmt.Errorf("无效分集 ID")
	}
	if err := validCommentCursor(cursor); err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]any{
		"comment_source":    4,
		"server_channel":    18,
		"group_id":          episodeID,
		"group_type":        30,
		"comment_type":      4,
		"sort":              1,
		"need_count":        true,
		"count":             20,
		"cursor":            cursor,
		"aid":               8662,
		"compliance_status": 0,
		"business_param": map[string]any{
			"book_id":  seriesID,
			"req_type": 0,
		},
	})
	if err != nil {
		return nil, err
	}

	path := "/novel/commentapi/comment/list/" + episodeID + "/v1/"
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := defaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
	ticket := time.Now().UnixMilli()
	rawQuery, sigHeaders := sign.SignPOST(path, pairs, ticket, deviceID, "7.3.9.32", body, nil)
	req, err := http.NewRequestWithContext(ctx, "POST", commentAPIOrigin+path+"?"+rawQuery, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range sigHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("Comment-Source", "4")
	req.Header.Set("Server-Channel", "18")
	req.Header.Set("User-Agent", s.userAgent)

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
		return nil, fmt.Errorf("评论接口返回 HTTP %d", resp.StatusCode)
	}

	page, err := parseCommentPage(raw, seriesID, episodeID, cursor)
	if err != nil {
		return nil, err
	}
	return sanitizeIPC(page), nil
}

// GetEpisodeComments loads one page of official episode comments. The first
// request uses an empty cursor; pagination passes the previous NextCursor.
func (s *HongguoService) GetEpisodeComments(seriesID, episodeID, cursor string) (*CommentPageDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	page, err := s.fetchEpisodeComments(ctx, seriesID, episodeID, cursor)
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		page.Items[i].AvatarURL = s.media.ImageURL(page.Items[i].AvatarURL)
	}
	return sanitizeIPC(page), nil
}

func (s *HongguoService) fetchEpisodeCommentReplies(ctx context.Context, seriesID, episodeID, commentID, cursor string) (*CommentPageDTO, error) {
	if !validContentID(seriesID) || !validContentID(episodeID) {
		return nil, fmt.Errorf("无效评论归属 ID")
	}
	if !validContentID(commentID) {
		return nil, fmt.Errorf("无效评论 ID")
	}
	if err := validReplyCursor(cursor); err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]any{
		"aid": 8662,
		"business_param": map[string]any{
			"book_id":    seriesID,
			"need_count": cursor == "",
		},
		"comment_id":        commentID,
		"comment_source":    504,
		"comment_type":      4,
		"compliance_status": 0,
		"count":             10,
		"cursor":            cursor,
		"group_id":          episodeID,
		"group_type":        30,
		"server_channel":    18,
	})
	if err != nil {
		return nil, err
	}

	path := "/novel/commentapi/reply/list/" + commentID + "/v1/"
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := defaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
	ticket := time.Now().UnixMilli()
	rawQuery, sigHeaders := sign.SignPOST(path, pairs, ticket, deviceID, "7.3.9.32", body, nil)
	req, err := http.NewRequestWithContext(ctx, "POST", commentAPIOrigin+path+"?"+rawQuery, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range sigHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("Comment-Source", "504")
	req.Header.Set("Server-Channel", "18")
	req.Header.Set("User-Agent", s.userAgent)

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
		return nil, fmt.Errorf("回复接口返回 HTTP %d", resp.StatusCode)
	}
	return parseReplyPage(raw, seriesID, episodeID, commentID, cursor)
}

// GetEpisodeCommentReplies loads one page of replies for an episode comment.
// First page uses an empty cursor; pagination passes the numeric NextCursor.
func (s *HongguoService) GetEpisodeCommentReplies(seriesID, episodeID, commentID, cursor string) (*CommentPageDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	page, err := s.fetchEpisodeCommentReplies(ctx, seriesID, episodeID, commentID, cursor)
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		page.Items[i].AvatarURL = s.media.ImageURL(page.Items[i].AvatarURL)
	}
	return sanitizeIPC(page), nil
}
