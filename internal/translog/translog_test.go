package translog

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
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
	dPath := filepath.Join(dir, EntriesDirSuffix[1:])
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

	name1, err := WriteEntry(dir, dir, line, now)
	if err != nil {
		t.Fatalf("first WriteEntry: %v", err)
	}
	name2, err := WriteEntry(dir, dir, line, now)
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
	if _, err := WriteEntry(dir, dir, line1, now); err != nil {
		t.Fatalf("WriteEntry line1: %v", err)
	}

	// Manufacture a collision by writing a different line with the same computed name.
	// We do this by directly creating the file with a different content.
	name := EntryFilename(line1, now)
	dPath := filepath.Join(dir, EntriesDirSuffix[1:])
	differentContent := "different content that hash-collides artificially\n"
	if err := os.WriteFile(filepath.Join(dPath, name), []byte(differentContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Now WriteEntry should fail because the file exists with different content.
	_, err := WriteEntry(dir, dir, line1, now)
	if err == nil {
		t.Fatal("expected error for different-content collision, got nil")
	}
}

// AFIRMA: um entries/ que é symlink é recusado pelo reader (não seguido cegamente).
func TestReadAll_DotDSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	dPath := filepath.Join(dir, EntriesDirSuffix[1:])
	target := t.TempDir()
	if !symlinkOrSkipTranslog(t, target, dPath) {
		return
	}
	_, err := ReadAll(dir)
	if err == nil {
		t.Fatal("expected error when entries/ is a symlink, got nil")
	}
}

// AFIRMA: arquivos dentro do .d/ que não têm extensão .entry são silenciosamente
// ignorados pelo reader (crash remnants, .DS_Store, future extensions).
func TestReadAll_NonEntryFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	dPath := filepath.Join(dir, EntriesDirSuffix[1:])
	if err := os.MkdirAll(dPath, 0755); err != nil {
		t.Fatal(err)
	}

	// A valid .entry file.
	validLine := "2026-10-22 10:00  ROADMAP-ok.md                         backlog → wip"
	validTime := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	if _, err := WriteEntry(dir, dir, validLine, validTime); err != nil {
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

// AFIRMA: o validate com by_agent e um .trackfw-log-entries/ não emite agent_namespace_hidden
// (provado pela adição de .trackfw-log-entries a isInfraDirName — verificado no package validator).
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
	if _, err := WriteEntry(dir, dir, line, now); err != nil {
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
	if _, err := WriteEntry(dir, dir, line1, t1); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteEntry(dir, dir, line2, t2); err != nil {
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

// AFIRMA: um arquivo escrito por WriteEntry em `.trackfw-log-entries/<name>.entry`
// NÃO é ignorado por padrões comuns de .gitignore (*.d, *.log, *.tmp, *.bak, *.swp),
// enquanto o nome antigo `.trackfw-log.d/x.entry` É ignorado pelo padrão *.d.
// Prova que o rename (ML-1C) corrige o problema real que motivou a renomeação.
func TestEntriesDirNotIgnoredByCommonGitignorePatterns(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repoDir := t.TempDir()

	// Initialise a git repo.
	mustRunGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "core.excludesFile="}, args...)...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustRunGit("init")
	mustRunGit("config", "user.email", "test@example.com")
	mustRunGit("config", "user.name", "Test")

	// Write a .gitignore with common patterns that historically silenced .d/ dirs.
	gitignore := "*.d\n*.log\n*.tmp\n*.bak\n*.swp\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	// ── positive arm: WriteEntry creates .trackfw-log-entries/<name>.entry ─
	// The literal ".trackfw-log-entries" here is the on-disk contract pin.
	// Changing EntriesDirSuffix without updating this test makes it red.
	line := "2026-10-22 10:00  ROADMAP-x.md                          backlog → wip"
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	entryName, writeErr := WriteEntry(repoDir, repoDir, line, now)
	if writeErr != nil {
		t.Fatalf("WriteEntry: %v", writeErr)
	}
	newEntryRel := filepath.Join(".trackfw-log-entries", entryName)

	checkIgnoreNew := exec.Command("git", "-c", "core.excludesFile=", "check-ignore", "-v", newEntryRel)
	checkIgnoreNew.Dir = repoDir
	if err := checkIgnoreNew.Run(); err == nil {
		// exit 0 means the file IS ignored — the rename did not fix the problem.
		t.Errorf("git check-ignore exit 0: .trackfw-log-entries/%s is ignored by .gitignore", entryName)
	} else {
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 1 {
			t.Errorf("git check-ignore returned unexpected non-zero %v (want exit 1 = not-ignored)", err)
		}
	}

	// ── contrario arm: old name (.trackfw-log.d/) IS ignored by *.d ────────
	// This arm proves the test bites: the old name would have silenced entries.
	oldDir := filepath.Join(repoDir, ".trackfw-log.d")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("mkdir old .d/: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "x.entry"),
		[]byte(line+"\n"), 0644); err != nil {
		t.Fatalf("write old entry: %v", err)
	}

	checkIgnoreOld := exec.Command("git", "-c", "core.excludesFile=", "check-ignore", "-v", ".trackfw-log.d/x.entry")
	checkIgnoreOld.Dir = repoDir
	if err := checkIgnoreOld.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			// exit 1 means NOT ignored — the contrario arm does not bite.
			t.Errorf("git check-ignore exit 1: .trackfw-log.d/x.entry is NOT ignored by *.d — contrario arm does not bite, test is inconclusive")
		} else {
			t.Errorf("git check-ignore unexpected error for old path: %v", err)
		}
	}
}

// symlinkOrSkipTranslog creates a symlink at link pointing to target.
// If creation fails due to missing privilege (Windows without Developer Mode
// or elevated process — WinError 1314, or permission denied), it skips the
// test. Any other error is a t.Fatalf.
//
// The check is by CONDITION (privilege failure), not by runtime.GOOS.
func symlinkOrSkipTranslog(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeErrorTranslog(err) {
		t.Skipf("symlink guard not exercised: creating symlink requires Developer Mode "+
			"(or elevated process) on this Windows: %v", err)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeErrorTranslog reports whether err is a privilege-missing
// symlink creation failure. Checks by error value, not by GOOS.
func isSymlinkPrivilegeErrorTranslog(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	if errors.As(err, &e) && e == 1314 {
		return true
	}
	return false
}

// AFIRMA: parseLine usa o offset da linha (ex: "-0300") para construir um
// time.FixedZone, de modo que a ordenação de entradas legadas contra arquivos
// .entry com timestamp UTC é correta independentemente do TZ da máquina leitora.
// Mordida: sem a correção, a linha "12:00 -0300" (= 15:00 UTC) é lida como 12:00
// UTC (antes de 14:00 UTC da entry) quando TZ=UTC — a entry aparece depois, errado.
func TestParseLine_HonorsLineOffsetUnderUTC(t *testing.T) {
	// Force time.Local to UTC so that lines without offset are parsed as UTC.
	// This is the setting that exposed the bug: with TZ=UTC, "12:00 -0300"
	// was previously parsed as 12:00 UTC instead of 15:00 UTC.
	origLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = origLocal })

	dir := t.TempDir()

	// Legacy line: "12:00 -0300" = 15:00 UTC.
	legacy := "2026-10-22 12:00  ROADMAP-legacy.md                   backlog → wip  -0300\n"
	if err := os.WriteFile(filepath.Join(dir, ".trackfw-log"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	// Entry file with timestamp 14:00 UTC — must sort BEFORE the legacy line (15:00 UTC).
	entryTime := time.Date(2026, 10, 22, 14, 0, 0, 0, time.UTC)
	entryLine := "2026-10-22 09:00  ROADMAP-entry.md                    backlog → wip"
	if _, err := WriteEntry(dir, dir, entryLine, entryTime); err != nil {
		t.Fatalf("WriteEntry: %v", err)
	}

	all, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(all))
	}

	// entry (14:00 UTC) must come before legacy (15:00 UTC).
	if all[0].Basename != "ROADMAP-entry.md" {
		t.Errorf("ORDER BROKEN under UTC: expected ROADMAP-entry.md first (14:00Z before 15:00Z), got %s first\n"+
			"  all[0]=%+v\n  all[1]=%+v",
			all[0].Basename, all[0], all[1])
	}

	// Confirm that the legacy Timestamp is 15:00 UTC (not 12:00 UTC).
	wantLegacyUTC := time.Date(2026, 10, 22, 15, 0, 0, 0, time.UTC)
	var legacyT *Transition
	for i := range all {
		if all[i].Basename == "ROADMAP-legacy.md" {
			legacyT = &all[i]
		}
	}
	if legacyT == nil {
		t.Fatal("ROADMAP-legacy.md not found in result")
	}
	if !legacyT.Timestamp.Equal(wantLegacyUTC) {
		t.Errorf("legacy Timestamp: want %v (UTC), got %v (UTC)", wantLegacyUTC, legacyT.Timestamp.UTC())
	}
}

// AFIRMA: WriteEntry recusa escrever quando um ancestral do diretório de entradas
// é um symlink que sai da raiz — a guarda interna de pathguard.RejectAndReport
// domina os três sítios de escrita (MkdirAll, CreateTemp, Rename) na própria função.
// Braço de controle sem symlink comprova que o teste morde: sem a guarda, a escrita
// atravessaria o symlink em vez de ser recusada.
func TestWriteEntry_RefusesSymlinkAncestor(t *testing.T) {
	root := t.TempDir()    // the project root
	outside := t.TempDir() // a directory outside root, target of the symlink

	// Place root/sub as a symlink pointing outside root.
	// symlinkOrSkipTranslog skips on platforms that require elevated privilege.
	subPath := filepath.Join(root, "sub")
	if !symlinkOrSkipTranslog(t, outside, subPath) {
		return
	}

	line := "2026-10-22 10:00  ROADMAP-x.md                          backlog → wip"
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)

	// WriteEntry with dir="sub/logs" — the ancestor "sub" is a symlink, so the
	// guard must refuse before creating anything.
	_, err := WriteEntry(root, "sub/logs", line, now)
	if err == nil {
		t.Error("expected WriteEntry to refuse a path through a symlink ancestor, got nil")
	}

	// Verify nothing was written under outside (the symlink target).
	des, readErr := os.ReadDir(outside)
	if readErr != nil {
		t.Fatalf("ReadDir(outside): %v", readErr)
	}
	if len(des) != 0 {
		t.Errorf("WriteEntry created %d entry/entries under outside/; expected nothing: %v", len(des), des)
	}

	// Control arm: same root, a different dir with no symlink — must succeed.
	_, err = WriteEntry(root, "safe", line, now)
	if err != nil {
		t.Errorf("control arm (no symlink): unexpected error: %v", err)
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
