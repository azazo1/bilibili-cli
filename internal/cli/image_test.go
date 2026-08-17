package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azazo1/bilibili-cli/internal/output"
	"gopkg.in/yaml.v3"
)

func TestImageCommandDownloadsVideoAssetsInReadOnlyMode(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/web-interface/view":
			fmt.Fprint(writer, `{"code":0,"data":{"title":"demo","pic":"`+serverURL+`/cover.webp","owner":{"face":"`+serverURL+`/avatar.png"}}}`)
		case "/cover.webp":
			writer.Write([]byte("cover-data"))
		case "/avatar.png":
			writer.Write([]byte("avatar-data"))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Config.Safety.ReadOnly = true
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	outDir := t.TempDir()
	coverPath := filepath.Join(outDir, "video-BV1ABcsztEcY-cover.webp")
	if err := os.WriteFile(coverPath, []byte("old-cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := NewRoot(app)
	root.SetArgs([]string{"image", "BV1ABcsztEcY", "--with-avatar", "--json", "-o", outDir})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	cover, err := os.ReadFile(coverPath)
	if err != nil || string(cover) != "cover-data" {
		t.Fatalf("cover was not replaced: %q, %v", cover, err)
	}
	avatar, err := os.ReadFile(filepath.Join(outDir, "video-BV1ABcsztEcY-avatar.png"))
	if err != nil || string(avatar) != "avatar-data" {
		t.Fatalf("avatar was not downloaded: %q, %v", avatar, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	data := envelope["data"].(map[string]any)
	assets := data["assets"].([]any)
	if envelope["ok"] != true || len(assets) != 2 || len(data["warnings"].([]any)) != 0 {
		t.Fatalf("unexpected image output: %#v", envelope)
	}
}

func TestImageCommandDefaultsNumericReferenceToUser(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/frontend/finger/spi":
			fmt.Fprint(writer, `{"code":0,"data":{"b_3":"b3","b_4":"b4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(writer, `{"code":0}`)
		case "/42/dynamic":
			fmt.Fprint(writer, `<script id="__RENDER_DATA__" type="application/json">{"access_id":"web-id"}</script>`)
		case "/x/web-interface/nav":
			fmt.Fprintf(writer, `{"code":0,"data":{"wbi_img":{"img_url":"%s/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"%s/wbi/fedcba9876543210fedcba9876543210.png"}}}`, serverURL, serverURL)
		case "/x/space/wbi/acc/info":
			fmt.Fprint(writer, `{"code":0,"data":{"mid":42,"name":"demo user","face":"`+serverURL+`/user.jpg"}}`)
		case "/user.jpg":
			writer.Write([]byte("user-avatar"))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	t.Setenv("BILI_SPACE_BASE_URL", server.URL)

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Config.Safety.ReadOnly = true
	app.Out = &output.Writer{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	outDir := t.TempDir()
	root := NewRoot(app)
	root.SetArgs([]string{"image", "42", "-o", outDir})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	avatar, err := os.ReadFile(filepath.Join(outDir, "user-42-avatar.jpg"))
	if err != nil || string(avatar) != "user-avatar" {
		t.Fatalf("default user avatar was not downloaded: %q, %v", avatar, err)
	}
}

func TestImageCommandKeepsCoverWhenAvatarUnavailable(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/web-interface/view":
			fmt.Fprint(writer, `{"code":0,"data":{"title":"demo","pic":"`+serverURL+`/cover.jpg","owner":{}}}`)
		case "/cover.jpg":
			writer.Write([]byte("cover-data"))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	outDir := t.TempDir()
	root := NewRoot(app)
	root.SetArgs([]string{"image", "video", "BV1ABcsztEcY", "--with-avatar", "--json", "-o", outDir})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "video-BV1ABcsztEcY-cover.jpg")); err != nil {
		t.Fatalf("cover was not retained: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if envelope["ok"] != true || len(data["assets"].([]any)) != 1 || len(data["warnings"].([]any)) != 1 {
		t.Fatalf("unexpected avatar warning output: %#v", envelope)
	}
}

func TestImageCommandEmitsYAMLEnvelopeWithoutProgress(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/web-interface/view":
			fmt.Fprint(writer, `{"code":0,"data":{"title":"demo","pic":"`+serverURL+`/cover.jpg"}}`)
		case "/cover.jpg":
			writer.Write([]byte("cover-data"))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"image", "BV1ABcsztEcY", "--yaml", "-o", t.TempDir()})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := yaml.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid YAML output: %v", err)
	}
	data := envelope["data"].(map[string]any)
	if envelope["ok"] != true || len(data["assets"].([]any)) != 1 {
		t.Fatalf("unexpected YAML image output: %#v", envelope)
	}
}

func TestImageCommandReturnsStructuredErrorForMissingPrimaryImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/x/web-interface/view" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		fmt.Fprint(writer, `{"code":0,"data":{"title":"demo"}}`)
	}))
	defer server.Close()

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"image", "BV1ABcsztEcY", "--json"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("missing primary image unexpectedly succeeded")
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON error output: %v", err)
	}
	errorData := envelope["error"].(map[string]any)
	if envelope["ok"] != false || errorData["code"] != "not_found" {
		t.Fatalf("unexpected image error output: %#v", envelope)
	}
}

func TestImageCommandHierarchy(t *testing.T) {
	root := NewRoot(newTestApp(t))
	for _, path := range [][]string{{"image"}, {"image", "user"}, {"image", "up"}, {"image", "video"}, {"image", "article"}, {"image", "bangumi"}, {"image", "media"}, {"image", "live"}} {
		command, _, err := root.Find(path)
		if err != nil || command == nil {
			t.Fatalf("image command path %v was not found: %v", path, err)
		}
	}
	command, _, err := root.Find([]string{"image", "up"})
	if err != nil || command.Name() != "user" || !strings.Contains(command.Use, "user") {
		t.Fatalf("up alias did not resolve user command: %#v, %v", command, err)
	}
}
