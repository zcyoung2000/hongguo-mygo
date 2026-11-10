package ipc

import (
	"context"
	"fmt"
	"strings"

	"myguo/internal/store"
)

type SpareDeviceDTO struct {
	DeviceID      string `json:"deviceId"`
	InstallID     string `json:"installId"`
	UserID        string `json:"userId,omitempty"`
	Status        string `json:"status"`
	FailureCount  int    `json:"failureCount"`
	CooldownUntil int64  `json:"cooldownUntil"`
	LastUsedAt    int64  `json:"lastUsedAt"`
}

type SpareDeviceSettingsDTO struct {
	RegistrationEnabled        bool  `json:"registrationEnabled"`
	RegistrationDailyLimit     int   `json:"registrationDailyLimit"`
	RegistrationMinIntervalSec int64 `json:"registrationMinIntervalSec"`
}

type SpareDeviceStateDTO struct {
	Settings SpareDeviceSettingsDTO `json:"settings"`
	Devices  []SpareDeviceDTO       `json:"devices"`
}

func (s *HongguoService) GetSpareDeviceState(context.Context) (*SpareDeviceStateDTO, error) {
	if s.spareDevices == nil {
		return nil, fmt.Errorf("备用设备池未启用")
	}
	settings, err := s.spareDevices.Settings()
	if err != nil {
		return nil, err
	}
	devices, err := s.spareDevices.Devices()
	if err != nil {
		return nil, err
	}
	dtos := make([]SpareDeviceDTO, 0, len(devices))
	for _, device := range devices {
		dtos = append(dtos, spareDeviceDTO(device))
	}
	return sanitizeIPC(&SpareDeviceStateDTO{
		Settings: SpareDeviceSettingsDTO{
			RegistrationEnabled:        settings.RegistrationEnabled,
			RegistrationDailyLimit:     settings.RegistrationDailyLimit,
			RegistrationMinIntervalSec: settings.RegistrationMinIntervalSec,
		},
		Devices: dtos,
	}), nil
}

func (s *HongguoService) ImportSpareDevice(_ context.Context, deviceID, installID, userID string) (*SpareDeviceStateDTO, error) {
	if s.spareDevices == nil {
		return nil, fmt.Errorf("备用设备池未启用")
	}
	if err := s.spareDevices.Import(store.SpareDevice{
		DeviceID: deviceID, InstallID: installID, UserID: userID,
	}); err != nil {
		return nil, err
	}
	return s.GetSpareDeviceState(context.Background())
}

func (s *HongguoService) DeleteSpareDevice(_ context.Context, deviceID string) (*SpareDeviceStateDTO, error) {
	if s.spareDevices == nil {
		return nil, fmt.Errorf("备用设备池未启用")
	}
	if err := s.spareDevices.Remove(strings.TrimSpace(deviceID)); err != nil {
		return nil, err
	}
	return s.GetSpareDeviceState(context.Background())
}

func (s *HongguoService) SetSpareDeviceRegistration(_ context.Context, enabled bool) (*SpareDeviceStateDTO, error) {
	if s.spareDevices == nil {
		return nil, fmt.Errorf("备用设备池未启用")
	}
	if err := s.spareDevices.SetRegistrationEnabled(enabled); err != nil {
		return nil, err
	}
	return s.GetSpareDeviceState(context.Background())
}

func spareDeviceDTO(device store.SpareDevice) SpareDeviceDTO {
	return SpareDeviceDTO{
		DeviceID: device.DeviceID, InstallID: device.InstallID, UserID: device.UserID,
		Status: device.Status, FailureCount: device.FailureCount,
		CooldownUntil: device.CooldownUntil, LastUsedAt: device.LastUsedAt,
	}
}
