package cli

import (
	"bytes"
	"context"
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
