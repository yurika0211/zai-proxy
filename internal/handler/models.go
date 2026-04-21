package handler

import (
	"encoding/json"
	"net/http"

	"zai-proxy/internal/model"
)

func HandleModels(w http.ResponseWriter, r *http.Request) {
	var models []model.ModelInfo
	for _, id := range model.ModelList {
		models = append(models, model.ModelInfo{
			ID:      id,
			Object:  "model",
			OwnedBy: "z.ai",
		})
	}

	response := model.ModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
