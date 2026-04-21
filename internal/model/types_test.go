package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// ===== Tool 类型序列化/反序列化 =====

func TestToolJSON(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "get_weather",
			Description: "获取天气信息",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "城市名称",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal Tool: %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal Tool: %v", err)
	}

	if decoded.Type != "function" {
		t.Errorf("Type = %q, want %q", decoded.Type, "function")
	}
	if decoded.Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", decoded.Function.Name, "get_weather")
	}
	if decoded.Function.Description != "获取天气信息" {
		t.Errorf("Function.Description = %q, want %q", decoded.Function.Description, "获取天气信息")
	}
}

func TestToolCallJSON(t *testing.T) {
	tc := ToolCall{
		ID:   "call_abc123",
		Type: "function",
		Function: FunctionCall{
			Name:      "get_weather",
			Arguments: `{"location":"北京"}`,
		},
		Index: 0,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal ToolCall: %v", err)
	}

	var decoded ToolCall
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal ToolCall: %v", err)
	}

	if decoded.ID != "call_abc123" {
		t.Errorf("ID = %q, want %q", decoded.ID, "call_abc123")
	}
	if decoded.Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", decoded.Function.Name, "get_weather")
	}
	if decoded.Function.Arguments != `{"location":"北京"}` {
		t.Errorf("Function.Arguments = %q, want %q", decoded.Function.Arguments, `{"location":"北京"}`)
	}
}

// ===== ChatRequest 带 Tools 序列化 =====

func TestChatRequestWithTools(t *testing.T) {
	reqJSON := `{
		"model": "GLM-4.7",
		"messages": [{"role": "user", "content": "北京天气怎么样？"}],
		"stream": true,
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "获取天气",
				"parameters": {"type": "object", "properties": {"location": {"type": "string"}}}
			}
		}],
		"tool_choice": "auto"
	}`

	var req ChatRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal ChatRequest: %v", err)
	}

	if req.Model != "GLM-4.7" {
		t.Errorf("Model = %q, want %q", req.Model, "GLM-4.7")
	}
	if len(req.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(req.Tools))
	}
	if req.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", req.Tools[0].Function.Name, "get_weather")
	}
	if req.ToolChoice != "auto" {
		t.Errorf("ToolChoice = %v, want %q", req.ToolChoice, "auto")
	}
}

func TestChatRequestWithoutTools(t *testing.T) {
	reqJSON := `{
		"model": "GLM-4.6",
		"messages": [{"role": "user", "content": "hello"}],
		"stream": false
	}`

	var req ChatRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal ChatRequest: %v", err)
	}

	if len(req.Tools) != 0 {
		t.Errorf("len(Tools) = %d, want 0", len(req.Tools))
	}
	if req.ToolChoice != nil {
		t.Errorf("ToolChoice = %v, want nil", req.ToolChoice)
	}
}

func TestChatRequestToolChoiceObject(t *testing.T) {
	reqJSON := `{
		"model": "GLM-4.7",
		"messages": [{"role": "user", "content": "test"}],
		"stream": false,
		"tools": [{"type": "function", "function": {"name": "fn1"}}],
		"tool_choice": {"type": "function", "function": {"name": "fn1"}}
	}`

	var req ChatRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tc, ok := req.ToolChoice.(map[string]interface{})
	if !ok {
		t.Fatalf("ToolChoice type = %T, want map[string]interface{}", req.ToolChoice)
	}
	if tc["type"] != "function" {
		t.Errorf("ToolChoice.type = %v, want %q", tc["type"], "function")
	}
}

// ===== Message 带 ToolCallID / ToolCalls 序列化 =====

func TestMessageWithToolCallID(t *testing.T) {
	msgJSON := `{
		"role": "tool",
		"content": "{\"temperature\": 25}",
		"tool_call_id": "call_abc123"
	}`

	var msg Message
	if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg.Role != "tool" {
		t.Errorf("Role = %q, want %q", msg.Role, "tool")
	}
	if msg.ToolCallID != "call_abc123" {
		t.Errorf("ToolCallID = %q, want %q", msg.ToolCallID, "call_abc123")
	}
}

func TestMessageWithToolCalls(t *testing.T) {
	msgJSON := `{
		"role": "assistant",
		"content": "",
		"tool_calls": [{
			"id": "call_xyz",
			"type": "function",
			"function": {"name": "get_weather", "arguments": "{\"location\":\"上海\"}"},
			"index": 0
		}]
	}`

	var msg Message
	if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Function.Name = %q, want %q", msg.ToolCalls[0].Function.Name, "get_weather")
	}
}

// ===== ToUpstreamMessage =====

func TestToUpstreamMessage_ToolRole(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    `{"temperature": 25}`,
		ToolCallID: "call_abc",
	}

	result := msg.ToUpstreamMessage(nil)

	if result["role"] != "tool" {
		t.Errorf("role = %v, want %q", result["role"], "tool")
	}
	if result["tool_call_id"] != "call_abc" {
		t.Errorf("tool_call_id = %v, want %q", result["tool_call_id"], "call_abc")
	}
	if result["content"] != `{"temperature": 25}` {
		t.Errorf("content = %v, want %q", result["content"], `{"temperature": 25}`)
	}
}

func TestToUpstreamMessage_AssistantWithToolCalls(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location":"北京"}`,
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_time",
					Arguments: `{"timezone":"Asia/Shanghai"}`,
				},
			},
		},
	}

	result := msg.ToUpstreamMessage(nil)

	if result["role"] != "assistant" {
		t.Errorf("role = %v, want %q", result["role"], "assistant")
	}

	toolCalls, ok := result["tool_calls"].([]map[string]interface{})
	if !ok {
		t.Fatalf("tool_calls type = %T, want []map[string]interface{}", result["tool_calls"])
	}
	if len(toolCalls) != 2 {
		t.Fatalf("len(tool_calls) = %d, want 2", len(toolCalls))
	}
	if toolCalls[0]["id"] != "call_1" {
		t.Errorf("tool_calls[0].id = %v, want %q", toolCalls[0]["id"], "call_1")
	}
	fn, ok := toolCalls[0]["function"].(map[string]interface{})
	if !ok {
		t.Fatalf("function type = %T", toolCalls[0]["function"])
	}
	if fn["name"] != "get_weather" {
		t.Errorf("function.name = %v, want %q", fn["name"], "get_weather")
	}
}

func TestToUpstreamMessage_PlainUser(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "hello",
	}

	result := msg.ToUpstreamMessage(nil)
	if result["role"] != "user" {
		t.Errorf("role = %v, want %q", result["role"], "user")
	}
	if result["content"] != "hello" {
		t.Errorf("content = %v, want %q", result["content"], "hello")
	}
	if _, exists := result["tool_call_id"]; exists {
		t.Error("tool_call_id should not be present for user messages")
	}
	if _, exists := result["tool_calls"]; exists {
		t.Error("tool_calls should not be present for user messages")
	}
}

func TestToUpstreamMessage_AssistantWithoutToolCalls(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "你好！",
	}

	result := msg.ToUpstreamMessage(nil)
	if result["role"] != "assistant" {
		t.Errorf("role = %v, want %q", result["role"], "assistant")
	}
	if result["content"] != "你好！" {
		t.Errorf("content = %v, want %q", result["content"], "你好！")
	}
	if _, exists := result["tool_calls"]; exists {
		t.Error("tool_calls should not be present when empty")
	}
}

// ===== Delta / MessageResp 带 ToolCalls =====

func TestDeltaWithToolCalls(t *testing.T) {
	delta := Delta{
		ToolCalls: []ToolCall{{
			ID:    "call_1",
			Type:  "function",
			Index: 0,
			Function: FunctionCall{
				Name:      "get_weather",
				Arguments: `{"location":"北京"}`,
			},
		}},
	}

	data, err := json.Marshal(delta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Delta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Name = %q, want %q", decoded.ToolCalls[0].Function.Name, "get_weather")
	}
}

func TestDeltaOmitsEmptyToolCalls(t *testing.T) {
	delta := Delta{Content: "hello"}

	data, err := json.Marshal(delta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// tool_calls 为空时应被 omitempty 省略
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, exists := raw["tool_calls"]; exists {
		t.Error("tool_calls should be omitted when empty")
	}
}

func TestMessageRespWithToolCalls(t *testing.T) {
	resp := MessageResp{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{{
			ID:    "call_1",
			Type:  "function",
			Index: 0,
			Function: FunctionCall{
				Name:      "search",
				Arguments: `{"query":"test"}`,
			},
		}},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded MessageResp
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Function.Arguments != `{"query":"test"}` {
		t.Errorf("Arguments = %q", decoded.ToolCalls[0].Function.Arguments)
	}
}

func TestMessageRespOmitsEmptyToolCalls(t *testing.T) {
	resp := MessageResp{
		Role:    "assistant",
		Content: "hello world",
	}

	data, _ := json.Marshal(resp)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, exists := raw["tool_calls"]; exists {
		t.Error("tool_calls should be omitted when empty")
	}
}

// ===== ChatCompletionChunk 带 tool_calls finish_reason =====

func TestChunkWithToolCallsFinishReason(t *testing.T) {
	reason := "tool_calls"
	chunk := ChatCompletionChunk{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1000,
		Model:   "glm-4.7",
		Choices: []Choice{{
			Index:        0,
			Delta:        &Delta{},
			FinishReason: &reason,
		}},
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ChatCompletionChunk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Choices[0].FinishReason == nil {
		t.Fatal("FinishReason is nil")
	}
	if *decoded.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", *decoded.Choices[0].FinishReason, "tool_calls")
	}
}

// ===== ChatCompletionResponse 带 tool_calls =====

func TestCompletionResponseWithToolCalls(t *testing.T) {
	reason := "tool_calls"
	resp := ChatCompletionResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: 1000,
		Model:   "glm-4.7",
		Choices: []Choice{{
			Index: 0,
			Message: &MessageResp{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{{
					ID:    "call_1",
					Type:  "function",
					Index: 0,
					Function: FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"北京"}`,
					},
				}},
			},
			FinishReason: &reason,
		}},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ChatCompletionResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Choices) != 1 {
		t.Fatalf("len(Choices) = %d", len(decoded.Choices))
	}
	msg := decoded.Choices[0].Message
	if msg == nil {
		t.Fatal("Message is nil")
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q", msg.ToolCalls[0].Function.Name)
	}
	if *decoded.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q", *decoded.Choices[0].FinishReason)
	}
}

// ===== Delta 指针：nil 时不出现在 JSON 中 =====

func TestChoiceDeltaNil_OmittedInJSON(t *testing.T) {
	reason := "stop"
	choice := Choice{
		Index: 0,
		Message: &MessageResp{
			Role:    "assistant",
			Content: "hello",
		},
		FinishReason: &reason,
	}

	data, _ := json.Marshal(choice)
	s := string(data)
	if strings.Contains(s, `"delta"`) {
		t.Errorf("nil Delta should be omitted, got: %s", s)
	}
}

// ===== Delta 指针：非 nil 时正常序列化 =====

func TestChoiceDeltaNotNil_SerializedInJSON(t *testing.T) {
	choice := Choice{
		Index: 0,
		Delta: &Delta{Content: "test content"},
	}

	data, _ := json.Marshal(choice)
	s := string(data)
	if !strings.Contains(s, `"delta"`) {
		t.Error("non-nil Delta should appear in JSON")
	}
	if !strings.Contains(s, `"test content"`) {
		t.Error("Delta content should be serialized")
	}
}
