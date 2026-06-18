package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/config"
)

type attentionResponse struct {
	Active    bool   `json:"active"`
	Roadmap   string `json:"roadmap,omitempty"`
	ML        string `json:"ml,omitempty"`
	Message   string `json:"message,omitempty"`
	Level     string `json:"level,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

func attentionHandler(w http.ResponseWriter, _ *http.Request, cfg config.ProjectConfig) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")

	attentionFile := filepath.Join(cfg.RoadmapDir, ".trackfw-attention.json")
	data, err := os.ReadFile(attentionFile)
	if err != nil {
		_ = json.NewEncoder(w).Encode(attentionResponse{Active: false})
		return
	}

	var resp attentionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		_ = json.NewEncoder(w).Encode(attentionResponse{Active: false})
		return
	}
	resp.Active = true
	_ = json.NewEncoder(w).Encode(resp)
}
