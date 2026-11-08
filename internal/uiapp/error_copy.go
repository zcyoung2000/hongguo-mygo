package uiapp

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"regexp"

	"github.com/beilunyang/openhongguo/sdk"
)

var authStatusPattern = regexp.MustCompile(`\b(?:HTTP|returned)\s+(401|403)\b`)

func userFacingError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, sdk.ErrAuthenticationRequired):
		return "会话已失效，请重新登录"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled),
		errors.Is(err, sdk.ErrTransport):
		return "网络异常，请检查网络后重试"
	case errors.Is(err, sdk.ErrRateLimited),
		errors.Is(err, sdk.ErrAnonymousLimited), errors.Is(err, sdk.ErrDeviceRejected),
		errors.Is(err, sdk.ErrChallengeRequired), errors.Is(err, sdk.ErrSignatureRejected):
		return "网络异常或访问受限，请稍后重试；匿名访问受限时可在设置中重置设备"
	}

	var apiErr *sdk.APIError
	if errors.As(err, &apiErr) && (apiErr.HTTPStatus == http.StatusUnauthorized || apiErr.HTTPStatus == http.StatusForbidden) {
		return "会话已失效，请重新登录"
	}
	if authStatusPattern.MatchString(err.Error()) {
		return "会话已失效，请重新登录"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "网络异常，请检查网络后重试"
	}
	return "加载失败，请稍后重试"
}

func featureError(scope string, err error) string {
	if err != nil {
		log.Printf("[uiapp] %s failed: %v", scope, err)
	}
	return userFacingError(err)
}
