package store

import (
	"testing"

	"myguo/internal/upstream"
)

func TestUpstreamHostStatePersistenceRoundTrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	picker := upstream.NewHostPicker(store.UpstreamStateStore())
	picker.MarkBad(upstream.FamilyPlayback, upstream.DefaultPlaybackHosts[0])
	picker.MarkGood(upstream.FamilyComment, upstream.DefaultCommentHosts[2])

	restored := upstream.NewHostPicker(store.UpstreamStateStore())
	if err := restored.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got, want := picker.Order(upstream.FamilyPlayback), restored.Order(upstream.FamilyPlayback); got[0] != want[0] {
		t.Fatalf("playback order = %v, want %v", got, want)
	}
	if got, want := picker.Order(upstream.FamilyComment), restored.Order(upstream.FamilyComment); got[0] != want[0] {
		t.Fatalf("comment order = %v, want %v", got, want)
	}
}
