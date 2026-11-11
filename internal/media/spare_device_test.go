package media

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/sparedevice"
	"myguo/internal/store"
)

type playbackClient struct {
	client *sdk.Client
	calls  int
	err    error
}

func (c *playbackClient) ResolvePlayback(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return &model.Playback{EpisodeID: "episode"}, nil
}

func TestSparePoolDisabledKeepsExistingPlaybackBehavior(t *testing.T) {
	server, err := newMediaServer(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	server.playbackResolver = fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		return nil, sdk.ErrAnonymousLimited
	}}
	server.resetDeviceForTest = func(context.Context) error { return errors.New("reset unavailable") }
	result, err := server.resolvePlaybackForSession(context.Background(), "7100000000000201", "session")
	if result != nil || err == nil || !strings.Contains(err.Error(), "playback client unavailable") {
		t.Fatalf("err = %v", err)
	}
}

func TestSparePoolResolvesWithoutChangingPrimaryIdentity(t *testing.T) {
	server, err := newMediaServer(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	state, err := store.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	if err := state.UpsertSpareDevice(store.SpareDevice{DeviceID: "did", InstallID: "iid"}); err != nil {
		t.Fatal(err)
	}
	manager := sparedevice.NewManager(state, sparedevice.WithClientFactory(func(sdk.ProfileStore) (sparedevice.PlaybackResolver, error) {
		return func(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error) {
			return &model.Playback{EpisodeID: "spare-episode"}, nil
		}, nil
	}))
	server.SetSpareDevicePool(manager)
	var primaryCalls int
	server.playbackResolver = fakePlaybackResolver{func(context.Context, string) (*playbackResolveResult, error) {
		primaryCalls++
		return nil, sdk.ErrAnonymousLimited
	}}
	server.resetDeviceForTest = func(context.Context) error { return errors.New("must not reset") }
	got, err := server.resolvePlaybackForSession(context.Background(), "episode", "session")
	if err != nil || got.EpisodeID != "spare-episode" {
		t.Fatalf("got = %+v err = %v", got, err)
	}
	if primaryCalls != 1 {
		t.Fatalf("primary called %d times", primaryCalls)
	}
}
