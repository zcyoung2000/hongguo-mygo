package ipc

import (
	"testing"
)

func TestParseDanmakuPage(t *testing.T) {
	raw := []byte(`{
		"code":0,
		"data":{
			"comment_list_info":{"next_query_danmaku_list_time":60000},
			"data_list":[{
				"comment":{
					"comment_id":"dm1",
					"common":{
						"group_id":"7001",
						"content":{"text":"hello"},
						"user_info":{"base_info":{"user_id":"42","user_name":"TestUser"}}
					},
					"expand":{"book_id":"9001","offset_time":31500,"show_digg_button":true},
					"stat":{"digg_count":2,"user_action":{"user_digg":true}}
				}
			}]
		}
	}`)
	page, err := parseDanmakuPage(raw, "9001", "7001", 30000, 30000)
	if err != nil {
		t.Fatalf("parseDanmakuPage() error = %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}
	item := page.Items[0]
	if item.ID != "dm1" || item.Text != "hello" || item.TimeMs != 31500 || item.LikeCount != 2 || !item.Liked || !item.ShowDigg {
		t.Fatalf("unexpected item: %+v", item)
	}
	if page.NextMs != 60000 {
		t.Fatalf("NextMs = %d, want 60000", page.NextMs)
	}
}

func TestParseDanmakuPageFallsBackToRequestedWindow(t *testing.T) {
	raw := []byte(`{"code":0,"data":{"data_list":[]}}`)
	page, err := parseDanmakuPage(raw, "9001", "7001", 120000, 30000)
	if err != nil {
		t.Fatalf("parseDanmakuPage() error = %v", err)
	}
	if page.NextMs != 150000 {
		t.Fatalf("NextMs = %d, want 150000", page.NextMs)
	}
}

func TestParseDanmakuPageRejectsWrongOwner(t *testing.T) {
	raw := []byte(`{
		"code":0,
		"data":{"data_list":[{"comment":{
			"comment_id":"dm2",
			"common":{"group_id":"7002","content":{"text":"wrong"}},
			"expand":{"book_id":"9001","offset_time":100}
		}}]}
	}`)
	page, err := parseDanmakuPage(raw, "9001", "7001", 0, 30000)
	if err != nil {
		t.Fatalf("parseDanmakuPage() error = %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("items = %d, want 0", len(page.Items))
	}
}

func TestParseDanmakuPageFiltersToRequestedWindow(t *testing.T) {
	raw := []byte(`{
		"code":0,
		"data":{"data_list":[
			{"comment":{"comment_id":"old","common":{"group_id":"7001","content":{"text":"old"}},"expand":{"book_id":"9001","offset_time":29000}}},
			{"comment":{"comment_id":"inside","common":{"group_id":"7001","content":{"text":"inside"}},"expand":{"book_id":"9001","offset_time":30000}}},
			{"comment":{"comment_id":"new","common":{"group_id":"7001","content":{"text":"new"}},"expand":{"book_id":"9001","offset_time":60000}}}
		]}
	}`)
	page, err := parseDanmakuPage(raw, "9001", "7001", 30000, 30000)
	if err != nil {
		t.Fatalf("parseDanmakuPage() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "inside" {
		t.Fatalf("items = %+v, want only inside", page.Items)
	}
}
