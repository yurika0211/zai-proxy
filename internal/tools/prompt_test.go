package tools

import (
	"testing"

	"zai-proxy/internal/model"
)

func TestBuildToolSystemPrompt_ToolChoiceNone(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	prompt := BuildToolSystemPrompt(tools, "none")
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
	// Should contain the "禁止调用" instruction
	found := false
	for _, keyword := range []string{"禁止", "none"} {
		if contains(prompt, keyword) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Prompt should contain tool_choice=none instruction")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceRequired(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	prompt := BuildToolSystemPrompt(tools, "required")
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
	if !contains(prompt, "必须") {
		t.Error("Prompt should contain '必须' for required tool_choice")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceAuto(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	prompt := BuildToolSystemPrompt(tools, "auto")
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
	// auto should not add any extra instruction
	if contains(prompt, "禁止") || contains(prompt, "必须调用工具") {
		t.Error("auto tool_choice should not add restrictive instructions")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceOpenAIFormat(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "get_weather", Description: "Get weather"}},
	}
	toolChoice := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name": "get_weather",
		},
	}
	prompt := BuildToolSystemPrompt(tools, toolChoice)
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
	if !contains(prompt, "get_weather") {
		t.Error("Prompt should mention the specific tool name")
	}
	if !contains(prompt, "必须调用工具") {
		t.Error("Prompt should contain '必须调用工具' for specific function tool_choice")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceAnthropicFormat(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "get_weather", Description: "Get weather"}},
	}
	toolChoice := map[string]interface{}{
		"type": "tool",
		"name": "get_weather",
	}
	prompt := BuildToolSystemPrompt(tools, toolChoice)
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
	if !contains(prompt, "get_weather") {
		t.Error("Prompt should mention the specific tool name")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceMapNone(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	toolChoice := map[string]interface{}{
		"type": "none",
	}
	prompt := BuildToolSystemPrompt(tools, toolChoice)
	if !contains(prompt, "禁止") {
		t.Error("Prompt should contain '禁止' for tool_choice type=none")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceMapRequired(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	toolChoice := map[string]interface{}{
		"type": "required",
	}
	prompt := BuildToolSystemPrompt(tools, toolChoice)
	if !contains(prompt, "必须") {
		t.Error("Prompt should contain '必须' for tool_choice type=required")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceMapAuto(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	toolChoice := map[string]interface{}{
		"type": "auto",
	}
	prompt := BuildToolSystemPrompt(tools, toolChoice)
	if contains(prompt, "禁止") || contains(prompt, "必须调用工具") {
		t.Error("auto tool_choice should not add restrictive instructions")
	}
}

func TestBuildToolSystemPrompt_EmptyTools(t *testing.T) {
	prompt := BuildToolSystemPrompt(nil, nil)
	if prompt != "" {
		t.Error("Prompt should be empty when no tools")
	}
}

func TestBuildToolSystemPrompt_NilToolChoice(t *testing.T) {
	tools := []model.Tool{
		{Type: "function", Function: model.ToolFunction{Name: "test_tool", Description: "A test tool"}},
	}
	prompt := BuildToolSystemPrompt(tools, nil)
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
	// nil tool_choice should not add any extra instruction
	if contains(prompt, "禁止") || contains(prompt, "必须调用工具") {
		t.Error("nil tool_choice should not add restrictive instructions")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}