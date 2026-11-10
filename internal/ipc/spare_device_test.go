package ipc

import (
	"context"
	"testing"

	"myguo/internal/sparedevice"
)

func TestSpareDeviceStateAndManualImport(t *testing.T) {
	state, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	service := &HongguoService{store: state}
	service.spareDevices = sparedevice.NewManager(state)

	got, err := service.GetSpareDeviceState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings.RegistrationEnabled || got.Settings.RegistrationDailyLimit != 1 || got.Settings.RegistrationMinIntervalSec != 86400 {
		t.Fatalf("default settings = %+v", got.Settings)
	}
	got, err = service.ImportSpareDevice(context.Background(), "did", "iid", "uid")
	if err != nil || len(got.Devices) != 1 || got.Devices[0].UserID != "uid" {
		t.Fatalf("import = %+v err = %v", got, err)
	}
	got, err = service.SetSpareDeviceRegistration(context.Background(), true)
	if err != nil || !got.Settings.RegistrationEnabled {
		t.Fatalf("registration state = %+v err = %v", got, err)
	}
	got, err = service.DeleteSpareDevice(context.Background(), "missing")
	if err == nil {
		t.Fatalf("missing delete succeeded: %+v", got)
	}
}
