package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractBVID(t *testing.T) {
	cases := map[string]string{
		"BV1ABcsztEcY":                                      "BV1ABcsztEcY",
		"https://www.bilibili.com/video/BV1ABcsztEcY?p=1": "BV1ABcsztEcY",
	}
	for input, expected := range cases {
		got, err := ExtractBVID(input)
		if err != nil || got != expected {
			t.Fatalf("ExtractBVID(%q) = %q, %v", input, got, err)
		}
	}
	if _, err := ExtractBVID("BV123"); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("invalid BV error code = %q", CodeOf(err))
	}
}

func TestFormatSubtitleTimeline(t *testing.T) {
	items := []map[string]any{
		{"from": 0.0, "to": 2.5, "content": "first"},
		{"from": 2.5, "to": 5.0, "content": "second"},
	}
	got := FormatSubtitleTimeline(items, "srt")
	expected := "1\n00:00:00,000 --> 00:00:02,500\nfirst\n\n2\n00:00:02,500 --> 00:00:05,000\nsecond\n"
	if got != expected {
		t.Fatalf("unexpected SRT output: %q", got)
	}
}

func TestClientUnwrapsDataAndMapsErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/x/web-interface/view" {
			if got := r.URL.Query().Get("bvid"); got != "BV1ABcsztEcY" {
				t.Fatalf("unexpected bvid: %s", got)
			}
			fmt.Fprint(w, `{"code":0,"data":{"title":"demo","aid":1}}`)
			return
		}
		fmt.Fprint(w, `{"code":-101,"message":"login required"}`)
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	info, err := client.GetVideoInfo(context.Background(), "BV1ABcsztEcY", nil)
	if err != nil || info["title"] != "demo" {
		t.Fatalf("GetVideoInfo() = %#v, %v", info, err)
	}
	var ignored map[string]any
	err = client.Request(context.Background(), http.MethodGet, "/failure", nil, nil, nil, &ignored)
	if CodeOf(err) != CodeNotAuthenticated {
		t.Fatalf("error code = %q, error = %v", CodeOf(err), err)
	}
}

func TestUserInfoWBIFlow(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/frontend/finger/spi":
			fmt.Fprint(w, `{"code":0,"data":{"b_3":"device3","b_4":"device4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			if !strings.Contains(r.Header.Get("Cookie"), "buvid_fp=") {
				t.Fatal("device activation omitted buvid_fp")
			}
			fmt.Fprint(w, `{"code":0}`)
		case "/946974/dynamic":
			fmt.Fprint(w, `<html><script id="__RENDER_DATA__" type="application/json">{"access_id":"render-id"}</script></html>`)
		case "/x/web-interface/nav":
			fmt.Fprintf(w, `{"code":-101,"data":{"wbi_img":{"img_url":"%s/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"%s/wbi/fedcba9876543210fedcba9876543210.png"}}}`, serverURL, serverURL)
		case "/x/space/wbi/acc/info":
			if r.URL.Query().Get("w_rid") == "" || r.URL.Query().Get("wts") == "" {
				t.Fatal("WBI signature fields missing")
			}
			if r.URL.Query().Get("w_webid") != "render-id" {
				t.Fatal("render access id missing")
			}
			if !strings.Contains(r.Header.Get("Cookie"), "buvid3=device3") {
				t.Fatal("device cookie missing")
			}
			fmt.Fprint(w, `{"code":0,"data":{"mid":946974,"name":"demo"}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	t.Setenv("BILI_SPACE_BASE_URL", server.URL)
	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	info, err := client.GetUserInfo(context.Background(), 946974, nil)
	if err != nil || info["name"] != "demo" {
		t.Fatalf("GetUserInfo() = %#v, %v", info, err)
	}
}

func TestMurmurFingerprint(t *testing.T) {
	if got := murmurFingerprint([]byte("hello"), 0); got != "cbd8a7b341bd9b025b1e906a48ae1d19" {
		t.Fatalf("murmur fingerprint = %s", got)
	}
}

func TestFindAccessID(t *testing.T) {
	body := `<script id="__RENDER_DATA__" type="application/json">{"props":{"access_id":"token"}}</script>`
	if got := findAccessID(body); got != "token" {
		t.Fatalf("access id = %q", got)
	}
}
