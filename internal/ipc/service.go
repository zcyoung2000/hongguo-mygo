package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/auth"
	"myguo/internal/catalog"
	"myguo/internal/library"
	"myguo/internal/media"
	"myguo/internal/mediautil"
	"myguo/internal/native"
	"myguo/internal/sanitize"
	userstore "myguo/internal/store"
	"myguo/internal/upstream"
)

type (
	SeriesCard           = catalog.SeriesCard
	FilterOptionDTO      = catalog.FilterOptionDTO
	FilterRowDTO         = catalog.FilterRowDTO
	DiscoverResponse     = catalog.DiscoverResponse
	RankTabDTO           = catalog.RankTabDTO
	RankItemDTO          = catalog.RankItemDTO
	RankResponse         = catalog.RankResponse
	ActorProfileDTO      = catalog.ActorProfileDTO
	ActorWorksResponse   = catalog.ActorWorksResponse
	NewReleaseItemDTO    = catalog.NewReleaseItemDTO
	NewReleaseResponse   = catalog.NewReleaseResponse
	HotSearchDTO         = catalog.HotSearchDTO
	EpisodeDTO           = catalog.EpisodeDTO
	SeriesDetailDTO      = catalog.SeriesDetailDTO
	Store                = userstore.Store
	UserProfile          = userstore.UserProfile
	HistoryItem          = userstore.HistoryItem
	FavoriteItem         = userstore.FavoriteItem
	LikeItem             = userstore.LikeItem
	ReservationItem      = userstore.ReservationItem
	CachedEpisode        = userstore.CachedEpisode
	FeedProgress         = userstore.FeedProgress
	QualityOptionDTO     = media.QualityOptionDTO
	QualitySelection     = media.QualitySelection
	MediaServer          = media.MediaServer
	MediaDiagnosticsDTO  = media.MediaDiagnosticsDTO
	PrefetchTask         = media.PrefetchTask
	PrefetchExecution    = media.PrefetchExecution
	UpdateNowPlayingArgs = native.UpdateNowPlayingArgs
	ConfirmDialogArgs    = native.ConfirmDialogArgs
	NotificationArgs     = native.NotificationArgs
)

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

func NewStore(dir string) (*Store, error) { return userstore.NewStore(dir) }

func NewMediaServer(client *sdk.Client) (*MediaServer, error) { return media.NewMediaServer(client) }

func ffmpegCommand() (string, error) { return media.FFmpegCommand() }

func normalizeVideoQuality(quality string) string { return mediautil.NormalizeVideoQuality(quality) }

func videoQualities() []string { return media.VideoQualities() }

func qualityLabel(quality string) string { return media.QualityLabel(quality) }

func normalizeImageURL(raw string) string { return mediautil.NormalizeImageURL(raw) }

func sanitizeIPC[T any](value T) T { return sanitize.Sanitize(value) }

func defaultDeviceQueryPairs(deviceID, installID, cdid, openudid string) [][2]string {
	return upstream.DefaultDeviceQueryPairs(deviceID, installID, cdid, openudid)
}

func formatHeatScore(n int64) string { return catalog.FormatHeatScore(n) }

func episodeCachePath(cacheDir, episodeID, quality string) string {
	return media.EpisodeCachePath(cacheDir, episodeID, quality)
}

func fileExists(path string) bool { return media.FileExists(path) }

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
	catalogOnce          sync.Once
	catalog              *catalog.Service
	libraryOnce          sync.Once
	library              *library.Service
	client               *sdk.Client
	store                *Store
	media                *MediaServer
	appDir               string
	deviceProfileStore   sdk.ProfileStore
	authTransport        auth.CredentialTransport
	deviceMu             sync.RWMutex
	authMu               sync.RWMutex
	userCookies          string
	userToken            string
	deviceID             string
	iid                  string
	cdid                 string
	openudid             string
	userAgent            string
	storeRegion          string
	httpClient           *http.Client
	cacheMu              sync.RWMutex
	cache                map[string]cacheItem
	feedMu               sync.Mutex
	metricsMu            sync.Mutex
	feedWarmupMu         sync.Mutex
	feedWarmupGeneration uint64
	episodeMu            sync.Mutex
	episodeCache         map[string]episodeCacheEntry
	episodeLoads         map[string]*episodeLoad
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

func (s *HongguoService) SetAuthTransport(transport auth.CredentialTransport) {
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
	s.authTransport.SetCredentials(cookies, token, s.storeRegion)
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
	if s.catalog != nil {
		s.catalog.Invalidate()
	}
	if s.library != nil {
		s.library.InvalidateUserData()
	}
	s.cacheMu.Lock()
	s.cache = make(map[string]cacheItem)
	s.cacheMu.Unlock()
}

func (s *HongguoService) GetProfile(ctx context.Context) (UserProfile, error) {
	p := s.store.GetProfile()
	if p.Avatar != "" {
		p.Avatar = s.media.ImageURL(p.Avatar)
	}
	return sanitizeIPC(p), nil
}

func (s *HongguoService) SetFeedScrollCapture(ctx context.Context, enabled bool) error {
	native.SetFeedScrollCapture(enabled)
	return nil
}

type RecommendFeedResult struct {
	Items   []SeriesCard `json:"items"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"hasMore"`
}

func (s *HongguoService) GetRecommendFeed(ctx context.Context, channel, cursor string) (*RecommendFeedResult, error) {
	// A one-page official feed can contain only two or three dramas. Start with
	// a small aggregate so pagination never begins already behind one swipe.
	return s.getRecommendFeed(ctx, channel, cursor, 6)
}

func (s *HongguoService) GetRecommendFeedBatch(ctx context.Context, channel, cursor string, minimumItems int) (*RecommendFeedResult, error) {
	return s.getRecommendFeed(ctx, channel, cursor, minimumItems)
}

func (s *HongguoService) getRecommendFeed(ctx context.Context, channel, cursor string, minimumItems int) (*RecommendFeedResult, error) {
	timeout := 30 * time.Second
	if minimumItems > 1 {
		// Background swipe-buffer fills should never hold the feed cursor lock
		// for the full foreground request budget.
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if channel == "关注" {
		favorites, err := s.GetFavorites(ctx)
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

func (s *HongguoService) GetRecommend(ctx context.Context) ([]SeriesCard, error) {
	cacheKey := "recommend"
	s.cacheMu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return sanitizeIPC(entry.val.([]SeriesCard)), nil
	}
	s.cacheMu.RUnlock()

	res, err := s.GetRecommendFeed(ctx, "", "")
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
	return mediautil.PageItems(page)
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

func shortFilterItems(page *model.ShortFilterPage) []model.SeriesSummary {
	if page == nil {
		return nil
	}
	return page.Items
}
