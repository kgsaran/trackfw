package serve

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"

	"github.com/kgsaran/trackfw/internal/config"
)

//go:embed static
var staticFiles embed.FS

// ExposureWarningTemplate is the pinned, byte-identical warning printed by all
// three runtimes (Go, Node.js, Python) whenever --host resolves to a
// non-loopback interface. See docs/cli-parity.md "Aviso ao usuário — string
// pinada" for the parity convention this follows.
const ExposureWarningTemplate = "WARNING: trackfw serve is binding to %s:%d — the governance chain (ADRs, REQs, roadmaps) will be readable without authentication by any device that can reach it."

// ExposureWarning formats the pinned warning for the given host:port.
func ExposureWarning(host string, port int) string {
	return fmt.Sprintf(ExposureWarningTemplate, host, port)
}

// IsLoopbackHost reports whether host is a loopback address ("127.0.0.1",
// "::1") or the "localhost" name. Any other value is treated as a network
// exposure and triggers ExposureWarning.
func IsLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Start registers HTTP routes and starts the server on the given host:port.
// host defaults to "127.0.0.1" (loopback-only) at the caller (see
// internal/commands/serve.go); an explicit non-loopback host is an opt-in
// exposure and prints ExposureWarning to stderr before listening.
func Start(port int, host string) error {
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

	if !IsLoopbackHost(host) {
		fmt.Fprintln(os.Stderr, ExposureWarning(host, port))
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("trackfw serve — listening on http://localhost:%d\n", port)
	return http.ListenAndServe(addr, mux)
}
