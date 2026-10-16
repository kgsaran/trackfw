//go:build windows

package validator_test

// Tests for the project_foreign_write_windows rule (D2/D3,
// ADR-2026-10-16-no-windows-o-validate-avisa-quando-o-projeto-concede-escrita-a-outro-usuario).
//
// These tests run only on Windows CI (windows-probe / windows-full-suites).
// They use SetNamedSecurityInfo to build real DACL fixtures inside t.TempDir().
//
// Each test asserts one conclusion (Regra de Reconciliação, CLAUDE.md):
//
//   (a) Root with (OI)(CI) Modify for S-1-5-11 (Authenticated Users): warning,
//       names the SID.
//   (b) Root with only owner, SYSTEM, Administrators ACEs: no warning.
//   (c) .trackfw\ with foreign write ACE, root clean: warning names .trackfw.
//   (d) ACE with only read/execute rights (mask 0x1200a9, not in D2): no warning.
//       This is the false-positive that P17 measured.
//   (e) Non-existent root path: D3 warning emitted.
//
// Mordidas obrigatórias (falsificação):
//   (a) Remove the d2WriteMask check → (d) emits a spurious warning.
//   (b) Remove the "no warning" path → (b) emits a spurious warning.
//   (c) Remove the .trackfw check → (c) produces no warning.
//   (d) Widen d2WriteMask to include read bits → (d) emits a spurious warning.
//   (e) Silence errors → (e) returns nil instead of a warning.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	. "github.com/kgsaran/trackfw/internal/validator"
)

// buildProjectRoot creates a t.TempDir and sets its DACL to sddl (PROTECTED to
// prevent the runner's parent directory from merging ACEs into the fixture).
func buildProjectRoot(t *testing.T, sddl string) string {
	t.Helper()
	dir := t.TempDir()
	applyDACLFromSDDL(t, dir, sddl)
	return dir
}

// applyDACLFromSDDL applies the DACL described by sddl to path, with
// PROTECTED_DACL_SECURITY_INFORMATION so parent inheritance does not interfere.
func applyDACLFromSDDL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("applyDACLFromSDDL: parsing %q: %v", sddl, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("applyDACLFromSDDL: DACL from %q: %v", sddl, err)
	}
	if err := windows.SetNamedSecurityInfo(
		path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	); err != nil {
		t.Fatalf("applyDACLFromSDDL: SetNamedSecurityInfo(%q): %v", path, err)
	}
}

// ownerSIDForTest returns the SID of the current process owner as a string.
func ownerSIDForTest(t *testing.T) string {
	t.Helper()
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("ownerSIDForTest: OpenCurrentProcessToken: %v", err)
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		t.Fatalf("ownerSIDForTest: GetTokenUser: %v", err)
	}
	sid, err := tu.User.Sid.Copy()
	if err != nil {
		t.Fatalf("ownerSIDForTest: SID.Copy: %v", err)
	}
	return sid.String()
}

// hasForeignWriteACEInDACL is a helper to check that at least one foreign write
// ACE exists in a DACL — used in validation tests to confirm control arms.
func hasForeignWriteACEInDACL(t *testing.T, path string, foreignSIDStr string) bool {
	t.Helper()
	sid, err := windows.StringToSid(foreignSIDStr)
	if err != nil {
		t.Fatalf("hasForeignWriteACEInDACL: StringToSid(%q): %v", foreignSIDStr, err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("hasForeignWriteACEInDACL: GetNamedSecurityInfo(%q): %v", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return false
	}
	const mask = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA |
		windows.ACCESS_MASK(0x40) | // FILE_DELETE_CHILD
		windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER |
		windows.GENERIC_WRITE | windows.GENERIC_ALL
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			break
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if aceSID.Equals(sid) && ace.Mask&mask != 0 {
			return true
		}
	}
	return false
}

// TestProjectForeignWrite_ForeignWriteACE asserts conclusion (a):
// a root directory with (OI)(CI) Modify for Authenticated Users (S-1-5-11)
// emits a warning that names the SID.
//
// Reconciliation: this test affirms "the rule fires when a non-trusted SID has
// a write ACE". It would fail if d2WriteMask excluded FILE_WRITE_DATA (0x2)
// or if the AU SID was silently added to the trusted set.
func TestProjectForeignWrite_ForeignWriteACE(t *testing.T) {
	ownerSID := ownerSIDForTest(t)
	// 0x1301bf = Modify: FILE_WRITE_DATA, FILE_APPEND_DATA, FILE_WRITE_EA,
	// FILE_WRITE_ATTRIBUTES, READ_CONTROL, SYNCHRONIZE, DELETE.
	sddl := "D:(A;OICI;0x1301bf;;;S-1-5-11)" +
		"(A;OICI;FA;;;" + ownerSID + ")" +
		"(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	root := buildProjectRoot(t, sddl)

	// Control arm: confirm the fixture actually has the AU write ACE.
	if !hasForeignWriteACEInDACL(t, root, "S-1-5-11") {
		t.Skip("fixture control arm: AU write ACE not present — volume may not support ACLs; skipping")
	}

	warnings := CheckProjectForeignWriteForTest(root)

	if len(warnings) == 0 {
		t.Fatalf("want ≥1 warning for AU write ACE, got none")
	}
	w := warnings[0]
	if !strings.Contains(w, "S-1-5-11") {
		t.Errorf("warning missing SID %q\nfull warning: %s", "S-1-5-11", w)
	}
	t.Logf("PROBE project_foreign_write (a) PASS: warning = %s", w)
}

// TestProjectForeignWrite_CleanDACL asserts conclusion (b):
// a root with only owner, SYSTEM, Administrators ACEs emits no warning.
//
// Reconciliation: this test is the control arm. It would fail if the rule
// emitted warnings for trusted SIDs (SYSTEM, Administrators, owner).
func TestProjectForeignWrite_CleanDACL(t *testing.T) {
	ownerSID := ownerSIDForTest(t)
	sddl := "D:(A;OICI;FA;;;" + ownerSID + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	root := buildProjectRoot(t, sddl)

	warnings := CheckProjectForeignWriteForTest(root)

	if len(warnings) != 0 {
		t.Errorf("want no warnings for clean DACL, got %d: %v", len(warnings), warnings)
	}
	t.Logf("PROBE project_foreign_write (b) PASS: no warnings for clean DACL")
}

// TestProjectForeignWrite_TrackfwDirForeignACE asserts conclusion (c):
// a clean project root but a .trackfw\ subdirectory with a foreign write ACE
// emits a warning naming .trackfw\.
//
// Reconciliation: this test affirms "the .trackfw\ check fires independently".
// It would fail if the rule only checked the root and not .trackfw\.
func TestProjectForeignWrite_TrackfwDirForeignACE(t *testing.T) {
	ownerSID := ownerSIDForTest(t)
	cleanSddl := "D:(A;OICI;FA;;;" + ownerSID + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	root := buildProjectRoot(t, cleanSddl)

	// Create .trackfw\ with foreign write ACE.
	trackfwDir := filepath.Join(root, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o700); err != nil {
		t.Fatalf("MkdirAll .trackfw: %v", err)
	}
	foreignSddl := "D:(A;OICI;0x1301bf;;;S-1-5-11)" +
		"(A;OICI;FA;;;" + ownerSID + ")" +
		"(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	applyDACLFromSDDL(t, trackfwDir, foreignSddl)

	// Control arm: confirm the .trackfw fixture has the AU write ACE.
	if !hasForeignWriteACEInDACL(t, trackfwDir, "S-1-5-11") {
		t.Skip("fixture control arm: AU write ACE not present in .trackfw — skipping")
	}

	warnings := CheckProjectForeignWriteForTest(root)

	found := false
	for _, w := range warnings {
		if strings.Contains(w, ".trackfw") {
			found = true
			t.Logf("PROBE project_foreign_write (c) PASS: warning = %s", w)
		}
	}
	if !found {
		t.Errorf("want warning naming .trackfw, got: %v", warnings)
	}
}

// TestProjectForeignWrite_ReadOnlyACENoWarning asserts conclusion (d):
// an ACE granting only read/execute (mask 0x1200a9, not in D2) does NOT emit
// a warning. This is the false-positive from probe P17 that the D2 mask
// is designed to avoid.
//
// Reconciliation: this test affirms "only D2 write rights trigger the rule".
// It would fail if d2WriteMask was widened to include READ_CONTROL (0x20000)
// or any pure-read right such as FILE_READ_DATA (0x1).
func TestProjectForeignWrite_ReadOnlyACENoWarning(t *testing.T) {
	ownerSID := ownerSIDForTest(t)
	// 0x1200a9 = RX (read+execute): FILE_READ_DATA, FILE_READ_EA,
	// FILE_EXECUTE, FILE_READ_ATTRIBUTES, READ_CONTROL, SYNCHRONIZE.
	sddl := "D:(A;OICI;0x1200a9;;;S-1-5-11)" +
		"(A;OICI;FA;;;" + ownerSID + ")" +
		"(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	root := buildProjectRoot(t, sddl)

	warnings := CheckProjectForeignWriteForTest(root)

	if len(warnings) != 0 {
		t.Errorf("want no warnings for read-only ACE, got %d: %v", len(warnings), warnings)
	}
	t.Logf("PROBE project_foreign_write (d) PASS: no warning for read-only ACE (mask 0x1200a9)")
}

// TestProjectForeignWrite_DACLReadError asserts conclusion (e):
// a non-existent path emits a D3 warning (error reading DACL is never silent).
//
// Reconciliation: this test affirms "D3: errors produce a warning, not nil".
// It would fail if checkDirForeignWrite silenced GetNamedSecurityInfo errors.
func TestProjectForeignWrite_DACLReadError(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "no-such-directory-xyz")

	warnings := CheckProjectForeignWriteForTest(nonexistent)

	if len(warnings) == 0 {
		t.Fatalf("want D3 warning for unreadable path, got none")
	}
	w := warnings[0]
	if !strings.Contains(w, "project_foreign_write_windows") {
		t.Errorf("D3 warning missing rule prefix\nfull warning: %s", w)
	}
	t.Logf("PROBE project_foreign_write (e) PASS: D3 warning = %s", w)
}
