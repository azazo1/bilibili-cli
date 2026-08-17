package model

import "testing"

func TestNormalizeSearchResultIncludesPublishedAt(t *testing.T) {
	type testCase struct {
		kind string
		item map[string]any
		id   string
	}
	cases := []testCase{
		{"article", map[string]any{"id": 1, "title": "article", "pub_time": 1700000000}, "1"},
		{"video", map[string]any{"bvid": "BV1test", "title": "video", "pubdate": 1700000000, "video_review": 2, "favorites": 3}, "BV1test"},
		{"user", map[string]any{"mid": 2, "uname": "user"}, "2"},
		{"bangumi", map[string]any{"season_id": 3, "title": "bangumi", "pubtime": 1700000000}, "3"},
		{"live", map[string]any{"roomid": 4, "title": "live", "live_time": "2024-01-02 03:04"}, "4"},
		{"media", map[string]any{"season_id": 5, "title": "media", "pubtime": 1700000000}, "5"},
	}
	for _, testCase := range cases {
		got := NormalizeSearchResult(testCase.kind, testCase.item)
		if got["id"] != testCase.id {
			t.Fatalf("NormalizeSearchResult(%s) id = %#v", testCase.kind, got["id"])
		}
		if testCase.kind != "user" && got["published_at"] == "" {
			t.Fatalf("NormalizeSearchResult(%s) missing published_at: %#v", testCase.kind, got)
		}
	}
}

func TestNormalizeSearchVideoMapsSearchStats(t *testing.T) {
	got := NormalizeSearchVideo(map[string]any{
		"bvid":         "BV1test",
		"title":        "video",
		"video_review": 7,
		"favorites":    8,
		"pubdate":      1700000000,
	})
	if got["danmaku"] != 7 || got["favorite"] != 8 || got["published_at"] == "" {
		t.Fatalf("unexpected normalized video: %#v", got)
	}
}

func TestNormalizeSearchAllKeepsResultType(t *testing.T) {
	got := NormalizeSearchResult("all", map[string]any{
		"result_type": "video",
		"bvid":        "BV1test",
		"title":       "video",
	})
	if got["result_type"] != "video" || got["bvid"] != "BV1test" {
		t.Fatalf("unexpected normalized all result: %#v", got)
	}
}
