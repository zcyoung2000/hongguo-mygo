package ipc

import (
	"context"
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

	svc := &HongguoService{store: store, media: &MediaServer{}}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := svc.SaveProgress(context.Background(), "710000000000001", "710000000000101", "Race", "", 1, 10, 1, 10); err != nil {
				t.Errorf("SaveProgress failed: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if _, err := svc.GetHistory(context.Background()); err != nil {
				t.Errorf("GetHistory failed: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}

func TestConcurrentCredentialAndMediaCallbackAccess(t *testing.T) {
	svc := &HongguoService{}
	var wg sync.WaitGroup
	wg.Add(2)
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
	wg.Wait()
}
