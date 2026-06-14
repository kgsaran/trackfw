package serve

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
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

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("trackfw serve — listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
