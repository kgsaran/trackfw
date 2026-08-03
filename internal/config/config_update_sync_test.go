package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_UpdateAndSyncNamespaces exercises AC2/AC3 of
// REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md: the
// eleven fields historically read by the artisanal scanners (update.go / linear.go / jira.go)
// resolve as strings via the single config.Load() path, exposed under cfg.Update and cfg.Sync.
// Keys stay flat at the YAML root — only the in-memory struct is namespaced.
func TestLoad_UpdateAndSyncNamespaces(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `hooks: husky
ci: github
backend: go
frontend: react
pkg_manager: npm
linear_api_key: lin_api_abc123
linear_team_id: TEAM-1
jira_base_url: "https://x.atlassian.net:443"
jira_email: bot@example.com
jira_token: jira_tok_xyz
jira_project: PROJ
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if cfg.Update.Hooks != "husky" {
		t.Errorf("Update.Hooks: want husky, got %q", cfg.Update.Hooks)
	}
	if cfg.Update.CI != "github" {
		t.Errorf("Update.CI: want github, got %q", cfg.Update.CI)
	}
	if cfg.Update.Backend != "go" {
		t.Errorf("Update.Backend: want go, got %q", cfg.Update.Backend)
	}
	if cfg.Update.Frontend != "react" {
		t.Errorf("Update.Frontend: want react, got %q", cfg.Update.Frontend)
	}
	if cfg.Update.PkgManager != "npm" {
		t.Errorf("Update.PkgManager: want npm, got %q", cfg.Update.PkgManager)
	}
	if cfg.Sync.LinearAPIKey != "lin_api_abc123" {
		t.Errorf("Sync.LinearAPIKey: want lin_api_abc123, got %q", cfg.Sync.LinearAPIKey)
	}
	if cfg.Sync.LinearTeamID != "TEAM-1" {
		t.Errorf("Sync.LinearTeamID: want TEAM-1, got %q", cfg.Sync.LinearTeamID)
	}
	if cfg.Sync.JiraBaseURL != "https://x.atlassian.net:443" {
		t.Errorf("Sync.JiraBaseURL: want https://x.atlassian.net:443, got %q", cfg.Sync.JiraBaseURL)
	}
	if cfg.Sync.JiraEmail != "bot@example.com" {
		t.Errorf("Sync.JiraEmail: want bot@example.com, got %q", cfg.Sync.JiraEmail)
	}
	if cfg.Sync.JiraToken != "jira_tok_xyz" {
		t.Errorf("Sync.JiraToken: want jira_tok_xyz, got %q", cfg.Sync.JiraToken)
	}
	if cfg.Sync.JiraProject != "PROJ" {
		t.Errorf("Sync.JiraProject: want PROJ, got %q", cfg.Sync.JiraProject)
	}
}

// TestLoad_UpdateAndSyncNamespaces_AbsentDefaultsEmpty confirms the default for an absent field
// in each of the eleven keys is "" (AC per ML-1A spec: "Default de campo ausente no YAML: string
// vazia, nos 3 CLIs").
func TestLoad_UpdateAndSyncNamespaces_AbsentDefaultsEmpty(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if cfg.Update.Hooks != "" || cfg.Update.CI != "" || cfg.Update.Backend != "" ||
		cfg.Update.Frontend != "" || cfg.Update.PkgManager != "" {
		t.Errorf("Update namespace: want all empty strings, got %+v", cfg.Update)
	}
	if cfg.Sync.LinearAPIKey != "" || cfg.Sync.LinearTeamID != "" || cfg.Sync.JiraBaseURL != "" ||
		cfg.Sync.JiraEmail != "" || cfg.Sync.JiraToken != "" || cfg.Sync.JiraProject != "" {
		t.Errorf("Sync namespace: want all empty strings, got %+v", cfg.Sync)
	}
}
