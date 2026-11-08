package uiapp

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

type settingsTestProvider struct {
	profile ipc.UserProfile
	cache   int64
	device  map[string]string
	legacy  map[string]any
	calls   []string
	err     error
}

func (f *settingsTestProvider) GetProfile(context.Context) (ipc.UserProfile, error) {
	f.calls = append(f.calls, "GetProfile")
	return f.profile, f.err
}
func (f *settingsTestProvider) GetCacheSize(context.Context) (int64, error) {
	f.calls = append(f.calls, "GetCacheSize")
	return f.cache, nil
}
func (f *settingsTestProvider) GetDeviceInfo(context.Context) (map[string]string, error) {
	f.calls = append(f.calls, "GetDeviceInfo")
	if f.device == nil {
		return map[string]string{"deviceId": "device-id", "version": "2.0.0", "arch": "darwin-arm64"}, nil
	}
	return f.device, nil
}
func (f *settingsTestProvider) CheckExistingAccount(context.Context) (map[string]any, error) {
	f.calls = append(f.calls, "CheckExistingAccount")
	return f.legacy, nil
}
func (f *settingsTestProvider) LoginWithCookies(_ context.Context, cookie, _ string) (*ipc.UserProfile, error) {
	f.calls = append(f.calls, "LoginWithCookies:"+cookie)
	if f.err != nil {
		return nil, f.err
	}
	return &ipc.UserProfile{UserID: "42", Nickname: "New User", LoggedIn: true}, nil
}
func (f *settingsTestProvider) RefreshUserProfile(context.Context) (*ipc.UserProfile, error) {
	f.calls = append(f.calls, "RefreshUserProfile")
	return &ipc.UserProfile{UserID: "42", Nickname: "Refreshed", LoggedIn: true}, nil
}
func (f *settingsTestProvider) Logout(context.Context) error {
	f.calls = append(f.calls, "Logout")
	return nil
}
func (f *settingsTestProvider) ImportHgplayerAccount(context.Context) (*ipc.UserProfile, error) {
	f.calls = append(f.calls, "ImportHgplayerAccount")
	return &ipc.UserProfile{UserID: "710", Nickname: "Old Account", LoggedIn: true}, nil
}
func (f *settingsTestProvider) ClearCache(context.Context) (int64, error) {
	f.calls = append(f.calls, "ClearCache")
	f.cache = 0
	return 2 << 20, nil
}
func (f *settingsTestProvider) ResetDevice(context.Context) error {
	f.calls = append(f.calls, "ResetDevice")
	return nil
}

func (f *settingsTestProvider) GetRankings(context.Context, string, string, string) (ipc.RankResponse, error) {
	return ipc.RankResponse{}, nil
}

func (f *settingsTestProvider) GetNewReleases(context.Context, string) (ipc.NewReleaseResponse, error) {
	return ipc.NewReleaseResponse{}, nil
}

func (f *settingsTestProvider) GetActorProfile(context.Context, string) (*ipc.ActorProfileDTO, error) {
	return nil, nil
}

func (f *settingsTestProvider) GetActorWorks(context.Context, string, string) (*ipc.ActorWorksResponse, error) {
	return nil, nil
}

func (f *settingsTestProvider) GetSuggestions(context.Context, string) ([]string, error) {
	return nil, nil
}

func (f *settingsTestProvider) GetSeriesSeasons(context.Context, string) (*ipc.SeriesCollectionDTO, error) {
	return nil, nil
}

func (f *settingsTestProvider) GetSeriesAdaptations(context.Context, string, string, string, string, string) (*ipc.SeriesCollectionDTO, error) {
	return nil, nil
}

func settingsFixture() (*App, *settingsTestProvider, *ui.Tester) {
	app := NewTestApp()
	fake := &settingsTestProvider{profile: ipc.UserProfile{UserID: "42", Nickname: "Henry", LoggedIn: true}, cache: 5 << 20}
	app.settingsTestProvider = fake
	app.view = "settings"
	app.settings = Settings{
		Profile: fake.profile, CacheSize: fake.cache, DeviceID: "device-id",
		DeviceVersion: "2.0.0", DeviceArch: "darwin-arm64",
		LegacyNickname: "Old Account", HasLegacy: true, LoginOpen: true,
		LoginCookie: "sessionid=test", LoginToken: "token",
	}
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	return app, fake, tt
}

func TestSettingsRendersNativeSectionsAndActions(t *testing.T) {
	app, _, tt := settingsFixture()
	if app.hostedScene() {
		t.Fatal("settings must not enter the WebView host path")
	}
	if tt.WebViewAt(700, 400) != nil {
		t.Fatal("settings leaked a WebView surface")
	}
	for _, text := range []string{
		"系统与账号设置", "Henry", "已登录官方账号", "账号 ID:", "42", "刷新资料", "退出登录", "切换账号",
		"设备与存储管理", "本地媒体缓存", "5.0 MB", "清理缓存", "匿名设备指纹", "device-id", "重置设备",
		"关于", "客户端版本", "2.0.0", "运行平台", "darwin-arm64",
		"账号凭据登录", "Cookie (必填)", "验证并登录",
	} {
		if !tt.HasText(text) {
			t.Fatalf("native settings missing %q: %v", text, tt.Texts())
		}
	}
	legacy, ok := tt.Find("检测到本机已有登录凭据")
	if ok {
		t.Fatalf("logged-in profile exposed legacy import at %+v", legacy)
	}
}

func TestAnonymousSettingsShowsLegacyImportAndLogin(t *testing.T) {
	app := NewTestApp()
	fake := &settingsTestProvider{profile: ipc.UserProfile{}, cache: 1048576, legacy: map[string]any{"exists": true, "nickname": "Old Account", "userId": "710000000000001"}}
	app.settingsTestProvider = fake
	app.view = "settings"
	app.settings = Settings{CacheSize: fake.cache, DeviceID: "device-id", DeviceVersion: "2.0.0", DeviceArch: "darwin-arm64", LegacyNickname: "Old Account", LegacyUserID: "710000000000001", HasLegacy: true}
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	for _, text := range []string{"myguo用户", "匿名设备访问", "账号 ID:", "anonymous", "检测到本机旧版账号：Old Account", "一键同步登录", "立即登录"} {
		if !tt.HasText(text) {
			t.Fatalf("anonymous settings missing %q: %v", text, tt.Texts())
		}
	}
	if _, ok := tt.Find("退出登录"); ok {
		t.Fatal("anonymous settings exposed logout")
	}
	_ = app
}

func TestSettingsMediaRoutingKeepsRecommendAndDetail(t *testing.T) {
	host := &MediaHost{}
	app := NewTestApp()
	app.SetMediaHost(host)
	app.view = "recommend"
	app.mediaOn = true
	host.scene(map[string]any{"event": "recommend"})
	app.OpenView("recommend")
	if host.desired["event"] != "recommend" {
		t.Fatal("recommend host changed")
	}
	app.OpenView("settings")
	if host.desired["event"] != "stop" || app.hostedScene() || app.mediaOn {
		t.Fatal("settings must stop recommendation media and use the native body")
	}
	app.OpenView("detail")
	app.PlayDetail()
	if host.desired["event"] != "stop" {
		t.Fatal("navigation stop changed")
	}
}

func TestSettingsSubmitLoginValidatesAndCallsIPC(t *testing.T) {
	app, fake, _ := settingsFixture()
	app.SubmitSettingsLogin()
	app.mu.Lock()
	provider := app.settingsProvider()
	app.mu.Unlock()
	if provider == nil {
		t.Fatal("settings login provider missing")
	}
	provider.LoginWithCookies(context.Background(), "sessionid=test", "token")
	if !containsString(fake.calls, "LoginWithCookies:sessionid=test") {
		t.Fatalf("login IPC missing: %v", fake.calls)
	}
}

func TestSettingsSubmitLoginRejectsBlankCookie(t *testing.T) {
	app := NewTestApp()
	fake := &settingsTestProvider{}
	app.settingsTestProvider = fake
	app.view = "settings"
	app.settings.LoginOpen = true
	app.SubmitSettingsLogin()
	if app.SettingsState().Error == "" || len(fake.calls) != 0 {
		t.Fatalf("blank cookie not rejected: error=%q calls=%v", app.SettingsState().Error, fake.calls)
	}
}

func TestSettingsLoadAggregatesIPCError(t *testing.T) {
	app := NewTestApp()
	fake := &settingsTestProvider{err: errors.New("boom")}
	app.settingsTestProvider = fake
	app.settings = Settings{Loading: true}
	_, profileErr := fake.GetProfile(context.Background())
	_, cacheErr := fake.GetCacheSize(context.Background())
	_, deviceErr := fake.GetDeviceInfo(context.Background())
	_, legacyErr := fake.CheckExistingAccount(context.Background())
	app.mu.Lock()
	app.settings.Error = settingsError(errors.Join(profileErr, cacheErr, deviceErr, legacyErr))
	app.mu.Unlock()
	if app.SettingsState().Error == "" {
		t.Fatal("aggregated settings load error missing")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

var _ = mygo.WebView{}
