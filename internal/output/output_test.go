package output

import (
	"bytes"
	"testing"
)

func TestResolveUsesConfiguredDefaultMode(t *testing.T) {
	t.Setenv("OUTPUT", "")
	writer := &Writer{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, DefaultMode: "json"}
	mode, err := writer.Resolve(false, false)
	if err != nil || mode != ModeJSON {
		t.Fatalf("Resolve() = %q, %v", mode, err)
	}
}
