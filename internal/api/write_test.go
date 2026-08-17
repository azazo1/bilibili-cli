package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPostTextDynamicUsesSignedJSONRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/nav":
			fmt.Fprint(w, `{"code":-101,"data":{"wbi_img":{"img_url":"https://example.test/0123456789abcdef0123456789abcdef.png","sub_url":"https://example.test/fedcba9876543210fedcba9876543210.png"}}}`)
		case "/x/dynamic/feed/create/dyn":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.URL.Query().Get("w_rid") == "" {
				t.Fatal("dynamic request is not signed JSON")
			}
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil || payload["dyn_req"] == nil {
				t.Fatal("dynamic payload is missing dyn_req")
			}
			fmt.Fprint(w, `{"code":0,"data":{"dynamic_id":123}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	credential := &Credential{Sessdata: "sess", BiliJct: "csrf"}
	result, err := client.PostTextDynamic(context.Background(), "hello", credential)
	if err != nil || intValue(result["dynamic_id"], 0) != 123 {
		t.Fatalf("PostTextDynamic() = %#v, %v", result, err)
	}
}

func TestDeleteDynamicUsesVCAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dynamic_svr/v1/dynamic_svr/rm_dynamic" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("dynamic_id") != "123" || values.Get("csrf") != "csrf" {
			t.Fatalf("unexpected form: %s", strings.TrimSpace(string(body)))
		}
		fmt.Fprint(w, `{"code":0,"data":null}`)
	}))
	defer server.Close()
	client := NewClient()
	client.VCBaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	credential := &Credential{Sessdata: "sess", BiliJct: "csrf"}
	if err := client.DeleteDynamic(context.Background(), 123, credential); err != nil {
		t.Fatal(err)
	}
}

