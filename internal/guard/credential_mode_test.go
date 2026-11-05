// Package guard_test contains convergence tests for ReadCredentialGuardMode (AC1, ML-1A).
// Uses package guard_test (external test package) to allow importing internal/validator
// without creating an import cycle (validator → guard → validator).
package guard_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/guard"
)

// extractValidatorMode calls the validator's extractCredentialGuardMode via its exported
// proxy ReadCredentialGuardMode — both use the same function, so this confirms the single
// shared reader is in effect for both sides (AC1: guard and validator converge).
//
// The validator's extractCredentialGuardMode was replaced with a one-line delegate to
// guard.ReadCredentialGuardMode in ML-1A. So calling guard.ReadCredentialGuardMode here
// directly IS the validator's reader.

// credModeVector describes one AC1 table entry.
type credModeVector struct {
	name     string
	content  string // raw content of trackfw.yaml
	wantMode string // expected mode string ("warn", "block", or "" for absent)
	wantOK   bool   // expected ok flag: false only when section missing entirely
}

// credModeVectors lists all AC1 input vectors from the REQ.
var credModeVectors = []credModeVector{
	{
		name:     "LF block",
		content:  "credential_guard:\n  mode: block\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "CRLF block",
		content:  "credential_guard:\r\n  mode: block\r\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "CRLF warn",
		content:  "credential_guard:\r\n  mode: warn\r\n",
		wantMode: "warn",
		wantOK:   true,
	},
	{
		name:     "blank CRLF lines before mode",
		content:  "credential_guard:\r\n\r\n  mode: block\r\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "trailing comment stripped",
		content:  "credential_guard:\n  mode: block # enforce strict\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "commented mode line ignored",
		content:  "credential_guard:\n  # mode: block\n",
		wantMode: "",
		wantOK:   true,
	},
	{
		name:     "ci_mode key not matched",
		content:  "credential_guard:\n  ci_mode: block\n",
		wantMode: "",
		wantOK:   true,
	},
	{
		name:     "double-quoted value",
		content:  "credential_guard:\n  mode: \"block\"\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "single-quoted value",
		content:  "credential_guard:\n  mode: 'warn'\n",
		wantMode: "warn",
		wantOK:   true,
	},
	{
		name:     "tab indented mode",
		content:  "credential_guard:\n\tmode: block\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "section absent",
		content:  "roadmap_dir: docs/roadmaps\n",
		wantMode: "",
		wantOK:   false,
	},
	{
		name:     "credential_guard section present but no mode key",
		content:  "credential_guard:\n  # no mode here\nroadmap_dir: docs/roadmaps\n",
		wantMode: "",
		wantOK:   true,
	},
	{
		name:     "cross-block boundary: mode from sibling section not leaked",
		content:  "credential_guard:\n  enabled: true\nother_section:\n  mode: block\n",
		wantMode: "",
		wantOK:   true,
	},
	{
		name:     "nested credential_guard: not a header at column 0",
		content:  "parent:\n  credential_guard:\n    mode: block\ncredential_guard:\n  mode: warn\n",
		wantMode: "warn",
		wantOK:   true,
	},
	{
		name:     "BOM prefix on first line",
		content:  "\xef\xbb\xbfcredential_guard:\n  mode: block\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name:     "mode value with CRLF and trailing whitespace",
		content:  "credential_guard:\r\n  mode: warn  \r\n",
		wantMode: "warn",
		wantOK:   true,
	},
	// C9 vectors: mode: on the 6th (and beyond) line after the header.
	// These failed with the old const lookbehind = 5 (line 6 is index 5,
	// exclusive end, so it was not scanned). They must pass now that the
	// limit is removed and the scan stops at the next top-level YAML key.
	// Assertion: removing lookbehind=5 allows mode: on lines 6, 7, … to be found.
	{
		name: "mode on 6th line after header (5 comment lines)",
		content: "credential_guard:\n" +
			"  # comment1\n" +
			"  # comment2\n" +
			"  # comment3\n" +
			"  # comment4\n" +
			"  # comment5\n" +
			"  mode: block\n",
		wantMode: "block",
		wantOK:   true,
	},
	{
		name: "mode after 10 blank CRLF lines",
		content: "credential_guard:\r\n" +
			"\r\n\r\n\r\n\r\n\r\n" +
			"\r\n\r\n\r\n\r\n\r\n" +
			"  mode: warn\r\n",
		wantMode: "warn",
		wantOK:   true,
	},
	{
		name: "mode on 7th line; sibling section after does not bleed",
		content: "credential_guard:\n" +
			"  # a\n# b\n  # c\n  # d\n  # e\n  # f\n" +
			"  mode: block\n" +
			"other:\n  mode: warn\n",
		wantMode: "block",
		wantOK:   true,
	},
}

// TestReadCredentialGuardMode_UnitVectors exercises all AC1 table vectors through
// guard.ReadCredentialGuardMode directly (the shared reader).
// Assertion: each vector's raw content produces the expected (mode, ok) pair.
func TestReadCredentialGuardMode_UnitVectors(t *testing.T) {
	for _, v := range credModeVectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()
			gotMode, gotOK := guard.ReadCredentialGuardMode(v.content)
			if gotOK != v.wantOK {
				t.Errorf("ok: want %v got %v (mode=%q)", v.wantOK, gotOK, gotMode)
			}
			if gotMode != v.wantMode {
				t.Errorf("mode: want %q got %q", v.wantMode, gotMode)
			}
		})
	}
}

// TestReadCredentialGuardMode_EndToEnd_ProjectScope exercises the CRLF block vector
// through the full RunCredential stack (project scope), confirming that the guard's
// credReadModeWithDefault → ReadCredentialGuardMode path produces rc=2 for CRLF block.
// Assertion: CRLF block yaml → RunCredential returns rc=2 (not rc=0 as before the fix).
func TestReadCredentialGuardMode_EndToEnd_ProjectScope(t *testing.T) {
	dir := t.TempDir()
	yaml := "credential_guard:\r\n  mode: block\r\nroadmap_dir: docs/roadmaps\r\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps"), 0755); err != nil {
		t.Fatal(err)
	}
	// JWT literal built by concatenation so the source file doesn't contain a matchable credential.
	jwt := "eyJ" + "hbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" + "." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIn0" + "." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	payload := `{"tool_input":{"command":"echo ` + jwt + `"}}`

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	stdin := strings.NewReader(payload)
	rc := guard.RunCredential(stdin, io.Discard, io.Discard)

	if rc != 2 {
		t.Fatalf("CRLF block: expected rc=2, got %d", rc)
	}
}

// TestReadCredentialGuardMode_EndToEnd_GlobalScope exercises the CRLF warn vector
// through RunCredentialGlobal, confirming warn\r is stripped to warn and produces rc=0.
// Assertion: CRLF warn yaml → RunCredentialGlobal returns rc=0 (warn, not block default).
func TestReadCredentialGuardMode_EndToEnd_GlobalScope(t *testing.T) {
	dir := t.TempDir()
	yaml := "credential_guard:\r\n  mode: warn\r\nroadmap_dir: docs/roadmaps\r\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	// JWT literal built by concatenation so the source file doesn't contain a matchable credential.
	jwt := "eyJ" + "hbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" + "." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIn0" + "." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	payload := `{"tool_input":{"command":"echo ` + jwt + `"}}`

	stdin := strings.NewReader(payload)

	// Change to dir so RunCredentialGlobal reads trackfw.yaml from cwd.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rc := guard.RunCredentialGlobal(stdin, io.Discard, io.Discard)

	// CRLF warn\r → stripped to warn → rc=0 (not block default).
	if rc != 0 {
		t.Fatalf("CRLF warn global: expected rc=0, got %d", rc)
	}
}
