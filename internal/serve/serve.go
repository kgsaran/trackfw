package serve

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

//go:embed static
var staticFiles embed.FS

// ExposureWarningTemplate is the pinned, byte-identical warning printed by all
// three runtimes (Go, Node.js, Python) whenever --host resolves to a
// non-loopback interface. See docs/cli-parity.md "`trackfw serve` —
// endereço de escuta, `--host` e aviso de exposição" for the parity
// convention this follows.
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

// DisplayURL formats the URL to print and open in the browser for the given
// host:port, following the same convention across all three runtimes (Go,
// Node.js, Python): "localhost" is kept only when host is "localhost" or a
// loopback IPv4 address (127.0.0.0/8), so the common case's output does not
// change; IPv6 hosts get bracket notation (RFC 3986); anything else is
// printed as-is. See docs/cli-parity.md "`trackfw serve` — endereço de
// escuta, `--host` e aviso de exposição" (ML-1B).
//
// The IPv4-vs-IPv6 classification is by *literal syntax* (presence of ":"),
// not by decoded address family — this matches Node's net.isIPv6(host) and
// Python's isinstance(ipaddress.ip_address(host), IPv6Address), both of
// which classify "::ffff:127.0.0.1" as IPv6 despite it embedding an IPv4
// address. Classifying by decoded family (net.IP.To4() != nil, which is
// non-nil even for IPv4-mapped IPv6 literals) would make Go print
// "localhost" for that host while Node/Python bracket it — a 3-way
// divergence in shared, pinned-convention logic. That host is declared
// out of scope (see roadmap Notes: none of the 3 runtimes can bind to it),
// so the divergence was unobservable in practice, but the classification
// still needs to agree so ML-1C's parity gate doesn't trip on it.
func DisplayURL(host string, port int) string {
	if host == "localhost" {
		return fmt.Sprintf("http://localhost:%d", port)
	}
	if strings.Contains(host, ":") {
		// IPv6 literal — always bracket, even when loopback (::1) or an
		// IPv4-mapped form (::ffff:127.0.0.1), so the printed/opened URL
		// matches what actually needs brackets to resolve.
		return fmt.Sprintf("http://[%s]:%d", host, port)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return fmt.Sprintf("http://localhost:%d", port)
	}
	return fmt.Sprintf("http://%s:%d", host, port)
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

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Print only after the listener is actually up — never claim to be
	// listening while the bind is still pending or has failed.
	fmt.Printf("trackfw serve — listening on %s\n", DisplayURL(host, port))
	fmt.Println("Press Ctrl+C to stop.")
	return http.Serve(ln, mux)
}
