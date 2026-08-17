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
			fmt.Fprint(w, `{"code":0,"data":{"item":[{"bvid":"BV1duration","title":"demo","duration":125,"play":9}]}}`)
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
	if !strings.Contains(stdout.String(), "\t02:05\t") {
		t.Fatalf("duration was not rendered from app response: %q", stdout.String())
	}
}
