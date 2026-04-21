package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRateLimiter_Defaults(t *testing.T) {
	rl := NewRateLimiter(0, 0)
	if rl.rate != 60 {
		t.Errorf("expected default rate 60, got %d", rl.rate)
	}
	if rl.burst != 60 {
		t.Errorf("expected default burst 60, got %d", rl.burst)
	}
}

func TestNewRateLimiter_CustomValues(t *testing.T) {
	rl := NewRateLimiter(100, 20)
	if rl.rate != 100 {
		t.Errorf("expected rate 100, got %d", rl.rate)
	}
	if rl.burst != 20 {
		t.Errorf("expected burst 20, got %d", rl.burst)
	}
}

func TestRateLimiter_AllowWithinBurst(t *testing.T) {
	rl := NewRateLimiter(60, 5)
	for i := 0; i < 5; i++ {
		if !rl.Allow("key1") {
			t.Errorf("request %d should be allowed within burst", i+1)
		}
	}
}

func TestRateLimiter_ExceedBurst(t *testing.T) {
	rl := NewRateLimiter(60, 3)
	rl.Allow("key1")
	rl.Allow("key1")
	rl.Allow("key1")
	if rl.Allow("key1") {
		t.Error("4th request should be rate limited")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(60, 2)
	rl.Allow("key1")
	rl.Allow("key1")
	// key2 should still have its own bucket
	if !rl.Allow("key2") {
		t.Error("different key should have separate bucket")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	rl.cleanup = 0 // force immediate cleanup on next check
	rl.Allow("key1")
	// After cleanup, key1 bucket should be removed if stale
	// This mainly tests no panic occurs
	rl.Allow("key2")
}

func TestRateLimitMiddleware_Allowed(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_RateLimited(t *testing.T) {
	rl := NewRateLimiter(60, 1)
	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"

	// First request allowed
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req)
	if w1.Code != http.StatusOK {
		t.Errorf("first request should be allowed, got %d", w1.Code)
	}

	// Second request rate limited
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request should be rate limited, got %d", w2.Code)
	}
}

func TestExtractIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	ip := extractIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ip)
	}
}

func TestExtractIP_XForwardedForSingle(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	ip := extractIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ip)
	}
}

func TestExtractIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "9.8.7.6")
	ip := extractIP(req)
	if ip != "9.8.7.6" {
		t.Errorf("expected 9.8.7.6, got %s", ip)
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:4321"
	ip := extractIP(req)
	if ip != "10.0.0.1:4321" {
		t.Errorf("expected 10.0.0.1:4321, got %s", ip)
	}
}

func TestExtractIP_XForwardedForPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "9.8.7.6")
	ip := extractIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("X-Forwarded-For should take priority, got %s", ip)
	}
}