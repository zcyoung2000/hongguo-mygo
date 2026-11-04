package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestSQLiteDeviceProfileRoundTrip(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	want, err := sdk.NewAnonymousDeviceProfile(nil, func() time.Time { return time.Unix(1, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	want.DeviceID = "710000000000001"
	want.InstallID = "710000000000002"
	want.StoreRegion = "cn-gd"
	want.Device = &model.DefaultDevice
	if err := store.SaveDeviceProfile(context.Background(), want); err != nil {
		t.Fatalf("SaveDeviceProfile() error = %v", err)
	}

	got, err := store.LoadDeviceProfile(context.Background())
	if err != nil {
		t.Fatalf("LoadDeviceProfile() error = %v", err)
	}
	if got.CDID != want.CDID || got.OpenUDID != want.OpenUDID || got.ClientUDID != want.ClientUDID ||
		got.OAID != want.OAID || got.ReqID != want.ReqID || got.DeviceID != want.DeviceID ||
		got.InstallID != want.InstallID || got.StoreRegion != want.StoreRegion ||
		!got.CreatedAt.Equal(want.CreatedAt) || got.Device.Brand != want.Device.Brand {
		t.Fatalf("loaded profile = %+v, want %+v", got, want)
	}
	if err := store.ResetDeviceProfile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadDeviceProfile(context.Background()); err == nil {
		t.Fatal("reset device profile still loads")
	}
}

func TestSQLiteProfileStoreSavedHookSeesCommittedProfile(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	profileStore := store.ProfileStore()
	hooked := make(chan *sdk.DeviceProfile, 1)
	profileStore.SetSavedHook(func(profile *sdk.DeviceProfile) {
		hooked <- profile
	})
	want, err := sdk.NewAnonymousDeviceProfile(nil, func() time.Time { return time.Unix(3, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	if err := profileStore.Save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-hooked:
		if got.CDID != want.CDID {
			t.Fatalf("hook profile CDID = %q, want %q", got.CDID, want.CDID)
		}
	case <-time.After(time.Second):
		t.Fatal("saved hook was not called")
	}
	if loaded, err := store.LoadDeviceProfile(context.Background()); err != nil || loaded.CDID != want.CDID {
		t.Fatalf("loaded profile = %+v, %v", loaded, err)
	}
}

func TestAccountAuthTransactionsPreserveScopedQueues(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	profile := UserProfile{UserID: "710000000000001", Nickname: "One", LoggedIn: true, DeviceID: "device"}
	if err := store.SetAuthenticatedAccount(AccountAuth{Cookies: "sessionid=one", Token: "one"}, profile, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveHistory(HistoryItem{SeriesID: "710000000000002", EpisodeNum: 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueHistoryUpload(HistoryItem{SeriesID: "710000000000003", EpisodeNum: 1}, "710000000000103", "user:710000000000001"); err != nil {
		t.Fatal(err)
	}

	if err := store.SetAuthenticatedAccount(AccountAuth{Cookies: "sessionid=two", Token: "two"}, UserProfile{
		UserID: "710000000000004", Nickname: "Two", LoggedIn: true, DeviceID: "device",
	}, true); err != nil {
		t.Fatal(err)
	}
	if items, err := store.GetHistory(); err != nil || len(items) != 0 {
		t.Fatalf("previous visible history survived identity change: %v, %v", items, err)
	}
	auth, ok := store.GetAccountAuth()
	if !ok || auth.Cookies != "sessionid=two" || auth.Token != "two" {
		t.Fatalf("account auth = %+v, ok = %v", auth, ok)
	}
	if pending, err := store.PendingHistoryUploads(10, "user:710000000000001"); err != nil || len(pending) != 1 {
		t.Fatalf("previous scoped queue = %v, %v", pending, err)
	}

	if err := store.SignOut("device"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GetAccountAuth(); ok {
		t.Fatal("sign out retained credentials")
	}
	if got := store.GetProfile(); got.LoggedIn || got.UserID != "anonymous" || got.DeviceID != "device" {
		t.Fatalf("signed-out profile = %+v", got)
	}
}

func TestOwnJSONIdentityFilesMigrateToSQLiteAndAreRemoved(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	device, err := sdk.NewAnonymousDeviceProfile(nil, func() time.Time { return time.Unix(2, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	deviceData, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "anonymous.json"), deviceData, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "device.json"), []byte(`{"user_agent":"Test Agent","store_region":"cn-gd"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "account.json"), []byte(`{
		"cookies":"sessionid=migrated",
		"token":"migrated-token",
		"profile":{"userId":"710000000000005","nickname":"Migrated","loggedIn":true}
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	auth, ok := store.GetAccountAuth()
	if !ok || auth.Cookies != "sessionid=migrated" || auth.Token != "migrated-token" {
		t.Fatalf("migrated auth = %+v, ok = %v", auth, ok)
	}
	if profile := store.GetProfile(); profile.UserID != "710000000000005" || profile.Nickname != "Migrated" {
		t.Fatalf("migrated profile = %+v", profile)
	}
	gotDevice, err := store.LoadDeviceProfile(context.Background())
	if err != nil || gotDevice.CDID != device.CDID {
		t.Fatalf("migrated device = %+v, %v", gotDevice, err)
	}
	if userAgent, region, ok := store.GetDeviceRuntime(); !ok || userAgent != "Test Agent" || region != "cn-gd" {
		t.Fatalf("device runtime = %q, %q, %v", userAgent, region, ok)
	}
	for _, name := range []string{"anonymous.json", "device.json", "account.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists, err = %v", name, err)
		}
	}
}
