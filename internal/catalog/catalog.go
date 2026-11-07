package catalog

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"io"
	"myguo/internal/sign"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SeriesCard struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	CoverURL     string   `json:"coverUrl"`
	EpisodeCount int      `json:"episodeCount"`
	Category     string   `json:"category"`
	ContentType  int      `json:"contentType,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	BadgeText    string   `json:"badgeText,omitempty"`
	Author       string   `json:"author"`
	Status       string   `json:"status"`
	Heat         string   `json:"heat"`
	Tags         []string `json:"tags"`
	Upcoming     bool     `json:"upcoming,omitempty"`
	Reserved     bool     `json:"reserved,omitempty"`
}

type FilterOptionDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type FilterRowDTO struct {
	Key     string            `json:"key"`
	Name    string            `json:"name"`
	Options []FilterOptionDTO `json:"options"`
}

type DiscoverResponse struct {
	Items   []SeriesCard `json:"items"`
	HasMore bool         `json:"hasMore"`
	Cursor  string       `json:"cursor"`
}

type RankTabDTO struct {
	ContentID string `json:"contentId"`
	BoardID   string `json:"boardId"`
	Title     string `json:"title"`
	IsDefault bool   `json:"isDefault"`
}

type RankItemDTO struct {
	EpisodeCount int      `json:"episodeCount,omitempty"`
	Position     int      `json:"position"`
	SeriesID     string   `json:"seriesId"`
	Title        string   `json:"title"`
	CoverURL     string   `json:"coverUrl"`
	Heat         string   `json:"heat"`
	Description  string   `json:"description"`
	ItemType     string   `json:"itemType"`
	Promoted     bool     `json:"promoted,omitempty"`
	Actor        string   `json:"actor,omitempty"`
	ActorID      string   `json:"actorId,omitempty"`
	UserID       string   `json:"userId,omitempty"`
	AvatarURL    string   `json:"avatarUrl,omitempty"`
	Intro        string   `json:"intro,omitempty"`
	Details      []string `json:"details,omitempty"`
}

type RankResponse struct {
	Contents []model.RankContent `json:"contents"`
	Content  string              `json:"content"`
	Board    string              `json:"board"`
	Items    []RankItemDTO       `json:"items"`
	Cursor   string              `json:"cursor,omitempty"`
	HasMore  bool                `json:"hasMore"`
}

type ActorProfileDTO struct {
	UserID      string       `json:"userId"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	AvatarURL   string       `json:"avatarUrl"`
	Tags        []string     `json:"tags"`
	Labels      []string     `json:"labels"`
	Followers   int64        `json:"followers"`
	Likes       int64        `json:"likes"`
	Works       []SeriesCard `json:"works"`
	HasMore     bool         `json:"hasMore"`
	Cursor      string       `json:"cursor"`
}

type ActorWorksResponse struct {
	Works   []SeriesCard `json:"works"`
	HasMore bool         `json:"hasMore"`
	Cursor  string       `json:"cursor"`
}

type NewReleaseItemDTO struct {
	SeriesID     string `json:"seriesId"`
	Title        string `json:"title"`
	CoverURL     string `json:"coverUrl"`
	EpisodeCount int    `json:"episodeCount"`
	Online       bool   `json:"online"`
	Heat         string `json:"heat"`
	Note         string `json:"note"`
	Category     string `json:"category"`
	Reserved     bool   `json:"reserved,omitempty"`
}

type NewReleaseResponse struct {
	CurrentDate string              `json:"currentDate"`
	Dates       []string            `json:"dates"`
	Items       []NewReleaseItemDTO `json:"items"`
}

type HotSearchDTO struct {
	Words []string `json:"words"`
}

type EpisodeDTO struct {
	ID           string `json:"id"`
	SeriesID     string `json:"seriesId"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Playable     bool   `json:"playable"`
	PosterURL    string `json:"posterUrl,omitempty"`
	CommentCount int64  `json:"commentCount"`
}

type SeriesDetailDTO struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	CoverURL     string       `json:"coverUrl"`
	EpisodeCount int          `json:"episodeCount"`
	Status       string       `json:"status"`
	Heat         string       `json:"heat"`
	Episodes     []EpisodeDTO `json:"episodes"`
	CommentCount int64        `json:"commentCount"`
	IsFavorite   bool         `json:"isFavorite"`
}

type cacheItem struct {
	val       any
	expiresAt time.Time
}

type Dependencies struct {
	Credentials       func() (cookies, token string)
	DeviceIdentity    func() (deviceID, installID, cdid, openudid string)
	Runtime           func() (userAgent, storeRegion string)
	ReservedSeriesIDs func(ctx context.Context) ([]string, error)
	Episodes          func(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error)
	IsFavorite        func(seriesID string) bool
}

type Service struct {
	client            *sdk.Client
	imageURL          func(string) string
	httpClient        *http.Client
	credentials       func() (cookies, token string)
	deviceIdentity    func() (deviceID, installID, cdid, openudid string)
	runtime           func() (userAgent, storeRegion string)
	reservedSeriesIDs func(ctx context.Context) ([]string, error)
	episodes          func(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error)
	isFavorite        func(seriesID string) bool
	cacheMu           sync.RWMutex
	cache             map[string]cacheItem
}

func NewService(client *sdk.Client, imageURL func(string) string, httpClient *http.Client, deps Dependencies) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if deps.Credentials == nil {
		deps.Credentials = func() (string, string) { return "", "" }
	}
	if deps.DeviceIdentity == nil {
		deps.DeviceIdentity = func() (string, string, string, string) { return "", "", "", "" }
	}
	if deps.Runtime == nil {
		deps.Runtime = func() (string, string) { return "", "" }
	}
	if deps.ReservedSeriesIDs == nil {
		deps.ReservedSeriesIDs = func(context.Context) ([]string, error) { return nil, nil }
	}
	if deps.Episodes == nil {
		deps.Episodes = func(context.Context, string) (*sdk.Page[model.Episode], error) {
			return nil, errors.New("episode provider unavailable")
		}
	}
	if deps.IsFavorite == nil {
		deps.IsFavorite = func(string) bool { return false }
	}
	return &Service{
		client: client, imageURL: imageURL, httpClient: httpClient,
		credentials: deps.Credentials, deviceIdentity: deps.DeviceIdentity,
		runtime: deps.Runtime, reservedSeriesIDs: deps.ReservedSeriesIDs,
		episodes: deps.Episodes, isFavorite: deps.IsFavorite,
		cache: make(map[string]cacheItem),
	}
}

func (s *Service) Invalidate() {
	s.cacheMu.Lock()
	s.cache = make(map[string]cacheItem)
	s.cacheMu.Unlock()
}

func (s *Service) InvalidatePrefixes(prefixes ...string) {
	if len(prefixes) == 0 {
		s.Invalidate()
		return
	}
	s.cacheMu.Lock()
	for key := range s.cache {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				delete(s.cache, key)
				break
			}
		}
	}
	s.cacheMu.Unlock()
}

func shortFilterSelections(genre string, values map[string]string) map[string][]string {
	selections := make(map[string][]string)
	if genre != "" && genre != "all" {
		selections["genre"] = []string{genre}
	}
	for key, value := range values {
		if key == "genre" || value == "" || value == "all" {
			continue
		}
		selections[key] = []string{value}
	}
	return selections
}

func pageItems[T any](page *sdk.Page[T]) []T {
	if page == nil {
		return nil
	}
	return page.Items
}

func shortRankItems(rank *model.ShortRanking) []model.RankItem {
	if rank == nil {
		return nil
	}
	return rank.Items
}

func shortRankContents(rank *model.ShortRanking) []model.RankContent {
	if rank == nil {
		return nil
	}
	return rank.Contents
}

func shortRankContent(rank *model.ShortRanking) string {
	if rank == nil {
		return ""
	}
	return rank.Content
}

func shortRankBoard(rank *model.ShortRanking) string {
	if rank == nil {
		return ""
	}
	return rank.Board
}

func newReleaseItems(page *model.NewReleasePage) []model.NewRelease {
	if page == nil {
		return nil
	}
	return page.Items
}

func newReleaseDate(page *model.NewReleasePage) string {
	if page == nil {
		return ""
	}
	return page.Date
}

func newReleaseDates(page *model.NewReleasePage) []string {
	if page == nil {
		return nil
	}
	return page.Dates
}

func workItems(page *model.WorkPage) []model.Work {
	if page == nil {
		return nil
	}
	return page.Items
}

func workCursor(page *model.WorkPage) string {
	if page == nil {
		return ""
	}
	return page.Cursor
}

func shortFilterItems(page *model.ShortFilterPage) []model.SeriesSummary {
	if page == nil {
		return nil
	}
	return page.Items
}

func shortFilterHasMore(page *model.ShortFilterPage) bool {
	return page != nil && page.HasMore
}

func shortFilterCursor(page *model.ShortFilterPage) string {
	if page == nil {
		return ""
	}
	return page.Cursor
}

func hotGuesses(hot *model.HotSearch) []model.HotItem {
	if hot == nil {
		return nil
	}
	return hot.Guesses
}

func hotBoards(hot *model.HotSearch) []model.HotBoard {
	if hot == nil {
		return nil
	}
	return hot.Boards
}

func (s *Service) GetFilterPanel(ctx context.Context, genre string) ([]FilterRowDTO, error) {
	cacheKey := "filter_panel:" + genre
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.val.([]FilterRowDTO), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	rows, err := s.client.ShortFilterPanel(ctx, genre)
	if err != nil {
		return nil, err
	}

	var res []FilterRowDTO
	for _, r := range rows {
		rowDTO := FilterRowDTO{
			Key:  r.Key,
			Name: r.Name,
		}
		for _, o := range r.Options {
			rowDTO.Options = append(rowDTO.Options, FilterOptionDTO{
				Name:  o.Name,
				Value: o.Value,
			})
		}
		res = append(res, rowDTO)
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: res, expiresAt: time.Now().Add(1 * time.Hour)}
	s.cacheMu.Unlock()

	return res, nil
}

func (s *Service) GetDiscover(ctx context.Context, genre string, selections map[string]string, cursor string) (DiscoverResponse, error) {
	selectionJSON, err := json.Marshal(selections)
	if err != nil {
		return DiscoverResponse{}, fmt.Errorf("encode discover selections: %w", err)
	}
	cacheKey := fmt.Sprintf("discover:%s:%s:%s", genre, selectionJSON, cursor)
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.val.(DiscoverResponse), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	shortSelections := shortFilterSelections(genre, selections)

	res, err := s.client.ShortFilter(ctx, sdk.ShortFilterQuery{
		Selections: shortSelections,
		Cursor:     cursor,
	})
	if err != nil {
		if errors.Is(err, sdk.ErrContentUnavailable) {
			resp := DiscoverResponse{
				Items:   []SeriesCard{},
				HasMore: false,
				Cursor:  "",
			}
			s.cacheMu.Lock()
			s.cache[cacheKey] = cacheItem{val: resp, expiresAt: time.Now().Add(5 * time.Minute)}
			s.cacheMu.Unlock()
			return resp, nil
		}
		return DiscoverResponse{}, err
	}

	resItems := shortFilterItems(res)
	cards := make([]SeriesCard, 0, len(resItems))
	for _, it := range resItems {
		var tags []string
		if it.Category != "" {
			tags = append(tags, it.Category)
		}
		tags = append(tags, fmt.Sprintf("%d集", it.EpisodeCount))

		cards = append(cards, SeriesCard{
			ID:           it.ID,
			Title:        it.Title,
			Description:  it.Description,
			CoverURL:     s.imageURL(it.CoverURL),
			EpisodeCount: it.EpisodeCount,
			Category:     it.Category,
			Author:       it.Author,
			Status:       "finished",
			Heat:         "精选",
			Tags:         tags,
		})
	}

	resp := DiscoverResponse{
		Items:   cards,
		HasMore: shortFilterHasMore(res),
		Cursor:  shortFilterCursor(res),
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: resp, expiresAt: time.Now().Add(5 * time.Minute)}
	s.cacheMu.Unlock()

	return resp, nil
}

func (s *Service) GetRankings(ctx context.Context, contentId, boardId, cursor string) (RankResponse, error) {
	userCookies, _ := s.credentials()
	rankScope := "anonymous"
	if userCookies != "" {
		rankScope = "account"
	}
	cacheKey := fmt.Sprintf("rank:%s:%s:%s:%s", rankScope, contentId, boardId, cursor)
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.val.(RankResponse), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	var rank *model.ShortRanking
	var err error
	if userCookies != "" {
		rank, err = s.fetchAccountShortRank(ctx, contentId, boardId, cursor)
		// A signed anonymous request can still show the public board when the
		// account request fails. Do not mix an account cursor with a new
		// anonymous session on later pages.
		if err != nil && cursor == "" {
			rank, err = s.client.ShortRank(ctx, sdk.ShortRankQuery{
				Content: contentId,
				Board:   boardId,
			})
		}
	} else {
		rank, err = s.client.ShortRank(ctx, sdk.ShortRankQuery{
			Content: contentId,
			Board:   boardId,
			Cursor:  cursor,
		})
	}
	if err != nil && errors.Is(err, sdk.ErrContentUnavailable) && boardId != "" && cursor == "" {
		// Fallback to default board if requested sub-board is unavailable
		rank, err = s.client.ShortRank(ctx, sdk.ShortRankQuery{
			Content: contentId,
		})
	}
	if err != nil {
		return RankResponse{}, err
	}

	var items []RankItemDTO
	for _, it := range shortRankItems(rank) {
		isActor := (shortRankContent(rank) == "ranklist_celebrity" || it.SeriesID == "") && (it.Actor != "" || it.UserID != "" || it.AvatarURL != "")
		if isActor {
			title := it.Actor
			if title == "" {
				title = it.Title
			}
			avatar := s.imageURL(it.AvatarURL)
			if avatar == "" && it.CoverURL != "" {
				avatar = s.imageURL(it.CoverURL)
			}
			desc := it.Intro
			if desc == "" && len(it.Details) > 0 {
				desc = strings.Join(it.Details, " · ")
			}
			items = append(items, RankItemDTO{
				Position:     it.Position,
				SeriesID:     it.SeriesID,
				EpisodeCount: it.EpisodeCount,
				Title:        title,
				CoverURL:     avatar,
				Heat:         it.Score,
				Description:  desc,
				ItemType:     "actor",
				Promoted:     it.Promoted,
				Actor:        it.Actor,
				ActorID:      it.ActorID,
				UserID:       it.UserID,
				AvatarURL:    avatar,
				Intro:        it.Intro,
				Details:      it.Details,
			})
		} else {
			desc := ""
			if it.EpisodeCount > 0 {
				desc = fmt.Sprintf("全%d集", it.EpisodeCount)
			} else if it.Actor != "" {
				desc = it.Actor
			}
			items = append(items, RankItemDTO{
				Position:     it.Position,
				SeriesID:     it.SeriesID,
				EpisodeCount: it.EpisodeCount,
				Title:        it.Title,
				CoverURL:     s.imageURL(it.CoverURL),
				Heat:         it.Score,
				Description:  desc,
				ItemType:     "drama",
				Promoted:     it.Promoted,
				Actor:        it.Actor,
			})
		}
	}

	resp := RankResponse{
		Contents: shortRankContents(rank),
		Content:  shortRankContent(rank),
		Board:    shortRankBoard(rank),
		Items:    items,
		Cursor:   shortRankCursor(rank),
		HasMore:  shortRankHasMore(rank),
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: resp, expiresAt: time.Now().Add(5 * time.Minute)}
	if rank != nil && rank.Board != "" && rank.Board != boardId {
		s.cache[fmt.Sprintf("rank:%s:%s", contentId, rank.Board)] = cacheItem{val: resp, expiresAt: time.Now().Add(5 * time.Minute)}
	}
	s.cacheMu.Unlock()

	return resp, nil
}

type rankCursor struct {
	Offset  int      `json:"o"`
	Seen    []string `json:"s,omitempty"`
	Session string   `json:"x,omitempty"`
	Content string   `json:"c,omitempty"`
	Board   string   `json:"b,omitempty"`
}

func decodeRankCursor(token string) (rankCursor, error) {
	cur := rankCursor{}
	if token == "" {
		return cur, nil
	}
	payload, ok := strings.CutPrefix(token, "s1.")
	if !ok {
		return cur, fmt.Errorf("invalid ranking cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || json.Unmarshal(raw, &cur) != nil || cur.Offset < 0 {
		return rankCursor{}, fmt.Errorf("invalid ranking cursor")
	}
	return cur, nil
}

func encodeRankCursor(cur rankCursor) (string, error) {
	raw, err := json.Marshal(cur)
	if err != nil {
		return "", err
	}
	return "s1." + base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Service) fetchAccountShortRank(ctx context.Context, contentId, boardId, cursor string) (*model.ShortRanking, error) {
	userCookies, userToken := s.credentials()
	cur, err := decodeRankCursor(cursor)
	if err != nil {
		return nil, err
	}
	if cur.Content != "" && contentId != "" && cur.Content != contentId {
		return nil, fmt.Errorf("ranking cursor belongs to another content")
	}
	if cur.Board != "" && boardId != "" && cur.Board != boardId {
		return nil, fmt.Errorf("ranking cursor belongs to another board")
	}
	if cur.Content != "" {
		contentId = cur.Content
	}
	if cur.Board != "" {
		boardId = cur.Board
	}

	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := [][2]string{
		{"iid", iid},
		{"device_id", deviceID},
		{"ac", "wifi"},
		{"channel", "xiaomi_8662_64"},
		{"aid", "8662"},
		{"app_name", "novelread"},
		{"cdid", cdid},
		{"version_code", "73932"},
		{"version_name", "7.3.9.32"},
		{"device_platform", "android"},
		{"os", "android"},
		{"ssmix", "a"},
		{"device_type", "23127PN0CC"},
		{"device_brand", "Xiaomi"},
		{"language", "zh"},
		{"os_api", "34"},
		{"os_version", "14"},
		{"manifest_version_code", "73932"},
		{"resolution", "1200*2670"},
		{"dpi", "460"},
		{"update_version_code", "73932"},
		{"host_abi", "arm64-v8a"},
		{"dragon_device_type", "phone"},
		{"pv_player", "73932"},
		{"compliance_status", "0"},
		{"need_personal_recommend", "1"},
		{"player_so_load", "1"},
		{"is_android_pad_screen", "0"},
		{"openudid", openudid},
		{"cell_id", "7470092475068071998"},
		{"tab_type", "26"},
		{"selected_items", contentId},
	}
	if boardId != "" {
		pairs = append(pairs, [2]string{"sub_selected_items", boardId})
	}
	pairs = append(pairs,
		[2]string{"offset", strconv.Itoa(cur.Offset)},
		[2]string{"limit", "0"},
		[2]string{"client_req_type", "2"},
		[2]string{"client_template", "2"},
		[2]string{"unlimited_selector_change_type", map[bool]string{true: "2", false: "1"}[cur.Offset > 0]},
	)
	if len(cur.Seen) > 0 && contentId != "ranklist_celebrity" {
		pairs = append(pairs, [2]string{"filter_ids", strings.Join(cur.Seen, ",")})
	}
	if cur.Session != "" {
		pairs = append(pairs, [2]string{"session_id", cur.Session})
	}
	pairs = append(pairs, [2]string{"user_is_login", "1"})

	ticket := time.Now().UnixMilli()
	rawQuery, sigHeaders := sign.SignGET(pairs, ticket, deviceID, "7.3.9.32", nil)
	reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/bookapi/bookmall/cell/change/v1/?" + rawQuery
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
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
	userAgent, storeRegion := s.runtime()
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("x-tt-store-region", storeRegion)
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
		return nil, fmt.Errorf("ranking API returned %d", resp.StatusCode)
	}
	return parseAccountShortRank(body, contentId, boardId, cur)
}

type shortRankID string

func (v *shortRankID) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		*v = ""
		return nil
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		*v = shortRankID(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return err
	}
	*v = shortRankID(number.String())
	return nil
}

type shortRankOption struct {
	Name     string      `json:"show_name"`
	ID       shortRankID `json:"selector_item_id"`
	Selected bool        `json:"is_selected"`
	Sub      struct {
		OuterRow struct {
			Items []shortRankOption `json:"items"`
		} `json:"outer_row"`
	} `json:"sub_cell_selector"`
}

func parseAccountShortRank(body []byte, contentId, boardId string, cur rankCursor) (*model.ShortRanking, error) {
	var env struct {
		Code json.RawMessage `json:"code"`
		Data struct {
			NextOffset int    `json:"next_offset"`
			HasMore    bool   `json:"has_more"`
			SessionID  string `json:"session_id"`
			CellView   struct {
				Selector struct {
					OuterRow struct {
						Items []shortRankOption `json:"items"`
					} `json:"outer_row"`
				} `json:"cell_selector"`
				CellData []struct {
					VideoData []struct {
						SeriesID     shortRankID `json:"series_id"`
						Title        string      `json:"title"`
						Cover        string      `json:"cover"`
						EpisodeCount int         `json:"episode_cnt"`
						Score        struct {
							Text string `json:"RecommendText"`
						} `json:"rec_text_item"`
					} `json:"video_data"`
					Users []struct {
						Info struct {
							Base struct {
								Name    string      `json:"user_name"`
								Avatar  string      `json:"user_avatar"`
								ActorID shortRankID `json:"actor_id_str"`
								UserID  shortRankID `json:"user_id"`
							} `json:"base_info"`
						} `json:"user_info"`
						Score string `json:"intro"`
						Rec   struct {
							Text string `json:"RecommendText"`
						} `json:"rec_text_item"`
						SubTitles []struct {
							Content string `json:"content"`
						} `json:"sub_title_list"`
					} `json:"ugc_user_data"`
				} `json:"cell_data"`
			} `json:"cell_view"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if code := strings.Trim(strings.TrimSpace(string(env.Code)), `"`); code != "" && code != "0" {
		return nil, fmt.Errorf("ranking API returned code %s", code)
	}

	out := &model.ShortRanking{Items: []model.RankItem{}}
	for _, item := range env.Data.CellView.Selector.OuterRow.Items {
		content := model.RankContent{ID: string(item.ID), Name: item.Name, Selected: item.Selected}
		for _, board := range item.Sub.OuterRow.Items {
			content.Boards = append(content.Boards, model.RankBoard{
				ID: string(board.ID), Name: board.Name, Selected: board.Selected,
			})
		}
		if content.ID != "" {
			out.Contents = append(out.Contents, content)
		}
	}

	for _, cell := range env.Data.CellView.CellData {
		for _, video := range cell.VideoData {
			if video.SeriesID == "" || strings.TrimSpace(video.Title) == "" {
				continue
			}
			out.Items = append(out.Items, model.RankItem{
				Position:     len(out.Items) + 1,
				SeriesID:     string(video.SeriesID),
				Title:        strings.TrimSpace(video.Title),
				CoverURL:     video.Cover,
				Score:        video.Score.Text,
				EpisodeCount: video.EpisodeCount,
			})
		}
		for _, user := range cell.Users {
			profileID := string(user.Info.Base.UserID)
			if profileID == "" {
				profileID = string(user.Info.Base.ActorID)
			}
			if profileID == "" || strings.TrimSpace(user.Info.Base.Name) == "" {
				continue
			}
			item := model.RankItem{
				Position:  len(out.Items) + 1,
				ActorID:   string(user.Info.Base.ActorID),
				UserID:    profileID,
				Actor:     strings.TrimSpace(user.Info.Base.Name),
				AvatarURL: user.Info.Base.Avatar,
				Score:     user.Rec.Text,
				Intro:     user.Score,
			}
			for _, subtitle := range user.SubTitles {
				if subtitle.Content != "" {
					item.Details = append(item.Details, subtitle.Content)
				}
			}
			out.Items = append(out.Items, item)
		}
	}

	for _, content := range out.Contents {
		if !content.Selected {
			continue
		}
		out.Content = content.ID
		for _, board := range content.Boards {
			if board.Selected {
				out.Board = board.ID
			}
		}
	}
	if out.Content == "" {
		out.Content = contentId
	}
	if out.Board == "" {
		out.Board = boardId
	}
	for i := range out.Items {
		out.Items[i].Position += cur.Offset
	}
	if env.Data.HasMore && env.Data.NextOffset > cur.Offset && len(out.Items) > 0 {
		next := cur
		next.Offset = env.Data.NextOffset
		next.Session = env.Data.SessionID
		next.Content = out.Content
		next.Board = out.Board
		for _, item := range out.Items {
			id := item.SeriesID
			if id == "" {
				id = item.ActorID
			}
			if id != "" {
				next.Seen = append(next.Seen, id)
			}
		}
		out.Cursor, _ = encodeRankCursor(next)
		out.HasMore = out.Cursor != ""
	}
	return out, nil
}

func shortRankCursor(rank *model.ShortRanking) string {
	if rank == nil {
		return ""
	}
	return rank.Cursor
}

func shortRankHasMore(rank *model.ShortRanking) bool {
	return rank != nil && rank.HasMore && rank.Cursor != ""
}

func formatHeatScore(n int64) string {
	return FormatHeatScore(n)
}

func FormatHeatScore(n int64) string {
	if n >= 100000000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/100000000.0), ".0") + "亿"
	} else if n >= 10000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/10000.0), ".0") + "万"
	}
	return strconv.FormatInt(n, 10)
}

func (s *Service) GetActorProfile(ctx context.Context, userID string) (*ActorProfileDTO, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	cacheKey := "actor_profile:" + userID
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.val.(*ActorProfileDTO), nil
	}
	s.cacheMu.RUnlock()

	// A 12-second preview can take longer than ten seconds on a cold network;
	// killing ffmpeg at exactly ten seconds turns a slow preload into a
	// playback error.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	person, err := s.client.Person(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch actor profile: %w", err)
	}
	if person == nil {
		return nil, fmt.Errorf("actor profile not found")
	}

	worksPage, err := s.client.PersonWorks(ctx, userID, "")
	var works []SeriesCard
	hasMore := false
	cursor := ""
	if err == nil && worksPage != nil {
		hasMore = worksPage.HasMore
		cursor = worksPage.Cursor
		for _, w := range worksPage.Items {
			category := "短剧"
			if len(w.Tags) > 0 {
				category = w.Tags[0]
			}
			heat := ""
			if w.Heat > 0 {
				heat = formatHeatScore(w.Heat) + "热度"
			} else if w.Plays > 0 {
				heat = formatHeatScore(w.Plays) + "播放"
			} else {
				heat = "热播作品"
			}
			works = append(works, SeriesCard{
				ID:           w.SeriesID,
				Title:        w.Title,
				Description:  w.Description,
				CoverURL:     s.imageURL(w.CoverURL),
				EpisodeCount: w.EpisodeCount,
				Category:     category,
				Author:       person.Name,
				Status:       "finished",
				Heat:         heat,
				Tags:         w.Tags,
			})
		}
	}

	dto := &ActorProfileDTO{
		UserID:      person.UserID,
		Name:        person.Name,
		Description: person.Description,
		AvatarURL:   s.imageURL(person.AvatarURL),
		Tags:        person.Tags,
		Labels:      person.Labels,
		Followers:   person.Followers,
		Likes:       person.Likes,
		Works:       works,
		HasMore:     hasMore,
		Cursor:      cursor,
	}
	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: dto, expiresAt: time.Now().Add(10 * time.Minute)}
	s.cacheMu.Unlock()

	return dto, nil
}

func (s *Service) GetActorWorks(ctx context.Context, userID, cursor string) (*ActorWorksResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	worksPage, err := s.client.PersonWorks(ctx, userID, cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch actor works: %w", err)
	}
	worksPageItems := workItems(worksPage)

	var works []SeriesCard
	for _, w := range worksPageItems {
		category := "短剧"
		if len(w.Tags) > 0 {
			category = w.Tags[0]
		}
		heat := ""
		if w.Heat > 0 {
			heat = formatHeatScore(w.Heat) + "热度"
		} else if w.Plays > 0 {
			heat = formatHeatScore(w.Plays) + "播放"
		} else {
			heat = "热播作品"
		}
		works = append(works, SeriesCard{
			ID:           w.SeriesID,
			Title:        w.Title,
			Description:  w.Description,
			CoverURL:     s.imageURL(w.CoverURL),
			EpisodeCount: w.EpisodeCount,
			Category:     category,
			Status:       "finished",
			Heat:         heat,
			Tags:         w.Tags,
		})
	}

	return &ActorWorksResponse{
		Works:   works,
		HasMore: worksPage != nil && worksPage.HasMore,
		Cursor:  workCursor(worksPage),
	}, nil
}

func (s *Service) GetNewReleases(ctx context.Context, date string) (NewReleaseResponse, error) {
	cacheKey := "new_releases:" + date
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.val.(NewReleaseResponse), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	nr, err := s.client.NewReleases(ctx, sdk.NewReleaseQuery{
		Date: date,
	})
	if err != nil {
		return NewReleaseResponse{}, err
	}

	reservedIDs := make(map[string]bool)
	if ids, err := s.reservedSeriesIDs(ctx); err == nil {
		for _, id := range ids {
			reservedIDs[id] = true
		}
	}

	var items []NewReleaseItemDTO
	for _, it := range newReleaseItems(nr) {
		cat := ""
		if len(it.Categories) > 0 {
			cat = it.Categories[0]
		}
		items = append(items, NewReleaseItemDTO{
			SeriesID:     it.SeriesID,
			Title:        it.Title,
			CoverURL:     s.imageURL(it.CoverURL),
			EpisodeCount: it.EpisodeCount,
			Online:       it.Online,
			Heat:         it.Heat,
			Note:         it.Note,
			Category:     cat,
			Reserved:     reservedIDs[it.SeriesID],
		})
	}

	resp := NewReleaseResponse{
		CurrentDate: newReleaseDate(nr),
		Dates:       newReleaseDates(nr),
		Items:       items,
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: resp, expiresAt: time.Now().Add(10 * time.Minute)}
	s.cacheMu.Unlock()

	return resp, nil
}

func (s *Service) Search(ctx context.Context, query string) ([]SeriesCard, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	res, err := s.client.Search(ctx, query, sdk.SearchOptions{})
	if err != nil {
		return nil, err
	}

	var cards []SeriesCard
	var missingCovers []int
	for _, it := range pageItems(res) {
		var tags []string
		if it.Category != "" {
			tags = append(tags, it.Category)
		}
		tags = append(tags, fmt.Sprintf("%d集", it.EpisodeCount))

		coverURL := s.imageURL(it.CoverURL)
		if coverURL == "" {
			missingCovers = append(missingCovers, len(cards))
		}
		cards = append(cards, SeriesCard{
			ID:           it.ID,
			Title:        it.Title,
			Description:  it.Description,
			CoverURL:     coverURL,
			EpisodeCount: it.EpisodeCount,
			Category:     it.Category,
			Author:       it.Author,
			Status:       "finished",
			Heat:         "搜索结果",
			Tags:         tags,
		})
	}

	// The search API omits series_cover for some results; batch-fetch
	// details to fill in the covers rather than rendering placeholders.
	if len(missingCovers) > 0 {
		ids := make([]string, 0, len(missingCovers))
		for _, idx := range missingCovers {
			ids = append(ids, cards[idx].ID)
		}
		bCtx, bCancel := context.WithTimeout(ctx, 8*time.Second)
		batch, bErr := s.client.GetSeriesBatch(bCtx, ids)
		bCancel()
		if bErr == nil {
			coverByID := make(map[string]string, len(batch))
			for _, b := range batch {
				if b.Series != nil && b.Series.CoverURL != "" {
					coverByID[b.Series.ID] = b.Series.CoverURL
				}
			}
			for _, idx := range missingCovers {
				if url, ok := coverByID[cards[idx].ID]; ok {
					cards[idx].CoverURL = s.imageURL(url)
				}
			}
		}
	}
	return cards, nil
}

func (s *Service) GetSuggestions(ctx context.Context, prefix string) ([]string, error) {
	if prefix == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	suggs, err := s.client.Suggest(ctx, prefix)
	if err != nil {
		return nil, err
	}
	var words []string
	for _, s := range suggs {
		words = append(words, s.Text)
	}
	return words, nil
}

func (s *Service) GetHotSearch(ctx context.Context) (HotSearchDTO, error) {
	cacheKey := "hot_search"
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.val.(HotSearchDTO), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hot, err := s.client.HotSearch(ctx)
	if err != nil {
		return HotSearchDTO{}, err
	}
	var words []string
	for _, g := range hotGuesses(hot) {
		if g.Title != "" {
			words = append(words, g.Title)
		}
	}
	for _, b := range hotBoards(hot) {
		for _, it := range b.Items {
			if it.Title != "" {
				words = append(words, it.Title)
			}
		}
	}
	resp := HotSearchDTO{Words: words}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: resp, expiresAt: time.Now().Add(10 * time.Minute)}
	s.cacheMu.Unlock()

	return resp, nil
}

func (s *Service) GetSeriesDetail(ctx context.Context, seriesID string) (*SeriesDetailDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	d, err := s.client.GetSeries(ctx, seriesID)
	if err != nil {
		if errors.Is(err, sdk.ErrAnonymousLimited) || strings.Contains(strings.ToLower(err.Error()), "anonymous") {
			_ = s.client.ResetDevice(ctx)
			d, err = s.client.GetSeries(ctx, seriesID)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch series detail: %w", err)
	}
	if d == nil {
		return nil, fmt.Errorf("series not found")
	}

	eps, err := s.episodes(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch episodes: %w", err)
	}

	var episodeDTOs []EpisodeDTO
	var commentCount int64
	for _, ep := range pageItems(eps) {
		episodeDTOs = append(episodeDTOs, EpisodeDTO{
			ID:           ep.ID,
			SeriesID:     seriesID,
			Number:       ep.Number,
			Title:        ep.Title,
			Playable:     true,
			PosterURL:    s.imageURL(ep.CoverURL),
			CommentCount: ep.CommentCount,
		})
		commentCount += ep.CommentCount
	}

	return &SeriesDetailDTO{
		ID:           d.ID,
		Title:        d.Title,
		Description:  d.Description,
		CoverURL:     s.imageURL(d.CoverURL),
		EpisodeCount: d.EpisodeCount,
		Status:       "finished",
		Heat:         "热播",
		Episodes:     episodeDTOs,
		CommentCount: commentCount,
		IsFavorite:   s.isFavorite(seriesID),
	}, nil
}
