package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func TestMyDynamicsCommandWorksInReadOnlyMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/nav":
			fmt.Fprint(w, `{"code":0,"data":{"mid":42,"wbi_img":{"img_url":"https://example.test/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"https://example.test/wbi/fedcba9876543210fedcba9876543210.png"}}}`)
		case "/x/frontend/finger/spi":
			fmt.Fprint(w, `{"code":0,"data":{"b_3":"device3","b_4":"device4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(w, `{"code":0}`)
		case "/x/polymer/web-dynamic/v1/feed/space":
			fmt.Fprint(w, `{"code":0,"data":{"items":[{"id_str":"99","modules":{"module_author":{"is_top":true}}},{"id_str":"100","modules":{"module_author":{"is_top":false}}}]}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Auth.Dir = t.TempDir()
	app.Auth.File = filepath.Join(app.Auth.Dir, "auth.json")
	if err := app.Auth.Save(&api.Credential{Sessdata: "session"}); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"me", "dynamic", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	items := result["data"].(map[string]any)["items"].([]any)
	if result["ok"] != true || len(items) != 1 || items[0].(map[string]any)["id"] != "100" {
		t.Fatalf("unexpected output: %#v", result)
	}
	stdout.Reset()
	root = NewRoot(app)
	root.SetArgs([]string{"me", "dynamic", "--top", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	items = result["data"].(map[string]any)["items"].([]any)
	if result["ok"] != true || len(items) != 2 {
		t.Fatalf("unexpected top output: %#v", result)
	}
}
