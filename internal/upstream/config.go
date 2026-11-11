package upstream

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultEnabled = true

func EnabledFromEnvironment() bool {
	value := strings.TrimSpace(os.Getenv("MYGUO_MULTI_HOST"))
	if value == "" {
		return DefaultEnabled
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func AttemptsFromEnvironment() int {
	value := strings.TrimSpace(os.Getenv("MYGUO_MULTI_HOST_ATTEMPTS"))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return DefaultMaxHostAttempts
	}
	return parsed
}

func CooldownFromEnvironment() time.Duration {
	value := strings.TrimSpace(os.Getenv("MYGUO_MULTI_HOST_COOLDOWN"))
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return DefaultCooldown
	}
	return parsed
}

func HalfOpenFromEnvironment() time.Duration {
	value := strings.TrimSpace(os.Getenv("MYGUO_MULTI_HOST_HALF_OPEN"))
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return DefaultHalfOpenInterval
	}
	return parsed
}
