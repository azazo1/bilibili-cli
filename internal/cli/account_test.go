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

	"github.com/azazo1/bilibili-cli/internal/output"
)

const cliSMSPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDjb4V7EidX/ym28t2ybo0U6t0n
6p4ej8VjqKHg100va6jkNbNTrLQqMCQCAYtXMXXp2Fwkk6WR+12N9zknLjf+C9sx
/+l48mjUU8RqahiFD1XT/u2e0m2EN029OhCgkHx3Fc/KlFSIbak93EH/XlYis0w+
Xl69GV6klzgxW6d2xQIDAQAB
-----END PUBLIC KEY-----`

func TestSMSLoginCommandWorksInReadOnlyMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/sms/send":
			fmt.Fprint(w, `{"code":0,"data":{"captcha_key":"captcha-key"}}`)
		case "/x/passport-login/web/key":
			fmt.Fprintf(w, `{"code":0,"data":{"key":%q}}`, cliSMSPublicKey)
		case "/x/passport-login/login/sms":
			fmt.Fprint(w, `{"code":0,"data":{"token_info":{"access_token":"access-token"},"cookie_info":{"cookies":[{"name":"SESSDATA","value":"session"},{"name":"bili_jct","value":"csrf"}]}}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	app := newTestApp(t)
	app.Config.Safety.ReadOnly = true
	app.API.PassportBaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Auth.Dir = t.TempDir()
	app.Auth.File = filepath.Join(app.Auth.Dir, "auth.json")
	stdout := &bytes.Buffer{}
	app.Out = &output.Writer{Stdout: stdout, Stderr: &bytes.Buffer{}, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"me", "login", "--phone", "13800138000", "--code", "123456", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON output: %v, %q", err, stdout.String())
	}
	if envelope["ok"] != true {
		t.Fatalf("unexpected login output: %#v", envelope)
	}
}
