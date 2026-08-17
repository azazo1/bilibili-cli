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
	details := errorData["details"].(map[string]any)
	if payload["ok"] != false || errorData["code"] != string(api.CodeInvalidInput) || !strings.Contains(details["usage"].(string), "bili video BV_OR_URL") {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestArgumentErrorIncludesCommandUsage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	result := stderr.String()
	if !strings.Contains(result, "accepts 1 arg(s), received 0") || !strings.Contains(result, "Usage:\n  bili video BV_OR_URL") {
		t.Fatalf("argument error did not include usage: %q", result)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestFlagErrorIncludesCommandUsage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "--unknown"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	result := stderr.String()
	if !strings.Contains(result, "unknown flag: --unknown") || !strings.Contains(result, "Usage:\n  bili video BV_OR_URL") {
		t.Fatalf("flag error did not include usage: %q", result)
	}
}

func TestArgumentErrorJSONIncludesUsage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "--json"})
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
	details := errorData["details"].(map[string]any)
	if !strings.Contains(details["usage"].(string), "bili video BV_OR_URL") {
		t.Fatalf("structured usage is missing: %#v", payload)
	}
}

func TestInvalidFlagValueIncludesCommandUsage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "subtitle", "BV1ABcsztEcY", "--type", "invalid"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	result := stderr.String()
	if !strings.Contains(result, "--type 仅支持 all, ai 或 non-ai") || !strings.Contains(result, "Usage:\n  bili video subtitle BV_OR_URL") {
		t.Fatalf("invalid flag value did not include usage: %q", result)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestConflictingOutputFlagsIncludeCommandUsage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "BV1ABcsztEcY", "--json", "--yaml"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	result := stderr.String()
	if !strings.Contains(result, "不能同时使用 --json 和 --yaml") || !strings.Contains(result, "Usage:\n  bili video BV_OR_URL") {
		t.Fatalf("conflicting output flags did not include usage: %q", result)
	}
}

func TestUnknownCommandIncludesRootUsage(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"unknown-command"})
	command, err := root.ExecuteContextC(context.Background())
	if err != nil {
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			err = app.failUsage(command, err)
		}
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	result := stderr.String()
	if !strings.Contains(result, "unknown command \"unknown-command\" for \"bili\"") || !strings.Contains(result, "Usage:\n  bili [command]") {
		t.Fatalf("unknown command did not include usage: %q", result)
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
	root.SetArgs([]string{"video", "hot", "--max", "1", "--json"})
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
	root.SetArgs([]string{"video", "like", "BV1ABcsztEcY", "--json"})
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
	root.SetArgs([]string{"me", "logout"})
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
	if data["available_subtitle_count"] != float64(2) || data["subtitle_count"] != float64(2) || first["line_count"] != float64(2) || second["is_ai"] != true {
		t.Fatalf("unexpected subtitle payload: %#v", data)
	}
}

func TestVideoSubtitleFiltersTracks(t *testing.T) {
	server := newSubtitleTestServer(t)
	defer server.Close()
	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "subtitle", "BV1ABcsztEcY", "--language", "zh_CN", "--type", "non-ai", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	subtitles := data["subtitles"].([]any)
	item := subtitles[0].(map[string]any)
	if data["available_subtitle_count"] != float64(2) || data["subtitle_count"] != float64(1) || item["id"] != "11" {
		t.Fatalf("unexpected filtered subtitle payload: %#v", data)
	}
}

func TestFilterSubtitleTracksSupportsIDs(t *testing.T) {
	tracks := []api.SubtitleTrack{
		{ID: "11", Language: "zh-CN"},
		{ID: "12", Language: "en-US", Type: 1},
	}
	filtered := filterSubtitleTracks(tracks, []string{"12"}, nil, "ai")
	if len(filtered) != 1 || filtered[0].ID != "12" {
		t.Fatalf("unexpected ID filter result: %#v", filtered)
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

func TestExportSubtitleFilesUsesSRTSuffixForAllTracks(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "subt.srt")
	items := []subtitleCommandItem{
		{Track: api.SubtitleTrack{Language: "zh-CN"}, Cues: []api.SubtitleCue{{From: 0, To: 1, Content: "normal"}}},
		{Track: api.SubtitleTrack{Language: "zh-CN", Type: 1}, Cues: []api.SubtitleCue{{From: 0, To: 1, Content: "ai"}}},
	}
	if err := exportSubtitleFiles(outputPath, "BV1ABcsztEcY", items); err != nil {
		t.Fatal(err)
	}
	normalPath := filepath.Join(filepath.Dir(outputPath), "subt-zh_CN.srt")
	aiPath := filepath.Join(filepath.Dir(outputPath), "subt-zh_CN-ai.srt")
	for _, path := range []string{normalPath, aiPath} {
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("unexpected suffix subtitle export: %s, %v", path, err)
		}
	}
	if items[0].OutputPath != normalPath || items[1].OutputPath != aiPath {
		t.Fatalf("unexpected subtitle output paths: %#v", items)
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

func TestCommandHierarchyUsesDomainParents(t *testing.T) {
	root := NewRoot(newTestApp(t))
	cases := [][]string{
		{"me"},
		{"me", "fav"},
		{"me", "history"},
		{"me", "dynamic"},
		{"video", "like"},
		{"video", "audio"},
		{"video", "hot"},
		{"video", "watch"},
		{"user", "video"},
		{"user", "follow"},
		{"dynamic", "post"},
		{"dynamic", "delete"},
	}
	for _, args := range cases {
		command, _, err := root.Find(args)
		if err != nil {
			t.Fatalf("command %v not found: %v", args, err)
		}
		if command == nil {
			t.Fatalf("command %v resolved to nil", args)
		}
	}
	if _, _, err := root.Find([]string{"whoami"}); err == nil {
		t.Fatal("legacy whoami command is still registered")
	}
}

func TestMeRejectsUnknownPositionalArgument(t *testing.T) {
	app := newTestApp(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: stderr, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"me", "video"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("unexpected command error: %v", err)
	}
	result := stderr.String()
	if !strings.Contains(result, "unknown command \"video\" for \"bili me\"") || !strings.Contains(result, "Usage:\n  bili me [flags]") {
		t.Fatalf("me argument error did not include usage: %q", result)
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
