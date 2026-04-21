package filter

import (
	"testing"
)

func TestThinkingFilter_ProcessThinking_FirstChunk(t *testing.T) {
	f := &ThinkingFilter{}
	// When HasSeenFirstThinking is false, it looks for "> " and skips it
	result := f.ProcessThinking("prefix > content after")
	if result != "content after" {
		t.Errorf("expected 'content after', got %q", result)
	}
	if !f.HasSeenFirstThinking {
		t.Error("expected HasSeenFirstThinking to be true")
	}
}

func TestThinkingFilter_ProcessThinking_WithPrefix(t *testing.T) {
	f := &ThinkingFilter{}
	result := f.ProcessThinking("> thinking content")
	if result != "thinking content" {
		t.Errorf("expected 'thinking content', got %q", result)
	}
	if !f.HasSeenFirstThinking {
		t.Error("expected HasSeenFirstThinking to be true")
	}
}

func TestThinkingFilter_ProcessThinking_ReplaceNewlinePrefix(t *testing.T) {
	f := &ThinkingFilter{}
	f.HasSeenFirstThinking = true
	result := f.ProcessThinking("line1\n> line2\n> line3")
	expected := "line1\nline2\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestThinkingFilter_ProcessThinking_BufferSuffix(t *testing.T) {
	f := &ThinkingFilter{}
	f.HasSeenFirstThinking = true
	result := f.ProcessThinking("content\n>")
	if result != "content" {
		t.Errorf("expected 'content', got %q", result)
	}
	if f.Buffer != "\n>" {
		t.Errorf("expected buffer '\\n>', got %q", f.Buffer)
	}
}

func TestThinkingFilter_ProcessThinking_BufferNewline(t *testing.T) {
	f := &ThinkingFilter{}
	f.HasSeenFirstThinking = true
	result := f.ProcessThinking("content\n")
	if result != "content" {
		t.Errorf("expected 'content', got %q", result)
	}
	if f.Buffer != "\n" {
		t.Errorf("expected buffer '\\n', got %q", f.Buffer)
	}
}

func TestThinkingFilter_Flush_Empty(t *testing.T) {
	f := &ThinkingFilter{}
	result := f.Flush()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestThinkingFilter_Flush_WithBuffer(t *testing.T) {
	f := &ThinkingFilter{Buffer: "buffered content"}
	result := f.Flush()
	if result != "buffered content" {
		t.Errorf("expected 'buffered content', got %q", result)
	}
	if f.Buffer != "" {
		t.Errorf("expected empty buffer, got %q", f.Buffer)
	}
}

func TestThinkingFilter_ExtractCompleteThinking_Valid(t *testing.T) {
	f := &ThinkingFilter{}
	editContent := "prefix > thinking line1\n> thinking line2\n</details> suffix"
	result := f.ExtractCompleteThinking(editContent)
	expected := "thinking line1\nthinking line2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestThinkingFilter_ExtractCompleteThinking_NoStart(t *testing.T) {
	f := &ThinkingFilter{}
	editContent := "no prefix here\n</details>"
	result := f.ExtractCompleteThinking(editContent)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestThinkingFilter_ExtractCompleteThinking_NoEnd(t *testing.T) {
	f := &ThinkingFilter{}
	editContent := "prefix > thinking content"
	result := f.ExtractCompleteThinking(editContent)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestThinkingFilter_ExtractIncrementalThinking_FirstCall(t *testing.T) {
	f := &ThinkingFilter{}
	editContent := "prefix > thinking line1\n> thinking line2\n</details> suffix"
	result := f.ExtractIncrementalThinking(editContent)
	expected := "thinking line1\nthinking line2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestThinkingFilter_ExtractIncrementalThinking_Subsequent(t *testing.T) {
	f := &ThinkingFilter{LastOutputChunk: "thinking line1"}
	editContent := "prefix > thinking line1\n> thinking line2\n> thinking line3\n</details> suffix"
	result := f.ExtractIncrementalThinking(editContent)
	expected := "\nthinking line2\nthinking line3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestThinkingFilter_ExtractIncrementalThinking_NoMatch(t *testing.T) {
	f := &ThinkingFilter{LastOutputChunk: "nonexistent"}
	editContent := "prefix > thinking line1\n> thinking line2\n</details> suffix"
	result := f.ExtractIncrementalThinking(editContent)
	expected := "thinking line1\nthinking line2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestThinkingFilter_ResetForNewRound(t *testing.T) {
	f := &ThinkingFilter{
		LastOutputChunk:      "some chunk",
		HasSeenFirstThinking: true,
	}
	f.ResetForNewRound()
	if f.LastOutputChunk != "" {
		t.Errorf("expected empty LastOutputChunk, got %q", f.LastOutputChunk)
	}
	if f.HasSeenFirstThinking {
		t.Error("expected HasSeenFirstThinking to be false")
	}
}

func TestThinkingFilter_ProcessThinking_MultipleChunks(t *testing.T) {
	f := &ThinkingFilter{}
	
	// First chunk with prefix
	result1 := f.ProcessThinking("> chunk1")
	if result1 != "chunk1" {
		t.Errorf("first chunk: expected 'chunk1', got %q", result1)
	}
	
	// Second chunk
	result2 := f.ProcessThinking("chunk2")
	if result2 != "chunk2" {
		t.Errorf("second chunk: expected 'chunk2', got %q", result2)
	}
	
	// Third chunk with newline prefix
	result3 := f.ProcessThinking("\n> chunk3")
	if result3 != "\nchunk3" {
		t.Errorf("third chunk: expected '\\nchunk3', got %q", result3)
	}
}
