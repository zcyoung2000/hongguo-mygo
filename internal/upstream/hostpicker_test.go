package upstream

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

type memoryStateStore struct {
	states []HostState
	err    error
}

func (store *memoryStateStore) LoadHostStates() ([]HostState, error) {
	return store.states, store.err
}

func (store *memoryStateStore) SaveHostStates(states []HostState) error {
	if store.err != nil {
		return store.err
	}
	store.states = append([]HostState(nil), states...)
	return nil
}

func TestHostPickerOrdersByHealthAndSuccess(t *testing.T) {
	picker := NewHostPicker(nil)
	picker.MarkBad(FamilyPlayback, DefaultPlaybackHosts[0])
	picker.MarkGood(FamilyPlayback, DefaultPlaybackHosts[1])

	got := picker.Order(FamilyPlayback)
	if got[0] != DefaultPlaybackHosts[1] || got[len(got)-1] != DefaultPlaybackHosts[0] {
		t.Fatalf("order = %v", got)
	}
}

func TestHostPickerCooldownAndHalfOpenRecovery(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	store := &memoryStateStore{}
	picker := NewHostPicker(store)
	picker.SetNow(func() time.Time { return now })
	picker.MarkBad(FamilyComment, DefaultCommentHosts[0])

	if got := picker.Order(FamilyComment); got[0] != DefaultCommentHosts[1] {
		t.Fatalf("during cooldown order = %v", got)
	}
	now = now.Add(DefaultCooldown + DefaultHalfOpenInterval)
	now = now.Add(DefaultHalfOpenInterval)
	if got := picker.Order(FamilyComment); got[0] != DefaultCommentHosts[0] || got[1] != DefaultCommentHosts[1] {
		t.Fatalf("after half-open order = %v states = %+v", got, picker.States())
	}
	picker.MarkGood(FamilyComment, DefaultCommentHosts[0])
	state := findState(t, picker.States(), FamilyComment, DefaultCommentHosts[0])
	if state.Health != -3 || state.ConsecutiveFailures != 0 {
		t.Fatalf("recovered state = %+v", state)
	}
}

func TestHostPickerFamiliesAreIsolated(t *testing.T) {
	picker := NewHostPicker(nil)
	picker.MarkBad(FamilyComment, DefaultCommentHosts[0])
	if got := picker.Order(FamilyPlayback); !reflect.DeepEqual(got, DefaultPlaybackHosts) {
		t.Fatalf("playback order changed: %v", got)
	}
	if got := picker.Order(FamilyComment); got[0] != DefaultCommentHosts[1] {
		t.Fatalf("comment order = %v", got)
	}
}

func TestHostPickerPersistenceRoundTrip(t *testing.T) {
	store := &memoryStateStore{}
	picker := NewHostPicker(store)
	picker.SetNow(func() time.Time {
		return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	})
	picker.MarkBad(FamilyPlayback, DefaultPlaybackHosts[0])
	picker.MarkGood(FamilyPlayback, DefaultPlaybackHosts[1])

	restored := NewHostPicker(store)
	if err := restored.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got, want := picker.Order(FamilyPlayback), restored.Order(FamilyPlayback); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored order = %v, want %v", got, want)
	}
}

func TestHostPickerPersistsAllFamiliesAndHosts(t *testing.T) {
	store := &memoryStateStore{}
	picker := NewHostPicker(store)
	picker.MarkBad(FamilyComment, DefaultCommentHosts[0])
	restored := NewHostPicker(store)
	if err := restored.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if len(restored.States()) != len(DefaultPlaybackHosts)+len(DefaultCommentHosts) {
		t.Fatalf("restored states = %+v", restored.States())
	}
}

func TestHostPickerRestoreIgnoresUnknownHosts(t *testing.T) {
	store := &memoryStateStore{states: []HostState{{
		Family: FamilyPlayback, Host: "unknown.example", Health: -20,
	}}}
	picker := NewHostPicker(store)
	if err := picker.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got := picker.Order(FamilyPlayback); !reflect.DeepEqual(got, DefaultPlaybackHosts) {
		t.Fatalf("order = %v", got)
	}
}

func TestHostPickerRestoreDecaysStaleBadState(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	store := &memoryStateStore{states: []HostState{{
		Family: FamilyPlayback,
		Host:   DefaultPlaybackHosts[0],
		Health: -20, ConsecutiveFailures: 5,
		LastFailureAt: now.Add(-2 * DefaultStaleAge).Unix(),
	}}}
	picker := NewHostPicker(store)
	picker.SetNow(func() time.Time { return now })
	if err := picker.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got := picker.Order(FamilyPlayback); got[0] != DefaultPlaybackHosts[0] {
		t.Fatalf("order after decay = %v", got)
	}
}

func TestHostPickerPersistenceFailureIsIgnoredByMarking(t *testing.T) {
	store := &memoryStateStore{err: errors.New("disk unavailable")}
	picker := NewHostPicker(store)
	picker.MarkBad(FamilyPlayback, DefaultPlaybackHosts[0])
	if got := picker.Order(FamilyPlayback); got[0] != DefaultPlaybackHosts[1] {
		t.Fatalf("order after failed persistence = %v", got)
	}
}

func findState(t *testing.T, states []HostState, family Family, host string) HostState {
	t.Helper()
	for _, state := range states {
		if state.Family == family && state.Host == host {
			return state
		}
	}
	t.Fatalf("state %s/%s not found", family, host)
	return HostState{}
}
