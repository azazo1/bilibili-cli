package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestRequestAppSignsQueryAndUsesAppBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/v2/view/like" || r.Header.Get("User-Agent") != appUserAgent {
			t.Fatalf("unexpected app request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("access_key") != "access-token" || r.Form.Get("appkey") != appKey || r.Form.Get("ts") == "" || r.Form.Get("sign") == "" {
			t.Fatalf("app signature fields missing: %v", r.Form)
		}
		fmt.Fprint(w, `{"code":0,"data":null}`)
	}))
	defer server.Close()
	client := NewClient()
	client.AppBaseURL = server.URL
	client.HTTP = server.Client()
	credential := &Credential{AccessKey: "access-token", Sessdata: "session", BiliJct: "csrf"}
	if err := client.RequestApp(context.Background(), http.MethodPost, "/x/v2/view/like", nil, mapForm("aid", "42"), credential, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitResponseIsNotRetried(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"code":-799,"message":"too frequent"}`)
	}))
	defer server.Close()
	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	if _, err := client.GetHotVideos(context.Background(), 1, 1); CodeOf(err) != CodeRateLimited {
		t.Fatalf("unexpected rate limit error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("rate limited request count = %d", requests)
	}
}

func TestLikeVideoUsesAppEndpointWithAccessKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			fmt.Fprint(w, `{"code":0,"data":{"aid":42}}`)
		case "/x/v2/view/like":
			if err := r.ParseForm(); err != nil || r.Form.Get("aid") != "42" || r.Form.Get("like") != "1" || r.Form.Get("access_key") != "access-token" || r.Form.Get("sign") == "" {
				t.Fatalf("unexpected app like form: %v", r.Form)
			}
			fmt.Fprint(w, `{"code":0,"data":null}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewClient()
	client.BaseURL = server.URL
	client.AppBaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	credential := &Credential{Sessdata: "session", BiliJct: "csrf", AccessKey: "access-token"}
	if err := client.LikeVideo(context.Background(), "BV1ABcsztEcY", false, credential); err != nil {
		t.Fatal(err)
	}
}

func mapForm(name, value string) url.Values {
	return url.Values{name: []string{value}}
}
