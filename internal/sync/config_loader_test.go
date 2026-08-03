package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// This file covers ML-2A of REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-
// carregador-nos-tres-clis.md: NewLinearClient/NewJiraClient now resolve linear_*/jira_* fields
// through the single config.Load() path (cfg.Sync) instead of the removed artisanal
// readConfigField scanner. Every test resets the config singleton (see internal/config/config.go
// Reset doc comment) because each case writes its own trackfw.yaml in a fresh cwd.

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	config.Reset()
	t.Cleanup(config.Reset)
	return dir
}

func writeYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// AC5 — trackfw.yaml value takes precedence over the env var fallback.
func TestNewLinearClient_FileTakesPrecedenceOverEnv(t *testing.T) {
	dir := chdirTemp(t)
	writeYAML(t, dir, "linear_api_key: file-key\nlinear_team_id: file-team\n")
	t.Setenv("LINEAR_API_KEY", "env-key")
	t.Setenv("LINEAR_TEAM_ID", "env-team")

	c, err := NewLinearClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIKey != "file-key" {
		t.Errorf("APIKey: want file-key (trackfw.yaml wins), got %q", c.APIKey)
	}
	if c.TeamID != "file-team" {
		t.Errorf("TeamID: want file-team (trackfw.yaml wins), got %q", c.TeamID)
	}
}

// AC5 — env var is used only when trackfw.yaml has no value (absent file entirely here).
func TestNewLinearClient_EnvFallbackWhenFileAbsent(t *testing.T) {
	chdirTemp(t) // no trackfw.yaml written
	t.Setenv("LINEAR_API_KEY", "env-key")
	t.Setenv("LINEAR_TEAM_ID", "env-team")

	c, err := NewLinearClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIKey != "env-key" || c.TeamID != "env-team" {
		t.Errorf("expected env fallback, got APIKey=%q TeamID=%q", c.APIKey, c.TeamID)
	}
}

// AC5 — error text is byte-identical to the pre-refactor scanner's messages.
func TestNewLinearClient_ErrorMessagesPreserved(t *testing.T) {
	chdirTemp(t)
	t.Setenv("LINEAR_API_KEY", "")
	t.Setenv("LINEAR_TEAM_ID", "")

	_, err := NewLinearClient()
	if err == nil {
		t.Fatal("expected error when both trackfw.yaml and env vars are empty")
	}
	want := "Linear API key not found. Set LINEAR_API_KEY env var or linear_api_key in trackfw.yaml"
	if err.Error() != want {
		t.Errorf("error text changed:\n got:  %q\n want: %q", err.Error(), want)
	}

	config.Reset()
	writeYAML(t, ".", "linear_api_key: k\n")
	_, err = NewLinearClient()
	if err == nil {
		t.Fatal("expected error for missing team ID")
	}
	want = "Linear Team ID not found. Set LINEAR_TEAM_ID env var or linear_team_id in trackfw.yaml"
	if err.Error() != want {
		t.Errorf("error text changed:\n got:  %q\n want: %q", err.Error(), want)
	}
}

// AC4 — quoted value, trailing comment, colon-embedded scalar and a nested homonym key
// (linear_api_key repeated inside an unrelated mapping) all resolve to the root-level value.
// The artisanal scanner matched the first line with the "field:" prefix at ANY indentation,
// so the nested homonym below would have silently hijacked the value; the single YAML-library
// loader must not.
func TestNewLinearClient_AC4TrickyYAMLCases(t *testing.T) {
	dir := chdirTemp(t)
	writeYAML(t, dir, `some_unrelated_map:
  linear_api_key: hijacked-nested-value
linear_api_key: "quoted-root-key"  # trailing comment
linear_team_id: root-team
`)

	c, err := NewLinearClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIKey != "quoted-root-key" {
		t.Errorf("APIKey: want root-level quoted value \"quoted-root-key\", got %q (nested homonym or comment leaked in)", c.APIKey)
	}
}

// AC5 — trackfw.yaml value takes precedence for Jira too.
func TestNewJiraClient_FileTakesPrecedenceOverEnv(t *testing.T) {
	dir := chdirTemp(t)
	writeYAML(t, dir, `jira_base_url: "https://file.atlassian.net:443"
jira_email: file@example.com
jira_token: file-token
jira_project: FILEPROJ
`)
	t.Setenv("JIRA_BASE_URL", "https://env.atlassian.net")
	t.Setenv("JIRA_EMAIL", "env@example.com")
	t.Setenv("JIRA_TOKEN", "env-token")
	t.Setenv("JIRA_PROJECT", "ENVPROJ")

	c, err := NewJiraClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.BaseURL != "https://file.atlassian.net:443" {
		t.Errorf("BaseURL: want file value with embedded colon intact, got %q", c.BaseURL)
	}
	if c.Email != "file@example.com" || c.Token != "file-token" || c.Project != "FILEPROJ" {
		t.Errorf("expected trackfw.yaml to win over env, got %+v", c)
	}
}

// AC5 — env var fallback when trackfw.yaml is absent.
func TestNewJiraClient_EnvFallbackWhenFileAbsent(t *testing.T) {
	chdirTemp(t)
	t.Setenv("JIRA_BASE_URL", "https://env.atlassian.net")
	t.Setenv("JIRA_EMAIL", "env@example.com")
	t.Setenv("JIRA_TOKEN", "env-token")
	t.Setenv("JIRA_PROJECT", "ENVPROJ")

	c, err := NewJiraClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.BaseURL != "https://env.atlassian.net" || c.Email != "env@example.com" ||
		c.Token != "env-token" || c.Project != "ENVPROJ" {
		t.Errorf("expected env fallback, got %+v", c)
	}
}

// AC5 — error text byte-identical to the pre-refactor scanner's messages.
func TestNewJiraClient_ErrorMessagesPreserved(t *testing.T) {
	chdirTemp(t)
	t.Setenv("JIRA_BASE_URL", "")
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_PROJECT", "")

	_, err := NewJiraClient()
	if err == nil {
		t.Fatal("expected error")
	}
	want := "Jira base URL not found. Set JIRA_BASE_URL env var or jira_base_url in trackfw.yaml"
	if err.Error() != want {
		t.Errorf("error text changed:\n got:  %q\n want: %q", err.Error(), want)
	}
}

// AC4 — colon-in-value scalar (jira_base_url with an embedded port) resolves whole.
func TestNewJiraClient_AC4ColonEmbeddedScalar(t *testing.T) {
	dir := chdirTemp(t)
	writeYAML(t, dir, `jira_base_url: "https://x.atlassian.net:443"
jira_email: bot@example.com
jira_token: tok
jira_project: PROJ
`)
	c, err := NewJiraClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.BaseURL != "https://x.atlassian.net:443" {
		t.Errorf("BaseURL: want full colon-embedded value, got %q", c.BaseURL)
	}
}
