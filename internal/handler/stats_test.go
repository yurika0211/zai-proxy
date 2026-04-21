package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"zai-proxy/internal/config"
)

func TestBeginRequest_IncrementsInFlight(t *testing.T) {
	// Reset stats for testing
	globalRequestStats.totalRequests.Store(0)
	globalRequestStats.inFlight.Store(0)

	globalRequestStats.beginRequest()

	if globalRequestStats.inFlight.Load() != 1 {
		t.Errorf("expected in-flight to be 1, got %d", globalRequestStats.inFlight.Load())
	}
}

func TestEndRequest_DecrementsInFlightAndIncrementsTotal(t *testing.T) {
	// Setup
	globalRequestStats.inFlight.Store(1)
	globalRequestStats.totalRequests.Store(0)

	globalRequestStats.endRequest()

	if globalRequestStats.inFlight.Load() != 0 {
		t.Errorf("expected in-flight to be 0, got %d", globalRequestStats.inFlight.Load())
	}

	if globalRequestStats.totalRequests.Load() != 1 {
		t.Errorf("expected total requests to be 1, got %d", globalRequestStats.totalRequests.Load())
	}
}

func TestSnapshot_HasValidData(t *testing.T) {
	// Reset stats
	globalRequestStats.totalRequests.Store(5)
	globalRequestStats.inFlight.Store(2)
	globalRequestStats.startedAt = time.Now().Add(-time.Hour)

	snapshot := globalRequestStats.snapshot()

	if snapshot.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", snapshot.Status)
	}

	if snapshot.TotalRequests != 5 {
		t.Errorf("expected 5 total requests, got %d", snapshot.TotalRequests)
	}

	if snapshot.InFlightRequests != 2 {
		t.Errorf("expected 2 in-flight requests, got %d", snapshot.InFlightRequests)
	}

	if snapshot.UptimeSeconds < 3600 {
		t.Errorf("expected uptime >= 3600s, got %d", snapshot.UptimeSeconds)
	}
}

func TestShouldTrackRequest_SkipsStatusPages(t *testing.T) {
	testCases := []struct {
		path     string
		method   string
		expected bool
	}{
		{"/", "GET", false},
		{"/healthz", "GET", false},
		{"/stats", "GET", false},
		{"/v1/chat/completions", "POST", true},
		{"/v1/messages", "POST", true},
		{"/", "OPTIONS", false},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		result := shouldTrackRequest(req)
		if result != tc.expected {
			t.Errorf("shouldTrackRequest(%s %s): expected %v, got %v", tc.method, tc.path, tc.expected, result)
		}
	}
}

func TestCurrentRPM_ReturnsZeroWhenNoMinuteChange(t *testing.T) {
	globalRequestStats.lastMinuteStart.Store(time.Now().Truncate(time.Minute).Unix())
	globalRequestStats.lastMinuteCount.Store(10)

	rpm := globalRequestStats.currentRPM()
	if rpm != 10 {
		t.Errorf("expected 10 RPM, got %d", rpm)
	}
}

// ===== HandleStats =====

func TestHandleStats_Disabled(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: false}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	HandleStats(w, r)

	if w.Code != 404 {
		t.Errorf("expected 404 when status page disabled, got %d", w.Code)
	}
}

func TestHandleStats_Enabled(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: true}
	globalRequestStats.totalRequests.Store(42)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	HandleStats(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %v", result["status"])
	}
}

func TestHandleStats_MethodNotAllowed(t *testing.T) {
	config.Cfg = &config.Config{EnableStatusPage: true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/stats", nil)
	HandleStats(w, r)

	if w.Code != 405 {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// ===== HandleHealth =====

func TestHandleHealth_OK(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health", nil)
	HandleHealth(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %v", result["status"])
	}
}

func TestHandleHealth_MethodNotAllowed(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/health", nil)
	HandleHealth(w, r)

	if w.Code != 405 {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleHealth_HeadAllowed(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("HEAD", "/health", nil)
	HandleHealth(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200 for HEAD, got %d", w.Code)
	}
}
