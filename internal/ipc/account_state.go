package ipc

import "fmt"

func anonymousProfile(deviceID string) UserProfile {
	return UserProfile{UserID: "anonymous", Nickname: "myguo用户", DeviceID: deviceID}
}

func (s *HongguoService) restoreSavedAccount() error {
	auth, hasAuth := s.store.GetAccountAuth()
	if !hasAuth {
		if s.store != nil {
			profile := s.store.GetProfile()
			stale := profile.LoggedIn || profile.UserID != "anonymous"
			if stale {
				if err := s.store.UpdateProfile(anonymousProfile(profile.DeviceID)); err != nil {
					return err
				}
			}
			// A stale logged-in identity with no credentials must not leak into
			// anonymous browsing. A normal guest session is preserved locally.
			if stale {
				if err := s.store.ClearUserData(); err != nil {
					return fmt.Errorf("清除无凭据账号数据失败: %w", err)
				}
			}
		}
		return nil
	}

	profile := s.store.GetProfile()
	if !profile.LoggedIn || profile.UserID == "" || profile.UserID == "anonymous" {
		return fmt.Errorf("保存的登录凭据缺少有效账号资料")
	}
	s.setAuthCredentials(auth.Cookies, auth.Token, profile.UserID)
	return nil
}
