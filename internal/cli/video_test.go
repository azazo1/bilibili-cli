package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/azazo1/bilibili-cli/internal/output"
)

func TestVideoCommandShowsMultiPartPagesInReadOnlyMode(t *testing.T) {
	pagesRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			fmt.Fprint(w, `{"code":0,"data":{"bvid":"BV1ABcsztEcY","title":"demo","duration":60,"owner":{"mid":1,"name":"up"},"stat":{"view":9}}}`)
		case "/x/player/pagelist":
			if r.URL.Query().Get("bvid") != "BV1ABcsztEcY" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			pagesRequested = true
			fmt.Fprint(w, `{"code":0,"data":[{"page":1,"cid":41,"part":"first"},{"page":2,"cid":42,"part":"second"}]}`)
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
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "BV1ABcsztEcY"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !pagesRequested {
		t.Fatal("video pages request was not sent")
	}
	if !strings.Contains(stdout.String(), "分P\t标题\nP1\tfirst\nP2\tsecond") {
		t.Fatalf("multi-part pages were not rendered: %q", stdout.String())
	}
}
