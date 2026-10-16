//go:build windows

package pathguard

// End-to-end Windows DACL tests for GuardedWrite.
//
// These tests run only on Windows CI (windows-full-suites).
//
// ADR: docs/adr/ADR-2026-10-16-dacl-explicita-para-arquivos-de-governanca-no-windows.md

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// maskWriteAny covers all write-capability bits in an ACCESS_ALLOWED_ACE.
const maskWriteAny = windows.GENERIC_WRITE |
	windows.GENERIC_ALL |
	windows.FILE_WRITE_DATA |
	windows.FILE_APPEND_DATA |
	windows.FILE_WRITE_EA |
	windows.FILE_WRITE_ATTRIBUTES |
	windows.DELETE |
	windows.WRITE_DAC |
	windows.WRITE_OWNER

// makeForeignWriteDir creates a temp dir with Authenticated Users:(OI)(CI)(Modify)
// injected, simulating C:\ inheritance (run 38902064559 / ML-W2a).
// Duplicated from fsperm_windows_test.go — Go test helpers cannot cross package boundaries.
func makeForeignWriteDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	auSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		t.Fatalf("StringToSid(AU): %v", err)
	}
	newSDDL := "D:(A;OICI;0x001200a9;;;" + auSID.String() + ")"
	injectSD, err := windows.SecurityDescriptorFromString(newSDDL)
	if err != nil {
		t.Fatalf("SecurityDescriptorFromString(%q): %v", newSDDL, err)
	}
	injectDACL, _, err := injectSD.DACL()
	if err != nil {
		t.Fatalf("injectSD.DACL: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, injectDACL, nil,
	); err != nil {
		t.Fatalf("SetNamedSecurityInfo (inject AU): %v", err)
	}
	return dir
}

func dACLHasWriteACEForSID(t *testing.T, path string, sid *windows.SID) bool {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q): %v", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("sd.DACL: %v", err)
	}
	if dacl == nil {
		return true // NULL DACL = everyone full control
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			break
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if aceSID.Equals(sid) && ace.Mask&maskWriteAny != 0 {
			return true
		}
	}
	return false
}

func dACLIsProtected(t *testing.T, path string) bool {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q): %v", path, err)
	}
	ctrl, _, err := sd.Control()
	if err != nil {
		t.Fatalf("sd.Control: %v", err)
	}
	return ctrl&windows.SE_DACL_PROTECTED != 0
}

// TestGuardedWrite_FinalFileHasRestrictiveDACL asserts that GuardedWrite
// produces a final file (after atomic rename) whose DACL has no write ACE for
// Authenticated Users / BUILTIN\Users / Everyone, and whose DACL is protected.
//
// Reconciliation:
//   - The control arm proves that a plain os.CreateTemp in the same permissive
//     dir DOES have an AU write ACE — so "no ACE" in the assertion arm is not
//     a vacuous result from an already-restricted volume.
//   - Both assertions would fail if GuardedWrite used os.CreateTemp instead of
//     fsperm.CreateRestrictedFile, because the rename preserves whatever DACL
//     the temp file was born with.
func TestGuardedWrite_FinalFileHasRestrictiveDACL(t *testing.T) {
	root := makeForeignWriteDir(t)

	auSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		t.Fatalf("StringToSid(AU): %v", err)
	}
	usersSID, err := windows.StringToSid("S-1-5-32-545")
	if err != nil {
		t.Fatalf("StringToSid(Users): %v", err)
	}
	everyoneSID, err := windows.StringToSid("S-1-1-0")
	if err != nil {
		t.Fatalf("StringToSid(Everyone): %v", err)
	}

	// Control arm: os.CreateTemp in the same dir inherits AU write ACE.
	controlFile, err := os.CreateTemp(root, ".control-*")
	if err != nil {
		t.Fatalf("os.CreateTemp (control): %v", err)
	}
	controlFile.Close()
	if !dACLHasWriteACEForSID(t, controlFile.Name(), auSID) {
		t.Skip("PROBE w2b control-arm: os.CreateTemp in foreign dir does NOT have AU write ACE — volume may already be restricted; skipping assertion arm (vacuous)")
	}
	t.Logf("PROBE w2b control-arm PASS: os.CreateTemp inherits AU write ACE from foreign dir")

	// Assertion arm: GuardedWrite final file must NOT have those ACEs.
	dest := filepath.Join(root, ".trackfw", "test-identity.json")
	data := []byte(`{"schema_version":1}` + "\n")
	if err := GuardedWrite(root, dest, data, 0o600); err != nil {
		t.Fatalf("GuardedWrite: %v", err)
	}

	for _, tc := range []struct {
		sid  *windows.SID
		name string
	}{
		{auSID, "S-1-5-11 (Authenticated Users)"},
		{usersSID, "S-1-5-32-545 (BUILTIN\\Users)"},
		{everyoneSID, "S-1-1-0 (Everyone)"},
	} {
		if dACLHasWriteACEForSID(t, dest, tc.sid) {
			t.Errorf("PROBE w2b assertion-arm FAIL: final file %q has write ACE for %s", dest, tc.name)
		} else {
			t.Logf("PROBE w2b assertion-arm PASS: no write ACE for %s", tc.name)
		}
	}
	if !dACLIsProtected(t, dest) {
		t.Errorf("PROBE w2b assertion-arm FAIL: final file %q DACL is not protected (SE_DACL_PROTECTED missing)", dest)
	} else {
		t.Logf("PROBE w2b assertion-arm PASS: DACL is protected (SE_DACL_PROTECTED set)")
	}
}

// TestGuardedWrite_LegacyFileRetainsDACL measures whether a legacy file in a
// .trackfw directory that pre-existed the GuardedWrite call retains or loses its
// inherited permissive ACEs after GuardedWrite calls SetRestrictiveDirACL.
//
// Reconciliation: this test MEASURES and reports the result — it does not assert
// a specific outcome. Windows does not automatically re-apply a directory's new
// DACL to existing children (that requires TreeSetNamedSecurityInfo or an
// explicit "Apply to all children" propagation). The expected result is that the
// legacy file RETAINS its permissive ACEs. Measure as it comes; update the note
// in the ADR if the behaviour on the CI runner differs from expectation.
func TestGuardedWrite_LegacyFileRetainsDACL(t *testing.T) {
	root := makeForeignWriteDir(t)

	auSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		t.Fatalf("StringToSid(AU): %v", err)
	}

	// Create .trackfw dir explicitly with the permissive inheritance.
	trackfwDir := filepath.Join(root, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o700); err != nil {
		t.Fatalf("MkdirAll .trackfw: %v", err)
	}

	// Create a "legacy" file that inherits the dir's permissive DACL.
	legacyPath := filepath.Join(trackfwDir, "legacy.json")
	if err := os.WriteFile(legacyPath, []byte(`{}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile legacy: %v", err)
	}
	legacyHadAU := dACLHasWriteACEForSID(t, legacyPath, auSID)
	t.Logf("PROBE w2b legacy-before: legacy.json has AU write ACE = %v", legacyHadAU)

	// Perform GuardedWrite for a DIFFERENT file in the same directory.
	dest := filepath.Join(trackfwDir, "new.json")
	if err := GuardedWrite(root, dest, []byte(`{"schema_version":1}`+"\n"), 0o600); err != nil {
		t.Fatalf("GuardedWrite for new.json: %v", err)
	}

	legacyAfterAU := dACLHasWriteACEForSID(t, legacyPath, auSID)
	t.Logf("PROBE w2b legacy-after: legacy.json has AU write ACE = %v (was %v before GuardedWrite)", legacyAfterAU, legacyHadAU)

	if legacyHadAU && !legacyAfterAU {
		t.Logf("PROBE w2b legacy-result: SetRestrictiveDirACL DID propagate to existing child (unexpected — note in ADR)")
	} else if legacyHadAU && legacyAfterAU {
		t.Logf("PROBE w2b legacy-result: legacy file RETAINS permissive ACE after GuardedWrite (expected — DACL propagation does not touch existing files)")
	} else {
		t.Logf("PROBE w2b legacy-result: legacy file did not have AU write ACE before GuardedWrite (volume may already be restricted)")
	}
}
