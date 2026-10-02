package validator

// validator_adr_prefix_test.go — tests for D1 criterion (ADR-2026-10-02) and
// rule adr_file_without_prefix (D4).
//
// Each test declares one sentence in its doc-comment starting with "Asserts:" — this
// is the reconciliation mandated by the Regra Dura de Reconciliação in CLAUDE.md.
//
// Overlay proof (run externally; FAIL / PASS evidence in the ML report):
//
//	TestWalkADRFilePathsForRule_NotasNotEnumerated         — sabotage: old predicate (!d.IsDir() && HasSuffix(path,".md"))
//	TestWalkADRFilePathsForRule_LowercaseADREnumerated     — sabotage: case-sensitive HasPrefix(name,"ADR-") without ToUpper
//	TestWalkADRFilePathsForRule_SymlinkDirNotEnumerated    — sabotage: old predicate (no IsRegular check)
//	TestWalkADRFilePathsForRule_SymlinkFileEnumerated      — sabotage: ML-1C revert (d.Type().IsRegular() only, no symlink branch)
//	TestWalkADRFilePathsForRule_SymlinkBrokenNotEnumerated — (passes with both old and new; broken link excluded by os.Stat error)
//	TestADRFileWithoutPrefix_*                             — sabotage: remove isADRFileName guard in validateADRFilesWithoutPrefix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestWalkADRFilePathsForRule_NotasNotEnumerated
//
// Asserts: NOTAS.md in adr_dirs is NOT enumerated after the D1 criterion change; only
// files with ADR- prefix (case-insensitive) and .md suffix are counted.
func TestWalkADRFilePathsForRule_NotasNotEnumerated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/adr/NOTAS.md", "# notas\n")
	writeFile(t, dir, "docs/adr/README.md", "# readme\n")
	writeFile(t, dir, "docs/adr/ADR-2026-01-01-real.md", "---\nstatus: Accepted\n---\n")

	got := walkADRFilePaths(filepath.Join(dir, "docs/adr"))
	for _, p := range got {
		name := filepath.Base(p)
		if name == "NOTAS.md" || name == "README.md" {
			t.Errorf("D1: %q must not be enumerated, got: %v", name, got)
		}
	}
	if len(got) != 1 {
		t.Errorf("D1: expected exactly 1 ADR (ADR-2026-01-01-real.md), got %d: %v", len(got), got)
	}
}

// TestWalkADRFilePathsForRule_LowercaseADREnumerated
//
// Asserts: adr-001-x.md (all-lowercase prefix) IS enumerated — the D1 criterion is
// case-insensitive (strings.ToUpper before HasPrefix).
func TestWalkADRFilePathsForRule_LowercaseADREnumerated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/adr/adr-001-lowercase.md", "---\nstatus: Accepted\n---\n")
	writeFile(t, dir, "docs/adr/Adr-002-mixedcase.md", "---\nstatus: Accepted\n---\n")

	got := walkADRFilePaths(filepath.Join(dir, "docs/adr"))
	if len(got) != 2 {
		t.Errorf("D1 case-insensitive: expected 2 ADRs (adr-001 and Adr-002), got %d: %v", len(got), got)
	}
}

// TestWalkADRFilePathsForRule_UppercaseADREnumerated
//
// Asserts: ADR-*.md (canonical uppercase prefix) IS enumerated.
func TestWalkADRFilePathsForRule_UppercaseADREnumerated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/adr/ADR-2026-10-01-canonical.md", "---\nstatus: Draft\n---\n")

	got := walkADRFilePaths(filepath.Join(dir, "docs/adr"))
	if len(got) != 1 {
		t.Errorf("D1: expected 1 ADR (ADR-2026-10-01-canonical.md), got %d: %v", len(got), got)
	}
}

// TestWalkADRFilePathsForRule_SymlinkDirNotEnumerated
//
// Asserts: a directory symlink named ADR-x.md in adr_dirs is NOT enumerated because
// os.Stat follows the link and returns a directory Mode, which is not IsRegular().
// Skipped when symlink creation fails (Windows without Developer Mode).
func TestWalkADRFilePathsForRule_SymlinkDirNotEnumerated(t *testing.T) {
	dir := t.TempDir()
	adrDir := filepath.Join(dir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a real directory outside adr_dirs to be the symlink target.
	outsideDir := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Real ADR file for vacuity: the enumerator must still work.
	writeFile(t, dir, "docs/adr/ADR-2026-01-01-real.md", "---\nstatus: Accepted\n---\n")

	symlinkPath := filepath.Join(adrDir, "ADR-evil.md")
	if !symlinkOrSkip(t, outsideDir, symlinkPath) {
		return
	}

	got := walkADRFilePaths(adrDir)
	for _, p := range got {
		if filepath.Base(p) == "ADR-evil.md" {
			t.Errorf("D1+A3: directory symlink ADR-evil.md must not be enumerated, got: %v", got)
		}
	}
	if len(got) != 1 {
		t.Errorf("D1+A3: expected 1 ADR (ADR-2026-01-01-real.md) after excluding symlink dir, got %d: %v", len(got), got)
	}
}

// TestWalkADRFilePathsForRule_SymlinkFileEnumerated — ML-1C
//
// Asserts: a symlink named ADR-link.md pointing to a regular .md file outside adr_dirs
// IS enumerated, because os.Stat follows the link and returns a regular file Mode.
// Skipped when symlink creation fails (Windows without Developer Mode).
func TestWalkADRFilePathsForRule_SymlinkFileEnumerated(t *testing.T) {
	dir := t.TempDir()
	adrDir := filepath.Join(dir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Real ADR file outside adr_dirs — this is the symlink target.
	outsideDir := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(outsideDir, "ADR-real.md")
	if err := os.WriteFile(realFile, []byte("---\nstatus: Accepted\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(adrDir, "ADR-link.md")
	if !symlinkOrSkip(t, realFile, symlinkPath) {
		return
	}

	got := walkADRFilePaths(adrDir)
	found := false
	for _, p := range got {
		if filepath.Base(p) == "ADR-link.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("ML-1C: symlink ADR-link.md → regular file must be enumerated, got: %v", got)
	}
}

// TestWalkADRFilePathsForRule_SymlinkBrokenNotEnumerated — ML-1C
//
// Asserts: a broken symlink named ADR-quebrado.md in adr_dirs is NOT enumerated because
// os.Stat returns an error for broken symlinks (target does not exist).
// Skipped when symlink creation fails (Windows without Developer Mode).
func TestWalkADRFilePathsForRule_SymlinkBrokenNotEnumerated(t *testing.T) {
	dir := t.TempDir()
	adrDir := filepath.Join(dir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Real ADR file for vacuity: the enumerator must still work.
	writeFile(t, dir, "docs/adr/ADR-2026-01-01-real.md", "---\nstatus: Accepted\n---\n")

	// Create a broken symlink (target does not exist).
	brokenTarget := filepath.Join(dir, "nonexistent", "ADR-real.md")
	symlinkPath := filepath.Join(adrDir, "ADR-quebrado.md")
	if !symlinkOrSkip(t, brokenTarget, symlinkPath) {
		return
	}

	got := walkADRFilePaths(adrDir)
	for _, p := range got {
		if filepath.Base(p) == "ADR-quebrado.md" {
			t.Errorf("ML-1C: broken symlink ADR-quebrado.md must not be enumerated, got: %v", got)
		}
	}
	if len(got) != 1 {
		t.Errorf("ML-1C: expected exactly 1 ADR (ADR-2026-01-01-real.md) with broken symlink excluded, got %d: %v", len(got), got)
	}
}

// TestADRFileWithoutPrefix_FrontmatterStatusDispara
//
// Asserts: a .md file in adr_dirs without ADR- prefix whose content has frontmatter
// status: fires adr_file_without_prefix (D4).
func TestADRFileWithoutPrefix_FrontmatterStatusDispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr\n")
	writeFile(t, dir, "docs/adr/decisao.md", "---\nstatus: Draft\ndate: 2026-10-01\n---\n# Decisão\n")

	warnings, err := validateADRFilesWithoutPrefix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasWarning(warnings, `"decisao.md"`) || !hasWarning(warnings, "declares a status") {
		t.Errorf("D4: expected warning for decisao.md with frontmatter status:, got: %v", warnings)
	}
}

// TestADRFileWithoutPrefix_CabecalhoStatusDispara
//
// Asserts: a .md file in adr_dirs without ADR- prefix with a "| Status: Draft" header
// line (no frontmatter) fires adr_file_without_prefix via resolveAdrStatus fallback.
func TestADRFileWithoutPrefix_CabecalhoStatusDispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr\n")
	writeFile(t, dir, "docs/adr/legado.md", "# Legado\n\n> Date: 2026-01-01 | Status: Draft\n\n## Context\n")

	warnings, err := validateADRFilesWithoutPrefix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasWarning(warnings, `"legado.md"`) || !hasWarning(warnings, "declares a status") {
		t.Errorf("D4: expected warning for legado.md with | Status: Draft header, got: %v", warnings)
	}
}

// TestADRFileWithoutPrefix_READMESemStatusNaoDispara
//
// Asserts: README.md in adr_dirs without any status field (no frontmatter, no | Status:
// header) does NOT fire adr_file_without_prefix (resolveAdrStatus returns "").
func TestADRFileWithoutPrefix_READMESemStatusNaoDispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr\n")
	writeFile(t, dir, "docs/adr/README.md", "# Index\n\nThis directory contains ADRs.\n")

	warnings, err := validateADRFilesWithoutPrefix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "README.md") {
			t.Errorf("D4: README.md without status must not fire adr_file_without_prefix, got: %v", warnings)
		}
	}
}

// TestADRFileWithoutPrefix_ADRPrefixadoNaoDispara
//
// Asserts: ADR-*.md files in adr_dirs do NOT fire adr_file_without_prefix even when
// they have a status — the rule only targets mis-named (non-prefixed) files.
func TestADRFileWithoutPrefix_ADRPrefixadoNaoDispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr\n")
	writeFile(t, dir, "docs/adr/ADR-2026-10-01-normal.md", "---\nstatus: Draft\n---\n# ADR\n")

	warnings, err := validateADRFilesWithoutPrefix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("D4: ADR-prefixed file must not fire adr_file_without_prefix, got: %v", warnings)
	}
}

// TestBlockedByDraftADR_ArquivoSemPrefixo — A1 from Wave 0 parecer
//
// Asserts: a REQ with blocked_by: pointing to decisao.md (without ADR- prefix, with
// status: Draft in frontmatter) still fires blocked_by_draft_adr — findADRFile does
// NOT apply the D1 prefix criterion (Wave 0 decision, ADR D2 exception).
func TestBlockedByDraftADR_ArquivoSemPrefixo(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	// ADR without ADR- prefix, but with frontmatter status: Draft.
	writeFile(t, dir, "docs/adr/decisao.md", "---\nstatus: Draft\ndate: 2026-10-01\nauthor: \"\"\n---\n\n# Decisão: bloqueio\n\n> Date: 2026-10-01 | Status: Draft\n")

	// REQ Open with blocked_by: decisao.md (no ADR- prefix, no .md extension is OK — basename).
	writeFile(t, dir, "docs/req/REQ-2026-10-01-bloqueada.md", `---
status: Open
date: 2026-10-01
author: ""
adr: ""
roadmap: ""
---

# REQ: bloqueada

> Date: 2026-10-01 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR:

## Blocked by ADRs
- decisao.md (Draft)

## Linked Roadmap
Roadmap:
`)

	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) == 0 {
		t.Errorf("A1: blocked_by_draft_adr must fire for decisao.md (no ADR- prefix, status: Draft); violations: %v", violations)
	}
}
