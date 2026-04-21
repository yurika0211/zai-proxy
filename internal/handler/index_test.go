package handler

import (
	"net/http/httptest"
	"testing"

	"zai-proxy/internal/config"
)

func TestHandleIndex_Disabled(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: false}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	HandleIndex(w, r)

	if w.Code != 404 {
		t.Errorf("expected 404 when status page disabled, got %d", w.Code)
	}
}

func TestHandleIndex_Enabled(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	HandleIndex(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleIndex_NonRootPath(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/other", nil)
	HandleIndex(w, r)

	if w.Code != 404 {
		t.Errorf("expected 404 for non-root path, got %d", w.Code)
	}
}

func TestHandleIndex_MethodNotAllowed(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	HandleIndex(w, r)

	if w.Code != 405 {
		t.Errorf("expected 405 for POST, got %d", w.Code)
	}
}