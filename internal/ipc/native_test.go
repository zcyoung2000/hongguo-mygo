package ipc

import (
	"context"
	"testing"
)

func TestNativeFeaturesIPC(t *testing.T) {
	svc := &HongguoService{}

	// Test UpdateNowPlaying
	ok, err := svc.UpdateNowPlaying(context.Background(), UpdateNowPlayingArgs{
		Title:    "测试短剧",
		Series:   "测试剧集",
		Episode:  1,
		Duration: 120,
		Position: 10,
		Playing:  true,
		Rate:     1.0,
		CoverURL: "https://example.com/cover.jpg",
	})
	if err != nil {
		t.Fatalf("UpdateNowPlaying failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected UpdateNowPlaying to return true")
	}

	// Test ClearNowPlaying
	ok, err = svc.ClearNowPlaying(context.Background())
	if err != nil {
		t.Fatalf("ClearNowPlaying failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClearNowPlaying to return true")
	}

	// Test SetPlaybackKeepAwake
	ok, err = svc.SetPlaybackKeepAwake(context.Background(), true)
	if err != nil {
		t.Fatalf("SetPlaybackKeepAwake(true) failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected SetPlaybackKeepAwake to return true")
	}

	ok, err = svc.SetPlaybackKeepAwake(context.Background(), false)
	if err != nil {
		t.Fatalf("SetPlaybackKeepAwake(false) failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected SetPlaybackKeepAwake to return true")
	}

	// Test SendNativeNotification (gracefully handles unpackaged test binary)
	_, err = svc.SendNativeNotification(context.Background(), NotificationArgs{
		Title:    "测试通知",
		Body:     "这是一个单元测试通知",
		SeriesID: "12345",
	})
	if err != nil {
		t.Fatalf("SendNativeNotification returned unexpected error: %v", err)
	}

	// Test TogglePictureInPicture
	ok, err = svc.TogglePictureInPicture(context.Background())
	if err != nil {
		t.Fatalf("TogglePictureInPicture returned unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected TogglePictureInPicture to return true")
	}
}
