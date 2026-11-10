package ipc

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
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
	"myguo/internal/resourcecache"
	"myguo/internal/sanitize"
	"myguo/internal/sparedevice"
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
	IsHLSCached          bool               `json:"isHlsCached"`
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

type FeedMetricDTO = userstore.FeedMetric

const metadataCacheCapacity = 256

type episodeCacheEntry struct {
	page      *sdk.Page[model.Episode]
	expiresAt time.Time
}

type episodeLoad struct {
	done   chan struct{}
	result episodeListResult
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
	discover             func(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error)
	resetDevice          func(context.Context) error
	seriesBatch          func(context.Context, []string) ([]model.SeriesDetail, error)
	store                *Store
	media                *MediaServer
	appDir               string
	deviceProfileStore   sdk.ProfileStore
	authTransport        auth.CredentialTransport
	identityMu           sync.RWMutex
	accountMu            sync.Mutex
	authMu               sync.RWMutex
	authUserID           string
	userCookies          string
	userToken            string
	deviceID             string
	iid                  string
	cdid                 string
	openudid             string
	userAgent            string
	storeRegion          string
	httpClient           *http.Client
	cache                *resourcecache.LRU
	feedMu               sync.Mutex
	feedWarmupMu         sync.Mutex
	feedWarmupGeneration uint64
	episodeMu            sync.Mutex
	episodeCache         map[string]episodeCacheEntry
	episodeCacheIndex    map[string]*list.Element
	episodeCacheLRU      *list.List
	episodeLoads         map[string]*episodeLoad
	spareDevices         *sparedevice.Manager
}

type ServiceOption func(*HongguoService)

func WithAuthTransport(transport auth.CredentialTransport) ServiceOption {
	return func(s *HongguoService) { s.setAuthTransport(transport) }
}

func WithDeviceProfileStore(store sdk.ProfileStore) ServiceOption {
	return func(s *HongguoService) { s.setDeviceProfileStore(store) }
}

func WithSpareDeviceManager(manager *sparedevice.Manager) ServiceOption {
	return func(s *HongguoService) { s.spareDevices = manager }
}

func NewHongguoService(client *sdk.Client, store *Store, media *MediaServer, appDir string, opts ...ServiceOption) *HongguoService {
	return newHongguoService(client, store, media, appDir, "", opts...)
}

func newHongguoService(client *sdk.Client, store *Store, media *MediaServer, appDir, home string, opts ...ServiceOption) *HongguoService {
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
		client:            client,
		store:             store,
		media:             media,
		appDir:            appDir,
		deviceID:          "4210501433995451",
		iid:               "4210501433184459",
		cdid:              "ad52a07c-7c54-41eb-8b47-750f78610278",
		openudid:          "dcffb1368df39e9b",
		userAgent:         "com.phoenix.read/73932 (Linux; U; Android 14; zh_CN; Xiaomi 14; Build/UKQ1.230804.001; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)",
		storeRegion:       "cn-gd",
		httpClient:        &http.Client{Transport: tr, Timeout: 12 * time.Second},
		cache:             resourcecache.New(metadataCacheCapacity),
		episodeCache:      make(map[string]episodeCacheEntry),
		episodeCacheLRU:   list.New(),
		episodeCacheIndex: make(map[string]*list.Element),
		episodeLoads:      make(map[string]*episodeLoad),
	}
	for _, opt := range opts {
		opt(svc)
	}
	if svc.spareDevices != nil {
		if transport, ok := svc.authTransport.(sdk.Transport); ok {
			svc.spareDevices = sparedevice.NewManager(store,
				sparedevice.WithClientConstructor(sdk.NewClient, transport),
				sparedevice.WithLogger(log.Default()),
			)
		}
		if svc.media != nil {
			svc.media.SetSpareDevicePool(svc.spareDevices)
		}
	}
	// Restore only credentials explicitly saved by this app. Legacy import is
	// offered separately and must never undo an explicit logout on restart.
	if err := svc.restoreSavedAccount(); err != nil {
		log.Printf("[account] restore failed: %v", err)
	}
	if err := svc.reloadDeviceIdentity(); err != nil {
		// The SDK will generate and persist a profile on first network use.
		log.Printf("[device] stored identity unavailable: %v", err)
	}
	if userAgent, region, ok := svc.store.GetDeviceRuntime(); ok {
		svc.identityMu.Lock()
		if userAgent != "" {
			svc.userAgent = userAgent
		}
		if region != "" {
			svc.storeRegion = region
		}
		svc.identityMu.Unlock()
	}

	return svc
}

func (s *HongguoService) setDeviceProfileStore(store sdk.ProfileStore) {
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	s.deviceProfileStore = store
	if sqliteStore, ok := store.(*userstore.SQLiteProfileStore); ok {
		// The hook is internal to the desktop service and is never exposed as
		// an IPC method.
		sqliteStore.SetSavedHook(s.applyDeviceProfile)
	}
}

func (s *HongguoService) setAuthTransport(transport auth.CredentialTransport) {
	s.authTransport = transport
	s.syncAuthTransport()
}

func (s *HongguoService) deviceIdentity() (deviceID, iid, cdid, openudid string) {
	s.identityMu.RLock()
	defer s.identityMu.RUnlock()
	return s.deviceID, s.iid, s.cdid, s.openudid
}

func (s *HongguoService) runtimeIdentity() (userAgent, storeRegion string) {
	s.identityMu.RLock()
	defer s.identityMu.RUnlock()
	return s.userAgent, s.storeRegion
}

func (s *HongguoService) setUserAgent(userAgent string) {
	s.identityMu.Lock()
	s.userAgent = userAgent
	s.identityMu.Unlock()
}

func (s *HongguoService) authCredentials() (cookies, token string) {
	s.authMu.RLock()
	defer s.authMu.RUnlock()
	return s.userCookies, s.userToken
}

func (s *HongguoService) setAuthCredentials(cookies, token string, userIDs ...string) {
	previousCookies, _ := s.authCredentials()
	s.authMu.Lock()
	s.userCookies = cookies
	s.userToken = token
	s.authUserID = ""
	if len(userIDs) > 0 {
		s.authUserID = userIDs[0]
	}
	s.authMu.Unlock()
	if previousCookies != cookies && s.media != nil {
		// Playback entitlements can change with the account; never reuse a
		// playback negotiated under the previous credential state.
		s.media.ClearPlaybackCache()
	}
	s.syncAuthTransport()
}

func (s *HongguoService) authSession() (userID, cookies, token string) {
	s.authMu.RLock()
	defer s.authMu.RUnlock()
	return s.authUserID, s.userCookies, s.userToken
}

func (s *HongguoService) syncAuthTransport() {
	if s.authTransport == nil {
		return
	}
	cookies, token := s.authCredentials()
	_, storeRegion := s.runtimeIdentity()
	s.authTransport.SetCredentials(cookies, token, storeRegion)
}

func (s *HongguoService) reloadDeviceIdentity() error {
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
	s.applyDeviceProfile(profile)
	return nil
}

// applyDeviceProfile is called after SQLite has committed the SDK profile.
// It uses a separate lock from SDK registration so a device reset callback can
// update the in-memory identity while the old profile is being replaced.
func (s *HongguoService) applyDeviceProfile(profile *sdk.DeviceProfile) {
	s.identityMu.Lock()
	s.deviceID = profile.DeviceID
	s.iid = profile.InstallID
	s.cdid = profile.CDID
	s.openudid = profile.OpenUDID
	if profile.StoreRegion != "" {
		s.storeRegion = profile.StoreRegion
	}
	s.identityMu.Unlock()
	// Guest registration can establish the store region before the first
	// login; authenticated transports must sign with the updated region.
	s.syncAuthTransport()
}

func (s *HongguoService) invalidateUserDataCaches() {
	s.catalogService().Invalidate()
	s.libraryService().InvalidateUserData()
	s.cache.Reset()
}

func (s *HongguoService) GetProfile(ctx context.Context) (UserProfile, error) {
	p := s.store.GetProfile()
	if p.Avatar != "" {
		p.Avatar = s.media.ImageURL(p.Avatar)
	}
	return sanitizeIPC(p), nil
}

func (s *HongguoService) SetFeedScrollCapture(ctx context.Context, enabled bool) (bool, error) {
	native.SetFeedScrollCapture(enabled)
	ready := native.ScrollGestureMonitorRunning()
	log.Printf("[feed-input] capture enabled=%t monitorReady=%t", enabled, ready)
	return ready, nil
}

type RecommendFeedResult struct {
	Items   []SeriesCard `json:"items"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"hasMore"`
}

func (s *HongguoService) GetRecommendFeed(ctx context.Context, channel, cursor string) (*RecommendFeedResult, error) {
	// Return the first usable page immediately. The renderer fills its swipe
	// buffer separately; aggregating pages here delays cold-start playback.
	return s.getRecommendFeed(ctx, channel, cursor, 1)
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

	feedKey := feedChannelKey(channel)

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

	// Keep a short recent-generation memory. Filtering by the entire watch
	// history made an official infinite feed look exhausted after enough
	// viewing; three generations suppress immediate repeats while preserving a
	// renewable recommendation stream after cursor or anonymous-session loss.
	played := s.store.GetRecentFeedSeenIDs(feedKey, generation)
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
		played = s.store.GetRecentFeedSeenIDs(feedKey, generation)
		return generation, err
	}

	discoverOnce := func(cur string) (*sdk.Page[model.SeriesSummary], string, error) {
		discover := s.client.Discover
		if s.discover != nil {
			discover = s.discover
		}
		p, err := discover(ctx, sdk.DiscoverOptions{
			Channel: channel,
			Cursor:  cur,
		})
		if err != nil && (errors.Is(err, sdk.ErrAnonymousLimited) || strings.Contains(strings.ToLower(err.Error()), "anonymous")) {
			// A cursor belongs to the anonymous session. Resetting the device
			// therefore also invalidates it; continue from a fresh stream.
			resetDevice := s.client.ResetDevice
			if s.resetDevice != nil {
				resetDevice = s.resetDevice
			}
			_ = resetDevice(ctx)
			cur = ""
			p, err = discover(ctx, sdk.DiscoverOptions{
				Channel: channel,
				Cursor:  cur,
			})
			return p, cur, err
		}
		return p, cur, err
	}

	cardify := func(items []model.SeriesSummary) ([]SeriesCard, error) {
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
				CoverURL:     it.CoverURL,
				EpisodeCount: it.EpisodeCount,
				Category:     it.Category,
				Author:       it.Author,
				Status:       "finished",
				Heat:         "热播中",
				Tags:         tags,
			})
		}
		if err := s.queueRecommendCards(feedKey, cards); err != nil {
			return nil, fmt.Errorf("save recommend queue: %w", err)
		}
		for i := range cards {
			cards[i].CoverURL = s.media.ImageURL(cards[i].CoverURL)
		}
		return cards, nil
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
	canContinue := false
	// The main recommendation stream is effectively unbounded. Even when one
	// official page reports no follow-up, the next generation can yield fresh
	// cards; only the followed-dramas channel has a true hard boundary.
	const resultHasMore = true
	for attempt := 0; attempt < maxLookAheadPages; attempt++ {
		canContinue = false
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
		canContinue = page.HasMore && page.Cursor != ""
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
			newCards, err := cardify(unseen)
			if err != nil {
				return nil, err
			}
			cards = append(cards, newCards...)
		}
		if len(cards) >= minRecommendFeedBatch {
			return sanitizeIPC(&RecommendFeedResult{
				Items:   cards,
				Cursor:  page.Cursor,
				HasMore: resultHasMore,
			}), nil
		}
		if !canContinue {
			break
		}
		queryCursor = page.Cursor
	}
	// A full scan of repeated pages does not exhaust the stream. Let the next
	// request resume this cursor instead of resetting back to the same prefix.
	if len(cards) > 0 || canContinue {
		return sanitizeIPC(&RecommendFeedResult{
			Items:   cards,
			Cursor:  resultCursor,
			HasMore: resultHasMore,
		}), nil
	}

	// The anonymous stream has ended without yielding any unseen cards.
	// Register a new anonymous fingerprint and request a fresh stream.
	_, err := startGeneration()
	if err != nil {
		return nil, fmt.Errorf("failed to start a fresh recommend feed: %w", err)
	}
	resetDevice := s.client.ResetDevice
	if s.resetDevice != nil {
		resetDevice = s.resetDevice
	}
	if err := resetDevice(ctx); err != nil {
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
			newCards, err := cardify(unseen)
			if err != nil {
				return nil, err
			}
			cards = append(cards, newCards...)
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
	return sanitizeIPC(&RecommendFeedResult{Cursor: resultCursor, HasMore: true}), nil
}

func (s *HongguoService) GetRecommend(ctx context.Context) ([]SeriesCard, error) {
	cacheKey := "recommend"
	if entry, ok := s.cache.Get(cacheKey); ok {
		return sanitizeIPC(entry.Value.([]SeriesCard)), nil
	}

	res, err := s.GetRecommendFeed(ctx, "", "")
	if err != nil {
		return nil, err
	}

	s.cache.Set(cacheKey, resourcecache.Item{Value: res.Items, ExpiresAt: time.Now().Add(5 * time.Minute)})

	return res.Items, nil
}

// listEpisodesCached deduplicates concurrent SDK calls and avoids repeating the
// same series-detail request while a feed item is played and preloaded.
func (s *HongguoService) listEpisodesCached(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error) {
	s.episodeMu.Lock()
	s.ensureEpisodeCache()
	s.forgetExpiredEpisodes(time.Now())
	if s.episodeLoads == nil {
		s.episodeLoads = make(map[string]*episodeLoad)
	}
	if entry, ok := s.episodeCache[seriesID]; ok {
		s.touchEpisode(seriesID)
		s.episodeMu.Unlock()
		return entry.page, nil
	}
	if load, ok := s.episodeLoads[seriesID]; ok {
		s.episodeMu.Unlock()
		select {
		case <-load.done:
			return load.result.page, load.result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	load := &episodeLoad{done: make(chan struct{})}
	s.episodeLoads[seriesID] = load
	s.episodeMu.Unlock()

	page, err := s.client.ListEpisodes(ctx, seriesID, sdk.EpisodeOptions{})
	fetchedAt := time.Now()
	s.episodeMu.Lock()
	delete(s.episodeLoads, seriesID)
	if err == nil && page != nil && len(page.Items) > 0 {
		s.rememberEpisode(seriesID, page, fetchedAt)
	}
	load.result = episodeListResult{page: page, err: err}
	close(load.done)
	s.episodeMu.Unlock()
	return page, err
}

func (s *HongguoService) ensureEpisodeCache() {
	if s.episodeCache == nil {
		s.episodeCache = make(map[string]episodeCacheEntry)
	}
	if s.episodeCacheLRU == nil {
		s.episodeCacheLRU = list.New()
	}
	if s.episodeCacheIndex == nil {
		s.episodeCacheIndex = make(map[string]*list.Element)
	}
}

func (s *HongguoService) forgetExpiredEpisodes(now time.Time) {
	for id, entry := range s.episodeCache {
		if !now.Before(entry.expiresAt) {
			delete(s.episodeCache, id)
			if element := s.episodeCacheIndex[id]; element != nil {
				s.episodeCacheLRU.Remove(element)
				delete(s.episodeCacheIndex, id)
			}
		}
	}
}

func (s *HongguoService) touchEpisode(seriesID string) {
	if _, ok := s.episodeCache[seriesID]; !ok {
		return
	}
	if element := s.episodeCacheIndex[seriesID]; element != nil {
		s.episodeCacheLRU.MoveToBack(element)
	}
}

func (s *HongguoService) rememberEpisode(seriesID string, page *sdk.Page[model.Episode], now time.Time) {
	const episodeCacheTTL = 5 * time.Minute
	const maxEpisodeCacheEntries = 128

	s.episodeCache[seriesID] = episodeCacheEntry{page: page, expiresAt: now.Add(episodeCacheTTL)}
	if element := s.episodeCacheIndex[seriesID]; element != nil {
		s.episodeCacheLRU.MoveToBack(element)
	} else {
		s.episodeCacheIndex[seriesID] = s.episodeCacheLRU.PushBack(seriesID)
	}
	for len(s.episodeCache) > maxEpisodeCacheEntries {
		oldest := s.episodeCacheLRU.Front()
		if oldest == nil {
			break
		}
		oldestID, _ := oldest.Value.(string)
		delete(s.episodeCache, oldestID)
		s.episodeCacheLRU.Remove(oldest)
		delete(s.episodeCacheIndex, oldestID)
	}
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
