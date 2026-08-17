package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTableUsesUntruncatedTSVOutsidePTY(t *testing.T) {
	var buffer bytes.Buffer
	value := "long-title-that-must-not-be-truncated"
	RenderTable(&buffer, "ignored title", []string{"ID", "Title"}, [][]string{{"1", value}}, TableOptions{})
	if got, want := buffer.String(), "ID\tTitle\n1\t"+value+"\n"; got != want {
		t.Fatalf("RenderTable() = %q, want %q", got, want)
	}
}

func TestRenderTableAlignsAndFitsInteractiveWidth(t *testing.T) {
	var buffer bytes.Buffer
	RenderTable(&buffer, "title", []string{"ID", "标题", "作者"}, [][]string{{"1", "a very long title", "作者名称"}}, TableOptions{Interactive: true, Width: 24})
	output := buffer.String()
	if strings.Contains(output, "\t") || !strings.Contains(output, "...") {
		t.Fatalf("unexpected interactive table: %q", output)
	}
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if displayWidth(line) > 24 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}

func TestRenderTableNoTruncateKeepsFullCells(t *testing.T) {
	var buffer bytes.Buffer
	value := "long-title-that-must-remain-visible"
	RenderTable(&buffer, "title", []string{"ID", "Title"}, [][]string{{"1", value}}, TableOptions{Interactive: true, Width: 12, NoTruncate: true})
	if !strings.Contains(buffer.String(), value) {
		t.Fatalf("RenderTable() truncated value: %q", buffer.String())
	}
}
