package main

import (
	"testing"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestSanitizeIPCRemovesInvalidUTF8AcrossNestedDTOs(t *testing.T) {
	value := RankResponse{
		Contents: []model.RankContent{
			{ID: "all", Name: "全部\xff", Boards: []model.RankBoard{{ID: "hot", Name: "热播\x80"}}},
		},
		Content: "all",
		Board:   "hot",
		Items: []RankItemDTO{{
			SeriesID: "100",
			Title:    "榜单剧\xff",
			Actor:    "演员\x80",
			Details:  []string{"67部作品\xff"},
		}},
	}

	got := sanitizeIPC(value)
	if got.Contents[0].Name != "全部" || got.Contents[0].Boards[0].Name != "热播" {
		t.Fatalf("contents were not sanitized: %+v", got.Contents)
	}
	if got.Items[0].Title != "榜单剧" || got.Items[0].Actor != "演员" {
		t.Fatalf("rank item was not sanitized: %+v", got.Items[0])
	}
	if got.Items[0].Details[0] != "67部作品" {
		t.Fatalf("rank details were not sanitized: %+v", got.Items[0].Details)
	}
}

func TestSanitizeIPCSanitizesMapsAndPointerAliases(t *testing.T) {
	payload := &map[string]any{
		"name\xff": []any{"A\xff", map[string]string{"nested\xff": "B\xff"}},
	}

	got := sanitizeIPC(payload)
	items, ok := (*got)["name"].([]any)
	if !ok {
		t.Fatalf("invalid map key was not replaced: %#v", *got)
	}
	nested := items[1].(map[string]string)
	if nested["nested"] != "B" {
		t.Fatalf("nested map text was not sanitized: %#v", nested)
	}
}
