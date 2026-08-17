package model

import "testing"

func TestNormalizeVideoSummary(t *testing.T) {
	value := map[string]any{
		"bvid":     "BV1ABcsztEcY",
		"title":    "<em>demo</em>",
		"duration": 125,
		"owner":    map[string]any{"mid": 42, "name": "owner"},
		"stat":     map[string]any{"view": 15000, "like": 10},
	}
	got := NormalizeVideoSummary(value)
	if got["title"] != "demo" || got["duration"] != "02:05" || got["url"] != "https://www.bilibili.com/video/BV1ABcsztEcY" {
		t.Fatalf("unexpected normalized video: %#v", got)
	}
	stats := Map(got["stats"])
	if stats["view"] != 15000 || stats["like"] != 10 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestNormalizeDynamicItemReadsLegacyCard(t *testing.T) {
	value := map[string]any{
		"desc": map[string]any{"dynamic_id": 9, "timestamp": 1700000000},
		"card": `{"item":{"content":"body"}}`,
	}
	got := NormalizeDynamicItem(value)
	if got["id"] != "9" || got["text"] != "body" || got["published_at"] == "" {
		t.Fatalf("unexpected normalized dynamic: %#v", got)
	}
}

