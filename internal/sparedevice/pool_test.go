package sparedevice

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/store"
)

func testManager(t *testing.T, factory ClientFactory) (*Manager, *store.Store, *bytes.Buffer) {
	t.Helper()
	state, err := store.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })
	logs := &bytes.Buffer{}
	manager := NewManager(state,
		WithClientFactory(factory),
		WithCooldown(time.Minute),
		WithFailureThreshold(2),
		WithClock(func() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC) }),
		WithLogger(log.New(logs, "", 0)),
	)
	return manager, state, logs
}

func TestManagerBorrowsOnlyForSwitchableFailureAndStaysSessionStable(t *testing.T) {
	var requested []string
	manager, state, logs := testManager(t, func(profile sdk.ProfileStore) (PlaybackResolver, error) {
		device, _ := profile.Load(context.Background())
		requested = append(requested, device.DeviceID)
		return func(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error) {
			return &model.Playback{EpisodeID: "episode"}, nil
		}, nil
	})
	if err := manager.Import(store.SpareDevice{DeviceID: "did-1", InstallID: "iid-1"}); err != nil {
		t.Fatal(err)
	}
	primary := func(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error) {
		return nil, sdk.ErrAnonymousLimited
	}
	first, err := manager.ResolveForSession(context.Background(), "session", "episode", sdk.ErrAnonymousLimited, primary)
	if err != nil || first.EpisodeID != "episode" {
		t.Fatalf("first = %+v err = %v", first, err)
	}
	second, err := manager.ResolveForSession(context.Background(), "session", "next", sdk.ErrRateLimited, primary)
	if err != nil || second.EpisodeID != "episode" {
		t.Fatalf("second = %+v err = %v", second, err)
	}
	if len(requested) != 2 || requested[0] != "did-1" || requested[1] != "did-1" {
		t.Fatalf("session changed devices: %v", requested)
	}
	devices, _ := state.SpareDevices()
	if devices[0].Status != store.SpareDeviceStatusBorrowed {
		t.Fatalf("device not borrowed = %+v", devices[0])
	}
	_ = logs
	manager.ReleaseSession("session")
	devices, _ = state.SpareDevices()
	if devices[0].Status != store.SpareDeviceStatusAvailable {
		t.Fatalf("release did not return device = %+v", devices[0])
	}
}

func TestManagerFailureCooldownAndDisable(t *testing.T) {
	manager, state, _ := testManager(t, func(sdk.ProfileStore) (PlaybackResolver, error) {
		return nil, errors.New("spare rejected")
	})
	if err := manager.Import(store.SpareDevice{DeviceID: "did-1", InstallID: "iid-1"}); err != nil {
		t.Fatal(err)
	}
	primary := func(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error) {
		return nil, sdk.ErrAnonymousLimited
	}
	if _, err := manager.ResolveForSession(context.Background(), "one", "episode", sdk.ErrAnonymousLimited, primary); err == nil {
		t.Fatal("failure was hidden")
	}
	manager = NewManager(state,
		WithClientFactory(func(sdk.ProfileStore) (PlaybackResolver, error) {
			return nil, errors.New("spare rejected")
		}),
		WithCooldown(time.Minute),
		WithFailureThreshold(2),
		WithClock(func() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC) }),
	)
	devices, _ := state.SpareDevices()
	if devices[0].FailureCount != 1 || devices[0].CooldownUntil == 0 {
		t.Fatalf("cooldown missing = %+v", devices[0])
	}
	if _, err := manager.ResolveForSession(context.Background(), "two", "episode", sdk.ErrAnonymousLimited, primary); !errors.Is(err, ErrNoDeviceAvailable) {
		t.Fatalf("cooldown bypass err = %v", err)
	}
}

func TestManagerDoesNotBorrowForUnswitchableFailure(t *testing.T) {
	manager, _, _ := testManager(t, func(sdk.ProfileStore) (PlaybackResolver, error) {
		t.Fatal("spare resolver called")
		return nil, nil
	})
	cause := errors.New("device rejected")
	_, err := manager.ResolveForSession(context.Background(), "session", "episode", cause, func(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error) {
		return nil, cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("err = %v", err)
	}
}
