package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestMetricsHandler_LogError_WhenEntriesDirIsRegularFile affirms R5: when
// translog.ReadAll fails (entries directory is a regular file, not a directory),
// the /api/metrics JSON response contains a non-empty log_error field. This
// ensures the board does not show zero metrics silently; the operator can see
// the degradation.
func TestMetricsHandler_LogError_WhenEntriesDirIsRegularFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Create the flat state directories so the handler can scan them.
	for _, s := range []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"} {
		if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps", s), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Place a regular FILE where .trackfw-log-entries/ should be — this triggers
	// a ReadAll error without requiring symlink creation (portable across platforms).
	entriesPath := filepath.Join(dir, "docs/roadmaps", ".trackfw-log-entries")
	if err := os.WriteFile(entriesPath, []byte("not-a-dir\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ProjectConfig{RoadmapDir: "docs/roadmaps"}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler(w, req, cfg)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	logErr, ok := body["log_error"]
	if !ok {
		t.Errorf("log_error field absent from JSON response; want non-empty string on ReadAll failure\nJSON: %+v", body)
	} else if s, _ := logErr.(string); s == "" {
		t.Errorf("log_error is present but empty; want non-empty error message\nJSON: %+v", body)
	}
}

// TestMetricsHandler_NoLogError_WhenLogHealthy affirms R5 negative arm: when the
// log directory is healthy (or absent), log_error must not appear in the JSON
// response.
func TestMetricsHandler_NoLogError_WhenLogHealthy(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	config.Reset()
	t.Cleanup(config.Reset)

	for _, s := range []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"} {
		if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps", s), 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ProjectConfig{RoadmapDir: "docs/roadmaps"}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler(w, req, cfg)

	// Read raw JSON to check for absence of log_error.
	raw := w.Body.String()
	if strings.Contains(raw, `"log_error"`) {
		t.Errorf("log_error must be absent when log is healthy; got: %s", raw)
	}
}
