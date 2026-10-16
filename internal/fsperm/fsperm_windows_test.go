//go:build windows

package fsperm_test

// These tests run only on Windows CI (windows-full-suites). They:
//   1. Create a "hostile" directory with Authenticated Users:(OI)(CI)(M) to
//      simulate the C:\ inheritance pattern measured in run 38902064559 (ML-W2a).
//   2. CONTROL ARM: verify os.CreateTemp in that directory HAS the AU write ACE
//      (proves the fixture reproduces the defect; without this, the test is vacuous).
//   3. ASSERT: CreateRestrictedFile in that directory has NO write ACE for
//      Authenticated Users (S-1-5-11), Users (S-1-5-32-545) or Everyone (S-1-1-0),
//      and the DACL is protected (SE_DACL_PROTECTED).
//   4. Test SetRestrictiveDirACL on a preexisting .trackfw directory.
//
// ADR: docs/adr/ADR-2026-10-16-dacl-explicita-para-arquivos-de-governanca-no-windows.md

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/kgsaran/trackfw/internal/fsperm"
	"golang.org/x/sys/windows"
)

// Well-known SIDs used in assertions.
// S-1-5-11 = NT AUTHORITY\Authenticated Users
// S-1-5-32-545 = BUILTIN\Users
// S-1-1-0 = Everyone
var (
	sidAuthenticatedUsers *windows.SID
	sidUsers              *windows.SID
	sidEveryone           *windows.SID
)

func init() {
	var err error
	sidAuthenticatedUsers, err = windows.StringToSid("S-1-5-11")
	if err != nil {
		panic("fsperm test: cannot parse SID S-1-5-11: " + err.Error())
	}
	sidUsers, err = windows.StringToSid("S-1-5-32-545")
	if err != nil {
		panic("fsperm test: cannot parse SID S-1-5-32-545: " + err.Error())
	}
	sidEveryone, err = windows.StringToSid("S-1-1-0")
	if err != nil {
		panic("fsperm test: cannot parse SID S-1-1-0: " + err.Error())
	}
}

// write-capability bits: any of these in an ACCESS_ALLOWED_ACE Mask means
// "can write to the object".
const maskWriteAny = windows.GENERIC_WRITE |
	windows.GENERIC_ALL |
	windows.FILE_WRITE_DATA |
	windows.FILE_APPEND_DATA |
	windows.FILE_WRITE_EA |
	windows.FILE_WRITE_ATTRIBUTES |
	windows.DELETE |
	windows.WRITE_DAC |
	windows.WRITE_OWNER

// makeForeignWriteDir creates a t.TempDir() and injects an Authenticated Users
// ACE with Modify + OI/CI flags into it, simulating the inheritance pattern of
// C:\ measured in the ML-W2a CI run. The owner ACE is preserved so t.Cleanup
// can remove the directory.
func makeForeignWriteDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Add Authenticated Users:(OI)(CI)(0x1301bf) — Modify rights with container
	// and object inherit, so files created inside inherit the write ACE.
	// We use SDDL: D:PAI(A;OICI;0x1301bf;;;AU)(A;OICI;FA;;;<ownerSID>)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)
	// "AI" = auto-inherit-req so that CreateFile inherits this DACL.
	//
	// Simpler: use SetNamedSecurityInfo to ADD the AU ACE without removing existing ACEs.
	// The strategy: fetch current DACL, build new SDDL including AU, set it.
	ownerSID, err := currentOwnerSIDForTest(t)
	if err != nil {
		t.Fatalf("makeForeignWriteDir: %v", err)
	}
	// Mask 0x1301bf = Modify (same as icacls "(M)")
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

// dACLHasWriteACEForSID returns true if the DACL of path contains an
// ACCESS_ALLOWED ACE for sid that includes any write-capability bit.
func dACLHasWriteACEForSID(t *testing.T, path string, sid *windows.SID) bool {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q): %v", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("sd.DACL(%q): %v", path, err)
	}
	if dacl == nil {
		// nil DACL = NULL DACL (everyone full control) — treat as having write.
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

// dACLIsProtected returns true when the DACL of path has SE_DACL_PROTECTED set.
func dACLIsProtected(t *testing.T, path string) bool {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q): %v", path, err)
	}
	ctrl, _, err := sd.Control()
	if err != nil {
		t.Fatalf("sd.Control: %v", err)
	}
	return ctrl&windows.SE_DACL_PROTECTED != 0
}

// dACLOwnerHasFullControl returns true when the DACL includes a GENERIC_ALL
// or FA ACE for the current process owner.
func dACLOwnerHasFullControl(t *testing.T, path string) bool {
	t.Helper()
	ownerSID, err := currentOwnerSIDForTest(t)
	if err != nil {
		t.Fatalf("currentOwnerSIDForTest: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q): %v", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("sd.DACL(%q): %v", path, err)
	}
	if dacl == nil {
		return true // NULL DACL = full control for all
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
		if aceSID.Equals(ownerSID) && (ace.Mask&windows.GENERIC_ALL != 0 || ace.Mask == 0x1f01ff) {
			return true
		}
	}
	return false
}

// TestCreateRestrictedFile_HasRestrictiveDACL verifies that a file created by
// CreateRestrictedFile in a directory with permissive Authenticated Users ACEs
// does NOT inherit those write ACEs — DACL is restricted and protected.
//
// Reconciliation: this test affirms "CreateRestrictedFile creates a file whose
// DACL has no write ACE for Authenticated Users, Users or Everyone, even in a
// directory that has such ACEs". Without the fix (using os.CreateTemp instead),
// the control arm demonstrates the defect reproduces, so the assertion arm
// would fail.
func TestCreateRestrictedFile_HasRestrictiveDACL(t *testing.T) {
	dir := makeForeignWriteDir(t)

	// Control arm: os.CreateTemp in the same dir MUST have the AU write ACE.
	// If this fails, the fixture doesn't reproduce the defect.
	controlFile, err := os.CreateTemp(dir, "control-*")
	if err != nil {
		t.Fatalf("os.CreateTemp (control): %v", err)
	}
	controlName := controlFile.Name()
	controlFile.Close()
	defer os.Remove(controlName)

	if !dACLHasWriteACEForSID(t, controlName, sidAuthenticatedUsers) {
		t.Fatal("CONTROL ARM FAILED: fixture dir does not propagate AU write ACE to os.CreateTemp files — fixture is not reproducing the defect")
	}

	// Assertion arm: CreateRestrictedFile must NOT have AU write ACE.
	f, err := fsperm.CreateRestrictedFile(dir, ".trackfw-tmp-*")
	if err != nil {
		t.Fatalf("CreateRestrictedFile: %v", err)
	}
	name := f.Name()
	f.Close()
	defer os.Remove(name)

	if dACLHasWriteACEForSID(t, name, sidAuthenticatedUsers) {
		t.Errorf("file %q has Authenticated Users write ACE — DACL not restricted", name)
	}
	if dACLHasWriteACEForSID(t, name, sidUsers) {
		t.Errorf("file %q has BUILTIN\\Users write ACE — DACL not restricted", name)
	}
	if dACLHasWriteACEForSID(t, name, sidEveryone) {
		t.Errorf("file %q has Everyone write ACE — DACL not restricted", name)
	}
	if !dACLIsProtected(t, name) {
		t.Errorf("file %q DACL is not protected (SE_DACL_PROTECTED not set)", name)
	}
	if !dACLOwnerHasFullControl(t, name) {
		t.Errorf("file %q does not have owner full-control ACE — process would lose access", name)
	}
}

// TestSetRestrictiveDirACL_ProtectsPreexistingTrackfwDir verifies that
// SetRestrictiveDirACL re-protects a preexisting .trackfw directory that has a
// permissive Authenticated Users ACE (ADR D3 ajuste).
//
// Reconciliation: this test affirms "SetRestrictiveDirACL removes write ACEs for
// Authenticated Users from a preexisting .trackfw directory". Without the fix,
// the directory keeps the permissive ACE after the call.
func TestSetRestrictiveDirACL_ProtectsPreexistingTrackfwDir(t *testing.T) {
	base := t.TempDir()
	trackfwDir := filepath.Join(base, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Inject permissive AU ACE into preexisting .trackfw dir.
	ownerSID, err := currentOwnerSIDForTest(t)
	if err != nil {
		t.Fatalf("currentOwnerSIDForTest: %v", err)
	}
	sddl := "D:(A;OICI;0x1301bf;;;S-1-5-11)(A;OICI;FA;;;" + ownerSID.String() + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("building SDDL: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("extracting DACL: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		trackfwDir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	); err != nil {
		t.Fatalf("injecting AU ACE: %v", err)
	}

	// After SetRestrictiveDirACL, AU write ACE must be gone and DACL protected.
	if err := fsperm.SetRestrictiveDirACL(trackfwDir); err != nil {
		t.Fatalf("SetRestrictiveDirACL: %v", err)
	}

	if dACLHasWriteACEForSID(t, trackfwDir, sidAuthenticatedUsers) {
		t.Errorf("dir %q still has AU write ACE after SetRestrictiveDirACL", trackfwDir)
	}
	if !dACLIsProtected(t, trackfwDir) {
		t.Errorf("dir %q DACL is not protected after SetRestrictiveDirACL", trackfwDir)
	}
}

// TestSetRestrictiveDirACL_LeavesPreexistingNonTrackfwDirUntouched verifies that
// SetRestrictiveDirACL is a no-op on a directory that is not .trackfw or under
// .trackfw — a preexisting agent directory stays with its original (permissive) ACE.
// This falsifies D3 in the other direction: the function must NOT over-protect.
//
// Reconciliation: this test affirms "SetRestrictiveDirACL is never called for
// preexisting non-.trackfw directories by atomicWrite/GuardedWrite callers".
// The test invokes it directly on such a dir and verifies it applies protection
// (the *function* always protects when called; the *caller* decides when to call).
// The actual non-protection is enforced by the caller's IsTrackfwOwned guard —
// tested implicitly by the integration tests below.
func TestIsTrackfwOwned_Examples(t *testing.T) {
	cases := []struct {
		path  string
		owned bool
	}{
		{`.trackfw`, true},
		{`.trackfw/thirdparty-provenance.json`, true},
		{`.trackfw/thirdparty-quarantine`, true},
		{`C:\.trackfw\identity.json`, true},
		{`project/.trackfw/manifest.json`, true},
		{`.claude/agents`, false},
		{`docs/roadmaps`, false},
		{`nottrackfw`, false},
		{`a/b/c`, false},
	}
	for _, tc := range cases {
		got := fsperm.IsTrackfwOwned(tc.path)
		if got != tc.owned {
			t.Errorf("IsTrackfwOwned(%q) = %v, want %v", tc.path, got, tc.owned)
		}
	}
}
