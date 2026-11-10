package media

import (
	"context"
	"errors"
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
	client := &playbackClient{err: sdk.ErrAnonymousLimited}
	server.setPrimaryPlaybackResolver(client)
	server.resetDeviceForTest = func(context.Context) error { return errors.New("reset unavailable") }
	if _, err := server.resolvePlaybackForSession(context.Background(), "episode", "session"); !errors.Is(err, sdk.ErrAnonymousLimited) {
		t.Fatalf("err = %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("calls = %d", client.calls)
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
	primary := &playbackClient{err: sdk.ErrAnonymousLimited}
	server.setPrimaryPlaybackResolver(primary)
	server.resetDeviceForTest = func(context.Context) error { return errors.New("must not reset") }
	got, err := server.resolvePlaybackForSession(context.Background(), "episode", "session")
	if err != nil || got.EpisodeID != "spare-episode" {
		t.Fatalf("got = %+v err = %v", got, err)
	}
	if primary.calls != 1 {
		t.Fatalf("primary called %d times", primary.calls)
	}
}
