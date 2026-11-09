package uiapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

type settingsProvider interface {
	GetProfile(context.Context) (ipc.UserProfile, error)
	GetCacheSize(context.Context) (int64, error)
	GetDeviceInfo(context.Context) (map[string]string, error)
	CheckExistingAccount(context.Context) (map[string]any, error)
	LoginWithCookies(context.Context, string, string) (*ipc.UserProfile, error)
	RefreshUserProfile(context.Context) (*ipc.UserProfile, error)
	Logout(context.Context) error
	ImportHgplayerAccount(context.Context) (*ipc.UserProfile, error)
	ClearCache(context.Context) (int64, error)
	ResetDevice(context.Context) error
}

func (a *App) settingsProvider() settingsProvider {
	if a.settingsTestProvider != nil {
		return a.settingsTestProvider
	}
	return a.service
}

func (a *App) LoadSettings() {
	a.mu.Lock()
	if !a.settings.Loading {
		a.mu.Unlock()
		return
	}
	version := uint64(a.generation)
	provider := a.settingsProvider()
	if provider == nil {
		a.finishSettingsLoadLocked(version, ipc.UserProfile{}, 0, nil, nil, errSettingsUnavailable)
		return
	}
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	profile, profileErr := provider.GetProfile(ctx)
	cacheSize, cacheErr := provider.GetCacheSize(ctx)
	device, deviceErr := provider.GetDeviceInfo(ctx)
	legacy, legacyErr := provider.CheckExistingAccount(ctx)
	err := firstError(profileErr, cacheErr, deviceErr, legacyErr)
	a.finishSettingsLoad(version, profile, cacheSize, device, legacy, err)
}

func (a *App) finishSettingsLoad(version uint64, profile ipc.UserProfile, cacheSize int64, device map[string]string, legacy map[string]any, err error) {
	a.mu.Lock()
	a.finishSettingsLoadLocked(version, profile, cacheSize, device, legacy, err)
}

func (a *App) finishSettingsLoadLocked(version uint64, profile ipc.UserProfile, cacheSize int64, device map[string]string, legacy map[string]any, err error) {
	defer a.mu.Unlock()
	if version != uint64(a.generation) {
		return
	}
	a.settings.Profile = profile
	a.settings.CacheSize = cacheSize
	a.settings.DeviceID = device["deviceId"]
	a.settings.DeviceVersion = device["version"]
	a.settings.DeviceArch = device["arch"]
	a.settings.HasLegacy = !profile.LoggedIn && legacy != nil && legacy["exists"] == true
	a.settings.LegacyNickname, _ = legacy["nickname"].(string)
	a.settings.LegacyUserID, _ = legacy["userId"].(string)
	a.settings.Loading = false
	a.settings.Error = settingsError(err)
	a.invalidate()
}

func settingsError(err error) string {
	if err == nil {
		return ""
	}
	return "设置加载失败：" + err.Error()
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

var errSettingsUnavailable = errors.New("设置服务不可用")

func formatCacheSize(size int64) string {
	const mb = 1024 * 1024
	return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
}

func accountBadge(loggedIn bool) string {
	if loggedIn {
		return "已登录官方账号"
	}
	return "匿名设备访问"
}

func legacyAccountName(state Settings) string {
	if state.LegacyNickname != "" {
		return state.LegacyNickname
	}
	if state.LegacyUserID != "" {
		return state.LegacyUserID
	}
	return "旧账号"
}

func (a *App) SetSettingsLoginOpen(open bool) {
	a.mu.Lock()
	a.settings.LoginOpen = open
	if !open {
		a.settings.LoginCookie, a.settings.LoginToken = "", ""
	}
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) SetSettingsCookie(value string) {
	a.mu.Lock()
	a.settings.LoginCookie = value
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) SetSettingsToken(value string) {
	a.mu.Lock()
	a.settings.LoginToken = value
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) PasteLoginCookie() {
	value := strings.TrimSpace(mygo.Clipboard.ReadText())
	if value == "" {
		a.setNotice("剪贴板为空")
		return
	}
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		cookie, token := parseCredentialJSON(value)
		if cookie != "" {
			a.mu.Lock()
			a.settings.LoginCookie, a.settings.LoginToken = cookie, token
			a.mu.Unlock()
			a.invalidate()
			return
		}
	}
	a.mu.Lock()
	a.settings.LoginCookie = value
	a.mu.Unlock()
	a.invalidate()
}

func parseCredentialJSON(value string) (cookie, token string) {
	var credential struct {
		Cookies string `json:"cookies"`
		Token   string `json:"token"`
	}
	if json.Unmarshal([]byte(value), &credential) != nil {
		return "", ""
	}
	return strings.TrimSpace(credential.Cookies), strings.TrimSpace(credential.Token)
}

func (a *App) SubmitSettingsLogin() {
	a.mu.Lock()
	cookie, token := strings.TrimSpace(a.settings.LoginCookie), strings.TrimSpace(a.settings.LoginToken)
	if cookie == "" {
		a.settings.Error = "请输入有效的 Cookie 凭据"
		a.mu.Unlock()
		a.invalidate()
		return
	}
	key := "settings-login"
	if !a.beginSettingsMutationLocked(key) {
		a.mu.Unlock()
		return
	}
	provider := a.settingsProvider()
	a.mu.Unlock()
	go a.runSettingsMutation(key, func(ctx context.Context) (string, error) {
		profile, err := provider.LoginWithCookies(ctx, cookie, token)
		if err != nil {
			return "", err
		}
		return "登录成功！欢迎回来，" + settingsDisplayName(profile.Nickname), nil
	})
}

func settingsDisplayName(value string) string {
	if strings.TrimSpace(value) == "" {
		return "用户"
	}
	return value
}

func (a *App) RefreshSettingsProfile() {
	a.beginSettingsAction("settings-refresh", func(ctx context.Context, provider settingsProvider) (string, error) {
		if _, err := provider.RefreshUserProfile(ctx); err != nil {
			return "", err
		}
		return "个人资料已刷新", nil
	})
}

func (a *App) LogoutSettings() {
	a.mu.Lock()
	confirm := a.confirm
	loggedIn := a.settings.Profile.LoggedIn
	a.mu.Unlock()
	if loggedIn && confirm != nil && !confirm("确定要退出当前登录账号吗？") {
		return
	}
	a.beginSettingsAction("settings-logout", func(ctx context.Context, provider settingsProvider) (string, error) {
		if err := provider.Logout(ctx); err != nil {
			return "", err
		}
		return "已安全退出登录", nil
	})
}

func (a *App) ImportSettingsLegacy() {
	a.beginSettingsAction("settings-import", func(ctx context.Context, provider settingsProvider) (string, error) {
		profile, err := provider.ImportHgplayerAccount(ctx)
		if err != nil {
			return "", err
		}
		return "账号导入成功！当前用户：" + settingsDisplayName(profile.Nickname), nil
	})
}

func (a *App) ClearSettingsCache() {
	a.beginSettingsAction("settings-clear-cache", func(ctx context.Context, provider settingsProvider) (string, error) {
		cleared, err := provider.ClearCache(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("已清理 %s 本地媒体缓存", formatCacheSize(cleared)), nil
	})
}

func (a *App) ResetSettingsDevice() {
	a.mu.Lock()
	confirm := a.confirm
	a.mu.Unlock()
	if confirm != nil && !confirm("确认重置匿名设备指纹？") {
		return
	}
	a.beginSettingsAction("settings-reset-device", func(ctx context.Context, provider settingsProvider) (string, error) {
		if err := provider.ResetDevice(ctx); err != nil {
			return "", err
		}
		return "已成功重置匿名设备", nil
	})
}

func (a *App) beginSettingsAction(key string, action func(context.Context, settingsProvider) (string, error)) {
	a.mu.Lock()
	if !a.beginSettingsMutationLocked(key) {
		a.mu.Unlock()
		return
	}
	provider := a.settingsProvider()
	a.mu.Unlock()
	go a.runSettingsMutation(key, func(ctx context.Context) (string, error) {
		return action(ctx, provider)
	})
}

func (a *App) beginSettingsMutationLocked(key string) bool {
	if a.settingsMutations == nil {
		a.settingsMutations = map[string]bool{}
	}
	if a.settingsMutations[key] {
		return false
	}
	a.settingsMutations[key] = true
	a.invalidate()
	return true
}

func (a *App) runSettingsMutation(key string, action func(context.Context) (string, error)) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	_, err := action(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.settingsMutations, key)
	a.settings.Error = settingsError(err)
	if err != nil {
		return
	}
	a.invalidate()
}

func (a *App) settingsButton(c *ui.Context, key, label, variant string, action func()) {
	row := ui.Row(c.Key(key)).Height(32).Padding(0, 14).Radius(8).
		AlignItems(ui.Center).Justify(ui.Center).
		Role(ui.RoleButton).Label(label).Focusable()
	switch variant {
	case "primary":
		row.LinearGradient(ui.LinearGradient{From: ui.Hex("#d91b42"), To: ui.Hex("#b81235"), Angle: 135}).
			Border(1, ui.RGBA(255, 255, 255, 0.18))
		hoverBackground(row, ui.RGB(209, 27, 66), ui.RGB(229, 36, 75))
	case "danger":
		row.Background(ui.RGBA(255, 77, 95, 0.14)).Border(1, ui.RGBA(255, 77, 95, 0.30))
		hoverBackground(row, ui.RGBA(255, 77, 95, 0.14), ui.RGBA(255, 77, 95, 0.24))
	case "accent":
		row.Background(ui.RGBA(254, 44, 85, 0.16)).Border(1, ui.RGBA(254, 44, 85, 0.36))
		hoverBackground(row, ui.RGBA(254, 44, 85, 0.16), ui.RGBA(254, 44, 85, 0.28))
	default:
		row.Background(ui.RGBA(255, 255, 255, 0.06)).Border(1, ui.RGBA(255, 255, 255, 0.09))
		hoverBackground(row, ui.RGBA(255, 255, 255, 0.06), ui.RGBA(255, 255, 255, 0.12))
	}
	if action != nil && row.Clicked() {
		go action()
	}
	row.Children(func() {
		text := ui.Text(c, label).FontSize(12)
		switch variant {
		case "primary":
			text.Bold().TextColor(ui.Hex("#ffffff"))
		case "danger":
			text.TextColor(ui.Hex("#ff8f98"))
		case "accent":
			text.TextColor(ui.Hex("#ff5d7c"))
		default:
			text.TextColor(ui.RGBA(255, 255, 255, 0.82))
		}
	})
}

func (a *App) settingsCard(c *ui.Context, key string, children func()) {
	ui.Column(c.Key(key)).Grow(1).Gap(16).Padding(20, 22).Radius(12).
		Background(ui.RGBA(20, 21, 28, 255)).
		Border(1, ui.RGBA(255, 255, 255, 0.08)).
		Children(children)
}

func (a *App) settingsItemRow(c *ui.Context, key, title, description, value, actionLabel, variant string, action func()) {
	ui.Row(c.Key(key)).Gap(18).AlignItems(ui.Center).Children(func() {
		info := ui.Column(c.Key(key + "-info")).Grow(1).Gap(4).MinWidth(0)
		info.Children(func() {
			ui.Text(c, title).FontSize(14).Bold().TextColor(ui.Hex("#ffffff")).SingleLine()
			ui.Text(c, description).FontSize(12).TextColor(c.Theme().TextMuted)
		})
		ui.Row(c.Key(key + "-right")).Gap(12).AlignItems(ui.Center).Justify(ui.End).MinWidth(190).Children(func() {
			if value != "" {
				ui.Text(c, value).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.78)).SingleLine()
			}
			if actionLabel != "" {
				a.settingsButton(c, key+"-action", actionLabel, variant, action)
			}
		})
	})
}

func settingsMeta(label, value string) (string, float32) {
	return label + ":" + value, 76
}

func (a *App) renderSettings(c *ui.Context) {
	pageHeader(c, "设置")
	state := a.settings
	scroll := ui.Scroll(c.Key("settings-scroll")).Grow(1).Gap(22).Padding(0, 0, 18, 0)
	scroll.Children(func() {
		state = a.settings
		a.settingsCard(c, "settings-account", func() {
			ui.Row(c.Key("settings-profile")).Gap(18).AlignItems(ui.Center).Children(func() {
				avatar := ui.Box(c.Key("settings-avatar-host")).Size(58, 58).Radius(29).
					Background(ui.Hex("#202230")).Border(2, ui.RGBA(255, 255, 255, 0.16)).Clip()
				avatar.Children(func() {
					if state.Profile.Avatar != "" {
						a.image(c, state.Profile.Avatar, 54, 54)
					}
				})
				profile := ui.Column(c.Key("settings-profile-info")).Grow(1).Gap(6).MinWidth(0)
				profile.Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						nickname := settingsDisplayName(state.Profile.Nickname)
						if !state.Profile.LoggedIn && state.Profile.Nickname == "" {
							nickname = "myguo用户"
						}
						ui.Text(c, nickname).FontSize(16).Bold().TextColor(ui.Hex("#ffffff")).SingleLine()
						badge := ui.Row(c).Padding(2, 8).Radius(12)
						badgeColor := ui.RGBA(255, 255, 255, 0.08)
						textColor := ui.RGBA(255, 255, 255, 0.70)
						if state.Profile.LoggedIn {
							badgeColor = ui.RGBA(34, 197, 94, 0.15)
							textColor = ui.Hex("#4ade80")
						}
						badge.Background(badgeColor).Children(func() {
							ui.Text(c, accountBadge(state.Profile.LoggedIn)).FontSize(11).TextColor(textColor)
						})
					})
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "账号 ID:").FontSize(12).TextColor(c.Theme().TextMuted)
						ui.Text(c, settingsAccountID(state)).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.72)).SingleLine()
					})
				})
				actions := ui.Row(c.Key("settings-profile-actions")).Gap(10).AlignItems(ui.Center).Justify(ui.End).MinWidth(250)
				actions.Children(func() {
					a.settingsButton(c, "refresh-profile", "刷新资料", "subtle", a.RefreshSettingsProfile)
					if state.Profile.LoggedIn {
						a.settingsButton(c, "logout", "退出登录", "danger", a.LogoutSettings)
						a.settingsButton(c, "show-login", "切换账号", "primary", func() { a.SetSettingsLoginOpen(true) })
					} else {
						a.settingsButton(c, "show-login", "立即登录", "primary", func() { a.SetSettingsLoginOpen(true) })
					}
				})
			})
			if !state.Profile.LoggedIn && state.HasLegacy {
				ui.Row(c.Key("settings-import-banner")).Gap(16).AlignItems(ui.Center).Padding(12, 16).Radius(10).
					Background(ui.RGBA(254, 44, 85, 0.08)).Border(1, ui.RGBA(254, 44, 85, 0.25)).Children(func() {
					ui.Column(c.Key("legacy-info")).Grow(1).Gap(2).Children(func() {
						ui.Text(c, "检测到本机已有登录凭据").FontSize(13).Bold().TextColor(ui.Hex("#ffffff"))
						ui.Textf(c, "检测到本机旧版账号：%s，支持一键同步", legacyAccountName(state)).FontSize(11).TextColor(c.Theme().TextMuted)
					})
					a.settingsButton(c, "import-legacy", "一键同步登录", "accent", a.ImportSettingsLegacy)
				})
			}
			if state.LoginOpen {
				a.renderSettingsLogin(c, state)
			}
		})

		ui.Text(c, "设备与存储管理").FontSize(12).Bold().TextColor(c.Theme().TextMuted)
		a.settingsCard(c, "settings-storage", func() {
			a.settingsItemRow(c, "cache-row", "本地媒体缓存", "清除本地已解密的视频切片缓存与封面海报格式转换临时文件", formatCacheSize(state.CacheSize), "清理缓存", "subtle", a.ClearSettingsCache)
			ui.Box(c.Key("storage-divider")).Height(1).Grow(1).Background(c.Theme().Border)
			a.settingsItemRow(c, "device-row", "匿名设备指纹", "当匿名模式触发官方频控限制或需要重置个性化推荐流时，重新生成设备指纹", state.DeviceID, "重置设备", "danger", a.ResetSettingsDevice)
		})

		ui.Text(c, "关于").FontSize(12).Bold().TextColor(c.Theme().TextMuted)
		a.settingsCard(c, "settings-about", func() {
			a.settingsInfoRow(c, "version-row", "客户端版本", state.DeviceVersion)
			ui.Box(c.Key("version-divider")).Height(1).Grow(1).Background(c.Theme().Border)
			a.settingsInfoRow(c, "arch-row", "运行平台", state.DeviceArch)
		})

		if state.Loading {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Padding(8).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在加载设置...").TextColor(c.Theme().TextMuted)
			})
		}
	})
}

func settingsAccountID(state Settings) string {
	if state.Profile.UserID != "" {
		return state.Profile.UserID
	}
	if state.Profile.LoggedIn {
		return "已同步"
	}
	return "anonymous"
}

func (a *App) settingsInfoRow(c *ui.Context, key, label, value string) {
	ui.Row(c.Key(key)).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted).Width(88)
		ui.Text(c, value).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.82)).SingleLine()
	})
}

func (a *App) renderSettingsLogin(c *ui.Context, state Settings) {
	ui.Box(c.Key("login-divider")).Height(1).Grow(1).Background(c.Theme().Border)
	ui.Column(c.Key("settings-login")).Gap(14).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, "账号凭据登录").FontSize(14).Bold().TextColor(ui.Hex("#ffffff"))
			ui.Text(c, "通过 Cookie / Token 同步红果/番茄短剧账户数据").FontSize(12).TextColor(c.Theme().TextMuted)
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Cookie (必填)").FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.80))
			})
			cookie := state.LoginCookie
			ui.TextArea(c, &cookie).Grow(1).Height(82).Padding(10, 12).Radius(8).
				Background(ui.RGBA(24, 25, 36, 255)).Border(1, ui.RGBA(255, 255, 255, 0.09)).
				Placeholder("粘贴完整 Cookie 或包含 cookies 字段的 JSON").Label("Cookie 凭据").
				OnChange(func() { a.SetSettingsCookie(cookie) })
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "x-tt-token (可选)").FontSize(12).TextColor(ui.RGBA(255, 255, 255, 0.80))
			token := state.LoginToken
			ui.TextInput(c, &token).Height(34).Padding(0, 12).Radius(8).
				Background(ui.RGBA(24, 25, 36, 255)).Border(1, ui.RGBA(255, 255, 255, 0.09)).
				Placeholder("输入长效会话 Token").Label("Token 凭据").
				OnChange(func() { a.SetSettingsToken(token) })
		})
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "凭据将通过官方接口验证并保存到本地。").FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
			ui.Row(c).Gap(10).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				a.settingsButton(c, "cancel-login", "收起", "subtle", func() { a.SetSettingsLoginOpen(false) })
				a.settingsButton(c, "submit-login", "验证并登录", "primary", a.SubmitSettingsLogin)
			})
		})
	})
}
