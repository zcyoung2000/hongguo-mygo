package catalog

import (
	"testing"

	"reflect"
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
						"user_info": {"base_info": {"user_name": "Actor", "user_id": 7100000000000201}},
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
	if rank.Items[1].ActorID != "7100000000000201" || rank.Items[1].Details[0] != "1部作品" {
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
