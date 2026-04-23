package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zai-proxy/internal/config"
	"zai-proxy/internal/model"
)

// ===== handleAnthropicNonStream =====

func TestHandleAnthropicNonStream_SimpleAnswer(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "Hello", ""),
		sseEvent("answer", " World", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleAnthropicNonStream(w, body, "msg_test", "glm-4.7", "claude-sonnet-4-6", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != "msg_test" {
		t.Errorf("id = %q, want msg_test", resp.ID)
	}
	if resp.Type != "message" {
		t.Errorf("type = %q, want message", resp.Type)
	}
	if resp.Role != "assistant" {
		t.Errorf("role = %q, want assistant", resp.Role)
	}
	if resp.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", resp.Model)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want end_turn", resp.StopReason)
	}
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" {
		t.Fatalf("content = %+v, want 1 text block", resp.Content)
	}
	if resp.Content[0].Text != "Hello World" {
		t.Errorf("text = %q, want 'Hello World'", resp.Content[0].Text)
	}
}

func TestHandleAnthropicNonStream_WithThinking(t *testing.T) {
	body := newFakeBody(
		sseEvent("thinking", "> Let me think...", ""),
		sseEvent("answer", "The answer is 42", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleAnthropicNonStream(w, body, "msg_think", "glm-4.7", "claude-sonnet-4-6", nil)

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Content) < 2 {
		t.Fatalf("expected at least 2 content blocks (thinking + text), got %d", len(resp.Content))
	}
	hasThinking := false
	hasText := false
	for _, block := range resp.Content {
		if block.Type == "thinking" {
			hasThinking = true
		}
		if block.Type == "text" {
			hasText = true
		}
	}
	if !hasThinking {
		t.Error("expected thinking block")
	}
	if !hasText {
		t.Error("expected text block")
	}
}

func TestHandleAnthropicNonStream_WithToolCalls(t *testing.T) {
	toolCallJSON := `{"id":"call_test","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"北京\"}"}}`
	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	tools := []model.Tool{{
		Type:     "function",
		Function: model.ToolFunction{Name: "get_weather"},
	}}

	w := httptest.NewRecorder()
	handleAnthropicNonStream(w, body, "msg_tool", "glm-4.7", "claude-sonnet-4-6", tools)

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q, want tool_use", resp.StopReason)
	}
	hasToolUse := false
	for _, block := range resp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			if block.Name != "get_weather" {
				t.Errorf("tool name = %q, want get_weather", block.Name)
			}
		}
	}
	if !hasToolUse {
		t.Error("expected tool_use block")
	}
}

func TestHandleAnthropicNonStream_EmptyContent(t *testing.T) {
	body := newFakeBody(
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleAnthropicNonStream(w, body, "msg_empty", "glm-4.7", "claude-sonnet-4-6", nil)

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Should have at least one text block (empty)
	if len(resp.Content) == 0 {
		t.Error("expected at least one content block for empty response")
	}
}

func TestHandleAnthropicNonStream_PromptToolCalls(t *testing.T) {
	// Test prompt-injected tool calls (◇ format extracted from answer text)
	body := newFakeBody(
		sseEvent("answer", "I'll check the weather. ◇get_weather◇{\"location\":\"Tokyo\"}◇", ""),
		sseEventDone(),
	)

	tools := []model.Tool{{
		Type:     "function",
		Function: model.ToolFunction{Name: "get_weather"},
	}}

	w := httptest.NewRecorder()
	handleAnthropicNonStream(w, body, "msg_ptool", "glm-4.7", "claude-sonnet-4-6", tools)

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q, want tool_use", resp.StopReason)
	}
}

// ===== convertAnthropicToInternal =====

func TestConvertAnthropicToInternal_SystemString(t *testing.T) {
	req := model.AnthropicRequest{
		System: "You are helpful",
		Messages: []model.AnthropicMessage{{
			Role:    "user",
			Content: "hello",
		}},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", messages[0].Role)
	}
	if messages[0].Content != "You are helpful" {
		t.Errorf("system content = %q", messages[0].Content)
	}
}

func TestConvertAnthropicToInternal_SystemArray(t *testing.T) {
	req := model.AnthropicRequest{
		System: []interface{}{
			map[string]interface{}{"type": "text", "text": "Be helpful"},
			map[string]interface{}{"type": "text", "text": " Be concise"},
		},
		Messages: []model.AnthropicMessage{{
			Role:    "user",
			Content: "hello",
		}},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", messages[0].Role)
	}
}

func TestConvertAnthropicToInternal_AssistantWithToolUse(t *testing.T) {
	req := model.AnthropicRequest{
		Messages: []model.AnthropicMessage{
			{
				Role:    "user",
				Content: "check weather",
			},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "Let me check."},
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "toolu_123",
						"name":  "get_weather",
						"input": map[string]interface{}{"location": "Tokyo"},
					},
				},
			},
		},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[1].Role != "assistant" {
		t.Errorf("second message role = %q, want assistant", messages[1].Role)
	}
	if len(messages[1].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(messages[1].ToolCalls))
	}
	if messages[1].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool name = %q, want get_weather", messages[1].ToolCalls[0].Function.Name)
	}
}

func TestConvertAnthropicToInternal_AssistantWithThinking(t *testing.T) {
	req := model.AnthropicRequest{
		Messages: []model.AnthropicMessage{
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "thinking", "thinking": "hmm"},
					map[string]interface{}{"type": "text", "text": "answer"},
				},
			},
		},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	// Thinking blocks should be skipped
	if messages[0].Content != "answer" {
		t.Errorf("content = %q, want answer", messages[0].Content)
	}
}

func TestConvertAnthropicToInternal_ToolChoiceAuto(t *testing.T) {
	req := model.AnthropicRequest{
		ToolChoice: map[string]interface{}{"type": "auto"},
	}

	_, _, toolChoice := convertAnthropicToInternal(req)
	if toolChoice != "auto" {
		t.Errorf("toolChoice = %v, want auto", toolChoice)
	}
}

func TestConvertAnthropicToInternal_ToolChoiceAny(t *testing.T) {
	req := model.AnthropicRequest{
		ToolChoice: map[string]interface{}{"type": "any"},
	}

	_, _, toolChoice := convertAnthropicToInternal(req)
	if toolChoice != "required" {
		t.Errorf("toolChoice = %v, want required", toolChoice)
	}
}

func TestConvertAnthropicToInternal_ToolChoiceNone(t *testing.T) {
	req := model.AnthropicRequest{
		ToolChoice: map[string]interface{}{"type": "none"},
	}

	_, _, toolChoice := convertAnthropicToInternal(req)
	if toolChoice != "none" {
		t.Errorf("toolChoice = %v, want none", toolChoice)
	}
}

func TestConvertAnthropicToInternal_ToolChoiceNamed(t *testing.T) {
	req := model.AnthropicRequest{
		ToolChoice: map[string]interface{}{"type": "tool", "name": "get_weather"},
	}

	_, _, toolChoice := convertAnthropicToInternal(req)
	tcMap, ok := toolChoice.(map[string]interface{})
	if !ok {
		t.Fatalf("toolChoice type = %T, want map", toolChoice)
	}
	if tcMap["type"] != "function" {
		t.Errorf("toolChoice type = %v, want function", tcMap["type"])
	}
}

// ===== anthropicToolName =====

func TestAnthropicToolName_Explicit(t *testing.T) {
	result := anthropicToolName(model.AnthropicTool{Name: "my_tool"})
	if result != "my_tool" {
		t.Errorf("expected my_tool, got %q", result)
	}
}

func TestAnthropicToolName_BashType(t *testing.T) {
	result := anthropicToolName(model.AnthropicTool{Type: "bash_20250124"})
	if result != "bash" {
		t.Errorf("expected bash, got %q", result)
	}
}

func TestAnthropicToolName_TextEditorType(t *testing.T) {
	result := anthropicToolName(model.AnthropicTool{Type: "text_editor_20250728"})
	if result != "str_replace_based_edit_tool" {
		t.Errorf("expected str_replace_based_edit_tool, got %q", result)
	}
}

func TestAnthropicToolName_UnknownType(t *testing.T) {
	result := anthropicToolName(model.AnthropicTool{Type: "custom_20250101"})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// ===== anthropicToolDescription =====

func TestAnthropicToolDescription_Explicit(t *testing.T) {
	result := anthropicToolDescription(model.AnthropicTool{Description: "My tool"})
	if result != "My tool" {
		t.Errorf("expected My tool, got %q", result)
	}
}

func TestAnthropicToolDescription_BashType(t *testing.T) {
	result := anthropicToolDescription(model.AnthropicTool{Type: "bash_20250124"})
	if !strings.Contains(result, "bash") {
		t.Errorf("expected bash description, got %q", result)
	}
}

func TestAnthropicToolDescription_TextEditorType(t *testing.T) {
	result := anthropicToolDescription(model.AnthropicTool{Type: "text_editor_20250728"})
	if !strings.Contains(result, "text") {
		t.Errorf("expected text editor description, got %q", result)
	}
}

func TestAnthropicToolDescription_UnknownType(t *testing.T) {
	result := anthropicToolDescription(model.AnthropicTool{Type: "custom_20250101"})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// ===== anthropicToolInputSchema =====

func TestAnthropicToolInputSchema_Explicit(t *testing.T) {
	schema := map[string]interface{}{"type": "object"}
	result := anthropicToolInputSchema(model.AnthropicTool{InputSchema: schema})
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["type"] != "object" {
		t.Errorf("expected object type, got %v", m["type"])
	}
}

func TestAnthropicToolInputSchema_BashType(t *testing.T) {
	result := anthropicToolInputSchema(model.AnthropicTool{Type: "bash_20250124"})
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["type"] != "object" {
		t.Errorf("expected object type, got %v", m["type"])
	}
}

func TestAnthropicToolInputSchema_TextEditorType(t *testing.T) {
	result := anthropicToolInputSchema(model.AnthropicTool{Type: "text_editor_20250728"})
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["type"] != "object" {
		t.Errorf("expected object type, got %v", m["type"])
	}
}

func TestAnthropicToolInputSchema_UnknownType(t *testing.T) {
	result := anthropicToolInputSchema(model.AnthropicTool{Type: "custom_20250101"})
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// ===== extractAnthropicToolResultContent =====

func TestExtractAnthropicToolResultContent_String(t *testing.T) {
	result := extractAnthropicToolResultContent("hello")
	if result != "hello" {
		t.Errorf("expected hello, got %q", result)
	}
}

func TestExtractAnthropicToolResultContent_ArrayWithText(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "output1"},
		map[string]interface{}{"type": "text", "text": "output2"},
	}
	result := extractAnthropicToolResultContent(content)
	if result != "output1output2" {
		t.Errorf("expected output1output2, got %q", result)
	}
}

func TestExtractAnthropicToolResultContent_ArrayWithoutText(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "image", "url": "http://example.com"},
	}
	result := extractAnthropicToolResultContent(content)
	if result == "" {
		t.Error("expected non-empty JSON fallback")
	}
}

func TestExtractAnthropicToolResultContent_Map(t *testing.T) {
	content := map[string]interface{}{"stdout": "ok", "stderr": ""}
	result := extractAnthropicToolResultContent(content)
	if !strings.Contains(result, "stdout") {
		t.Errorf("expected JSON with stdout, got %q", result)
	}
}

func TestExtractAnthropicToolResultContent_Nil(t *testing.T) {
	result := extractAnthropicToolResultContent(nil)
	if result != "null" {
		t.Errorf("expected 'null', got %q", result)
	}
}

// ===== anthropicImageBlockToDataURL =====

func TestAnthropicImageBlockToDataURL_NoSource(t *testing.T) {
	result := anthropicImageBlockToDataURL(model.AnthropicContentBlock{})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestAnthropicImageBlockToDataURL_NonBase64(t *testing.T) {
	result := anthropicImageBlockToDataURL(model.AnthropicContentBlock{
		Source: &model.AnthropicImageSource{Type: "url", MediaType: "image/png", Data: "abc"},
	})
	if result != "" {
		t.Errorf("expected empty for non-base64 source, got %q", result)
	}
}

func TestAnthropicImageBlockToDataURL_Base64(t *testing.T) {
	result := anthropicImageBlockToDataURL(model.AnthropicContentBlock{
		Source: &model.AnthropicImageSource{Type: "base64", MediaType: "image/png", Data: "QUJD"},
	})
	if result != "data:image/png;base64,QUJD" {
		t.Errorf("expected data:image/png;base64,QUJD, got %q", result)
	}
}

// ===== HandleMessages =====

func TestHandleMessages_MissingAPIKey(t *testing.T) {
	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleMessages_InvalidBody(t *testing.T) {
	body := strings.NewReader(`{invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("x-api-key", "test-token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleMessages_AuthorizationBearer(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		if token != "bearer-token" {
			t.Fatalf("token = %q, want bearer-token", token)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("answer", "hello", ""),
				sseEventDone(),
			),
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("Authorization", "Bearer bearer-token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestHandleMessages_UpstreamError(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		return nil, "", &upstreamStatusError{StatusCode: 503, Body: "service unavailable"}
	}

	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("x-api-key", "test-token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

// ===== HandleChatCompletions =====

func TestHandleChatCompletions_MissingToken(t *testing.T) {
	body := strings.NewReader(`{"model":"GLM-4.7","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	w := httptest.NewRecorder()

	HandleChatCompletions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleChatCompletions_InvalidBody(t *testing.T) {
	body := strings.NewReader(`{invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleChatCompletions_DefaultModel(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	var capturedModel string
	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		capturedModel = modelName
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("answer", "hello", ""),
				sseEventDone(),
			),
		}, "glm-4.6", nil
	}

	// Empty model should default to GLM-4.6
	body := strings.NewReader(`{"model":"","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if capturedModel != "GLM-4.6" {
		t.Errorf("model = %q, want GLM-4.6", capturedModel)
	}
}

func TestHandleChatCompletions_UpstreamError(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		return nil, "", &upstreamStatusError{StatusCode: 503, Body: "service unavailable"}
	}

	body := strings.NewReader(`{"model":"GLM-4.7","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	HandleChatCompletions(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestHandleChatCompletions_UpstreamHTTPError(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       newFakeBody(`rate limited`),
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"GLM-4.7","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	HandleChatCompletions(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
}

// ===== resolveAllowedOrigin =====

func TestResolveAllowedOrigin_EmptyOrigins(t *testing.T) {
	config.SetConfigForTest(&config.Config{AllowedOrigins: nil})
	defer config.SetConfigForTest(nil)

	_, ok := resolveAllowedOrigin("http://example.com")
	if ok {
		t.Error("expected false for empty origins")
	}
}

func TestResolveAllowedOrigin_Wildcard(t *testing.T) {
	config.SetConfigForTest(&config.Config{AllowedOrigins: []string{"*"}})
	defer config.SetConfigForTest(nil)

	origin, ok := resolveAllowedOrigin("http://example.com")
	if !ok {
		t.Error("expected true for wildcard")
	}
	if origin != "http://example.com" {
		t.Errorf("origin = %q, want http://example.com", origin)
	}
}

func TestResolveAllowedOrigin_WildcardNoRequestOrigin(t *testing.T) {
	config.SetConfigForTest(&config.Config{AllowedOrigins: []string{"*"}})
	defer config.SetConfigForTest(nil)

	origin, ok := resolveAllowedOrigin("")
	if !ok {
		t.Error("expected true for wildcard with no request origin")
	}
	if origin != "*" {
		t.Errorf("origin = %q, want *", origin)
	}
}

func TestResolveAllowedOrigin_SpecificMatch(t *testing.T) {
	config.SetConfigForTest(&config.Config{AllowedOrigins: []string{"http://example.com", "http://other.com"}})
	defer config.SetConfigForTest(nil)

	origin, ok := resolveAllowedOrigin("http://example.com")
	if !ok {
		t.Error("expected true for matching origin")
	}
	if origin != "http://example.com" {
		t.Errorf("origin = %q, want http://example.com", origin)
	}
}

func TestResolveAllowedOrigin_SpecificNoMatch(t *testing.T) {
	config.SetConfigForTest(&config.Config{AllowedOrigins: []string{"http://example.com"}})
	defer config.SetConfigForTest(nil)

	_, ok := resolveAllowedOrigin("http://evil.com")
	if ok {
		t.Error("expected false for non-matching origin")
	}
}

func TestResolveAllowedOrigin_SpecificNoRequestOrigin(t *testing.T) {
	config.SetConfigForTest(&config.Config{AllowedOrigins: []string{"http://example.com"}})
	defer config.SetConfigForTest(nil)

	origin, ok := resolveAllowedOrigin("")
	if !ok {
		t.Error("expected true for specific origins with no request origin")
	}
	if origin != "http://example.com" {
		t.Errorf("origin = %q, want http://example.com", origin)
	}
}

// ===== emitAnthropicTextDelta =====

func TestEmitAnthropicTextDelta_EmptyText(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w
	idx := 0
	inThinking := false
	inText := false
	inToolUse := false
	hasContent := false

	emitAnthropicTextDelta(w, flusher, &idx, &inThinking, &inText, &inToolUse, &hasContent, "")

	if hasContent {
		t.Error("expected no content for empty text")
	}
}

func TestEmitAnthropicTextDelta_FromThinking(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w
	idx := 0
	inThinking := true
	inText := false
	inToolUse := false
	hasContent := false

	emitAnthropicTextDelta(w, flusher, &idx, &inThinking, &inText, &inToolUse, &hasContent, "hello")

	result := w.Body.String()
	if !strings.Contains(result, "content_block_stop") {
		t.Error("expected thinking block to be closed")
	}
	if !strings.Contains(result, "content_block_start") {
		t.Error("expected text block to be started")
	}
	if !strings.Contains(result, "text_delta") {
		t.Error("expected text delta")
	}
}

func TestEmitAnthropicTextDelta_FromToolUse(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w
	idx := 0
	inThinking := false
	inText := false
	inToolUse := true
	hasContent := false

	emitAnthropicTextDelta(w, flusher, &idx, &inThinking, &inText, &inToolUse, &hasContent, "hello")

	result := w.Body.String()
	if !strings.Contains(result, "content_block_stop") {
		t.Error("expected tool_use block to be closed")
	}
}

// ===== emitAnthropicToolUse =====

func TestEmitAnthropicToolUse_WithID(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w
	idx := 0
	inToolUse := false

	tc := model.ToolCall{
		ID:   "toolu_test123",
		Type: "function",
		Function: model.FunctionCall{
			Name:      "get_weather",
			Arguments: `{"location":"Tokyo"}`,
		},
	}

	emitAnthropicToolUse(w, flusher, &idx, &inToolUse, tc)

	result := w.Body.String()
	if !strings.Contains(result, "tool_use") {
		t.Error("expected tool_use in output")
	}
	if !strings.Contains(result, "toolu_test123") {
		t.Error("expected tool ID in output")
	}
	if !strings.Contains(result, "get_weather") {
		t.Error("expected tool name in output")
	}
}

func TestEmitAnthropicToolUse_WithoutID(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w
	idx := 0
	inToolUse := false

	tc := model.ToolCall{
		ID:   "",
		Type: "function",
		Function: model.FunctionCall{
			Name:      "bash",
			Arguments: `{"command":"ls"}`,
		},
	}

	emitAnthropicToolUse(w, flusher, &idx, &inToolUse, tc)

	result := w.Body.String()
	if !strings.Contains(result, "toolu_") {
		t.Error("expected auto-generated toolu_ ID")
	}
}

func TestEmitAnthropicToolUse_ClosePreviousToolUse(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w
	idx := 0
	inToolUse := true // Already in a tool_use block

	tc := model.ToolCall{
		ID:   "toolu_new",
		Type: "function",
		Function: model.FunctionCall{
			Name:      "bash",
			Arguments: `{}`,
		},
	}

	emitAnthropicToolUse(w, flusher, &idx, &inToolUse, tc)

	result := w.Body.String()
	// Should have closed the previous block and started a new one
	if strings.Count(result, "content_block_stop") < 1 {
		t.Error("expected previous tool_use block to be closed")
	}
}

// ===== sendAnthropicSSE =====

func TestSendAnthropicSSE(t *testing.T) {
	w := httptest.NewRecorder()
	flusher := w

	sendAnthropicSSE(w, flusher, "message_start", map[string]string{"type": "message_start"})

	result := w.Body.String()
	if !strings.Contains(result, "event: message_start") {
		t.Error("expected event type in SSE")
	}
	if !strings.Contains(result, "data: ") {
		t.Error("expected data in SSE")
	}
}

// ===== writeAnthropicError =====

func TestWriteAnthropicErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()

	writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "Bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errResp["type"] != "error" {
		t.Errorf("type = %v, want error", errResp["type"])
	}
	errObj, _ := errResp["error"].(map[string]interface{})
	if errObj["type"] != "invalid_request_error" {
		t.Errorf("error type = %v, want invalid_request_error", errObj["type"])
	}
}