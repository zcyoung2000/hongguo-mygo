package uiapp

import (
	"context"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
	"myguo/internal/native"
)

const serviceTimeout = 15 * time.Second

type CatalogItem struct {
	Kind                          string
	BadgeText, Author, Status     string
	EpisodeID                     string
	ActorID                       string
	Upcoming, Reserved, Completed bool
	PublishAt                     string
	UpdatedAt                     int64
	Progress                      float64
	ResumeEpisode                 int
	ResumePosition                float64
	ID                            string
	Title                         string
	Description                   string
	CoverURL                      string
	EpisodeCount                  int
	Category                      string
	Rank                          int
	Heat                          string
	Subtitles                     []string
}

type CatalogPage struct {
	Items   []CatalogItem
	Cursor  string
	HasMore bool
	Loading bool
	Error   string
}

type Detail struct {
	Source               *ipc.SourceNovelDTO
	Actors               []ipc.ActorInfo
	Upcoming             bool
	Heat                 string
	FollowCount          int64
	LikeCount            int64
	PublishText          string
	RelatedError         string
	Seasons, Adaptations DetailCollection
	ID                   string
	Title                string
	Description          string
	CoverURL             string
	Tags                 []string
	StatusText           string
	Category             string
	PlayCount            int64
	EpisodeCount         int
	Episodes             []CatalogItem
	Favorite             bool
	Reserved             bool
	Related              []CatalogItem
	RelatedTitle         string
	RelatedLoading       bool
	Loading              bool
	Error                string
}

type FilterOption struct {
	Label string
	Value string
}

type FilterRow struct {
	Title   string
	Key     string
	Options []FilterOption
}

type Library struct {
	Title       string
	Description string
	Empty       bool
	Loaded      bool
	Loading     bool
	Error       string
	Items       []CatalogItem
}

type Catalog struct {
	GenreIndex  int
	Genre       []string
	Filters     []FilterRow
	Selection   []int
	Loading     bool
	LoadingMore bool
	Error       string
	HasMore     bool
	Cursor      string
	seenCursors map[string]bool
	Items       []CatalogItem
}

type discoverSection int

const (
	discoverExpanded discoverSection = iota
	discoverCollapsed
)

type LibraryTab int

const (
	libraryHistory LibraryTab = iota
	libraryFavorites
	libraryLikes
	libraryReservations
)

type LibraryView struct {
	Tab       LibraryTab
	History   Library
	Favorites Library
	Reserved  Library
	Likes     Library
	Grids     [4]*ui.GridState
	Grid      int
	Selected  int
}

type Settings struct {
	Profile        ipc.UserProfile
	CacheSize      int64
	DeviceID       string
	DeviceVersion  string
	DeviceArch     string
	LegacyNickname string
	LegacyUserID   string
	HasLegacy      bool
	Loading        bool
	LoginOpen      bool
	LoginCookie    string
	LoginToken     string
	Error          string
}

// App state and all render snapshots are protected by mu. Network work never holds it.
type App struct {
	ambientActive               bool
	ambientLeft                 ui.Color
	ambientTop                  ui.Color
	feedChannel                 string
	feedChrome                  FeedChrome
	feedChromeKnown             bool
	discoverPageScroll          ui.ScrollState
	catalogScroll               ui.ScrollState
	rankScroll                  ui.ScrollState
	actorScroll                 ui.ScrollState
	playerFixture               bool
	detailEpisodesList          ui.ListState
	detailEpisodesScroll        ui.ScrollState
	detailScroll                ui.ScrollState
	detailLeftScroll            ui.ScrollState
	sidebarWidth                float32
	preferencesPath             string
	preferencesLoaded           bool
	preferencesPending          sync.WaitGroup
	preferencesMu               sync.Mutex
	media                       *MediaHost
	mediaOn                     bool
	mediaSession                int64
	featuresSource              featureProvider
	libraryMutations            libraryMutations
	confirm                     func(string) bool
	mutations                   map[string]bool
	settingsMutations           map[string]bool
	detailRevisions             map[string]uint64
	detailAdaptationKeys        map[string]string
	resumeSequence              uint64
	rank                        RankingState
	releases                    ReleaseState
	actor                       ActorState
	libraryVisible              [4][]CatalogItem
	searchGrid                  *ui.GridState
	catalogGrid                 *ui.GridState
	discoverFilterState         discoverSection
	releaseGrid                 *ui.GridState
	actorWorksGrid              *ui.GridState
	suggestions                 []string
	suggestionVersion           uint64
	suggestionIndex             int
	historyFilter, historyQuery string
	reservationsPending         bool
	navigation                  []navigationEntry
	navIndex                    int
	mu                          sync.Mutex
	generation                  uint64
	catalogGeneration           uint64
	libraryGeneration           [4]uint64
	started                     bool
	resume                      CatalogItem
	service                     *ipc.HongguoService
	artwork                     *artworkLoader
	invalidateFn                func()
	testMode                    bool
	favorite                    favoriteProvider
	detailSource                detailProvider

	view     string
	seriesID string
	loading  bool
	error    string
	notice   string

	query                string
	search               CatalogPage
	catalog              Catalog
	detail               Detail
	libraryData          libraryProvider
	settingsTestProvider settingsProvider
	library              LibraryView
	settings             Settings
	playerOn             bool
}

func NewApp(service *ipc.HongguoService) *App {
	return &App{
		service:    service,
		view:       "discover",
		catalog:    Catalog{Genre: []string{"真人剧", "漫剧", "AI剧"}, HasMore: true},
		rank:       RankingState{Content: "all", Board: "ranklist_hot_sc", HasMore: true},
		navigation: []navigationEntry{{View: "discover"}},
		navIndex:   0,
	}
}

type libraryProvider interface {
	GetHistory(ctx context.Context) ([]ipc.HistoryItem, error)
	GetFavorites(ctx context.Context) ([]ipc.FavoriteItem, error)
	GetReservations(ctx context.Context) ([]ipc.ReservationItem, error)
	GetLikes(ctx context.Context) ([]ipc.LikeItem, error)
}

type favoriteProvider interface {
	ToggleFavorite(ctx context.Context, seriesID, title, coverURL, desc string, totalEpisodes int) (bool, error)
	ToggleReservation(ctx context.Context, seriesID, title, coverURL, note, publishAt string) (bool, error)
}

type detailProvider interface {
	GetSeriesPage(ctx context.Context, seriesID string) (*ipc.SeriesDetailPageDTO, error)
	GetSeriesRelated(ctx context.Context, seriesID string) (*ipc.SeriesCollectionDTO, error)
	GetFilterPanel(ctx context.Context, genre string) ([]ipc.FilterRowDTO, error)
	GetDiscover(ctx context.Context, genre string, selections map[string]string, cursor string) (ipc.DiscoverResponse, error)
}

func NewTestApp() *App {
	return &App{
		view:     "discover",
		catalog:  Catalog{Genre: []string{"真人剧", "漫剧", "AI剧"}, HasMore: true},
		rank:     RankingState{Content: "all", Board: "ranklist_hot_sc", HasMore: true},
		testMode: true,
	}
}

func NewLibraryGrids() [4]*ui.GridState {
	return [4]*ui.GridState{{}, {}, {}, {}}
}

func (a *App) SetInvalidator(fn func()) {
	a.mu.Lock()
	a.invalidateFn = fn
	a.mu.Unlock()
}
func (a *App) SetLibraryGrids(grids [4]*ui.GridState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.library.Grids = grids
}
func (a *App) SetAmbientGlow(active bool, left, top [3]uint8) {
	a.mu.Lock()
	a.ambientActive = active
	if active {
		a.ambientLeft = ui.RGB(left[0], left[1], left[2])
		a.ambientTop = ui.RGB(top[0], top[1], top[2])
	}
	invalidate := a.invalidateFn
	a.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}
func (a *App) View() string              { a.mu.Lock(); defer a.mu.Unlock(); return a.view }
func (a *App) Query() string             { a.mu.Lock(); defer a.mu.Unlock(); return a.query }
func (a *App) GlobalError() string       { a.mu.Lock(); defer a.mu.Unlock(); return a.error }
func (a *App) Notice() string            { a.mu.Lock(); defer a.mu.Unlock(); return a.notice }
func (a *App) PlayerVisible() bool       { a.mu.Lock(); defer a.mu.Unlock(); return a.mediaOn }
func (a *App) SearchState() CatalogPage  { a.mu.Lock(); defer a.mu.Unlock(); return a.search }
func (a *App) CatalogState() Catalog     { a.mu.Lock(); defer a.mu.Unlock(); return a.catalog }
func (a *App) LibraryState() LibraryView { a.mu.Lock(); defer a.mu.Unlock(); return a.library }
func (a *App) DetailState() Detail       { a.mu.Lock(); defer a.mu.Unlock(); return a.detail }
func (a *App) SettingsState() Settings   { a.mu.Lock(); defer a.mu.Unlock(); return a.settings }
func (a *App) SeriesID() string          { a.mu.Lock(); defer a.mu.Unlock(); return a.seriesID }

// Called under mu, including by Render. Published slices are replaced, never edited.
func (a *App) catalogGenre() string {
	if a.catalog.GenreIndex < 0 || a.catalog.GenreIndex >= len(a.catalog.Genre) {
		return "short_play"
	}
	return []string{"short_play", "comic_series", "ai_series"}[a.catalog.GenreIndex]
}
func catalogValues(genre string, rows []FilterRow, selected []int) map[string]string {
	values := map[string]string{}
	for index, row := range rows {
		if index < len(selected) && selected[index] > 0 && selected[index] < len(row.Options) {
			values[row.Key] = row.Options[selected[index]].Value
		}
	}
	if len(values) == 0 {
		values["genre"] = genre
	}
	return values
}
func (a *App) SelectGenre(next int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if next == a.catalog.GenreIndex || next < 0 || next >= len(a.catalog.Genre) {
		return
	}
	a.catalog.GenreIndex = next
	a.catalog.Filters, a.catalog.Selection = nil, nil
	a.resetCatalog()
	if !a.testMode {
		go a.loadFilters()
		go a.loadCatalog(false)
	}
}
func (a *App) resetCatalog() {
	a.catalogGeneration++
	a.catalog.Items, a.catalog.Cursor, a.catalog.Error = nil, "", ""
	a.catalog.HasMore, a.catalog.Loading, a.catalog.LoadingMore = true, false, false
	a.catalog.seenCursors = nil
	a.discoverPageScroll.Y = 0
	a.invalidate()
}
func (a *App) SelectFilter(rowIndex, optionIndex int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if rowIndex < 0 || rowIndex >= len(a.catalog.Filters) || optionIndex < 0 || optionIndex >= len(a.catalog.Filters[rowIndex].Options) {
		return
	}
	selected := append([]int(nil), a.catalog.Selection...)
	for len(selected) < len(a.catalog.Filters) {
		selected = append(selected, 0)
	}
	if selected[rowIndex] == optionIndex {
		return
	}
	selected[rowIndex] = optionIndex
	a.catalog.Selection = selected
	a.resetCatalog()
	if !a.testMode {
		go a.loadCatalog(false)
	}
}
func (a *App) SearchSubmitted() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	term := strings.TrimSpace(a.query)
	if term == "" {
		return false
	}
	a.navigateLocked(navigationEntry{View: "search"}, true)
	a.suggestions = nil
	a.suggestionVersion++
	a.search = CatalogPage{Loading: true}
	generation := a.generation
	if !a.testMode {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
			defer cancel()
			items, err := a.service.Search(ctx, term)
			a.mu.Lock()
			defer a.mu.Unlock()
			if a.generation != generation {
				return
			}
			a.search = CatalogPage{Items: removeSearchPageCardLabels(convertCards(items))}
			if err != nil {
				a.search.Error = "搜索失败：" + err.Error()
			}
			a.invalidate()
		}()
	}
	return true
}
func (a *App) SearchChanged() bool { return false }
func (a *App) SetQuery(value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if value == a.query {
		return
	}
	a.query = value
	a.suggestionVersion++
	a.suggestions = nil
	a.suggestionIndex = -1
	if strings.TrimSpace(value) != "" && !a.testMode {
		go a.suggest(value, a.suggestionVersion)
	}
	a.invalidate()
}
func (a *App) OpenView(view string) {
	log.Printf("[uiapp] OpenView target=%s current=%s", view, a.view)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.view == view && a.started {
		return
	}
	a.navigateLocked(navigationEntry{View: view}, true)
}
func (a *App) OpenDetail(seriesID string) { a.OpenItem(CatalogItem{ID: seriesID}) }
func (a *App) OpenItem(item CatalogItem) {
	if item.ActorID != "" {
		a.OpenActor(item.ActorID)
		return
	}
	if item.ID == "" || item.Kind == "source_novel" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.navigateLocked(navigationEntry{View: "detail", Item: item}, true)
}
func (a *App) ToggleDetailFavorite()    { a.toggleDetail(false) }
func (a *App) ToggleDetailReservation() { a.toggleDetail(true) }
func (a *App) toggleDetail(reservation bool) {
	a.mu.Lock()
	detail, generation, provider := a.detail, a.generation, a.favoriteProvider()
	kind := "favorite"
	if reservation {
		kind = "reservation"
	}
	key := kind + ":" + detail.ID
	if detail.ID == "" || detail.Loading || !a.beginMutationLocked(key) {
		a.mu.Unlock()
		return
	}
	confirm := a.confirm
	a.mu.Unlock()
	go func() {
		if reservation && detail.Reserved && confirm != nil && !confirm("确认取消预约《"+detail.Title+"》？") {
			a.mu.Lock()
			a.endMutationLocked(key)
			a.mu.Unlock()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
		defer cancel()
		var enabled bool
		var err error
		if reservation {
			enabled, err = provider.ToggleReservation(ctx, detail.ID, detail.Title, detail.CoverURL, detail.Description, "")
		} else {
			enabled, err = provider.ToggleFavorite(ctx, detail.ID, detail.Title, detail.CoverURL, detail.Description, detail.EpisodeCount)
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		a.endMutationLocked(key)
		if err != nil {
			if a.generation == generation {
				a.fail("更新失败：", err)
			}
			return
		}
		if reservation {
			a.applyReservationLocked(detail.ID, enabled)
		} else if a.detail.ID == detail.ID {
			a.detail.Favorite = enabled
		}
		a.invalidate()
	}()
}
func (a *App) favoriteProvider() favoriteProvider {
	if a.favorite != nil {
		return a.favorite
	}
	return a.service
}
func (a *App) ChooseEpisode(num int) {
	a.mu.Lock()
	loaded := num > 0 && a.media != nil && !a.detail.Loading && a.detail.ID != "" && !a.detail.Upcoming
	if loaded {
		a.detailScroll.Y = 0
	}
	a.mu.Unlock()
	if !loaded {
		return
	}
	a.playLocked(num, -1)
}
func (a *App) OpenRelated(seriesID string) { a.OpenDetail(seriesID) }
func (a *App) LoadMoreCatalog() {
	if a.testMode && a.detailProvider() == nil {
		return
	}
	a.loadCatalog(true)
}
func (a *App) LoadMoreSearch() bool { return false }
func (a *App) RefreshLibrary(tab LibraryTab) {
	if !a.testMode {
		go a.loadLibraryWithRefresh(tab, true)
	}
}
func (a *App) PlayerResume() { a.MediaCommand(native.MediaCommandPayload{Command: "play"}) }
func (a *App) PlayerPause()  { a.MediaCommand(native.MediaCommandPayload{Command: "pause"}) }

func (a *App) PlayerPrevious() { a.MediaCommand(native.MediaCommandPayload{Command: "prev"}) }

func (a *App) PlayerNext() { a.MediaCommand(native.MediaCommandPayload{Command: "next"}) }
func (a *App) PlayerPictureInPicture() {
	a.MediaCommand(native.MediaCommandPayload{Command: "pip"})
}
func (a *App) loadFilters() {
	a.mu.Lock()
	genre, generation, provider := a.catalogGenre(), a.catalogGeneration, a.detailProvider()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	rows, err := provider.GetFilterPanel(ctx, genre)
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.catalogGeneration {
		return
	}
	if err != nil {
		a.fail("筛选加载失败：", err)
		return
	}
	a.catalog.Filters = convertFilters(rows)
	a.error = ""
	a.invalidate()
}
func (a *App) loadCatalog(more bool) {
	a.mu.Lock()
	if a.catalog.Loading || a.catalog.LoadingMore || (more && !a.catalog.HasMore) {
		a.mu.Unlock()
		return
	}
	genre, generation, provider := a.catalogGenre(), a.catalogGeneration, a.detailProvider()
	if provider == nil {
		a.mu.Unlock()
		return
	}
	a.catalog.Loading, a.catalog.LoadingMore = !more, more
	values := catalogValues(genre, a.catalog.Filters, a.catalog.Selection)
	cursor := ""
	if more {
		cursor = a.catalog.Cursor
	}
	a.invalidate()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	page, err := provider.GetDiscover(ctx, genre, values, cursor)
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.catalogGeneration {
		return
	}
	a.catalog.Loading, a.catalog.LoadingMore = false, false
	if err != nil {
		a.catalog.Error = "目录加载失败，请稍后重试"
		a.fail("目录加载失败：", err)
		return
	}
	items := convertCards(page.Items)
	if more {
		items = append(append([]CatalogItem(nil), a.catalog.Items...), items...)
	}
	a.error = ""
	a.catalog.Items, a.catalog.Cursor, a.catalog.HasMore, a.catalog.Error = dedupeCards(items), page.Cursor, nextCursor(&a.catalog.seenCursors, cursor, page.Cursor, page.HasMore, more), ""
	a.invalidate()
}
func (a *App) loadDetail(seriesID string) {
	a.mu.Lock()
	generation := a.generation
	a.mu.Unlock()
	a.loadDetailAt(seriesID, generation)
}
func (a *App) loadDetailAt(seriesID string, generation uint64) {
	a.mu.Lock()
	revision := a.detailRevisions[seriesID]
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	page, err := a.detailProvider().GetSeriesPage(ctx, seriesID)
	a.mu.Lock()
	if generation != a.generation {
		a.mu.Unlock()
		return
	}
	if revision != a.detailRevisions[seriesID] {
		a.mu.Unlock()
		go a.loadDetailAt(seriesID, generation)
		return
	}
	if err != nil || page == nil {
		a.detail.Loading, a.detail.Error = false, "详情加载失败，请稍后重试"
		a.invalidate()
		a.mu.Unlock()
		return
	}
	a.detail = convertDetail(seriesID, page)
	a.detail.RelatedLoading = true
	a.invalidate()
	a.mu.Unlock()
	if a.featuresSource != nil || a.service != nil {
		go a.loadSupplement(seriesID, generation, false)
	}
	a.loadRelatedAt(seriesID, generation)
}
func (a *App) loadRelatedAt(seriesID string, generation uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	collection, err := a.detailProvider().GetSeriesRelated(ctx, seriesID)
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.generation {
		return
	}
	a.detail.RelatedLoading = false
	a.detail.RelatedError = ""
	if err != nil || collection == nil {
		a.detail.RelatedError = "相关推荐加载失败"
	} else {
		a.detail.Related, a.detail.RelatedTitle = clearRelatedCollectionCardFields(convertCards(collection.Items)), collection.Title
	}
	a.invalidate()
}

func removeSearchPageCardLabels(items []CatalogItem) []CatalogItem {
	for index := range items {
		items[index].BadgeText = ""
		items[index].Category = ""
	}
	return items
}

func clearRelatedCollectionCardFields(items []CatalogItem) []CatalogItem {
	for i := range items {
		items[i].BadgeText = ""
		items[i].Category = ""
	}
	return items
}
func (a *App) detailProvider() detailProvider {
	if a.detailSource != nil {
		return a.detailSource
	}
	if a.service != nil {
		return a.service
	}
	return nil
}
func (a *App) libraryAt(tab LibraryTab) *Library {
	switch tab {
	case libraryHistory:
		return &a.library.History
	case libraryFavorites:
		return &a.library.Favorites
	case libraryLikes:
		return &a.library.Likes
	default:
		return &a.library.Reserved
	}
}

// Both tabs share a header, so entering either must load both real datasets.
func (a *App) loadLibrary(tab LibraryTab) {
	if tab == libraryFavorites || tab == libraryLikes {
		var pending sync.WaitGroup
		for _, collection := range []LibraryTab{libraryFavorites, libraryLikes} {
			pending.Add(1)
			go func(tab LibraryTab) {
				defer pending.Done()
				a.loadLibraryTab(tab)
			}(collection)
		}
		pending.Wait()
		return
	}
	a.loadLibraryTab(tab)
}

func libraryCountLabel(library Library) string {
	if library.Loaded || len(library.Items) > 0 || library.Empty {
		return strconv.Itoa(len(library.Items))
	}
	if library.Error != "" {
		return "—"
	}
	return "…"
}

func (a *App) loadLibraryTab(tab LibraryTab) {
	a.loadLibraryWithRefresh(tab, false)
	if !a.testMode && a.service != nil {
		a.loadLibraryWithRefresh(tab, true)
	}
}
func (a *App) loadLibraryWithRefresh(tab LibraryTab, refresh bool) {
	a.mu.Lock()
	a.libraryGeneration[tab]++
	generation := a.libraryGeneration[tab]
	library := a.libraryAt(tab)
	library.Loading, library.Error, library.Empty = true, "", false
	a.refreshVisibleLibraryLocked(tab)
	provider := a.libraryProvider()
	a.invalidate()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	var items []CatalogItem
	var err error
	if refresher, ok := provider.(libraryRefresher); refresh && ok {
		switch tab {
		case libraryHistory:
			items, err = convertHistory(refresher.RefreshHistory(ctx))
		case libraryFavorites:
			items, err = convertFavorites(refresher.RefreshFavorites(ctx))
		case libraryLikes:
			items, err = convertLikes(refresher.RefreshLikes(ctx))
		default:
			items, err = convertReservations(refresher.RefreshReservations(ctx))
		}
	} else {
		switch tab {
		case libraryHistory:
			items, err = convertHistory(provider.GetHistory(ctx))
		case libraryFavorites:
			items, err = convertFavorites(provider.GetFavorites(ctx))
		case libraryLikes:
			items, err = convertLikes(provider.GetLikes(ctx))
		default:
			items, err = convertReservations(provider.GetReservations(ctx))
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.libraryGeneration[tab] {
		return
	}
	library = a.libraryAt(tab)
	library.Loading = false
	if err != nil {
		library.Error = "数据加载失败，请稍后重试"
		log.Printf("[library] tab=%d load failed: %v", tab, err)
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		if tab == libraryReservations {
			return reservationBefore(items[i].PublishAt, items[j].PublishAt)
		}
		return items[i].Rank < items[j].Rank
	})
	a.error = ""
	library.Items, library.Empty, library.Loaded = items, len(items) == 0, true
	library.Title = map[LibraryTab]string{libraryHistory: "观看历史", libraryFavorites: "我的收藏", libraryLikes: "我的点赞", libraryReservations: "新剧预约"}[tab]
	a.refreshVisibleLibraryLocked(tab)
	a.invalidate()
}
func (a *App) libraryProvider() libraryProvider {
	if a.libraryData != nil {
		return a.libraryData
	}
	return a.service
}

// open, fail, setNotice and invalidate require mu. Player commands enqueue work and never wait for the UI thread.
func (a *App) open(view, seriesID string) {
	a.generation++
	a.view, a.seriesID, a.error, a.loading, a.playerOn = view, seriesID, "", false, false
	a.notice = ""
	a.resetResumeLocked(seriesID)
	a.mediaOn = false
	if a.media != nil {
		a.media.Stop()
	}
	a.detail = Detail{}
	a.invalidate()
}
func (a *App) fail(prefix string, err error) { a.error = prefix + err.Error(); a.invalidate() }
func (a *App) setNotice(message string)      { a.notice = message; a.invalidate() }
func (a *App) invalidate() {
	if a.invalidateFn != nil {
		a.invalidateFn()
	}
}

func convertCards(cards []ipc.SeriesCard) []CatalogItem {
	items := make([]CatalogItem, 0, len(cards))
	for _, card := range cards {
		items = append(items, CatalogItem{
			Kind: card.Kind, BadgeText: card.BadgeText, Author: card.Author, Status: card.Status, ID: card.ID, Title: card.Title, Description: card.Description, CoverURL: card.CoverURL,
			EpisodeCount: card.EpisodeCount, Category: card.Category, Heat: card.Heat, Subtitles: card.Tags, Upcoming: card.Upcoming, Reserved: card.Reserved, ActorID: actorCardID(card),
		})
	}
	return items
}

func convertFilters(rows []ipc.FilterRowDTO) []FilterRow {
	out := make([]FilterRow, 0, len(rows))
	for _, row := range rows {
		if row.Key == "genre" || len(row.Options) == 0 {
			continue
		}
		options := []FilterOption{{Label: "全部", Value: ""}}
		for _, option := range row.Options {
			options = append(options, FilterOption{Label: option.Name, Value: option.Value})
		}
		out = append(out, FilterRow{Title: strings.TrimPrefix(row.Name, "全部"), Key: row.Key, Options: options})
	}
	return out
}

func convertHistory(items []ipc.HistoryItem, err error) ([]CatalogItem, error) {
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(items))
	for index, item := range items {
		title := item.Title
		if title == "" {
			if item.EpisodeNum > 0 {
				title = "第" + strconv.Itoa(item.EpisodeNum) + "集"
			} else {
				title = "未知短剧"
			}
		}
		out = append(out, CatalogItem{
			ID: item.SeriesID, Title: title,
			CoverURL: item.CoverURL, EpisodeCount: item.TotalEpisodes, Rank: index,
			ResumeEpisode: item.EpisodeNum, ResumePosition: item.PositionSec,
			UpdatedAt: item.UpdatedAt, Progress: historyPercent(item.PositionSec, float64(item.DurationSec)), Completed: item.TotalEpisodes > 0 && item.EpisodeNum == item.TotalEpisodes && historyPercent(item.PositionSec, float64(item.DurationSec)) >= 95,
		})
	}
	return out, nil
}

func convertFavorites(items []ipc.FavoriteItem, err error) ([]CatalogItem, error) {
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(items))
	for index, item := range items {
		title := item.Title
		if title == "" {
			title = "未知短剧"
		}
		out = append(out, CatalogItem{
			ID: item.SeriesID, Title: title, Description: item.Description, CoverURL: item.CoverURL,
			EpisodeCount: item.TotalEpisodes, Rank: index,
		})
	}
	return out, nil
}

func convertReservations(items []ipc.ReservationItem, err error) ([]CatalogItem, error) {
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(items))
	for index, item := range items {
		title := item.Title
		if title == "" {
			title = "未知短剧"
		}
		out = append(out, CatalogItem{
			ID: item.SeriesID, Title: title, Description: item.Note, CoverURL: item.CoverURL,
			PublishAt: item.PublishAt, Upcoming: !item.IsOnline, Reserved: true, Rank: index,
		})
	}
	return out, nil
}

func convertLikes(items []ipc.LikeItem, err error) ([]CatalogItem, error) {
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(items))
	for index, item := range items {
		title := item.Title
		if title == "" {
			title = "未知短剧"
		}
		out = append(out, CatalogItem{
			ID: item.SeriesID, EpisodeID: item.EpisodeID, ResumeEpisode: item.EpisodeNum, ResumePosition: -1, Title: title, Description: item.Description, CoverURL: item.CoverURL,
			EpisodeCount: item.TotalEpisodes, Rank: index,
		})
	}
	return out, nil
}

func convertDetail(seriesID string, page *ipc.SeriesDetailPageDTO) Detail {
	episodes := make([]CatalogItem, 0, len(page.Episodes))
	for _, episode := range page.Episodes {
		episodes = append(episodes, CatalogItem{
			ID: episode.ID, Title: episode.Title, CoverURL: episode.PosterURL,
			EpisodeCount: episode.Number, Rank: episode.Number,
		})
	}
	return Detail{
		ID: seriesID, Title: page.Title, Description: page.Description, CoverURL: page.CoverURL,
		Source: page.Source, Actors: page.Actors, Upcoming: page.Upcoming || page.EpisodeCount == 0, Heat: page.Heat, FollowCount: page.FollowCount, LikeCount: page.LikeCount, PublishText: page.PublishText, Seasons: DetailCollection{Loading: true}, Adaptations: DetailCollection{Loading: true},
		Tags: page.Tags, StatusText: page.StatusText, Category: page.Category, PlayCount: page.PlayCount,
		EpisodeCount: page.EpisodeCount, Episodes: episodes, Favorite: page.IsFavorite, Reserved: page.Reserved,
		RelatedTitle: "相关推荐",
	}
}

func historyPercent(position, duration float64) float64 {
	if duration <= 0 {
		return 0
	}
	value := position * 100 / duration
	if value > 100 {
		return 100
	}
	if value < 0 {
		return 0
	}
	return value
}
func actorCardID(card ipc.SeriesCard) string {
	if card.Kind == "actor" {
		return card.ID
	}
	return ""
}
