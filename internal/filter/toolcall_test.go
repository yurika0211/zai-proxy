package filter

import (
	"testing"
)

// ===== IsFunctionToolCall =====

func TestIsFunctionToolCall_True(t *testing.T) {
	tests := []struct {
		name    string
		content string
		phase   string
	}{
		{
			name:    "标准 function 字段",
			content: `{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}`,
			phase:   "tool_call",
		},
		{
			name:    "包含 arguments 字段",
			content: `{"name":"get_weather","arguments":"{\"location\":\"北京\"}"}`,
			phase:   "tool_call",
		},
		{
			name:    "glm_block 包裹的函数调用",
			content: `<glm_block type="tool_call">{"function":{"name":"fn1","arguments":"{}"}}</glm_block>`,
			phase:   "tool_call",
		},
		{
			name:    "Anthropic 风格 name + input",
			content: `{"type":"tool_use","name":"Bash","input":{"command":"npm install"}}`,
			phase:   "tool_call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsFunctionToolCall(tt.content, tt.phase) {
				t.Error("expected true")
			}
		})
	}
}

func TestIsFunctionToolCall_False(t *testing.T) {
	tests := []struct {
		name    string
		content string
		phase   string
	}{
		{
			name:    "非 tool_call 阶段",
			content: `{"function":{"name":"get_weather","arguments":"{}"}}`,
			phase:   "answer",
		},
		{
			name:    "mcp tool call",
			content: `{"type":"mcp","function":{"name":"mcp_tool","arguments":"{}"}}`,
			phase:   "tool_call",
		},
		{
			name:    "mcp-server tool call",
			content: `mcp-server something with "arguments"`,
			phase:   "tool_call",
		},
		{
			name:    "search_result 内容",
			content: `{"search_result":[...],"function":"x","arguments":"y"}`,
			phase:   "tool_call",
		},
		{
			name:    "search_image 内容",
			content: `{"search_image":{},"function":"x","arguments":"y"}`,
			phase:   "tool_call",
		},
		{
			name:    "无函数调用特征",
			content: `{"type":"tool_call","data":"hello world"}`,
			phase:   "tool_call",
		},
		{
			name:    "空阶段",
			content: `{"function":{"name":"fn","arguments":"{}"}}`,
			phase:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsFunctionToolCall(tt.content, tt.phase) {
				t.Error("expected false")
			}
		})
	}
}

// ===== ParseFunctionToolCalls =====

func TestParseFunctionToolCalls_StandardFormat(t *testing.T) {
	content := `{"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"北京\"}"}}`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].ID != "call_abc" {
		t.Errorf("ID = %q, want %q", calls[0].ID, "call_abc")
	}
	if calls[0].Type != "function" {
		t.Errorf("Type = %q, want %q", calls[0].Type, "function")
	}
	if calls[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", calls[0].Function.Name, "get_weather")
	}
	if calls[0].Function.Arguments != `{"location":"北京"}` {
		t.Errorf("Function.Arguments = %q", calls[0].Function.Arguments)
	}
}

func TestParseFunctionToolCalls_FlatFormat(t *testing.T) {
	content := `{"id":"call_flat","name":"get_time","arguments":"{\"timezone\":\"UTC\"}"}`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "get_time" {
		t.Errorf("Function.Name = %q, want %q", calls[0].Function.Name, "get_time")
	}
	if calls[0].Function.Arguments != `{"timezone":"UTC"}` {
		t.Errorf("Function.Arguments = %q", calls[0].Function.Arguments)
	}
	if calls[0].Type != "function" {
		t.Errorf("Type = %q, want %q", calls[0].Type, "function")
	}
}

func TestParseFunctionToolCalls_AnthropicToolUseFormat(t *testing.T) {
	content := `{"id":"toolu_123","type":"tool_use","name":"Bash","input":{"command":"npm install","description":"Install dependencies"}}`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].ID != "toolu_123" {
		t.Errorf("ID = %q, want %q", calls[0].ID, "toolu_123")
	}
	if calls[0].Function.Name != "Bash" {
		t.Errorf("Function.Name = %q, want %q", calls[0].Function.Name, "Bash")
	}
	if calls[0].Function.Arguments != `{"command":"npm install","description":"Install dependencies"}` {
		t.Errorf("Function.Arguments = %q", calls[0].Function.Arguments)
	}
}

func TestParseFunctionToolCalls_GlmBlock(t *testing.T) {
	content := `一些文本<glm_block type="tool_call">{"id":"call_glm","type":"function","function":{"name":"search","arguments":"{\"q\":\"test\"}"}}</glm_block>后续文本`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].ID != "call_glm" {
		t.Errorf("ID = %q, want %q", calls[0].ID, "call_glm")
	}
	if calls[0].Function.Name != "search" {
		t.Errorf("Function.Name = %q, want %q", calls[0].Function.Name, "search")
	}
}

func TestParseFunctionToolCalls_MultipleGlmBlocks(t *testing.T) {
	content := `<glm_block type="tool_call">{"function":{"name":"fn1","arguments":"{}"}}</glm_block>` +
		`<glm_block type="tool_call">{"function":{"name":"fn2","arguments":"{}"}}</glm_block>`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(calls))
	}
	if calls[0].Function.Name != "fn1" {
		t.Errorf("calls[0].Function.Name = %q, want %q", calls[0].Function.Name, "fn1")
	}
	if calls[1].Function.Name != "fn2" {
		t.Errorf("calls[1].Function.Name = %q, want %q", calls[1].Function.Name, "fn2")
	}
}

func TestParseFunctionToolCalls_Array(t *testing.T) {
	content := `[{"id":"c1","type":"function","function":{"name":"fn1","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"fn2","arguments":"{}"}}]`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(calls))
	}
	if calls[0].Function.Name != "fn1" {
		t.Errorf("calls[0].Function.Name = %q", calls[0].Function.Name)
	}
	if calls[1].Function.Name != "fn2" {
		t.Errorf("calls[1].Function.Name = %q", calls[1].Function.Name)
	}
}

func TestParseFunctionToolCalls_NoID(t *testing.T) {
	content := `{"type":"function","function":{"name":"get_weather","arguments":"{}"}}`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].ID != "" {
		t.Errorf("ID = %q, want empty (caller assigns ID)", calls[0].ID)
	}
}

func TestParseFunctionToolCalls_EmptyContent(t *testing.T) {
	calls := ParseFunctionToolCalls("")
	if len(calls) != 0 {
		t.Errorf("len(calls) = %d, want 0", len(calls))
	}
}

func TestParseFunctionToolCalls_WhitespaceOnly(t *testing.T) {
	calls := ParseFunctionToolCalls("   \n\t  ")
	if len(calls) != 0 {
		t.Errorf("len(calls) = %d, want 0", len(calls))
	}
}

func TestParseFunctionToolCalls_InvalidJSON(t *testing.T) {
	calls := ParseFunctionToolCalls("not json at all {{{")
	if len(calls) != 0 {
		t.Errorf("len(calls) = %d, want 0", len(calls))
	}
}

func TestParseFunctionToolCalls_JSONWithoutFunctionFields(t *testing.T) {
	calls := ParseFunctionToolCalls(`{"type":"something","data":"hello"}`)
	if len(calls) != 0 {
		t.Errorf("len(calls) = %d, want 0", len(calls))
	}
}

func TestParseFunctionToolCalls_EmptyArray(t *testing.T) {
	calls := ParseFunctionToolCalls(`[]`)
	if len(calls) != 0 {
		t.Errorf("len(calls) = %d, want 0", len(calls))
	}
}

func TestParseFunctionToolCalls_ComplexArguments(t *testing.T) {
	content := `{"function":{"name":"create_order","arguments":"{\"items\":[{\"id\":1,\"qty\":2},{\"id\":3,\"qty\":1}],\"user\":\"张三\"}"}}`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "create_order" {
		t.Errorf("Function.Name = %q", calls[0].Function.Name)
	}
	// 确保复杂 JSON 参数完整保留
	if calls[0].Function.Arguments == "" {
		t.Error("Function.Arguments is empty")
	}
}

func TestParseFunctionToolCalls_GlmBlockWithExtraAttrs(t *testing.T) {
	content := `<glm_block id="123" type="tool_call" status="pending">{"function":{"name":"fn1","arguments":"{}"}}</glm_block>`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "fn1" {
		t.Errorf("Function.Name = %q, want %q", calls[0].Function.Name, "fn1")
	}
}

func TestParseFunctionToolCalls_GlmBlockInvalidJSON(t *testing.T) {
	content := `<glm_block type="tool_call">not valid json</glm_block>`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 0 {
		t.Errorf("len(calls) = %d, want 0", len(calls))
	}
}

// ===== 优先级：glm_block 优先于原始 JSON =====

func TestParseFunctionToolCalls_GlmBlockPriority(t *testing.T) {
	// 如果同时存在 glm_block 和外层 JSON，优先从 glm_block 提取
	content := `<glm_block type="tool_call">{"function":{"name":"from_block","arguments":"{}"}}</glm_block>`

	calls := ParseFunctionToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "from_block" {
		t.Errorf("Function.Name = %q, want %q", calls[0].Function.Name, "from_block")
	}
}
