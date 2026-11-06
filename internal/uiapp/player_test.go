package uiapp

import (
	"context"

	"myguo/internal/ipc"
)

type noopPlayback struct{}

func (noopPlayback) PlayEpisode(context.Context, string, string, int, string, bool) (*ipc.PlaybackDTO, error) {
	return nil, nil
}
