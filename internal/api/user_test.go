package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestGetUserDynamicsUsesSignedSpaceRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/nav":
			fmt.Fprint(w, `{"code":0,"data":{"wbi_img":{"img_url":"https://example.test/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"https://example.test/wbi/fedcba9876543210fedcba9876543210.png"}}}`)
		case "/x/polymer/web-dynamic/v1/feed/space":
			query := r.URL.Query()
			if _, ok := query["offset"]; !ok || query.Get("offset") != "" || query.Get("host_mid") != "42" {
				t.Fatalf("unexpected pagination query: %s", r.URL.RawQuery)
			}
			if _, ok := query["need_top"]; ok {
				t.Fatalf("deprecated need_top query is present: %s", r.URL.RawQuery)
			}
			if query.Get("timezone_offset") != "-480" || query.Get("features") != userDynamicFeatures || query.Get("platform") != "web" || query.Get("web_location") != userDynamicWebLocation {
				t.Fatalf("missing dynamic page context: %s", r.URL.RawQuery)
			}
			if query.Get("x-bili-device-req-json") != `{"platform":"web","device":"pc","spmid":"333.1387"}` || query.Get("w_rid") == "" || query.Get("wts") == "" || query.Get("dm_img_str") == "" {
				t.Fatalf("unsigned dynamic request: %s", r.URL.RawQuery)
			}
			if r.Header.Get("Origin") != "https://space.bilibili.com" || r.Header.Get("Referer") != "https://space.bilibili.com/42/dynamic" || !strings.Contains(r.Header.Get("Cookie"), "SESSDATA=session") {
				t.Fatalf("unexpected dynamic request headers: %#v", r.Header)
			}
			fmt.Fprint(w, `{"code":0,"data":{"items":[{"id_str":"99"}]}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "device3", Buvid4: "device4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	data, err := client.GetUserDynamics(context.Background(), 42, 0, &Credential{Sessdata: "session"})
	if err != nil || len(mapList(data["items"])) != 1 {
		t.Fatalf("GetUserDynamics() = %#v, %v", data, err)
	}
}
