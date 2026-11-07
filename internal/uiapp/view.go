package uiapp

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

//go:embed icons/discover.svg
var discoverSVG []byte

//go:embed icons/search.svg
var searchSVG []byte

//go:embed icons/history.svg
var historySVG []byte

//go:embed icons/favorite.svg
var favoriteSVG []byte

//go:embed icons/reservation.svg
var reservationSVG []byte

//go:embed icons/recommend.svg
var recommendSVG []byte

//go:embed icons/like.svg
var likeSVG []byte

//go:embed icons/rank.svg
var rankSVG []byte

//go:embed icons/new.svg
var newSVG []byte

//go:embed icons/settings.svg
var settingsSVG []byte

//go:embed icons/episodes.svg
var episodesSVG []byte

//go:embed icons/close.svg
var closeSVG []byte

var (
	rankIcon        = ui.MustParseSVG(rankSVG)
	newIcon         = ui.MustParseSVG(newSVG)
	settingsIcon    = ui.MustParseSVG(settingsSVG)
	likeIcon        = ui.MustParseSVG(likeSVG)
	recommendIcon   = ui.MustParseSVG(recommendSVG)
	discoverIcon    = ui.MustParseSVG(discoverSVG)
	searchIcon      = ui.MustParseSVG(searchSVG)
	historyIcon     = ui.MustParseSVG(historySVG)
	favoriteIcon    = ui.MustParseSVG(favoriteSVG)
	reservationIcon = ui.MustParseSVG(reservationSVG)
	episodesIcon    = ui.MustParseSVG(episodesSVG)
	closeIcon       = ui.MustParseSVG(closeSVG)
)

func (a *App) Render(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.player != nil {
		a.player.beginArtworkFrame()
	}
	theme := hongguoTheme(c)
	c.SetTheme(&theme)
	width := a.sidebarWidth
	if width == 0 {
		width = defaultSidebarWidth
	}
	ui.Box(c.Key("app-shell")).Fill().Children(func() {
		ui.Split(c, &width, func() { a.renderSidebar(c) }, func() { a.renderBody(c) }).Fill().Background(c.Theme().Background).OnChange(func() { a.SetSidebarWidth(width) })
		a.renderWindowDragRegions(c, width)
	})
}

func (a *App) renderWindowDragRegions(c *ui.Context, sidebarWidth float32) {
	bar := c.TitleBar()
	// Paint these transparent hit regions after the content. Padding alone
	// does not generate a native titlebar hit, and WebView CSS cannot make
	// the native sidebar draggable. Leave the system window controls clear.
	ui.Box(c.Key("sidebar-titlebar-drag")).Absolute().Top(0).Left(bar.Left).
		Width(max(0, sidebarWidth-bar.Left)).Height(44).DragWindow()
	ui.Box(c.Key("sidebar-brand-drag")).Absolute().Top(44).Left(0).
		Width(sidebarWidth).Height(40).DragWindow()
	if !a.hostedScene() {
		ui.Box(c.Key("content-titlebar-drag")).Absolute().Top(0).Left(sidebarWidth + 1).Right(bar.Right).
			Height(32).DragWindow()
	}
}

// hongguoTheme aligns native chrome with the established frontend palette:
// near-black surfaces, the Hongguo red accent and compact control spacing.
func hongguoTheme(c *ui.Context) ui.Theme {
	t := *c.Theme()
	t.Dark = true
	t.Background = ui.Hex("#0e0f14")
	t.Surface = ui.Hex("#14151c")
	t.SurfaceHover = ui.Hex("#1e202a")
	t.SurfacePressed = ui.Hex("#262938")
	t.Border = ui.RGBA(255, 255, 255, 0.08)
	t.Text = ui.Hex("#ffffff")
	t.TextMuted = ui.RGBA(255, 255, 255, 0.65)
	t.Accent = ui.Hex("#e21a44")
	t.AccentHover = ui.Hex("#fe2c55")
	t.AccentPressed = ui.Hex("#c31639")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#ff4d5f")
	t.Selection = ui.RGBA(226, 26, 68, 0.28)
	t.Focus = ui.RGBA(254, 44, 85, 0.58)
	t.Inverse = ui.Hex("#262938")
	t.InverseText = ui.Hex("#ffffff")
	t.Radius = 8
	t.Spacing = 3
	return t
}
func (a *App) renderSidebar(c *ui.Context) {
	ui.Box(c.Key("sidebar-container")).Fill().Children(func() {
		ui.Scroll(c.Key("sidebar")).Fill().Background(ui.RGBA(18, 20, 28, 0.60)).Padding(0, 10, 16, 10).Gap(2).Children(func() {
			// Draggable traffic light spacer (no brand text)
			ui.Box(c.Key("sidebar-traffic-lights")).Height(44).DragWindow()

			// 发现 Section
			ui.Text(c, "发现").FontSize(11).Bold().TextColor(ui.RGBA(255, 255, 255, 0.48)).Padding(10, 10, 4).LetterSpacing(0.6)
			a.sidebarItem(c, "recommend", "推荐", recommendIcon)
			a.sidebarItem(c, "discover", "找剧", discoverIcon)
			a.sidebarItem(c, "rank", "排行榜", rankIcon)
			a.sidebarItem(c, "new", "新剧", newIcon)

			// 我的 Section
			ui.Text(c, "我的").FontSize(11).Bold().TextColor(ui.RGBA(255, 255, 255, 0.48)).Padding(14, 10, 4).LetterSpacing(0.6)
			a.sidebarItem(c, "history", "历史", historyIcon)
			a.sidebarItem(c, "favorites", "收藏", favoriteIcon)
			a.sidebarItem(c, "reservations", "预约", reservationIcon)
			a.sidebarItem(c, "settings", "设置", settingsIcon)
		})

		if a.ambientActive {
			ui.Box(c.Key("sidebar-ambient-glow")).Absolute().Top(0).Right(0).Bottom(0).Width(120).
				PassThrough().
				LinearGradient(ui.LinearGradient{
					From:  a.ambientLeft.Alpha(0.45),
					To:    ui.Transparent,
					Angle: 270,
				})
		}
	})
}
func (a *App) hostedScene() bool {
	return a.media != nil && (a.mediaOn || a.view == "recommend" || a.view == "settings")
}

// Match the frontend's feedback token: short, interruptible, and paint-only.
var pageTransition = ui.ElementTransition{
	Duration: 160 * time.Millisecond,
	Ease:     ui.CubicBezier(0.23, 1, 0.32, 1),
	Colors:   true,
	Enter:    &ui.Motion{Opacity: 0},
}

func (a *App) renderBody(c *ui.Context) {
	// Keep this parent stable, but give native pages and the media surface distinct
	// identities. A removed WebView must not retain a hole over the next page.
	ui.Column(c.Key("content-host")).Fill().MinWidth(0).MinHeight(0).Background(c.Theme().Background).Children(func() {
		if a.hostedScene() {
			page := ui.Column(c.Key("hosted-page")).Fill().MinWidth(0).MinHeight(0)
			page.Transition(pageTransition).Children(func() {
				ui.WebView(c.Key("media-surface"), a.media.WebView()).Fill().MinWidth(0).MinHeight(0)
			})
			if a.view == "recommend" && !a.mediaOn {
				a.renderFeedChannels(c)
			}
			if a.mediaOn {
				a.renderNativePlayerControls(c)
			}
			return
		}
		page := ui.Column(c.Key("native-page:"+a.view+":"+a.seriesID)).Fill().MinWidth(0).MinHeight(0).Padding(42, 16, 12, 16).Gap(10)
		page.Transition(pageTransition).Children(func() {
			if a.error != "" {
				ui.Text(c, a.error).TextColor(c.Theme().Danger)
			}
			if a.notice != "" {
				ui.Text(c, a.notice).TextColor(c.Theme().Accent)
			}
			switch a.view {
			case "search":
				a.renderSearch(c)
			case "history":
				a.renderLibrary(c, libraryHistory)
			case "favorites":
				a.renderLibrary(c, libraryFavorites)
			case "likes":
				a.renderLibrary(c, libraryLikes)
			case "reservations":
				a.renderLibrary(c, libraryReservations)
			case "detail":
				a.renderDetail(c)
			case "rank":
				a.renderRank(c)
			case "new":
				a.renderReleases(c)
			case "actor":
				a.renderActor(c)
			case "settings":
				a.renderSettings(c)
			default:
				a.renderCatalog(c)
			}
		})
	})
}

func (a *App) sidebarItem(c *ui.Context, id, label string, icon *ui.SVG) {
	selected := a.view == id
	row := ui.Row(c.Key(id)).Height(36).Role(ui.RoleButton).Label(label).Focusable().
		Padding(8, 12).Radius(8).Gap(11).AlignItems(ui.Center).
		Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})

	if selected {
		row.LinearGradient(ui.LinearGradient{
			From:  ui.Hex("#d91b42"),
			To:    ui.Hex("#b81235"),
			Angle: 135,
		}).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.35))
	} else if row.Hovered() {
		row.Background(ui.RGBA(255, 255, 255, 0.08))
	}

	if row.Clicked() || row.Shortcut(0, ui.KeyEnter) || row.Shortcut(0, ui.KeySpace) {
		go a.OpenView(id)
	}

	row.Children(func() {
		ic := ui.Icon(c, icon).Size(17, 17)
		txt := ui.Text(c, label).FontSize(13)
		if selected {
			ic.TextColor(ui.Hex("#ffffff"))
			txt.Bold().TextColor(ui.Hex("#ffffff"))
		} else if row.Hovered() {
			ic.TextColor(ui.Hex("#ffffff"))
			txt.TextColor(ui.Hex("#ffffff"))
		} else {
			ic.TextColor(ui.RGBA(255, 255, 255, 0.72))
			txt.TextColor(ui.RGBA(255, 255, 255, 0.82))
		}
	})
}

func (a *App) pillChip(c *ui.Context, key, label string, selected bool, onClick func()) {
	chip := ui.Row(c.Key(key)).Height(28).Padding(4, 14).Radius(14).
		Role(ui.RoleButton).Label(label).Focusable().AlignItems(ui.Center).Justify(ui.Center).
		Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})
	if selected {
		chip.LinearGradient(ui.LinearGradient{
			From:  ui.Hex("#d91b42"),
			To:    ui.Hex("#b81235"),
			Angle: 135,
		}).Border(1, ui.RGBA(255, 255, 255, 0.22)).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.35))
	} else {
		chip.Material(glass.Glass{Interactive: true}).Border(1, ui.RGBA(255, 255, 255, 0.08))
		if chip.Hovered() {
			chip.Border(1, ui.RGBA(255, 255, 255, 0.18))
		}
	}
	if chip.Clicked() || chip.Shortcut(0, ui.KeyEnter) || chip.Shortcut(0, ui.KeySpace) {
		if onClick != nil {
			onClick()
		}
	}
	chip.Children(func() {
		txt := ui.Text(c, label).FontSize(12)
		if selected {
			txt.Bold().TextColor(ui.Hex("#ffffff"))
		} else if chip.Hovered() {
			txt.TextColor(ui.Hex("#ffffff"))
		} else {
			txt.TextColor(ui.RGBA(255, 255, 255, 0.75))
		}
	})
}

type slidingTabMatch struct{ group string }

type slidingTabItem struct {
	Key   string
	Label string
}

func (a *App) renderSlidingTabs(c *ui.Context, key, group string, tabs []slidingTabItem, selectedKey string, onSelect func(string)) {
	track := ui.Row(c.Key(key)).AlignSelf(ui.Start).Padding(3).Gap(2).Radius(10).
		Material(glass.Glass{}).
		Border(1, ui.RGBA(255, 255, 255, 0.08)).
		AlignItems(ui.Center)
	track.Children(func() {
		for _, tab := range tabs {
			tKey, tLabel := tab.Key, tab.Label
			selected := selectedKey == tKey
			chip := ui.Row(c.Key(key+":"+tKey)).Height(28).Padding(0, 16).Radius(7).
				Role(ui.RoleButton).Label(tLabel).AlignItems(ui.Center).Justify(ui.Center)
			if !selected && chip.Hovered() {
				chip.Background(ui.RGBA(255, 255, 255, 0.08))
			}
			if chip.Clicked() {
				if onSelect != nil {
					onSelect(tKey)
				}
			}
			chip.Children(func() {
				// 1. In-flow sizer: guarantees chip maintains its natural width without collapsing
				ui.Row(c.Key("sizer:" + group + ":" + tKey)).Opacity(0).Children(func() {
					ui.Text(c, tLabel).FontSize(12).Bold().SingleLine()
				})
				// 2. Pure background sliding glider: slides across tabs without moving the text
				if selected {
					ui.Box(c.Key("sliding-pill:"+group)).Absolute().Fill().Radius(7).
						LinearGradient(ui.LinearGradient{
							From:  ui.Hex("#d91b42"),
							To:    ui.Hex("#b81235"),
							Angle: 135,
						}).Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.35)).
						Transition(ui.ElementTransition{
							Duration: 240 * time.Millisecond,
							Ease:     ui.CubicBezier(0.25, 1, 0.5, 1),
							Match:    slidingTabMatch{group: group},
						})
				}
				// 3. Stationary text label: stays in place over the glider, only cross-fading color
				ui.Row(c.Key("tab-label:" + group + ":" + tKey)).Absolute().Fill().
					AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					txt := ui.Text(c, tLabel).FontSize(12).SingleLine().
						Transition(ui.ElementTransition{Duration: 180 * time.Millisecond, Colors: true})
					if selected {
						txt.Bold().TextColor(ui.Hex("#ffffff"))
					} else {
						txt.TextColor(ui.RGBA(255, 255, 255, 0.70))
					}
				})
			})
		}
	})
}

func (a *App) renderCatalog(c *ui.Context) {
	catalog := a.catalog
	a.maybeLoadMore(a.catalogScroll.Y, a.catalogScroll.MaxY, catalog.HasMore, catalog.Loading || catalog.LoadingMore, a.LoadMoreCatalog)

	scroll := ui.Scroll(c.Key("catalog-page-scroll")).Grow(1).Gap(14).TrackScroll(&a.catalogScroll)
	scroll.Children(func() {
		a.renderSearchBar(c)

		ui.Column(c.Key("catalog-filter-card")).Padding(14, 18).Gap(10).Radius(14).
			Material(glass.Glass{Clarity: 0.15}).
			Border(1, ui.RGBA(255, 255, 255, 0.08)).Children(func() {
			ui.Row(c.Key("genre-row")).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Text(c, "体裁").FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.65)).Width(40).Padding(5, 0, 0, 0)
				ui.Row(c).Wrap().Gap(8).Grow(1).Children(func() {
					for index, genre := range catalog.Genre {
						gIndex := index
						selected := gIndex == catalog.GenreIndex
						a.pillChip(c, "genre:"+genre, genre, selected, func() {
							go a.SelectGenre(gIndex)
						})
					}
				})
			})
			if len(catalog.Filters) > 0 {
				for rowIndex, row := range catalog.Filters {
					rIdx := rowIndex
					rTitle := row.Title
					rOptions := row.Options
					selected := 0
					if rIdx < len(catalog.Selection) {
						selected = catalog.Selection[rIdx]
					}
					ui.Row(c.Key(fmt.Sprintf("filter-row-%d", rIdx))).Gap(12).AlignItems(ui.Start).Children(func() {
						ui.Text(c, rTitle).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.65)).Width(40).Padding(5, 0, 0, 0)
						ui.Row(c).Wrap().Gap(8).Grow(1).Children(func() {
							for optIndex, opt := range rOptions {
								oIdx := optIndex
								oLabel := opt.Label
								a.pillChip(c, fmt.Sprintf("opt-%d-%d", rIdx, oIdx), oLabel, oIdx == selected, func() {
									go a.SelectFilter(rIdx, oIdx)
								})
							}
						})
					})
				}
			}
		})

		if catalog.Loading && len(catalog.Items) == 0 {
			ui.Text(c, "正在加载目录...").TextColor(c.Theme().TextMuted)
			a.filterSkeleton(c)
			a.gridSkeleton(c, 12)
			return
		}
		if catalog.Error != "" {
			a.errorView(c, catalog.Error, func() { go a.loadCatalog(len(catalog.Items) > 0) })
			if len(catalog.Items) == 0 {
				return
			}
		}
		if len(catalog.Items) == 0 {
			a.emptyView(c, "暂无短剧", "换个筛选条件试试")
			return
		}

		availWidth := scroll.LayoutBounds().W
		if availWidth <= 0 {
			availWidth = 980
		}
		gap := float32(14)
		minCardWidth := float32(150)
		cols := max(2, int((availWidth+gap)/(minCardWidth+gap)))
		cardWidth := (availWidth - float32(cols-1)*gap) / float32(cols)

		ui.Grid(c.Key("catalog-grid-flow")).Columns(cols).Gap(gap).Children(func() {
			for _, item := range catalog.Items {
				a.catalogCardSized(c, item, cardWidth)
			}
		})
		a.scrollLoading(c, catalog.LoadingMore)
	})
}

func (a *App) maybeLoadMore(y, maxY float32, hasMore, loading bool, load func()) {
	if maxY <= 0 || y < maxY*0.85 || !hasMore || loading {
		return
	}
	go load()
}

func (a *App) scrollLoading(c *ui.Context, loading bool) {
	if !loading {
		return
	}
	ui.Row(c).Padding(8).Justify(ui.Center).Children(func() {
		ui.Spinner(c)
		ui.Text(c, "正在加载...").TextColor(c.Theme().TextMuted)
	})
}

func formatRelativeTime(ts int64) string {
	if ts <= 0 {
		return "刚刚"
	}
	now := time.Now().Unix()
	diff := now - ts
	if diff < 60 {
		return "刚刚"
	}
	if diff < 3600 {
		return fmt.Sprintf("%d分钟前", diff/60)
	}
	if diff < 86400 {
		return fmt.Sprintf("%d小时前", diff/3600)
	}
	if diff < 86400*30 {
		return fmt.Sprintf("%d天前", diff/86400)
	}
	return time.Unix(ts, 0).Format("01月02日")
}

func (a *App) catalogCard(c *ui.Context, item CatalogItem) {
	a.catalogCardSized(c, item, 150)
}

func (a *App) catalogCardSized(c *ui.Context, item CatalogItem, width float32) {
	t := c.Theme()
	card := ui.Column(c.Key(item.ID + "|" + item.EpisodeID)).Role(ui.RoleButton).Label(item.Title).Focusable().
		Width(width).MinWidth(width).MaxWidth(width).Gap(6)
	shouldOpen := item.Kind != "source_novel" && (a.view != "reservations" || !item.Upcoming)
	if shouldOpen && card.Hovered() {
		card.Background(ui.RGBA(255, 255, 255, 0.04))
	}
	if shouldOpen && (card.Clicked() || card.Shortcut(0, ui.KeyEnter) || card.Shortcut(0, ui.KeySpace)) {
		go a.OpenItem(item)
	}

	topLeftBadge := ""
	bottomRightBadge := ""

	if a.view == "new" {
		if !item.Upcoming {
			topLeftBadge = "已上线"
		}
		if item.Completed {
			bottomRightBadge = "完结"
		}
	} else if a.view == "history" {
		if item.ResumeEpisode > 0 {
			topLeftBadge = fmt.Sprintf("看到第 %d 集", item.ResumeEpisode)
		} else {
			topLeftBadge = "看到第 1 集"
		}
		if item.EpisodeCount > 0 {
			bottomRightBadge = fmt.Sprintf("共%d集", item.EpisodeCount)
		}
	} else if a.view == "favorites" {
		topLeftBadge = "收藏"
		if item.EpisodeCount > 0 {
			bottomRightBadge = fmt.Sprintf("%d集", item.EpisodeCount)
		}
	} else if a.view == "likes" {
		topLeftBadge = "点赞"
		if item.EpisodeCount > 0 {
			bottomRightBadge = fmt.Sprintf("%d集", item.EpisodeCount)
		}
	}

	card.Children(func() {
		posterBox := ui.Box(c.Key("poster-box")).Size(width, width*4/3).Radius(8).Clip()
		posterBox.Children(func() {
			a.image(c, item.CoverURL, width, width*4/3)
			if topLeftBadge != "" {
				ui.Row(c.Key("badge-tl")).Absolute().Top(6).Left(6).
					Padding(2, 6).Radius(4).Background(ui.RGBA(0, 0, 0, 0.65)).
					Children(func() {
						ui.Text(c, topLeftBadge).FontSize(11).Bold().TextColor(ui.Hex("#ffffff"))
					})
			}
			if bottomRightBadge != "" {
				ui.Row(c.Key("badge-br")).Absolute().Bottom(6).Right(6).
					Padding(2, 6).Radius(4).Background(ui.RGBA(0, 0, 0, 0.65)).
					Children(func() {
						ui.Text(c, bottomRightBadge).FontSize(11).Bold().TextColor(ui.Hex("#ffffff"))
					})
			}
		})

		ui.Text(c, item.Title).Bold().FontSize(13).TextColor(ui.Hex("#ffffff")).SingleLine().Width(width)

		if a.view == "history" {
			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Width(width).Children(func() {
				badge := fmt.Sprintf("看到第%d集", max(1, item.ResumeEpisode))
				if item.Completed {
					badge = "已看完"
				}
				ui.Text(c, badge).FontSize(12).Bold().TextColor(ui.Hex("#ff3358"))
				relTime := formatRelativeTime(item.UpdatedAt)
				ui.Text(c, relTime).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.50))
			})
		} else if a.view == "reservations" {
			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Width(width).Children(func() {
				ui.Text(c, "上线日期").FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.60))
				ui.Text(c, formatPublishDate(item.PublishAt)).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.85))
			})
		} else if a.view == "new" {
			if item.Upcoming {
				if item.PublishAt != "" {
					ui.Text(c, formatPublishDate(item.PublishAt)).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.60)).SingleLine().Width(width)
				}
				label := "预约"
				if item.Reserved {
					label = "取消预约"
				}
				if ui.Button(c, label).Clicked() {
					go a.ToggleItemReservation(item)
				}
			} else {
				if item.Heat != "" {
					ui.Text(c, item.Heat).FontSize(12).Bold().TextColor(ui.Hex("#ff3358")).SingleLine().Width(width)
				}
				ui.Text(c, "新剧").FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.50)).SingleLine().Width(width)
			}
		} else if a.view != "favorites" && a.view != "likes" {
			if item.Upcoming && item.PublishAt != "" {
				ui.Text(c, formatPublishDate(item.PublishAt)).TextColor(t.TextMuted).SingleLine().Width(width)
			}
			if !item.Upcoming && item.BadgeText != "" {
				ui.Text(c, item.BadgeText).TextColor(t.TextMuted).Width(width)
			}
			if !item.Upcoming && item.Category != "" {
				ui.Text(c, item.Category).TextColor(t.TextMuted).SingleLine().Width(width)
			}
			if !item.Upcoming && (item.Heat != "" || item.EpisodeCount > 0) {
				meta := item.Heat
				if item.EpisodeCount > 0 {
					meta = fmt.Sprintf("%d集", item.EpisodeCount)
					if item.Heat != "" {
						meta += " · " + item.Heat
					}
				}
				ui.Text(c, meta).TextColor(t.TextMuted).SingleLine().Width(width)
			}
			if item.Upcoming {
				label := "预约"
				if item.Reserved {
					label = "取消预约"
				}
				if ui.Button(c, label).Clicked() {
					go a.ToggleItemReservation(item)
				}
			}
		}
	})
}

func (a *App) renderSearchBar(c *ui.Context) {
	query := a.query
	bar := ui.Row(c.Key("search-bar-wrap")).Height(38).Radius(19).
		Background(ui.RGBA(29, 31, 43, 255)).
		Border(1, ui.RGBA(255, 255, 255, 0.08)).
		Padding(0, 14).AlignItems(ui.Center).Gap(10)
	bar.Children(func() {
		ui.Icon(c, searchIcon).Size(16, 16).TextColor(ui.RGBA(255, 255, 255, 0.50))
		ui.TextInputBase(c.Key("search-query"), &query).Grow(1).
			Placeholder("搜索短剧、角色、演员...").
			Label("搜索短剧").
			OnTextKey(a.searchTextKey).
			OnChange(func() { a.SetQuery(query) }).
			OnSubmit(func() { a.SetQuery(query); go a.SearchSubmitted() })
		if query != "" {
			clearBtn := ui.Row(c.Key("search-clear-btn")).Size(20, 20).Radius(10).
				Background(ui.RGBA(255, 255, 255, 0.12)).
				AlignItems(ui.Center).Justify(ui.Center).
				Role(ui.RoleButton).Label("清空搜索")
			if clearBtn.Hovered() {
				clearBtn.Background(ui.RGBA(255, 255, 255, 0.22))
			}
			if clearBtn.Clicked() {
				a.query = ""
				a.generation++ // Discard an in-flight search response after clearing.
				a.suggestions = nil
				a.suggestionIndex = -1
				a.suggestionVersion++
				a.search = CatalogPage{}
			}
			clearBtn.Children(func() {
				ui.Icon(c, closeIcon).Size(12, 12).TextColor(ui.RGBA(255, 255, 255, 0.85))
			})
		}
	})
	if len(a.suggestions) > 0 {
		ui.Row(c.Key("search-suggestions")).Wrap().Gap(6).Padding(4, 0).Children(func() {
			for _, suggestion := range a.suggestions {
				s := suggestion
				chip := ui.Row(c.Key("sug:"+s)).Height(26).Padding(3, 12).Radius(13).
					Background(ui.RGBA(255, 255, 255, 0.06)).Border(1, ui.RGBA(255, 255, 255, 0.09)).
					Role(ui.RoleButton).Label(s)
				if chip.Hovered() {
					chip.Background(ui.RGBA(255, 255, 255, 0.12))
				}
				if chip.Clicked() {
					a.query = s
					go a.SearchSubmitted()
				}
				chip.Children(func() {
					ui.Text(c, s).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.85))
				})
			}
		})
	}
}

func (a *App) renderSearch(c *ui.Context) {
	a.renderSearchBar(c)
	search := a.search
	if a.query == "" && len(search.Items) == 0 && !search.Loading && search.Error == "" {
		a.emptyView(c, "搜索短剧", "输入剧名、角色或演员，发现想看的短剧")
		return
	}
	if search.Loading {
		ui.Text(c, "正在搜索...").TextColor(c.Theme().TextMuted)
		a.gridSkeleton(c, 12)
		return
	}
	if search.Error != "" {
		a.errorView(c, search.Error, func() { go a.SearchSubmitted() })
		return
	}
	if len(search.Items) == 0 {
		a.emptyView(c, "暂无搜索结果", "请尝试其他关键词")
		return
	}
	a.cardGrid(c, "search-grid", nil, nil, search.Items, 0, nil).Grow(1)
}

func (a *App) renderLibrary(c *ui.Context, tab LibraryTab) {
	state := a.library
	library := state.History
	grid := 0
	switch tab {
	case libraryFavorites:
		library, grid = state.Favorites, 1
	case libraryLikes:
		library, grid = state.Likes, 2
	case libraryReservations:
		library, grid = state.Reserved, 3
	}

	if tab == libraryHistory {
		ui.Row(c.Key("history-header")).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "观看历史").FontSize(20).Bold().TextColor(ui.Hex("#ffffff"))
				a.renderSlidingTabs(c, "hist-tabs", "history-filter", []slidingTabItem{
					{"", "全部"},
					{"unwatched", "未看完"},
					{"completed", "已看完"},
				}, a.historyFilter, func(key string) {
					a.historyFilter = key
				})
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				query := a.historyQuery
				sWrap := ui.Row(c.Key("hist-search")).Height(32).Width(200).Radius(16).
					Background(ui.RGBA(24, 25, 36, 255)).
					Border(1, ui.RGBA(255, 255, 255, 0.08)).
					Padding(0, 10).AlignItems(ui.Center).Gap(6)
				sWrap.Children(func() {
					ui.Icon(c, searchIcon).Size(14, 14).TextColor(ui.RGBA(255, 255, 255, 0.50))
					ui.TextInputBase(c.Key("hist-search-input"), &query).Grow(1).
						Placeholder("搜索历史剧名...").Label("搜索历史").
						OnChange(func() { a.SetHistoryQuery(query) })
					if query != "" {
						histClear := ui.Row(c.Key("hist-search-clear")).Size(16, 16).Radius(8).
							Background(ui.RGBA(255, 255, 255, 0.12)).
							AlignItems(ui.Center).Justify(ui.Center).
							Role(ui.RoleButton).Label("清空搜索")
						if histClear.Clicked() {
							a.SetHistoryQuery("")
						}
						histClear.Children(func() {
							ui.Icon(c, closeIcon).Size(10, 10).TextColor(ui.RGBA(255, 255, 255, 0.85))
						})
					}
				})
			})
		})
	}
	if tab == libraryFavorites || tab == libraryLikes {
		favCount := libraryCountLabel(state.Favorites)
		likeCount := libraryCountLabel(state.Likes)
		title := "我的收藏"
		if tab == libraryLikes {
			title = "我的点赞"
		}
		ui.Row(c.Key("fav-header")).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(20).Bold().TextColor(ui.Hex("#ffffff"))
				ui.Row(c).Padding(3, 10).Radius(12).Background(ui.RGBA(255, 255, 255, 0.06)).Children(func() {
					ui.Textf(c, "共 %s 部收藏 · %s 个点赞", favCount, likeCount).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.65))
				})
			})
			a.renderSlidingTabs(c, "fav-tabs", "fav-views", []slidingTabItem{
				{"favorites", fmt.Sprintf("收藏 %s", favCount)},
				{"likes", fmt.Sprintf("点赞 %s", likeCount)},
			}, a.view, func(vId string) {
				go a.OpenView(vId)
			})
		})
	}
	if tab == libraryReservations {
		online, pending := 0, 0
		for _, item := range library.Items {
			if item.Upcoming {
				pending++
			} else {
				online++
			}
		}
		total := len(library.Items)
		ui.Row(c.Key("res-header")).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "我的预约").FontSize(20).Bold().TextColor(ui.Hex("#ffffff"))
				ui.Row(c).Padding(3, 10).Radius(12).Background(ui.RGBA(255, 255, 255, 0.06)).Children(func() {
					ui.Textf(c, "新剧预约 · 共 %d 部 · 已上线 %d · 待上线 %d", total, online, pending).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.65))
				})
			})
			resSelected := "online"
			if a.reservationsPending {
				resSelected = "pending"
			}
			a.renderSlidingTabs(c, "res-tabs", "res-pending", []slidingTabItem{
				{"online", fmt.Sprintf("已上线 %d", online)},
				{"pending", fmt.Sprintf("待上线 %d", pending)},
			}, resSelected, func(pKey string) {
				a.reservationsPending = (pKey == "pending")
			})
		})
	}
	if library.Loading && len(library.Items) == 0 {
		ui.Text(c, "正在加载本地数据...").TextColor(c.Theme().TextMuted)
		a.gridSkeleton(c, 12)
		return
	}

	if library.Error != "" {
		a.errorView(c, library.Error, func() { go a.RefreshLibrary(tab) })
		if len(library.Items) == 0 {
			return
		}
	}
	if library.Empty {
		a.emptyView(c, "暂无记录", "浏览和播放后才会形成真实记录")
		return
	}
	library.Items = a.visibleLibrary(tab)
	if len(library.Items) == 0 {
		a.emptyView(c, "暂无匹配记录", "可以调整筛选条件")
		return
	}
	if a.library.Grids[grid] == nil {
		a.library.Grids[grid] = &ui.GridState{}
	}
	a.library.Grids[grid].Key = func(item int) any { return library.Items[item].ID + "|" + library.Items[item].EpisodeID }
	a.library.Grids[grid].Label = func(item int) string { return library.Items[item].Title }
	a.cardGrid(c, "library-grid", a.library.Grids[grid], nil, library.Items, 40, func(item CatalogItem) {
		if tab == libraryHistory {
			return
		}
		label := map[LibraryTab]string{libraryFavorites: "取消收藏", libraryLikes: "取消点赞", libraryReservations: "取消预约"}[tab]
		actBtn := ui.Row(c.Key("act:"+item.ID+"|"+label)).Height(28).Padding(0, 10).Radius(6).
			Background(ui.RGBA(255, 255, 255, 0.05)).
			Border(1, ui.RGBA(255, 255, 255, 0.08)).
			AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label(label)
		if actBtn.Hovered() {
			actBtn.Background(ui.RGBA(255, 255, 255, 0.10)).Border(1, ui.RGBA(255, 255, 255, 0.14))
		}
		if actBtn.Clicked() {
			go a.RemoveLibraryItem(tab, item)
		}
		actBtn.Children(func() {
			txt := ui.Text(c, label).FontSize(12)
			if actBtn.Hovered() {
				txt.TextColor(ui.Hex("#ffffff"))
			} else {
				txt.TextColor(ui.RGBA(255, 255, 255, 0.72))
			}
		})
	}).Grow(1)
}

func (a *App) renderDetail(c *ui.Context) {
	detail := a.detail
	if detail.Loading && detail.Title == "" {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 0, 10, 0).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在加载详情...").TextColor(c.Theme().TextMuted)
		})
		a.detailSkeleton(c)
		return
	}
	if detail.Error != "" && detail.Title == "" {
		a.errorView(c, detail.Error, func() { go a.OpenDetail(detail.ID) })
		return
	}
	scroll := ui.Scroll(c.Key("detail-page-scroll")).Grow(1).Gap(c.Theme().Space(3)).TrackScroll(&a.detailScroll)
	scroll.Children(func() {
		// Top Bar with a single back navigation entry.
		ui.Row(c.Key("detail-top-bar")).AlignItems(ui.Center).Gap(12).Padding(0, 0, 4, 0).Children(func() {
			ui.Row(c.Key("detail-top-bar-left")).AlignItems(ui.Center).Gap(12).Children(func() {
				backBtn := ui.Row(c.Key("detail-nav-back")).Height(30).Padding(0, 14).Radius(7).
					Material(glass.Glass{Interactive: true}).
					Border(1, ui.RGBA(255, 255, 255, 0.12)).
					AlignItems(ui.Center).Role(ui.RoleButton).Label("返回短剧列表").Focusable()
				if backBtn.Hovered() {
					backBtn.Background(ui.RGBA(255, 255, 255, 0.10))
				}
				if backBtn.Clicked() || backBtn.Shortcut(0, ui.KeyEnter) || backBtn.Shortcut(0, ui.KeySpace) {
					go a.NavigateHistory(-1)
				}
				backBtn.Children(func() {
					ui.Text(c, "← 返回").FontSize(13).TextColor(ui.Hex("#ffffff")).Bold()
				})
				ui.Text(c, "短剧详情").FontSize(13).TextColor(ui.RGBA(255, 255, 255, 0.50))
			})
		})

		// Hero Info Row
		ui.Row(c.Key("detail-hero-row")).Gap(c.Theme().Space(4)).AlignItems(ui.Start).Children(func() {
			a.image(c, detail.CoverURL, 160, 220)
			header := ui.Column(c.Key("detail-hero-info")).Grow(1).Gap(8)
			header.Children(func() {
				ui.Text(c, detail.Title).FontSize(24).Bold().TextColor(ui.Hex("#ffffff"))
				if len(detail.Tags) > 0 {
					ui.Text(c, strings.Join(detail.Tags, " · ")).FontSize(13).TextColor(c.Theme().TextMuted)
				}
				ui.Textf(c, "%s · %d 人在追 · %d 赞", detail.Heat, detail.FollowCount, detail.LikeCount).FontSize(12).TextColor(c.Theme().TextMuted)
				if detail.PublishText != "" {
					ui.Text(c, detail.PublishText).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				if detail.Description != "" {
					ui.Text(c, detail.Description).FontSize(13).TextColor(ui.RGBA(255, 255, 255, 0.82))
				}
				if detail.StatusText != "" || detail.PlayCount > 0 {
					ui.Textf(c, "%s · %d次播放", detail.StatusText, detail.PlayCount).FontSize(12).TextColor(c.Theme().TextMuted)
				}

				// Action buttons aligned right under the metadata
				ui.Row(c.Key("detail-actions")).Gap(10).AlignItems(ui.Center).Padding(6, 0, 0, 0).Children(func() {
					if detail.Upcoming {
						ui.Row(c).Padding(8, 14).Radius(8).Background(ui.RGBA(255, 255, 255, 0.06)).
							Border(1, ui.RGBA(255, 255, 255, 0.10)).Children(func() {
							ui.Text(c, "该剧尚未上线，上线后可继续观看").FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.65))
						})
					} else {
						playLabel := "播放 / 继续观看"
						if a.resume.ResumeEpisode > 1 {
							playLabel = fmt.Sprintf("继续观看第 %d 集", a.resume.ResumeEpisode)
						}
						playBtn := ui.Row(c.Key("detail-hero-play")).Height(36).Padding(0, 20).Radius(8).
							LinearGradient(ui.LinearGradient{
								From:  ui.Hex("#d91b42"),
								To:    ui.Hex("#b81235"),
								Angle: 180,
							}).Border(1, ui.RGBA(255, 255, 255, 0.22)).
							Shadow(0, 3, 10, 0, ui.RGBA(0, 0, 0, 0.35)).
							AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label(playLabel)
						if playBtn.Hovered() {
							playBtn.LinearGradient(ui.LinearGradient{
								From:  ui.Hex("#e5244b"),
								To:    ui.Hex("#c4173b"),
								Angle: 180,
							})
						}
						if playBtn.Clicked() {
							go a.PlayDetail()
						}
						playBtn.Children(func() {
							ui.Text(c, playLabel).FontSize(13).Bold().TextColor(ui.Hex("#ffffff"))
						})
					}

					// Favorite button
					favLabel := "收藏"
					if detail.Favorite {
						favLabel = "已收藏"
					}
					favBtn := ui.Row(c.Key("detail-hero-fav")).Height(36).Padding(0, 16).Radius(8).
						Material(glass.Glass{Interactive: true}).
						Border(1, ui.RGBA(255, 255, 255, 0.12)).
						AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label(favLabel)
					if detail.Favorite {
						favBtn.Background(ui.RGBA(226, 26, 68, 0.16)).Border(1, ui.Hex("#e21a44"))
					} else if favBtn.Hovered() {
						favBtn.Background(ui.RGBA(255, 255, 255, 0.12))
					}
					if favBtn.Clicked() {
						go a.ToggleDetailFavorite()
					}
					favBtn.Children(func() {
						txt := ui.Text(c, favLabel).FontSize(13)
						if detail.Favorite {
							txt.Bold().TextColor(ui.Hex("#ff3358"))
						} else {
							txt.TextColor(ui.Hex("#ffffff"))
						}
					})

					// Reservation button if upcoming or already reserved
					if detail.Upcoming || detail.Reserved {
						resLabel := "预约"
						if detail.Reserved {
							resLabel = "已预约"
						}
						resBtn := ui.Row(c.Key("detail-hero-res")).Height(36).Padding(0, 16).Radius(8).
							Material(glass.Glass{Interactive: true}).
							Border(1, ui.RGBA(255, 255, 255, 0.12)).
							AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label(resLabel)
						if detail.Reserved {
							resBtn.Background(ui.RGBA(226, 26, 68, 0.16)).Border(1, ui.Hex("#e21a44"))
						} else if resBtn.Hovered() {
							resBtn.Background(ui.RGBA(255, 255, 255, 0.12))
						}
						if resBtn.Clicked() {
							go a.ToggleDetailReservation()
						}
						resBtn.Children(func() {
							txt := ui.Text(c, resLabel).FontSize(13)
							if detail.Reserved {
								txt.Bold().TextColor(ui.Hex("#ff3358"))
							} else {
								txt.TextColor(ui.Hex("#ffffff"))
							}
						})
					}
				})
			})
		})

		if detail.Source != nil {
			ui.Row(c.Key("detail-source-card")).Padding(c.Theme().Space(2), c.Theme().Space(4)).Radius(10).
				Background(ui.RGBA(255, 255, 255, 0.04)).
				Border(1, ui.RGBA(255, 255, 255, 0.08)).
				Gap(c.Theme().Space(3)).AlignItems(ui.Center).Children(func() {
				a.image(c, detail.Source.CoverURL, 54, 72)
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Text(c, "原著小说").FontSize(11).TextColor(ui.Hex("#ff3358"))
					ui.Text(c, detail.Source.Title).FontSize(14).Bold().TextColor(ui.Hex("#ffffff"))
					if len(detail.Source.Tags) > 0 {
						ui.Text(c, strings.Join(detail.Source.Tags, " · ")).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.60))
					}
				})
			})
		}
		if len(detail.Actors) > 0 {
			ui.Column(c.Key("detail-actors-card")).Gap(c.Theme().Space(2)).Children(func() {
				ui.Text(c, "演员阵容").FontSize(15).Bold().TextColor(ui.Hex("#ffffff"))
				ui.Row(c.Key("detail-actors-flow")).Wrap().Gap(8).Children(func() {
					for _, actor := range detail.Actors {
						act := actor
						ui.Row(c.Key("actor:"+act.Name)).Padding(4, 10).Radius(6).
							Background(ui.RGBA(255, 255, 255, 0.06)).
							Border(1, ui.RGBA(255, 255, 255, 0.08)).
							AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Text(c, act.Name).FontSize(12).Bold().TextColor(ui.Hex("#ffffff"))
							if act.Role != "" {
								ui.Text(c, act.Role).FontSize(11).TextColor(ui.RGBA(255, 255, 255, 0.60))
							}
						})
					}
				})
			})
		}
		a.renderCollection(c, "同系列短剧", detail.Seasons, func() { go a.RetrySupplement(false) })
		a.renderCollection(c, "同一剧本改编", detail.Adaptations, func() { go a.RetrySupplement(true) })

		// Episode selection with aligned keycaps
		ui.Row(c.Key("detail-episodes-header")).AlignItems(ui.Baseline).Gap(10).Children(func() {
			ui.Text(c, "选集").FontSize(17).Bold().TextColor(ui.Hex("#ffffff"))
			if len(detail.Episodes) > 0 {
				ui.Textf(c, "共 %d 集", len(detail.Episodes)).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.50))
			}
		})
		if len(detail.Episodes) == 0 {
			ui.Text(c, "暂无可播放选集").TextColor(c.Theme().TextMuted)
		} else {
			episodes := detail.Episodes
			visibleIndexes := a.detailEpisodeCollapsedIndexes(episodes)
			ui.Row(c.Key("detail-episodes-wrap")).Wrap().Gap(8).Children(func() {
				for index, episode := range episodes {
					if !a.detailEpisodesExpanded {
						shown := false
						for _, visibleIndex := range visibleIndexes {
							if index == visibleIndex {
								shown = true
								break
							}
						}
						if !shown {
							continue
						}
					}
					ep := episode
					isActive := a.resume.ResumeEpisode == ep.EpisodeCount
					numStr := fmt.Sprintf("%d", ep.EpisodeCount)
					epBtn := ui.Row(c.Key(fmt.Sprintf("ep-%d", ep.EpisodeCount))).
						Width(52).Height(34).Radius(8).
						AlignItems(ui.Center).Justify(ui.Center).
						Role(ui.RoleButton).Label(fmt.Sprintf("第%d集", ep.EpisodeCount))
					if isActive {
						epBtn.LinearGradient(ui.LinearGradient{
							From:  ui.Hex("#d91b42"),
							To:    ui.Hex("#b81235"),
							Angle: 135,
						}).Border(1, ui.RGBA(255, 255, 255, 0.22)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.35))
					} else {
						epBtn.Background(ui.RGBA(255, 255, 255, 0.05)).
							Border(1, ui.RGBA(255, 255, 255, 0.08))
						if epBtn.Hovered() {
							epBtn.Background(ui.RGBA(255, 255, 255, 0.12)).Border(1, ui.RGBA(255, 255, 255, 0.16))
						}
					}
					if epBtn.Clicked() {
						go a.ChooseEpisode(ep.EpisodeCount)
					}
					epBtn.Children(func() {
						txt := ui.Text(c, numStr).FontSize(13)
						if isActive {
							txt.Bold().TextColor(ui.Hex("#ffffff"))
							epBtn.ScrollIntoView()
						} else {
							txt.TextColor(ui.RGBA(255, 255, 255, 0.85))
						}
					})
				}
			})
			if len(episodes) > detailEpisodeCollapsedCount(len(episodes)) {
				toggleLabel := "展开全部"
				if a.detailEpisodesExpanded {
					toggleLabel = "收起"
				}
				toggle := ui.Row(c.Key("detail-episode-toggle")).Height(32).Padding(0, 14).Radius(8).
					Background(ui.RGBA(255, 255, 255, 0.05)).
					Border(1, ui.RGBA(255, 255, 255, 0.08)).
					AlignItems(ui.Center).Justify(ui.Center).Gap(6).
					Role(ui.RoleButton).Label(toggleLabel)
				if toggle.Hovered() {
					toggle.Background(ui.RGBA(255, 255, 255, 0.12)).Border(1, ui.RGBA(255, 255, 255, 0.16))
				}
				if toggle.Clicked() {
					a.detailEpisodesExpanded = !a.detailEpisodesExpanded
				}
				toggle.Children(func() {
					icon := ui.Icon(c, chevronIcon).Size(16, 16).TextColor(ui.RGBA(255, 255, 255, 0.72))
					if a.detailEpisodesExpanded {
						icon.Rotate(180)
					}
					ui.Text(c, toggleLabel).FontSize(13).TextColor(ui.RGBA(255, 255, 255, 0.85))
				})
			}
		}

		ui.Text(c, detail.RelatedTitle).Bold()
		if detail.RelatedLoading {
			a.loadingView(c, "正在加载相关推荐...")
		} else if detail.RelatedError != "" {
			a.errorView(c, detail.RelatedError, func() { go a.RetryRelated() })
		} else if len(detail.Related) == 0 {
			ui.Text(c, "暂无相关推荐").TextColor(c.Theme().TextMuted)
		} else {
			ui.Row(c.Key("detail-related-wrap")).Wrap().Gap(14).Children(func() {
				for _, item := range detail.Related {
					a.catalogCard(c, item)
				}
			})
		}
	})
}

func (a *App) loadingView(c *ui.Context, message string) {
	ui.Row(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Spinner(c)
		ui.Text(c, message)
	})
}

func (a *App) detailSkeleton(c *ui.Context) {
	scroll := ui.Scroll(c).Grow(1).Gap(c.Theme().Space(4))
	scroll.Children(func() {
		ui.Row(c).Gap(c.Theme().Space(4)).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(160, 220).Radius(12).Background(c.Theme().SurfaceHover)
			ui.Column(c).Grow(1).Gap(12).Children(func() {
				ui.Box(c).Height(26).Radius(8).Background(c.Theme().SurfaceHover)
				ui.Row(c).Gap(10).Children(func() {
					for range 4 {
						ui.Box(c).Size(66, 20).Radius(10).Background(c.Theme().SurfaceHover)
					}
				})
				ui.Row(c).Gap(8).Children(func() {
					for range 4 {
						ui.Box(c).Size(58, 24).Radius(12).Background(c.Theme().SurfaceHover)
					}
				})
				ui.Column(c).Gap(8).Children(func() {
					ui.Box(c).Height(13).Radius(6).Background(c.Theme().SurfaceHover).FillWidth()
					ui.Box(c).Height(13).Radius(6).Background(c.Theme().SurfaceHover).FillWidth()
					ui.Box(c).Width(190).Height(13).Radius(6).Background(c.Theme().SurfaceHover)
				})
				ui.Row(c).Gap(10).Children(func() {
					ui.Box(c).Size(140, 38).Radius(19).Background(c.Theme().SurfaceHover)
					ui.Box(c).Size(88, 38).Radius(19).Background(c.Theme().SurfaceHover)
				})
			})
		})
		ui.Column(c).Gap(12).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(52).Height(18).Radius(6).Background(c.Theme().SurfaceHover)
				ui.Box(c).Width(44).Height(14).Radius(6).Background(c.Theme().SurfaceHover)
			})
			ui.Row(c).Wrap().Gap(8).Children(func() {
				for range 12 {
					ui.Box(c).Size(68, 32).Radius(8).Background(c.Theme().SurfaceHover)
				}
			})
		})
		ui.Column(c).Gap(12).Children(func() {
			ui.Box(c).Width(90).Height(18).Radius(6).Background(c.Theme().SurfaceHover)
			a.gridSkeleton(c, 4)
		})
	})
}

func (a *App) gridSkeleton(c *ui.Context, count int) {
	ui.Row(c).Wrap().Gap(c.Theme().Space(4)).Children(func() {
		for range count {
			ui.Column(c).Size(150, 250).Gap(10).Children(func() {
				ui.Box(c).Size(150, 200).Radius(10).Background(c.Theme().SurfaceHover)
				ui.Box(c).Height(14).Radius(6).Background(c.Theme().SurfaceHover)
				ui.Box(c).Width(82).Height(11).Radius(6).Background(c.Theme().SurfaceHover)
			})
		}
	})
}

func (a *App) filterSkeleton(c *ui.Context) {
	ui.Column(c).Padding(16, 20).Gap(14).Radius(12).Border(1, c.Theme().Border).Background(c.Theme().Surface).Children(func() {
		a.filterSkeletonRow(c, 3)
		a.filterSkeletonRow(c, 5)
		a.filterSkeletonRow(c, 4)
	})
}

func (a *App) filterSkeletonRow(c *ui.Context, chips int) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(40, 14).Radius(6).Background(c.Theme().SurfaceHover)
		ui.Row(c).Gap(8).Children(func() {
			for range chips {
				ui.Box(c).Size(52, 28).Radius(14).Background(c.Theme().SurfaceHover)
			}
		})
	})
}

func (a *App) rankSkeleton(c *ui.Context) {
	ui.Column(c).Grow(1).Gap(10).Children(func() {
		for range 8 {
			ui.Row(c).Gap(16).AlignItems(ui.Center).Padding(12, 18).Radius(12).Border(1, c.Theme().Border).Background(c.Theme().Surface).Children(func() {
				ui.Box(c).Size(32, 26).Radius(6).Background(c.Theme().SurfaceHover)
				ui.Box(c).Size(50, 68).Radius(8).Background(c.Theme().SurfaceHover)
				ui.Column(c).Grow(1).Gap(8).Children(func() {
					ui.Box(c).Height(16).Radius(6).Background(c.Theme().SurfaceHover).FillWidth()
					ui.Box(c).Width(130).Height(12).Radius(6).Background(c.Theme().SurfaceHover)
				})
				ui.Box(c).Size(82, 28).Radius(14).Background(c.Theme().SurfaceHover)
			})
		}
	})
}

func (a *App) emptyView(c *ui.Context, title, description string) {
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Text(c, title).Bold()
		ui.Text(c, description).TextColor(c.Theme().TextMuted)
	})
}

func (a *App) errorView(c *ui.Context, message string, retry func()) {
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, message).TextColor(c.Theme().Danger)
		if retry != nil && ui.PrimaryButton(c, "重试").Clicked() {
			go retry()
		}
	})
}

func (a *App) image(c *ui.Context, source string, width, height float32) {
	// Always retain the same sized host, whether loading, failed or decoded.
	// Bounds are from the previous layout, so wait one frame for new hosts.
	host := ui.Box(c).Size(width, height).Radius(10).Clip().Background(c.Theme().SurfaceHover)
	bounds := host.Bounds()
	if bounds.W <= 0 || bounds.H <= 0 {
		c.Invalidate()
		return
	}
	windowWidth, windowHeight := c.Size()
	const preload = float32(300)
	if bounds.Y+bounds.H < -preload || bounds.Y > windowHeight+preload ||
		bounds.X+bounds.W < 0 || bounds.X > windowWidth {
		return
	}
	var bitmap ui.ImageSource
	if a.player != nil {
		bitmap = a.player.Artwork(source)
	}
	if bitmap != nil {
		host.Children(func() {
			ui.Image(c, bitmap).Size(width, height).Fit(ui.Cover).Radius(10)
		})
	}
}

func formatEpisode(number int) string {
	return fmt.Sprintf("第%d集", number)
}

func formatPublishDate(value string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", "20060102"} {
		if date, err := time.Parse(layout, value); err == nil {
			return date.Format("2006-01-02")
		}
	}
	return value
}

// GridView rows have a fixed height. Size them for their actual metadata and
// actions so reservation/removal controls cannot be clipped beneath a poster.
func catalogRowHeight(items []CatalogItem) float32 {
	var tallest float32 = 260
	for _, item := range items {
		var height float32 = 242
		if item.Heat != "" || item.EpisodeCount > 0 {
			height += 26
		}
		if item.BadgeText != "" {
			height += 26
		}
		if item.Category != "" {
			height += 26
		}
		if item.Upcoming {
			height += 70
		}
		tallest = max(tallest, height)
	}
	return tallest
}

func (a *App) renderSettings(c *ui.Context) {
	a.emptyView(c, "设置暂不可用", "请重新打开应用后重试")
}
