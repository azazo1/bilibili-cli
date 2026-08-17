package api

import "testing"

func TestVideoDownloadURLsFromPlayURL(t *testing.T) {
	urls := videoDownloadURLsFromPlayURL(map[string]any{
		"dash": map[string]any{
			"audio": []any{map[string]any{"base_url": "https://cdn.example/audio.m4s"}},
			"video": []any{map[string]any{"backupUrl": []any{map[string]any{"url": "https://cdn.example/video.m4s"}}}},
		},
	})
	if urls.AudioURL != "https://cdn.example/audio.m4s" || urls.VideoURL != "https://cdn.example/video.m4s" {
		t.Fatalf("unexpected DASH URLs: %#v", urls)
	}

	combined := videoDownloadURLsFromPlayURL(map[string]any{
		"durl": []any{map[string]any{"url": "https://cdn.example/video.mp4"}},
	})
	if combined.CombinedURL != "https://cdn.example/video.mp4" {
		t.Fatalf("unexpected combined URL: %#v", combined)
	}
}

func TestVideoDownloadURLsFromPlayURLAcceptsDirectURL(t *testing.T) {
	urls := videoDownloadURLsFromPlayURL(map[string]any{"url": "https://cdn.example/video.mp4"})
	if urls.CombinedURL != "https://cdn.example/video.mp4" {
		t.Fatalf("unexpected direct URL: %#v", urls)
	}
}
