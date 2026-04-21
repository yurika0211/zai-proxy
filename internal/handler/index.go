package handler

import (
	"net/http"

	"zai-proxy/internal/config"
	"zai-proxy/internal/static"
)

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || !config.Cfg.EnableStatusPage {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(static.IndexHTML)
}
