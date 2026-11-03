package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"myguo/internal/native"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *HongguoService) ResetDevice(ctx context.Context) error {
	return s.ResetDeviceWithContext(ctx)
}

func (s *HongguoService) ResetDeviceWithContext(ctx context.Context) error {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.client.ResetDevice(ctx); err != nil {
		return err
	}
	if s.deviceProfileStore != nil {
		if err := s.reloadDeviceIdentityLocked(); err != nil {
			return err
		}
	}
	s.invalidateUserDataCaches()
	return nil
}

func (s *HongguoService) ClearCache(ctx context.Context) (int64, error) {
	s.invalidateUserDataCaches()
	return s.media.ClearCache()
}

func (s *HongguoService) GetCacheSize(ctx context.Context) (int64, error) {
	return s.media.GetCacheSize(), nil
}

func (s *HongguoService) GetDeviceInfo(ctx context.Context) (map[string]string, error) {
	deviceID, iid, cdid, openudid := s.deviceIdentity()
	return map[string]string{
		"deviceId":    deviceID,
		"installId":   iid,
		"cdid":        cdid,
		"openudid":    openudid,
		"storeRegion": s.storeRegion,
		"version":     "2.0.0",
		"arch":        "darwin-arm64",
	}, nil
}

func (s *HongguoService) LoginWithCookies(ctx context.Context, cookies, token string) (*UserProfile, error) {
	cookies = strings.TrimSpace(cookies)
	token = strings.TrimSpace(token)

	// Support parsing raw JSON input if the user pasted a JSON string
	if strings.HasPrefix(cookies, "{") && strings.HasSuffix(cookies, "}") {
		var rawObj struct {
			Cookies string `json:"cookies"`
			Token   string `json:"token"`
		}
		if err := json.Unmarshal([]byte(cookies), &rawObj); err == nil && rawObj.Cookies != "" {
			cookies = rawObj.Cookies
			if token == "" && rawObj.Token != "" {
				token = rawObj.Token
			}
		}
	}

	if cookies == "" {
		return nil, fmt.Errorf("Cookie 不能为空")
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	reqURL := "https://api5-normal-sinfonlinea.fqnovel.com/reading/user/info/v1/?aid=8662&app_name=novelread&version_code=73932"
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建验证请求失败: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Cookie", cookies)
	if token != "" {
		req.Header.Set("x-tt-token", token)
	}
	req.Header.Set("x-tt-store-region", s.storeRegion)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求官方登录接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取接口响应失败: %w", err)
	}

	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			UserName  string `json:"user_name"`
			AvatarURL string `json:"avatar_url"`
			UserID    string `json:"user_id"`
			BizUserID string `json:"biz_user_id"`
			ReqID     any    `json:"req_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("解析用户信息响应失败: %w", err)
	}

	if res.Code != 0 {
		msg := res.Message
		if msg == "" {
			msg = "凭据无效或已失效"
		}
		return nil, fmt.Errorf("登录验证未通过: %s (code: %d)", msg, res.Code)
	}

	// Anonymous /user/info responses also return code 0. req_id is not an
	// account ID, so only a non-zero account or business account proves login.
	uid := firstNonEmpty(
		nonZeroUserID(res.Data.BizUserID),
		nonZeroUserID(res.Data.UserID),
	)
	if uid == "" {
		return nil, fmt.Errorf("登录验证未通过: 凭据未返回有效账号")
	}
	nickname := strings.TrimSpace(res.Data.UserName)
	if nickname == "" {
		nickname = "myguo用户"
	}
	avatar := strings.TrimSpace(res.Data.AvatarURL)

	// Save to account.json
	accPath := filepath.Join(s.appDir, "account.json")
	accData := map[string]any{
		"cookies": cookies,
		"token":   token,
		"profile": map[string]any{
			"loggedIn": true,
			"userId":   uid,
			"nickname": nickname,
			"avatar":   avatar,
		},
	}
	accBytes, _ := json.MarshalIndent(accData, "", "  ")
	_ = os.WriteFile(accPath, accBytes, 0644)

	// Update service state
	s.setAuthCredentials(cookies, token)

	profile := UserProfile{
		UserID:   uid,
		Nickname: nickname,
		Avatar:   avatar,
		LoggedIn: true,
	}
	_ = s.store.UpdateProfile(profile)

	// Invalidate user data caches
	s.invalidateUserDataCaches()

	if profile.Avatar != "" {
		profile.Avatar = s.media.ImageURL(profile.Avatar)
	}
	return sanitizeIPC(&profile), nil
}

func nonZeroUserID(value string) string {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return ""
	}
	return strings.TrimSpace(value)
}

func (s *HongguoService) RefreshUserProfile(ctx context.Context) (*UserProfile, error) {
	userCookies, userToken := s.authCredentials()
	if userCookies == "" {
		return nil, fmt.Errorf("当前未登录")
	}
	return s.LoginWithCookies(ctx, userCookies, userToken)
}

func (s *HongguoService) Logout(ctx context.Context) error {
	s.setAuthCredentials("", "")

	accPath := filepath.Join(s.appDir, "account.json")
	_ = os.Remove(accPath)

	anonProfile := UserProfile{
		UserID:   "anonymous",
		Nickname: "myguo用户",
		Avatar:   "",
		LoggedIn: false,
	}
	_ = s.store.UpdateProfile(anonProfile)

	s.cacheMu.Lock()
	s.cache = make(map[string]cacheItem)
	s.cacheMu.Unlock()
	s.invalidateUserDataCaches()

	return nil
}

func (s *HongguoService) CheckExistingAccount(ctx context.Context) (map[string]any, error) {
	home, _ := os.UserHomeDir()
	oldAccPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	data, err := os.ReadFile(oldAccPath)
	if err != nil {
		return sanitizeIPC(map[string]any{"exists": false}), nil
	}
	var acc struct {
		Cookies string `json:"cookies"`
		Token   string `json:"token"`
		Profile struct {
			UserID   string `json:"userId"`
			Nickname string `json:"nickname"`
			Avatar   string `json:"avatar"`
			LoggedIn bool   `json:"loggedIn"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &acc); err != nil || acc.Cookies == "" {
		return sanitizeIPC(map[string]any{"exists": false}), nil
	}

	avatar := acc.Profile.Avatar
	if avatar != "" {
		avatar = s.media.ImageURL(avatar)
	}

	return sanitizeIPC(map[string]any{
		"exists":   true,
		"nickname": acc.Profile.Nickname,
		"userId":   acc.Profile.UserID,
		"avatar":   avatar,
	}), nil
}

func (s *HongguoService) ImportHgplayerAccount(ctx context.Context) (*UserProfile, error) {
	home, _ := os.UserHomeDir()
	oldAccPath := filepath.Join(home, "Library", "Application Support", "hgplayer", "account.json")
	data, err := os.ReadFile(oldAccPath)
	if err != nil {
		return nil, fmt.Errorf("未找到旧版应用账号数据")
	}
	var acc struct {
		Cookies string `json:"cookies"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(data, &acc); err != nil || acc.Cookies == "" {
		return nil, fmt.Errorf("旧版账号数据无效")
	}
	return s.LoginWithCookies(ctx, acc.Cookies, acc.Token)
}


// ============================================================================
// macOS Native Integration IPC Handlers
// ============================================================================

func (s *HongguoService) UpdateNowPlaying(ctx context.Context, args UpdateNowPlayingArgs) (bool, error) {
	cacheDir := ""
	if s.media != nil {
		cacheDir = s.media.CacheDir()
	}
	native.UpdateNowPlaying(cacheDir, args)
	return true, nil
}

func (s *HongguoService) ClearNowPlaying(ctx context.Context) (bool, error) {
	native.ClearNowPlaying()
	return true, nil
}

func (s *HongguoService) SetPlaybackKeepAwake(ctx context.Context, active bool) (bool, error) {
	native.SetPlaybackKeepAwake(active)
	return true, nil
}

func (s *HongguoService) ShowConfirmDialog(ctx context.Context, args ConfirmDialogArgs) (bool, error) {
	return native.ShowConfirmDialog(args)
}

func (s *HongguoService) SendNativeNotification(ctx context.Context, args NotificationArgs) (bool, error) {
	return native.SendNotification(args)
}

func (s *HongguoService) TogglePictureInPicture(ctx context.Context) (bool, error) {
	native.TogglePictureInPicture()
	return true, nil
}
