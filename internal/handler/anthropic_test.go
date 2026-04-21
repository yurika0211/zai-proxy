package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zai-proxy/internal/model"
)

func TestConvertAnthropicToInternal_ImageBlock(t *testing.T) {
	req := model.AnthropicRequest{
		Messages: []model.AnthropicMessage{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "看图"},
				map[string]interface{}{
					"type": "image",
					"source": map[string]interface{}{
						"type":       "base64",
						"media_type": "image/png",
						"data":       "QUJD",
					},
				},
			},
		}},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Role != "user" {
		t.Fatalf("role = %q, want user", messages[0].Role)
	}
	text, imageURLs := messages[0].ParseContent()
	if text != "看图" {
		t.Fatalf("text = %q, want 看图", text)
	}
	if len(imageURLs) != 1 {
		t.Fatalf("len(imageURLs) = %d, want 1", len(imageURLs))
	}
	if imageURLs[0] != "data:image/png;base64,QUJD" {
		t.Fatalf("imageURLs[0] = %q", imageURLs[0])
	}
}

func TestConvertAnthropicToInternal_ToolResultBlock(t *testing.T) {
	req := model.AnthropicRequest{
		Messages: []model.AnthropicMessage{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "toolu_test",
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": "npm install done"},
					},
				},
			},
		}},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Role != "tool" {
		t.Fatalf("role = %q, want tool", messages[0].Role)
	}
	if messages[0].ToolCallID != "toolu_test" {
		t.Fatalf("tool_call_id = %q, want toolu_test", messages[0].ToolCallID)
	}
	if messages[0].Content != "npm install done" {
		t.Fatalf("content = %#v, want npm install done", messages[0].Content)
	}
}

func TestConvertAnthropicToInternal_ToolResultObjectBlock(t *testing.T) {
	req := model.AnthropicRequest{
		Messages: []model.AnthropicMessage{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "toolu_test",
					"content": map[string]interface{}{
						"stdout": "ok",
						"stderr": "",
					},
				},
			},
		}},
	}

	messages, _, _ := convertAnthropicToInternal(req)
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Role != "tool" {
		t.Fatalf("role = %q, want tool", messages[0].Role)
	}
	if messages[0].Content != `{"stderr":"","stdout":"ok"}` {
		t.Fatalf("content = %#v, want JSON object string", messages[0].Content)
	}
}

func TestConvertAnthropicToInternal_SchemaLessAnthropicTools(t *testing.T) {
	req := model.AnthropicRequest{
		Tools: []model.AnthropicTool{
			{Type: "bash_20250124", Name: "bash"},
			{Type: "text_editor_20250728", Name: "str_replace_based_edit_tool"},
		},
	}

	_, tools, _ := convertAnthropicToInternal(req)
	if len(tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(tools))
	}
	if tools[0].Function.Name != "bash" {
		t.Fatalf("tools[0].Function.Name = %q, want bash", tools[0].Function.Name)
	}
	if tools[0].Function.Parameters == nil {
		t.Fatal("expected synthesized schema for bash tool")
	}
	if tools[1].Function.Name != "str_replace_based_edit_tool" {
		t.Fatalf("tools[1].Function.Name = %q, want str_replace_based_edit_tool", tools[1].Function.Name)
	}
	params, ok := tools[1].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("text editor parameters type = %T, want map[string]interface{}", tools[1].Function.Parameters)
	}
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties on synthesized text editor schema")
	}
	if _, ok := properties["old_str"]; !ok {
		t.Fatal("expected old_str in synthesized text editor schema")
	}
	if _, ok := properties["insert_text"]; !ok {
		t.Fatal("expected insert_text in synthesized text editor schema")
	}
}

func TestHandleMessages_DoesNotEnableBuiltinToolsWithoutExplicitTools(t *testing.T) {
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
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("x-api-key", "token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if capturedModel != "GLM-4.7" {
		t.Fatalf("capturedModel = %q, want GLM-4.7", capturedModel)
	}
}

func TestHandleMessages_EnablesToolsWhenExplicitToolsProvided(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	var capturedModel string
	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		capturedModel = modelName
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("tool_call", "", `{"type":"function","function":{"name":"get_weather","arguments":"{}"}}`),
				sseEventDone(),
			),
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"get_weather","input_schema":{"type":"object"}}],"stream":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("x-api-key", "token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if capturedModel != "GLM-4.7" {
		t.Fatalf("capturedModel = %q, want GLM-4.7", capturedModel)
	}
}

func TestHandleMessages_AnthropicStyleToolUseIsReturnedToClient(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("tool_call", "", `{"type":"tool_use","name":"Bash","input":{"command":"npm install","description":"Install React dependencies"}}`),
				sseEventDone(),
			),
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"安装依赖"}],"tools":[{"name":"Bash","input_schema":{"type":"object"}}],"stream":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("x-api-key", "token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Fatalf("stop_reason = %q, want tool_use", resp.StopReason)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(resp.Content))
	}
	if resp.Content[0].Type != "tool_use" {
		t.Fatalf("content[0].Type = %q, want tool_use", resp.Content[0].Type)
	}
	if resp.Content[0].Name != "Bash" {
		t.Fatalf("content[0].Name = %q, want Bash", resp.Content[0].Name)
	}
	if string(resp.Content[0].Input) != `{"command":"npm install","description":"Install React dependencies"}` {
		t.Fatalf("content[0].Input = %s", string(resp.Content[0].Input))
	}
}

func TestHandleMessages_StreamAnthropicStyleToolUseIsReturnedToClient(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("tool_call", "", `{"type":"tool_use","name":"Bash","input":{"command":"npm install","description":"Install React dependencies"}}`),
				sseEventDone(),
			),
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"claude-sonnet-4-6","max_tokens":128,"messages":[{"role":"user","content":"安装依赖"}],"tools":[{"name":"Bash","input_schema":{"type":"object"}}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("x-api-key", "token")
	w := httptest.NewRecorder()

	HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	result := w.Body.String()
	if !strings.Contains(result, "event: content_block_start") {
		t.Fatalf("missing content_block_start event: %s", result)
	}
	if !strings.Contains(result, `"type":"tool_use"`) {
		t.Fatalf("missing tool_use block: %s", result)
	}
	if !strings.Contains(result, `"name":"Bash"`) {
		t.Fatalf("missing tool name: %s", result)
	}
	if !strings.Contains(result, `"partial_json":"{\"command\":\"npm install\",\"description\":\"Install React dependencies\"}"`) {
		t.Fatalf("missing input_json_delta payload: %s", result)
	}
	if !strings.Contains(result, `"stop_reason":"tool_use"`) {
		t.Fatalf("missing tool_use stop reason: %s", result)
	}
}

func TestHandleChatCompletions_AcceptsXAPIKey(t *testing.T) {
	oldMakeUpstreamRequest := makeUpstreamRequest
	defer func() { makeUpstreamRequest = oldMakeUpstreamRequest }()

	makeUpstreamRequest = func(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
		if token != "token-from-header" {
			t.Fatalf("token = %q, want token-from-header", token)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: newFakeBody(
				sseEvent("answer", "hello", ""),
				sseEventDone(),
			),
		}, "glm-4.7", nil
	}

	body := strings.NewReader(`{"model":"GLM-4.7","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("x-api-key", "token-from-header")
	w := httptest.NewRecorder()

	HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestWriteAnthropicNonStreamTurn_UsageIsZeroWhenUnknown(t *testing.T) {
	w := httptest.NewRecorder()
	writeAnthropicNonStreamTurn(w, "msg_test", "claude-sonnet-4-6", assistantTurn{Content: "hello"})

	var resp model.AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Usage.InputTokens != 0 || resp.Usage.OutputTokens != 0 {
		t.Fatalf("usage = %+v, want zeros", resp.Usage)
	}
}

func TestHandleAnthropicStream_UsageIsZeroWhenUnknown(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "hello", ""),
		sseEventDone(),
	)
	w := httptest.NewRecorder()
	handleAnthropicStream(w, body, "msg_test", "glm-4.7", "claude-sonnet-4-6", nil)

	result := w.Body.String()
	if !strings.Contains(result, `"usage":{"input_tokens":0,"output_tokens":0}`) {
		t.Fatalf("stream result missing zero usage: %s", result)
	}
}
