package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("BILI_CONFIG_DIR", t.TempDir())
	return NewApp()
}

func TestVideoInvalidBVIDEmitsStructuredError(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "invalid", "--json"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	var payload map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("invalid JSON: %v", decodeErr)
	}
	errorData := payload["error"].(map[string]any)
	if payload["ok"] != false || errorData["code"] != string(api.CodeInvalidInput) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestHotCommandUsesNormalizedEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/web-interface/popular" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("ps") != "1" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"code":0,"data":{"list":[{"bvid":"BV1ABcsztEcY","title":"demo","duration":60,"owner":{"mid":1,"name":"up"},"stat":{"view":9}}]}}`)
	}))
	defer server.Close()

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"hot", "--max", "1", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	items := data["items"].([]any)
	item := items[0].(map[string]any)
	if envelope["ok"] != true || item["duration"] != "01:00" || item["bvid"] != "BV1ABcsztEcY" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestReadOnlyBlocksAccountWriteCommands(t *testing.T) {
	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"like", "BV1ABcsztEcY", "--json"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	errorData := payload["error"].(map[string]any)
	if errorData["code"] != string(api.CodePermissionDenied) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestReadOnlyAllowsLogout(t *testing.T) {
	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	tempDir := t.TempDir()
	app.Auth.Dir = tempDir
	app.Auth.File = filepath.Join(tempDir, "auth.json")
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"logout"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() == 0 {
		t.Fatal("logout did not render completion")
	}
}

func TestNewAppAppliesDefaultTimeoutWithoutCreatingConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("BILI_CONFIG_DIR", configDir)
	t.Setenv("BILI_HTTP_TIMEOUT", "7")
	app := NewApp()
	if app.API.HTTP.Timeout != 30*time.Second {
		t.Fatalf("timeout = %s", app.API.HTTP.Timeout)
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("NewApp created config file: %v", err)
	}
}

func TestConfigInitCreatesDefaultFile(t *testing.T) {
	app := newTestApp(t)
	root := NewRoot(app)
	root.SetArgs([]string{"config", "init"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(app.ConfigStore.File); err != nil {
		t.Fatalf("config init did not create config file: %v", err)
	}
}

func TestVideoSubtitleListsAllTracks(t *testing.T) {
	server := newSubtitleTestServer(t)
	defer server.Close()
	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "subtitle", "BV1ABcsztEcY", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	subtitles := data["subtitles"].([]any)
	first := subtitles[0].(map[string]any)
	second := subtitles[1].(map[string]any)
	if data["subtitle_count"] != float64(2) || first["line_count"] != float64(2) || second["is_ai"] != true {
		t.Fatalf("unexpected subtitle payload: %#v", data)
	}
}

func TestVideoSubtitleAliasExportsEachTrack(t *testing.T) {
	server := newSubtitleTestServer(t)
	defer server.Close()
	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Out = &output.Writer{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	outputDir := t.TempDir()
	root := NewRoot(app)
	root.SetArgs([]string{"video", "st", "BV1ABcsztEcY", "-o", outputDir})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	zhPath := filepath.Join(outputDir, "BV1ABcsztEcY.01.zh-CN.srt")
	enPath := filepath.Join(outputDir, "BV1ABcsztEcY.02.en-US.srt")
	zhData, err := os.ReadFile(zhPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(enPath); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zhData), "00:00:00,000 --> 00:00:01,500") {
		t.Fatalf("unexpected SRT content: %q", zhData)
	}
}

func TestExportSubtitleFilesWritesSingleSRTPath(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "subtitle.srt")
	items := []subtitleCommandItem{{
		Track: api.SubtitleTrack{Language: "zh-CN"},
		Cues:  []api.SubtitleCue{{From: 0, To: 1, Content: "line"}},
	}}
	if err := exportSubtitleFiles(outputPath, "BV1ABcsztEcY", items); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 || items[0].OutputPath != outputPath {
		t.Fatalf("unexpected direct subtitle export: %#v", items)
	}
}

func TestVideoSubtitleCommandReplacesLegacyFlags(t *testing.T) {
	root := NewRoot(newTestApp(t))
	video, _, err := root.Find([]string{"video"})
	if err != nil {
		t.Fatal(err)
	}
	if video.Flags().Lookup("subtitle") != nil {
		t.Fatal("legacy subtitle flag is still registered")
	}
	subtitle, _, err := root.Find([]string{"video", "st"})
	if err != nil {
		t.Fatal(err)
	}
	if subtitle.Name() != "subtitle" {
		t.Fatalf("unexpected subtitle command: %s", subtitle.Name())
	}
}

func newSubtitleTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/player/pagelist":
			fmt.Fprint(w, `{"code":0,"data":[{"cid":42}]}`)
		case "/x/player/v2":
			fmt.Fprintf(w, `{"code":0,"data":{"subtitle":{"subtitles":[{"id":11,"lan":"zh-CN","lan_doc":"中文","subtitle_url":"%s/subtitles/zh.json","author":{"mid":7,"name":"up"},"type":0},{"id":12,"lan":"en-US","lan_doc":"English","subtitle_url":"%s/subtitles/en.json","type":1,"ai_type":3,"ai_status":1}]}}}`, serverURL, serverURL)
		case "/subtitles/zh.json":
			fmt.Fprint(w, `{"body":[{"from":0,"to":1.5,"content":"first"},{"from":1.5,"to":3,"content":"second"}]}`)
		case "/subtitles/en.json":
			fmt.Fprint(w, `{"body":[{"from":0,"to":1.5,"content":"one"}]}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	serverURL = server.URL
	return server
}
