package handler

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"zai-proxy/internal/config"
	"zai-proxy/internal/model"
	"zai-proxy/internal/version"
)

type requestStats struct {
	startedAt       time.Time
	totalRequests   atomic.Uint64
	inFlight        atomic.Int64
	lastMinuteStart atomic.Int64
	lastMinuteCount atomic.Int64
}

type statsSnapshot struct {
	Status            string    `json:"status"`
	Listen            string    `json:"listen"`
	LogLevel          string    `json:"log_level"`
	CORS              bool      `json:"cors"`
	AllowedOrigins    []string  `json:"allowed_origins"`
	StartedAt         time.Time `json:"started_at"`
	StartedAtUnix     int64     `json:"started_at_unix"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	TotalRequests     uint64    `json:"total_requests"`
	InFlightRequests  int64     `json:"in_flight_requests"`
	RequestsPerMinute int64     `json:"requests_per_minute"`
	SupportedModels   int       `json:"supported_models"`
	FrontendVersion   string    `json:"frontend_version"`
	Endpoints         []string  `json:"endpoints"`
}

var globalRequestStats = requestStats{
	startedAt: time.Now(),
}

func withRequestStats(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldTrackRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		globalRequestStats.beginRequest()
		defer globalRequestStats.endRequest()
		next.ServeHTTP(w, r)
	})
}

func shouldTrackRequest(r *http.Request) bool {
	if r.Method == http.MethodOptions {
		return false
	}

	switch r.URL.Path {
	case "/", "/healthz", "/stats":
		return false
	default:
		return true
	}
}

func (s *requestStats) beginRequest() {
	s.inFlight.Add(1)
}

func (s *requestStats) endRequest() {
	s.inFlight.Add(-1)
	s.totalRequests.Add(1)

	minute := time.Now().Truncate(time.Minute).Unix()
	prev := s.lastMinuteStart.Load()
	if prev != minute && s.lastMinuteStart.CompareAndSwap(prev, minute) {
		s.lastMinuteCount.Store(0)
	}
	s.lastMinuteCount.Add(1)
}

func (s *requestStats) snapshot() statsSnapshot {
	cfg := config.GetConfig()
	uptime := time.Since(s.startedAt).Round(time.Second)
	if uptime < 0 {
		uptime = 0
	}

	return statsSnapshot{
		Status:            "ok",
		Listen:            cfg.Listen,
		LogLevel:          cfg.LogLevel,
		CORS:              cfg.EnableCORS,
		AllowedOrigins:    append([]string(nil), cfg.AllowedOrigins...),
		StartedAt:         s.startedAt.UTC(),
		StartedAtUnix:     s.startedAt.Unix(),
		UptimeSeconds:     int64(uptime / time.Second),
		TotalRequests:     s.totalRequests.Load(),
		InFlightRequests:  s.inFlight.Load(),
		RequestsPerMinute: s.currentRPM(),
		SupportedModels:   len(model.ModelList),
		FrontendVersion:   version.GetFeVersion(),
		Endpoints: []string{
			"/v1/models",
			"/v1/chat/completions",
			"/v1/messages",
			"/healthz",
			"/stats",
		},
	}
}

func (s *requestStats) currentRPM() int64 {
	currentMinute := time.Now().Truncate(time.Minute).Unix()
	if s.lastMinuteStart.Load() != currentMinute {
		return 0
	}
	return s.lastMinuteCount.Load()
}

func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	if !cfg.EnableStatusPage {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := globalRequestStats.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"started_at":       snapshot.StartedAt.Format(time.RFC3339),
		"uptime_seconds":   snapshot.UptimeSeconds,
		"frontend_version": version.GetFeVersion(),
	})
}

// HandleHealth returns a simple health check response always available
// (does not require EnableStatusPage). Suitable for load balancer probes.
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func HandleStats(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	if !cfg.EnableStatusPage {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, globalRequestStats.snapshot())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
