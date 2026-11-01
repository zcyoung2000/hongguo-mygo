package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type MediaCommandPayload struct {
	Command  string  `json:"command"` // "play", "pause", "toggle", "next", "prev", "seek", "pip"
	Position float64 `json:"position,omitempty"`
}

type NavigatePayload struct {
	View     string `json:"view,omitempty"`
	SeriesID string `json:"seriesId,omitempty"`
}

type UpdateNowPlayingArgs struct {
	Title    string  `json:"title"`
	Series   string  `json:"series"`
	Episode  int     `json:"episode"`
	Duration float64 `json:"duration"`
	Position float64 `json:"position"`
	Playing  bool    `json:"playing"`
	Rate     float64 `json:"rate"`
	CoverURL string  `json:"coverUrl"`
}

type ConfirmDialogArgs struct {
	Title       string `json:"title"`
	Message     string `json:"message"`
	Detail      string `json:"detail"`
	ConfirmText string `json:"confirmText"`
	CancelText  string `json:"cancelText"`
	Destructive bool   `json:"destructive"`
}

type NotificationArgs struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	SeriesID string `json:"seriesId,omitempty"`
}

var (
	mediaCommandEvent = mygo.NewEvent[MediaCommandPayload]("mediaCommand")
	navigateEvent     = mygo.NewEvent[NavigatePayload]("navigate")

	powerMu      sync.Mutex
	powerRelease func()

	nowPlayingMu sync.Mutex
	lastNowArgs  UpdateNowPlayingArgs
)

func initNativeFeatures() {
	initNowPlaying()
	setupAppMenu()
}

func cleanupNativeFeatures() {
	powerMu.Lock()
	if powerRelease != nil {
		powerRelease()
		powerRelease = nil
	}
	powerMu.Unlock()

	mygo.NowPlaying.Clear()
}

func initNowPlaying() {
	mygo.NowPlaying.OnCommand(func(cmd mygo.MediaCommand, pos time.Duration) {
		if mainWindow == nil {
			return
		}
		var action string
		switch cmd {
		case mygo.MediaPlay:
			action = "play"
		case mygo.MediaPause:
			action = "pause"
		case mygo.MediaNext:
			action = "next"
		case mygo.MediaPrevious:
			action = "prev"
		case mygo.MediaSeek:
			action = "seek"
		default:
			action = "toggle"
		}
		_ = mediaCommandEvent.Emit(mainWindow, MediaCommandPayload{
			Command:  action,
			Position: pos.Seconds(),
		})
	})
}

func setupAppMenu() {
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{
			Label: "文件",
			Submenu: []*mygo.MenuItem{
				{
					Label:       "发现推荐",
					Accelerator: "CmdOrCtrl+1",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("recommend", "")
					},
				},
				{
					Label:       "分类找剧",
					Accelerator: "CmdOrCtrl+2",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("discover", "")
					},
				},
				{
					Label:       "短剧榜单",
					Accelerator: "CmdOrCtrl+3",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("rank", "")
					},
				},
				{
					Label:       "最新上线",
					Accelerator: "CmdOrCtrl+4",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("new", "")
					},
				},
				mygo.Separator(),
				{
					Label:       "播放历史",
					Accelerator: "CmdOrCtrl+5",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("history", "")
					},
				},
				{
					Label:       "我的收藏",
					Accelerator: "CmdOrCtrl+6",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("favorites", "")
					},
				},
				{
					Label:       "新剧预约",
					Accelerator: "CmdOrCtrl+7",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitNavigate("reservations", "")
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleClose},
			},
		},
		{Role: mygo.RoleEditMenu},
		{
			Label: "播放",
			Submenu: []*mygo.MenuItem{
				{
					Label: "播放 / 暂停",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitMediaCommand("toggle", 0)
					},
				},
				{
					Label:       "上一集",
					Accelerator: "CmdOrCtrl+[",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitMediaCommand("prev", 0)
					},
				},
				{
					Label:       "下一集",
					Accelerator: "CmdOrCtrl+]",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitMediaCommand("next", 0)
					},
				},
				mygo.Separator(),
				{
					Label:       "画中画小窗",
					Accelerator: "CmdOrCtrl+P",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						emitMediaCommand("pip", 0)
					},
				},
				{Role: mygo.RoleToggleFullScreen},
			},
		},
		{
			Label: "视图",
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleReload},
				{Role: mygo.RoleForceReload},
				{Role: mygo.RoleToggleDevTools},
				mygo.Separator(),
				{Role: mygo.RoleResetZoom},
				{Role: mygo.RoleZoomIn},
				{Role: mygo.RoleZoomOut},
			},
		},
		{Role: mygo.RoleWindowMenu},
	})
	mygo.App.SetMenu(menu)
}

func emitMediaCommand(cmd string, pos float64) {
	if mainWindow == nil {
		return
	}
	_ = mediaCommandEvent.Emit(mainWindow, MediaCommandPayload{
		Command:  cmd,
		Position: pos,
	})
}

func emitNavigate(view, seriesID string) {
	if mainWindow == nil {
		return
	}
	mainWindow.Focus()
	_ = navigateEvent.Emit(mainWindow, NavigatePayload{
		View:     view,
		SeriesID: seriesID,
	})
}

func updateNowPlaying(cacheDir string, args UpdateNowPlayingArgs) {
	nowPlayingMu.Lock()
	defer nowPlayingMu.Unlock()

	lastNowArgs = args

	state := mygo.PlaybackPaused
	if args.Playing {
		state = mygo.PlaybackPlaying
	}
	rate := args.Rate
	if rate <= 0 {
		rate = 1.0
	}

	artist := args.Series
	if args.Episode > 0 {
		if artist != "" {
			artist = fmt.Sprintf("%s · 第 %d 集", artist, args.Episode)
		} else {
			artist = fmt.Sprintf("第 %d 集", args.Episode)
		}
	}

	var artworkBytes []byte
	if args.CoverURL != "" && cacheDir != "" {
		hexHash := fmt.Sprintf("%x", sha256.Sum256([]byte(args.CoverURL)))
		coverPath := filepath.Join(cacheDir, "images", hexHash+".jpg")
		if data, err := os.ReadFile(coverPath); err == nil && len(data) > 0 {
			artworkBytes = data
		}
	}

	dur := time.Duration(args.Duration * float64(time.Second))
	pos := time.Duration(args.Position * float64(time.Second))

	mygo.NowPlaying.Set(mygo.NowPlayingInfo{
		Title:    args.Title,
		Artist:   artist,
		Album:    "红果短剧",
		Artwork:  artworkBytes,
		Duration: dur,
		Position: pos,
		Rate:     rate,
		State:    state,
		Commands: mygo.MediaPlay | mygo.MediaPause | mygo.MediaNext | mygo.MediaPrevious | mygo.MediaSeek,
	})
}

func clearNowPlaying() {
	nowPlayingMu.Lock()
	defer nowPlayingMu.Unlock()
	lastNowArgs = UpdateNowPlayingArgs{}
	mygo.NowPlaying.Clear()
}

func setPlaybackKeepAwake(active bool) {
	if !mygo.App.IsReady() {
		return
	}
	powerMu.Lock()
	defer powerMu.Unlock()

	if active {
		if powerRelease == nil {
			powerRelease = mygo.Power.KeepAwake("Hongguo Video Playback", true)
		}
	} else {
		if powerRelease != nil {
			powerRelease()
			powerRelease = nil
		}
	}
}

func showConfirmDialog(args ConfirmDialogArgs) (bool, error) {
	if !mygo.App.IsReady() {
		return false, nil
	}
	confirmText := args.ConfirmText
	if confirmText == "" {
		confirmText = "确认"
	}
	cancelText := args.CancelText
	if cancelText == "" {
		cancelText = "取消"
	}

	msgType := mygo.MessageQuestion
	if args.Destructive {
		msgType = mygo.MessageWarning
	}

	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:        mainWindow,
		Type:          msgType,
		Title:         args.Title,
		Message:       args.Message,
		Detail:        args.Detail,
		Buttons:       []string{confirmText, cancelText},
		DefaultButton: 0,
		CancelButton:  1,
	})
	if err != nil {
		return false, err
	}
	return res.Button == 0, nil
}

func sendNativeNotification(args NotificationArgs) (bool, error) {
	if !mygo.App.IsReady() || !mygo.NotificationsSupported() {
		return false, nil
	}
	n := mygo.NewNotification(mygo.NotificationOptions{
		Title: args.Title,
		Body:  args.Body,
	})
	n.OnClick(func() {
		if mainWindow != nil {
			mainWindow.Focus()
			if args.SeriesID != "" {
				_ = navigateEvent.Emit(mainWindow, NavigatePayload{
					View:     "detail",
					SeriesID: args.SeriesID,
				})
			}
		}
	})
	if err := n.Show(); err != nil {
		return false, err
	}
	return true, nil
}

func togglePictureInPicture() {
	if !mygo.App.IsReady() {
		return
	}
	if mainWindow != nil && mainWindow.Page() != nil {
		mainWindow.Page().TogglePictureInPicture()
	}
}
