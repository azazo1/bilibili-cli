package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetAudioURLUsesWBIPlayURL(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/player/pagelist":
			fmt.Fprint(w, `{"code":0,"data":[{"cid":42}]}`)
		case "/x/web-interface/nav":
			fmt.Fprintf(w, `{"code":0,"data":{"wbi_img":{"img_url":"%s/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"%s/wbi/fedcba9876543210fedcba9876543210.png"}}}`, serverURL, serverURL)
		case "/x/player/wbi/playurl":
			query := r.URL.Query()
			if query.Get("cid") != "42" || query.Get("fnval") != "4048" || query.Get("w_rid") == "" || query.Get("wts") == "" {
				t.Fatalf("unexpected WBI playurl query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"code":0,"data":{"dash":{"audio":[{"base_url":"https://cdn.example/audio.m4s"}]}}}`)
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
	url, err := client.GetAudioURL(context.Background(), "BV1ABcsztEcY", &Credential{Sessdata: "session"})
	if err != nil || url != "https://cdn.example/audio.m4s" {
		t.Fatalf("GetAudioURL() = %q, %v", url, err)
	}
}
