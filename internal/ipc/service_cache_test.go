package ipc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"myguo/internal/resourcecache"
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

func TestIPCResourceCacheEvictsAndRejectsOversizedKeys(t *testing.T) {
	svc := &HongguoService{cache: resourcecache.New(metadataCacheCapacity)}
	keys := make([]string, 0, metadataCacheCapacity+1)
	for index := 0; index <= metadataCacheCapacity; index++ {
		key := "series_page:" + string(rune('a'+index/26%26)) + string(rune('a'+index/676%26)) + string(rune('a'+index/17576%26)) + fmt.Sprintf("%04d", index)
		keys = append(keys, key)
		svc.cache.Set(key, resourcecache.Item{Value: index, ExpiresAt: time.Now().Add(time.Minute)})
	}
	if svc.cache.Len() != metadataCacheCapacity {
		t.Fatalf("cache length = %d, want %d", svc.cache.Len(), metadataCacheCapacity)
	}
	if _, ok := svc.cache.Get(keys[0]); ok {
		t.Fatal("oldest entry was not evicted")
	}
	if item, ok := svc.cache.Get(keys[len(keys)-1]); !ok || item.Value != metadataCacheCapacity {
		t.Fatalf("newest entry = %#v, %v", item, ok)
	}

	oversized := make([]byte, resourcecache.MaxKeyLength+1)
	if svc.cache.Set(string(oversized), resourcecache.Item{Value: "rejected"}) {
		t.Fatal("oversized cache key was accepted")
	}
}
