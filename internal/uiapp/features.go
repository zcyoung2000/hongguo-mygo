package uiapp

import (
	"context"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
	"myguo/internal/native"
)

type featureProvider interface {
	GetRankings(context.Context, string, string, string) (ipc.RankResponse, error)
	GetNewReleases(context.Context, string) (ipc.NewReleaseResponse, error)
	GetActorProfile(context.Context, string) (*ipc.ActorProfileDTO, error)
	GetActorWorks(context.Context, string, string) (*ipc.ActorWorksResponse, error)
	GetSuggestions(context.Context, string) ([]string, error)
	GetSeriesSeasons(context.Context, string) (*ipc.SeriesCollectionDTO, error)
	GetSeriesAdaptations(context.Context, string, string, string, string, string) (*ipc.SeriesCollectionDTO, error)
}
type RankingState struct {
	Contents               []model.RankContent
	Content, Board, Cursor string
	Items                  []ipc.RankItemDTO
	List                   *ui.ListState
	Loading, HasMore       bool
	Error                  string
	version                uint64
	seenCursors            map[string]bool
}
type ReleaseState struct {
	Calendar bool
	Date     string
	Dates    []string
	Items    []CatalogItem
	Grid     *ui.GridState
	Loading  bool
	Error    string
	version  uint64
}
type ActorState struct {
	ID               string
	Profile          *ipc.ActorProfileDTO
	Works            []CatalogItem
	Cursor           string
	WorksGrid        *ui.GridState
	HasMore, Loading bool
	Error            string
	version          uint64
	seenCursors      map[string]bool
	requesting       bool
}
type DetailCollection struct {
	Title   string
	Items   []CatalogItem
	Loading bool
	Error   string
}
type navigationEntry struct {
	View    string
	Item    CatalogItem
	ActorID string
}

func (a *App) featureProvider() featureProvider {
	if a.featuresSource != nil {
		return a.featuresSource
	}
	return a.service
}
func (a *App) SetMediaHost(host *MediaHost) { a.mu.Lock(); defer a.mu.Unlock(); a.media = host }

func (a *App) TogglePlayerFixture() { a.mu.Lock(); defer a.mu.Unlock(); a.playerFixture = true }

func (a *App) MediaCommand(payload native.MediaCommandPayload) {
	a.mu.Lock()
	host, active := a.media, a.mediaOn || a.view == "recommend"
	a.mu.Unlock()
	if host != nil && active {
		host.Command(payload)
	}
}
func (a *App) CloseMedia() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeMediaLocked()
}
func (a *App) ClosePlayer(session int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.mediaOn || session != a.mediaSession {
		return
	}
	a.closeMediaLocked()
}
func (a *App) CloseMediaSession(session int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.mediaOn || session != a.mediaSession {
		return
	}
	a.closeMediaLocked()
}
func (a *App) closeMediaLocked() {
	a.mediaOn = false
	a.resume = CatalogItem{}
	if a.view == "detail" && !a.testMode && a.detail.ID != "" {
		if a.media != nil {
			a.media.Stop()
		}
		go a.loadDetailAt(a.detail.ID, a.generation)
	} else if a.media != nil {
		a.media.Stop()
	}
	a.invalidate()
}
func (a *App) PlayDetail() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.detail.Loading || a.detail.ID == "" || a.detail.Upcoming {
		return
	}
	num, position := 1, float64(-1)
	if a.resume.ID == a.detail.ID && a.resume.ResumeEpisode > 0 {
		num, position = a.resume.ResumeEpisode, a.resume.ResumePosition
	}
	a.playLocked(num, position)
}
func (a *App) playLocked(num int, position float64) {
	if a.media != nil {
		a.mediaOn = true
		a.mediaSession++
		a.media.Play(a.detail.ID, num, position, a.mediaSession)
	}
	a.invalidate()
}
func (a *App) LibraryChanged() {
	a.mu.Lock()
	a.invalidateLibraryReadsLocked()
	a.bumpDetailRevisionLocked("favorite:" + a.detail.ID)
	a.mu.Unlock()
}
func (a *App) navigateLocked(entry navigationEntry, record bool) {
	if record {
		if a.navIndex < 0 || len(a.navigation) == 0 {
			a.navigation = []navigationEntry{entry}
			a.navIndex = 0
		} else {
			a.navigation = append(a.navigation[:a.navIndex+1], entry)
			a.navIndex++
		}
	}
	a.started = true
	a.open(entry.View, entry.Item.ID)
	switch entry.View {
	case "detail":
		a.resume = entry.Item
		a.detail = Detail{ID: entry.Item.ID, Loading: true, RelatedLoading: true}
		if !a.testMode {
			go a.loadDetailAt(entry.Item.ID, a.generation)
		}
	case "actor":
		a.actor = ActorState{ID: entry.ActorID, Loading: true, version: a.actor.version + 1}
		if !a.testMode {
			go a.loadActor(false)
		}
	case "recommend":
		if a.media != nil {
			a.media.RecommendChannel(a.feedChannel)
		}
	case "settings":
		if a.media != nil {
			a.media.Settings()
		}
	case "discover":
		if !a.testMode {
			go a.loadFilters()
			go a.loadCatalog(false)
		}
	case "rank":
		if !a.testMode {
			go a.LoadRank(false)
		}
	case "new":
		if !a.testMode {
			go a.LoadReleases()
		}
	case "history":
		if !a.testMode {
			go a.loadLibrary(libraryHistory)
		}
	case "favorites":
		if !a.testMode {
			go a.loadLibrary(libraryFavorites)
		}
	case "likes":
		if !a.testMode {
			go a.loadLibrary(libraryLikes)
		}
	case "reservations":
		if !a.testMode {
			go a.loadLibrary(libraryReservations)
		}
	}
	a.invalidate()
}
func (a *App) NavigateHistory(delta int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mediaOn {
		a.mediaOn = false
		if a.media != nil {
			a.media.Stop()
		}
		a.invalidate()
		return
	}
	next := a.navIndex + delta
	if next < 0 || next >= len(a.navigation) {
		if delta < 0 && (a.view == "detail" || a.view == "actor") {
			a.navigateLocked(navigationEntry{View: "discover"}, true)
		}
		return
	}
	a.navIndex = next
	a.navigateLocked(a.navigation[next], false)
}
func (a *App) OpenActor(id string) {
	if id == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.navigateLocked(navigationEntry{View: "actor", ActorID: id}, true)
}
func (a *App) SelectRank(content, board string) {
	a.mu.Lock()
	a.rank.Content, a.rank.Board = content, board
	a.rank.version++
	a.rank.Loading = false
	a.rank.Items, a.rank.Cursor, a.rank.Error = nil, "", ""
	a.rank.HasMore = false
	a.mu.Unlock()
	a.LoadRank(false)
}
func (a *App) LoadRank(more bool) {
	a.mu.Lock()
	if a.rank.Loading || (more && !a.rank.HasMore) {
		a.mu.Unlock()
		return
	}
	a.rank.Loading = true
	a.rank.Error = ""
	version := a.rank.version
	provider := a.featureProvider()
	content, board, cursor := a.rank.Content, a.rank.Board, ""
	if more {
		cursor = a.rank.Cursor
	}
	a.invalidate()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	result, err := provider.GetRankings(ctx, content, board, cursor)
	a.mu.Lock()
	defer a.mu.Unlock()
	if version != a.rank.version {
		return
	}
	a.rank.Loading = false
	if err != nil {
		a.rank.Error = err.Error()
		a.invalidate()
		return
	}
	items := result.Items
	if more {
		items = append(append([]ipc.RankItemDTO(nil), a.rank.Items...), items...)
	}
	a.rank.Contents, a.rank.Content, a.rank.Board = result.Contents, result.Content, result.Board
	a.rank.Items, a.rank.Cursor, a.rank.HasMore, a.rank.Error = dedupeRankItems(items), result.Cursor, nextCursor(&a.rank.seenCursors, cursor, result.Cursor, result.HasMore, more), ""
	a.invalidate()
}
func (a *App) SelectRelease(calendar bool, date string) {
	a.mu.Lock()
	a.releases.Calendar, a.releases.Date = calendar, date
	a.releases.version++
	a.releases.Loading = false
	a.mu.Unlock()
	a.LoadReleases()
}
func (a *App) LoadReleases() {
	a.mu.Lock()
	if a.releases.Loading {
		a.mu.Unlock()
		return
	}
	a.releases.Loading = true
	a.releases.Error = ""
	calendar, date, version, provider := a.releases.Calendar, a.releases.Date, a.releases.version, a.featureProvider()
	a.invalidate()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	var items []CatalogItem
	var dates []string
	var selected string
	var err error
	if calendar {
		var result ipc.NewReleaseResponse
		result, err = provider.GetNewReleases(ctx, date)
		dates, selected = result.Dates, result.CurrentDate
		for _, it := range result.Items {
			items = append(items, CatalogItem{ID: it.SeriesID, Title: it.Title, CoverURL: it.CoverURL, EpisodeCount: it.EpisodeCount, Heat: it.Heat, Description: it.Note, Category: it.Category, Upcoming: !it.Online, Reserved: it.Reserved, PublishAt: selected})
		}
	} else {
		var result ipc.RankResponse
		result, err = provider.GetRankings(ctx, "all", "ranklist_new_rank_sc", "")
		for _, it := range result.Items {
			if it.SeriesID != "" && it.ItemType != "actor" {
				items = append(items, CatalogItem{ID: it.SeriesID, Title: it.Title, CoverURL: it.CoverURL, EpisodeCount: it.EpisodeCount, Heat: it.Heat, Description: it.Description})
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if version != a.releases.version {
		return
	}
	a.releases.Loading = false
	if err != nil {
		a.releases.Error = err.Error()
	} else {
		a.releases.Items, a.releases.Dates, a.releases.Error = items, dates, ""
		if a.releases.Date == "" {
			a.releases.Date = selected
		}
	}
	a.invalidate()
}
func (a *App) loadActor(more bool) {
	a.mu.Lock()
	id, version, provider, cursor := a.actor.ID, a.actor.version, a.featureProvider(), a.actor.Cursor
	if a.actor.requesting || more && !a.actor.HasMore {
		a.mu.Unlock()
		return
	}
	a.actor.Loading, a.actor.requesting = true, true
	a.actor.Error = ""
	a.invalidate()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	var profile *ipc.ActorProfileDTO
	var works []CatalogItem
	var hasMore bool
	var next string
	var err error
	if more {
		var result *ipc.ActorWorksResponse
		result, err = provider.GetActorWorks(ctx, id, cursor)
		if result != nil {
			works, hasMore, next = convertCards(result.Works), result.HasMore, result.Cursor
		}
	} else {
		profile, err = provider.GetActorProfile(ctx, id)
		if profile != nil {
			works, hasMore, next = convertCards(profile.Works), profile.HasMore, profile.Cursor
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if id != a.actor.ID || version != a.actor.version {
		return
	}
	a.actor.Loading, a.actor.requesting = false, false
	if err != nil {
		a.actor.Error = err.Error()
	} else {
		if more {
			works = append(append([]CatalogItem(nil), a.actor.Works...), works...)
		} else {
			a.actor.Profile = profile
		}
		a.actor.Works, a.actor.Cursor, a.actor.HasMore, a.actor.Error = dedupeCards(works), next, nextCursor(&a.actor.seenCursors, cursor, next, hasMore, more), ""
	}
	a.invalidate()
}
func (a *App) loadSupplement(seriesID string, generation uint64, adaptations bool) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	a.mu.Lock()
	source := a.detail.Source
	excludeIDs := []string{seriesID}
	for _, item := range a.detail.Seasons.Items {
		excludeIDs = append(excludeIDs, item.ID)
	}
	provider := a.featureProvider()
	a.mu.Unlock()
	var result *ipc.SeriesCollectionDTO
	var err error
	if adaptations {
		sourceID, title, cover := "", "", ""
		if source != nil {
			sourceID, title, cover = source.ID, source.Title, source.CoverURL
		}
		result, err = provider.GetSeriesAdaptations(ctx, seriesID, sourceID, title, cover, strings.Join(excludeIDs, ","))
	} else {
		result, err = provider.GetSeriesSeasons(ctx, seriesID)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.generation {
		return
	}
	value := DetailCollection{}
	if err != nil {
		value.Error = err.Error()
	} else if result != nil {
		value.Title, value.Items = result.Title, convertCards(result.Items)
	}
	if adaptations {
		a.detail.Adaptations = value
	} else {
		a.detail.Seasons = value
		go a.loadSupplement(seriesID, generation, true)
	}
	a.invalidate()
}
func (a *App) suggest(query string, version uint64) {
	time.Sleep(200 * time.Millisecond)
	a.mu.Lock()
	if version != a.suggestionVersion || a.query != query {
		a.mu.Unlock()
		return
	}
	provider := a.featureProvider()
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	items, err := provider.GetSuggestions(ctx, query)
	a.mu.Lock()
	defer a.mu.Unlock()
	if version != a.suggestionVersion || a.query != query {
		return
	}
	if err == nil {
		a.suggestions = items
		a.suggestionIndex = -1
	}
	a.invalidate()
}
func (a *App) SelectSuggestion(delta int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.suggestions) == 0 {
		return
	}
	a.suggestionIndex = (a.suggestionIndex + delta + len(a.suggestions)) % len(a.suggestions)
	a.query = a.suggestions[a.suggestionIndex]
	a.invalidate()
}
func dedupeCards(items []CatalogItem) []CatalogItem {
	out := make([]CatalogItem, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		if !seen[it.ID] {
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	return out
}

// Existing library mutations use existing exported services and authentic rows.
type libraryMutations interface {
	ToggleLike(context.Context, string, string, string, string, int) (bool, error)
	ToggleFavorite(context.Context, string, string, string, string, int) (bool, error)
	ToggleReservation(context.Context, string, string, string, string, string) (bool, error)
}

func (a *App) RemoveLibraryItem(tab LibraryTab, item CatalogItem) {
	if tab == libraryHistory {
		return
	}
	a.mu.Lock()
	kind := map[LibraryTab]string{libraryHistory: "history", libraryFavorites: "favorite", libraryLikes: "like", libraryReservations: "reservation"}[tab]
	key := kind + ":" + item.ID
	if tab == libraryLikes {
		key += ":" + item.EpisodeID
	}
	if !a.beginMutationLocked(key) {
		a.mu.Unlock()
		return
	}
	confirm, provider := a.confirm, a.libraryMutations
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.endMutationLocked(key); a.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	if provider == nil {
		provider = a.service
	}
	var err error
	switch tab {
	case libraryFavorites:
		_, err = provider.ToggleFavorite(ctx, item.ID, item.Title, item.CoverURL, item.Description, item.EpisodeCount)
	case libraryLikes:
		_, err = provider.ToggleLike(ctx, item.ID, item.EpisodeID, item.Title, item.CoverURL, item.ResumeEpisode)
	case libraryReservations:
		if confirm != nil && !confirm("确认取消预约《"+item.Title+"》？") {
			return
		}
		_, err = provider.ToggleReservation(ctx, item.ID, item.Title, item.CoverURL, item.Description, item.PublishAt)
	}
	a.mu.Lock()
	if err != nil {
		a.fail("删除失败：", err)
		a.mu.Unlock()
		return
	}
	if tab == libraryReservations {
		a.applyReservationLocked(item.ID, false)
	}
	if tab == libraryFavorites && a.detail.ID == item.ID {
		a.detail.Favorite = false
	}
	a.libraryGeneration[tab]++
	library := a.libraryAt(tab)
	rows := make([]CatalogItem, 0, len(library.Items))
	for _, row := range library.Items {
		if row.ID == item.ID && (tab != libraryLikes || row.EpisodeID == item.EpisodeID) {
			continue
		}
		rows = append(rows, row)
	}
	library.Items, library.Empty = rows, len(rows) == 0
	a.invalidate()
	a.mu.Unlock()
}
func (a *App) SetLibraryFilter(filter, query string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.historyFilter, a.historyQuery = filter, query
	a.invalidate()
}
func (a *App) visibleLibrary(tab LibraryTab) []CatalogItem {
	library := a.libraryAt(tab)
	items := make([]CatalogItem, 0, len(library.Items))
	for _, it := range library.Items {
		if tab == libraryHistory {
			if a.historyFilter == "completed" && !it.Completed || a.historyFilter == "unwatched" && it.Completed {
				continue
			}
			if !strings.Contains(strings.ToLower(it.Title), strings.ToLower(strings.TrimSpace(a.historyQuery))) {
				continue
			}
		}
		if tab == libraryReservations && it.Upcoming != a.reservationsPending {
			continue
		}
		items = append(items, it)
	}
	return items
}
func (a *App) ToggleItemReservation(item CatalogItem) {
	a.mu.Lock()
	key := "reservation:" + item.ID
	if !a.beginMutationLocked(key) {
		a.mu.Unlock()
		return
	}
	confirm, provider := a.confirm, a.favoriteProvider()
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.endMutationLocked(key); a.mu.Unlock() }()
	if item.Reserved && confirm != nil && !confirm("确认取消预约《"+item.Title+"》？") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	enabled, err := provider.ToggleReservation(ctx, item.ID, item.Title, item.CoverURL, item.Description, item.PublishAt)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.fail("预约失败：", err)
		return
	}
	a.applyReservationLocked(item.ID, enabled)
	a.invalidate()
}

func (a *App) SetConfirmation(confirm func(string) bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.confirm = confirm
}

func (a *App) ExitMedia() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mediaOn = false
	a.resume = CatalogItem{}
	if a.media != nil {
		a.media.Stop()
	}
	if a.view == "detail" && !a.testMode {
		go a.loadDetailAt(a.detail.ID, a.generation)
	}
	a.invalidate()
}
func (a *App) SetHistoryQuery(query string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.historyQuery = query
	a.invalidate()
}

func (a *App) RetrySupplement(adaptations bool) {
	a.mu.Lock()
	id, generation := a.detail.ID, a.generation
	a.mu.Unlock()
	a.loadSupplement(id, generation, adaptations)
}
func (a *App) searchTextKey(k ui.TextKey) bool {
	if k.Key == ui.KeyEscape {
		a.mu.Lock()
		a.suggestions = nil
		a.suggestionVersion++
		a.mu.Unlock()
		return true
	}
	if k.Key != ui.KeyDown && k.Key != ui.KeyUp {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.suggestions) == 0 {
		return false
	}
	delta := 1
	if k.Key == ui.KeyUp {
		delta = -1
	}
	a.suggestionIndex = (a.suggestionIndex + delta + len(a.suggestions)) % len(a.suggestions)
	a.query = a.suggestions[a.suggestionIndex]
	k.SetText(a.query)
	return true
}

type libraryRefresher interface {
	RefreshHistory(context.Context) ([]ipc.HistoryItem, error)
	RefreshFavorites(context.Context) ([]ipc.FavoriteItem, error)
	RefreshLikes(context.Context) ([]ipc.LikeItem, error)
	RefreshReservations(context.Context) ([]ipc.ReservationItem, error)
}

func reservationBefore(a, b string) bool {
	parse := func(value string) time.Time {
		for _, layout := range []string{time.RFC3339, "2006-01-02", "20060102"} {
			if date, err := time.Parse(layout, value); err == nil {
				return date
			}
		}
		return time.Time{}
	}
	first, second := parse(a), parse(b)
	if first.IsZero() {
		return false
	}
	return second.IsZero() || first.Before(second)
}

// Cursor history belongs to a result set, so a cycle across several pages stops
// pagination without suppressing a retry of a failed request.
func nextCursor(seen *map[string]bool, current, next string, hasMore, more bool) bool {
	if !more || *seen == nil {
		*seen = map[string]bool{}
	}
	(*seen)[current] = true
	return hasMore && next != "" && !(*seen)[next]
}
func dedupeRankItems(items []ipc.RankItemDTO) []ipc.RankItemDTO {
	out := make([]ipc.RankItemDTO, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		id := it.SeriesID
		if it.ItemType == "actor" {
			id = it.UserID
			if id == "" {
				id = it.ActorID
			}
		}
		key := it.ItemType + ":" + id
		if id != "" && !seen[key] {
			seen[key] = true
			out = append(out, it)
		}
	}
	return out
}
func (a *App) beginMutationLocked(key string) bool {
	if a.mutations[key] {
		return false
	}
	if a.mutations == nil {
		a.mutations = map[string]bool{}
	}
	a.mutations[key] = true
	a.bumpDetailRevisionLocked(key)
	a.invalidateLibraryReadsLocked()
	return true
}
func (a *App) endMutationLocked(key string) {
	delete(a.mutations, key)
	a.bumpDetailRevisionLocked(key)
	a.invalidateLibraryReadsLocked()
	a.invalidate()
}
func (a *App) invalidateLibraryReadsLocked() {
	for tab := libraryHistory; tab <= libraryReservations; tab++ {
		a.libraryGeneration[tab]++
		a.libraryAt(tab).Loading = false
	}
}
func (a *App) applyReservationLocked(id string, enabled bool) {
	update := func(items []CatalogItem) []CatalogItem {
		rows := append([]CatalogItem(nil), items...)
		for i := range rows {
			if rows[i].ID == id {
				rows[i].Reserved = enabled
			}
		}
		return rows
	}
	a.releases.Items = update(a.releases.Items)
	a.catalog.Items = update(a.catalog.Items)
	a.search.Items = update(a.search.Items)
	if a.detail.ID == id {
		a.detail.Reserved = enabled
	}
}
func (a *App) RetryRelated() {
	a.mu.Lock()
	id, generation := a.detail.ID, a.generation
	if id == "" || a.detail.RelatedLoading {
		a.mu.Unlock()
		return
	}
	a.detail.RelatedLoading = true
	a.detail.RelatedError = ""
	a.invalidate()
	a.mu.Unlock()
	a.loadRelatedAt(id, generation)
}

func (a *App) bumpDetailRevisionLocked(key string) {
	kind, id, ok := strings.Cut(key, ":")
	if ok && (kind == "favorite" || kind == "reservation") {
		if a.detailRevisions == nil {
			a.detailRevisions = map[string]uint64{}
		}
		a.detailRevisions[id]++
	}
}
