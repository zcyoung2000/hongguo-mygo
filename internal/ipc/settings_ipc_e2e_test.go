package ipc

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	userstore "myguo/internal/store"
)

func TestSettingsIPCReportsStorageDeviceAndLegacyAccountWithoutRuntimeWiring(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	appDir := t.TempDir()
	home := t.TempDir()
	legacy := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"cookies":"sessionid=old","profile":{"userId":"710000000000001","nickname":"Old Account"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	media := newTestMediaServer(t, cacheDir)
	cacheDir = media.CacheDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "episode.bin"), []byte(strings.Repeat("media", 1024)), 0600); err != nil {
		t.Fatal(err)
	}
	store, storeErr := userstore.NewStore(appDir)
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	service := newHongguoService(nil, store, media, appDir, home)
	t.Cleanup(func() { CloseService(service) })
	service.httpClient = &http.Client{Transport: ipcHandlerTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "user/info") {
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_id":"710000000000001","user_name":"Old Account"}}`))
			return
		}
		http.NotFound(w, r)
	})}}

	size, err := service.GetCacheSize(context.Background())
	if err != nil || size != 5120 {
		t.Fatalf("GetCacheSize = %d, %v; want 5120", size, err)
	}
	device, err := service.GetDeviceInfo(context.Background())
	if err != nil || device["deviceId"] == "" || device["version"] != "2.0.0" || device["arch"] == "" {
		t.Fatalf("GetDeviceInfo = %+v, %v", device, err)
	}
	if _, statErr := os.Stat(legacy); statErr != nil {
		t.Fatalf("legacy fixture disappeared: %v, home=%s", statErr, home)
	}
	t.Setenv("HOME", home)
	existing, err := service.CheckExistingAccount(context.Background())
	if err != nil || existing["exists"] != true || existing["nickname"] != "Old Account" {
		t.Fatalf("CheckExistingAccount = %+v, %v", existing, err)
	}
	if _, err := service.ImportHgplayerAccount(context.Background()); err != nil {
		t.Fatalf("ImportHgplayerAccount failed: %v", err)
	}
	profile, err := service.GetProfile(context.Background())
	if err != nil || !profile.LoggedIn || profile.Nickname != "Old Account" || profile.UserID != "710000000000001" {
		t.Fatalf("profile after import = %+v, %v", profile, err)
	}
	cleared, err := service.ClearCache(context.Background())
	if err != nil || cleared != 5120 {
		t.Fatalf("ClearCache = %d, %v", cleared, err)
	}
	if remaining := media.GetCacheSize(); remaining != 0 {
		t.Fatalf("cache remaining = %d", remaining)
	}
	if _, ok := service.store.GetAccountAuth(); !ok {
		t.Fatal("ClearCache must not clear account credentials")
	}
}

func TestSettingsIPCImportRejectsInvalidLegacyData(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	home := t.TempDir()
	legacy := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"cookies":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, storeErr := userstore.NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	service := newHongguoService(nil, store, newTestMediaServer(t, t.TempDir()), t.TempDir(), home)
	t.Cleanup(func() { CloseService(service) })
	t.Setenv("HOME", home)
	if _, err := service.ImportHgplayerAccount(context.Background()); err == nil {
		t.Fatal("invalid legacy account was accepted")
	}
	if existing, err := service.CheckExistingAccount(context.Background()); err != nil || existing["exists"] != false {
		t.Fatalf("invalid legacy account visible = %+v, %v", existing, err)
	}
}

func TestSettingsIPCLoginValidationKeepsUnauthenticatedProfile(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"user_id":"0","user_name":"anonymous"}}`))
	})
	store, storeErr := userstore.NewStore(t.TempDir())
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	service := newHongguoService(nil, store, nil, t.TempDir(), t.TempDir())
	t.Cleanup(func() { CloseService(service) })
	service.httpClient = &http.Client{Transport: ipcHandlerTransport{handler: handler}}
	if _, err := service.LoginWithCookies(context.Background(), "sessionid=test", ""); err == nil {
		t.Fatal("anonymous user info was accepted as login")
	}
	profile, err := service.GetProfile(context.Background())
	if err != nil || profile.LoggedIn || profile.UserID != "anonymous" {
		t.Fatalf("failed login changed visible profile: %+v, %v", profile, err)
	}
	if _, err := service.RefreshUserProfile(context.Background()); err == nil {
		t.Fatal("refresh succeeded without login")
	}
}

func newTestMediaServer(t *testing.T, cacheDir string) *MediaServer {
	t.Helper()
	oldHome, hadHome := os.LookupEnv("HOME")
	t.Setenv("HOME", cacheDir)
	ms, err := NewMediaServer(nil)
	if hadHome {
		t.Setenv("HOME", oldHome)
	} else {
		os.Unsetenv("HOME")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.Close)
	return ms
}
