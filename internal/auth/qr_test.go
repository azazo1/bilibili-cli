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

func TestQRLoginUsesPassportHostForGenerateAndPoll(t *testing.T) {
	generateRequests := 0
	pollRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/web/qrcode/generate":
			generateRequests++
			fmt.Fprint(w, `{"code":0,"data":{"url":"https://example.com/qr","qrcode_key":"test-key"}}`)
		case "/x/passport-login/web/qrcode/poll":
			pollRequests++
			if r.URL.Query().Get("qrcode_key") != "test-key" {
				t.Fatalf("unexpected qrcode key: %s", r.URL.Query().Get("qrcode_key"))
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
