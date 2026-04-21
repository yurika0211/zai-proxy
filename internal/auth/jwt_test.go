package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestDecodeJWTPayload_ValidToken(t *testing.T) {
	// Create a valid JWT payload
	payload := map[string]interface{}{"id": "user123"}
	payloadJSON, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadJSON)
	token := "header." + encoded + ".signature"

	result, err := DecodeJWTPayload(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != "user123" {
		t.Errorf("expected ID=user123, got %s", result.ID)
	}
}

func TestDecodeJWTPayload_WithPadding(t *testing.T) {
	payload := map[string]interface{}{"id": "testuser"}
	payloadJSON, _ := json.Marshal(payload)
	encoded := base64.URLEncoding.EncodeToString(payloadJSON)
	token := "header." + encoded + ".signature"

	result, err := DecodeJWTPayload(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "testuser" {
		t.Errorf("expected ID=testuser, got %s", result.ID)
	}
}

func TestDecodeJWTPayload_EmptyToken(t *testing.T) {
	result, err := DecodeJWTPayload("")
	if err != nil {
		t.Fatalf("expected nil result for empty token, got error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestDecodeJWTPayload_SinglePart(t *testing.T) {
	result, err := DecodeJWTPayload("justonepart")
	if err != nil {
		t.Fatalf("expected nil result, got error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestDecodeJWTPayload_InvalidBase64(t *testing.T) {
	token := "header.!!!invalid!!!.signature"
	_, err := DecodeJWTPayload(token)
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestDecodeJWTPayload_InvalidJSON(t *testing.T) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	token := "header." + encoded + ".signature"
	_, err := DecodeJWTPayload(token)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestDecodeJWTPayload_MissingID(t *testing.T) {
	payload := map[string]interface{}{"name": "test"}
	payloadJSON, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadJSON)
	token := "header." + encoded + ".signature"

	result, err := DecodeJWTPayload(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "" {
		t.Errorf("expected empty ID, got %s", result.ID)
	}
}