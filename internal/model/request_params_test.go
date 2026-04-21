package model

import (
	"encoding/json"
	"testing"
)

func TestChatRequest_Temperature(t *testing.T) {
	temp := 0.7
	req := ChatRequest{
		Model:       "glm-4.7",
		Temperature: &temp,
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", req.Temperature)
	}
}

func TestChatRequest_MaxTokens(t *testing.T) {
	maxTokens := 4096
	req := ChatRequest{
		Model:     "glm-4.7",
		MaxTokens: &maxTokens,
	}
	if req.MaxTokens == nil || *req.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %v, want 4096", req.MaxTokens)
	}
}

func TestChatRequest_ToolStream(t *testing.T) {
	req := ChatRequest{
		Model:      "glm-4.7",
		ToolStream: true,
	}
	if !req.ToolStream {
		t.Error("ToolStream should be true")
	}
}

func TestChatRequest_ParallelToolCalls(t *testing.T) {
	parallel := false
	req := ChatRequest{
		Model:             "glm-4.7",
		ParallelToolCalls: &parallel,
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls != false {
		t.Error("ParallelToolCalls should be false")
	}
}

func TestChatRequest_ToRequestParams(t *testing.T) {
	temp := 0.5
	topP := 0.9
	maxTokens := 2048
	freqPen := 0.1
	presPen := 0.2
	seed := 42

	req := ChatRequest{
		Model:            "glm-4.7",
		Temperature:      &temp,
		TopP:             &topP,
		MaxTokens:        &maxTokens,
		FrequencyPenalty: &freqPen,
		PresencePenalty:  &presPen,
		Seed:             &seed,
		ToolStream:       true,
	}

	params := req.ToRequestParams()

	if params.Temperature == nil || *params.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", params.Temperature)
	}
	if params.TopP == nil || *params.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", params.TopP)
	}
	if params.MaxTokens == nil || *params.MaxTokens != 2048 {
		t.Errorf("MaxTokens = %v, want 2048", params.MaxTokens)
	}
	if params.FrequencyPenalty == nil || *params.FrequencyPenalty != 0.1 {
		t.Errorf("FrequencyPenalty = %v, want 0.1", params.FrequencyPenalty)
	}
	if params.PresencePenalty == nil || *params.PresencePenalty != 0.2 {
		t.Errorf("PresencePenalty = %v, want 0.2", params.PresencePenalty)
	}
	if params.Seed == nil || *params.Seed != 42 {
		t.Errorf("Seed = %v, want 42", params.Seed)
	}
	if !params.ToolStream {
		t.Error("ToolStream should be true")
	}
}

func TestChatRequest_ToRequestParams_NilFields(t *testing.T) {
	req := ChatRequest{Model: "glm-4.7"}
	params := req.ToRequestParams()

	if params.Temperature != nil {
		t.Errorf("Temperature should be nil, got %v", params.Temperature)
	}
	if params.TopP != nil {
		t.Errorf("TopP should be nil, got %v", params.TopP)
	}
	if params.MaxTokens != nil {
		t.Errorf("MaxTokens should be nil, got %v", params.MaxTokens)
	}
	if params.FrequencyPenalty != nil {
		t.Errorf("FrequencyPenalty should be nil, got %v", params.FrequencyPenalty)
	}
	if params.PresencePenalty != nil {
		t.Errorf("PresencePenalty should be nil, got %v", params.PresencePenalty)
	}
	if params.Seed != nil {
		t.Errorf("Seed should be nil, got %v", params.Seed)
	}
	if params.ToolStream {
		t.Error("ToolStream should be false by default")
	}
}

func TestChatRequest_JSONDeserialization(t *testing.T) {
	jsonStr := `{
		"model": "glm-4.7",
		"messages": [{"role": "user", "content": "hello"}],
		"temperature": 0.8,
		"top_p": 0.95,
		"max_tokens": 1024,
		"tool_stream": true,
		"parallel_tool_calls": false,
		"stream": true
	}`

	var req ChatRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.Temperature == nil || *req.Temperature != 0.8 {
		t.Errorf("Temperature = %v, want 0.8", req.Temperature)
	}
	if req.TopP == nil || *req.TopP != 0.95 {
		t.Errorf("TopP = %v, want 0.95", req.TopP)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %v, want 1024", req.MaxTokens)
	}
	if !req.ToolStream {
		t.Error("ToolStream should be true")
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls != false {
		t.Error("ParallelToolCalls should be false")
	}
	if !req.Stream {
		t.Error("Stream should be true")
	}
}

func TestChatRequest_JSONDeserialization_OptionalFields(t *testing.T) {
	jsonStr := `{
		"model": "glm-4.7",
		"messages": [{"role": "user", "content": "hello"}]
	}`

	var req ChatRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.Temperature != nil {
		t.Errorf("Temperature should be nil, got %v", req.Temperature)
	}
	if req.MaxTokens != nil {
		t.Errorf("MaxTokens should be nil, got %v", req.MaxTokens)
	}
	if req.ToolStream {
		t.Error("ToolStream should be false by default")
	}
}

func TestUsage_Fields(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &PromptTokensDetails{
			CachedTokens: 20,
		},
	}

	if usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", usage.PromptTokens)
	}
	if usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d, want 50", usage.CompletionTokens)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", usage.TotalTokens)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 20 {
		t.Errorf("PromptTokensDetails.CachedTokens = %v, want 20", usage.PromptTokensDetails)
	}
}

func TestUsage_JSONSerialization(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if int(parsed["prompt_tokens"].(float64)) != 100 {
		t.Errorf("prompt_tokens = %v, want 100", parsed["prompt_tokens"])
	}
	if int(parsed["completion_tokens"].(float64)) != 50 {
		t.Errorf("completion_tokens = %v, want 50", parsed["completion_tokens"])
	}
	if int(parsed["total_tokens"].(float64)) != 150 {
		t.Errorf("total_tokens = %v, want 150", parsed["total_tokens"])
	}
	// prompt_tokens_details should be omitted when nil
	if _, ok := parsed["prompt_tokens_details"]; ok {
		t.Error("prompt_tokens_details should be omitted when nil")
	}
}

func TestChatCompletionResponse_Usage(t *testing.T) {
	resp := ChatCompletionResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "glm-4.7",
		Choices: []Choice{},
		Usage:   Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	usage, ok := parsed["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("usage field missing or wrong type")
	}
	if int(usage["prompt_tokens"].(float64)) != 10 {
		t.Errorf("usage.prompt_tokens = %v, want 10", usage["prompt_tokens"])
	}
	if int(usage["total_tokens"].(float64)) != 30 {
		t.Errorf("usage.total_tokens = %v, want 30", usage["total_tokens"])
	}
}

func TestChatCompletionChunk_Usage(t *testing.T) {
	chunk := ChatCompletionChunk{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "glm-4.7",
		Choices: []Choice{},
		Usage:   &Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	usage, ok := parsed["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("usage field missing or wrong type in chunk")
	}
	if int(usage["total_tokens"].(float64)) != 30 {
		t.Errorf("usage.total_tokens = %v, want 30", usage["total_tokens"])
	}
}

func TestChatCompletionChunk_UsageNil(t *testing.T) {
	chunk := ChatCompletionChunk{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "glm-4.7",
		Choices: []Choice{},
		Usage:   nil,
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, ok := parsed["usage"]; ok {
		t.Error("usage should be omitted when nil in chunk")
	}
}

func TestRequestParams_ZeroValues(t *testing.T) {
	params := RequestParams{}
	if params.Temperature != nil {
		t.Error("Temperature should be nil")
	}
	if params.TopP != nil {
		t.Error("TopP should be nil")
	}
	if params.MaxTokens != nil {
		t.Error("MaxTokens should be nil")
	}
	if params.FrequencyPenalty != nil {
		t.Error("FrequencyPenalty should be nil")
	}
	if params.PresencePenalty != nil {
		t.Error("PresencePenalty should be nil")
	}
	if params.Seed != nil {
		t.Error("Seed should be nil")
	}
	if params.ToolStream {
		t.Error("ToolStream should be false")
	}
}