package library

import "testing"

func TestDecodeRemoteHistoryShapes(t *testing.T) {
	for _, body := range []string{
		`{"code":0,"data":{"data_list":[{"series_id":"s","vid_index":2}],"has_more":true,"next_offset":50}}`,
		`{"code":0,"data":[{"series_id":"s","vid_index":2}],"has_more":true,"next_offset":50}`,
	} {
		page, err := decodeRemoteHistoryPage([]byte(body))
		if err != nil || len(page.Items) != 1 || page.Items[0].SeriesID != "s" || !page.HasMore || page.NextOffset != 50 {
			t.Fatalf("%s: %+v %v", body, page, err)
		}
	}
	for _, body := range []string{`{"data":[]}`, `{"data":{"data_list":[]}}`} {
		page, err := decodeRemoteHistoryPage([]byte(body))
		if err != nil || len(page.Items) != 0 {
			t.Fatalf("empty: %+v %v", page, err)
		}
	}
	for _, body := range []string{`{"code":1,"data":[]}`, `{"data":null}`, `{}`, `{"data":{}}`, `{"data":"bad"}`, `{"data":[1]}`} {
		if _, err := decodeRemoteHistoryPage([]byte(body)); err == nil {
			t.Fatalf("accepted invalid response %s", body)
		}
	}
}
