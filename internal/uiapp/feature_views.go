package uiapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func (a *App) renderRank(c *ui.Context) {
	rank := a.rank
	a.maybeLoadMore(a.rankScroll.Y, a.rankScroll.MaxY, rank.HasMore, rank.Loading, func() { a.LoadRank(true) })

	// Top Category Switcher Track: [全部] [系列剧] [漫剧] [真人剧] [AI剧] [演员]
	if len(rank.Contents) > 0 {
		var tabs []slidingTabItem
		for _, content := range rank.Contents {
			tabs = append(tabs, slidingTabItem{Key: content.ID, Label: content.Name})
		}
		a.renderSlidingTabs(c, "rank-contents-track", "rank-contents", tabs, rank.Content, func(cID string) {
			for _, content := range rank.Contents {
				if content.ID == cID {
					board := ""
					if len(content.Boards) > 0 {
						board = content.Boards[0].ID
					}
					go a.SelectRank(cID, board)
					break
				}
			}
		})
	}

	// Sub-category Pills (Board Tabs)
	for _, content := range rank.Contents {
		if content.ID != rank.Content {
			continue
		}
		if len(content.Boards) > 0 {
			ui.Row(c.Key("rank-boards-row")).Wrap().Gap(8).Padding(4, 0).Children(func() {
				for _, board := range content.Boards {
					bID := board.ID
					bName := board.Name
					selected := bID == rank.Board
					a.pillChip(c, "board:"+bID, bName, selected, func() {
						go a.SelectRank(content.ID, bID)
					})
				}
			})
		}
	}

	if rank.Error != "" {
		a.errorView(c, rank.Error, func() { go a.LoadRank(len(rank.Items) > 0) })
	}
	if rank.Loading && len(rank.Items) == 0 {
		ui.Text(c, "正在加载排行榜...").TextColor(c.Theme().TextMuted)
		a.rankSkeleton(c)
		return
	}
	if len(rank.Items) == 0 {
		a.emptyView(c, "暂无上榜短剧", "可以选择其他榜单")
		return
	}
	ui.Scroll(c.Key("rank-scroll")).TrackScroll(&a.rankScroll).Grow(1).Gap(10).Children(func() {
		for index, item := range rank.Items {
			a.rankItem(c, index, item)
		}
		a.scrollLoading(c, rank.Loading)
	})
}

func (a *App) rankItem(c *ui.Context, index int, item ipc.RankItemDTO) {
	actorID := item.UserID
	if actorID == "" {
		actorID = item.ActorID
	}
	id, cover := item.SeriesID, item.CoverURL
	if item.ItemType == "actor" {
		id = actorID
		if item.AvatarURL != "" {
			cover = item.AvatarURL
		}
	}
	positionLabel := fmt.Sprintf("第%d名", item.Position)
	if item.Promoted {
		positionLabel = "平台推荐"
	}

	row := ui.Row(c.Key(fmt.Sprintf("%s|%s|%d", item.SeriesID, actorID, index))).
		Gap(16).AlignItems(ui.Center).Padding(12, 18).Radius(12).
		Border(1, ui.RGBA(255, 255, 255, 0.06)).
		Background(ui.RGBA(255, 255, 255, 0.035)).
		Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, 0.25))

	if row.Hovered() {
		row.Background(ui.RGBA(255, 255, 255, 0.065)).Border(1, ui.RGBA(255, 255, 255, 0.12))
	}
	row.Role(ui.RoleButton).Label(positionLabel + " " + item.Title).Focusable()
	if item.ItemType != "actor" && (row.Clicked() || row.Shortcut(0, ui.KeyEnter) || row.Shortcut(0, ui.KeySpace)) {
		go a.OpenItem(CatalogItem{ID: id, ResumeEpisode: 1, ResumePosition: -1})
	}

	row.Children(func() {
		// Big rank number or promoted text
		ui.Box(c).Width(36).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			if item.Promoted {
				ui.Text(c, "平台推荐").FontSize(11).Bold().TextColor(ui.Hex("#fe2c55"))
			} else {
				posStr := fmt.Sprintf("%d", item.Position)
				color := rankPositionColor(item.Position, false)
				size := float32(22)
				if item.Position >= 10 {
					size = 18
				}
				ui.Text(c, posStr).FontSize(size).Bold().Italic().TextColor(color)
			}
		})

		// Poster thumbnail (3:4)
		a.image(c, cover, 48, 64)

		// Title & metadata
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, item.Title).FontSize(14).Bold().TextColor(ui.Hex("#ffffff")).SingleLine()
			meta := []string{}
			if item.ItemType != "actor" {
				meta = append(meta, "短剧")
			}
			if item.Heat != "" {
				meta = append(meta, item.Heat)
			}
			if item.EpisodeCount > 0 {
				meta = append(meta, fmt.Sprintf("%d集", item.EpisodeCount))
			}
			if item.ItemType == "actor" {
				if item.Intro != "" {
					meta = append(meta, item.Intro)
				}
				if len(item.Details) > 0 {
					meta = append(meta, strings.Join(item.Details, " · "))
				}
			}
			if len(meta) > 0 {
				ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.60)).SingleLine()
			}
		})

		// Right Action Button
		if item.ItemType == "actor" {
			actBtn := ui.Row(c.Key("actor-btn:"+actorID)).Height(32).Padding(0, 14).Radius(8).
				Background(ui.RGBA(255, 255, 255, 0.08)).Border(1, ui.RGBA(255, 255, 255, 0.12)).
				AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label("主页")
			if actBtn.Hovered() {
				actBtn.Background(ui.RGBA(255, 255, 255, 0.14))
			}
			if actBtn.Clicked() {
				go a.OpenActor(actorID)
			}
			actBtn.Children(func() {
				ui.Text(c, "主页").FontSize(12).TextColor(ui.Hex("#ffffff"))
			})
		} else {
			playBtn := ui.Row(c.Key("play-btn:"+id)).Height(32).Padding(0, 16).Radius(8).
				LinearGradient(ui.LinearGradient{
					From:  ui.Hex("#d91b42"),
					To:    ui.Hex("#b81235"),
					Angle: 180,
				}).Border(1, ui.RGBA(255, 255, 255, 0.20)).
				Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.35)).
				AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label("立即播放")
			if playBtn.Hovered() {
				playBtn.LinearGradient(ui.LinearGradient{
					From:  ui.Hex("#e5244b"),
					To:    ui.Hex("#c4173b"),
					Angle: 180,
				})
			}
			if playBtn.Clicked() {
				go a.OpenItem(CatalogItem{ID: id, ResumeEpisode: 1, ResumePosition: -1})
			}
			playBtn.Children(func() {
				ui.Text(c, "立即播放").FontSize(12).Bold().TextColor(ui.Hex("#ffffff"))
			})
		}
	})
}

func rankPositionColor(position int, promoted bool) ui.Color {
	if promoted {
		return ui.RGB(255, 44, 85)
	}
	switch position {
	case 1:
		return ui.RGB(255, 215, 0)
	case 2:
		return ui.RGB(209, 213, 219)
	case 3:
		return ui.RGB(245, 158, 11)
	}
	return ui.RGBA(255, 255, 255, 0.45)
}

func (a *App) renderReleases(c *ui.Context) {
	value := a.releases
	a.renderSlidingTabs(c, "releases-track", "releases-kind", []slidingTabItem{
		{"hot", "热播新剧"},
		{"calendar", "新剧日历"},
	}, func() string {
		if value.Calendar {
			return "calendar"
		}
		return "hot"
	}(), func(key string) {
		go a.SelectRelease(key == "calendar", "")
	})
	if value.Calendar {
		ui.Row(c).Wrap().Gap(8).Padding(4, 0).Children(func() {
			for _, date := range value.Dates {
				dStr := date
				selected := dStr == value.Date
				tab := ui.Column(c.Key(dStr)).Padding(6, 14).Radius(10).
					Border(1, ui.RGBA(255, 255, 255, 0.08)).
					Background(ui.RGBA(255, 255, 255, 0.04)).AlignItems(ui.Center).Gap(2)
				if selected {
					tab.LinearGradient(ui.LinearGradient{
						From:  ui.Hex("#d91b42"),
						To:    ui.Hex("#b81235"),
						Angle: 135,
					}).Border(1, ui.RGBA(255, 255, 255, 0.22)).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.35))
				} else if tab.Hovered() {
					tab.Background(ui.RGBA(255, 255, 255, 0.08))
				}
				tab.Role(ui.RoleButton).Label(formatCalendarLabel(dStr)).Focusable()
				if tab.Clicked() || tab.Shortcut(0, ui.KeyEnter) || tab.Shortcut(0, ui.KeySpace) {
					go a.SelectRelease(true, dStr)
				}
				tab.Children(func() {
					txtW := ui.Text(c, calendarWeekday(dStr)).FontSize(11)
					txtD := ui.Text(c, calendarDate(dStr)).FontSize(13).Bold()
					if selected {
						txtW.TextColor(ui.Hex("#ffffff"))
						txtD.TextColor(ui.Hex("#ffffff"))
					} else {
						txtW.TextColor(ui.RGBA(255, 255, 255, 0.60))
						txtD.TextColor(ui.RGBA(255, 255, 255, 0.85))
					}
				})
			}
		})
	}
	if value.Error != "" {
		a.errorView(c, value.Error, func() { go a.LoadReleases() })
	}
	if value.Loading {
		ui.Text(c, "正在加载新剧...").TextColor(c.Theme().TextMuted)
		a.gridSkeleton(c, 12)
		return
	}
	if len(value.Items) == 0 {
		a.emptyView(c, "暂无新剧", "可以选择其他日期")
		return
	}
	a.cardGrid(c, "releases-grid", nil, nil, value.Items, 0, nil).Grow(1)
}
func (a *App) renderActor(c *ui.Context) {
	actor := a.actor
	a.maybeLoadMore(a.actorScroll.Y, a.actorScroll.MaxY, actor.HasMore, actor.Loading || actor.Profile == nil, func() { a.loadActor(true) })

	// Top Bar with back button
	ui.Row(c.Key("actor-top-bar")).AlignItems(ui.Center).Gap(12).Padding(0, 0, 8, 0).Children(func() {
		backBtn := ui.Row(c.Key("actor-nav-back")).Height(30).Padding(0, 14).Radius(7).
			Material(glass.Glass{Interactive: true}).
			Border(1, ui.RGBA(255, 255, 255, 0.12)).
			AlignItems(ui.Center).Role(ui.RoleButton).Label("返回短剧详情").Focusable()
		if backBtn.Hovered() {
			backBtn.Background(ui.RGBA(255, 255, 255, 0.10))
		}
		if backBtn.Clicked() || backBtn.Shortcut(0, ui.KeyEnter) || backBtn.Shortcut(0, ui.KeySpace) {
			go a.NavigateHistory(-1)
		}
		backBtn.Children(func() {
			ui.Text(c, "← 返回").FontSize(13).TextColor(ui.Hex("#ffffff")).Bold()
		})
		ui.Text(c, "演员个人主页").FontSize(13).TextColor(ui.RGBA(255, 255, 255, 0.50))
	})

	if actor.Profile != nil {
		profile := actor.Profile
		ui.Row(c).Gap(12).Children(func() {
			a.image(c, profile.AvatarURL, 90, 90)
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, profile.Name).FontSize(22).Bold()
				ui.Text(c, profile.Description)
				ui.Text(c, strings.Join(append(append([]string(nil), profile.Tags...), profile.Labels...), " · "))
				ui.Textf(c, "%d 粉丝 · %d 获赞 · %d 部参演短剧", profile.Followers, profile.Likes, len(actor.Works))
			})
		})
	}
	if actor.Error != "" {
		a.errorView(c, actor.Error, func() { go a.loadActor(actor.Profile != nil) })
	}
	if actor.Loading && actor.Profile == nil {
		ui.Text(c, "正在加载演员主页...").TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(18).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(90, 90).Radius(45).Background(c.Theme().SurfaceHover)
			ui.Column(c).Grow(1).Gap(12).Children(func() {
				ui.Box(c).Width(180).Height(26).Radius(8).Background(c.Theme().SurfaceHover)
				ui.Row(c).Gap(18).Children(func() {
					for range 3 {
						ui.Box(c).Size(64, 22).Radius(8).Background(c.Theme().SurfaceHover)
					}
				})
				ui.Box(c).Height(13).Radius(6).Background(c.Theme().SurfaceHover).FillWidth()
			})
		})
		ui.Box(c).Height(18)
		a.gridSkeleton(c, 12)
		return
	}
	a.cardGrid(c, "actor-grid", nil, &a.actorScroll, actor.Works, 0, nil).Grow(1)
	a.scrollLoading(c, actor.Loading)
}
func (a *App) renderCollection(c *ui.Context, label string, value DetailCollection, retry func()) {
	if value.Title != "" {
		label = value.Title
	}
	if !value.Loading && value.Error == "" && len(value.Items) == 0 {
		return
	}
	section := ui.Column(c.Key("detail-collection:" + label)).Gap(8)
	section.Children(func() {
		ui.Text(c, label).Bold().FontSize(15).TextColor(ui.Hex("#ffffff"))
		if value.Loading {
			ui.Text(c, "正在加载...").FontSize(12).TextColor(c.Theme().TextMuted)
			return
		}
		if value.Error != "" {
			a.errorView(c, value.Error, retry)
			return
		}
		ui.ScrollHorizontal(c.Key("col-scroll-" + label)).Height(catalogRowHeight(value.Items)).Children(func() {
			ui.Row(c.Key("collection-" + label)).Gap(c.Theme().Space(3)).Children(func() {
				for _, item := range value.Items {
					a.catalogCard(c, item)
				}
			})
		})
	})
}

func calendarWeekday(value string) string {
	date, ok := parseCalendarDate(value)
	if !ok {
		return "日期"
	}
	return []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[date.Weekday()]
}

func calendarDate(value string) string {
	date, ok := parseCalendarDate(value)
	if !ok {
		return value
	}
	return fmt.Sprintf("%02d/%02d", int(date.Month()), date.Day())
}

func formatCalendarLabel(value string) string {
	date, ok := parseCalendarDate(value)
	if !ok {
		return value
	}
	return date.Format("2006-01-02")
}

func parseCalendarDate(value string) (time.Time, bool) {
	for _, layout := range []string{"20060102", "2006-01-02"} {
		if date, err := time.Parse(layout, value); err == nil {
			return date, true
		}
	}
	return time.Time{}, false
}
