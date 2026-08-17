package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func TestSearchCommandAllowsReadOnlyAndIncludesPublishedAt(t *testing.T) {
	var sawSearch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/frontend/finger/spi":
			fmt.Fprint(w, `{"code":0,"data":{"b_3":"b3","b_4":"b4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(w, `{"code":0}`)
		case "/x/web-interface/nav":
			fmt.Fprint(w, `{"code":0,"data":{"wbi_img":{"img_url":"https://example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png","sub_url":"https://example.com/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png"}}}`)
		case "/x/web-interface/wbi/search/type":
			if r.URL.Query().Get("search_type") != "article" || r.URL.Query().Get("order") != "pubdate" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			sawSearch = true
			fmt.Fprint(w, `{"code":0,"data":{"result":[{"id":7,"title":"article","author":"writer","pub_time":1700000000}]}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"search", "topic", "--type", "article", "--order", "pubdate"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !sawSearch {
		t.Fatal("search request was not sent")
	}
	result := stdout.String()
	publishedAt := time.Unix(1700000000, 0).Local().Format("2006-01-02 15:04")
	if strings.Contains(result, "ok: true") || !strings.Contains(result, "#\tID\t标题") || !strings.Contains(result, "article") || !strings.Contains(result, publishedAt) {
		t.Fatalf("unexpected TSV search output: %q", result)
	}
}

func TestCompleteTableKeepsExplicitYAML(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "auto"}
	rendered := false
	err := app.CompleteTable(map[string]any{"id": "item"}, output.ModeYAML, false, true, func(io.Writer) {
		rendered = true
	})
	if err != nil || rendered || !strings.Contains(stdout.String(), "ok: true") {
		t.Fatalf("CompleteTable() = %v, rendered=%v, output=%q", err, rendered, stdout.String())
	}
}

func TestSearchRowsIncludePublishedColumn(t *testing.T) {
	for _, kind := range []string{"article", "video", "user", "bangumi", "live", "media"} {
		parsed, ok := api.ParseSearchType(kind)
		if !ok {
			t.Fatalf("ParseSearchType(%q) failed", kind)
		}
		headers := searchHeaders(parsed, true)
		if !strings.Contains(strings.Join(headers, "|"), "发布时间") {
			t.Fatalf("headers missing published column: %#v", headers)
		}
	}
}

func TestUserSearchRowsOmitPublishedColumnWithoutSourceField(t *testing.T) {
	headers := searchHeaders(api.SearchTypeUser, false)
	if strings.Contains(strings.Join(headers, "|"), "发布时间") {
		t.Fatalf("unexpected headers: %#v", headers)
	}
	rows := searchRows(api.SearchTypeUser, []map[string]any{{"uid": "42", "name": "user"}}, false)
	if len(rows) != 1 || len(rows[0]) != len(headers) {
		t.Fatalf("rows do not match headers: headers=%#v rows=%#v", headers, rows)
	}
}
