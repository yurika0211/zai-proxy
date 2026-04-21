package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zai-proxy/internal/config"
)

func TestNewRouter_OptionsBypassAddsCORSHeaders(t *testing.T) {
	config.Cfg = &config.Config{
		Listen:           ":8000",
		LogLevel:         "info",
		EnableCORS:       true,
		AllowedOrigins:   []string{"*"},
		EnableStatusPage: true,
	}

	req := httptest.NewRequest(http.MethodOptions, "/v1/messages", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")

	rr := httptest.NewRecorder()
	NewRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("unexpected Access-Control-Allow-Origin: %q", got)
	}
}

func TestNewRouter_HealthzAvailable(t *testing.T) {
	config.Cfg = &config.Config{
		Listen:           ":8000",
		LogLevel:         "info",
		EnableCORS:       true,
		AllowedOrigins:   []string{"*"},
		EnableStatusPage: true,
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	NewRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type: %q", got)
	}
}
