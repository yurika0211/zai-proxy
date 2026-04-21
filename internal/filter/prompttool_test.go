package filter

import (
	"testing"
)

func TestExtractPromptToolCalls_NoToolCall(t *testing.T) {
	content := "Hello, this is a normal response."
	clean, calls := ExtractPromptToolCalls(content)
	if clean != content {
		t.Errorf("expected content unchanged, got %q", clean)
	}
	if len(calls) != 0 {
		t.Error("expected no tool calls")
	}
}

func TestExtractPromptToolCalls_SingleCall(t *testing.T) {
	content := `Here is the result:
<tool_call>{"name": "get_weather", "arguments": {"city": "Beijing"}}</tool_call>
Done.`

	clean, calls := ExtractPromptToolCalls(content)

	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "get_weather" {
		t.Errorf("expected name get_weather, got %s", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"city":"Beijing"}` && calls[0].Function.Arguments != `{"city": "Beijing"}` {
		t.Errorf("unexpected arguments: %s", calls[0].Function.Arguments)
	}
	if calls[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
	if calls[0].Type != "function" {
		t.Errorf("expected type function, got %s", calls[0].Type)
	}
	// Clean content should not contain tool_call tags
	if clean == content {
		t.Error("expected content to be cleaned")
	}
	if contains := "Here is the result:"; !containsStr(clean, contains) {
		t.Errorf("expected clean content to contain %q", contains)
	}
	if containsStr(clean, "<tool_call>") {
		t.Error("clean content should not contain <tool_call>")
	}
}

func TestExtractPromptToolCalls_MultipleCalls(t *testing.T) {
	content := `<tool_call>{"name": "func_a", "arguments": {"x": 1}}</tool_call>
<tool_call>{"name": "func_b", "arguments": {"y": 2}}</tool_call>`

	clean, calls := ExtractPromptToolCalls(content)

	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "func_a" {
		t.Errorf("expected first call func_a, got %s", calls[0].Function.Name)
	}
	if calls[1].Function.Name != "func_b" {
		t.Errorf("expected second call func_b, got %s", calls[1].Function.Name)
	}
	if calls[0].Index != 0 || calls[1].Index != 1 {
		t.Error("expected sequential indices")
	}
	if clean != "" {
		t.Errorf("expected empty clean content, got %q", clean)
	}
}

func TestExtractPromptToolCalls_OnlyToolCall(t *testing.T) {
	content := `<tool_call>{"name": "calculate", "arguments": {"expression": "2+2"}}</tool_call>`

	clean, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "calculate" {
		t.Errorf("expected calculate, got %s", calls[0].Function.Name)
	}
	if clean != "" {
		t.Errorf("expected empty clean content, got %q", clean)
	}
}

func TestExtractPromptToolCalls_WithWhitespace(t *testing.T) {
	content := `<tool_call>
  {"name": "test", "arguments": {}}
</tool_call>`

	_, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "test" {
		t.Errorf("expected test, got %s", calls[0].Function.Name)
	}
}

func TestExtractPromptToolCalls_AnthropicInputFormat(t *testing.T) {
	content := `<tool_call>{"name": "Bash", "input": {"command": "npm install", "description": "Install dependencies"}}</tool_call>`

	clean, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "Bash" {
		t.Errorf("expected Bash, got %s", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"command": "npm install", "description": "Install dependencies"}` && calls[0].Function.Arguments != `{"command":"npm install","description":"Install dependencies"}` {
		t.Errorf("unexpected arguments: %s", calls[0].Function.Arguments)
	}
	if clean != "" {
		t.Errorf("expected empty clean content, got %q", clean)
	}
}

func TestHasPromptToolCallOpen(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"hello", false},
		{"<tool_call>{}", true},
		{"<tool_call>{}</tool_call>", false},
		{"text <tool_call>partial...", true},
		{"<tool_call>a</tool_call><tool_call>b", true},
		{"[TOOL]partial", true},
		{"[TOOL]{\"name\":\"x\"}[/TOOL]", false},
		{"[TOOL_CALL]partial", true},
		{"[TOOL_CALL]{\"name\":\"x\"}[/TOOL_CALL]", false},
	}
	for _, tt := range tests {
		if got := HasPromptToolCallOpen(tt.content); got != tt.expected {
			t.Errorf("HasPromptToolCallOpen(%q) = %v, want %v", tt.content, got, tt.expected)
		}
	}
}

// ===== [TOOL]...[/TOOL] 格式 =====

func TestExtractPromptToolCalls_AltToolFormat(t *testing.T) {
	content := `[TOOL]{"name": "get_weather", "arguments": {"city": "上海"}}[/TOOL]`

	clean, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", calls[0].Function.Name)
	}
	if calls[0].Type != "function" {
		t.Errorf("expected type function, got %s", calls[0].Type)
	}
	if clean != "" {
		t.Errorf("expected empty clean, got %q", clean)
	}
}

func TestExtractPromptToolCalls_AltToolCallFormat(t *testing.T) {
	content := `好的，我来调用工具。
[TOOL_CALL]{"name": "create_file", "arguments": {"filename": "test.txt", "content": "hello"}}[/TOOL_CALL]`

	clean, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "create_file" {
		t.Errorf("expected create_file, got %s", calls[0].Function.Name)
	}
	if !containsStr(clean, "好的") {
		t.Errorf("expected clean to contain surrounding text, got %q", clean)
	}
}

// ===== markdown JSON block 格式 =====

func TestExtractPromptToolCalls_JsonBlockFormat(t *testing.T) {
	content := "我来调用工具：\n```json\n{\"name\": \"get_weather\", \"arguments\": {\"city\": \"北京\"}}\n```\n"

	clean, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", calls[0].Function.Name)
	}
	if containsStr(clean, "```") {
		t.Errorf("expected clean to not contain code block, got %q", clean)
	}
}

// ===== 混合格式 =====

func TestExtractPromptToolCalls_MixedFormats(t *testing.T) {
	content := `<tool_call>{"name": "func_a", "arguments": {}}</tool_call>
[TOOL]{"name": "func_b", "arguments": {}}[/TOOL]`

	_, calls := ExtractPromptToolCalls(content)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	// <tool_call> 优先被解析
	names := map[string]bool{}
	for _, c := range calls {
		names[c.Function.Name] = true
	}
	if !names["func_a"] || !names["func_b"] {
		t.Error("expected both func_a and func_b to be extracted")
	}
}

// ===== <tool_call> 优先于其他格式 =====

func TestExtractPromptToolCalls_ToolCallPriority(t *testing.T) {
	content := `<tool_call>{"name": "correct", "arguments": {}}</tool_call>`

	_, calls := ExtractPromptToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "correct" {
		t.Errorf("expected correct, got %s", calls[0].Function.Name)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
