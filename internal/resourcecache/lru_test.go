package resourcecache

import (
	"strings"
	"testing"
	"time"
)

func TestLRUEvictsOldestAndReadsNewEntry(t *testing.T) {
	cache := New(2)
	cache.Set("first", Item{Value: "old"})
	cache.Set("second", Item{Value: "keep"})
	cache.Set("third", Item{Value: "new", ExpiresAt: time.Now().Add(time.Minute)})

	if _, ok := cache.Get("first"); ok {
		t.Fatal("oldest entry was not evicted")
	}
	if item, ok := cache.Get("third"); !ok || item.Value != "new" {
		t.Fatalf("new entry = %#v, %v", item, ok)
	}
}

func TestLRURejectsOversizedKey(t *testing.T) {
	cache := New(2)
	key := strings.Repeat("k", MaxKeyLength+1)
	if cache.Set(key, Item{Value: "value"}) {
		t.Fatal("oversized key was accepted")
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("oversized key was readable")
	}
}
