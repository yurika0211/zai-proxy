package tools

import (
	"strings"
	"testing"

	"zai-proxy/internal/model"
)

func TestBuildToolSystemPrompt_Basic(t *testing.T) {
	tools := []model.Tool{
		{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"city"},
				},
			},
		},
	}

	result := BuildToolSystemPrompt(tools, nil)

	if !strings.Contains(result, "get_weather") {
		t.Error("should contain tool name")
	}
	if !strings.Contains(result, "Get current weather") {
		t.Error("should contain description")
	}
	if !strings.Contains(result, "<tool_call>") {
		t.Error("should contain format instruction")
	}
	if !strings.Contains(result, "city") {
		t.Error("should contain parameter info")
	}
}

func TestBuildToolSystemPrompt_Empty(t *testing.T) {
	result := BuildToolSystemPrompt(nil, nil)
	if result != "" {
		t.Error("should return empty for nil tools")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceNone(t *testing.T) {
	tools := []model.Tool{{
		Type:     "function",
		Function: model.ToolFunction{Name: "test"},
	}}

	result := BuildToolSystemPrompt(tools, "none")
	if !strings.Contains(result, "禁止调用任何工具") {
		t.Error("should instruct not to call tools")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceRequired(t *testing.T) {
	tools := []model.Tool{{
		Type:     "function",
		Function: model.ToolFunction{Name: "test"},
	}}

	result := BuildToolSystemPrompt(tools, "required")
	if !strings.Contains(result, "必须包含至少一个") {
		t.Error("should instruct to call at least one tool")
	}
}

func TestBuildToolSystemPrompt_ToolChoiceSpecific(t *testing.T) {
	tools := []model.Tool{{
		Type:     "function",
		Function: model.ToolFunction{Name: "get_weather"},
	}}

	choice := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name": "get_weather",
		},
	}

	result := BuildToolSystemPrompt(tools, choice)
	if !strings.Contains(result, `必须调用工具 "get_weather"`) {
		t.Error("should instruct to call specific tool")
	}
}

func TestBuildToolSystemPrompt_WithExecCommand(t *testing.T) {
	tools := []model.Tool{{
		Type:     "function",
		Function: model.ToolFunction{Name: "exec_command"},
	}}

	result := BuildToolSystemPrompt(tools, nil)
	if !strings.Contains(result, "不要假装已经执行") {
		t.Error("should instruct the model not to pretend command execution")
	}
	if !strings.Contains(result, "shell 语法") {
		t.Error("should mention shell syntax restrictions")
	}
}

func TestConvertToolCallToText(t *testing.T) {
	toolCalls := []model.ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: model.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"city":"Beijing"}`,
			},
		},
	}

	result := ConvertToolCallToText(toolCalls)
	if !strings.Contains(result, "<tool_call>") {
		t.Error("should contain <tool_call> tag")
	}
	if !strings.Contains(result, "get_weather") {
		t.Error("should contain function name")
	}
	if !strings.Contains(result, "Beijing") {
		t.Error("should contain arguments")
	}
}

func TestConvertToolResultToText(t *testing.T) {
	result := ConvertToolResultToText("call_123", `{"temp": 25}`)
	if !strings.Contains(result, "call_123") {
		t.Error("should contain call ID")
	}
	if !strings.Contains(result, `{"temp": 25}`) {
		t.Error("should contain result content")
	}
	if !strings.Contains(result, "<tool_result") {
		t.Error("should contain <tool_result> tag")
	}
}
