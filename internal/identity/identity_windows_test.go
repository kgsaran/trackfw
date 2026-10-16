//go:build windows

package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestSave_WritesAtomicallyWithPermissions is the Windows version of the
// permissions test. Instead of asserting Mode().Perm() == 0o600 (which reflects
// only the READONLY attribute on Windows and is not a valid proxy for DACL-based
// access control), this version asserts that:
//   - the identity file has no write ACE for Authenticated Users, BUILTIN\Users
//     or Everyone (per ADR D2), and
//   - the DACL is protected (SE_DACL_PROTECTED, per ADR R-4), and
//   - the DACL includes an owner full-control ACE (so the process doesn't lock
//     itself out), and
//   - no leftover temp files remain (unchanged from the POSIX version), and
//   - the file content is valid JSON ending with a newline.
//
// Reconciliation: this test affirms "Save produces a file whose DACL has no
// write ACE for Authenticated Users/Users/Everyone on Windows, and the DACL is
// protected". It would fail if fsperm.CreateRestrictedFile were replaced with
// os.CreateTemp (which inherits the parent directory's permissive DACL).
func TestSave_WritesAtomicallyWithPermissions(t *testing.T) {
	homeDir := t.TempDir()
	cfg := Config{Agents: map[string]AgentIdentity{}}

	if err := Save(homeDir, cfg); err != nil {
		t.Fatalf("Save falhou: %v", err)
	}

	path := identityPath(homeDir)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("arquivo de identidade deveria existir: %v", err)
	}

	// DACL assertions: no foreign write ACEs; DACL protected; owner has control.
	assertNoForeignWriteACE(t, path, "S-1-5-11")  // Authenticated Users
	assertNoForeignWriteACE(t, path, "S-1-5-32-545") // BUILTIN\Users
	assertNoForeignWriteACE(t, path, "S-1-1-0")   // Everyone
	assertDACLIsProtected(t, path)
	assertOwnerHasWriteACE(t, path)

	// No leftover temp files should remain in the directory.
	entries, err := os.ReadDir(filepath.Join(homeDir, ".trackfw"))
	if err != nil {
		t.Fatalf("falha ao listar diretorio: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".trackfw-tmp-") {
			t.Fatalf("arquivo temporario nao foi limpo: %s", entry.Name())
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("falha ao ler arquivo salvo: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("arquivo salvo deveria terminar com newline")
	}
	var roundTrip map[string]json.RawMessage
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("arquivo salvo deveria ser JSON valido: %v", err)
	}
}

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

func assertNoForeignWriteACE(t *testing.T, path, sidStr string) {
	t.Helper()
	sid, err := windows.StringToSid(sidStr)
	if err != nil {
		t.Fatalf("StringToSid(%q): %v", sidStr, err)
	}
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
		t.Errorf("%q has NULL DACL (everyone full control) — unexpected", path)
		return
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
			t.Errorf("%q has write ACE for %s (mask 0x%x) — DACL not restricted", path, sidStr, ace.Mask)
		}
	}
}

func assertDACLIsProtected(t *testing.T, path string) {
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
	if ctrl&windows.SE_DACL_PROTECTED == 0 {
		t.Errorf("%q DACL is not protected (SE_DACL_PROTECTED not set, control=0x%x)", path, ctrl)
	}
}

func assertOwnerHasWriteACE(t *testing.T, path string) {
	t.Helper()
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("OpenCurrentProcessToken: %v", err)
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser: %v", err)
	}
	ownerSID := tu.User.Sid

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
		return // NULL DACL means full control for everyone
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
		if aceSID.Equals(ownerSID) && ace.Mask&maskWriteAny != 0 {
			return // owner has write access — OK
		}
	}
	t.Errorf("%q has no write ACE for the process owner — process may lose access to its own file", path)
}
