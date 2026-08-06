package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_CredentialGuard_DefaultsToWarn covers ML-1A of ROADMAP-2026-08-05-hooks-de-guarda-
// contra-materializacao-de-credenciais-reais-por-subagentes.md: absent credential_guard key
// (and any project that ran `trackfw validate` before this ML existed) must default to "warn".
func TestLoad_CredentialGuard_DefaultsToWarn(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.CredentialGuard.Mode != "warn" {
		t.Errorf("CredentialGuard.Mode: want warn (default), got %q", cfg.CredentialGuard.Mode)
	}
}

// TestLoad_CredentialGuard_ModeBlock proves a trackfw.yaml with credential_guard: {mode: block}
// is read correctly.
func TestLoad_CredentialGuard_ModeBlock(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := "credential_guard:\n  mode: block\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.CredentialGuard.Mode != "block" {
		t.Errorf("CredentialGuard.Mode: want block, got %q", cfg.CredentialGuard.Mode)
	}
}

// TestLoad_CredentialGuard_InvalidModeFallsBackToWarn proves an unrecognized mode value is
// treated the same as absent — falls back to the safe default instead of propagating garbage.
func TestLoad_CredentialGuard_InvalidModeFallsBackToWarn(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := "credential_guard:\n  mode: nonsense\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.CredentialGuard.Mode != "warn" {
		t.Errorf("CredentialGuard.Mode: want warn (fallback), got %q", cfg.CredentialGuard.Mode)
	}
}

func TestLoad_NoFile(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 1 || cfg.ADRDirs[0] != "docs/adr" {
		t.Errorf("ADRDirs: want [docs/adr], got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: want docs/req, got %s", cfg.REQDir)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir: want docs/roadmaps, got %s", cfg.RoadmapDir)
	}
	if cfg.RoadmapNamespacing != "flat" {
		t.Errorf("RoadmapNamespacing: want flat, got %s", cfg.RoadmapNamespacing)
	}
	if cfg.WipLimit != 1 {
		t.Errorf("WipLimit: want 1, got %d", cfg.WipLimit)
	}
	if cfg.WipBySquad {
		t.Error("WipBySquad: want false, got true")
	}
	if cfg.RequireReqInCommit {
		t.Error("RequireReqInCommit: want false, got true")
	}
}

func TestLoad_WithFile_AllFields(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `adr_dirs:
  - docs/adr/zeus
  - docs/adr/done
req_dir: docs/requisições
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
  - zeus
  - apolo
  - afrodite
governance_mode: lenient
lenient_until: 2026-07-13
wip_limit: 2
wip_by_squad: true
require_req_in_commit: true
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 2 || cfg.ADRDirs[0] != "docs/adr/zeus" || cfg.ADRDirs[1] != "docs/adr/done" {
		t.Errorf("ADRDirs: got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/requisições" {
		t.Errorf("REQDir: want docs/requisições, got %s", cfg.REQDir)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir: got %s", cfg.RoadmapDir)
	}
	if cfg.RoadmapNamespacing != "by_agent" {
		t.Errorf("RoadmapNamespacing: want by_agent, got %s", cfg.RoadmapNamespacing)
	}
	if len(cfg.Agents) != 3 || cfg.Agents[0] != "zeus" || cfg.Agents[1] != "apolo" || cfg.Agents[2] != "afrodite" {
		t.Errorf("Agents: got %v", cfg.Agents)
	}
	if cfg.GovernanceMode != "lenient" {
		t.Errorf("GovernanceMode: want lenient, got %s", cfg.GovernanceMode)
	}
	if cfg.LenientUntil != "2026-07-13" {
		t.Errorf("LenientUntil: want 2026-07-13, got %s", cfg.LenientUntil)
	}
	if cfg.WipLimit != 2 {
		t.Errorf("WipLimit: want 2, got %d", cfg.WipLimit)
	}
	if !cfg.WipBySquad {
		t.Error("WipBySquad: want true, got false")
	}
	if !cfg.RequireReqInCommit {
		t.Error("RequireReqInCommit: want true, got false")
	}
}

func TestLoad_WithFile_PartialFields(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `req_dir: docs/requisitos
wip_limit: 3
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	// explicitly set field
	if cfg.REQDir != "docs/requisitos" {
		t.Errorf("REQDir: want docs/requisitos, got %s", cfg.REQDir)
	}
	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: want 3, got %d", cfg.WipLimit)
	}

	// omitted fields must use defaults
	if len(cfg.ADRDirs) != 1 || cfg.ADRDirs[0] != "docs/adr" {
		t.Errorf("ADRDirs should be default, got %v", cfg.ADRDirs)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir should be default, got %s", cfg.RoadmapDir)
	}
	if cfg.RoadmapNamespacing != "flat" {
		t.Errorf("RoadmapNamespacing should be default, got %s", cfg.RoadmapNamespacing)
	}
	if cfg.WipBySquad {
		t.Error("WipBySquad should be false (default)")
	}
}

func TestLoad_ADRDirs_List(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `adr_dirs:
  - docs/adr/zeus
  - docs/adr/apolo
  - docs/adr/afrodite
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 3 {
		t.Fatalf("ADRDirs: want 3 entries, got %d: %v", len(cfg.ADRDirs), cfg.ADRDirs)
	}
	if cfg.ADRDirs[0] != "docs/adr/zeus" {
		t.Errorf("ADRDirs[0]: want docs/adr/zeus, got %s", cfg.ADRDirs[0])
	}
	if cfg.ADRDirs[1] != "docs/adr/apolo" {
		t.Errorf("ADRDirs[1]: want docs/adr/apolo, got %s", cfg.ADRDirs[1])
	}
	if cfg.ADRDirs[2] != "docs/adr/afrodite" {
		t.Errorf("ADRDirs[2]: want docs/adr/afrodite, got %s", cfg.ADRDirs[2])
	}
}

func TestLoad_Agents_List(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `agents:
  - zeus
  - apolo
  - afrodite
  - artemis
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.Agents) != 4 {
		t.Fatalf("Agents: want 4, got %d: %v", len(cfg.Agents), cfg.Agents)
	}
	expected := []string{"zeus", "apolo", "afrodite", "artemis"}
	for i, name := range expected {
		if cfg.Agents[i] != name {
			t.Errorf("Agents[%d]: want %s, got %s", i, name, cfg.Agents[i])
		}
	}
}

func TestReset(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	// first load without file — defaults
	cfg1 := Load()
	if cfg1.WipLimit != 1 {
		t.Errorf("first load: WipLimit want 1, got %d", cfg1.WipLimit)
	}

	// write a file and reset
	yaml := `wip_limit: 5
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	Reset()

	cfg2 := Load()
	if cfg2.WipLimit != 5 {
		t.Errorf("after Reset: WipLimit want 5, got %d", cfg2.WipLimit)
	}
}

func TestLoad_StripsQuotesFromForgeAndTraceIDField(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := "forge: \"github\"\ntrace_id_field: 'req_id'\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.Forge != "github" {
		t.Fatalf("Forge: want github, got %q", cfg.Forge)
	}
	if cfg.TraceIdField != "req_id" {
		t.Fatalf("TraceIdField: want req_id, got %q", cfg.TraceIdField)
	}
}

// TestLoad_Malformed_FailsLoud proves Load — unlike parse — turns a genuine YAML syntax error
// into a fatal stderr message + non-zero exit, instead of the pre-ML-1B silent fallback to
// defaults (which was a regression relative to the handcrafted parser: an invalid file used to
// leave the rest of the config readable line-by-line; the library-based parser discards the
// whole document, so silently keeping defaults meant a typo could make trackfw quietly run with
// wip_limit=1 instead of the value actually configured).
func TestLoad_Malformed_FailsLoud(t *testing.T) {
	assertLoadFailsLoud(t, "agents: [zeus, apolo\nwip_limit: 3\n")
}

// TestLoad_MultipleDocuments_FailsLoud closes a divergence found in the ML-1B cross-CLI audit:
// yaml.Unmarshal silently decodes only the first "---"-delimited document in a stream (no
// error), while Node's `yaml` (MULTIPLE_DOCS) and PyYAML's yaml.compose() ("expected a single
// document in the stream") both reject a multi-document trackfw.yaml outright. Without
// hasMultipleDocuments, Go alone would exit 0 (silently reading only the first document) where
// Node and Python exit 1 — see hasMultipleDocuments's doc comment.
func TestLoad_MultipleDocuments_FailsLoud(t *testing.T) {
	assertLoadFailsLoud(t, "wip_limit: 3\n---\nwip_limit: 5\n")
}

// TestLoad_UndefinedAliasReference_FailsLoud closes a second divergence found in the same
// audit: a forward reference to an anchor not yet defined (b: *x / a: &x 3) is invalid per the
// YAML spec — yaml.v3 errors with "unknown anchor 'x' referenced" and PyYAML raises a
// ComposerError. This document has no "---" and no flow-sequence issue, so it exercises the
// primary yaml.Unmarshal error path, not hasMultipleDocuments.
func TestLoad_UndefinedAliasReference_FailsLoud(t *testing.T) {
	assertLoadFailsLoud(t, "b: *x\na: &x 3\n")
}

// TestLoad_DuplicateKeys_NotMalformed proves duplicate top-level keys do NOT trigger the fatal
// path in Go: yaml.Unmarshal into a generic *yaml.Node does not validate key uniqueness (only
// Decode into a typed struct would), and PyYAML's yaml.compose() is equally permissive — both
// silently resolve to "last key wins". Node's `yaml` package is the outlier here (it flags
// DUPLICATE_KEY as a composer error), so Node's parse() explicitly whitelists that one error
// code as non-fatal (see NON_FATAL_ERROR_CODES in npm/src/config/index.js) to keep the three
// CLIs' fatal trigger identical. This test guards the Go side of that convergence: Go must
// stay silent here, or the three would diverge again in the opposite direction.
func TestLoad_DuplicateKeys_NotMalformed(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	dup := "wip_limit: 3\nwip_limit: 4\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(dup), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.WipLimit != 4 {
		t.Errorf("WipLimit: got %d, want 4 (last key wins, no fatal exit)", cfg.WipLimit)
	}
}

// assertLoadFailsLoud writes content as trackfw.yaml in a fresh temp cwd, calls Load(), and
// asserts it hit the fatal path (osExit(1) + MalformedConfigMessage on stderr).
func assertLoadFailsLoud(t *testing.T, content string) {
	t.Helper()
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origExit := osExit
	var gotCode int
	exited := false
	osExit = func(code int) {
		exited = true
		gotCode = code
	}
	defer func() { osExit = origExit }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	Load()
	_ = w.Close()
	os.Stderr = origStderr

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	if !exited {
		t.Fatal("Load did not call osExit on malformed YAML")
	}
	if gotCode != 1 {
		t.Errorf("exit code: got %d, want 1", gotCode)
	}
	if stderr != MalformedConfigMessage+"\n" {
		t.Errorf("stderr: got %q, want %q", stderr, MalformedConfigMessage+"\n")
	}
}
