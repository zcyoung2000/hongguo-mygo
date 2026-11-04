package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type savedAccount struct {
	Cookies string      `json:"cookies"`
	Token   string      `json:"token"`
	Profile UserProfile `json:"profile"`
}

func anonymousProfile(deviceID string) UserProfile {
	return UserProfile{UserID: "anonymous", Nickname: "myguo用户", DeviceID: deviceID}
}

func readAccountFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func writeAccountFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".account-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func restoreAccountFile(path string, previous []byte) error {
	if previous != nil {
		return writeAccountFile(path, previous)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *HongguoService) restoreSavedAccount() error {
	data, readErr := readAccountFile(filepath.Join(s.appDir, "account.json"))
	var account savedAccount
	var decodeErr error
	if len(data) > 0 {
		decodeErr = json.Unmarshal(data, &account)
	}
	if readErr != nil || decodeErr != nil || strings.TrimSpace(account.Cookies) == "" {
		if s.store != nil {
			profile := s.store.GetProfile()
			if profile.LoggedIn || profile.UserID != "anonymous" {
				if err := s.store.UpdateProfile(anonymousProfile(profile.DeviceID)); err != nil {
					return errors.Join(readErr, decodeErr, err)
				}
			}
		}
		return errors.Join(readErr, decodeErr)
	}
	account.Profile.LoggedIn = true
	if account.Profile.Nickname == "" {
		account.Profile.Nickname = "myguo用户"
	}
	if s.store != nil {
		if err := s.store.UpdateProfile(account.Profile); err != nil {
			return fmt.Errorf("恢复用户资料失败: %w", err)
		}
	}
	s.setAuthCredentials(account.Cookies, account.Token, account.Profile.UserID)
	return nil
}
