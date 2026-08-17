package auth

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWaitForGeetestValidationAcceptsBrowserResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output := &lockedBuffer{}
	type result struct {
		validation geetestValidation
		err        error
	}
	completed := make(chan result, 1)
	go func() {
		validation, err := waitForGeetestValidation(ctx, map[string]any{"gt": "gt"}, output)
		completed <- result{validation: validation, err: err}
	}()
	rootURL := waitForGeetestURL(t, output)
	response, err := http.Get(rootURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`const resultPath = ("[^"]+");`).FindStringSubmatch(string(page))
	if len(match) != 2 {
		t.Fatalf("result path missing from page: %s", page)
	}
	if !strings.Contains(string(page), `<body>
  <script src="https://static.geetest.com/static/js/fullpage.0.0.0.js"></script>`) {
		t.Fatal("Geetest script is not loaded after body initialization")
	}
	resultPath, err := strconv.Unquote(match[1])
	if err != nil {
		t.Fatal(err)
	}
	response, err = http.Post(rootURL+resultPath, "application/json", strings.NewReader(`{"geetest_challenge":"challenge","geetest_validate":"validate","geetest_seccode":"seccode"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected result status: %d", response.StatusCode)
	}
	select {
	case received := <-completed:
		if received.err != nil || received.validation.Challenge != "challenge" || received.validation.Validate != "validate" || received.validation.Seccode != "seccode" {
			t.Fatalf("unexpected Geetest result: %#v, %v", received.validation, received.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Geetest validation did not finish")
	}
}

func TestDecodeGeetestValidationRejectsMissingFields(t *testing.T) {
	if _, err := decodeGeetestValidation(strings.NewReader(`{"geetest_challenge":"challenge"}`)); err == nil {
		t.Fatal("expected incomplete validation error")
	}
}

func waitForGeetestURL(t *testing.T, output *lockedBuffer) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		lines := strings.Fields(output.String())
		for _, line := range lines {
			if strings.HasPrefix(line, "http://127.0.0.1:") {
				return line
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("local verification URL was not rendered: %q", output.String())
	return ""
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
