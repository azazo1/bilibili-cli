package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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

func TestGetVideoDownloadURLsForPage(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/player/pagelist":
			fmt.Fprint(w, `{"code":0,"data":[{"cid":41,"part":"first"},{"cid":42,"part":"second"}]}`)
		case "/x/web-interface/nav":
			fmt.Fprintf(w, `{"code":0,"data":{"wbi_img":{"img_url":"%s/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"%s/wbi/fedcba9876543210fedcba9876543210.png"}}}`, serverURL, serverURL)
		case "/x/player/wbi/playurl":
			if r.URL.Query().Get("cid") != "42" {
				t.Fatalf("unexpected cid: %s", r.URL.Query().Get("cid"))
			}
			fmt.Fprint(w, `{"code":0,"data":{"dash":{"audio":[{"base_url":"https://cdn.example/audio.m4s"}],"video":[{"base_url":"https://cdn.example/video.m4s"}]}}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	urls, err := client.GetVideoDownloadURLsForPage(context.Background(), "BV1ABcsztEcY", 2, &Credential{Sessdata: "session"})
	if err != nil {
		t.Fatal(err)
	}
	if urls.Page != 2 || urls.PageCount != 2 || urls.PartTitle != "second" || len(urls.Pages) != 2 || urls.Pages[1].Title != "second" {
		t.Fatalf("unexpected page metadata: %#v", urls)
	}
}
