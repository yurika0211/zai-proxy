package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"zai-proxy/internal/model"
)

// fakeReadCloser 将 string 包装为 io.ReadCloser
type fakeReadCloser struct {
	io.Reader
}

func (f *fakeReadCloser) Close() error { return nil }

func newFakeBody(lines ...string) io.ReadCloser {
	return &fakeReadCloser{Reader: strings.NewReader(strings.Join(lines, "\n"))}
}

// 构造上游 SSE 数据行
func sseEvent(phase, deltaContent, editContent string) string {
	data := model.UpstreamData{}
	data.Data.Phase = phase
	data.Data.DeltaContent = deltaContent
	data.Data.EditContent = editContent
	b, _ := json.Marshal(data)
	return fmt.Sprintf("data: %s", string(b))
}

func sseEventDone() string {
	return sseEvent("done", "", "")
}

func dummyTools() []model.Tool {
	return []model.Tool{{
		Type: "function",
		Function: model.ToolFunction{
			Name:        "get_weather",
			Description: "获取天气",
		},
	}}
}

// ===== 流式：普通文本回复 =====

func TestStreamResponse_NormalContent(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "Hello", ""),
		sseEvent("answer", " World", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	result := w.Body.String()

	// 应包含内容 chunk
	if !strings.Contains(result, "Hello") {
		t.Error("missing 'Hello' in stream output")
	}
	if !strings.Contains(result, "World") {
		t.Error("missing 'World' in stream output")
	}

	// finish_reason 应该是 "stop"
	if !strings.Contains(result, `"finish_reason":"stop"`) {
		t.Error("finish_reason should be 'stop'")
	}

	// 应以 [DONE] 结尾
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("missing [DONE]")
	}
}

// ===== 流式：tool_call 回复 =====

func TestStreamResponse_ToolCall(t *testing.T) {
	toolCallJSON := `{"id":"call_test123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"北京\"}"}}`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	result := w.Body.String()

	// 应包含 tool_calls
	if !strings.Contains(result, `"tool_calls"`) {
		t.Error("missing tool_calls in stream output")
	}
	if !strings.Contains(result, `"get_weather"`) {
		t.Error("missing function name in stream output")
	}
	if !strings.Contains(result, `call_test123`) {
		t.Error("missing tool call ID in stream output")
	}

	// finish_reason 应该是 "tool_calls"
	if !strings.Contains(result, `"finish_reason":"tool_calls"`) {
		t.Error("finish_reason should be 'tool_calls'")
	}
}

// ===== 流式：tool_call 无 ID（自动分配）=====

func TestStreamResponse_ToolCallAutoID(t *testing.T) {
	toolCallJSON := `{"type":"function","function":{"name":"get_weather","arguments":"{}"}}`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	result := w.Body.String()

	// 应自动分配 call_ 前缀的 ID
	if !strings.Contains(result, `"id":"call_`) {
		t.Error("missing auto-generated tool call ID")
	}
	if !strings.Contains(result, `"finish_reason":"tool_calls"`) {
		t.Error("finish_reason should be 'tool_calls'")
	}
}

// ===== 流式：无 tools 时 tool_call 阶段被忽略 =====

func TestStreamResponse_ToolCallWithoutToolsDef(t *testing.T) {
	toolCallJSON := `{"type":"function","function":{"name":"get_weather","arguments":"{}"}}`

	body := newFakeBody(
		sseEvent("answer", "text before", ""),
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	// 不传 tools，tool_call 不应被解析为函数调用
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	result := w.Body.String()

	// finish_reason 应为 "stop"（没有检测到函数调用）
	if !strings.Contains(result, `"finish_reason":"stop"`) {
		t.Error("finish_reason should be 'stop' when no tools defined")
	}
}

// ===== 流式：mcp tool_call 被跳过 =====

func TestStreamResponse_McpToolCallSkipped(t *testing.T) {
	mcpContent := `{"type":"mcp","name":"mcp-server-xxx","arguments":"{}"}`

	body := newFakeBody(
		sseEvent("answer", "response text", ""),
		sseEvent("tool_call", "", mcpContent),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	result := w.Body.String()

	// mcp 类型的 tool_call 不应出现在输出中
	if strings.Contains(result, `mcp-server`) {
		t.Error("mcp tool call should be filtered out")
	}
	// 应为 "stop"（mcp 不算用户函数调用）
	if !strings.Contains(result, `"finish_reason":"stop"`) {
		t.Error("finish_reason should be 'stop'")
	}
}

// ===== 流式：混合内容 + tool_call =====

func TestStreamResponse_ContentThenToolCall(t *testing.T) {
	toolCallJSON := `{"function":{"name":"get_weather","arguments":"{}"}}`

	body := newFakeBody(
		sseEvent("answer", "Let me check ", ""),
		sseEvent("answer", "the weather.", ""),
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	result := w.Body.String()

	if !strings.Contains(result, "Let me check") {
		t.Error("missing content text")
	}
	if !strings.Contains(result, `"get_weather"`) {
		t.Error("missing tool call")
	}
	if !strings.Contains(result, `"finish_reason":"tool_calls"`) {
		t.Error("finish_reason should be 'tool_calls'")
	}
}

// ===== 流式：多个 tool_call =====

func TestStreamResponse_MultipleToolCalls(t *testing.T) {
	toolCallJSON := `[{"id":"c1","type":"function","function":{"name":"fn1","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"fn2","arguments":"{}"}}]`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	result := w.Body.String()

	if !strings.Contains(result, `"fn1"`) {
		t.Error("missing fn1")
	}
	if !strings.Contains(result, `"fn2"`) {
		t.Error("missing fn2")
	}

	// 验证 chunk 数量：每个 tool_call 一个 delta chunk（包含 "tool_calls" 在 delta 中）
	chunks := strings.Split(result, "data: ")
	toolCallDeltaChunks := 0
	for _, chunk := range chunks {
		// 只计算 delta 中包含 tool_calls 的 chunk，排除 finish_reason 中的
		if strings.Contains(chunk, `"tool_calls":[{`) {
			toolCallDeltaChunks++
		}
	}
	if toolCallDeltaChunks != 2 {
		t.Errorf("tool_call delta chunks = %d, want 2", toolCallDeltaChunks)
	}
}

// ===== 非流式：普通文本回复 =====

func TestNonStreamResponse_NormalContent(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "Hello World", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	var resp model.ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("len(Choices) = %d", len(resp.Choices))
	}
	if resp.Choices[0].Message == nil {
		t.Fatal("Message is nil")
	}
	if resp.Choices[0].Message.Content != "Hello World" {
		t.Errorf("Content = %q, want %q", resp.Choices[0].Message.Content, "Hello World")
	}
	if *resp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", *resp.Choices[0].FinishReason, "stop")
	}
	if len(resp.Choices[0].Message.ToolCalls) != 0 {
		t.Errorf("len(ToolCalls) = %d, want 0", len(resp.Choices[0].Message.ToolCalls))
	}
}

// ===== 非流式：tool_call 回复 =====

func TestNonStreamResponse_ToolCall(t *testing.T) {
	toolCallJSON := `{"id":"call_ns","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"上海\"}"}}`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	msg := resp.Choices[0].Message
	if msg == nil {
		t.Fatal("Message is nil")
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", msg.ToolCalls[0].Function.Name, "get_weather")
	}
	if msg.ToolCalls[0].Function.Arguments != `{"location":"上海"}` {
		t.Errorf("Function.Arguments = %q", msg.ToolCalls[0].Function.Arguments)
	}
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", *resp.Choices[0].FinishReason, "tool_calls")
	}
}

// ===== 非流式：tool_call 无 ID =====

func TestNonStreamResponse_ToolCallAutoID(t *testing.T) {
	toolCallJSON := `{"function":{"name":"fn1","arguments":"{}"}}`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if !strings.HasPrefix(msg.ToolCalls[0].ID, "call_") {
		t.Errorf("ID = %q, should have 'call_' prefix", msg.ToolCalls[0].ID)
	}
}

// ===== 非流式：无 tools 定义时不解析 tool_call =====

func TestNonStreamResponse_ToolCallWithoutToolsDef(t *testing.T) {
	toolCallJSON := `{"function":{"name":"get_weather","arguments":"{}"}}`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	var resp model.ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if *resp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", *resp.Choices[0].FinishReason, "stop")
	}
	if len(resp.Choices[0].Message.ToolCalls) != 0 {
		t.Errorf("len(ToolCalls) = %d, want 0", len(resp.Choices[0].Message.ToolCalls))
	}
}

// ===== 非流式：mcp tool_call 被跳过 =====

func TestNonStreamResponse_McpToolCallSkipped(t *testing.T) {
	mcpContent := `{"type":"mcp","name":"mcp-server-xxx","arguments":"{}"}`

	body := newFakeBody(
		sseEvent("answer", "response", ""),
		sseEvent("tool_call", "", mcpContent),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if *resp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", *resp.Choices[0].FinishReason, "stop")
	}
	if len(resp.Choices[0].Message.ToolCalls) != 0 {
		t.Errorf("should not have tool_calls for mcp")
	}
}

// ===== 非流式：内容 + tool_call =====

func TestNonStreamResponse_ContentAndToolCall(t *testing.T) {
	toolCallJSON := `{"function":{"name":"get_weather","arguments":"{}"}}`

	body := newFakeBody(
		sseEvent("answer", "checking weather...", ""),
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	msg := resp.Choices[0].Message
	if msg.Content != "checking weather..." {
		t.Errorf("Content = %q, want %q", msg.Content, "checking weather...")
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", *resp.Choices[0].FinishReason, "tool_calls")
	}
}

// ===== 非流式：多个 tool_call =====

func TestNonStreamResponse_MultipleToolCalls(t *testing.T) {
	toolCallJSON := `[{"id":"c1","type":"function","function":{"name":"fn1","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"fn2","arguments":"{\"x\":1}"}}]`

	body := newFakeBody(
		sseEvent("tool_call", "", toolCallJSON),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 2 {
		t.Fatalf("len(ToolCalls) = %d, want 2", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "fn1" {
		t.Errorf("ToolCalls[0].Function.Name = %q", msg.ToolCalls[0].Function.Name)
	}
	if msg.ToolCalls[1].Function.Name != "fn2" {
		t.Errorf("ToolCalls[1].Function.Name = %q", msg.ToolCalls[1].Function.Name)
	}
	if msg.ToolCalls[0].Index != 0 || msg.ToolCalls[1].Index != 1 {
		t.Errorf("Indices = [%d, %d], want [0, 1]", msg.ToolCalls[0].Index, msg.ToolCalls[1].Index)
	}
}

// ===== 非流式：glm_block 包裹的 tool_call =====

func TestNonStreamResponse_GlmBlockToolCall(t *testing.T) {
	editContent := `<glm_block type="tool_call">{"id":"call_glm","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"深圳\"}"}}</glm_block>`

	body := newFakeBody(
		sseEvent("tool_call", "", editContent),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call_glm" {
		t.Errorf("ID = %q, want %q", msg.ToolCalls[0].ID, "call_glm")
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q", msg.ToolCalls[0].Function.Name)
	}
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q", *resp.Choices[0].FinishReason)
	}
}

// ===== 流式：SSE headers 验证 =====

func TestStreamResponse_Headers(t *testing.T) {
	body := newFakeBody(sseEventDone())

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}
}

// ===== 非流式：response headers 验证 =====

func TestNonStreamResponse_Headers(t *testing.T) {
	body := newFakeBody(sseEventDone())

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// ===== 流式：空数据 =====

func TestStreamResponse_EmptyBody(t *testing.T) {
	body := newFakeBody(sseEventDone())

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	result := w.Body.String()
	if !strings.Contains(result, `"finish_reason":"stop"`) {
		t.Error("should have stop finish_reason")
	}
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("missing [DONE]")
	}
}

// ===== 流式：[DONE] 信号 =====

func TestStreamResponse_DoneSignal(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "hello", ""),
		"data: [DONE]",
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	result := w.Body.String()
	if !strings.Contains(result, "hello") {
		t.Error("missing content")
	}
}

// ===== 非流式：response 格式完整性 =====

func TestNonStreamResponse_FullFormat(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "test response", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	var resp model.ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.ID != "chatcmpl-test" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("Object = %q", resp.Object)
	}
	if resp.Model != "glm-4.7" {
		t.Errorf("Model = %q", resp.Model)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("Role = %q", resp.Choices[0].Message.Role)
	}
}

// ===== 流式：prompt 注入模式 <tool_call> 在 answer 文本中 =====

func TestStreamResponse_PromptInjectionToolCall(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "好的，我来查询。\n", ""),
		sseEvent("answer", `<tool_call>{"name":"get_weather","arguments":{"city":"北京"}}</tool_call>`, ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	result := w.Body.String()

	if !strings.Contains(result, `"tool_calls"`) {
		t.Error("missing tool_calls in prompt injection stream")
	}
	if !strings.Contains(result, `"get_weather"`) {
		t.Error("missing function name")
	}
	if !strings.Contains(result, `"finish_reason":"tool_calls"`) {
		t.Error("finish_reason should be tool_calls")
	}
}

// ===== 非流式：prompt 注入模式 =====

func TestNonStreamResponse_PromptInjectionToolCall(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "我来查询天气。\n<tool_call>{\"name\":\"get_weather\",\"arguments\":{\"city\":\"上海\"}}</tool_call>", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", dummyTools())

	var resp model.ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q", msg.ToolCalls[0].Function.Name)
	}
	if strings.Contains(msg.Content, "<tool_call>") {
		t.Error("content should not contain <tool_call> tags")
	}
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want tool_calls", *resp.Choices[0].FinishReason)
	}
}

// ===== 非流式：response 中不应有 delta 字段 =====

func TestNonStreamResponse_NoDeltaField(t *testing.T) {
	body := newFakeBody(
		sseEvent("answer", "hello", ""),
		sseEventDone(),
	)

	w := httptest.NewRecorder()
	handleNonStreamResponse(w, body, "chatcmpl-test", "glm-4.7", nil)

	result := w.Body.String()
	if strings.Contains(result, `"delta"`) {
		t.Error("non-streaming response should not contain delta field")
	}
}

// TestMarshalChunk_Success tests successful marshaling
func TestMarshalChunk_Success(t *testing.T) {
	chunk := model.ChatCompletionChunk{
		ID:      "test-id",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "glm-4",
	}

	data := marshalChunk(chunk)
	if data == nil {
		t.Error("expected non-nil data for valid chunk")
	}

	var result model.ChatCompletionChunk
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("failed to unmarshal result: %v", err)
	}
	if result.ID != "test-id" {
		t.Errorf("expected test-id, got %s", result.ID)
	}
}

// TestMarshalChunk_NilInput tests with nil input
func TestMarshalChunk_NilInput(t *testing.T) {
	data := marshalChunk(nil)
	if data == nil {
		t.Error("expected non-nil data for nil input (marshals to 'null')")
	}
}

// TestTruncate_ShortString tests truncate with short string
func TestTruncate_ShortString(t *testing.T) {
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

// TestTruncate_ExactLength tests truncate with exact length
func TestTruncate_ExactLength(t *testing.T) {
	result := truncate("hello", 5)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

// TestTruncate_LongString tests truncate with long string
func TestTruncate_LongString(t *testing.T) {
	result := truncate("hello world", 5)
	if result != "hello..." {
		t.Errorf("expected 'hello...', got %q", result)
	}
}

// TestTruncate_EmptyString tests truncate with empty string
func TestTruncate_EmptyString(t *testing.T) {
	result := truncate("", 10)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
