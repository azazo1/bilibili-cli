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
