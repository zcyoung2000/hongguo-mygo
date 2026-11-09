package uiapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"myguo/internal/ipc"
)

type mutatingSettingsProvider struct {
	settingsTestProvider
	mu           sync.Mutex
	loginStarted chan struct{}
	loginError   error
	clearStarted chan struct{}
	clearError   error
}

func (f *mutatingSettingsProvider) LoginWithCookies(_ context.Context, cookie, token string) (*ipc.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loginStarted != nil {
		close(f.loginStarted)
	}
	f.settingsTestProvider.LoginWithCookies(context.Background(), cookie, token)
	if f.loginError != nil {
		return nil, f.loginError
	}
	f.profile = ipc.UserProfile{UserID: "42", Nickname: "New User", LoggedIn: true}
	return &f.profile, nil
}

func (f *mutatingSettingsProvider) ClearCache(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.clearStarted != nil {
		close(f.clearStarted)
	}
	cleared, err := f.settingsTestProvider.ClearCache(context.Background())
	if err != nil {
		return 0, err
	}
	return cleared, f.clearError
}

func (f *mutatingSettingsProvider) GetProfile(ctx context.Context) (ipc.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settingsTestProvider.GetProfile(ctx)
}

func (f *mutatingSettingsProvider) GetCacheSize(ctx context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settingsTestProvider.GetCacheSize(ctx)
}

func (f *mutatingSettingsProvider) GetDeviceInfo(ctx context.Context) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settingsTestProvider.GetDeviceInfo(ctx)
}

func waitForSettingsMutation(t *testing.T, app *App) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		app.mu.Lock()
		busy := len(app.settingsMutations) != 0 || app.settings.Loading
		app.mu.Unlock()
		if !busy {
			return
		}
		select {
		case <-deadline:
			t.Fatal("settings mutation did not finish")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestSettingsMutationSuccessShowsMessageAndRefreshesState(t *testing.T) {
	app := NewTestApp()
	fake := &mutatingSettingsProvider{loginStarted: make(chan struct{})}
	fake.profile = ipc.UserProfile{UserID: "42", Nickname: "New User", LoggedIn: true}
	app.settingsTestProvider = fake
	app.settings = Settings{
		Profile:     ipc.UserProfile{UserID: "old", Nickname: "Old User", LoggedIn: true},
		CacheSize:   5 << 20,
		Error:       "旧错误",
		LoginOpen:   true,
		LoginCookie: "sessionid=test",
	}
	app.SubmitSettingsLogin()
	select {
	case <-fake.loginStarted:
	case <-time.After(time.Second):
		t.Fatal("settings login action did not start")
	}
	waitForSettingsMutation(t, app)

	state := app.SettingsState()
	if !state.Profile.LoggedIn || state.Profile.UserID != "42" || state.Profile.Nickname != "New User" {
		t.Fatalf("login did not refresh profile: %+v", state.Profile)
	}
	if state.CacheSize != 0 {
		t.Fatalf("login did not refresh cache size: %d", state.CacheSize)
	}
	if state.Error != "" {
		t.Fatalf("old settings error survived success: %q", state.Error)
	}
	if state.Success != "登录成功！欢迎回来，New User" {
		t.Fatalf("settings success message missing: %q", state.Success)
	}

	fake.cache = 5 << 20
	fake.clearStarted = make(chan struct{})
	app.ClearSettingsCache()
	select {
	case <-fake.clearStarted:
	case <-time.After(time.Second):
		t.Fatal("settings clear-cache action did not start")
	}
	waitForSettingsMutation(t, app)
	state = app.SettingsState()
	if state.CacheSize != 0 || state.Success != "已清理 2.0 MB 本地媒体缓存" {
		t.Fatalf("cache state/message not refreshed: size=%d success=%q", state.CacheSize, state.Success)
	}
}

func TestSettingsMutationFailureKeepsError(t *testing.T) {
	app := NewTestApp()
	fake := &mutatingSettingsProvider{clearStarted: make(chan struct{}), clearError: errors.New("cache unavailable")}
	fake.profile = ipc.UserProfile{UserID: "42", Nickname: "Henry", LoggedIn: true}
	app.settingsTestProvider = fake
	app.settings = Settings{Profile: fake.profile, CacheSize: 5 << 20}

	app.ClearSettingsCache()
	select {
	case <-fake.clearStarted:
	case <-time.After(time.Second):
		t.Fatal("settings clear-cache action did not start")
	}
	deadline := time.After(time.Second)
	for app.SettingsState().Error == "" {
		select {
		case <-deadline:
			t.Fatal("settings mutation error missing")
		case <-time.After(time.Millisecond):
		}
	}

	state := app.SettingsState()
	if state.CacheSize != 5<<20 {
		t.Fatalf("failed mutation changed cache size: %d", state.CacheSize)
	}
	if state.Success != "" || state.Error != "设置加载失败：cache unavailable" {
		t.Fatalf("failed mutation state = success=%q error=%q", state.Success, state.Error)
	}
}
