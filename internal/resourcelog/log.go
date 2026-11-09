package resourcelog

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultMaxLogBytes  = 5 * 1024 * 1024
	DefaultMaxLogFiles  = 4
	maxRedactedURLRunes = 2048
)

func RedactURL(raw string) string {
	if raw == "" || len(raw) > maxRedactedURLRunes {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	path := "/" + strings.TrimPrefix(parsed.EscapedPath(), "/")
	return parsed.Scheme + "://" + parsed.Host + path
}

func OpenRotated(path string, maxBytes int64, maxFiles int) (*os.File, error) {
	if maxBytes < 1 || maxFiles < 1 {
		return nil, fmt.Errorf("log rotation limits must be positive")
	}
	if info, err := os.Stat(path); err == nil && info.Size() >= maxBytes {
		if err := rotate(path, maxFiles); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func rotate(path string, maxFiles int) error {
	oldest := fmt.Sprintf("%s.%d", path, maxFiles)
	if _, err := os.Lstat(oldest); err == nil {
		if err := os.Remove(oldest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for index := maxFiles - 1; index >= 1; index-- {
		source := fmt.Sprintf("%s.%d", path, index)
		target := fmt.Sprintf("%s.%d", path, index+1)
		if err := os.Rename(source, target); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(path, fmt.Sprintf("%s.1", path))
}
