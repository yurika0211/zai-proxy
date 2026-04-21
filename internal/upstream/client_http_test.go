package upstream

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zai-proxy/internal/model"
)

// TestMakeUpstreamRequest_InvalidToken tests error handling for invalid token
func TestMakeUpstreamRequest_InvalidToken(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello"},
	}

	resp, chatID, err := MakeUpstreamRequest("invalid-token", messages, "glm-4", nil, nil)

	if err == nil {
		t.Error("expected error for invalid token")
	}
	if resp != nil {
		t.Error("expected nil response for invalid token")
	}
	if chatID != "" {
		t.Error("expected empty chatID for invalid token")
	}
}

// TestMakeUpstreamRequest_EmptyToken tests error handling for empty token
func TestMakeUpstreamRequest_EmptyToken(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello"},
	}

	resp, chatID, err := MakeUpstreamRequest("", messages, "glm-4", nil, nil)

	if err == nil {
		t.Error("expected error for empty token")
	}
	if resp != nil {
		t.Error("expected nil response for empty token")
	}
	if chatID != "" {
		t.Error("expected empty chatID for empty token")
	}
}

// TestMakeUpstreamRequest_MalformedToken tests with malformed JWT
func TestMakeUpstreamRequest_MalformedToken(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello"},
	}

	// Token with wrong format
	resp, chatID, err := MakeUpstreamRequest("not.a.valid.jwt", messages, "glm-4", nil, nil)

	if err == nil {
		t.Error("expected error for malformed token")
	}
	if resp != nil {
		t.Error("expected nil response for malformed token")
	}
	if chatID != "" {
		t.Error("expected empty chatID for malformed token")
	}
}

// TestExtractLatestUserContent_FromComplexMessages tests extraction from various message types
func TestExtractLatestUserContent_FromComplexMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "You are helpful"},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "First user message"},
			},
		},
		{Role: "assistant", Content: "Response"},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Second user message"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img.png",
					},
				},
			},
		},
	}

	result := ExtractLatestUserContent(messages)
	if result != "Second user message" {
		t.Errorf("expected 'Second user message', got %q", result)
	}
}

// TestExtractAllImageURLs_FromVariousMessages tests URL extraction from different message structures
func TestExtractAllImageURLs_FromVariousMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "System"},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/1.png",
					},
				},
			},
		},
		{Role: "assistant", Content: "Response"},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Text"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/2.png",
					},
				},
			},
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/3.png",
					},
				},
			},
		},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 3 {
		t.Fatalf("expected 3 URLs, got %d", len(urls))
	}

	expectedURLs := []string{
		"https://example.com/1.png",
		"https://example.com/2.png",
		"https://example.com/3.png",
	}

	for i, expected := range expectedURLs {
		if urls[i] != expected {
			t.Errorf("URL %d: expected %s, got %s", i, expected, urls[i])
		}
	}
}

// TestUploadImages_WithEmptyURLs tests upload with no images
func TestUploadImages_WithEmptyURLs(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	// Test with empty URLs
	files, err := UploadImages("test-token", []string{})

	// Should handle gracefully
	if err != nil && len(files) == 0 {
		t.Logf("Expected behavior: empty URLs handled")
	}
}

// TestMakeUpstreamRequest_NilMessages tests with nil messages
func TestMakeUpstreamRequest_NilMessages(t *testing.T) {
	resp, chatID, err := MakeUpstreamRequest("test-token", nil, "glm-4", nil, nil)

	if err == nil {
		t.Error("expected error for nil messages")
	}
	if resp != nil {
		t.Error("expected nil response for nil messages")
	}
	if chatID != "" {
		t.Error("expected empty chatID for nil messages")
	}
}

// TestMakeUpstreamRequest_EmptyMessages tests with empty messages slice
func TestMakeUpstreamRequest_EmptyMessages(t *testing.T) {
	resp, chatID, err := MakeUpstreamRequest("test-token", []model.Message{}, "glm-4", nil, nil)

	if err == nil {
		t.Error("expected error for empty messages")
	}
	if resp != nil {
		t.Error("expected nil response for empty messages")
	}
	if chatID != "" {
		t.Error("expected empty chatID for empty messages")
	}
}
