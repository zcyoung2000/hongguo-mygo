package catalog

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"myguo/internal/resourcecache"
)

func TestParseAccountShortRankBuildsItemsAndCursor(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"data": {
			"has_more": true,
			"next_offset": 2,
			"session_id": "session",
			"cell_view": {
				"cell_selector": {"outer_row": {"items": [
					{"show_name": "全部", "selector_item_id": "all", "is_selected": true, "sub_cell_selector": {"outer_row": {"items": [
						{"show_name": "推荐榜", "selector_item_id": "ranklist_hot_sc", "is_selected": true}
					]}}}
				]}},
				"cell_data": [{
					"video_data": [{
						"series_id": 7100000000000101,
						"title": "Ranked One",
						"cover": "https://example.test/one.jpg",
						"episode_cnt": 90,
						"rec_text_item": {"RecommendText": "992万推荐"}
					}],
					"ugc_user_data": [{
						"user_info": {"base_info": {"user_name": "Actor", "user_id": 7100000000000201, "actor_id_str": 7100000000000301}},
						"rec_text_item": {"RecommendText": "999万人气"},
						"intro": "作品一",
						"sub_title_list": [{"content": "1部作品"}]
					}]
				}]
			}
		}
	}`)

	rank, err := parseAccountShortRank(body, "all", "ranklist_hot_sc", rankCursor{})
	if err != nil {
		t.Fatalf("parseAccountShortRank() error = %v", err)
	}
	if rank.Content != "all" || rank.Board != "ranklist_hot_sc" {
		t.Fatalf("selection = %s/%s, want all/ranklist_hot_sc", rank.Content, rank.Board)
	}
	if len(rank.Items) != 2 || rank.Items[0].SeriesID != "7100000000000101" || rank.Items[0].EpisodeCount != 90 {
		t.Fatalf("unexpected video item: %+v", rank.Items)
	}
	if rank.Items[1].UserID != "7100000000000201" || rank.Items[1].ActorID != "7100000000000301" || rank.Items[1].Details[0] != "1部作品" {
		t.Fatalf("unexpected actor item: %+v", rank.Items[1])
	}
	if rank.Cursor == "" || !rank.HasMore {
		t.Fatalf("expected a continuation cursor, got cursor=%q hasMore=%v", rank.Cursor, rank.HasMore)
	}
	next, err := decodeRankCursor(rank.Cursor)
	if err != nil {
		t.Fatalf("decodeRankCursor() error = %v", err)
	}
	if next.Offset != 2 || next.Session != "session" || next.Content != "all" || next.Board != "ranklist_hot_sc" {
		t.Fatalf("unexpected cursor: %+v", next)
	}
}

func TestInvalidatePrefixesOnlyRemovesMatchingEntries(t *testing.T) {
	service := NewService(nil, nil, nil, Dependencies{})
	service.cache.Set("new_releases:today", resourcecache.Item{Value: "stale"})
	service.cache.Set("new_releases:tomorrow", resourcecache.Item{Value: "stale"})
	service.cache.Set("hot_search", resourcecache.Item{Value: "keep", ExpiresAt: time.Now().Add(time.Minute)})

	service.InvalidatePrefixes("new_releases:")

	if _, ok := service.cache.Get("new_releases:today"); ok {
		t.Fatal("matching entry was not invalidated")
	}
	if _, ok := service.cache.Get("new_releases:tomorrow"); ok {
		t.Fatal("matching entry was not invalidated")
	}
	if item, ok := service.cache.Get("hot_search"); !ok || item.Value != "keep" {
		t.Fatalf("unmatched entry = %#v, %v", item, ok)
	}
}

func TestExpiredCatalogCacheIsNotServed(t *testing.T) {
	service := NewService(nil, nil, nil, Dependencies{})
	service.cache.Set("expired", resourcecache.Item{Value: "old", ExpiresAt: time.Now().Add(-time.Second)})
	service.cache.Set("fresh", resourcecache.Item{Value: "new", ExpiresAt: time.Now().Add(time.Minute)})

	expired, expiredOK := service.cache.Get("expired")
	fresh, freshOK := service.cache.Get("fresh")
	if expiredOK {
		t.Fatal("expired entry is still considered valid")
	}
	if !freshOK {
		t.Fatal("fresh entry was rejected")
	}
	if fresh.Value != "new" || expired.Value != nil {
		t.Fatalf("cache vals = %v/%v", expired.Value, fresh.Value)
	}
}

func TestCatalogMetadataCacheEvictsAndRejectsOversizedKeys(t *testing.T) {
	service := NewService(nil, nil, nil, Dependencies{})
	keys := make([]string, 0, metadataCacheCapacity+1)
	for index := 0; index <= metadataCacheCapacity; index++ {
		key := fmt.Sprintf("catalog:%04d", index)
		keys = append(keys, key)
		service.cache.Set(key, resourcecache.Item{Value: index, ExpiresAt: time.Now().Add(time.Minute)})
	}
	if service.cache.Len() != metadataCacheCapacity {
		t.Fatalf("cache length = %d, want %d", service.cache.Len(), metadataCacheCapacity)
	}
	if _, ok := service.cache.Get(keys[0]); ok {
		t.Fatal("oldest entry was not evicted")
	}
	if item, ok := service.cache.Get(keys[len(keys)-1]); !ok || item.Value != metadataCacheCapacity {
		t.Fatalf("newest entry = %#v, %v", item, ok)
	}

	oversized := make([]byte, resourcecache.MaxKeyLength+1)
	if service.cache.Set(string(oversized), resourcecache.Item{Value: "rejected"}) {
		t.Fatal("oversized cache key was accepted")
	}
}

func TestShortFilterSelections(t *testing.T) {
	tests := []struct {
		name   string
		genre  string
		values map[string]string
		want   map[string][]string
	}{
		{
			name:   "uses every selected filter row",
			genre:  "short_play",
			values: map[string]string{"category_dim_role": "cate_26", "category_dim_epoch": "cate_127", "sort": ""},
			want: map[string][]string{
				"genre":              {"short_play"},
				"category_dim_role":  {"cate_26"},
				"category_dim_epoch": {"cate_127"},
			},
		},
		{
			name:   "frontend genre overrides any stale row value",
			genre:  "comic_series",
			values: map[string]string{"genre": "short_play"},
			want:   map[string][]string{"genre": {"comic_series"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortFilterSelections(tt.genre, tt.values)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("shortFilterSelections() = %v, want %v", got, tt.want)
			}
		})
	}
}
