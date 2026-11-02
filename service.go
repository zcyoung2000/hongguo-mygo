package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/sign"
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
	Status       string   `json:"status"` // "updating" or "finished"
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

type RankTabDTO struct {
	ContentID string `json:"contentId"`
	BoardID   string `json:"boardId"`
	Title     string `json:"title"`
	IsDefault bool   `json:"isDefault"`
}

type RankItemDTO struct {
	Position    int      `json:"position"`
	SeriesID    string   `json:"seriesId"`
	Title       string   `json:"title"`
	CoverURL    string   `json:"coverUrl"`
	Heat        string   `json:"heat"`
	Description string   `json:"description"`
	ItemType    string   `json:"itemType"` // "drama" or "actor"
	Promoted    bool     `json:"promoted,omitempty"`
	Actor       string   `json:"actor,omitempty"`
	ActorID     string   `json:"actorId,omitempty"`
	UserID      string   `json:"userId,omitempty"`
	AvatarURL   string   `json:"avatarUrl,omitempty"`
	Intro       string   `json:"intro,omitempty"`
	Details     []string `json:"details,omitempty"`
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

type PlaybackDTO struct {
	SeriesID             string             `json:"seriesId"`
	EpisodeID            string             `json:"episodeId"`
	EpisodeNum           int                `json:"episodeNum"`
	Title                string             `json:"title"`
	StreamURL            string             `json:"streamUrl"`
	FullStreamURL        string             `json:"fullStreamUrl,omitempty"`
	ProgressiveStreamURL string             `json:"progressiveStreamUrl,omitempty"`
	HLSStreamURL         string             `json:"hlsStreamUrl,omitempty"`
	PosterURL            string             `json:"posterUrl,omitempty"`
	DurationSec          int                `json:"durationSec"`
	LastPosition         float64            `json:"lastPosition"`
	IsLiked              bool               `json:"isLiked"`
	CommentCount         int64              `json:"commentCount"`
	Quality              string             `json:"quality,omitempty"`
	Qualities            []QualityOptionDTO `json:"qualities,omitempty"`
	IsFullCached         bool               `json:"isFullCached"`
}

type QualityOptionDTO struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Codec     string `json:"codec,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Bitrate   int64  `json:"bitrate,omitempty"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
}

type MediaDiagnosticsDTO struct {
	NetworkProfile     string `json:"networkProfile"`
	NetworkBytesPerSec int64  `json:"networkBytesPerSec"`
	NetworkFailures    int64  `json:"networkFailures"`
	PlaybackCacheHits  int64  `json:"playbackCacheHits"`
	PlaybackCacheMiss  int64  `json:"playbackCacheMiss"`
	ProgressiveStarts  int64  `json:"progressiveStarts"`
	ProgressiveHits    int64  `json:"progressiveHits"`
	HLSStarts          int64  `json:"hlsStarts"`
	HLSHits            int64  `json:"hlsHits"`
	FullDownloads      int64  `json:"fullDownloads"`
	PrefetchCompleted  int64  `json:"prefetchCompleted"`
	PrefetchFailed     int64  `json:"prefetchFailed"`
	PrefetchCanceled   int64  `json:"prefetchCanceled"`
	PrefetchDropped    int64  `json:"prefetchDropped"`
}

type FeedMetricDTO struct {
	Type                string `json:"type"`
	FromIndex           int    `json:"fromIndex,omitempty"`
	ToIndex             int    `json:"toIndex,omitempty"`
	BufferedAtSwitch    bool   `json:"bufferedAtSwitch"`
	DurationMS          int64  `json:"durationMs"`
	TimeToFirstFrameMS  int64  `json:"timeToFirstFrameMs,omitempty"`
	HandoffFirstFrameMS int64  `json:"handoffFirstFrameMs,omitempty"`
	FirstFrameSource    string `json:"firstFrameSource,omitempty"`
	SwitchReadyMS       int64  `json:"switchReadyMs,omitempty"`
	RebufferCount       int    `json:"rebufferCount,omitempty"`
	RebufferMS          int64  `json:"rebufferMs,omitempty"`
	NetworkProfile      string `json:"networkProfile,omitempty"`
	NetworkBytesPerSec  int64  `json:"networkBytesPerSec,omitempty"`
	NetworkFailures     int64  `json:"networkFailures,omitempty"`
	PrefetchCompleted   int64  `json:"prefetchCompleted,omitempty"`
	PrefetchFailed      int64  `json:"prefetchFailed,omitempty"`
	PrefetchCanceled    int64  `json:"prefetchCanceled,omitempty"`
	PrefetchDropped     int64  `json:"prefetchDropped,omitempty"`
	HLSStarts           int64  `json:"hlsStarts,omitempty"`
	HLSHits             int64  `json:"hlsHits,omitempty"`
	StreamMode          string `json:"streamMode,omitempty"`
	CacheHit            bool   `json:"cacheHit"`
	FullFallback        bool   `json:"fullFallback"`
	Quality             string `json:"quality,omitempty"`
	At                  string `json:"at"`
}

type QualitySelection struct {
	Options     []QualityOptionDTO `json:"options"`
	Selected    string             `json:"selected"`
	DurationSec int                `json:"-"`
}

type cacheItem struct {
	val       any
	expiresAt time.Time
}

type episodeCacheEntry struct {
	page      *sdk.Page[model.Episode]
	expiresAt time.Time
}

type episodeLoad struct {
	done chan episodeListResult
}

type episodeListResult struct {
	page *sdk.Page[model.Episode]
	err  error
}

type HongguoService struct {
	client                 *sdk.Client
	store                  *Store
	media                  *MediaServer
	appDir                 string
	deviceProfileStore     sdk.ProfileStore
	authTransport          authCredentialTransport
	deviceMu               sync.RWMutex
	authMu                 sync.RWMutex
	userCookies            string
	userToken              string
	deviceID               string
	iid                    string
	cdid                   string
	openudid               string
	userAgent              string
	storeRegion            string
	httpClient             *http.Client
	cacheMu                sync.RWMutex
	cache                  map[string]cacheItem
	historyMu              sync.Mutex
	cachedHistory          []HistoryItem
	historyCachedAt        time.Time
	historySyncing         bool
	historyGeneration      uint64
	historyMutationMu      sync.Mutex
	historyMutationSyncing bool
	historyMutationWake    bool
	favMu                  sync.Mutex
	cachedFavorites        []FavoriteItem
	favCachedAt            time.Time
	favSyncing             bool
	favGeneration          uint64
	favMutationMu          sync.Mutex
	likeMu                 sync.Mutex
	cachedLikes            []LikeItem
	likeCachedAt           time.Time
	likeSyncing            bool
	likeGeneration         uint64
	resMu                  sync.Mutex
	cachedReservations     []ReservationItem
	resCachedAt            time.Time
	resSyncing             bool
	resGeneration          uint64
	feedMu                 sync.Mutex
	metricsMu              sync.Mutex
	feedWarmupMu           sync.Mutex
	feedWarmupGeneration   uint64
	episodeMu              sync.Mutex
	episodeCache           map[string]episodeCacheEntry
	episodeLoads           map[string]*episodeLoad
}

func NewHongguoService(client *sdk.Client, store *Store, media *MediaServer, appDir string) *HongguoService {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	svc := &HongguoService{
		client:       client,
		store:        store,
		media:        media,
		appDir:       appDir,
		deviceID:     "4210501433995451",
		iid:          "4210501433184459",
		cdid:         "ad52a07c-7c54-41eb-8b47-750f78610278",
		openudid:     "dcffb1368df39e9b",
		userAgent:    "com.phoenix.read/73932 (Linux; U; Android 14; zh_CN; Xiaomi 14; Build/UKQ1.230804.001; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)",
		storeRegion:  "cn-gd",
		httpClient:   &http.Client{Transport: tr, Timeout: 12 * time.Second},
		cache:        make(map[string]cacheItem),
		episodeCache: make(map[string]episodeCacheEntry),
		episodeLoads: make(map[string]*episodeLoad),
	}
	home, _ := os.UserHomeDir()
	// 1. Try loading account.json from appDir or fallback to hgplayer
	accPaths := []string{
		filepath.Join(appDir, "account.json"),
		filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json"),
	}
	for _, p := range accPaths {
		if data, err := os.ReadFile(p); err == nil {
			var acc struct {
				Cookies string `json:"cookies"`
				Token   string `json:"token"`
				Profile struct {
					UserID   string `json:"userId"`
					Nickname string `json:"nickname"`
					Avatar   string `json:"avatar"`
					LoggedIn bool   `json:"loggedIn"`
				} `json:"profile"`
			}
			if err := json.Unmarshal(data, &acc); err == nil && acc.Cookies != "" {
				svc.setAuthCredentials(acc.Cookies, acc.Token)
				if acc.Profile.Nickname != "" {
					p := svc.store.GetProfile()
					p.Nickname = acc.Profile.Nickname
					p.UserID = acc.Profile.UserID
					p.Avatar = acc.Profile.Avatar
					p.LoggedIn = true
					_ = svc.store.UpdateProfile(p)
				}
				break
			}
		}
	}

	// 2. Try loading anonymous.json / device.json for matching device credentials
	anonPaths := []string{
		filepath.Join(appDir, "anonymous.json"),
		filepath.Join(home, "Library", "Application Support", "hgplayer", "anonymous.json"),
	}
	for _, p := range anonPaths {
		if data, err := os.ReadFile(p); err == nil {
			var anon struct {
				DeviceID  string `json:"device_id_str"`
				InstallID string `json:"install_id_str"`
				CDID      string `json:"cdid"`
				OpenUDID  string `json:"openudid"`
			}
			if err := json.Unmarshal(data, &anon); err == nil && anon.DeviceID != "" {
				svc.deviceID = anon.DeviceID
				svc.iid = anon.InstallID
				svc.cdid = anon.CDID
				svc.openudid = anon.OpenUDID
				break
			}
		}
	}

	devPaths := []string{
		filepath.Join(appDir, "device.json"),
		filepath.Join(home, "Library", "Application Support", "hgplayer", "device.json"),
	}
	for _, p := range devPaths {
		if data, err := os.ReadFile(p); err == nil {
			var dev struct {
				DeviceID    string `json:"device_id"`
				IID         string `json:"iid"`
				UserAgent   string `json:"user_agent"`
				StoreRegion string `json:"store_region"`
			}
			if err := json.Unmarshal(data, &dev); err == nil {
				if dev.UserAgent != "" {
					svc.userAgent = dev.UserAgent
				}
				if dev.StoreRegion != "" {
					svc.storeRegion = dev.StoreRegion
				}
				break
			}
		}
	}

	return svc
}

func (s *HongguoService) SetDeviceProfileStore(store sdk.ProfileStore) {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	s.deviceProfileStore = store
}

func (s *HongguoService) SetAuthTransport(transport authCredentialTransport) {
	s.authTransport = transport
	s.syncAuthTransport()
}

func (s *HongguoService) deviceIdentity() (deviceID, iid, cdid, openudid string) {
	s.deviceMu.RLock()
	defer s.deviceMu.RUnlock()
	return s.deviceID, s.iid, s.cdid, s.openudid
}

func (s *HongguoService) authCredentials() (cookies, token string) {
	s.authMu.RLock()
	defer s.authMu.RUnlock()
	return s.userCookies, s.userToken
}

func (s *HongguoService) setAuthCredentials(cookies, token string) {
	previousCookies, _ := s.authCredentials()
	s.authMu.Lock()
	s.userCookies = cookies
	s.userToken = token
	s.authMu.Unlock()
	if previousCookies != cookies && s.media != nil {
		// Playback entitlements can change with the account; never reuse a
		// playback negotiated under the previous credential state.
		s.media.ClearPlaybackCache()
	}
	s.syncAuthTransport()
}

func (s *HongguoService) syncAuthTransport() {
	if s.authTransport == nil {
		return
	}
	cookies, token := s.authCredentials()
	s.authTransport.setCredentials(cookies, token, s.storeRegion)
}

func (s *HongguoService) reloadDeviceIdentity() error {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	return s.reloadDeviceIdentityLocked()
}

func (s *HongguoService) reloadDeviceIdentityLocked() error {
	if s.deviceProfileStore == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	profile, err := s.deviceProfileStore.Load(ctx)
	if err != nil {
		return err
	}
	if profile == nil || profile.DeviceID == "" || profile.InstallID == "" {
		return fmt.Errorf("device profile has no identity")
	}
	s.deviceID = profile.DeviceID
	s.iid = profile.InstallID
	s.cdid = profile.CDID
	s.openudid = profile.OpenUDID
	return nil
}

func (s *HongguoService) invalidateUserDataCaches() {
	s.cacheMu.Lock()
	s.cache = make(map[string]cacheItem)
	s.cacheMu.Unlock()
	s.historyMu.Lock()
	s.cachedHistory = nil
	s.historyCachedAt = time.Time{}
	s.historyGeneration++
	s.historyMu.Unlock()
	s.favMu.Lock()
	s.cachedFavorites = nil
	s.favCachedAt = time.Time{}
	s.favGeneration++
	s.favMu.Unlock()
	s.likeMu.Lock()
	s.cachedLikes = nil
	s.likeCachedAt = time.Time{}
	s.likeGeneration++
	s.likeMu.Unlock()
	s.resMu.Lock()
	s.cachedReservations = nil
	s.resCachedAt = time.Time{}
	s.resGeneration++
	s.resMu.Unlock()
}

func (s *HongguoService) currentHistoryGeneration() uint64 {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	return s.historyGeneration
}

func (s *HongguoService) currentFavoriteGeneration() uint64 {
	s.favMu.Lock()
	defer s.favMu.Unlock()
	return s.favGeneration
}

func (s *HongguoService) currentLikeGeneration() uint64 {
	s.likeMu.Lock()
	defer s.likeMu.Unlock()
	return s.likeGeneration
}

func (s *HongguoService) currentReservationGeneration() uint64 {
	s.resMu.Lock()
	defer s.resMu.Unlock()
	return s.resGeneration
}

func (s *HongguoService) GetProfile() (UserProfile, error) {
	p := s.store.GetProfile()
	if p.Avatar != "" {
		p.Avatar = s.media.ImageURL(p.Avatar)
	}
	return sanitizeIPC(p), nil
}

func (s *HongguoService) SetFeedScrollCapture(enabled bool) error {
	feedScrollCapture.Store(enabled)
	return nil
}

type RecommendFeedResult struct {
	Items   []SeriesCard `json:"items"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"hasMore"`
}

func (s *HongguoService) GetRecommendFeed(channel, cursor string) (*RecommendFeedResult, error) {
	// A one-page official feed can contain only two or three dramas. Start with
	// a small aggregate so pagination never begins already behind one swipe.
	return s.getRecommendFeed(channel, cursor, 6)
}

func (s *HongguoService) GetRecommendFeedBatch(channel, cursor string, minimumItems int) (*RecommendFeedResult, error) {
	return s.getRecommendFeed(channel, cursor, minimumItems)
}

func (s *HongguoService) getRecommendFeed(channel, cursor string, minimumItems int) (*RecommendFeedResult, error) {
	timeout := 30 * time.Second
	if minimumItems > 1 {
		// Background swipe-buffer fills should never hold the feed cursor lock
		// for the full foreground request budget.
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if channel == "关注" {
		favorites, err := s.GetFavorites()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch followed dramas: %w", err)
		}
		cards := make([]SeriesCard, 0, len(favorites))
		for _, fav := range favorites {
			if !validContentID(fav.SeriesID) {
				continue
			}
			cards = append(cards, SeriesCard{
				ID:           fav.SeriesID,
				Title:        fav.Title,
				Description:  fav.Description,
				CoverURL:     fav.CoverURL,
				EpisodeCount: fav.TotalEpisodes,
				Category:     "关注",
				Heat:         "已关注",
				Tags:         []string{"关注"},
			})
		}
		return sanitizeIPC(&RecommendFeedResult{Items: cards}), nil
	}

	// Serialize cursor updates so concurrent preload requests cannot advance
	// the same channel twice.
	s.feedMu.Lock()
	defer s.feedMu.Unlock()

	feedKey := channel
	if feedKey == "" {
		feedKey = "推荐"
	}

	progress, _ := s.store.GetFeedProgress(feedKey)
	queryCursor := cursor
	if queryCursor == "refresh" || queryCursor == "next" || queryCursor == "" {
		// The persisted opaque cursor survives app restarts. An empty request
		// therefore means "continue this channel", not "restart at zero".
		queryCursor = progress.Cursor
	}
	generation := progress.Generation
	if generation <= 0 {
		generation = 1
	}

	// Feed deduplication is generation-scoped. Filtering by the entire watch
	// history made an official infinite feed look exhausted after enough
	// viewing; `feed_seen` matches the app's per-session recommendation stream.
	played := s.store.GetFeedSeenIDs(feedKey, generation)
	rememberSeenPage := func(page *sdk.Page[model.SeriesSummary]) {
		ids := make([]string, 0, len(pageItems(page)))
		for _, item := range pageItems(page) {
			ids = append(ids, item.ID)
		}
		_ = s.store.SaveFeedSeenIDs(feedKey, generation, ids)
	}
	startGeneration := func() (int64, error) {
		next, err := s.store.StartNextFeedGeneration(feedKey)
		if next <= 0 {
			next = 1
		}
		generation = next
		played = s.store.GetFeedSeenIDs(feedKey, generation)
		return generation, err
	}

	discoverOnce := func(cur string) (*sdk.Page[model.SeriesSummary], string, error) {
		p, err := s.client.Discover(ctx, sdk.DiscoverOptions{
			Channel: channel,
			Cursor:  cur,
		})
		if err != nil && (errors.Is(err, sdk.ErrAnonymousLimited) || strings.Contains(strings.ToLower(err.Error()), "anonymous")) {
			// A cursor belongs to the anonymous session. Resetting the device
			// therefore also invalidates it; continue from a fresh stream.
			_ = s.client.ResetDevice(ctx)
			cur = ""
			p, err = s.client.Discover(ctx, sdk.DiscoverOptions{
				Channel: channel,
				Cursor:  cur,
			})
			return p, cur, err
		}
		return p, cur, err
	}

	cardify := func(items []model.SeriesSummary) []SeriesCard {
		cards := make([]SeriesCard, 0, len(items))
		for _, it := range items {
			tags := make([]string, 0, 2)
			if it.Category != "" {
				tags = append(tags, it.Category)
			}
			if it.EpisodeCount > 0 {
				tags = append(tags, fmt.Sprintf("%d集", it.EpisodeCount))
			}
			cards = append(cards, SeriesCard{
				ID:           it.ID,
				Title:        it.Title,
				Description:  it.Description,
				CoverURL:     s.media.ImageURL(it.CoverURL),
				EpisodeCount: it.EpisodeCount,
				Category:     it.Category,
				Author:       it.Author,
				Status:       "finished",
				Heat:         "热播中",
				Tags:         tags,
			})
		}
		return cards
	}

	savePage := func(page *sdk.Page[model.SeriesSummary]) {
		_ = s.store.SaveFeedProgress(feedKey, page.Cursor, generation)
	}

	// The official cell page can contain only two to four dramas. Aggregate a
	// swipe buffer in one IPC call so rapid vertical navigation never waits for
	// a second page request at the list boundary.
	const maxLookAheadPages = 5
	minRecommendFeedBatch := minimumItems
	if minRecommendFeedBatch < 1 {
		minRecommendFeedBatch = 1
	}
	cards := make([]SeriesCard, 0, minRecommendFeedBatch)
	batchSeen := make(map[string]bool, minRecommendFeedBatch)
	resultCursor := queryCursor
	// The main recommendation stream is effectively unbounded. Even when one
	// official page reports no follow-up, the next generation can yield fresh
	// cards; only the followed-dramas channel has a true hard boundary.
	const resultHasMore = true
	for attempt := 0; attempt < maxLookAheadPages; attempt++ {
		page, effectiveCursor, err := discoverOnce(queryCursor)
		if err != nil {
			if queryCursor != "" {
				// An expired cursor is not fatal. Restart the same generation
				// at zero; history filtering still protects the user.
				_, _ = startGeneration()
				queryCursor = ""
				continue
			}
			return nil, fmt.Errorf("failed to fetch recommend feed: %w", err)
		}
		if page == nil || len(page.Items) == 0 {
			if queryCursor != "" {
				_, _ = startGeneration()
				queryCursor = ""
				continue
			}
			break
		}

		if effectiveCursor != queryCursor {
			// The SDK recovered by resetting the anonymous session. Start a new
			// generation so cursor bookkeeping matches the new fingerprint.
			_, _ = startGeneration()
		}

		savePage(page)
		rememberSeenPage(page)
		resultCursor = page.Cursor
		unseen := make([]model.SeriesSummary, 0, len(page.Items))
		for _, item := range page.Items {
			if !played[item.ID] && !batchSeen[item.ID] {
				unseen = append(unseen, item)
			}
		}
		if len(unseen) > 0 {
			for _, item := range unseen {
				played[item.ID] = true
				batchSeen[item.ID] = true
			}
			cards = append(cards, cardify(unseen)...)
		}
		if len(cards) >= minRecommendFeedBatch {
			return sanitizeIPC(&RecommendFeedResult{
				Items:   cards,
				Cursor:  page.Cursor,
				HasMore: resultHasMore,
			}), nil
		}
		if !page.HasMore || page.Cursor == "" {
			break
		}
		queryCursor = page.Cursor
	}
	if len(cards) > 0 {
		return sanitizeIPC(&RecommendFeedResult{
			Items:   cards,
			Cursor:  resultCursor,
			HasMore: resultHasMore,
		}), nil
	}

	// The anonymous stream has ended (or only contains played/replayed cards).
	// Register a new anonymous fingerprint and request a fresh stream.
	_, err := startGeneration()
	if err != nil {
		return nil, fmt.Errorf("failed to start a fresh recommend feed: %w", err)
	}
	if err := s.client.ResetDevice(ctx); err != nil {
		// Discover() can also recover from a rejected profile. Continue so a
		// transient registration failure does not make the feed disappear.
		_ = err
	}
	queryCursor = ""

	cards = make([]SeriesCard, 0, minRecommendFeedBatch)
	resultCursor = ""
	for attempt := 0; attempt < 8; attempt++ {
		page, _, err := discoverOnce(queryCursor)
		if err != nil || page == nil || len(page.Items) == 0 {
			break
		}

		_ = s.store.SaveFeedProgress(feedKey, page.Cursor, generation)
		rememberSeenPage(page)
		resultCursor = page.Cursor

		unseen := make([]model.SeriesSummary, 0, len(page.Items))
		for _, item := range page.Items {
			if !played[item.ID] && !batchSeen[item.ID] {
				unseen = append(unseen, item)
			}
		}
		if len(unseen) > 0 {
			for _, item := range unseen {
				played[item.ID] = true
				batchSeen[item.ID] = true
			}
			cards = append(cards, cardify(unseen)...)
		}
		if len(cards) >= minRecommendFeedBatch {
			return sanitizeIPC(&RecommendFeedResult{
				Items:   cards,
				Cursor:  page.Cursor,
				HasMore: resultHasMore,
			}), nil
		}
		if !page.HasMore || page.Cursor == "" {
			break
		}
		queryCursor = page.Cursor
	}
	if len(cards) > 0 {
		return sanitizeIPC(&RecommendFeedResult{
			Items:   cards,
			Cursor:  resultCursor,
			HasMore: resultHasMore,
		}), nil
	}

	// An empty result from a recommendation stream is usually recoverable
	// (cursor expiry, anonymous registration, or a temporarily played-out
	// page). Keep the feed retryable instead of turning it into a hard end.
	return sanitizeIPC(&RecommendFeedResult{HasMore: true}), nil
}

func (s *HongguoService) GetRecommend() ([]SeriesCard, error) {
	cacheKey := "recommend"
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.([]SeriesCard)), nil
	}
	s.cacheMu.RUnlock()

	res, err := s.GetRecommendFeed("", "")
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: res.Items, expiresAt: time.Now().Add(5 * time.Minute)}
	s.cacheMu.Unlock()

	return res.Items, nil
}

// listEpisodesCached deduplicates concurrent SDK calls and avoids repeating the
// same series-detail request while a feed item is played and preloaded.
func (s *HongguoService) listEpisodesCached(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error) {
	const episodeCacheTTL = 5 * time.Minute

	s.episodeMu.Lock()
	if s.episodeCache == nil {
		s.episodeCache = make(map[string]episodeCacheEntry)
	}
	if s.episodeLoads == nil {
		s.episodeLoads = make(map[string]*episodeLoad)
	}
	if entry, ok := s.episodeCache[seriesID]; ok && time.Now().Before(entry.expiresAt) {
		s.episodeMu.Unlock()
		return entry.page, nil
	}
	if load, ok := s.episodeLoads[seriesID]; ok {
		done := load.done
		s.episodeMu.Unlock()
		result := <-done
		return result.page, result.err
	}
	load := &episodeLoad{done: make(chan episodeListResult, 1)}
	s.episodeLoads[seriesID] = load
	s.episodeMu.Unlock()

	page, err := s.client.ListEpisodes(ctx, seriesID, sdk.EpisodeOptions{})
	s.episodeMu.Lock()
	delete(s.episodeLoads, seriesID)
	if err == nil && page != nil && len(page.Items) > 0 {
		s.episodeCache[seriesID] = episodeCacheEntry{page: page, expiresAt: time.Now().Add(episodeCacheTTL)}
	}
	s.episodeMu.Unlock()

	load.done <- episodeListResult{page: page, err: err}
	close(load.done)
	return page, err
}

func (s *HongguoService) listEpisodes(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error) {
	eps, err := s.listEpisodesCached(ctx, seriesID)
	if err != nil && (errors.Is(err, sdk.ErrAnonymousLimited) || strings.Contains(strings.ToLower(err.Error()), "anonymous")) {
		_ = s.client.ResetDevice(ctx)
		eps, err = s.listEpisodesCached(ctx, seriesID)
	}
	return eps, err
}

func pageItems[T any](page *sdk.Page[T]) []T {
	if page == nil {
		return nil
	}
	return page.Items
}

func pageCursor[T any](page *sdk.Page[T]) string {
	if page == nil {
		return ""
	}
	return page.Cursor
}

func pageHasMore[T any](page *sdk.Page[T]) bool {
	return page != nil && page.HasMore
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

func (s *HongguoService) GetFilterPanel(genre string) ([]FilterRowDTO, error) {
	cacheKey := "filter_panel:" + genre
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.([]FilterRowDTO)), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
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

	return sanitizeIPC(res), nil
}

func (s *HongguoService) GetDiscover(genre string, selections map[string]string, cursor string) (DiscoverResponse, error) {
	selectionJSON, err := json.Marshal(selections)
	if err != nil {
		return DiscoverResponse{}, fmt.Errorf("encode discover selections: %w", err)
	}
	cacheKey := fmt.Sprintf("discover:%s:%s:%s", genre, selectionJSON, cursor)
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.(DiscoverResponse)), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
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
			return sanitizeIPC(resp), nil
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
			CoverURL:     s.media.ImageURL(it.CoverURL),
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

	return sanitizeIPC(resp), nil
}

func (s *HongguoService) GetRankings(contentId, boardId, cursor string) (RankResponse, error) {
	userCookies, _ := s.authCredentials()
	rankScope := "anonymous"
	if userCookies != "" {
		rankScope = "account"
	}
	cacheKey := fmt.Sprintf("rank:%s:%s:%s:%s", rankScope, contentId, boardId, cursor)
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.(RankResponse)), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
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
			avatar := s.media.ImageURL(it.AvatarURL)
			if avatar == "" && it.CoverURL != "" {
				avatar = s.media.ImageURL(it.CoverURL)
			}
			desc := it.Intro
			if desc == "" && len(it.Details) > 0 {
				desc = strings.Join(it.Details, " · ")
			}
			items = append(items, RankItemDTO{
				Position:    it.Position,
				SeriesID:    it.SeriesID,
				Title:       title,
				CoverURL:    avatar,
				Heat:        it.Score,
				Description: desc,
				ItemType:    "actor",
				Promoted:    it.Promoted,
				Actor:       it.Actor,
				ActorID:     it.ActorID,
				UserID:      it.UserID,
				AvatarURL:   avatar,
				Intro:       it.Intro,
				Details:     it.Details,
			})
		} else {
			desc := ""
			if it.EpisodeCount > 0 {
				desc = fmt.Sprintf("全%d集", it.EpisodeCount)
			} else if it.Actor != "" {
				desc = it.Actor
			}
			items = append(items, RankItemDTO{
				Position:    it.Position,
				SeriesID:    it.SeriesID,
				Title:       it.Title,
				CoverURL:    s.media.ImageURL(it.CoverURL),
				Heat:        it.Score,
				Description: desc,
				ItemType:    "drama",
				Promoted:    it.Promoted,
				Actor:       it.Actor,
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

	return sanitizeIPC(resp), nil
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

func (s *HongguoService) fetchAccountShortRank(ctx context.Context, contentId, boardId, cursor string) (*model.ShortRanking, error) {
	userCookies, userToken := s.authCredentials()
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
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("x-tt-store-region", s.storeRegion)
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
			id := string(user.Info.Base.ActorID)
			if id == "" {
				id = string(user.Info.Base.UserID)
			}
			if id == "" || strings.TrimSpace(user.Info.Base.Name) == "" {
				continue
			}
			item := model.RankItem{
				Position:  len(out.Items) + 1,
				ActorID:   id,
				UserID:    string(user.Info.Base.UserID),
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
	if n >= 100000000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/100000000.0), ".0") + "亿"
	} else if n >= 10000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/10000.0), ".0") + "万"
	}
	return strconv.FormatInt(n, 10)
}

func (s *HongguoService) GetActorProfile(userID string) (*ActorProfileDTO, error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
				CoverURL:     s.media.ImageURL(w.CoverURL),
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
		AvatarURL:   s.media.ImageURL(person.AvatarURL),
		Tags:        person.Tags,
		Labels:      person.Labels,
		Followers:   person.Followers,
		Likes:       person.Likes,
		Works:       works,
		HasMore:     hasMore,
		Cursor:      cursor,
	}
	dto = sanitizeIPC(dto)

	s.cacheMu.Lock()
	s.cache[cacheKey] = cacheItem{val: dto, expiresAt: time.Now().Add(10 * time.Minute)}
	s.cacheMu.Unlock()

	return dto, nil
}

func (s *HongguoService) GetActorWorks(userID, cursor string) (*ActorWorksResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
			CoverURL:     s.media.ImageURL(w.CoverURL),
			EpisodeCount: w.EpisodeCount,
			Category:     category,
			Status:       "finished",
			Heat:         heat,
			Tags:         w.Tags,
		})
	}

	return sanitizeIPC(&ActorWorksResponse{
		Works:   works,
		HasMore: worksPage != nil && worksPage.HasMore,
		Cursor:  workCursor(worksPage),
	}), nil
}

func (s *HongguoService) GetNewReleases(date string) (NewReleaseResponse, error) {
	cacheKey := "new_releases:" + date
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.(NewReleaseResponse)), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	nr, err := s.client.NewReleases(ctx, sdk.NewReleaseQuery{
		Date: date,
	})
	if err != nil {
		return NewReleaseResponse{}, err
	}

	reservedIDs := make(map[string]bool)
	if reservations, err := s.GetReservations(); err == nil {
		for _, item := range reservations {
			reservedIDs[item.SeriesID] = true
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
			CoverURL:     s.media.ImageURL(it.CoverURL),
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

	return sanitizeIPC(resp), nil
}

func (s *HongguoService) Search(query string) ([]SeriesCard, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
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

		coverURL := s.media.ImageURL(it.CoverURL)
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
		bCtx, bCancel := context.WithTimeout(context.Background(), 8*time.Second)
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
					cards[idx].CoverURL = s.media.ImageURL(url)
				}
			}
		}
	}
	return sanitizeIPC(cards), nil
}

func (s *HongguoService) GetSuggestions(prefix string) ([]string, error) {
	if prefix == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	suggs, err := s.client.Suggest(ctx, prefix)
	if err != nil {
		return nil, err
	}
	var words []string
	for _, s := range suggs {
		words = append(words, s.Text)
	}
	return sanitizeIPC(words), nil
}

func (s *HongguoService) GetHotSearch() (HotSearchDTO, error) {
	cacheKey := "hot_search"
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.(HotSearchDTO)), nil
	}
	s.cacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	return sanitizeIPC(resp), nil
}

func (s *HongguoService) GetSeriesDetail(seriesID string) (*SeriesDetailDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
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

	eps, err := s.listEpisodes(ctx, seriesID)
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
			PosterURL:    s.media.ImageURL(ep.CoverURL),
			CommentCount: ep.CommentCount,
		})
		commentCount += ep.CommentCount
	}

	return sanitizeIPC(&SeriesDetailDTO{
		ID:           d.ID,
		Title:        d.Title,
		Description:  d.Description,
		CoverURL:     s.media.ImageURL(d.CoverURL),
		EpisodeCount: d.EpisodeCount,
		Status:       "finished",
		Heat:         "热播",
		Episodes:     episodeDTOs,
		CommentCount: commentCount,
		IsFavorite:   s.store.IsFavorite(seriesID),
	}), nil
}

func (s *HongguoService) PlayEpisode(seriesID, episodeID string, episodeNum int, quality string, preferHLS bool) (*PlaybackDTO, error) {
	if episodeNum <= 0 {
		episodeNum = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var episodePosterURL string
	var commentCount int64
	var playbackDurationSec int
	cachedStart := false
	if episodeID == "" {
		// A full local cache can start instantly without a quality probe or
		// another episode-list round trip. Series details/comments still load
		// in the background on the frontend.
		if cached, ok := s.store.GetCachedEpisodeAt(seriesID, quality, episodeNum); ok &&
			cached.EpisodeNum == episodeNum &&
			fileExists(episodeCachePath(s.media.cacheDir, cached.EpisodeID, cached.Quality)) {
			episodeID = cached.EpisodeID
			episodeNum = cached.EpisodeNum
			quality = cached.Quality
			cachedStart = true
		}
	}

	if episodeID == "" && !cachedStart {
		eps, err := s.listEpisodes(ctx, seriesID)
		if err == nil && len(pageItems(eps)) > 0 {
			targetNum := episodeNum
			found := false
			var selected model.Episode
			for _, ep := range pageItems(eps) {
				if ep.Number == targetNum {
					episodeID = ep.ID
					selected = ep
					playbackDurationSec = int(selected.Duration.Seconds())
					commentCount = ep.CommentCount
					found = true
					break
				}
			}
			if !found {
				items := pageItems(eps)
				episodeID = items[0].ID
				episodeNum = items[0].Number
				selected = items[0]
				commentCount = items[0].CommentCount
			}
			episodePosterURL = s.media.ImageURL(selected.CoverURL)
		}
	}

	if episodeID == "" {
		return nil, fmt.Errorf("failed to resolve episode ID for %s", seriesID)
	}

	var qualityInfo QualitySelection
	if cachedStart {
		qualityInfo = QualitySelection{
			Options:     fallbackQualityOptions(quality),
			Selected:    quality,
			DurationSec: playbackDurationSec,
		}
	} else {
		info, err := s.media.PlaybackQualities(ctx, episodeID, quality)
		if err != nil {
			// Playback can still succeed through MediaServer's own fallback.
			qualityInfo = QualitySelection{
				Options:     fallbackQualityOptions(normalizeVideoQuality(quality)),
				Selected:    normalizeVideoQuality(quality),
				DurationSec: playbackDurationSec,
			}
		} else {
			qualityInfo = info
		}
	}
	_ = s.store.RememberEpisode(seriesID, episodeID, episodeNum, qualityInfo.Selected)
	isFullCached := fileExists(episodeCachePath(s.media.cacheDir, episodeID, qualityInfo.Selected))
	fullStreamURL := s.media.VideoURL(episodeID, seriesID, episodeNum, qualityInfo.Selected)
	streamURL := fullStreamURL
	progressiveStreamURL := ""
	if !isFullCached {
		streamURL = s.media.ProgressiveVideoURL(episodeID, seriesID, episodeNum, qualityInfo.Selected)
		progressiveStreamURL = streamURL
	}
	hlsStreamURL := ""
	if preferHLS && !isFullCached {
		if _, err := s.media.StartHLSStream(seriesID, episodeID, episodeNum, qualityInfo.Selected); err == nil {
			// Return the selected media playlist. A full master would let the
			// WebView choose a higher rendition based on fast local HTTP and
			// bypass the feed's upstream-network scheduler.
			hlsStreamURL = s.media.HLSPlaylistURL(episodeID, seriesID, episodeNum, qualityInfo.Selected)
			streamURL = hlsStreamURL
		}
	}

	hist, err := s.store.GetHistory()
	if err != nil {
		return nil, err
	}
	var lastPos float64
	for _, h := range hist {
		if h.SeriesID == seriesID && h.EpisodeNum == episodeNum {
			lastPos = h.PositionSec
			break
		}
	}

	isLiked := s.store.IsLiked(seriesID, episodeID, episodeNum)

	return sanitizeIPC(&PlaybackDTO{
		SeriesID:             seriesID,
		EpisodeID:            episodeID,
		EpisodeNum:           episodeNum,
		Title:                fmt.Sprintf("第%d集", episodeNum),
		StreamURL:            streamURL,
		FullStreamURL:        fullStreamURL,
		ProgressiveStreamURL: progressiveStreamURL,
		HLSStreamURL:         hlsStreamURL,
		PosterURL:            episodePosterURL,
		DurationSec:          firstPositiveInt(qualityInfo.DurationSec, playbackDurationSec),
		LastPosition:         lastPos,
		IsLiked:              isLiked,
		CommentCount:         commentCount,
		Quality:              qualityInfo.Selected,
		Qualities:            qualityInfo.Options,
		IsFullCached:         isFullCached,
	}), nil
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeNetworkProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "weak", "medium", "strong":
		return strings.ToLower(strings.TrimSpace(profile))
	default:
		return "unknown"
	}
}

func truncateString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func warmupWindowLimits(profile string) (next, previous int) {
	switch normalizeNetworkProfile(profile) {
	case "weak":
		// Preserve the immediate next progressive stream, but defer all full-file
		// work until playback proves that the connection can sustain it.
		return 1, 0
	case "medium":
		return 2, 1
	case "strong":
		return 3, 2
	default:
		return 2, 1
	}
}

func fallbackQualityOptions(selected string) []QualityOptionDTO {
	selected = normalizeVideoQuality(selected)
	options := make([]QualityOptionDTO, 0, len(videoQualities))
	for _, quality := range videoQualities {
		options = append(options, QualityOptionDTO{
			ID:        quality,
			Label:     qualityLabel(quality),
			Available: true,
			Active:    quality == selected,
		})
	}
	return options
}

// WarmupFeed performs bidirectional background preheating for adjacent dramas in the recommend feed.
func (s *HongguoService) WarmupFeed(centerSeriesID, centerEpisodeID string, prevSeriesIDs []string, nextSeriesIDs []string, quality, networkProfile string) error {
	quality = normalizeVideoQuality(quality)
	networkProfile = normalizeNetworkProfile(networkProfile)
	s.feedWarmupMu.Lock()
	s.feedWarmupGeneration++
	warmupGeneration := s.feedWarmupGeneration
	s.feedWarmupMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		type warmupItem struct {
			id           string
			priority     int
			deferred     bool
			prefetchOnly bool
		}
		items := make([]warmupItem, 0, 6)
		seen := make(map[string]bool, 6)
		appendSeries := func(id string, priority int, deferred, prefetchOnly bool) {
			if id != "" && !seen[id] {
				seen[id] = true
				items = append(items, warmupItem{id: id, priority: priority, deferred: deferred, prefetchOnly: prefetchOnly})
			}
		}
		nextLimit, previousLimit := warmupWindowLimits(networkProfile)
		appendSeries(centerSeriesID, 0, false, false)
		for i, id := range nextSeriesIDs {
			if i >= nextLimit {
				break
			}
			// The immediate next may fall back to a full file if its renderer
			// stream cannot start. Farther items are HLS-prefetch only so they
			// never compete with the handoff path.
			appendSeries(id, i+1, true, i > 0)
		}
		for i, id := range prevSeriesIDs {
			if i >= previousLimit {
				break
			}
			appendSeries(id, len(nextSeriesIDs)+i+1, true, true)
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].priority < items[j].priority })

		var tasksMu sync.Mutex
		tasks := make([]prefetchTask, 0, len(items))
		publish := func(task prefetchTask) {
			tasksMu.Lock()
			tasks = append(tasks, task)
			snapshot := make([]prefetchTask, len(tasks))
			copy(snapshot, tasks)
			tasksMu.Unlock()

			// Serialize snapshots through the warmup lock. Otherwise a stale,
			// smaller snapshot could replace the window after a larger one and
			// cancel work that was just admitted.
			s.feedWarmupMu.Lock()
			defer s.feedWarmupMu.Unlock()
			if warmupGeneration != s.feedWarmupGeneration {
				return
			}
			s.media.ReplacePrefetchWindow(snapshot)
		}
		resolveItem := func(item warmupItem) {
			seriesID := item.id
			episodes, err := s.listEpisodes(ctx, seriesID)
			if err != nil || len(pageItems(episodes)) == 0 {
				return
			}
			episode := pageItems(episodes)[0]
			_ = s.store.RememberEpisode(seriesID, episode.ID, episode.Number, quality)
			publish(prefetchTask{
				seriesID:     seriesID,
				episodeID:    episode.ID,
				number:       episode.Number,
				quality:      quality,
				protectOnly:  seriesID == centerSeriesID,
				deferred:     item.deferred,
				prefetchOnly: item.prefetchOnly,
				priority:     item.priority,
			})
		}

		// The renderer supplies the playing episode ID. Publishing it first gives
		// MediaServer an immediate protection key and cancels every stale feed
		// download without waiting for adjacent-series metadata requests.
		if centerEpisodeID != "" {
			publish(prefetchTask{
				seriesID:    centerSeriesID,
				episodeID:   centerEpisodeID,
				number:      1,
				quality:     quality,
				protectOnly: true,
			})
		}

		var workers sync.WaitGroup
		for _, item := range items {
			if item.id == centerSeriesID && centerEpisodeID != "" {
				continue
			}
			workers.Add(1)
			go func(item warmupItem) {
				defer workers.Done()
				resolveItem(item)
			}(item)
		}
		workers.Wait()
	}()

	return nil
}

func (s *HongguoService) GetMediaDiagnostics() (MediaDiagnosticsDTO, error) {
	return s.media.MediaDiagnostics(), nil
}

func (s *HongguoService) RecordFeedMetrics(metrics []FeedMetricDTO) error {
	if len(metrics) == 0 {
		return nil
	}
	if len(metrics) > 32 {
		metrics = metrics[len(metrics)-32:]
	}
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	path := filepath.Join(s.appDir, "feed-metrics.jsonl")
	if info, err := os.Stat(path); err == nil && info.Size() > 1024*1024 {
		_ = os.Rename(path, path+".1")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	writer := bufio.NewWriter(file)
	for _, metric := range metrics {
		metric.Type = truncateString(metric.Type, 32)
		metric.NetworkProfile = truncateString(metric.NetworkProfile, 16)
		metric.StreamMode = truncateString(metric.StreamMode, 24)
		metric.Quality = truncateString(metric.Quality, 16)
		metric.FirstFrameSource = truncateString(metric.FirstFrameSource, 24)
		if metric.At == "" {
			metric.At = now
		} else {
			metric.At = truncateString(metric.At, 48)
		}
		encoded, err := json.Marshal(metric)
		if err != nil {
			continue
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func (s *HongguoService) SaveProgress(seriesID, episodeID string, title, coverURL string, episodeNum, totalEpisodes int, positionSec, durationSec float64) error {
	seriesID = strings.TrimSpace(seriesID)
	if !validContentID(seriesID) {
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

func (s *HongguoService) GetHistory() ([]HistoryItem, error) {
	s.historyMu.Lock()

	// 1. If in-memory cache is fresh (< 30s), return instantly
	if len(s.cachedHistory) > 0 && time.Since(s.historyCachedAt) < 30*time.Second {
		cached := make([]HistoryItem, len(s.cachedHistory))
		copy(cached, s.cachedHistory)
		s.historyMu.Unlock()
		return sanitizeIPC(cached), nil
	}

	// 2. Check local store (sub-millisecond instant return)
	localItems, err := s.store.GetHistory()
	if err != nil {
		s.historyMu.Unlock()
		return nil, err
	}
	if len(localItems) > 0 {
		for i := range localItems {
			localItems[i].CoverURL = s.media.ImageURL(localItems[i].CoverURL)
		}
		localItems = sanitizeIPC(localItems)
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
				s.syncRemoteHistory(generation)
			}()
		}
		s.historyMu.Unlock()
		return sanitizeIPC(localItems), nil
	}
	s.historyMu.Unlock()

	// 3. Local store is empty (e.g. first app launch with an existing account) -> sync synchronously
	s.syncRemoteHistory(s.currentHistoryGeneration())

	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	list, err := s.store.GetHistory()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].CoverURL = s.media.ImageURL(list[i].CoverURL)
	}
	list = sanitizeIPC(list)
	s.cachedHistory = list
	s.historyCachedAt = time.Now()
	return sanitizeIPC(list), nil
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

func (s *HongguoService) fetchRemoteHistoryPage(ctx context.Context, offset int64) (*remoteHistoryPage, error) {
	userCookies, userToken := s.authCredentials()
	if userCookies == "" {
		return nil, nil
	}
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := append(defaultDeviceQueryPairs(deviceID, iid, cdid, openudid),
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
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Cookie", userCookies)
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("x-tt-store-region", s.storeRegion)
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

func (s *HongguoService) syncRemoteHistory(generation uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			bCtx, bCancel := context.WithTimeout(context.Background(), 8*time.Second)
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
			CoverURL:      normalizeImageURL(cover),
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
		list[i].CoverURL = s.media.ImageURL(list[i].CoverURL)
	}
	s.cachedHistory = list
	s.historyCachedAt = time.Now()
	s.historyMu.Unlock()
	s.triggerPendingHistorySync()
}
func (s *HongguoService) DeleteHistory(seriesID string) error {
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

func (s *HongguoService) ClearHistory() error {
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

func (s *HongguoService) ToggleFavorite(seriesID, title, coverURL, desc string, totalEpisodes int) (bool, error) {
	seriesID = strings.TrimSpace(seriesID)
	if !validContentID(seriesID) {
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

func (s *HongguoService) IsFavorite(seriesID string) (bool, error) {
	return s.store.IsFavorite(seriesID), nil
}

func (s *HongguoService) GetFavorites() ([]FavoriteItem, error) {
	s.favMu.Lock()

	// 1. If in-memory cache is fresh (< 30s), return instantly
	if len(s.cachedFavorites) > 0 && time.Since(s.favCachedAt) < 30*time.Second {
		cached := make([]FavoriteItem, len(s.cachedFavorites))
		copy(cached, s.cachedFavorites)
		s.favMu.Unlock()
		return sanitizeIPC(cached), nil
	}

	userCookies, _ := s.authCredentials()

	// 2. Check local store (sub-millisecond instant return)
	localItems, err := s.store.GetFavorites()
	if err != nil {
		s.favMu.Unlock()
		return nil, err
	}
	if len(localItems) > 0 {
		for i := range localItems {
			localItems[i].CoverURL = s.media.ImageURL(localItems[i].CoverURL)
		}
		localItems = sanitizeIPC(localItems)
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
				s.syncRemoteFavorites(generation)
			}()
		}
		s.favMu.Unlock()
		return sanitizeIPC(localItems), nil
	}
	s.favMu.Unlock()

	if userCookies == "" {
		s.favMu.Lock()
		s.cachedFavorites = localItems
		s.favCachedAt = time.Now()
		s.favMu.Unlock()
		return sanitizeIPC(localItems), nil
	}

	// 3. Local store empty -> synchronous fetch
	s.syncRemoteFavorites(s.currentFavoriteGeneration())

	s.favMu.Lock()
	defer s.favMu.Unlock()
	list, err := s.store.GetFavorites()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].CoverURL = s.media.ImageURL(list[i].CoverURL)
	}
	list = sanitizeIPC(list)
	s.cachedFavorites = list
	s.favCachedAt = time.Now()
	return sanitizeIPC(list), nil
}

func (s *HongguoService) syncRemoteFavorites(generation uint64) {
	userCookies, _ := s.authCredentials()
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userCookies, userToken := s.authCredentials()
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
	req.Header.Set("User-Agent", s.userAgent)
	if userCookies != "" {
		req.Header.Set("Cookie", userCookies)
	}
	if userToken != "" {
		req.Header.Set("x-tt-token", userToken)
	}
	req.Header.Set("x-tt-store-region", s.storeRegion)
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
			bCtx, bCancel := context.WithTimeout(context.Background(), 8*time.Second)
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
				CoverURL:      normalizeImageURL(b.Series.CoverURL),
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

func (s *HongguoService) refreshFavoriteCache(generation uint64) {
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
		list[i].CoverURL = s.media.ImageURL(list[i].CoverURL)
	}
	s.cachedFavorites = list
	s.favCachedAt = time.Now()
	s.favMu.Unlock()
}
func (s *HongguoService) ToggleLike(seriesID, episodeID, title, coverURL string, episodeNum int) (bool, error) {
	userCookies, _ := s.authCredentials()
	seriesID = strings.TrimSpace(seriesID)
	episodeID = strings.TrimSpace(episodeID)
	if !validContentID(seriesID) || !validContentID(episodeID) {
		return false, fmt.Errorf("点赞 ID 无效")
	}
	liked := !s.store.IsLiked(seriesID, episodeID, episodeNum)
	if userCookies != "" {
		if err := s.requestEpisodeLikeMutation(seriesID, episodeID, liked); err != nil {
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

func (s *HongguoService) GetLikes() ([]LikeItem, error) {
	userCookies, _ := s.authCredentials()
	s.likeMu.Lock()

	if len(s.cachedLikes) > 0 && time.Since(s.likeCachedAt) < 30*time.Second {
		cached := make([]LikeItem, len(s.cachedLikes))
		copy(cached, s.cachedLikes)
		s.likeMu.Unlock()
		return sanitizeIPC(cached), nil
	}

	localItems, err := s.store.GetLikes()
	if err != nil {
		s.likeMu.Unlock()
		return nil, err
	}
	if len(localItems) > 0 {
		for i := range localItems {
			localItems[i].CoverURL = s.media.ImageURL(localItems[i].CoverURL)
		}
		localItems = sanitizeIPC(localItems)
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
				_ = s.syncRemoteLikes(generation)
			}()
		}
		s.likeMu.Unlock()
		return sanitizeIPC(localItems), nil
	}
	s.likeMu.Unlock()

	if userCookies == "" {
		return []LikeItem{}, nil
	}
	if err := s.syncRemoteLikes(s.currentLikeGeneration()); err != nil {
		return nil, err
	}

	s.likeMu.Lock()
	defer s.likeMu.Unlock()
	list, err := s.store.GetLikes()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].CoverURL = s.media.ImageURL(list[i].CoverURL)
	}
	list = sanitizeIPC(list)
	s.cachedLikes = list
	s.likeCachedAt = time.Now()
	return sanitizeIPC(list), nil
}

func (s *HongguoService) syncRemoteLikes(generation uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		list[i].CoverURL = s.media.ImageURL(list[i].CoverURL)
	}
	s.cachedLikes = sanitizeIPC(list)
	s.likeCachedAt = time.Now()
	s.likeMu.Unlock()
	return nil
}

func (s *HongguoService) fetchAccountLikes(ctx context.Context) ([]LikeItem, error) {
	userCookies, userToken := s.authCredentials()
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	pairs := append(defaultDeviceQueryPairs(deviceID, iid, cdid, openudid),
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
	req.Header.Set("User-Agent", s.userAgent)
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
			CoverURL:      normalizeImageURL(cover),
			EpisodeNum:    0,
			TotalEpisodes: video.EpisodeCnt,
			Description:   firstNonEmpty(row.SeriesIntro, video.Description),
			AddedAt:       addedAt,
		})
	}
	return items, nil
}

func (s *HongguoService) requestEpisodeLikeMutation(seriesID, episodeID string, liked bool) error {
	userCookies, userToken := s.authCredentials()
	actionType := 3
	if !liked {
		actionType = 4
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	pairs := defaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
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
	req.Header.Set("User-Agent", s.userAgent)

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

func (s *HongguoService) ToggleReservation(seriesID, title, coverURL, note, publishAt string) (bool, error) {
	userCookies, _ := s.authCredentials()
	seriesID = strings.TrimSpace(seriesID)
	if !validContentID(seriesID) {
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
	if err := s.requestReservationMutation(seriesID, opType); err != nil {
		return false, err
	}
	if !wasReserved {
		_, _ = s.SendNativeNotification(NotificationArgs{
			Title:    "新剧预约成功",
			Body:     fmt.Sprintf("《%s》已加入预约列表，上线后将第一时间通知您！", title),
			SeriesID: seriesID,
		})
	}
	if _, err := s.syncRemoteReservations(generation); err != nil {
		// The write succeeded even when confirmation is slow or unavailable.
		return !wasReserved, nil
	}
	return !wasReserved, nil
}

func (s *HongguoService) invalidateReservationCaches() {
	s.cacheMu.Lock()
	for key := range s.cache {
		if strings.HasPrefix(key, "series_page:") || strings.HasPrefix(key, "new_releases:") {
			delete(s.cache, key)
		}
	}
	s.cacheMu.Unlock()
}

func (s *HongguoService) requestReservationMutation(seriesID string, opType int) error {
	userCookies, userToken := s.authCredentials()
	itemID, err := strconv.ParseInt(seriesID, 10, 64)
	if err != nil || itemID <= 0 {
		return fmt.Errorf("预约编号无效")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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
	pairs := defaultDeviceQueryPairs(deviceID, iid, cdid, openudid)
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
	req.Header.Set("User-Agent", s.userAgent)

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

func (s *HongguoService) GetReservations() ([]ReservationItem, error) {
	s.resMu.Lock()

	// 1. If in-memory cache is fresh (< 60s), return instantly
	if len(s.cachedReservations) > 0 && time.Since(s.resCachedAt) < 60*time.Second {
		cached := make([]ReservationItem, len(s.cachedReservations))
		copy(cached, s.cachedReservations)
		s.resMu.Unlock()
		return sanitizeIPC(cached), nil
	}

	// 2. Check local store (sub-millisecond instant return)
	storeList, err := s.store.GetReservations()
	if err != nil {
		s.resMu.Unlock()
		return nil, err
	}
	for i := range storeList {
		storeList[i].CoverURL = s.media.ImageURL(storeList[i].CoverURL)
	}
	storeList = sanitizeIPC(storeList)

	userCookies, _ := s.authCredentials()
	if userCookies == "" {
		s.cachedReservations = storeList
		s.resCachedAt = time.Now()
		s.resMu.Unlock()
		return sanitizeIPC(storeList), nil
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
				_, _ = s.syncRemoteReservations(generation)
			}()
		}
		s.resMu.Unlock()
		return sanitizeIPC(storeList), nil
	}
	s.resMu.Unlock()

	// 3. Local store empty -> synchronous fetch
	results, err := s.syncRemoteReservations(s.currentReservationGeneration())
	return sanitizeIPC(results), err
}

func (s *HongguoService) syncRemoteReservations(generation uint64) ([]ReservationItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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
			_, _ = s.SendNativeNotification(NotificationArgs{
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
		storeList[i].CoverURL = s.media.ImageURL(storeList[i].CoverURL)
	}
	storeList = sanitizeIPC(storeList)
	s.cachedReservations = storeList
	s.resCachedAt = time.Now()
	s.resMu.Unlock()

	return sanitizeIPC(storeList), nil
}

func (s *HongguoService) fetchAccountReservations(ctx context.Context, online bool) ([]ReservationItem, error) {
	userCookies, userToken := s.authCredentials()
	var results []ReservationItem
	offset := 0

	for {
		deviceID, iid, cdid, openudid := s.deviceIdentity()
		pairs := append(defaultDeviceQueryPairs(deviceID, iid, cdid, openudid),
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
		req.Header.Set("User-Agent", s.userAgent)
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
				CoverURL:  s.media.ImageURL(cover),
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

func (s *HongguoService) ResetDevice() error {
	return s.resetDeviceWithContext(context.Background())
}

func (s *HongguoService) resetDeviceWithContext(ctx context.Context) error {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.client.ResetDevice(ctx); err != nil {
		return err
	}
	if s.deviceProfileStore != nil {
		if err := s.reloadDeviceIdentityLocked(); err != nil {
			return err
		}
	}
	s.invalidateUserDataCaches()
	return nil
}

func (s *HongguoService) ClearCache() (int64, error) {
	s.invalidateUserDataCaches()
	return s.media.ClearCache()
}

func (s *HongguoService) GetCacheSize() (int64, error) {
	return s.media.GetCacheSize(), nil
}

func (s *HongguoService) GetDeviceInfo() (map[string]string, error) {
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	return map[string]string{
		"deviceId":    deviceID,
		"installId":   iid,
		"cdid":        cdid,
		"openudid":    openudid,
		"storeRegion": s.storeRegion,
		"version":     "2.0.0",
		"arch":        "darwin-arm64",
	}, nil
}

func (s *HongguoService) LoginWithCookies(cookies, token string) (*UserProfile, error) {
	cookies = strings.TrimSpace(cookies)
	token = strings.TrimSpace(token)

	// Support parsing raw JSON input if the user pasted a JSON string
	if strings.HasPrefix(cookies, "{") && strings.HasSuffix(cookies, "}") {
		var rawObj struct {
			Cookies string `json:"cookies"`
			Token   string `json:"token"`
		}
		if err := json.Unmarshal([]byte(cookies), &rawObj); err == nil && rawObj.Cookies != "" {
			cookies = rawObj.Cookies
			if token == "" && rawObj.Token != "" {
				token = rawObj.Token
			}
		}
	}

	if cookies == "" {
		return nil, fmt.Errorf("Cookie 不能为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/user/info/v1/?aid=8662&app_name=novelread&version_code=73932"
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建验证请求失败: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Cookie", cookies)
	if token != "" {
		req.Header.Set("x-tt-token", token)
	}
	req.Header.Set("x-tt-store-region", s.storeRegion)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求官方登录接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取接口响应失败: %w", err)
	}

	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			UserName  string `json:"user_name"`
			AvatarURL string `json:"avatar_url"`
			UserID    string `json:"user_id"`
			BizUserID string `json:"biz_user_id"`
			ReqID     any    `json:"req_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("解析用户信息响应失败: %w", err)
	}

	if res.Code != 0 {
		msg := res.Message
		if msg == "" {
			msg = "凭据无效或已失效"
		}
		return nil, fmt.Errorf("登录验证未通过: %s (code: %d)", msg, res.Code)
	}

	// Anonymous /user/info responses also return code 0. req_id is not an
	// account ID, so only a non-zero account or business account proves login.
	uid := firstNonEmpty(
		nonZeroUserID(res.Data.BizUserID),
		nonZeroUserID(res.Data.UserID),
	)
	if uid == "" {
		return nil, fmt.Errorf("登录验证未通过: 凭据未返回有效账号")
	}
	nickname := strings.TrimSpace(res.Data.UserName)
	if nickname == "" {
		nickname = "myguo用户"
	}
	avatar := strings.TrimSpace(res.Data.AvatarURL)

	// Save to account.json
	accPath := filepath.Join(s.appDir, "account.json")
	accData := map[string]any{
		"cookies": cookies,
		"token":   token,
		"profile": map[string]any{
			"loggedIn": true,
			"userId":   uid,
			"nickname": nickname,
			"avatar":   avatar,
		},
	}
	accBytes, _ := json.MarshalIndent(accData, "", "  ")
	_ = os.WriteFile(accPath, accBytes, 0644)

	// Update service state
	s.setAuthCredentials(cookies, token)

	profile := UserProfile{
		UserID:   uid,
		Nickname: nickname,
		Avatar:   avatar,
		LoggedIn: true,
	}
	_ = s.store.UpdateProfile(profile)

	// Invalidate user data caches
	s.invalidateUserDataCaches()

	if profile.Avatar != "" {
		profile.Avatar = s.media.ImageURL(profile.Avatar)
	}
	return sanitizeIPC(&profile), nil
}

func nonZeroUserID(value string) string {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return ""
	}
	return strings.TrimSpace(value)
}

func (s *HongguoService) RefreshUserProfile() (*UserProfile, error) {
	userCookies, userToken := s.authCredentials()
	if userCookies == "" {
		return nil, fmt.Errorf("当前未登录")
	}
	return s.LoginWithCookies(userCookies, userToken)
}

func (s *HongguoService) Logout() error {
	s.setAuthCredentials("", "")

	accPath := filepath.Join(s.appDir, "account.json")
	_ = os.Remove(accPath)

	anonProfile := UserProfile{
		UserID:   "anonymous",
		Nickname: "myguo用户",
		Avatar:   "",
		LoggedIn: false,
	}
	_ = s.store.UpdateProfile(anonProfile)

	s.cacheMu.Lock()
	s.cache = make(map[string]cacheItem)
	s.cacheMu.Unlock()
	s.invalidateUserDataCaches()

	return nil
}

func (s *HongguoService) CheckExistingAccount() (map[string]any, error) {
	home, _ := os.UserHomeDir()
	oldAccPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	data, err := os.ReadFile(oldAccPath)
	if err != nil {
		return sanitizeIPC(map[string]any{"exists": false}), nil
	}
	var acc struct {
		Cookies string `json:"cookies"`
		Token   string `json:"token"`
		Profile struct {
			UserID   string `json:"userId"`
			Nickname string `json:"nickname"`
			Avatar   string `json:"avatar"`
			LoggedIn bool   `json:"loggedIn"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &acc); err != nil || acc.Cookies == "" {
		return sanitizeIPC(map[string]any{"exists": false}), nil
	}

	avatar := acc.Profile.Avatar
	if avatar != "" {
		avatar = s.media.ImageURL(avatar)
	}

	return sanitizeIPC(map[string]any{
		"exists":   true,
		"nickname": acc.Profile.Nickname,
		"userId":   acc.Profile.UserID,
		"avatar":   avatar,
	}), nil
}

func (s *HongguoService) ImportHgplayerAccount() (*UserProfile, error) {
	home, _ := os.UserHomeDir()
	oldAccPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	data, err := os.ReadFile(oldAccPath)
	if err != nil {
		return nil, fmt.Errorf("未找到旧版应用账号数据")
	}
	var acc struct {
		Cookies string `json:"cookies"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(data, &acc); err != nil || acc.Cookies == "" {
		return nil, fmt.Errorf("旧版账号数据无效")
	}
	return s.LoginWithCookies(acc.Cookies, acc.Token)
}

func defaultDeviceQueryPairs(deviceID, installID, cdid, openudid string) [][2]string {
	return [][2]string{
		{"aid", "8662"},
		{"app_name", "novelread"},
		{"version_code", "73932"},
		{"version_name", "7.3.9.32"},
		{"device_platform", "android"},
		{"os_version", "14"},
		{"device_type", "23127PN0CC"},
		{"device_brand", "Xiaomi"},
		{"channel", "xiaomi_8662_64"},
		{"device_id", deviceID},
		{"iid", installID},
		{"cdid", cdid},
		{"openudid", openudid},
		{"dpi", "460"},
		{"update_version_code", "73932"},
	}
}

// ============================================================================
// macOS Native Integration IPC Handlers
// ============================================================================

func (s *HongguoService) UpdateNowPlaying(args UpdateNowPlayingArgs) (bool, error) {
	cacheDir := ""
	if s.media != nil {
		cacheDir = s.media.cacheDir
	}
	updateNowPlaying(cacheDir, args)
	return true, nil
}

func (s *HongguoService) ClearNowPlaying() (bool, error) {
	clearNowPlaying()
	return true, nil
}

func (s *HongguoService) SetPlaybackKeepAwake(active bool) (bool, error) {
	setPlaybackKeepAwake(active)
	return true, nil
}

func (s *HongguoService) ShowConfirmDialog(args ConfirmDialogArgs) (bool, error) {
	return showConfirmDialog(args)
}

func (s *HongguoService) SendNativeNotification(args NotificationArgs) (bool, error) {
	return sendNativeNotification(args)
}

func (s *HongguoService) TogglePictureInPicture() (bool, error) {
	togglePictureInPicture()
	return true, nil
}
