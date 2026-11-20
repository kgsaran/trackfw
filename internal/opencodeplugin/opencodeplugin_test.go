package opencodeplugin_test

// opencodeplugin_test.go — tests for Render (ML-5E, REQ-2026-11-17).
//
// Each test documents the conclusion it asserts (CLAUDE.md Regra Dura de
// Reconciliação): the conclusion line is the sentence another agent would
// use to decide whether to run this test.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/opencodeplugin"
)

// TestRender_MarkerOnFirstLine asserts:
// Render("project") and Render("global") both return content whose first line
// equals Marker — the audit sentinel required by ADR-2026-11-19 D1.
func TestRender_MarkerOnFirstLine(t *testing.T) {
	for _, scope := range []string{"project", "global"} {
		b, err := opencodeplugin.Render(scope)
		if err != nil {
			t.Fatalf("Render(%q): %v", scope, err)
		}
		firstLine := string(b)
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		firstLine = strings.TrimRight(firstLine, "\r")
		if firstLine != opencodeplugin.Marker {
			t.Errorf("Render(%q) first line = %q, want %q", scope, firstLine, opencodeplugin.Marker)
		}
	}
}

// TestRender_ScopeDiffers asserts:
// Render("project") and Render("global") differ in exactly one line — the
// SCOPE constant line. The global variant replaces "project" with "global".
func TestRender_ScopeDiffers(t *testing.T) {
	proj, err := opencodeplugin.Render("project")
	if err != nil {
		t.Fatalf("Render(project): %v", err)
	}
	glob, err := opencodeplugin.Render("global")
	if err != nil {
		t.Fatalf("Render(global): %v", err)
	}
	if bytes.Equal(proj, glob) {
		t.Error("Render(project) and Render(global) are byte-identical — they should differ on the SCOPE line")
	}
	// Project variant must contain the project SCOPE token.
	if !bytes.Contains(proj, []byte(`const SCOPE = "project"`)) {
		t.Error("Render(project) does not contain 'const SCOPE = \"project\"'")
	}
	// Global variant must contain the global SCOPE token.
	if !bytes.Contains(glob, []byte(`const SCOPE = "global"`)) {
		t.Error("Render(global) does not contain 'const SCOPE = \"global\"'")
	}
	// Global variant must NOT contain the project token.
	if bytes.Contains(glob, []byte(`const SCOPE = "project"`)) {
		t.Error("Render(global) still contains 'const SCOPE = \"project\"' — replacement failed")
	}
}

// TestRender_ProjectIsMutable asserts:
// Modifying the slice returned by Render("project") does not corrupt the
// embedded asset — the next Render("project") call returns the original content.
// (bytes.Clone in Render prevents shared-slice corruption.)
func TestRender_ProjectIsMutable(t *testing.T) {
	b1, err := opencodeplugin.Render("project")
	if err != nil {
		t.Fatalf("Render(project) first call: %v", err)
	}
	// Corrupt the returned slice.
	b1[0] = 'X'

	b2, err := opencodeplugin.Render("project")
	if err != nil {
		t.Fatalf("Render(project) second call: %v", err)
	}
	if b2[0] == 'X' {
		t.Error("Render(project) returned a shared slice — mutation of first call corrupted second call")
	}
}

// TestRender_UnknownScopeReturnsError asserts:
// Render with a scope other than "project" or "global" returns a non-nil error.
func TestRender_UnknownScopeReturnsError(t *testing.T) {
	_, err := opencodeplugin.Render("staging")
	if err == nil {
		t.Error("Render(\"staging\") returned nil error; want error for unknown scope")
	}
}
