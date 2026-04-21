package handler

import (
	"net/http"
	"strings"
	"testing"

	"zai-proxy/internal/model"
	toolset "zai-proxy/internal/tools"
)

func TestRunAutoToolLoopExecutesInjectedBuiltin(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	callCount := 0
	var secondRequestMessages []model.Message
	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		callCount++
		switch callCount {
		case 1:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: newFakeBody(
					sseEvent("tool_call", "", `{"type":"function","function":{"name":"get_current_time","arguments":"{\"timezone\":\"UTC\"}"}}`),
					sseEventDone(),
				),
			}, "glm-4.7", nil
		case 2:
			secondRequestMessages = append([]model.Message(nil), messages...)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: newFakeBody(
					sseEvent("answer", "The current UTC time is ready.", ""),
					sseEventDone(),
				),
			}, "glm-4.7", nil
		default:
			t.Fatalf("unexpected upstream call %d", callCount)
			return nil, "", nil
		}
	}

	effectiveTools := toolset.ResolveEffectiveTools("GLM-4.7-tools", nil)
	turn, modelName, err := runAutoToolLoop(
		"token", []model.Message{{Role: "user", Content: "现在几点"}}, "GLM-4.7-tools", effectiveTools.Tools, nil, effectiveTools.InjectedBuiltinNames,
		"call_", model.RequestParams{})
	if err != nil {
		t.Fatalf("runAutoToolLoop error: %v", err)
	}
	if modelName != "glm-4.7" {
		t.Fatalf("modelName = %q, want glm-4.7", modelName)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", callCount)
	}
	if len(turn.ToolCalls) != 0 {
		t.Fatalf("expected final answer without tool calls, got %d tool calls", len(turn.ToolCalls))
	}
	if !strings.Contains(turn.Content, "UTC") {
		t.Fatalf("final content = %q, want UTC answer", turn.Content)
	}

	if len(secondRequestMessages) != 3 {
		t.Fatalf("expected 3 messages on second request, got %d", len(secondRequestMessages))
	}
	assistantMsg := secondRequestMessages[1]
	if assistantMsg.Role != "assistant" || len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call message, got role=%q toolCalls=%d", assistantMsg.Role, len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].Function.Name != "get_current_time" {
		t.Fatalf("tool name = %q, want get_current_time", assistantMsg.ToolCalls[0].Function.Name)
	}
	toolMsg := secondRequestMessages[2]
	if toolMsg.Role != "tool" {
		t.Fatalf("tool role = %q, want tool", toolMsg.Role)
	}
	if toolMsg.ToolCallID == "" {
		t.Fatal("expected tool_call_id to be populated")
	}
	if !strings.Contains(toolMsg.Content.(string), `"tool":"get_current_time"`) {
		t.Fatalf("tool content = %q, want get_current_time execution result", toolMsg.Content)
	}
}

func TestRunAutoToolLoopLeavesUnknownToolCallsForClient(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	callCount := 0
	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("tool_call", "", `{"type":"function","function":{"name":"get_weather","arguments":"{}"}}`),
				sseEventDone(),
			),
		}, "glm-4.7", nil
	}

	effectiveTools := toolset.ResolveEffectiveTools("GLM-4.7-tools", nil)
	turn, _, err := runAutoToolLoop(
		"token", []model.Message{{Role: "user", Content: "天气如何"}}, "GLM-4.7-tools", effectiveTools.Tools, nil, effectiveTools.InjectedBuiltinNames,
		"call_", model.RequestParams{})
	if err != nil {
		t.Fatalf("runAutoToolLoop error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 upstream call, got %d", callCount)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("expected tool call to be returned to client, got %d", len(turn.ToolCalls))
	}
	if turn.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("tool name = %q, want get_weather", turn.ToolCalls[0].Function.Name)
	}
}
