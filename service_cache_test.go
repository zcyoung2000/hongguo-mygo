package main

import (
	"os"
	"sync"
	"testing"
)

func TestHistoryCacheInvalidationIsSynchronized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hg_cache_race_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	svc := &HongguoService{store: store, media: &MediaServer{port: 1}}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := svc.SaveProgress("710000000000001", "710000000000101", "Race", "", 1, 10, 1, 10); err != nil {
				t.Errorf("SaveProgress failed: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if _, err := svc.GetHistory(); err != nil {
				t.Errorf("GetHistory failed: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}

func TestConcurrentCredentialAndMediaCallbackAccess(t *testing.T) {
	svc := &HongguoService{}
	ms := &MediaServer{}
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			svc.setAuthCredentials("cookie", "token")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _ = svc.authCredentials()
		}
	}()
	go func() {
		defer wg.Done()
		callback := func(string, string, int, string) error { return nil }
		ms.SetEpisodeCacheCallback(callback)
	}()
	go func() {
		defer wg.Done()
		ms.rememberCachedEpisode("series", "episode", 1, "720p")
	}()
	wg.Wait()
}
