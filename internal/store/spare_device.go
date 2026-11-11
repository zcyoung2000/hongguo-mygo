package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNoSpareDevice = sql.ErrNoRows

const (
	SpareDeviceStatusAvailable = "available"
	SpareDeviceStatusBorrowed  = "borrowed"
	SpareDeviceStatusDisabled  = "disabled"
)

type SpareDevice struct {
	DeviceID      string `json:"deviceId"`
	InstallID     string `json:"installId"`
	UserID        string `json:"userId,omitempty"`
	CDID          string `json:"cdid,omitempty"`
	OpenUDID      string `json:"openudid,omitempty"`
	Profile       string `json:"profile,omitempty"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	LastUsedAt    int64  `json:"lastUsedAt"`
	FailureCount  int    `json:"failureCount"`
	CooldownUntil int64  `json:"cooldownUntil"`
}

type SpareDeviceSettings struct {
	RegistrationEnabled        bool   `json:"registrationEnabled"`
	RegistrationDailyLimit     int    `json:"registrationDailyLimit"`
	RegistrationMinIntervalSec int64  `json:"registrationMinIntervalSec"`
	RegistrationLastAt         int64  `json:"registrationLastAt"`
	RegistrationDay            string `json:"registrationDay"`
	RegistrationDayCount       int    `json:"registrationDayCount"`
	UpdatedAt                  int64  `json:"updatedAt"`
}

type SpareDeviceBorrowEvent struct {
	Session   string `json:"session"`
	DeviceID  string `json:"deviceId,omitempty"`
	EpisodeID string `json:"episodeId,omitempty"`
	Cause     string `json:"cause,omitempty"`
	Event     string `json:"event"`
	Detail    string `json:"detail,omitempty"`
	At        int64  `json:"at"`
}

func (s *Store) SpareDevices() ([]SpareDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT device_id, install_id, user_id, cdid, openudid, profile, status,
		       created_at, updated_at, last_used_at, failure_count, cooldown_until
		FROM spare_devices
		ORDER BY CASE status WHEN 'available' THEN 0 WHEN 'borrowed' THEN 1 ELSE 2 END,
		         failure_count, last_used_at, device_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []SpareDevice
	for rows.Next() {
		var device SpareDevice
		if err := rows.Scan(
			&device.DeviceID, &device.InstallID, &device.UserID, &device.CDID, &device.OpenUDID,
			&device.Profile, &device.Status, &device.CreatedAt, &device.UpdatedAt,
			&device.LastUsedAt, &device.FailureCount, &device.CooldownUntil,
		); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func validSpareDeviceIDs(deviceID, installID string) error {
	deviceID, installID = strings.TrimSpace(deviceID), strings.TrimSpace(installID)
	if deviceID == "" || installID == "" || deviceID == "0" || installID == "0" {
		return fmt.Errorf("备用设备缺少有效的 did/iid")
	}
	if len(deviceID) > 128 || len(installID) > 128 || len([]rune(deviceID)) != len(deviceID) {
		return fmt.Errorf("备用设备 did/iid 格式无效")
	}
	return nil
}

func (s *Store) UpsertSpareDevice(device SpareDevice) error {
	if err := validSpareDeviceIDs(device.DeviceID, device.InstallID); err != nil {
		return err
	}
	device.DeviceID = strings.TrimSpace(device.DeviceID)
	device.InstallID = strings.TrimSpace(device.InstallID)
	device.UserID = strings.TrimSpace(device.UserID)
	device.CDID = strings.TrimSpace(device.CDID)
	device.OpenUDID = strings.TrimSpace(device.OpenUDID)
	device.Profile = strings.TrimSpace(device.Profile)
	if device.Profile != "" && len(device.Profile) > 64*1024 {
		return fmt.Errorf("备用设备 profile 过大")
	}
	switch device.Status {
	case "", SpareDeviceStatusAvailable:
		device.Status = SpareDeviceStatusAvailable
	case SpareDeviceStatusBorrowed, SpareDeviceStatusDisabled:
	default:
		return fmt.Errorf("备用设备状态无效")
	}
	now := time.Now().Unix()
	created := device.CreatedAt
	if created <= 0 {
		created = now
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO spare_devices (
			device_id, install_id, user_id, cdid, openudid, profile, status,
			created_at, updated_at, last_used_at, failure_count, cooldown_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
			install_id = excluded.install_id,
			user_id = excluded.user_id,
			cdid = excluded.cdid,
			openudid = excluded.openudid,
			profile = excluded.profile,
			status = excluded.status,
			updated_at = excluded.updated_at,
			cooldown_until = excluded.cooldown_until
	`, device.DeviceID, device.InstallID, device.UserID, device.CDID, device.OpenUDID,
		device.Profile, device.Status, created, now, device.LastUsedAt,
		device.FailureCount, device.CooldownUntil)
	return err
}

func (s *Store) DeleteSpareDevice(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("备用设备 ID 不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.db.Exec(`DELETE FROM spare_devices WHERE device_id = ?`, deviceID)
	if err != nil {
		return err
	}
	if deleted, _ := result.RowsAffected(); deleted == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetSpareDeviceSettings() (SpareDeviceSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	settings := SpareDeviceSettings{
		RegistrationDailyLimit:     1,
		RegistrationMinIntervalSec: 86400,
	}
	var enabled int
	err := s.db.QueryRow(`
		SELECT registration_enabled, registration_daily_limit,
		       registration_min_interval_sec, registration_last_at,
		       registration_day, registration_day_count, updated_at
		FROM spare_device_settings WHERE id = 1
	`).Scan(&enabled, &settings.RegistrationDailyLimit, &settings.RegistrationMinIntervalSec,
		&settings.RegistrationLastAt, &settings.RegistrationDay,
		&settings.RegistrationDayCount, &settings.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	settings.RegistrationEnabled = enabled != 0
	return settings, err
}

func (s *Store) SetSpareDeviceRegistrationEnabled(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO spare_device_settings (
			id, registration_enabled, registration_daily_limit,
			registration_min_interval_sec, updated_at
		) VALUES (1, ?, 1, 86400, ?)
		ON CONFLICT(id) DO UPDATE SET
			registration_enabled = excluded.registration_enabled,
			updated_at = excluded.updated_at
	`, boolInt(enabled), time.Now().Unix())
	return err
}

func (s *Store) SpareDeviceRegistrationState(now time.Time) (SpareDeviceSettings, bool, error) {
	settings, err := s.GetSpareDeviceSettings()
	if err != nil || !settings.RegistrationEnabled {
		return settings, false, err
	}
	day := now.UTC().Format(time.DateOnly)
	count := settings.RegistrationDayCount
	if settings.RegistrationDay != day {
		count = 0
	}
	allowed := count < settings.RegistrationDailyLimit &&
		(settings.RegistrationLastAt == 0 ||
			now.Sub(time.Unix(settings.RegistrationLastAt, 0)) >= time.Duration(settings.RegistrationMinIntervalSec)*time.Second)
	return settings, allowed, nil
}

func (s *Store) BorrowAvailableSpareDevice(session, episodeID, cause string, failureThreshold int, cooldown time.Duration, now time.Time) (SpareDevice, error) {
	if strings.TrimSpace(session) == "" {
		return SpareDevice{}, fmt.Errorf("备用设备借用会话不能为空")
	}
	if failureThreshold < 1 {
		failureThreshold = 1
	}
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return SpareDevice{}, err
	}
	defer tx.Rollback()
	deviceID, installID, userID, cdid, openudid, profile, createdAt, failureCount, err := func() (string, string, string, string, string, string, int64, int, error) {
		row := tx.QueryRow(`
			SELECT device_id, install_id, user_id, cdid, openudid, profile, created_at, failure_count
			FROM spare_devices
			WHERE status = 'available' AND cooldown_until <= ?
			ORDER BY failure_count, last_used_at, device_id
			LIMIT 1
		`, now.Unix())
		var values struct {
			deviceID, installID, userID, cdid, openudid, profile string
			createdAt                                            int64
			failures                                             int
		}
		err := row.Scan(&values.deviceID, &values.installID, &values.userID, &values.cdid,
			&values.openudid, &values.profile, &values.createdAt, &values.failures)
		return values.deviceID, values.installID, values.userID, values.cdid, values.openudid,
			values.profile, values.createdAt, values.failures, err
	}()
	if errors.Is(err, sql.ErrNoRows) {
		return SpareDevice{}, sql.ErrNoRows
	}
	if err != nil {
		return SpareDevice{}, err
	}
	if _, err := tx.Exec(`
		UPDATE spare_devices
		SET status = 'borrowed', last_used_at = ?, updated_at = ?
		WHERE device_id = ? AND status = 'available'
	`, now.Unix(), now.Unix(), deviceID); err != nil {
		return SpareDevice{}, err
	}
	if err := appendSpareDeviceEventTx(tx, SpareDeviceBorrowEvent{
		Session: session, DeviceID: deviceID, EpisodeID: episodeID,
		Cause: cause, Event: "borrow", At: now.Unix(),
	}); err != nil {
		return SpareDevice{}, err
	}
	if err := tx.Commit(); err != nil {
		return SpareDevice{}, err
	}
	return SpareDevice{
		DeviceID: deviceID, InstallID: installID, UserID: userID, CDID: cdid,
		OpenUDID: openudid, Profile: profile, Status: SpareDeviceStatusBorrowed,
		CreatedAt: createdAt, UpdatedAt: now.Unix(), LastUsedAt: now.Unix(),
		FailureCount: failureCount,
	}, nil
}

func (s *Store) CompleteSpareDeviceBorrow(session, deviceID string, failed bool, failureThreshold int, cooldown time.Duration, now time.Time) error {
	if strings.TrimSpace(session) == "" || strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("备用设备归还参数无效")
	}
	if failureThreshold < 1 {
		failureThreshold = 1
	}
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var failures int
	if err := tx.QueryRow(`
		SELECT failure_count FROM spare_devices WHERE device_id = ?
	`, deviceID).Scan(&failures); err != nil {
		return err
	}
	nextFailures := failures
	status := SpareDeviceStatusAvailable
	cooldownUntil := int64(0)
	if failed {
		nextFailures++
		cooldownUntil = now.Add(cooldown).Unix()
		if nextFailures >= failureThreshold {
			status = SpareDeviceStatusDisabled
		}
	}
	if _, err := tx.Exec(`
		UPDATE spare_devices
		SET status = ?, failure_count = ?, cooldown_until = ?, updated_at = ?
		WHERE device_id = ?
	`, status, nextFailures, cooldownUntil, now.Unix(), deviceID); err != nil {
		return err
	}
	event := "release"
	detail := ""
	if failed {
		event = "failure"
		detail = fmt.Sprintf("failures=%d", nextFailures)
	}
	if err := appendSpareDeviceEventTx(tx, SpareDeviceBorrowEvent{
		Session: session, DeviceID: deviceID, Event: event,
		Detail: detail, At: now.Unix(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func appendSpareDeviceEventTx(tx *sql.Tx, event SpareDeviceBorrowEvent) error {
	if event.At == 0 {
		event.At = time.Now().Unix()
	}
	_, err := tx.Exec(`
		INSERT INTO spare_device_borrows (
			session, device_id, episode_id, cause, event, detail, at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, event.Session, event.DeviceID, event.EpisodeID, event.Cause,
		event.Event, event.Detail, event.At)
	return err
}

func (s *Store) ConsumeSpareDeviceRegistration(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var settings SpareDeviceSettings
	var enabled int
	err := s.db.QueryRow(`
		SELECT registration_enabled, registration_daily_limit,
		       registration_min_interval_sec, registration_last_at,
		       registration_day, registration_day_count
		FROM spare_device_settings WHERE id = 1
	`).Scan(&enabled, &settings.RegistrationDailyLimit, &settings.RegistrationMinIntervalSec,
		&settings.RegistrationLastAt, &settings.RegistrationDay, &settings.RegistrationDayCount)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("备用设备注册开关未开启")
	}
	if err != nil {
		return err
	}
	if enabled == 0 {
		return fmt.Errorf("备用设备注册开关未开启")
	}
	day := now.UTC().Format(time.DateOnly)
	count := settings.RegistrationDayCount
	if settings.RegistrationDay != day {
		count = 0
	}
	if count >= settings.RegistrationDailyLimit {
		return fmt.Errorf("备用设备注册超过单日上限")
	}
	if settings.RegistrationLastAt > 0 && now.Sub(time.Unix(settings.RegistrationLastAt, 0)) < time.Duration(settings.RegistrationMinIntervalSec)*time.Second {
		return fmt.Errorf("备用设备注册间隔未到")
	}
	_, err = s.db.Exec(`
		INSERT INTO spare_device_settings (
			id, registration_enabled, registration_daily_limit,
			registration_min_interval_sec, registration_last_at,
			registration_day, registration_day_count, updated_at
		) VALUES (1, 1, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(id) DO UPDATE SET
			registration_last_at = excluded.registration_last_at,
			registration_day = excluded.registration_day,
			registration_day_count = excluded.registration_day_count,
			updated_at = excluded.updated_at
	`, settings.RegistrationDailyLimit, settings.RegistrationMinIntervalSec,
		now.Unix(), day, now.Unix())
	return err
}

func (s *Store) AppendSpareDeviceEvent(event SpareDeviceBorrowEvent) error {
	event.Session = strings.TrimSpace(event.Session)
	event.Event = strings.TrimSpace(event.Event)
	if event.Session == "" || event.Event == "" {
		return fmt.Errorf("备用设备审计事件缺少会话或类型")
	}
	if event.At == 0 {
		event.At = time.Now().Unix()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO spare_device_borrows (
			session, device_id, episode_id, cause, event, detail, at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, event.Session, event.DeviceID, event.EpisodeID, event.Cause, event.Event,
		event.Detail, event.At)
	return err
}

func (s *Store) SpareDeviceEvents(session string) ([]SpareDeviceBorrowEvent, error) {
	session = strings.TrimSpace(session)
	if session == "" {
		return nil, fmt.Errorf("备用设备审计会话不能为空")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT session, device_id, episode_id, cause, event, detail, at
		FROM spare_device_borrows WHERE session = ? ORDER BY at, id
	`, session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []SpareDeviceBorrowEvent
	for rows.Next() {
		var event SpareDeviceBorrowEvent
		if err := rows.Scan(&event.Session, &event.DeviceID, &event.EpisodeID,
			&event.Cause, &event.Event, &event.Detail, &event.At); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
