package handler

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleIndex)
	mux.HandleFunc("/health", HandleHealth)
	mux.HandleFunc("/healthz", HandleHealthz)
	mux.HandleFunc("/stats", HandleStats)
	mux.HandleFunc("/v1/models", HandleModels)
	mux.HandleFunc("/v1/chat/completions", HandleChatCompletions)
	mux.HandleFunc("/v1/messages", HandleMessages)

	return withRequestStats(withCORS(withOptionsBypass(mux)))
}
