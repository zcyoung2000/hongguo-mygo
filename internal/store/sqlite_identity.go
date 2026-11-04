package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func anonymousUserProfile(deviceID string) UserProfile {
	return UserProfile{UserID: "anonymous", Nickname: "myguo用户", DeviceID: deviceID}
}

func clearUserDataTx(tx *sql.Tx) error {
	for _, table := range []string{
		"history", "favorites", "likes", "reservations",
		"history_tombstones", "history_clear_state",
	} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	return nil
}

type SQLiteProfileStore struct {
	store     *Store
	savedHook func(*sdk.DeviceProfile)
}

// ProfileStore adapts the SQLite identity tables to the SDK interface.
func (s *Store) ProfileStore() *SQLiteProfileStore { return &SQLiteProfileStore{store: s} }

// SetSavedHook lets the desktop service refresh its in-memory identity after
// the SDK registers or updates the SQLite-backed profile.
func (p *SQLiteProfileStore) SetSavedHook(hook func(*sdk.DeviceProfile)) {
	p.savedHook = hook
}

func (p *SQLiteProfileStore) Load(ctx context.Context) (*sdk.DeviceProfile, error) {
	return p.store.LoadDeviceProfile(ctx)
}

func (p *SQLiteProfileStore) Save(ctx context.Context, profile *sdk.DeviceProfile) error {
	if err := p.store.SaveDeviceProfile(ctx, profile); err != nil {
		return err
	}
	if p.savedHook != nil {
		p.savedHook(profile)
	}
	return nil
}

func (p *SQLiteProfileStore) Reset(ctx context.Context) error {
	return p.store.ResetDeviceProfile(ctx)
}

func (s *Store) GetAccountAuth() (AccountAuth, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var auth AccountAuth
	err := s.db.QueryRow(`SELECT cookies, token FROM account_auth WHERE id = 1`).
		Scan(&auth.Cookies, &auth.Token)
	if err != nil || auth.Cookies == "" {
		return AccountAuth{}, false
	}
	return auth, true
}

// SetAuthenticatedAccount publishes credentials, profile, and (for an identity
// change) visible data removal in one transaction. A failure cannot leave a
// new credential beside another account's visible rows.
func (s *Store) SetAuthenticatedAccount(auth AccountAuth, p UserProfile, resetVisibleData bool) error {
	if auth.Cookies == "" {
		return fmt.Errorf("登录凭据不能为空")
	}
	if p.UserID == "" || p.UserID == "anonymous" {
		return fmt.Errorf("登录账号 ID 无效")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()

	if resetVisibleData {
		if err := clearUserDataTx(tx); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO account_auth (id, cookies, token, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			cookies = excluded.cookies,
			token = excluded.token,
			updated_at = excluded.updated_at
	`, auth.Cookies, auth.Token, now); err != nil {
		return err
	}
	if err := updateProfileTx(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

// SignOut atomically forgets credentials and account-visible rows. Scoped sync
// queues are intentionally retained for the original account.
func (s *Store) SignOut(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM account_auth WHERE id = 1`); err != nil {
		return err
	}
	if err := clearUserDataTx(tx); err != nil {
		return err
	}
	if err := updateProfileTx(tx, anonymousUserProfile(deviceID)); err != nil {
		return err
	}
	return tx.Commit()
}

func updateProfileTx(tx *sql.Tx, p UserProfile) error {
	loggedIn := 0
	if p.LoggedIn {
		loggedIn = 1
	}
	_, err := tx.Exec(`
		INSERT INTO profile (id, user_id, nickname, avatar, device_id, logged_in)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			nickname = excluded.nickname,
			avatar = excluded.avatar,
			device_id = excluded.device_id,
			logged_in = excluded.logged_in
	`, p.UserID, p.Nickname, p.Avatar, p.DeviceID, loggedIn)
	return err
}

func (s *Store) LoadDeviceProfile(context.Context) (*sdk.DeviceProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		p       sdk.DeviceProfile
		created int64
		d       model.Device
		pad     int
	)
	err := s.db.QueryRow(`
		SELECT cdid, openudid, clientudid, oaid, reqid,
		       device_id, install_id, device_token,
		       ttreq, store_region, store_region_src, created_at,
		       device_brand, device_model, device_os_version, device_os_api,
		       device_build, device_resolution, device_dpi, device_pad, device_channel
		FROM device_profiles WHERE id = 1
	`).Scan(
		&p.CDID, &p.OpenUDID, &p.ClientUDID, &p.OAID, &p.ReqID,
		&p.DeviceID, &p.InstallID, &p.DeviceToken,
		&p.TTReq, &p.StoreRegion, &p.StoreRegionSrc, &created,
		&d.Brand, &d.Model, &d.OSVersion, &d.OSAPI,
		&d.Build, &d.Resolution, &d.DPI, &pad, &d.Channel,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sdk.ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	if created > 0 {
		p.CreatedAt = time.Unix(created, 0).UTC()
	}
	if d.Brand != "" || d.Model != "" || d.OSVersion != "" || d.OSAPI != 0 ||
		d.Build != "" || d.Resolution != "" || d.DPI != 0 || d.Channel != "" || pad != 0 {
		d.Pad = pad != 0
		p.Device = &d
	}
	return &p, nil
}

func (s *Store) SaveDeviceProfile(_ context.Context, profile *sdk.DeviceProfile) error {
	if profile == nil {
		return sdk.ErrInvalidOptions
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	createdAt := profile.CreatedAt.Unix()
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}
	var (
		d   model.Device
		pad int
	)
	if profile.Device != nil {
		d = *profile.Device
		if d.Pad {
			pad = 1
		}
	}
	_, err := s.db.Exec(`
		INSERT INTO device_profiles (
			id, cdid, openudid, clientudid, oaid, reqid,
			device_id, install_id, device_token,
			ttreq, store_region, store_region_src, created_at,
			device_brand, device_model, device_os_version, device_os_api,
			device_build, device_resolution, device_dpi, device_pad, device_channel
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			cdid = excluded.cdid,
			openudid = excluded.openudid,
			clientudid = excluded.clientudid,
			oaid = excluded.oaid,
			reqid = excluded.reqid,
			device_id = excluded.device_id,
			install_id = excluded.install_id,
			device_token = excluded.device_token,
			ttreq = excluded.ttreq,
			store_region = excluded.store_region,
			store_region_src = excluded.store_region_src,
			created_at = excluded.created_at,
			device_brand = excluded.device_brand,
			device_model = excluded.device_model,
			device_os_version = excluded.device_os_version,
			device_os_api = excluded.device_os_api,
			device_build = excluded.device_build,
			device_resolution = excluded.device_resolution,
			device_dpi = excluded.device_dpi,
			device_pad = excluded.device_pad,
			device_channel = excluded.device_channel
	`, profile.CDID, profile.OpenUDID, profile.ClientUDID, profile.OAID, profile.ReqID,
		profile.DeviceID, profile.InstallID, profile.DeviceToken,
		profile.TTReq, profile.StoreRegion, profile.StoreRegionSrc, createdAt,
		d.Brand, d.Model, d.OSVersion, d.OSAPI,
		d.Build, d.Resolution, d.DPI, pad, d.Channel)
	return err
}

func (s *Store) ResetDeviceProfile(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM device_profiles WHERE id = 1`)
	return err
}

func (s *Store) GetDeviceRuntime() (userAgent, storeRegion string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	err := s.db.QueryRow(`
		SELECT user_agent, store_region FROM device_runtime WHERE id = 1
	`).Scan(&userAgent, &storeRegion)
	return userAgent, storeRegion, err == nil && (userAgent != "" || storeRegion != "")
}

func (s *Store) SaveDeviceRuntime(userAgent, storeRegion string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO device_runtime (id, user_agent, store_region, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_agent = excluded.user_agent,
			store_region = excluded.store_region,
			updated_at = excluded.updated_at
	`, userAgent, storeRegion, time.Now().Unix())
	return err
}

// migrateOwnJSONFiles is a one-time import from releases that kept identity in
// JSON files. New data is only ever written to SQLite.
func (s *Store) migrateOwnJSONFiles() error {
	// Migrate the device first so an account profile with an empty DeviceID
	// can still inherit the restored anonymous identity.
	if err := s.migrateOwnDeviceJSON(); err != nil {
		return err
	}
	if err := s.migrateOwnRuntimeJSON(); err != nil {
		return err
	}
	return s.migrateOwnAccountJSON()
}

func (s *Store) migrateOwnAccountJSON() error {
	path := filepath.Join(s.dir(), "account.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if _, ok := s.GetAccountAuth(); ok {
		return removeMigratedJSON(path)
	}
	var legacy struct {
		Cookies string      `json:"cookies"`
		Token   string      `json:"token"`
		Profile UserProfile `json:"profile"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil || legacy.Cookies == "" ||
		legacy.Profile.UserID == "" || legacy.Profile.UserID == "anonymous" {
		log.Printf("[account] legacy account.json was invalid; removing it")
		return retireInvalidJSON(path)
	}
	if legacy.Profile.DeviceID == "" {
		legacy.Profile.DeviceID = s.GetProfile().DeviceID
	}
	legacy.Profile.LoggedIn = true
	if legacy.Profile.Nickname == "" {
		legacy.Profile.Nickname = "myguo用户"
	}
	if err := s.SetAuthenticatedAccount(AccountAuth{
		Cookies: legacy.Cookies,
		Token:   legacy.Token,
	}, legacy.Profile, false); err != nil {
		return err
	}
	log.Printf("[account] migrated account credentials into SQLite")
	return removeMigratedJSON(path)
}

func (s *Store) migrateOwnDeviceJSON() error {
	path := filepath.Join(s.dir(), "anonymous.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if _, err := s.LoadDeviceProfile(context.Background()); err == nil {
		return removeMigratedJSON(path)
	} else if !errors.Is(err, sdk.ErrProfileNotFound) {
		return err
	}
	var profile sdk.DeviceProfile
	if err := json.Unmarshal(data, &profile); err != nil || profile.CDID == "" {
		log.Printf("[device] legacy anonymous.json was invalid; removing it")
		return retireInvalidJSON(path)
	}
	if err := s.SaveDeviceProfile(context.Background(), &profile); err != nil {
		return err
	}
	log.Printf("[device] migrated anonymous profile into SQLite")
	return removeMigratedJSON(path)
}

func (s *Store) migrateOwnRuntimeJSON() error {
	path := filepath.Join(s.dir(), "device.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if _, _, ok := s.GetDeviceRuntime(); ok {
		return removeMigratedJSON(path)
	}
	var legacy struct {
		UserAgent   string `json:"user_agent"`
		StoreRegion string `json:"store_region"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil ||
		(legacy.UserAgent == "" && legacy.StoreRegion == "") {
		log.Printf("[device] legacy device.json was invalid; removing it")
		return retireInvalidJSON(path)
	}
	if err := s.SaveDeviceRuntime(legacy.UserAgent, legacy.StoreRegion); err != nil {
		return err
	}
	return removeMigratedJSON(path)
}

func (s *Store) dir() string { return filepath.Dir(s.dbPath) }

func removeMigratedJSON(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove migrated %s failed: %w", filepath.Base(path), err)
	}
	return nil
}

func retireInvalidJSON(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove invalid %s failed: %w", filepath.Base(path), err)
	}
	return nil
}
