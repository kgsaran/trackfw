package serve

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParseLog_Empty — diretório inexistente retorna slice vazia (não nil, não panic).
// AFIRMA: ParseLog não retorna nil para um diretório inexistente — callers podem iterar
// sobre o resultado sem verificar nil.
func TestParseLog_Empty(t *testing.T) {
	// ML-1A: ParseLog now takes a directory, not a file path.
	result, err := ParseLog("/caminho/inexistente/roadmaps")
	if err != nil {
		t.Fatalf("esperado err=nil para diretório inexistente, obteve: %v", err)
	}
	if result == nil {
		t.Fatal("esperado slice vazia, obteve nil")
	}
	if len(result) != 0 {
		t.Errorf("esperado 0 transições, obteve %d", len(result))
	}
}

// TestParseLog_ValidLines — arquivo legado com linhas válidas retorna transições corretas.
func TestParseLog_ValidLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".trackfw-log")

	content := "2026-06-12 14:30  ROADMAP-feature-auth.md     backlog → wip\n" +
		"2026-06-13 09:00  ROADMAP-feature-login.md    wip → done\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// ML-1A: ParseLog takes the directory, not the file path; the legacy file is read automatically.
	result, err := ParseLog(dir)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("esperado 2 transições, obteve %d", len(result))
	}

	// Primeira transição.
	// ML-1A: translog.ReadAll parses timestamps in time.Local (matching metrics/validator)
	// rather than UTC as the old serve.ParseLog did. Update expectations accordingly.
	first := result[0]
	wantTS1, _ := time.ParseInLocation("2006-01-02 15:04", "2026-06-12 14:30", time.Local)
	if !first.Timestamp.Equal(wantTS1) {
		t.Errorf("Timestamp[0]: esperado %v, obteve %v", wantTS1, first.Timestamp)
	}
	if first.Basename != "ROADMAP-feature-auth.md" {
		t.Errorf("Basename[0]: esperado %q, obteve %q", "ROADMAP-feature-auth.md", first.Basename)
	}
	if first.From != "backlog" {
		t.Errorf("From[0]: esperado %q, obteve %q", "backlog", first.From)
	}
	if first.To != "wip" {
		t.Errorf("To[0]: esperado %q, obteve %q", "wip", first.To)
	}

	// Segunda transição.
	second := result[1]
	wantTS2, _ := time.ParseInLocation("2006-01-02 15:04", "2026-06-13 09:00", time.Local)
	if !second.Timestamp.Equal(wantTS2) {
		t.Errorf("Timestamp[1]: esperado %v, obteve %v", wantTS2, second.Timestamp)
	}
	if second.Basename != "ROADMAP-feature-login.md" {
		t.Errorf("Basename[1]: esperado %q, obteve %q", "ROADMAP-feature-login.md", second.Basename)
	}
	if second.From != "wip" {
		t.Errorf("From[1]: esperado %q, obteve %q", "wip", second.From)
	}
	if second.To != "done" {
		t.Errorf("To[1]: esperado %q, obteve %q", "done", second.To)
	}
}

// TestParseLog_SkipsInvalidLines — linhas mal-formatadas são ignoradas silenciosamente.
func TestParseLog_SkipsInvalidLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".trackfw-log")

	content := "linha invalida sem formato correto\n" +
		"2026-06-12 14:30  ROADMAP-valido.md     backlog → wip\n" +
		"outra linha invalida\n" +
		"data-errada 99:99  ROADMAP-x.md  backlog → wip\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// ML-1A: ParseLog takes the directory; the legacy file is read automatically.
	result, err := ParseLog(dir)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	// Apenas a linha válida deve ser retornada.
	if len(result) != 1 {
		t.Errorf("esperado 1 transição válida, obteve %d: %+v", len(result), result)
	}
	if len(result) > 0 && result[0].Basename != "ROADMAP-valido.md" {
		t.Errorf("Basename inesperado: %q", result[0].Basename)
	}
}

// TestParseLog_MultipleTransitions — múltiplas transições do mesmo roadmap são todas capturadas.
func TestParseLog_MultipleTransitions(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".trackfw-log")

	content := "2026-06-10 10:00  ROADMAP-X.md  backlog → wip\n" +
		"2026-06-11 11:00  ROADMAP-X.md  wip → blocked\n" +
		"2026-06-12 12:00  ROADMAP-X.md  blocked → wip\n" +
		"2026-06-13 13:00  ROADMAP-X.md  wip → done\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// ML-1A: ParseLog takes the directory.
	result, err := ParseLog(dir)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("esperado 4 transições, obteve %d", len(result))
	}

	expectedTransitions := []struct{ from, to string }{
		{"backlog", "wip"},
		{"wip", "blocked"},
		{"blocked", "wip"},
		{"wip", "done"},
	}
	for i, exp := range expectedTransitions {
		if result[i].Basename != "ROADMAP-X.md" {
			t.Errorf("[%d] Basename: esperado %q, obteve %q", i, "ROADMAP-X.md", result[i].Basename)
		}
		if result[i].From != exp.from {
			t.Errorf("[%d] From: esperado %q, obteve %q", i, exp.from, result[i].From)
		}
		if result[i].To != exp.to {
			t.Errorf("[%d] To: esperado %q, obteve %q", i, exp.to, result[i].To)
		}
	}
}
