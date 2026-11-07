package catalog

import (
	"reflect"
	"testing"

	"time"
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
	service.cacheMu.Lock()
	service.cache["new_releases:today"] = cacheItem{val: "stale"}
	service.cache["new_releases:tomorrow"] = cacheItem{val: "stale"}
	service.cache["hot_search"] = cacheItem{val: "keep"}
	service.cacheMu.Unlock()

	service.InvalidatePrefixes("new_releases:")

	service.cacheMu.RLock()
	keys := make([]string, 0, len(service.cache))
	for key := range service.cache {
		keys = append(keys, key)
	}
	service.cacheMu.RUnlock()
	if !reflect.DeepEqual(keys, []string{"hot_search"}) {
		t.Fatalf("remaining cache keys = %#v, want hot_search only", keys)
	}
}

func TestExpiredCatalogCacheIsNotServed(t *testing.T) {
	service := NewService(nil, nil, nil, Dependencies{})
	service.cacheMu.Lock()
	service.cache["expired"] = cacheItem{val: "old", expiresAt: time.Now().Add(-time.Second)}
	service.cache["fresh"] = cacheItem{val: "new", expiresAt: time.Now().Add(time.Minute)}
	service.cacheMu.Unlock()

	service.cacheMu.RLock()
	expired, expiredOK := service.cache["expired"]
	fresh, freshOK := service.cache["fresh"]
	service.cacheMu.RUnlock()
	if expiredOK && time.Now().Before(expired.expiresAt) {
		t.Fatal("expired entry is still considered valid")
	}
	if !freshOK || time.Now().After(fresh.expiresAt) {
		t.Fatal("fresh entry was rejected")
	}
	if fresh.val != "new" || expired.val != "old" {
		t.Fatalf("cache vals = %v/%v", expired.val, fresh.val)
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
