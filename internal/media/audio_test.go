package media

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
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

func TestDownloadFileWithThreadsUsesHTTPRanges(t *testing.T) {
	content := bytes.Repeat([]byte("0123456789"), 100)
	var active int32
	var maximum int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := 0, len(content)-1
		if value := r.Header.Get("Range"); value != "" {
			if _, err := fmt.Sscanf(value, "bytes=%d-%d", &start, &end); err != nil {
				http.Error(w, "invalid range", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
			w.WriteHeader(http.StatusPartialContent)
		}
		current := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maximum)
			if current <= old || atomic.CompareAndSwapInt32(&maximum, old, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		_, _ = w.Write(content[start : end+1])
		atomic.AddInt32(&active, -1)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "media.bin")
	written, err := DownloadFileWithThreads(context.Background(), server.URL, path, nil, 4)
	if err != nil || written != int64(len(content)) {
		t.Fatalf("DownloadFileWithThreads() = %d, %v", written, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, content) {
		t.Fatalf("unexpected ranged download: %d bytes, %v", len(data), err)
	}
	if atomic.LoadInt32(&maximum) < 2 {
		t.Fatalf("range requests were not concurrent: maximum=%d", maximum)
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
