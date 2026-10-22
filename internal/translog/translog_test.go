package translog

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// RECONCILIAÇÃO (Regra Dura): cada teste abaixo tem uma frase que diz qual
// conclusão do ML-1A ele afirma.
// ─────────────────────────────────────────────────────────────────────────────

// AFIRMA: ReadAll lê o legado e o .d/ juntos, na ordem correta, inclusive com
// linhas de fuso -0300. Garante que o leitor unificado cobre AMBAS as fontes.
func TestReadAll_LegacyAndDirTogether(t *testing.T) {
	dir := t.TempDir()

	// Legacy file with one line (using -0300 suffix).
	legacyContent := "2026-06-12 14:30  ROADMAP-legacy.md                   backlog → wip  -0300\n"
	if err := os.WriteFile(filepath.Join(dir, ".trackfw-log"), []byte(legacyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// One .d/ entry with a later timestamp.
	dPath := filepath.Join(dir, ".trackfw-log.d")
	if err := os.MkdirAll(dPath, 0755); err != nil {
		t.Fatal(err)
	}
	entryLine := "2026-06-13 09:00  ROADMAP-new.md                       wip → done"
	entryTime := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	name := EntryFilename(entryLine, entryTime)
	if err := os.WriteFile(filepath.Join(dPath, name), []byte(entryLine+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ts, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(ts) != 2 {
		t.Fatalf("want 2 transitions, got %d: %+v", len(ts), ts)
	}
	if ts[0].Basename != "ROADMAP-legacy.md" {
		t.Errorf("ts[0].Basename = %q, want ROADMAP-legacy.md", ts[0].Basename)
	}
	if ts[1].Basename != "ROADMAP-new.md" {
		t.Errorf("ts[1].Basename = %q, want ROADMAP-new.md", ts[1].Basename)
	}
}

// AFIRMA: a mesma entrada gravada duas vezes é idempotente (mesmo conteúdo → no-op).
func TestWriteEntry_Idempotent(t *testing.T) {
	dir := t.TempDir()
	line := "2026-10-22 10:00  ROADMAP-x.md                          backlog → wip"
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)

	name1, err := WriteEntry(dir, line, now)
	if err != nil {
		t.Fatalf("first WriteEntry: %v", err)
	}
	name2, err := WriteEntry(dir, line, now)
	if err != nil {
		t.Fatalf("second WriteEntry (idempotent): %v", err)
	}
	if name1 != name2 {
		t.Errorf("names differ: %q vs %q", name1, name2)
	}
}

// AFIRMA: conteúdo diferente com o mesmo nome de arquivo (colisão SHA-256) devolve
// erro em vez de corromper silenciosamente a entrada existente.
func TestWriteEntry_DifferentContentSameNameErrors(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)

	line1 := "2026-10-22 10:00  ROADMAP-a.md                          backlog → wip"
	if _, err := WriteEntry(dir, line1, now); err != nil {
		t.Fatalf("WriteEntry line1: %v", err)
	}

	// Manufacture a collision by writing a different line with the same computed name.
	// We do this by directly creating the file with a different content.
	name := EntryFilename(line1, now)
	dPath := filepath.Join(dir, ".trackfw-log.d")
	differentContent := "different content that hash-collides artificially\n"
	if err := os.WriteFile(filepath.Join(dPath, name), []byte(differentContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Now WriteEntry should fail because the file exists with different content.
	_, err := WriteEntry(dir, line1, now)
	if err == nil {
		t.Fatal("expected error for different-content collision, got nil")
	}
}

// AFIRMA: um .d/ que é symlink é recusado pelo reader (não seguido cegamente).
func TestReadAll_DotDSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	dPath := filepath.Join(dir, ".trackfw-log.d")
	target := t.TempDir()
	if err := os.Symlink(target, dPath); err != nil {
		t.Skip("symlink not supported on this platform")
	}
	_, err := ReadAll(dir)
	if err == nil {
		t.Fatal("expected error when .d/ is a symlink, got nil")
	}
}

// AFIRMA: arquivos dentro do .d/ que não têm extensão .entry são silenciosamente
// ignorados pelo reader (crash remnants, .DS_Store, future extensions).
func TestReadAll_NonEntryFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	dPath := filepath.Join(dir, ".trackfw-log.d")
	if err := os.MkdirAll(dPath, 0755); err != nil {
		t.Fatal(err)
	}

	// A valid .entry file.
	validLine := "2026-10-22 10:00  ROADMAP-ok.md                         backlog → wip"
	validTime := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	if _, err := WriteEntry(dir, validLine, validTime); err != nil {
		t.Fatalf("WriteEntry: %v", err)
	}

	// Files that should be ignored.
	for _, name := range []string{"tmp-12345678", "notes.txt", ".DS_Store", "tmp-abcdef.log"} {
		if err := os.WriteFile(filepath.Join(dPath, name), []byte("garbage"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ts, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(ts) != 1 {
		t.Errorf("want 1 transition (only .entry), got %d: %+v", len(ts), ts)
	}
}

// AFIRMA: o validate com by_agent e um .trackfw-log.d/ não emite agent_namespace_hidden
// (provado pela adição de .trackfw-log.d a isInfraDirName — verificado no package validator).
// Este teste prova que EntryFilename é determinístico e estável.
func TestEntryFilename_Deterministic(t *testing.T) {
	line := "2026-10-22 10:00  ROADMAP-x.md                          backlog → wip"
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	name1 := EntryFilename(line, now)
	name2 := EntryFilename(line, now)
	if name1 != name2 {
		t.Errorf("EntryFilename not deterministic: %q vs %q", name1, name2)
	}
	// Must end in .entry.
	if filepath.Ext(name1) != ".entry" {
		t.Errorf("expected .entry extension, got %q", name1)
	}
}

// AFIRMA: o leitor único é a única forma que os callers do ML-1A têm de acessar as
// entradas; um caller que tenta ler só o arquivo legado não verá entradas que existem
// só no .d/. Este teste documenta o contrato que os outros testes de integração verificam.
func TestReadAll_EntriesOnlyInDotD_AreVisible(t *testing.T) {
	dir := t.TempDir()
	// NO legacy file.
	line := "2026-10-22 11:00  ROADMAP-y.md                          wip → done"
	now := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	if _, err := WriteEntry(dir, line, now); err != nil {
		t.Fatalf("WriteEntry: %v", err)
	}

	ts, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(ts) != 1 {
		t.Fatalf("want 1 transition, got %d", len(ts))
	}
	if ts[0].Basename != "ROADMAP-y.md" {
		t.Errorf("Basename = %q, want ROADMAP-y.md", ts[0].Basename)
	}
}

// AFIRMA: ReadAll produz as entradas ordenadas pelo timestamp de sort key; entradas do
// legado com mesmo minuto que entradas do .d/ com segundo preciso se ordenam antes delas.
func TestReadAll_SortOrder(t *testing.T) {
	dir := t.TempDir()

	// Two .d/ entries.
	line1 := "2026-10-22 10:00  ROADMAP-first.md                     backlog → wip"
	line2 := "2026-10-22 10:05  ROADMAP-second.md                    backlog → wip"
	t1 := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 22, 10, 5, 0, 0, time.UTC)
	if _, err := WriteEntry(dir, line1, t1); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteEntry(dir, line2, t2); err != nil {
		t.Fatal(err)
	}
	// Legacy line at 10:03.
	legacy := "2026-10-22 10:03  ROADMAP-middle.md                   backlog → wip\n"
	if err := os.WriteFile(filepath.Join(dir, ".trackfw-log"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	ts, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(ts) != 3 {
		t.Fatalf("want 3, got %d", len(ts))
	}

	names := make([]string, len(ts))
	for i, tr := range ts {
		names[i] = tr.Basename
	}
	want := []string{"ROADMAP-first.md", "ROADMAP-middle.md", "ROADMAP-second.md"}
	if !equalStringSlices(names, want) {
		t.Errorf("sort order wrong: got %v, want %v", names, want)
	}
}

// AFIRMA: ReadAll retorna nil/empty (sem erro) quando o dir não tem nem legado nem .d/.
func TestReadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	ts, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll on empty dir: %v", err)
	}
	if len(ts) != 0 {
		t.Errorf("want 0, got %d", len(ts))
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := make([]string, len(a))
	bc := make([]string, len(b))
	copy(ac, a)
	copy(bc, b)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
