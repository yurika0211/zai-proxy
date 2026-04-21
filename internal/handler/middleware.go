package handler

import (
	"net/http"
	"strings"

	"zai-proxy/internal/config"
)

func withOptionsBypass(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !config.Cfg.EnableCORS || r.Method != http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		origin, allowed := resolveAllowedOrigin(r.Header.Get("Origin"))
		if !allowed {
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}

		applyCORSHeaders(w, r, origin)
		w.WriteHeader(http.StatusNoContent)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.Cfg.EnableCORS {
			if origin, allowed := resolveAllowedOrigin(r.Header.Get("Origin")); allowed {
				applyCORSHeaders(w, r, origin)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func applyCORSHeaders(w http.ResponseWriter, r *http.Request, origin string) {
	if origin == "" {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	if origin != "*" {
		w.Header().Set("Vary", "Origin")
	}

	allowHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
	if allowHeaders == "" {
		allowHeaders = "Authorization, Content-Type, x-api-key, anthropic-version"
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
	w.Header().Set("Access-Control-Max-Age", "86400")
}

func resolveAllowedOrigin(requestOrigin string) (string, bool) {
	origins := config.Cfg.AllowedOrigins
	if len(origins) == 0 {
		return "", false
	}
	if len(origins) == 1 && origins[0] == "*" {
		if requestOrigin != "" {
			return requestOrigin, true
		}
		return "*", true
	}
	if requestOrigin == "" {
		return origins[0], true
	}
	for _, origin := range origins {
		if strings.EqualFold(origin, requestOrigin) {
			return requestOrigin, true
		}
	}
	return "", false
}
