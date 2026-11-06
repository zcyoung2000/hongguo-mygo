package uiapp

import (
	_ "embed"
	"fmt"

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

var (
	discoverIcon    = ui.MustParseSVG(discoverSVG)
	searchIcon      = ui.MustParseSVG(searchSVG)
	historyIcon     = ui.MustParseSVG(historySVG)
	favoriteIcon    = ui.MustParseSVG(favoriteSVG)
	reservationIcon = ui.MustParseSVG(reservationSVG)
)

func (a *App) Render(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := c.Theme()
	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(t.Background)
	root.Children(func() {
		sidebar := ui.Column(c).Width(190).Background(t.Surface).Padding(44, 12, 16, 16).Gap(4)
		sidebar.Children(func() {
			ui.Text(c, "MyGo").FontSize(20).Bold().Padding(0, 0, 16)
			a.sidebarItem(c, "discover", "找剧", discoverIcon)
			a.sidebarItem(c, "search", "搜索", searchIcon)
			ui.Text(c, "我的").TextColor(t.TextMuted).Padding(16, 0, 8)
			a.sidebarItem(c, "history", "历史", historyIcon)
			a.sidebarItem(c, "favorites", "收藏", favoriteIcon)
			a.sidebarItem(c, "likes", "点赞", favoriteIcon)
			a.sidebarItem(c, "reservations", "预约", reservationIcon)
		})

		content := ui.Column(c).Grow(1).MinWidth(0).Padding(42, 20, 16, 20).Gap(12)
		content.Children(func() {
			if a.error != "" {
				ui.Text(c, a.error).TextColor(t.Danger)
			}
			if a.notice != "" {
				ui.Text(c, a.notice).TextColor(t.Accent)
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
			default:
				a.renderCatalog(c)
			}
		})

		if a.playerOn {
			player := ui.Column(c).Width(400).Gap(8).Background(t.Surface).Padding(42, 14, 14)
			player.Children(func() {
				ui.Text(c, "播放器").Bold()
				ui.WebView(c, a.player.webview).Grow(1).Radius(12).Background(ui.Hex("#000000"))
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "上一集").Clicked() {
						go a.PlayerPrevious()
					}
					if ui.Button(c, "播放").Clicked() {
						go a.PlayerResume()
					}
					if ui.Button(c, "暂停").Clicked() {
						go a.PlayerPause()
					}
					if ui.Button(c, "下一集").Clicked() {
						go a.PlayerNext()
					}
				})
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "画中画").Clicked() {
						go a.PlayerPictureInPicture()
					}
				})
			})
		}
	})
}

func (a *App) sidebarItem(c *ui.Context, id, label string, icon *ui.SVG) {
	t := c.Theme()
	selected := a.view == id
	row := ui.Row(c.Key(id)).Padding(8, 10).Radius(8).Gap(8)
	if selected {
		row.Background(t.Accent).TextColor(t.AccentText)
	}
	if row.Clicked() {
		go a.OpenView(id)
	}
	row.Children(func() {
		ui.Icon(c, icon)
		ui.Text(c, label)
	})
}

func (a *App) renderCatalog(c *ui.Context) {
	catalog := a.catalog
	ui.Row(c).Gap(8).Children(func() {
		for index, genre := range catalog.Genre {
			if index == catalog.GenreIndex {
				ui.PrimaryButton(c, genre)
			} else if ui.Button(c, genre).Clicked() {
				next := index
				go a.SelectGenre(next)
			}
		}
	})
	scroll := ui.Scroll(c).Grow(1).Gap(12)
	scroll.Children(func() {
		if catalog.Loading && len(catalog.Items) == 0 {
			a.loadingView(c, "正在加载目录...")
			return
		}
		if catalog.Error != "" && len(catalog.Items) == 0 {
			a.errorView(c, catalog.Error, func() { go a.loadCatalog(false) })
			return
		}
		if len(catalog.Items) == 0 {
			a.emptyView(c, "暂无短剧", "换个筛选条件试试")
			return
		}
		ui.GridView(c, nil, len(catalog.Items), 150, 240, func(item int) {
			a.catalogCard(c, catalog.Items[item])
		}).Grow(1)
		if catalog.LoadingMore {
			ui.Row(c).Padding(8).Children(func() { ui.Spinner(c); ui.Text(c, "加载更多...") })
		} else if catalog.HasMore {
			if ui.PrimaryButton(c, "加载更多").Clicked() {
				go a.LoadMoreCatalog()
			}
		}
	})
}

func (a *App) catalogCard(c *ui.Context, item CatalogItem) {
	t := c.Theme()
	card := ui.Column(c.Key(item.ID)).Gap(6)
	if card.Clicked() {
		go a.OpenItem(item)
	}
	card.Children(func() {
		a.image(c, item.CoverURL, 150, 200)
		ui.Text(c, item.Title).Bold().SingleLine()
		ui.Textf(c, "%d集 · %s", item.EpisodeCount, item.Heat).TextColor(t.TextMuted).SingleLine()
	})
}

func (a *App) renderSearch(c *ui.Context) {
	query := a.query
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c, &query).Grow(1).Placeholder("搜索短剧").OnChange(func() { a.SetQuery(query) }).OnSubmit(func() { a.SetQuery(query); go a.SearchSubmitted() })
		if ui.PrimaryButton(c, "搜索").Clicked() {
			a.query = query
			go a.SearchSubmitted()
		}
	})
	search := a.search
	if search.Loading {
		a.loadingView(c, "正在搜索...")
		return
	}
	if search.Error != "" {
		a.errorView(c, search.Error, nil)
		return
	}
	if len(search.Items) == 0 {
		a.emptyView(c, "暂无搜索结果", "请尝试其他关键词")
		return
	}
	ui.GridView(c, nil, len(search.Items), 150, 240, func(item int) {
		a.catalogCard(c, search.Items[item])
	}).Grow(1)
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
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "刷新").Clicked() {
			go a.RefreshLibrary(tab)
		}
		if tab == libraryHistory && ui.Button(c, "清空历史").Clicked() {
			go a.ClearHistory()
		}
	})
	if library.Loading {
		a.loadingView(c, "正在加载本地数据...")
		return
	}
	if library.Error != "" {
		a.errorView(c, library.Error, func() { go a.RefreshLibrary(tab) })
		return
	}
	if library.Empty {
		a.emptyView(c, "暂无记录", "浏览和播放后才会形成真实记录")
		return
	}
	if a.library.Grids[grid] == nil {
		a.library.Grids[grid] = &ui.GridState{}
	}
	a.library.Grids[grid].Key = func(item int) any { return library.Items[item].ID }
	a.library.Grids[grid].Label = func(item int) string { return library.Items[item].Title }
	ui.GridView(c, a.library.Grids[grid], len(library.Items), 150, 240, func(item int) {
		a.catalogCard(c, library.Items[item])
	}).Grow(1)
}

func (a *App) renderDetail(c *ui.Context) {
	detail := a.detail
	if detail.Loading && detail.Title == "" {
		a.loadingView(c, "正在加载详情...")
		return
	}
	if detail.Error != "" && detail.Title == "" {
		a.errorView(c, detail.Error, func() { go a.OpenDetail(detail.ID) })
		return
	}
	scroll := ui.Scroll(c).Grow(1).Gap(12)
	scroll.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			a.image(c, detail.CoverURL, 160, 220)
			header := ui.Column(c).Grow(1).Gap(6)
			header.Children(func() {
				ui.Text(c, detail.Title).FontSize(22).Bold()
				ui.Text(c, detail.Description).TextColor(c.Theme().TextMuted)
				ui.Textf(c, "%s · %d次播放", detail.StatusText, detail.PlayCount).TextColor(c.Theme().TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					if detail.Favorite {
						ui.PrimaryButton(c, "已收藏").OnClick(func() { go a.ToggleDetailFavorite() })
					} else if ui.Button(c, "收藏").Clicked() {
						go a.ToggleDetailFavorite()
					}
					if detail.Reserved {
						ui.PrimaryButton(c, "已预约").OnClick(func() { go a.ToggleDetailReservation() })
					} else if ui.Button(c, "预约").Clicked() {
						go a.ToggleDetailReservation()
					}
				})
			})
		})
		ui.Text(c, "选集").Bold()
		if len(detail.Episodes) == 0 {
			ui.Text(c, "暂无可播放选集").TextColor(c.Theme().TextMuted)
		} else {
			ui.GridView(c, nil, len(detail.Episodes), 72, 48, func(item int) {
				episode := detail.Episodes[item]
				if ui.Button(c, episode.Title).Clicked() {
					go a.ChooseEpisode(episode.EpisodeCount)
				}
			}).Height(160)
		}
		ui.Text(c, detail.RelatedTitle).Bold()
		if detail.RelatedLoading {
			a.loadingView(c, "正在加载相关推荐...")
		} else if len(detail.Related) == 0 {
			ui.Text(c, "暂无相关推荐").TextColor(c.Theme().TextMuted)
		} else {
			ui.GridView(c, nil, len(detail.Related), 150, 240, func(item int) {
				a.catalogCard(c, detail.Related[item])
			}).Height(280)
		}
	})
}

func (a *App) loadingView(c *ui.Context, message string) {
	ui.Row(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Spinner(c)
		ui.Text(c, message)
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
	var bitmap ui.ImageSource
	if a.player != nil {
		bitmap = a.player.Artwork(source)
	}
	if bitmap == nil {
		ui.Box(c).Size(width, height).Radius(10).Background(c.Theme().Surface).Children(func() { ui.Text(c, "暂无海报").TextColor(c.Theme().TextMuted) })
		return
	}
	ui.Image(c, bitmap).Size(width, height).Fit(ui.Cover).Radius(10)
}

func formatEpisode(number int) string {
	return fmt.Sprintf("第%d集", number)
}
