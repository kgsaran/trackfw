package serve

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/kgsaran/trackfw/internal/config"
)

//go:embed static
var staticFiles embed.FS

// Start registers HTTP routes and starts the server on the given port.
func Start(port int) error {
	mux := http.NewServeMux()

	// Serve static assets from embed.FS
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("serve: sub FS: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Index — serve index.html for root path only
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	// API endpoints
	cfg := config.Load()
	mux.HandleFunc("/api/board", func(w http.ResponseWriter, r *http.Request) {
		boardHandler(w, r, cfg)
	})
	mux.HandleFunc("/api/chain", func(w http.ResponseWriter, r *http.Request) {
		chainHandler(w, r, cfg)
	})
	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, cfg)
	})
	mux.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		fileHandler(w, r, cfg)
	})
	mux.HandleFunc("/api/attention", func(w http.ResponseWriter, r *http.Request) {
		attentionHandler(w, r, cfg)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("trackfw serve — listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
