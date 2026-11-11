package store

import (
	"errors"
	"testing"
	"time"
)

func TestSpareDevicePoolBorrowReleaseAndFailureThreshold(t *testing.T) {
	state, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	device := SpareDevice{DeviceID: "did-1", InstallID: "iid-1", UserID: "uid-1"}
	if err := state.UpsertSpareDevice(device); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	borrowed, err := state.BorrowAvailableSpareDevice("session", "episode", "limited", 2, 15*time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if borrowed.DeviceID != device.DeviceID || borrowed.Status != SpareDeviceStatusBorrowed {
		t.Fatalf("borrowed = %+v", borrowed)
	}
	if _, err := state.BorrowAvailableSpareDevice("other", "episode", "limited", 2, 15*time.Minute, now); !errors.Is(err, ErrNoSpareDevice) {
		t.Fatalf("second borrow err = %v", err)
	}
	if err := state.CompleteSpareDeviceBorrow("session", device.DeviceID, true, 2, 15*time.Minute, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ := state.SpareDevices()
	if got[0].Status != SpareDeviceStatusAvailable || got[0].FailureCount != 1 || got[0].CooldownUntil != now.Add(16*time.Minute).Unix() {
		t.Fatalf("after first failure = %+v", got[0])
	}
	if _, err := state.BorrowAvailableSpareDevice("session2", "episode", "limited", 2, 15*time.Minute, now.Add(17*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := state.CompleteSpareDeviceBorrow("session2", device.DeviceID, true, 2, 15*time.Minute, now.Add(18*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ = state.SpareDevices()
	if got[0].Status != SpareDeviceStatusDisabled || got[0].FailureCount != 2 {
		t.Fatalf("after threshold = %+v", got[0])
	}
	events, err := state.SpareDeviceEvents("session")
	if err != nil || len(events) != 2 {
		t.Fatalf("events = %+v err = %v", events, err)
	}
}

func TestSpareDeviceRegistrationSwitchAndThrottle(t *testing.T) {
	state, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	if _, allowed, err := state.SpareDeviceRegistrationState(now); err != nil || allowed {
		t.Fatalf("default allowed = %v err = %v", allowed, err)
	}
	if err := state.SetSpareDeviceRegistrationEnabled(true); err != nil {
		t.Fatal(err)
	}
	if _, allowed, err := state.SpareDeviceRegistrationState(now); err != nil || !allowed {
		t.Fatalf("enabled allowed = %v err = %v", allowed, err)
	}
	if err := state.ConsumeSpareDeviceRegistration(now); err != nil {
		t.Fatal(err)
	}
	if err := state.ConsumeSpareDeviceRegistration(now.Add(time.Hour)); err == nil {
		t.Fatal("interval throttle missing")
	}
	settings, err := state.GetSpareDeviceSettings()
	if err != nil || settings.RegistrationDayCount != 1 || settings.RegistrationLastAt != now.Unix() {
		t.Fatalf("daily state = %+v err = %v", settings, err)
	}
}
