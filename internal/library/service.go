package library

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	userstore "myguo/internal/store"
)

type (
	HistoryItem     = userstore.HistoryItem
	FavoriteItem    = userstore.FavoriteItem
	LikeItem        = userstore.LikeItem
	ReservationItem = userstore.ReservationItem
)

type Notification struct {
	Title    string
	Body     string
	SeriesID string
}

type Dependencies struct {
	Store            *userstore.Store
	Client           *sdk.Client
	ImageURL         func(string) string
	Credentials      func() (cookies, token string)
	DeviceIdentity   func() (deviceID, installID, cdid, openudid string)
	Runtime          func() (userAgent, storeRegion string)
	HTTPClient       *http.Client
	Episodes         func(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error)
	InvalidateCaches func(prefixes ...string)
	Notify           func(ctx context.Context, notification Notification)
}

type Service struct {
	store            *userstore.Store
	client           *sdk.Client
	imageURL         func(string) string
	credentials      func() (cookies, token string)
	deviceIdentity   func() (deviceID, installID, cdid, openudid string)
	runtimeInfo      func() (userAgent, storeRegion string)
	httpClient       *http.Client
	episodes         func(ctx context.Context, seriesID string) (*sdk.Page[model.Episode], error)
	invalidateCaches func(prefixes ...string)
	notify           func(ctx context.Context, notification Notification)

	historyMu              sync.Mutex
	cachedHistory          []HistoryItem
	historyCachedAt        time.Time
	historySyncing         bool
	historyGeneration      uint64
	historyMutationMu      sync.Mutex
	historyMutationSyncing bool
	historyMutationWake    bool

	favMu           sync.Mutex
	cachedFavorites []FavoriteItem
	favCachedAt     time.Time
	favSyncing      bool
	favGeneration   uint64
	favMutationMu   sync.Mutex

	likeMu         sync.Mutex
	cachedLikes    []LikeItem
	likeCachedAt   time.Time
	likeSyncing    bool
	likeGeneration uint64
	backgroundWG   sync.WaitGroup

	resMu              sync.Mutex
	cachedReservations []ReservationItem
	resCachedAt        time.Time
	resSyncing         bool
	resGeneration      uint64
}

func (s *Service) WaitForBackgroundSync() { s.backgroundWG.Wait() }

func NewService(deps Dependencies) *Service {
	if deps.ImageURL == nil {
		deps.ImageURL = func(raw string) string { return raw }
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
	if deps.HTTPClient == nil {
		deps.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if deps.Episodes == nil {
		deps.Episodes = func(context.Context, string) (*sdk.Page[model.Episode], error) {
			return nil, errors.New("episode provider unavailable")
		}
	}
	if deps.InvalidateCaches == nil {
		deps.InvalidateCaches = func(prefixes ...string) {}
	}
	if deps.Notify == nil {
		deps.Notify = func(context.Context, Notification) {}
	}
	return &Service{
		store:            deps.Store,
		client:           deps.Client,
		imageURL:         deps.ImageURL,
		credentials:      deps.Credentials,
		deviceIdentity:   deps.DeviceIdentity,
		runtimeInfo:      deps.Runtime,
		httpClient:       deps.HTTPClient,
		episodes:         deps.Episodes,
		invalidateCaches: deps.InvalidateCaches,
		notify:           deps.Notify,
	}
}

func (s *Service) InvalidateUserData() {
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

func (s *Service) currentHistoryGeneration() uint64 {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	return s.historyGeneration
}

func (s *Service) currentFavoriteGeneration() uint64 {
	s.favMu.Lock()
	defer s.favMu.Unlock()
	return s.favGeneration
}

func (s *Service) currentLikeGeneration() uint64 {
	s.likeMu.Lock()
	defer s.likeMu.Unlock()
	return s.likeGeneration
}

func (s *Service) currentReservationGeneration() uint64 {
	s.resMu.Lock()
	defer s.resMu.Unlock()
	return s.resGeneration
}

func (s *Service) userAgentValue() string {
	userAgent, _ := s.runtimeInfo()
	return userAgent
}

func (s *Service) storeRegionValue() string {
	_, storeRegion := s.runtimeInfo()
	return storeRegion
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
