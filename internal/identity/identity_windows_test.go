//go:build windows

package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSave_WritesAtomicallyWithPermissions is the Windows version of the
// permissions test. On Windows, os.Chmod(0o600) only toggles the READONLY
// attribute — it does not set ACL-based access control. Measured in CI run
// 38902064559 (ML-W2a): Mode().Perm() returns 0o666 (not 0o600) on an NTFS
// volume. ADR-2026-10-16-no-windows-o-validate-avisa-... decision D1 declares
// this a known limit and keeps 0o600 as the intent for POSIX.
//
// This test asserts the Windows-measured semantics:
//   - the identity file exists after Save,
//   - the file content is valid JSON ending with a newline,
//   - no leftover temp files remain,
//   - Mode().Perm() == 0o666 (the only non-READONLY value Go synthesises on NTFS) — D1.
//
// Reconciliation (Regra de Reconciliação, CLAUDE.md):
//   - The Mode().Perm() == 0o666 assertion locks the measured Windows semantics.
//     Go synthesises Mode() from FILE_ATTRIBUTE_READONLY only (os/types_windows.go),
//     returning 0o444 or 0o666 regardless of NTFS ACLs. The assertion tightens the
//     claim to the exact measured value so that any future change to Go's NTFS
//     Mode() implementation — e.g. an ACL-aware path that could return 0o600 — causes
//     a failure worth investigating. The proof that os.Chmod(0o600) is inert on NTFS
//     is probe P17/icacls, not this test (D1 of ADR-2026-10-16).
//   - The JSON + newline + no-temp assertions affirm "Save is atomic and produces
//     valid output" — they would fail if the write was truncated or if the temp
//     rename cleanup was broken.
func TestSave_WritesAtomicallyWithPermissions(t *testing.T) {
	homeDir := t.TempDir()
	cfg := Config{Agents: map[string]AgentIdentity{}}

	if err := Save(homeDir, cfg); err != nil {
		t.Fatalf("Save falhou: %v", err)
	}

	path := identityPath(homeDir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("arquivo de identidade deveria existir: %v", err)
	}

	// D1 (ADR-2026-10-16-no-windows-o-validate-avisa-...): os.Chmod(0o600) is
	// inert on NTFS. Mode().Perm() is expected to be exactly 0o666 (measured in CI
	// run 38902064559). Go synthesises Mode() from FILE_ATTRIBUTE_READONLY only
	// (os/types_windows.go), so only 0o444 and 0o666 are possible.
	// Assert the measured value; the POSIX 0o600 assertion lives in
	// identity_notwindows_test.go.
	perm := info.Mode().Perm()
	t.Logf("PROBE w2b identity perm=%04o (Windows NTFS; expected 0o666 per D1)", perm)
	if perm != 0o666 {
		t.Errorf("Mode().Perm() = 0o%o on Windows — expected 0o666 (Go synthesises from "+
			"FILE_ATTRIBUTE_READONLY only; NTFS ACLs are not reflected in Mode()). "+
			"If this is 0o444, the file was marked read-only. "+
			"If it is 0o600, a new ACL-aware os.Chmod may have been introduced — update test and ADR.",
			perm)
	}

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
