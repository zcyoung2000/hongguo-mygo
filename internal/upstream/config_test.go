package upstream

import (
	"testing"
	"time"
)

func TestConfigurationDefaultsAndOverrides(t *testing.T) {
	t.Setenv("MYGUO_MULTI_HOST", "")
	t.Setenv("MYGUO_MULTI_HOST_ATTEMPTS", "")
	t.Setenv("MYGUO_MULTI_HOST_COOLDOWN", "")
	t.Setenv("MYGUO_MULTI_HOST_HALF_OPEN", "")
	t.Setenv("MYGUO_MULTI_HOST_STALE_AGE", "")
	if !EnabledFromEnvironment() || AttemptsFromEnvironment() != DefaultMaxHostAttempts ||
		CooldownFromEnvironment() != DefaultCooldown || HalfOpenFromEnvironment() != DefaultHalfOpenInterval ||
		StaleAgeFromEnvironment() != DefaultStaleAge {
		t.Fatal("default configuration changed")
	}

	t.Setenv("MYGUO_MULTI_HOST", "0")
	t.Setenv("MYGUO_MULTI_HOST_ATTEMPTS", "2")
	t.Setenv("MYGUO_MULTI_HOST_COOLDOWN", "11m")
	t.Setenv("MYGUO_MULTI_HOST_HALF_OPEN", "31s")
	t.Setenv("MYGUO_MULTI_HOST_STALE_AGE", "25h")
	if EnabledFromEnvironment() || AttemptsFromEnvironment() != 2 ||
		CooldownFromEnvironment() != 11*time.Minute || HalfOpenFromEnvironment() != 31*time.Second ||
		StaleAgeFromEnvironment() != 25*time.Hour {
		t.Fatal("configuration overrides were ignored")
	}
}
