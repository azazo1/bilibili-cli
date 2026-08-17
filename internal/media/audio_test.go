package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

