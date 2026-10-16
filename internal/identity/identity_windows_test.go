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
//   - Mode().Perm() is NOT 0o600 (it is 0o666 on a standard NTFS runner) — D1.
//
// Reconciliation (Regra de Reconciliação, CLAUDE.md):
//   - The Mode().Perm() != 0o600 assertion affirms "os.Chmod(0o600) is inert on
//     Windows NTFS" (D1 of ADR-2026-10-16). It would fail if a future change
//     made Go's os.Chmod write an ACL-based 0o600 on Windows, which would be a
//     surprise worth catching.
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
	// inert on NTFS. Mode().Perm() is expected to be 0o666 (measured in CI run
	// 38902064559). Assert the Windows-measured value; the POSIX 0o600 assertion
	// lives in identity_notwindows_test.go.
	perm := info.Mode().Perm()
	t.Logf("PROBE w2b identity perm=%04o (Windows NTFS; expected 0o666 per D1)", perm)
	if perm == 0o600 {
		t.Errorf("Mode().Perm() = 0o600 on Windows — unexpected: os.Chmod(0o600) should be inert on NTFS (D1 ADR-2026-10-16); this may indicate a new ACL-aware os.Chmod; update the test and the ADR")
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
