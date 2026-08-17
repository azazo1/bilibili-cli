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

func TestNormalizeVideoSummaryReadsCreatedTime(t *testing.T) {
	got := NormalizeVideoSummary(map[string]any{
		"bvid":    "BV1created",
		"title":   "video",
		"created": 1700000000,
	})
	if got["published_at"] == "" {
		t.Fatalf("published_at is missing: %#v", got)
	}
}

func TestNormalizeUserReadsNavLevelAndMoneyFields(t *testing.T) {
	got := NormalizeUser(map[string]any{
		"mid":  42,
		"uname": "owner",
		"money": 12.5,
		"wallet": map[string]any{"bcoin_balance": 3},
		"level_info": map[string]any{"current_level": 6},
	})
	if got["level"] != 6 || got["coins"] != 12 || got["bcoins"] != 3 {
		t.Fatalf("unexpected normalized user: %#v", got)
	}
}

func TestNormalizeUserPrefersNestedFieldsOverZeroPlaceholders(t *testing.T) {
	got := NormalizeUser(map[string]any{
		"mid":        42,
		"uname":      "owner",
		"level":      0,
		"money":      0,
		"coins":      12,
		"level_info": map[string]any{"current_level": 6},
	})
	if got["level"] != 6 || got["coins"] != 12 {
		t.Fatalf("unexpected placeholder fallback: %#v", got)
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
