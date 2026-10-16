//go:build !windows

package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSave_WritesAtomicallyWithPermissions asserts that Save writes the identity
// file atomically (no leftover temp files) and with mode 0600/0700 on POSIX.
// On Windows the POSIX mode assertion differs in identity_windows_test.go:
// Mode().Perm() reflects only the READONLY attribute on Windows and is not a
// valid proxy for ACL-based access control (D1, ADR-2026-10-16-no-windows-o-validate-avisa-...).
//
// Reconciliation: this test affirms "Save produces a file with mode 0600 and a
// directory with mode 0700 on POSIX". It would fail if GuardedWrite stopped
// calling os.Chmod(0o600) on the file before renaming it.
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
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissao do arquivo = %o, esperava 0600", info.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Join(homeDir, ".trackfw"))
	if err != nil {
		t.Fatalf("diretorio .trackfw deveria existir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("permissao do diretorio = %o, esperava 0700", dirInfo.Mode().Perm())
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
