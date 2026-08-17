package auth

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
)

const authSMSPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDjb4V7EidX/ym28t2ybo0U6t0n
6p4ej8VjqKHg100va6jkNbNTrLQqMCQCAYtXMXXp2Fwkk6WR+12N9zknLjf+C9sx
/+l48mjUU8RqahiFD1XT/u2e0m2EN029OhCgkHx3Fc/KlFSIbak93EH/XlYis0w+
Xl69GV6klzgxW6d2xQIDAQAB
-----END PUBLIC KEY-----`

func TestSMSLoginReadsInputsAndSavesCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/sms/send":
			fmt.Fprint(w, `{"code":0,"data":{"captcha_key":"captcha-key"}}`)
		case "/x/passport-login/web/key":
			fmt.Fprintf(w, `{"code":0,"data":{"key":%q}}`, authSMSPublicKey)
		case "/x/passport-login/login/sms":
			fmt.Fprint(w, `{"code":0,"data":{"token_info":{"access_token":"access-token"},"cookie_info":{"cookies":[{"name":"SESSDATA","value":"session"},{"name":"bili_jct","value":"csrf"}]}}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	dir := t.TempDir()
	store := &Store{Client: client, Dir: dir, File: filepath.Join(dir, "auth.json")}
	output := &bytes.Buffer{}
	credential, err := store.SMSLogin(context.Background(), SMSLoginOptions{CountryCode: 86}, strings.NewReader("13800138000\n123456\n"), output)
	if err != nil {
		t.Fatal(err)
	}
	if credential == nil || credential.AccessKey != "access-token" || credential.Sessdata != "session" {
		t.Fatalf("unexpected credential: %#v", credential)
	}
	if !strings.Contains(output.String(), "短信验证码已发送") || !strings.Contains(output.String(), "登录成功") {
		t.Fatalf("unexpected login output: %q", output.String())
	}
	saved, err := store.LoadSaved()
	if err != nil || saved == nil || saved.AccessKey != "access-token" {
		t.Fatalf("saved credential = %#v, %v", saved, err)
	}
}

func TestSMSLoginUsesExistingCaptchaKeyWithoutSending(t *testing.T) {
	loginRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/web/key":
			fmt.Fprintf(w, `{"code":0,"data":{"key":%q}}`, authSMSPublicKey)
		case "/x/passport-login/login/sms":
			loginRequests++
			fmt.Fprint(w, `{"code":0,"data":{"token_info":{"access_token":"access-token"}}}`)
		case "/x/passport-login/sms/send":
			t.Fatalf("SMS send should not be called when captcha_key is provided")
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := api.NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	dir := t.TempDir()
	store := &Store{Client: client, Dir: dir, File: filepath.Join(dir, "auth.json")}
	if _, err := store.SMSLogin(context.Background(), SMSLoginOptions{Phone: "13800138000", Code: "123456", CaptchaKey: "existing-key"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if loginRequests != 1 {
		t.Fatalf("login request count = %d", loginRequests)
	}
}
