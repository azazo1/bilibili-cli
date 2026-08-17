package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/skip2/go-qrcode"
)

func TestQRLoginUsesTVAppEndpoints(t *testing.T) {
	generateRequests := 0
	pollRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-tv-login/qrcode/auth_code":
			generateRequests++
			if r.Method != http.MethodPost || r.URL.Query().Get("local_id") != "0" || r.URL.Query().Get("mobi_app") != "android_hd" || r.URL.Query().Get("appkey") == "" || r.URL.Query().Get("sign") == "" {
				t.Fatalf("unexpected TV QR request: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"code":0,"data":{"url":"https://example.com/qr","auth_code":"test-key"}}`)
		case "/x/passport-tv-login/qrcode/poll":
			pollRequests++
			if r.Method != http.MethodPost || r.URL.Query().Get("auth_code") != "test-key" || r.URL.Query().Get("local_id") != "0" || r.URL.Query().Get("appkey") == "" || r.URL.Query().Get("sign") == "" {
				t.Fatalf("unexpected TV poll request: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"code":86038,"message":"expired"}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	store := &Store{Client: client}
	_, err := store.QRLogin(context.Background(), io.Discard)
	if api.CodeOf(err) != api.CodeInvalidInput {
		t.Fatalf("unexpected QR login error: %v", err)
	}
	if generateRequests != 1 || pollRequests != 1 {
		t.Fatalf("unexpected passport request count: generate=%d poll=%d", generateRequests, pollRequests)
	}
}

func TestPollTVQRCodeExtractsAppCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/passport-tv-login/qrcode/poll" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("auth_code") != "test-key" || r.URL.Query().Get("appkey") == "" || r.URL.Query().Get("sign") == "" {
			t.Fatalf("unexpected TV poll query: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"code":0,"data":{"token_info":{"access_token":"access-token","refresh_token":"refresh-token"},"cookie_info":{"cookies":[{"name":"SESSDATA","value":"session%2Cvalue"},{"name":"bili_jct","value":"csrf"},{"name":"DedeUserID","value":"42"}]}}}`)
	}))
	defer server.Close()

	client := api.NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	credential, state, err := (&Store{Client: client}).pollTVQRCode(context.Background(), "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if state != "done" || credential == nil || credential.Sessdata != "session%2Cvalue" || credential.BiliJct != "csrf" || credential.AccessKey != "access-token" || credential.RefreshToken != "refresh-token" {
		t.Fatalf("unexpected QR credential: %#v, state=%s", credential, state)
	}
}

func TestPollTVQRCodeReportsWaiting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":86039,"message":"waiting"}`)
	}))
	defer server.Close()

	client := api.NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	credential, state, err := (&Store{Client: client}).pollTVQRCode(context.Background(), "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if credential != nil || state != "waiting" {
		t.Fatalf("unexpected TV QR state: credential=%#v state=%s", credential, state)
	}
}

func TestRenderQRUsesHalfHeightBlocks(t *testing.T) {
	content := "https://example.com/login"
	rendered, err := renderQR(content)
	if err != nil {
		t.Fatal(err)
	}
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	matrix := addQRQuietZone(code.Bitmap())
	lines := strings.Split(rendered, "\n")
	if len(lines) != (len(matrix)+1)/2 {
		t.Fatalf("unexpected compact QR height: %d", len(lines))
	}
	for _, line := range lines {
		if utf8.RuneCountInString(line) != len(matrix[0]) {
			t.Fatalf("unexpected compact QR width: %d", utf8.RuneCountInString(line))
		}
	}
}
