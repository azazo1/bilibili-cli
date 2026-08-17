package cli

import (
	"testing"

	"github.com/azazo1/bilibili-cli/internal/api"
)

func TestPreferredSubtitleTracksUsesLanguageThenManualPriority(t *testing.T) {
	tracks := []api.SubtitleTrack{
		{ID: "en-manual", Language: "en-US"},
		{ID: "zh-ai", Language: "zh-CN", Type: 1},
		{ID: "other-manual", Language: "ja-JP"},
		{ID: "zh-manual", Language: "zh-CN"},
		{ID: "en-ai", Language: "en-US", Type: 1},
	}
	ordered := preferredSubtitleTracks(tracks)
	ids := make([]string, 0, len(ordered))
	for _, track := range ordered {
		ids = append(ids, track.ID)
	}
	expected := []string{"zh-manual", "zh-ai", "en-manual", "en-ai", "other-manual"}
	for index := range expected {
		if ids[index] != expected[index] {
			t.Fatalf("unexpected subtitle order: %#v", ids)
		}
	}
}

func TestPreferredSubtitleTracksKeepsSingleTrack(t *testing.T) {
	tracks := []api.SubtitleTrack{{ID: "only", Language: "fr-FR", Type: 1}}
	ordered := preferredSubtitleTracks(tracks)
	if len(ordered) != 1 || ordered[0].ID != "only" {
		t.Fatalf("unexpected single subtitle selection: %#v", ordered)
	}
}
