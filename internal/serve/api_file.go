package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// fileHandler handles GET /api/file?path=<relative-path>.
// Returns the raw file content as text/plain.
// Enforces that the resolved path is within one of the allowed directories
// (ADRDirs, REQDir, RoadmapDir) to prevent path traversal attacks.
func fileHandler(w http.ResponseWriter, r *http.Request, cfg config.ProjectConfig) {
	setCORSHeaders(w)

	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}

	// Resolve relative to the current working directory
	workDir, err := os.Getwd()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Clean the path to eliminate any .. sequences
	cleanedPath := filepath.Clean(rawPath)
	absPath := filepath.Join(workDir, cleanedPath)

	// Build the list of allowed root directories (absolute)
	var allowedDirs []string
	for _, d := range cfg.ADRDirs {
		allowedDirs = append(allowedDirs, filepath.Join(workDir, filepath.Clean(d)))
	}
	allowedDirs = append(allowedDirs, filepath.Join(workDir, filepath.Clean(cfg.REQDir)))
	allowedDirs = append(allowedDirs, filepath.Join(workDir, filepath.Clean(cfg.RoadmapDir)))

	// Security check: resolved path must start with an allowed directory
	allowed := false
	for _, dir := range allowedDirs {
		// Ensure dir has trailing separator so "docs/adr2" doesn't match "docs/adr"
		prefix := dir
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if strings.HasPrefix(absPath, prefix) || absPath == dir {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}
