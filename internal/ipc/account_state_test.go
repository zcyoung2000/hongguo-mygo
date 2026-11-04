package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func accountTestService(t *testing.T, appDir, home string) (*HongguoService, *Store) {
	t.Helper()
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(appDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := newHongguoService(nil, store, nil, appDir, home)
	t.Cleanup(func() { CloseService(service) })
	service.httpClient = &http.Client{Transport: ipcHandlerTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"user_id":"710000000000099","user_name":"Test account"}}`))
	})}}
	return service, store
}

func TestLogoutRestartDoesNotImportLegacyCredentials(t *testing.T) {
	home := t.TempDir()
	legacyPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"cookies":"sessionid=legacy","profile":{"userId":"710000000000088","nickname":"Legacy","loggedIn":true}}`)
	if err := os.WriteFile(legacyPath, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	appDir := t.TempDir()
	service, store := accountTestService(t, appDir, home)
	if cookies, _ := service.authCredentials(); cookies != "" {
		t.Fatal("startup imported credentials without an explicit import")
	}
	if _, err := service.LoginWithCookies(context.Background(), "sessionid=current", "token"); err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted := newHongguoService(nil, store, nil, appDir, home)
	t.Cleanup(func() { CloseService(restarted) })
	if cookies, _ := restarted.authCredentials(); cookies != "" || store.GetProfile().LoggedIn {
		t.Fatal("explicit logout did not survive restart with a legacy account present")
	}
	if data, err := os.ReadFile(legacyPath); err != nil || !bytes.Equal(data, legacy) {
		t.Fatal("logout changed the other app's account")
	}
}

func TestLoginPersistenceFailureDoesNotPublishSession(t *testing.T) {
	appDir := t.TempDir()
	service, store := accountTestService(t, appDir, t.TempDir())
	if err := os.Mkdir(filepath.Join(appDir, "account.json"), 0755); err != nil {
		t.Fatal(err)
	}
	profile, err := service.LoginWithCookies(context.Background(), "sessionid=new", "token")
	if err == nil || profile != nil {
		t.Fatalf("failed persistence reported success: profile=%v err=%v", profile, err)
	}
	if cookies, _ := service.authCredentials(); cookies != "" || store.GetProfile().LoggedIn {
		t.Fatal("failed persistence changed the active account")
	}
}

func TestLoginDatabaseFailureRestoresPreviousCredentials(t *testing.T) {
	appDir := t.TempDir()
	service, store := accountTestService(t, appDir, t.TempDir())
	previous, err := json.Marshal(savedAccount{Cookies: "sessionid=previous", Profile: UserProfile{UserID: "710000000000001", Nickname: "Previous", LoggedIn: true}})
	if err != nil {
		t.Fatal(err)
	}
	accountPath := filepath.Join(appDir, "account.json")
	if err := writeAccountFile(accountPath, previous); err != nil {
		t.Fatal(err)
	}
	if err := service.restoreSavedAccount(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if profile, err := service.LoginWithCookies(context.Background(), "sessionid=new", "token"); err == nil || profile != nil {
		t.Fatal("database failure reported login success")
	}
	if data, err := os.ReadFile(accountPath); err != nil || !bytes.Equal(data, previous) {
		t.Fatal("failed login replaced the previously saved credentials")
	}
	if cookies, _ := service.authCredentials(); cookies != "sessionid=previous" {
		t.Fatal("failed login replaced the in-memory session")
	}
}

func TestSavedAccountRestoresAndUsesPrivateFilePermissions(t *testing.T) {
	appDir := t.TempDir()
	home := t.TempDir()
	service, store := accountTestService(t, appDir, home)
	if _, err := service.LoginWithCookies(context.Background(), "sessionid=current", "token"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(appDir, "account.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("credential file permissions = %v, %v", info, err)
	}
	restarted := newHongguoService(nil, store, nil, appDir, home)
	t.Cleanup(func() { CloseService(restarted) })
	if cookies, token := restarted.authCredentials(); cookies != "sessionid=current" || token != "token" || !store.GetProfile().LoggedIn {
		t.Fatal("saved session was not restored")
	}
}

func TestMissingCredentialsClearStaleLoggedInProfile(t *testing.T) {
	service, store := accountTestService(t, t.TempDir(), t.TempDir())
	if err := store.UpdateProfile(UserProfile{UserID: "710000000000001", Nickname: "Previous", DeviceID: "device", LoggedIn: true}); err != nil {
		t.Fatal(err)
	}
	if err := service.restoreSavedAccount(); err != nil {
		t.Fatal(err)
	}
	profile := store.GetProfile()
	if profile.LoggedIn || profile.UserID != "anonymous" || profile.DeviceID != "device" {
		t.Fatalf("stale profile survived: %+v", profile)
	}
}

func TestSeriesPageRefreshesUserFlagsWithoutMutatingCache(t *testing.T) {
	service, store := accountTestService(t, t.TempDir(), t.TempDir())
	const id = "710000000000001"
	cached := &SeriesDetailPageDTO{SeriesDetailDTO: SeriesDetailDTO{ID: id, Title: "Catalog metadata"}}
	service.cache["series_page:"+id] = cacheItem{val: cached, expiresAt: time.Now().Add(5 * time.Minute)}
	if _, err := service.ToggleFavorite(context.Background(), id, "Catalog metadata", "", "", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ToggleReservation(ReservationItem{SeriesID: id, Title: "Catalog metadata"}); err != nil {
		t.Fatal(err)
	}
	page, err := service.GetSeriesPage(context.Background(), id)
	if err != nil || !page.IsFavorite || !page.Reserved || page.Title != cached.Title {
		t.Fatalf("stale user flags: %+v, %v", page, err)
	}
	if cached.IsFavorite || cached.Reserved || page == cached {
		t.Fatal("response mutated the shared catalog cache")
	}
	if _, err := service.ToggleFavorite(context.Background(), id, "Catalog metadata", "", "", 10); err != nil {
		t.Fatal(err)
	}
	page, err = service.GetSeriesPage(context.Background(), id)
	if err != nil || page.IsFavorite {
		t.Fatal("unfavorite did not refresh the cached detail response")
	}
}
