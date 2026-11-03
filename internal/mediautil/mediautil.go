package mediautil

import (
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/beilunyang/openhongguo/sdk"
)

const DefaultVideoQuality = "720p"

var videoQualities = []string{"1080p", "720p", "540p", "480p", "360p"}

func NormalizeVideoQuality(quality string) string {
	requested := strings.ToLower(strings.TrimSpace(quality))
	for _, candidate := range videoQualities {
		if requested == candidate {
			return candidate
		}
	}
	return DefaultVideoQuality
}

func ValidContentID(id string) bool {
	if len(id) < 6 || len(id) > 30 {
		return false
	}
	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func NormalizeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Strip local proxy prefix if present
	if idx := strings.Index(raw, "/img?u="); idx != -1 {
		raw = raw[idx+len("/img?u="):]
		if dec, err := base64.URLEncoding.DecodeString(raw); err == nil && len(dec) > 0 && strings.HasPrefix(string(dec), "http") {
			raw = string(dec)
		} else if decStd, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decStd) > 0 && strings.HasPrefix(string(decStd), "http") {
			raw = string(decStd)
		}
	}
	// If double-encoded or starts with https%3A%2F%2F, unescape once
	if strings.HasPrefix(raw, "https%3A%2F%2F") || strings.HasPrefix(raw, "http%3A%2F%2F") {
		if unescaped, err := url.QueryUnescape(raw); err == nil {
			raw = unescaped
		}
	}
	return raw
}

func PageItems[T any](page *sdk.Page[T]) []T {
	if page == nil {
		return nil
	}
	return page.Items
}
