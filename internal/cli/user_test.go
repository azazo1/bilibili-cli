package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func TestUserVideosTableUsesAppDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/frontend/finger/spi":
			fmt.Fprint(w, `{"code":0,"data":{"b_3":"b3","b_4":"b4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(w, `{"code":0}`)
		case "/x/v2/space/archive/cursor":
			fmt.Fprint(w, `{"code":0,"data":{"item":[{"bvid":"BV1duration","title":"demo","duration":125,"play":9,"publish_time_text":"2024-01-02 03:04"}]}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.AppBaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Auth.Dir = t.TempDir()
	app.Auth.File = filepath.Join(app.Auth.Dir, "auth.json")
	if err := app.Auth.Save(&api.Credential{Sessdata: "session", AccessKey: "access-token"}); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"up", "video", "42"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	result := stdout.String()
	if !strings.Contains(result, "\t发布时间\t") || !strings.Contains(result, "\t2024-01-02 03:04\t02:05\t") {
		t.Fatalf("duration or published time was not rendered from app response: %q", result)
	}
}

func TestResolveUIDAcceptsSpaceURL(t *testing.T) {
	app := newTestApp(t)
	command := NewRoot(app)
	uid, err := resolveUID(command, app, "https://space.bilibili.com/3493112693394137", output.ModeJSON)
	if err != nil || uid != 3493112693394137 {
		t.Fatalf("resolveUID() = %d, %v", uid, err)
	}
}

func TestUserListsCommandWorksInReadOnlyMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/x/polymer/web-space/seasons_series_list" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":1,"page_size":10,"total":1},"seasons_list":[{"meta":{"mid":42,"season_id":7,"title":"season","total":2}}],"series_list":[{"meta":{"mid":42,"series_id":9,"name":"series","total":3}}]}}}`)
	}))
	defer server.Close()

	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"user", "lists", "42", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 2 || data["owner"].(map[string]any)["id"] != "42" {
		t.Fatalf("unexpected directory payload: %#v", data)
	}
	if _, ok := items[0].(map[string]any)["kind"]; ok {
		t.Fatalf("list type leaked into payload: %#v", items[0])
	}
}

func TestUserListsCommandResolvesUserName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/frontend/finger/spi":
			fmt.Fprint(writer, `{"code":0,"data":{"b_3":"b3","b_4":"b4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(writer, `{"code":0}`)
		case "/x/web-interface/nav":
			fmt.Fprint(writer, `{"code":0,"data":{"wbi_img":{"img_url":"https://example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png","sub_url":"https://example.com/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png"}}}`)
		case "/x/web-interface/wbi/search/type":
			query := request.URL.Query()
			if query.Get("keyword") != "demo-up" || query.Get("search_type") != "bili_user" {
				t.Fatalf("unexpected user search query: %s", request.URL.RawQuery)
			}
			fmt.Fprint(writer, `{"code":0,"data":{"result":[{"mid":42,"uname":"demo-up"}]}}`)
		case "/x/polymer/web-space/seasons_series_list":
			if request.URL.Query().Get("mid") != "42" {
				t.Fatalf("unexpected list query: %s", request.URL.RawQuery)
			}
			fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":1,"page_size":10,"total":1},"seasons_list":[],"series_list":[]}}}`)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"user", "lists", "demo-up", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["data"].(map[string]any)["owner"].(map[string]any)["id"] != "42" {
		t.Fatalf("unexpected resolved user payload: %#v", envelope)
	}
}

func TestUserListsCommandLoadsSelectedList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/polymer/web-space/seasons_series_list":
			fmt.Fprint(writer, `{"code":0,"data":{"items_lists":{"page":{"page_num":1,"page_size":10,"total":1},"seasons_list":[{"meta":{"mid":42,"season_id":7,"title":"list","total":1}}],"series_list":[]}}}`)
		case "/x/polymer/web-space/seasons_archives_list":
			query := request.URL.Query()
			if query.Get("mid") != "42" || query.Get("season_id") != "7" || query.Get("page_num") != "1" || query.Get("page_size") != "30" || query.Get("sort_reverse") != "false" {
				t.Fatalf("unexpected season query: %s", request.URL.RawQuery)
			}
			fmt.Fprint(writer, `{"code":0,"data":{"meta":{"mid":42,"season_id":7,"title":"list","total":1},"page":{"page_num":1,"page_size":30,"total":1},"archives":[{"bvid":"BV1ABcsztEcY","title":"video","duration":60,"stat":{"view":8}}]}}`)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"user", "lists", "42/7", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	items := data["items"].([]any)
	if data["id"] != "7" || len(items) != 1 || items[0].(map[string]any)["bvid"] != "BV1ABcsztEcY" {
		t.Fatalf("unexpected list payload: %#v", data)
	}
	if _, ok := data["kind"]; ok {
		t.Fatalf("list type leaked into payload: %#v", data)
	}
}

func TestUserListsCommandValidatesPage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"user", "lists", "42", "--page", "0", "--json"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["error"].(map[string]any)["code"] != string(api.CodeInvalidInput) {
		t.Fatalf("unexpected error payload: %#v", envelope)
	}
}

func TestHasPublishedAtRequiresSourceField(t *testing.T) {
	if hasPublishedAt([]map[string]any{{"published_at": ""}}) {
		t.Fatal("empty published_at enabled the column")
	}
	if !hasPublishedAt([]map[string]any{{"published_at": "2024-01-02T03:04:05+08:00"}}) {
		t.Fatal("published_at did not enable the column")
	}
}

func TestUserVideosTitleIncludesOwnerAndUID(t *testing.T) {
	got := userVideosTitle([]map[string]any{{"owner": map[string]any{"name": "up"}}}, 42)
	if got != "用户 up (UID: 42) 的最新 1 个视频" {
		t.Fatalf("userVideosTitle() = %q", got)
	}
}
