package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetUserVideosUsesAppCursorWithAccessKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/v2/space/archive/cursor" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("vmid") != "42" || query.Get("access_key") != "access-token" || query.Get("appkey") != appKey || query.Get("sign") == "" {
			t.Fatalf("unexpected app cursor query: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"code":0,"data":{"item":[{"bvid":"BV1ABcsztEcY","title":"demo","duration":60,"author":"up","play":9}]}}`)
	}))
	defer server.Close()
	client := NewClient()
	client.AppBaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	items, err := client.GetUserVideos(context.Background(), 42, 10, &Credential{Sessdata: "session", AccessKey: "access-token"})
	if err != nil || len(items) != 1 || stringValue(items[0]["bvid"]) != "BV1ABcsztEcY" {
		t.Fatalf("GetUserVideos() = %#v, %v", items, err)
	}
}
