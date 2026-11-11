package i18nguard_test

// TestNoPTLiteralsInProductGo — AFIRMA que nenhum literal de string executável nos
// arquivos de produto de internal/ e cmd/ contém palavra portuguesa fora da lista de
// permissão fechada. A lista de permissão cobre exatamente: (1) leitores bilíngues de
// contrato (ADR D1/D5), (2) palavras-chave de probes.go para detecção de domínio PT,
// (3) a sonda de caminho "docs/requisições" em discover.go, (4) nomes próprios de
// presets (ADR D6) e (5) diagnósticos em inglês que nomeiam o marcador PT aceito
// (ADR D1/D5 — a mensagem está em EN, mas cita a forma PT que o leitor aceita).
//
// TestNoPTLiteralsInStaticWeb — AFIRMA que app.js e index.html não contêm texto PT
// fora de comentários de linha ou de bloco.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// ─── Portuguese detection ────────────────────────────────────────────────────

// ptAccentChars are the accented characters that mark a string as Portuguese.
const ptAccentChars = "ãõçáéíóúâêôàÃÕÇÁÉÍÓÚÂÊÔ"

// ptUnaccentedWords is a closed list of unaccented PT-only words measured
// directly from this repository (ML-3G, F5). Word-boundary patterns, case-insensitive.
// Every entry was validated against the full product tree; words removed because they
// also appear in legitimate English contexts are listed with reasons at the bottom.
var ptUnaccentedWords = []*regexp.Regexp{
	// ── Existing entries ──────────────────────────────────────────────────────
	regexp.MustCompile(`(?i)\babandona[do]*\b`),
	regexp.MustCompile(`(?i)\bcancela[do]*\b`),
	regexp.MustCompile(`\bCaducou\b`),
	regexp.MustCompile(`(?i)\bPendente\b`),
	regexp.MustCompile(`(?i)\bNenhum[ao]?\b`),
	// "Gates da wave" is the PT bilingual form of the roadmap gate header marker.
	// Detecting it ensures any new emission outside of the designated reader
	// sites is caught.
	regexp.MustCompile(`\bGates da wave\b`),
	// "Arquivos afetados" is the PT bilingual form of the files-affected label
	// read by barrier.go.
	regexp.MustCompile(`\bArquivos afetados\b`),

	// ── Added in ML-3G (F5) — measured from product tree before addition ──────
	// These words have no English homograph and do not appear in English contexts
	// in string literals. Words considered but discarded follow the list.
	regexp.MustCompile(`(?i)\baviso\b`),         // PT "warning" — not English
	regexp.MustCompile(`(?i)\bprosseguindo\b`),  // PT "proceeding" (gerund) — not English
	regexp.MustCompile(`(?i)\berro\b`),          // PT "error" — EN form is "error"; \berro\b never matches "error" or "erroneous"
	regexp.MustCompile(`(?i)\bfalh(a|ou)\b`),    // PT "failure/fail/failed" — not English
	regexp.MustCompile(`(?i)\binvalido\b`),      // PT "invalid" — EN form is "invalid"; "invalido" is unambiguously PT
	regexp.MustCompile(`(?i)\bvalidos\b`),       // PT plural "valid" — EN uses "valid"; "validos" is unambiguously PT
	regexp.MustCompile(`(?i)\barquivo\b`),       // PT "file" — not English
	regexp.MustCompile(`(?i)\bmalformado\b`),    // PT "malformed" — EN form is "malformed"; "malformado" is PT suffix
	regexp.MustCompile(`(?i)\bignorando\b`),     // PT "ignoring" (gerund) — not English
	regexp.MustCompile(`(?i)\bdiretorio\b`),     // PT "directory" — not English
	regexp.MustCompile(`(?i)\bencontrad[ao]\b`), // PT "found" (PP) — not English
	regexp.MustCompile(`(?i)\bcriado\b`),        // PT "created" (PP) — not English; "created" in EN is different
	regexp.MustCompile(`(?i)\bgravado\b`),       // PT "saved/written" (PP) — not English
	regexp.MustCompile(`(?i)\bpelo\b`),          // PT preposition "by the" — not English
	regexp.MustCompile(`(?i)\bpela\b`),          // PT preposition "by the" (f) — not English
	regexp.MustCompile(`(?i)\bdesta\b`),         // PT "of this" (f) — not English
	regexp.MustCompile(`(?i)\bdeste\b`),         // PT "of this" (m) — not English
	regexp.MustCompile(`(?i)\bquando\b`),        // PT "when" — not English
	regexp.MustCompile(`(?i)\bentao\b`),         // PT "then" — not English
	regexp.MustCompile(`(?i)\btambem\b`),        // PT "also" — not English
	// Discarded words (measured; kept causing false positives):
	//   "valor"    — colides with the English word "valor" (courage/merit)
	//   "nao"      — colides with common English abbreviations and identifiers
	//   "sao"      — colides with common English words ("sao" is used in many contexts)
	//   "voce"     — colides with possible identifiers/names
	//   "deve"     — colides with "deve" in English (past tense contexts are rare but possible)
	//   "apenas"   — colides with "apenas" appearing in English test data or identifiers
	//   "ainda"    — colides with identifiers
	//   "usando"   — colides with "using" false-positive with shared root; also "usando" appears in comments
	//   "porque"   — colides with identifiers and test data
	//   "existe"   — colides with identifiers
	//   "obrigatorio" — only appears with accented form "obrigatório"; unaccented form not measured
	//   "esperado" — colides with "expected" false-positive pattern
	//   "linha"    — colides with "line" and common identifiers ("linha" is used in EN code too)
}

func containsPT(s string) bool {
	for _, r := range s {
		if strings.ContainsRune(ptAccentChars, r) {
			return true
		}
	}
	for _, re := range ptUnaccentedWords {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// ─── Allowlist ───────────────────────────────────────────────────────────────

type ptAllowEntry struct {
	File  string // path relative to module root (forward slashes)
	Value string // exact unquoted string value
	Class string // 1=reader, 2=probe, 3=probe-path, 4=preset, 5=en-diag-naming-pt
	Why   string // reason
}

// allowedPTLiterals is the closed allowlist of legitimate PT string literals.
// Key invariant (enforced by TestAllowlistCoversAllPTLiterals): every entry
// must still match at least one literal in the product code; a stale entry
// fails the test.
var allowedPTLiterals = []ptAllowEntry{
	// ── Class 1: bilingual contract readers (ADR D1/D5) ──────────────────────
	{
		File:  "internal/config/config.go",
		Value: "## Critérios de Aceite",
		Class: "1",
		Why:   "AcceptanceMarkers bilingual default — reader of consumer document",
	},
	{
		File:  "internal/generators/roadmap.go",
		Value: "## critérios de aceite",
		Class: "1",
		Why:   "NewRoadmapFromREQ parser checks section heading (G3-12 reader)",
	},
	{
		File:  "internal/sync/sync.go",
		Value: "## Motivação",
		Class: "1",
		Why:   "extractMotivation reads '## Motivação' as bilingual alias for '## Motivation'",
	},
	{
		File:  "internal/validator/validator_req_done_criteria.go",
		Value: `(?i)^## (?:Acceptance Criteria|Crit[eé]rios de [Aa]ceite)`,
		Class: "1",
		Why:   "reqACHeadingRe — bilingual regex reading the REQ acceptance-criteria heading",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: `^\*\*(?:Gates da wave|Wave gates):\*\*`,
		Class: "1",
		Why:   "GatesHeaderRe — bilingual regex for the gates header (ADR D1)",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: `^\*\*(?:Acceptance criteria|Crit[eé]rios de aceite):\*\*`,
		Class: "1",
		Why:   "CriteriaHeaderRe — bilingual regex reading the acceptance-criteria heading in roadmap MLs",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: `^ {2,}(?:Caducou|Lapsed):\s*\S`,
		Class: "1",
		Why:   "LapsedContinuationRe — bilingual regex reading Caducou:/Lapsed: continuation",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: "abandonado",
		Class: "1",
		Why:   "terminatedVocabulary map entry — reader of PT termination marker",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: "cancelado",
		Class: "1",
		Why:   "StatusIsCancelled reader — PT alias for 'cancelled'",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: "Caducou:",
		Class: "1",
		Why:   "lapsedLineHasContent prefix reader (D3 bilingual lapsed marker)",
	},
	{
		File:  "internal/commands/barrier.go",
		Value: "**Arquivos afetados:**",
		Class: "1",
		Why:   "barrier reads '**Arquivos afetados:**' as bilingual alias for '**Files affected:**'",
	},
	// ── Class 2: domain-probe PT keywords (probes.go detector) ──────────────
	// Only the accented keywords appear here: the accent check detects them.
	// Unaccented probe keywords ("banco", "fila", "modelo", etc.) are not in
	// ptUnaccentedWords — they are not flagged, so they do not need allowlisting.
	{
		File:  "internal/generators/probes.go",
		Value: "autenticação",
		Class: "2",
		Why:   "auth domain probe keyword",
	},
	{
		File:  "internal/generators/probes.go",
		Value: "migração",
		Class: "2",
		Why:   "database domain probe keyword (PT: migration)",
	},
	{
		File:  "internal/generators/probes.go",
		Value: "persistência",
		Class: "2",
		Why:   "database domain probe keyword (PT: persistence)",
	},
	{
		File:  "internal/generators/probes.go",
		Value: "notificação",
		Class: "2",
		Why:   "messaging domain probe keyword (PT: notification)",
	},
	// ── Class 3: discovery probe path ────────────────────────────────────────
	{
		File:  "internal/discover/discover.go",
		Value: "docs/requisições",
		Class: "3",
		Why:   "REQ directory probe path for PT-named directories",
	},
	// ── Class 4: preset display names (ADR D6) ───────────────────────────────
	{
		File:  "internal/identity/preset.go",
		Value: "Ártemis",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Métis",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Dédalo",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Padmé",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Mônica",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Cascão",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Rá",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Ísis",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Hórus",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Anúbis",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/identity/preset.go",
		Value: "Osíris",
		Class: "4",
		Why:   "Preset display name (ADR D6)",
	},
	{
		File:  "internal/commands/identity_wizard.go",
		Value: "Turma da Mônica (Franjinha, Cebolinha, Mônica...)",
		Class: "4",
		Why:   "Preset selector option label — proper name (ADR D6)",
	},
	{
		File:  "internal/commands/identity_wizard.go",
		Value: "Egyptian pantheon (Thoth, Ísis, Anúbis...)",
		Class: "4",
		Why:   "Preset selector option label — proper name (ADR D6)",
	},
	// ── Class 5: English diagnostics that name the accepted PT marker ─────────
	// Deviation from the four-class constraint in the roadmap: these are English
	// messages that CITE the PT form as the accepted alternative. Stripping the
	// PT token from these messages would misstate the contract.
	{
		File:  "internal/commands/help.go",
		Value: `["## Acceptance Criteria", "## Critérios de Aceite"]`,
		Class: "5",
		Why:   "Help text documenting the acceptance_markers config key default value",
	},
	{
		File: "internal/commands/help.go",
		Value: "acceptance_markers:\n  - \"## Acceptance Criteria\"\n  - \"## Critérios de Aceite\"\n  - \"## AC\"",
		Class: "5",
		Why:   "Help text documenting the acceptance_markers config key example",
	},
	{
		File:  "internal/validator/validator_roadmap_gates.go",
		Value: "roadmap %q (wip) Wave 0 has no gate block; add '**Wave gates:**' or '**Gates da wave:**' followed by a ```bash fence with a real gate command (AC7, ADR-2026-09-18 decision 4)",
		Class: "5",
		Why:   "EN diagnostic that names both accepted gate-header forms (ADR D1)",
	},
	{
		File:  "internal/validator/validator_roadmap_gates.go",
		Value: "roadmap %q (wip) Wave 0 gate block was not found: '**Wave gates:**' or '**Gates da wave:**' is not followed by a ```bash fence before the next heading (AC7, ADR-2026-09-18 decision 4)",
		Class: "5",
		Why:   "EN diagnostic that names both accepted gate-header forms (ADR D1)",
	},
	{
		File:  "internal/roadmapdoc/roadmapdoc.go",
		Value: "near-miss gates header at line %d: the line looks like a gates header but does not match the accepted form exactly (leading space, list/blockquote marker, invisible character, homoglyph or case) — write '**Wave gates:**' or '**Gates da wave:**' at column 0 (D8, ADR-2026-11-08)",
		Class: "5",
		Why:   "EN near-miss error that names both accepted gate-header forms (ADR D1/D8)",
	},
	{
		File:  "internal/validator/validator_req_done_criteria.go",
		Value: "REQ %q has Status: Done but contains open acceptance criteria (- [ ] without a Lapsed: (or Caducou:) justification); mark as met or add a Lapsed: (or Caducou:) justification (req_done_open_criteria, D4 ADR-2026-10-04)",
		Class: "5",
		Why:   "EN diagnostic citing the bilingual lapsed marker; Caducou: is the PT alias still accepted by the reader",
	},
}

// ─── Allowlist index ─────────────────────────────────────────────────────────

func buildAllowSet() map[string]bool {
	m := make(map[string]bool, len(allowedPTLiterals))
	for _, e := range allowedPTLiterals {
		m[e.File+"\x00"+e.Value] = true
	}
	return m
}

// ─── Module root discovery ────────────────────────────────────────────────────

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

// ─── Go product file scanner ─────────────────────────────────────────────────

type violation struct {
	File  string
	Line  int
	Value string
}

func scanGoProductFiles(t *testing.T, root string) []violation {
	t.Helper()
	var violations []violation

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			// Skip vendor, testdata, internal/pathguard/testdata
			if base == "vendor" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Only scan internal/ and cmd/
		if !strings.HasPrefix(rel, "internal/") && !strings.HasPrefix(rel, "cmd/") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Errorf("%s: cannot parse: %v", rel, err)
			return nil
		}

		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}

			val := lit.Value
			var unquoted string
			if strings.HasPrefix(val, "`") {
				// Raw string literal: strip enclosing backticks
				unquoted = val[1 : len(val)-1]
			} else {
				// Interpreted string literal
				u, err := strconv.Unquote(val)
				if err != nil {
					return true
				}
				unquoted = u
			}

			if containsPT(unquoted) {
				violations = append(violations, violation{
					File:  rel,
					Line:  fset.Position(lit.Pos()).Line,
					Value: unquoted,
				})
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking source tree: %v", err)
	}
	return violations
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestNoPTLiteralsInProductGo(t *testing.T) {
	root := findModuleRoot(t)
	allowSet := buildAllowSet()

	violations := scanGoProductFiles(t, root)

	var failures []string
	for _, v := range violations {
		key := v.File + "\x00" + v.Value
		if !allowSet[key] {
			display := v.Value
			if len([]rune(display)) > 80 {
				display = string([]rune(display)[:77]) + "..."
			}
			failures = append(failures, fmt.Sprintf("  %s:%d: %q", v.File, v.Line, display))
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		t.Errorf("Portuguese literal(s) found in product code (not in allowlist):\n%s\n\nIf this is a bilingual reader or preset name, add it to allowedPTLiterals in pt_literals_test.go with a reason.", strings.Join(failures, "\n"))
	}
}

// TestAllowlistIsExhaustive verifies that every entry in allowedPTLiterals
// still matches at least one literal in the product code. A stale entry
// fails the test, keeping the allowlist closed.
func TestAllowlistIsExhaustive(t *testing.T) {
	root := findModuleRoot(t)
	violations := scanGoProductFiles(t, root)

	// Build set of all PT literals actually found
	found := make(map[string]bool, len(violations))
	for _, v := range violations {
		found[v.File+"\x00"+v.Value] = true
	}

	var stale []string
	for _, e := range allowedPTLiterals {
		key := e.File + "\x00" + e.Value
		if !found[key] {
			stale = append(stale, fmt.Sprintf("  %s: %q (class %s)", e.File, truncate(e.Value, 60), e.Class))
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("Allowlist has stale entries (literal no longer exists in product code); remove or update them:\n%s", strings.Join(stale, "\n"))
	}
}

// TestDetectorUnit exercises the containsPT detector directly.
// Each sub-test has one sentence explaining what it asserts.
func TestDetectorUnit(t *testing.T) {
	t.Run("accented_char_triggers", func(t *testing.T) {
		// AFIRMA: um literal com caractere acentuado PT é detectado
		if !containsPT("Critérios de Aceite") {
			t.Error("expected 'é' to trigger PT detection")
		}
	})
	t.Run("unaccented_caducou_triggers", func(t *testing.T) {
		// AFIRMA: a palavra "Caducou" (sem acento) é detectada como PT
		if !containsPT("Caducou: foo") {
			t.Error("expected 'Caducou' to trigger PT detection")
		}
	})
	t.Run("unaccented_pendente_triggers", func(t *testing.T) {
		// AFIRMA: a palavra "Pendente" (sem acento) é detectada como PT
		if !containsPT("**Status:** ⬜ Pendente") {
			t.Error("expected 'Pendente' to trigger PT detection")
		}
	})
	t.Run("english_only_passes", func(t *testing.T) {
		// AFIRMA: texto puramente em inglês não é detectado como PT
		if containsPT("No roadmap found. Create one with 'trackfw roadmap new'.") {
			t.Error("English-only string wrongly detected as PT")
		}
	})
	t.Run("accented_char_in_raw_string_triggers", func(t *testing.T) {
		// AFIRMA: acento PT em raw string (backtick) também é detectado
		if !containsPT(`["## Acceptance Criteria", "## Critérios de Aceite"]`) {
			t.Error("expected accent in raw string to trigger PT detection")
		}
	})
	t.Run("f5_literal_triggers", func(t *testing.T) {
		// AFIRMA: o literal F5 (manager.go:512 antes da correção) é detectado pelos novos ptUnaccentedWords — aviso e prosseguindo
		if !containsPT(`aviso: %q declara o mesmo name %q que %q; prosseguindo por --force`) {
			t.Error("expected F5 literal to trigger PT detection via 'aviso' and 'prosseguindo'")
		}
	})
	t.Run("f5_new_literal_passes", func(t *testing.T) {
		// AFIRMA: a tradução EN do literal F5 não é detectada como PT
		if containsPT(`warning: %q declares the same name %q as %q; proceeding due to --force`) {
			t.Error("translated F5 literal wrongly detected as PT")
		}
	})
	t.Run("erroneous_not_triggered", func(t *testing.T) {
		// AFIRMA: a palavra "erroneous" (inglês) não ativa o detector — a fronteira de palavra impede o match
		if containsPT("erroneous configuration detected") {
			t.Error("'erroneous' wrongly detected as PT ('erro' word boundary must not match)")
		}
	})
}

// TestNoPTLiteralsInStaticWeb checks app.js and index.html for PT text
// outside of line comments (//) and block comments (/* */ and <!-- -->).
// It uses the same containsPT detector as the Go scanner so the two scanners
// cannot diverge (ML-3G corretivo: ptWebRe was a parallel list that did not
// include the 21 unaccented words added in ML-3G).
func TestNoPTLiteralsInStaticWeb(t *testing.T) {
	root := findModuleRoot(t)
	files := []string{
		filepath.Join(root, "internal", "serve", "static", "app.js"),
		filepath.Join(root, "internal", "serve", "static", "index.html"),
	}

	for _, fpath := range files {
		rel, _ := filepath.Rel(root, fpath)
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(fpath)
		if err != nil {
			t.Errorf("reading %s: %v", rel, err)
			continue
		}

		lines := strings.Split(string(data), "\n")
		inBlockComment := false

		for i, line := range lines {
			stripped := strings.TrimSpace(line)

			// Track block comment state
			if inBlockComment {
				if strings.Contains(line, "*/") || strings.Contains(line, "-->") {
					inBlockComment = false
				}
				continue
			}

			// Enter block comment
			if strings.Contains(stripped, "/*") || strings.Contains(stripped, "<!--") {
				// Check for same-line close
				if !strings.Contains(stripped, "*/") && !strings.Contains(stripped, "-->") {
					inBlockComment = true
				}
				continue
			}

			// Skip full-line // comments and JSDoc continuation lines
			if strings.HasPrefix(stripped, "//") || strings.HasPrefix(stripped, "*") {
				continue
			}

			// Remove inline // comments
			if idx := indexLineComment(line); idx >= 0 {
				line = line[:idx]
			}

			if containsPT(line) {
				t.Errorf("%s:%d: PT text outside comment: %q", rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// indexLineComment returns the byte index of the start of a // comment in s,
// or -1 if none. It respects string literals so URLs do not trigger false positives.
func indexLineComment(s string) int {
	inStr := false
	var strChar rune
	runes := []rune(s)
	for i, r := range runes {
		if inStr {
			if r == strChar && (i == 0 || runes[i-1] != '\\') {
				inStr = false
			}
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			inStr = true
			strChar = r
			continue
		}
		if r == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			// convert rune index to byte index
			return len(string(runes[:i]))
		}
	}
	return -1
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-3]) + "..."
}

// TestI18nLocalesOnlyEN — AFIRMA que internal/i18n/locales/ contém apenas
// en-US.json; qualquer outro arquivo JSON reprova, impedindo a reintrodução
// de catálogos PT/ES (ML-3E).
func TestI18nLocalesOnlyEN(t *testing.T) {
	root := findModuleRoot(t)
	localesDir := filepath.Join(root, "internal", "i18n", "locales")
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("reading locales dir %s: %v", localesDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if e.Name() != "en-US.json" {
			t.Errorf("unexpected file in internal/i18n/locales/: %s — only en-US.json is allowed (ML-3E, AC2)", e.Name())
		}
	}
}

// silence unused unicode import in some Go versions
var _ = unicode.Letter
