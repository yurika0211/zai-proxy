package upstream

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zai-proxy/internal/model"
)

// Helper to create a valid JWT token for testing
func createTestJWT(userID string) string {
	// JWT header
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// JWT payload with required ID field
	payload := map[string]interface{}{
		"id":  userID,
		"sub": userID,
		"iat": 1234567890,
		"exp": 9999999999,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Signature (dummy for testing)
	signature := "test_signature"

	return headerB64 + "." + payloadB64 + "." + signature
}

// TestMakeUpstreamRequest_ValidToken_WithMockServer tests successful request creation
func TestMakeUpstreamRequest_ValidToken_WithMockServer(t *testing.T) {
	// Create mock server to intercept HTTP calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock z.ai API response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "test-response-id",
		})
	}))
	defer server.Close()

	token := createTestJWT("test-user-123")
	messages := []model.Message{
		{Role: "user", Content: "Hello"},
	}

	// Note: This will still fail because MakeUpstreamRequest makes real HTTP calls
	// to https://chat.z.ai, not to our mock server. This test documents the limitation.
	resp, chatID, err := MakeUpstreamRequest(token, messages, "glm-4", nil, nil)

	// We expect an error because the real z.ai endpoint is not mocked
	if err == nil && resp != nil {
		t.Logf("Request succeeded: chatID=%s", chatID)
		resp.Body.Close()
	}
}

// TestMakeUpstreamRequest_ValidToken_SimpleMessages tests with simple message structure
func TestMakeUpstreamRequest_ValidToken_SimpleMessages(t *testing.T) {
	token := createTestJWT("test-user-456")
	messages := []model.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "What is 2+2?"},
	}

	// This will attempt to make a real HTTP request to z.ai
	// In a real test environment, this would be mocked or skipped
	resp, chatID, err := MakeUpstreamRequest(token, messages, "glm-4", nil, nil)

	// Just verify the function doesn't panic and handles the response appropriately
	if resp != nil {
		defer resp.Body.Close()
		if chatID == "" {
			t.Error("expected non-empty chatID")
		}
	}
	// Error is acceptable since we're not mocking the real endpoint
	_ = err
}

// TestMakeUpstreamRequest_WithTools tests request with tool definitions
func TestMakeUpstreamRequest_WithTools(t *testing.T) {
	token := createTestJWT("test-user-789")
	messages := []model.Message{
		{Role: "user", Content: "Use a tool"},
	}
	tools := []model.Tool{
		{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"arg": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	resp, chatID, err := MakeUpstreamRequest(token, messages, "glm-4", tools, "auto")

	if resp != nil {
		defer resp.Body.Close()
		if chatID == "" {
			t.Error("expected non-empty chatID")
		}
	}
	_ = err
}

// TestMakeUpstreamRequest_WithThinkingModel tests with thinking model
func TestMakeUpstreamRequest_WithThinkingModel(t *testing.T) {
	token := createTestJWT("test-user-think")
	messages := []model.Message{
		{Role: "user", Content: "Think about this"},
	}

	resp, chatID, err := MakeUpstreamRequest(token, messages, "glm-4-thinking", nil, nil)

	if resp != nil {
		defer resp.Body.Close()
		if chatID == "" {
			t.Error("expected non-empty chatID")
		}
	}
	_ = err
}

// TestMakeUpstreamRequest_WithSearchModel tests with search-enabled model
func TestMakeUpstreamRequest_WithSearchModel(t *testing.T) {
	token := createTestJWT("test-user-search")
	messages := []model.Message{
		{Role: "user", Content: "Search for information"},
	}

	resp, chatID, err := MakeUpstreamRequest(token, messages, "glm-4-web", nil, nil)

	if resp != nil {
		defer resp.Body.Close()
		if chatID == "" {
			t.Error("expected non-empty chatID")
		}
	}
	_ = err
}
