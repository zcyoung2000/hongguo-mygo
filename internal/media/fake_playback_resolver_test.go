package media

import "context"

type fakePlaybackResolver struct {
	fn func(ctx context.Context, episodeID string) (*playbackResolveResult, error)
}

func (r fakePlaybackResolver) ResolvePlayback(ctx context.Context, episodeID string) (*playbackResolveResult, error) {
	return r.fn(ctx, episodeID)
}
