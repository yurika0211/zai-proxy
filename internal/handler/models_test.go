package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zai-proxy/internal/model"
)

// TestHandleModels tests the models endpoint
func TestHandleModels(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var response model.ModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("expected object 'list', got %q", response.Object)
	}

	if len(response.Data) == 0 {
		t.Error("expected non-empty models list")
	}

	for _, m := range response.Data {
		if m.Object != "model" {
			t.Errorf("expected model object type, got %q", m.Object)
		}
		if m.OwnedBy != "z.ai" {
			t.Errorf("expected OwnedBy 'z.ai', got %q", m.OwnedBy)
		}
	}
}

// TestHandleModels_ContentType verifies correct content type
func TestHandleModels_ContentType(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

// TestHandleModels_ValidJSON verifies response is valid JSON
func TestHandleModels_ValidJSON(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req)

	var response model.ModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}

// TestHandleModels_AllModelsPresent verifies all models are returned
func TestHandleModels_AllModelsPresent(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req)

	var response model.ModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify we have at least some models
	if len(response.Data) < 1 {
		t.Error("expected at least 1 model")
	}

	// Verify each model has required fields
	for i, m := range response.Data {
		if m.ID == "" {
			t.Errorf("model %d has empty ID", i)
		}
		if m.Object != "model" {
			t.Errorf("model %d has wrong object type: %s", i, m.Object)
		}
	}
}
