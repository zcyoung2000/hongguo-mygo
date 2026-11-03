package ipc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	userstore "myguo/internal/store"
)

type ipcHandlerTransport struct{ handler http.Handler }

func (transport ipcHandlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

func TestLoginAndLogoutPersistAndClearProfile(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()
	appDir := t.TempDir()
	var sawCookie string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "user/info") {
			sawCookie = r.Header.Get("Cookie")
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_name":"Henry","user_id":"710000000000001"}}`))
			return
		}
		http.NotFound(w, r)
	})
	service := NewHongguoService(nil, store, nil, appDir)
	service.httpClient = &http.Client{Transport: ipcHandlerTransport{handler: handler}}

	profile, err := service.LoginWithCookies(context.Background(), "sessionid=account", "token")
	if err != nil {
		t.Fatalf("LoginWithCookies() error = %v", err)
	}
	if profile.Nickname != "Henry" || profile.UserID != "710000000000001" || !profile.LoggedIn {
		t.Fatalf("profile = %+v", profile)
	}
	if sawCookie != "sessionid=account" {
		t.Fatalf("login cookie = %q", sawCookie)
	}
	if cookies, _ := service.authCredentials(); cookies != "sessionid=account" {
		t.Fatalf("cached cookies = %q", cookies)
	}
	accountPath := filepath.Join(appDir, "account.json")
	if _, err := os.Stat(accountPath); err != nil {
		t.Fatalf("account.json was not persisted: %v", err)
	}

	if err := service.Logout(context.Background()); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := os.Stat(accountPath); !os.IsNotExist(err) {
		t.Fatalf("account.json still exists, err = %v", err)
	}
	if cookies, _ := service.authCredentials(); cookies != "" {
		t.Fatalf("cookies after logout = %q", cookies)
	}
	afterProfile := store.GetProfile()
	if afterProfile.LoggedIn || afterProfile.UserID != "anonymous" {
		t.Fatalf("profile after logout = %+v", afterProfile)
	}
}
