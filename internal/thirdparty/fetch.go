// Package thirdparty implements the network fetch, marker-based rejection
// and checksum primitives used by the two-phase third-party artifact gate
// (skills/agents installed from a URL). See:
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md
// (D3, D6, D7).
package thirdparty

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxContentSize is the network fetch cap for third-party text artifacts
// (D7): 2 MiB — deliberately small, since this is text, not a binary
// release asset (the plugin subsystem that downloaded binary assets was
// removed; see docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md).
const maxContentSize = 2 << 20

// maxRedirects is the maximum number of redirect hops followed before
// fetching a third-party artifact (D7). The net/http default is 10 hops
// without re-validating scheme; this package revalidates https at every
// hop and caps at 3.
const maxRedirects = 3

// fetchClient is a client dedicated to this package (D7).
var fetchClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("redirect to non-https URL refused: %s", req.URL.String())
		}
		return nil
	},
}

// allowedContentTypes are the only Content-Type values accepted for a
// third-party artifact fetch (D7); anything else is refused, not merely
// warned about.
var allowedContentTypes = map[string]bool{
	"text/plain":      true,
	"text/markdown":   true,
	"text/x-markdown": true,
}

// Fetch downloads the content at rawURL under the D7 network policy:
// https-only (validated before the first Get), a 30s timeout, at most 3
// redirect hops (each revalidated for https), a 2 MiB size cap, and a
// Content-Type allowlist.
func Fetch(rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("refused: URL scheme must be https, got %q", parsed.Scheme)
	}

	resp, err := fetchClient.Get(rawURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch failed: HTTP %d for %s", resp.StatusCode, rawURL)
	}

	contentType := resp.Header.Get("Content-Type")
	if !contentTypeAllowed(contentType) {
		return nil, fmt.Errorf("refused: unsupported Content-Type %q (allowed: text/plain, text/markdown, text/x-markdown)", contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxContentSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) > maxContentSize {
		return nil, fmt.Errorf("refused: content exceeds %d bytes", maxContentSize)
	}

	return body, nil
}

// contentTypeAllowed checks the Content-Type header against the D7
// allowlist, tolerating an optional "; charset=..." (or any other
// parameter) suffix.
func contentTypeAllowed(contentType string) bool {
	base := strings.TrimSpace(contentType)
	if idx := strings.Index(base, ";"); idx >= 0 {
		base = base[:idx]
	}
	base = strings.ToLower(strings.TrimSpace(base))
	return allowedContentTypes[base]
}
