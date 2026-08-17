package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDownloadFileReportsProgress(t *testing.T) {
	const content = "downloaded media"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "media.bin")
	updates := []DownloadProgress{}
	written, err := DownloadFileWithProgress(context.Background(), server.URL, path, nil, func(progress DownloadProgress) {
		updates = append(updates, progress)
	})
	if err != nil || written != int64(len(content)) {
		t.Fatalf("DownloadFileWithProgress() = %d, %v", written, err)
	}
	if len(updates) == 0 {
		t.Fatal("missing progress updates")
	}
	last := updates[len(updates)-1]
	if last.Written != int64(len(content)) || last.Total != int64(len(content)) {
		t.Fatalf("unexpected final progress: %#v", last)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != content {
		t.Fatalf("unexpected download: %q, %v", data, err)
	}
}

func TestSplitPCM16WAV(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.wav")
	pcm := make([]byte, 16000*3*2)
	if err := writePCM16WAV(input, pcm, 16000); err != nil {
		t.Fatal(err)
	}
	segments, err := SplitAudio(context.Background(), input, filepath.Join(dir, "parts"), 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 2 {
		t.Fatalf("segment count = %d", len(segments))
	}
	for _, path := range segments {
		info, statErr := os.Stat(path)
		if statErr != nil || info.Size() <= 44 {
			t.Fatalf("invalid segment %s: %v", path, statErr)
		}
	}
}
