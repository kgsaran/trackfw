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
//       names the SID, remedy is /remove:g (explicit ACE), no /inheritance:d.
//   (b) Root with only owner, SYSTEM, Administrators ACEs: no warning.
//   (c) .trackfw\ with foreign write ACE, root clean: warning names .trackfw.
//   (d) ACE with only read/execute rights (mask 0x1200a9, not in D2): no warning.
//       This is the false-positive that P17 measured.
//   (e) Non-existent root path: D3 warning emitted.
//   (f) Root with (CI)(IO) FILE_ADD_SUBDIRECTORY for S-1-5-11 (R-1 fix): warning,
//       names the SID (CONTAINER_INHERIT_ACE with INHERIT_ONLY_ACE now counts).
//
// Additional table test:
//   EachBit: each D2 right in isolation triggers the rule (ensures no single bit
//            can be removed from d2WriteMask without breaking coverage).
//
// Remedy branch test:
//   InheritedACERemedy: child directory inherits OICI write ACE from parent;
//       warning cites /inheritance:d (inherited ACE path).
//
// Mordidas obrigatórias (falsificação):
//   (a) Remove the d2WriteMask check → (d) emits a spurious warning.
//   (b) Remove the "no warning" path → (b) emits a spurious warning.
//   (c) Remove the .trackfw check → (c) produces no warning.
//   (d) Widen d2WriteMask to include read bits → (d) emits a spurious warning.
//   (e) Silence errors → (e) returns nil instead of a warning.
//   (f) Remove CONTAINER_INHERIT_ACE from the ACE-counting condition → (f) returns no warning.
//   EachBit: remove that bit from d2WriteMask → the corresponding subcase returns no warning.
//   InheritedACERemedy: change inherited path to explicit path → assertion on /inheritance:d fails.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	. "github.com/kgsaran/trackfw/internal/validator"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// hasForeignWriteACEInDACL checks that at least one explicit foreign write
// ACE exists in a DACL — used to confirm control arms.
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

// readStoredMaskForSID reads the union of ACCESS_MASK values for all
// ACCESS_ALLOWED ACEs for sid in the directory's DACL (all ACEs, all flags).
// Returns 0 on error or if no matching ACE is found.
func readStoredMaskForSID(t *testing.T, dir string, sid *windows.SID) windows.ACCESS_MASK {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Logf("readStoredMaskForSID: GetNamedSecurityInfo(%q): %v", dir, err)
		return 0
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return 0
	}
	var combined windows.ACCESS_MASK
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if aceSID.Equals(sid) {
			combined |= ace.Mask
		}
	}
	return combined
}

// hasInheritedACEForSID checks that the directory has at least one ACCESS_ALLOWED
// ACE for sid with the INHERITED_ACE flag and a d2WriteMask intersection.
func hasInheritedACEForSID(t *testing.T, dir string, sid *windows.SID) bool {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("hasInheritedACEForSID: GetNamedSecurityInfo: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return false
	}
	const mask windows.ACCESS_MASK = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA |
		windows.ACCESS_MASK(0x40) | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER |
		windows.GENERIC_WRITE | windows.GENERIC_ALL
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		flags := uint32(ace.Header.AceFlags)
		if flags&windows.INHERITED_ACE == 0 {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if aceSID.Equals(sid) && ace.Mask&mask != 0 {
			return true
		}
	}
	return false
}

// localD2WriteMask mirrors d2WriteMask in the production code.
// Must be kept in sync with the constant in validator_project_foreign_write_windows.go.
// Used by TestProjectForeignWrite_ForeignWriteACE_EachBit for the isolation assertion.
const localD2WriteMask windows.ACCESS_MASK = windows.FILE_WRITE_DATA |
	windows.FILE_APPEND_DATA |
	windows.ACCESS_MASK(0x40) | // FILE_DELETE_CHILD
	windows.DELETE |
	windows.WRITE_DAC |
	windows.WRITE_OWNER |
	windows.GENERIC_WRITE |
	windows.GENERIC_ALL

// ---------------------------------------------------------------------------
// Tests (a)–(f)
// ---------------------------------------------------------------------------

// TestProjectForeignWrite_ForeignWriteACE asserts conclusion (a):
// a root directory with (OI)(CI) Modify for Authenticated Users (S-1-5-11)
// emits a warning that names the SID; with an explicit ACE the remedy cites
// /remove:g and not /inheritance:d.
//
// Reconciliation: this test affirms "the rule fires when a non-trusted SID has a
// write ACE on the root, the warning names the SID, and the remedy for an explicit
// ACE is /remove:g without /inheritance:d". It would fail if (a) d2WriteMask
// excluded ALL D2 bits present in 0x1301bf, (b) S-1-5-11 were added to the trusted
// set, or (c) the remedy message switched to the inherited-ACE form. The per-bit
// isolation is proven by TestProjectForeignWrite_ForeignWriteACE_EachBit.
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
	// R-5: explicit ACE fixture must produce /remove:g remedy and NOT /inheritance:d.
	if !strings.Contains(w, "/remove:g") {
		t.Errorf("warning for explicit ACE should mention /remove:g\nfull warning: %s", w)
	}
	if strings.Contains(w, "/inheritance:d") {
		t.Errorf("warning for explicit ACE must NOT mention /inheritance:d\nfull warning: %s", w)
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

// TestProjectForeignWrite_ContainerInheritOnlyACE asserts conclusion (f):
// a root with (CI)(IO) FILE_ADD_SUBDIRECTORY for S-1-5-11 emits a warning
// naming the SID. This is the R-1 fix: CONTAINER_INHERIT_ACE (CI) without
// OBJECT_INHERIT_ACE (OI), combined with INHERIT_ONLY_ACE (IO), now counts —
// it lets the attacker pre-create .trackfw (FILE_ADD_SUBDIRECTORY via container
// inheritance), which is the R-A vector measured in P19.
//
// Reconciliation: this test affirms "the rule fires when a CI|IO ACE grants a
// D2 write right to a non-trusted SID, even though INHERIT_ONLY is set and OI
// is absent". It would fail if CONTAINER_INHERIT_ACE were removed from the
// ACE-counting condition (reverting R-1), causing the ACE to be silently skipped.
func TestProjectForeignWrite_ContainerInheritOnlyACE(t *testing.T) {
	ownerSID := ownerSIDForTest(t)
	// (A;CIIO;AD;;;S-1-5-11): CI=CONTAINER_INHERIT, IO=INHERIT_ONLY,
	// AD=FILE_ADD_SUBDIRECTORY (0x4, same bit as FILE_APPEND_DATA).
	sddl := "D:(A;CIIO;AD;;;S-1-5-11)" +
		"(A;OICI;FA;;;" + ownerSID + ")" +
		"(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	root := buildProjectRoot(t, sddl)

	warnings := CheckProjectForeignWriteForTest(root)

	if len(warnings) == 0 {
		t.Fatalf("want ≥1 warning for CI|IO AU write ACE (R-1), got none")
	}
	w := warnings[0]
	if !strings.Contains(w, "S-1-5-11") {
		t.Errorf("warning missing SID %q\nfull warning: %s", "S-1-5-11", w)
	}
	t.Logf("PROBE project_foreign_write (f) PASS: warning = %s", w)
}

// ---------------------------------------------------------------------------
// Table test: per-bit isolation
// ---------------------------------------------------------------------------

// TestProjectForeignWrite_ForeignWriteACE_EachBit verifies that each D2 write
// right in isolation triggers the rule when the only foreign ACE has exactly
// that bit. This proves that removing any single bit from d2WriteMask would
// break coverage for that right.
//
// Reconciliation: each subcase affirms "a foreign ACE whose stored mask intersects
// d2WriteMask only at this one bit triggers the rule". It would fail if that bit
// were removed from d2WriteMask. The isolation assertion (stored mask ∩ d2 == bit)
// uses Skip (not Fatal) if Windows maps/expands the right on storage, because the
// rule still detects via the expanded specific bits — but the isolation claim no
// longer holds for that bit.
func TestProjectForeignWrite_ForeignWriteACE_EachBit(t *testing.T) {
	ownerSID := ownerSIDForTest(t)

	auSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		t.Fatalf("StringToSid S-1-5-11: %v", err)
	}

	cases := []struct {
		name string
		mask windows.ACCESS_MASK
	}{
		{"FILE_WRITE_DATA", windows.FILE_WRITE_DATA},
		{"FILE_APPEND_DATA", windows.FILE_APPEND_DATA},
		{"FILE_DELETE_CHILD", windows.ACCESS_MASK(0x40)},
		{"DELETE", windows.DELETE},
		{"WRITE_DAC", windows.WRITE_DAC},
		{"WRITE_OWNER", windows.WRITE_OWNER},
		{"GENERIC_WRITE", windows.GENERIC_WRITE},
		{"GENERIC_ALL", windows.GENERIC_ALL},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sddl := fmt.Sprintf("D:(A;OICI;0x%08x;;;S-1-5-11)", uint32(tc.mask)) +
				"(A;OICI;FA;;;" + ownerSID + ")" +
				"(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
			root := buildProjectRoot(t, sddl)

			// Isolation assertion: read back the stored ACE mask for AU and verify
			// that its intersection with d2WriteMask equals exactly the target bit.
			// If Windows expanded or remapped the right, Skip (not Fatal) because
			// the rule still fires via the expanded bits, but the isolation is weaker.
			storedMask := readStoredMaskForSID(t, root, auSID)
			storedD2Bits := storedMask & localD2WriteMask
			if storedD2Bits != tc.mask {
				t.Skipf(
					"isolation: Windows mapped/split the stored mask for %s: "+
						"input 0x%08x, stored d2 bits 0x%08x (expected 0x%08x) — "+
						"rule may still detect via expanded bits; isolation not exact",
					tc.name, uint32(tc.mask), uint32(storedD2Bits), uint32(tc.mask),
				)
			}

			warnings := CheckProjectForeignWriteForTest(root)

			if len(warnings) == 0 {
				t.Fatalf("want ≥1 warning for foreign SID holding %s (0x%08x), got none",
					tc.name, uint32(tc.mask))
			}
			if !strings.Contains(warnings[0], "S-1-5-11") {
				t.Errorf("warning missing SID S-1-5-11\nfull warning: %s", warnings[0])
			}
			t.Logf("PROBE project_foreign_write (a/%s) PASS: stored=0x%08x warning = %s",
				tc.name, uint32(storedMask), warnings[0])
		})
	}
}

// ---------------------------------------------------------------------------
// Remedy branch test: inherited ACE
// ---------------------------------------------------------------------------

// TestProjectForeignWrite_InheritedACERemedy verifies the R-5 inherited-ACE
// remedy path: when a child directory inherits an OICI write ACE from its parent,
// the warning cites /inheritance:d (in addition to /remove:g), not just /remove:g.
//
// The fixture creates a parent directory with an OI|CI AU write ACE (PROTECTED
// from its own grandparent), then creates a child directory that inherits naturally.
// The child's inherited ACEs carry INHERITED_ACE (0x10) — confirmed by the control
// arm before proceeding.
//
// Reconciliation: this test affirms "for an inherited ACE the warning mentions
// /inheritance:d; for an explicit ACE it does not". It would fail if the code
// always used the explicit-ACE message path (removing the INHERITED_ACE check),
// or if the inherited-ACE message were changed to omit /inheritance:d.
func TestProjectForeignWrite_InheritedACERemedy(t *testing.T) {
	ownerSID := ownerSIDForTest(t)
	// Parent has OICI write ACE for AU (file_write_data = 0x2).
	// PROTECTED: parent's own DACL is not contaminated by its parent (the OS temp dir).
	// The OICI flags cause the ACE to propagate to child containers/objects.
	parentSddl := "D:(A;OICI;0x2;;;S-1-5-11)" +
		"(A;OICI;FA;;;" + ownerSID + ")" +
		"(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	parent := t.TempDir()
	applyDACLFromSDDL(t, parent, parentSddl)

	// Child inherits from parent. Do NOT apply PROTECTED to child — we want
	// it to inherit the AU ACE with INHERITED_ACE flag.
	child := filepath.Join(parent, "project")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("MkdirAll child: %v", err)
	}

	// Control arm: verify the child actually received an inherited AU ACE.
	auSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		t.Fatalf("StringToSid: %v", err)
	}
	if !hasInheritedACEForSID(t, child, auSID) {
		t.Skip("fixture: child did not inherit AU write ACE from parent — " +
			"environment may not support ACL inheritance; skipping")
	}

	warnings := CheckProjectForeignWriteForTest(child)

	if len(warnings) == 0 {
		t.Fatalf("want ≥1 warning for inherited AU write ACE, got none")
	}
	w := warnings[0]
	if !strings.Contains(w, "S-1-5-11") {
		t.Errorf("warning missing SID S-1-5-11\nfull warning: %s", w)
	}
	// R-5: inherited ACE must produce /inheritance:d remedy.
	if !strings.Contains(w, "/inheritance:d") {
		t.Errorf("warning for inherited ACE should mention /inheritance:d\nfull warning: %s", w)
	}
	if !strings.Contains(w, "/remove:g") {
		t.Errorf("warning should mention /remove:g\nfull warning: %s", w)
	}
	t.Logf("PROBE project_foreign_write (inherited) PASS: warning = %s", w)
}
