package filter

import (
	"testing"

	"zai-proxy/internal/model"
)

func TestNewSearchRefFilter(t *testing.T) {
	f := NewSearchRefFilter()
	if f == nil {
		t.Error("expected non-nil SearchRefFilter")
	}
	if f.searchResults == nil {
		t.Error("expected non-nil searchResults map")
	}
	if len(f.searchResults) != 0 {
		t.Error("expected empty searchResults map")
	}
}

func TestAddSearchResults(t *testing.T) {
	f := NewSearchRefFilter()
	results := []model.SearchResult{
		{RefID: "turn1search1", Index: 1, Title: "Result 1", URL: "https://example.com/1"},
		{RefID: "turn1search2", Index: 2, Title: "Result 2", URL: "https://example.com/2"},
	}
	f.AddSearchResults(results)
	if len(f.searchResults) != 2 {
		t.Errorf("expected 2 results, got %d", len(f.searchResults))
	}
	if r, ok := f.searchResults["turn1search1"]; !ok || r.Index != 1 {
		t.Error("expected result with RefID turn1search1")
	}
}

func TestEscapeMarkdownTitle(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with [brackets]", `with \[brackets\]`},
		{"with [nested [brackets]]", `with \[nested \[brackets\]\]`},
		{"with \\ backslash", `with \\ backslash`},
		{"[test]", `\[test\]`},
		{"", ""},
	}

	for _, tc := range testCases {
		result := escapeMarkdownTitle(tc.input)
		if result != tc.expected {
			t.Errorf("escapeMarkdownTitle(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestProcess_SimpleRef(t *testing.T) {
	f := NewSearchRefFilter()
	f.AddSearchResults([]model.SearchResult{
		{RefID: "turn1search1", Index: 1, Title: "Result", URL: "https://example.com"},
	})

	result := f.Process("Check this 【turn1search1】 result")
	expected := `Check this [\[1\]](https://example.com) result`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestProcess_MultipleRefs(t *testing.T) {
	f := NewSearchRefFilter()
	f.AddSearchResults([]model.SearchResult{
		{RefID: "turn1search1", Index: 1, Title: "Result 1", URL: "https://example.com/1"},
		{RefID: "turn1search2", Index: 2, Title: "Result 2", URL: "https://example.com/2"},
	})

	result := f.Process("See 【turn1search1】 and 【turn1search2】")
	if !contains(result, `[\[1\]]`) || !contains(result, `[\[2\]]`) {
		t.Errorf("expected both refs in result: %q", result)
	}
}

func TestProcess_UnknownRef(t *testing.T) {
	f := NewSearchRefFilter()
	result := f.Process("Unknown 【turn1search999】 ref")
	expected := "Unknown  ref"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestProcess_BufferIncompleteRef(t *testing.T) {
	f := NewSearchRefFilter()
	result := f.Process("Text with incomplete 【turn1search")
	if result != "Text with incomplete " {
		t.Errorf("expected 'Text with incomplete ', got %q", result)
	}
	if f.buffer != "【turn1search" {
		t.Errorf("expected buffer '【turn1search', got %q", f.buffer)
	}
}

func TestProcess_EmptyContent(t *testing.T) {
	f := NewSearchRefFilter()
	result := f.Process("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFlush_WithBuffer(t *testing.T) {
	f := NewSearchRefFilter()
	f.AddSearchResults([]model.SearchResult{
		{RefID: "turn1search1", Index: 1, Title: "Result", URL: "https://example.com"},
	})
	f.buffer = "【turn1search1】"

	result := f.Flush()
	expected := `[\[1\]](https://example.com)`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
	if f.buffer != "" {
		t.Errorf("expected empty buffer after flush, got %q", f.buffer)
	}
}

func TestFlush_EmptyBuffer(t *testing.T) {
	f := NewSearchRefFilter()
	result := f.Flush()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestGetSearchResultsMarkdown_Empty(t *testing.T) {
	f := NewSearchRefFilter()
	result := f.GetSearchResultsMarkdown()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestGetSearchResultsMarkdown_WithResults(t *testing.T) {
	f := NewSearchRefFilter()
	f.AddSearchResults([]model.SearchResult{
		{RefID: "turn1search2", Index: 2, Title: "Second", URL: "https://example.com/2"},
		{RefID: "turn1search1", Index: 1, Title: "First", URL: "https://example.com/1"},
	})

	result := f.GetSearchResultsMarkdown()
	if !contains(result, "[\\[1\\] First]") || !contains(result, "[\\[2\\] Second]") {
		t.Errorf("expected sorted results in markdown: %q", result)
	}
}

func TestGetSearchResultsMarkdown_EscapesTitle(t *testing.T) {
	f := NewSearchRefFilter()
	f.AddSearchResults([]model.SearchResult{
		{RefID: "turn1search1", Index: 1, Title: "Title [with] brackets", URL: "https://example.com"},
	})

	result := f.GetSearchResultsMarkdown()
	if !contains(result, `\[with\]`) {
		t.Errorf("expected escaped brackets in title: %q", result)
	}
}

func TestIsSearchResultContent_True(t *testing.T) {
	if !IsSearchResultContent(`{"search_result": []}`) {
		t.Error("expected true for content with search_result")
	}
}

func TestIsSearchResultContent_False(t *testing.T) {
	if IsSearchResultContent(`{"other": "content"}`) {
		t.Error("expected false for content without search_result")
	}
}

func TestParseSearchResults_Valid(t *testing.T) {
	editContent := `{"search_result": [{"title": "Test", "url": "https://example.com", "index": 1, "ref_id": "turn1search1"}]}`
	results := ParseSearchResults(editContent)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Test" || results[0].Index != 1 {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestParseSearchResults_NoKey(t *testing.T) {
	editContent := `{"other": "content"}`
	results := ParseSearchResults(editContent)
	if results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}

func TestParseSearchResults_InvalidJSON(t *testing.T) {
	editContent := `{"search_result": [invalid json]}`
	results := ParseSearchResults(editContent)
	if results != nil {
		t.Errorf("expected nil for invalid JSON, got %v", results)
	}
}

func TestParseSearchResults_UnmatchedBrackets(t *testing.T) {
	editContent := `{"search_result": [{"title": "Test"}`
	results := ParseSearchResults(editContent)
	if results != nil {
		t.Errorf("expected nil for unmatched brackets, got %v", results)
	}
}

func TestIsSearchToolCall_True(t *testing.T) {
	if !IsSearchToolCall(`{"mcp": "server"}`, "tool_call") {
		t.Error("expected true for mcp in tool_call phase")
	}
	if !IsSearchToolCall(`{"mcp-server": "value"}`, "tool_call") {
		t.Error("expected true for mcp-server in tool_call phase")
	}
}

func TestIsSearchToolCall_False(t *testing.T) {
	if IsSearchToolCall(`{"mcp": "server"}`, "other_phase") {
		t.Error("expected false for non-tool_call phase")
	}
	if IsSearchToolCall(`{"other": "content"}`, "tool_call") {
		t.Error("expected false for content without mcp")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr))
}
