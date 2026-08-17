package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveUserListReference(t *testing.T) {
	client := NewClient()
	cases := []struct {
		input    string
		ownerID  int64
		listID   int64
		kindHint UserListKind
	}{
		{input: "7855491", ownerID: 7855491},
		{input: "7855491/8565435", ownerID: 7855491, listID: 8565435},
		{input: "https://space.bilibili.com/7855491", ownerID: 7855491},
		{input: "https://space.bilibili.com/7855491/lists?sid=8565435", ownerID: 7855491, listID: 8565435},
		{input: "https://space.bilibili.com/7855491/lists/3610862?type=series", ownerID: 7855491, listID: 3610862, kindHint: userListKindSeries},
		{input: "https://www.bilibili.com/space/7855491/channel/collectiondetail?sid=8565435", ownerID: 7855491, listID: 8565435, kindHint: userListKindSeason},
	}
	for _, testCase := range cases {
		reference, err := client.ResolveUserListReference(context.Background(), testCase.input)
		if err != nil || reference.OwnerID != testCase.ownerID || reference.ListID != testCase.listID || reference.KindHint != testCase.kindHint {
			t.Fatalf("ResolveUserListReference(%q) = %#v, %v", testCase.input, reference, err)
		}
	}
	for _, input := range []string{"7855491/", "7855491/8565435/1", "https://example.com/7855491/lists?sid=8565435"} {
		if _, err := client.ResolveUserListReference(context.Background(), input); CodeOf(err) != CodeInvalidInput {
			t.Fatalf("ResolveUserListReference(%q) error = %v", input, err)
		}
	}
}

func TestResolveUserListReferenceFollowsB23Redirect(t *testing.T) {
	client := NewClient()
	client.HTTP = &http.Client{Transport: userListRoundTripper(func(request *http.Request) (*http.Response, error) {
		response := &http.Response{
			Header:  make(http.Header),
			Body:    io.NopCloser(strings.NewReader("")),
			Request: request,
		}
		switch request.URL.Host {
		case "b23.tv":
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://space.bilibili.com/7855491/lists?sid=3610862&type=series")
		case "space.bilibili.com":
			response.StatusCode = http.StatusOK
		default:
			t.Fatalf("unexpected host: %s", request.URL.Host)
		}
		return response, nil
	})}
	reference, err := client.ResolveUserListReference(context.Background(), "b23.tv/demo")
	if err != nil || reference.OwnerID != 7855491 || reference.ListID != 3610862 || reference.KindHint != userListKindSeries {
		t.Fatalf("ResolveUserListReference() = %#v, %v", reference, err)
	}
}

func TestGetUserListDirectoryFlattensListKinds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/x/polymer/web-space/home/seasons_series" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("mid") != "42" || query.Get("page_num") != "2" || query.Get("page_size") != "10" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		if request.Header.Get("Origin") != "https://space.bilibili.com" || request.Header.Get("Referer") != "https://space.bilibili.com/42" {
			t.Fatalf("unexpected headers: %#v", request.Header)
		}
		fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":2,"page_size":10,"total":3},"seasons_list":[{"meta":{"mid":42,"season_id":7,"title":"season","total":2}}],"series_list":[{"meta":{"mid":42,"series_id":9,"name":"series","total":3}}]}}}`)
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	directory, err := client.GetUserListDirectory(context.Background(), 42, 2, nil)
	if err != nil || directory.OwnerID != 42 || directory.Page.Number != 2 || directory.Page.Size != 10 || directory.Page.Total != 3 {
		t.Fatalf("GetUserListDirectory() = %#v, %v", directory, err)
	}
	if len(directory.Items) != 2 || directory.Items[0].ID != 7 || directory.Items[1].ID != 9 || directory.Items[1].Title != "series" {
		t.Fatalf("unexpected directory items: %#v", directory.Items)
	}
}

func TestGetUserListUsesResolvedSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/polymer/web-space/home/seasons_series":
			fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":1,"page_size":10,"total":1},"seasons_list":[],"series_list":[{"meta":{"mid":42,"series_id":9,"name":"series","total":31}}]}}}`)
		case "/x/series/archives":
			query := request.URL.Query()
			if query.Get("mid") != "42" || query.Get("series_id") != "9" || query.Get("pn") != "2" || query.Get("ps") != "30" || query.Get("sort") != "desc" {
				t.Fatalf("unexpected series query: %s", request.URL.RawQuery)
			}
			if request.Header.Get("Origin") != "https://space.bilibili.com" || request.Header.Get("Referer") != "https://space.bilibili.com/42" {
				t.Fatalf("unexpected series headers: %#v", request.Header)
			}
			fmt.Fprint(writer, `{"code":0,"data":{"page":{"num":2,"size":30,"total":31},"archives":[{"bvid":"BV1ABcsztEcY","title":"video","duration":60,"stat":{"view":8}}]}}`)
		case "/x/polymer/web-space/seasons_archives_list":
			t.Fatal("season endpoint should not be requested")
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	list, err := client.GetUserList(context.Background(), UserListReference{OwnerID: 42, ListID: 9}, 2, nil)
	if err != nil || list.Metadata.ID != 9 || list.Metadata.Title != "series" || list.Metadata.Total != 31 || list.Page.Number != 2 || len(list.Archives) != 1 {
		t.Fatalf("GetUserList() = %#v, %v", list, err)
	}
}

func TestGetUserListRejectsUnmatchedOrAmbiguousID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/x/polymer/web-space/home/seasons_series" {
			t.Fatalf("detail endpoint should not be requested: %s", request.URL.Path)
		}
		fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":1,"page_size":10,"total":1},"seasons_list":[{"meta":{"mid":42,"season_id":7,"title":"season"}},{"meta":{"mid":99,"season_id":8,"title":"other"}}],"series_list":[{"meta":{"mid":42,"series_id":7,"name":"series"}}]}}}`)
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	if _, err := client.GetUserList(context.Background(), UserListReference{OwnerID: 42, ListID: 8}, 1, nil); CodeOf(err) != CodeNotFound {
		t.Fatalf("unmatched owner error = %v", err)
	}
	if _, err := client.GetUserList(context.Background(), UserListReference{OwnerID: 42, ListID: 7}, 1, nil); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("ambiguous ID error = %v", err)
	}
}

func TestGetUserListUsesKindHintForAmbiguousID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/polymer/web-space/home/seasons_series":
			fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":1,"page_size":10,"total":1},"seasons_list":[{"meta":{"mid":42,"season_id":7,"title":"season"}}],"series_list":[{"meta":{"mid":42,"series_id":7,"name":"series"}}]}}}`)
		case "/x/series/archives":
			fmt.Fprint(writer, `{"code":0,"data":{"page":{"num":1,"size":30,"total":0},"archives":[]}}`)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	list, err := client.GetUserList(context.Background(), UserListReference{OwnerID: 42, ListID: 7, KindHint: userListKindSeries}, 1, nil)
	if err != nil || list.Metadata.Title != "series" || len(list.Archives) != 0 {
		t.Fatalf("GetUserList() = %#v, %v", list, err)
	}
}

type userListRoundTripper func(*http.Request) (*http.Response, error)

func (f userListRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
