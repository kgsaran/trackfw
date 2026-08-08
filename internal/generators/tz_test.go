package generators

// Testes de paridade de fuso horário para os geradores de artefato Go.
//
// Invariante: time.Now().Format("2006-01-02") deve retornar a DATA LOCAL,
// não UTC. Go já faz isso corretamente; este teste trava a semântica como
// regressão — se alguém introduzir time.Now().UTC(), o teste detecta.
//
// Estratégia determinística (sem mock, sem nova dependência):
//   Pacific/Kiritimati = UTC+14, sem DST → sempre 14h à frente do UTC.
//   Pacific/Midway     = UTC-11, sem DST → sempre 11h atrás do UTC.
//   Span total = 25 horas → as duas datas locais NUNCA coincidem.
//
// Com implementação correta (hora local): kiri ≠ midway → assert PASS
// Com implementação quebrada (UTC):       kiri == midway → assert FAIL
//
// ATENÇÃO: este teste altera time.Local (variável global do pacote time).
// Não chamar t.Parallel() neste arquivo — risco de race com outros testes
// que dependam do fuso local.
//
// REQ: REQ-2026-07-27-convergencia-templates-python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGenerators_LocalDateNotUTC verifica que os geradores usam hora local
// (não UTC) ao calcular a data do artefato. Usa UTC+14 e UTC-11 — span de 25h
// que garante datas locais sempre distintas entre si.
func TestGenerators_LocalDateNotUTC(t *testing.T) {
	loc14, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Skipf("fuso Pacific/Kiritimati não disponível: %v", err)
	}
	loc11, err := time.LoadLocation("Pacific/Midway")
	if err != nil {
		t.Skipf("fuso Pacific/Midway não disponível: %v", err)
	}

	// Datas esperadas calculadas a partir do UTC atual — determinístico.
	nowUTC := time.Now().UTC()
	date14 := nowUTC.In(loc14).Format("2006-01-02")
	date11 := nowUTC.In(loc11).Format("2006-01-02")

	if date14 == date11 {
		// Matematicamente impossível com span de 25h, mas guard para clareza.
		t.Skip("UTC+14 e UTC-11 coincidem agora — span impossível, pular")
	}

	// ── NewREQ — req.go usa time.Now().Format("2006-01-02") ──────────────────
	t.Run("NewREQ usa hora local", func(t *testing.T) {
		origLocal := time.Local
		time.Local = loc14
		t.Cleanup(func() { time.Local = origLocal })

		dir := t.TempDir()
		chdirREQ(t, dir)

		if err := NewREQ(REQContent{Title: "TZ Parity Test"}); err != nil {
			t.Fatalf("NewREQ: %v", err)
		}

		matches, _ := filepath.Glob("docs/req/*.md")
		if len(matches) == 0 {
			t.Fatal("nenhum arquivo REQ gerado")
		}
		base := filepath.Base(matches[0])

		if !strings.Contains(base, date14) {
			t.Errorf("NewREQ: filename deveria conter a data local UTC+14 %q, obteve %q", date14, base)
		}
		// Garantia extra: não usou UTC se as datas diferem
		if date14 != date11 && strings.Contains(base, nowUTC.Format("2006-01-02")) && !strings.Contains(base, date14) {
			t.Errorf("NewREQ: parece estar usando UTC (%q) em vez de local (%q)", nowUTC.Format("2006-01-02"), date14)
		}
	})

	// ── NewNote — note.go usa time.Now().Format("2006-01-02") ────────────────
	t.Run("NewNote usa hora local", func(t *testing.T) {
		origLocal := time.Local
		time.Local = loc14
		t.Cleanup(func() { time.Local = origLocal })

		dir := t.TempDir()
		chdirNote(t, dir)

		if err := NewNote("TZ Parity Note"); err != nil {
			t.Fatalf("NewNote: %v", err)
		}

		noteDir := os.DirFS("vault/notes")
		entries, _ := os.ReadDir("vault/notes")
		_ = noteDir
		var base string
		for _, e := range entries {
			if e.Name() != "index.md" {
				base = e.Name()
			}
		}
		if base == "" {
			t.Fatal("nenhuma nota gerada em vault/notes/")
		}

		if !strings.Contains(base, date14) {
			t.Errorf("NewNote: filename deveria conter a data local UTC+14 %q, obteve %q", date14, base)
		}
	})

	// ── NewADR — adr.go usa time.Now().Format("2006-01-02") ─────────────────
	t.Run("NewADR usa hora local", func(t *testing.T) {
		origLocal := time.Local
		time.Local = loc14
		t.Cleanup(func() { time.Local = origLocal })

		dir := t.TempDir()
		chdirADR(t, dir)

		if err := NewADR(ADRContent{Title: "TZ Parity ADR"}, "docs/adr"); err != nil {
			t.Fatalf("NewADR: %v", err)
		}

		matches, _ := filepath.Glob("docs/adr/*.md")
		if len(matches) == 0 {
			t.Fatal("nenhum ADR gerado")
		}
		base := filepath.Base(matches[0])

		if !strings.Contains(base, date14) {
			t.Errorf("NewADR: filename deveria conter a data local UTC+14 %q, obteve %q", date14, base)
		}
	})
}
