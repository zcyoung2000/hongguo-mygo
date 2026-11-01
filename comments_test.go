package main

import "testing"

func TestParseCommentPage(t *testing.T) {
	raw := []byte(`{
		"code": 0,
		"data": {
			"common_list_info": {
				"total": 24,
				"has_more": true,
				"cursor": "{\"offset\":20,\"session_id\":\"next-session\"}"
			},
			"data_list": [{
				"comment": {
					"comment_id": "c1",
					"common": {
						"group_id": "7100000000001702",
						"status": 1,
						"content": {"text": "好看"},
						"user_info": {"base_info": {"user_name": "Alice", "user_avatar": "http://img.invalid/a.jpg"}},
						"create_timestamp": 1760000000
					},
					"expand": {"book_id": "7100000000001701"},
					"stat": {"digg_count": 3, "reply_count": 1}
				}
			}, {
				"comment": {
					"comment_id": "c2",
					"common": {"group_id": "7100000000009999", "status": 1, "content": {"text": "wrong group"}},
					"expand": {"book_id": "7100000000001701"}
				}
			}]
		}
	}`)

	page, err := parseCommentPage(raw, "7100000000001701", "7100000000001702", "")
	if err != nil {
		t.Fatalf("parseCommentPage err = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "c1" || page.Items[0].Author != "Alice" {
		t.Fatalf("items = %+v", page.Items)
	}
	if page.Total != 24 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("page = %+v", page)
	}
}

func TestParseReplyPage(t *testing.T) {
	raw := []byte(`{
		"code": 0,
		"data": {
			"comment": {
				"common": {"group_id": "7100000000001702"},
				"expand": {"book_id": "7100000000001701"}
			},
			"comment_list_info": {
				"total": 3,
				"has_more": true,
				"cursor": "10"
			},
			"reply_list": [{
				"reply_id": "r1",
				"common": {
					"content": {"text": "同意"},
					"user_info": {"base_info": {"user_name": "Bob", "user_avatar": "http://img.invalid/b.jpg"}},
					"create_timestamp": 1760000100
				},
				"stat": {"digg_count": 2},
				"reply_to_user_info": {"base_info": {"user_name": "Alice"}}
			}]
		}
	}`)

	page, err := parseReplyPage(raw, "7100000000001701", "7100000000001702", "c1", "")
	if err != nil {
		t.Fatalf("parseReplyPage err = %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %+v", page.Items)
	}
	reply := page.Items[0]
	if reply.ID != "r1" || reply.Author != "Bob" || reply.Content != "同意" || reply.ReplyTo != "Alice" {
		t.Fatalf("reply = %+v", reply)
	}
	if page.Total != 3 || !page.HasMore || page.NextCursor != "10" {
		t.Fatalf("page = %+v", page)
	}
}
