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

func TestSearchForwardsTypeOrderAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/web-interface/wbi/search/type" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("search_type") != "media_ft" || query.Get("order") != "stow" || query.Get("page") != "2" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.Header.Get("Origin") != "https://search.bilibili.com" || !strings.Contains(r.Referer(), "media_ft") {
			t.Fatalf("unexpected search headers: %#v", r.Header)
		}
		fmt.Fprint(w, `{"code":0,"data":{"result":[{"season_id":7,"title":"demo"}]}}`)
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	client.wbiKey = strings.Repeat("a", 32)
	client.wbiExpires = time.Now().Add(time.Hour)
	items, err := client.Search(context.Background(), "topic", SearchOptions{
		Type:  SearchTypeMedia,
		Order: SearchOrder("最多收藏"),
		Page:  2,
	})
	if err != nil || len(items) != 1 {
		t.Fatalf("Search() = %#v, %v", items, err)
	}
}

func TestSearchTypeAndOrderAliases(t *testing.T) {
	typeCase := []struct {
		input string
		want  SearchType
		api   string
	}{
		{"article", SearchTypeArticle, "article"},
		{"视频", SearchTypeVideo, "video"},
		{"bili_user", SearchTypeUser, "bili_user"},
		{"bangumi", SearchTypeBangumi, "media_bangumi"},
		{"直播", SearchTypeLive, "live_room"},
		{"media_ft", SearchTypeMedia, "media_ft"},
	}
	for _, testCase := range typeCase {
		got, ok := ParseSearchType(testCase.input)
		if !ok || got != testCase.want {
			t.Fatalf("ParseSearchType(%q) = %q, %v", testCase.input, got, ok)
		}
		apiValue, ok := got.apiValue()
		if !ok || apiValue != testCase.api {
			t.Fatalf("apiValue(%q) = %q, %v", got, apiValue, ok)
		}
	}

	orderCase := []struct {
		input string
		want  SearchOrder
	}{
		{"totalrank", SearchOrderComprehensive},
		{"最多播放", SearchOrderMostPlayed},
		{"pubdate", SearchOrderLatest},
		{"最多弹幕", SearchOrderMostDanmaku},
		{"stow", SearchOrderMostFavorite},
	}
	for _, testCase := range orderCase {
		got, ok := ParseSearchOrder(testCase.input)
		if !ok || got != testCase.want {
			t.Fatalf("ParseSearchOrder(%q) = %q, %v", testCase.input, got, ok)
		}
	}
}

func TestSearchRejectsUnsupportedOptions(t *testing.T) {
	client := NewClient()
	_, err := client.Search(context.Background(), "topic", SearchOptions{Type: SearchType("unknown")})
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("Search() error code = %q", CodeOf(err))
	}
}
