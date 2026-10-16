//go:build windows

package integrations

// End-to-end Windows DACL tests for atomicWrite.
//
// These tests run only on Windows CI (windows-full-suites). atomicWrite is a
// package-private function, so this file is in package integrations (white-box).
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
// Duplicated here — Go test helpers cannot cross package boundaries.
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
// Mask 0x1301bf = Modify (same as icacls "(M)").
// Owner/SY/BA retain FA to avoid locking the process out of the directory.
// Duplicated from fsperm_windows_test.go — Go test helpers cannot cross package boundaries.
func makeForeignWriteDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ownerSID, err := currentOwnerSIDForTest(t)
	if err != nil {
		t.Fatalf("makeForeignWriteDir: %v", err)
	}
	// 0x1301bf = Modify (icacls "(M)"): includes FILE_WRITE_DATA, FILE_APPEND_DATA,
	// FILE_WRITE_EA, FILE_WRITE_ATTRIBUTES, READ_CONTROL, SYNCHRONIZE, DELETE.
	sddl := "D:(A;OICI;0x1301bf;;;S-1-5-11)(A;OICI;FA;;;" + ownerSID.String() + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("makeForeignWriteDir: building SDDL %q: %v", sddl, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("makeForeignWriteDir: extracting DACL: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	); err != nil {
		t.Fatalf("makeForeignWriteDir: SetNamedSecurityInfo: %v", err)
	}
	return dir
}

func currentOwnerSIDForTest(t *testing.T) (*windows.SID, error) {
	t.Helper()
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, err
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return tu.User.Sid.Copy()
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
		return true
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

// TestAtomicWrite_FinalFileHasRestrictiveDACL asserts that atomicWrite produces
// a final file (after atomic rename) whose DACL has no write ACE for Authenticated
// Users / BUILTIN\Users / Everyone, and whose DACL is protected.
//
// Reconciliation:
//   - The control arm proves that a plain os.CreateTemp in the same permissive
//     dir DOES have an AU write ACE — so "no ACE" in the assertion arm is not a
//     vacuous result from an already-restricted volume.
//   - Both assertions would fail if atomicWrite used os.CreateTemp instead of
//     fsperm.CreateRestrictedFile, because os.Rename preserves the temp file's DACL.
func TestAtomicWrite_FinalFileHasRestrictiveDACL(t *testing.T) {
	dir := makeForeignWriteDir(t)

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
	controlFile, err := os.CreateTemp(dir, ".control-*")
	if err != nil {
		t.Fatalf("os.CreateTemp (control): %v", err)
	}
	controlFile.Close()
	if !dACLHasWriteACEForSID(t, controlFile.Name(), auSID) {
		t.Skip("PROBE w2b atomicWrite control-arm: os.CreateTemp does NOT have AU write ACE — volume may already be restricted; skipping assertion arm")
	}
	t.Logf("PROBE w2b atomicWrite control-arm PASS: os.CreateTemp inherits AU write ACE")

	// Assertion arm: atomicWrite final file must NOT have those ACEs.
	dest := filepath.Join(dir, "manifest.json")
	if err := atomicWrite(dest, []byte(`{}`+"\n"), 0o600); err != nil {
		t.Fatalf("atomicWrite: %v", err)
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
			t.Errorf("PROBE w2b atomicWrite assertion-arm FAIL: final file %q has write ACE for %s", dest, tc.name)
		} else {
			t.Logf("PROBE w2b atomicWrite assertion-arm PASS: no write ACE for %s", tc.name)
		}
	}
	if !dACLIsProtected(t, dest) {
		t.Errorf("PROBE w2b atomicWrite assertion-arm FAIL: final file %q DACL is not protected", dest)
	} else {
		t.Logf("PROBE w2b atomicWrite assertion-arm PASS: DACL is protected")
	}
}
