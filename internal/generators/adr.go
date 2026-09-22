package generators

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/kgsaran/trackfw/internal/pathguard"
	"golang.org/x/text/unicode/norm"
)

// ADRContent contém os campos de um ADR a ser gerado.
type ADRContent struct {
	Title        string
	Context      string
	Decision     string
	Consequences string
	Alternatives string
}

// NewADR gera um arquivo ADR em adrDir com base no conteúdo fornecido.
// Campos preenchidos são inseridos diretamente; campos vazios mantêm o placeholder HTML.
// O chamador resolve adrDir (via config.Load().ADRDirs[0] para escopo "project" ou via
// GlobalADRDir(home) para escopo "global") — esta função não lê trackfw.yaml, permitindo
// uso em --scope global sem exigir projeto/trackfw.yaml no cwd.
func NewADR(content ADRContent, adrDir string) error {
	// Guard adrDir before MkdirAll. The root depends on scope:
	//   project scope (adrDir relative or beneath cwd): use projectRoot() so all
	//     ancestors between root and adrDir are checked.
	//   global scope (adrDir absolute, outside cwd): use absAdrDir as root — only
	//     symlinks within adrDir are caught; ancestors above it are an accepted residual
	//     (documented per ADR decision 3, named exception for explicit global paths).
	absAdrDir, err := filepath.Abs(adrDir)
	if err != nil {
		return fmt.Errorf("resolving adrDir: %w", err)
	}
	// Guard adrDir BEFORE EvalSymlinks: if absAdrDir itself is a symlink (or has a symlink
	// ancestor), EvalSymlinks would resolve it to the target outside the tree, making
	// Beneath(pr, resolved) false and defeating the containment check. We use the raw
	// absolute path for the guard so that RejectSymlinks can detect the symlink.
	guardRoot := absAdrDir // default: global scope — root at adrDir itself
	if pr, prErr := projectRoot(); prErr == nil {
		if pathguard.Beneath(pr, absAdrDir) {
			guardRoot = pr // project scope — full ancestor check from project root
		}
	}
	absFilename := filepath.Join(absAdrDir, ".trackfw-new-adr") // probe path inside adrDir
	if guardErr := pathguard.RejectSymlinks(guardRoot, absFilename); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absAdrDir, guardErr)
		return fmt.Errorf("refusing write to %s: %w", absAdrDir, guardErr)
	}
	// Save pre-EvalSymlinks path for the leaf guard below: the leaf guard must
	// use the raw absolute path (same namespace as guardRoot) rather than the
	// resolved canonical path, to avoid the EvalSymlinks-before-guard anti-pattern
	// (Wave 1 fix, ADR-2026-09-18 decision 3).
	absAdrDirRaw := absAdrDir
	// EvalSymlinks for canonical path (after guard passes — only for non-symlink paths).
	if resolved, resolveErr := filepath.EvalSymlinks(absAdrDir); resolveErr == nil {
		absAdrDir = resolved
	}

	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		return err
	}

	slug := toSlug(content.Title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s/ADR-%s-%s.md", adrDir, date, slug)

	contextSection := "<!-- What is the situation that motivates this decision? -->"
	if content.Context != "" {
		contextSection = content.Context
	}

	decisionSection := "<!-- What was decided? -->"
	if content.Decision != "" {
		decisionSection = content.Decision
	}

	consequencesSection := "<!-- What are the positive and negative consequences of this decision? -->"
	if content.Consequences != "" {
		consequencesSection = content.Consequences
	}

	alternativesSection := "<!-- What other options were evaluated and why were they rejected? -->"
	if content.Alternatives != "" {
		alternativesSection = content.Alternatives
	}

	body := fmt.Sprintf(`---
status: Proposed
date: %s
author: ""
---

# ADR: %s

> Date: %s | Status: Proposed

## Context
%s

## Decision
%s

## Consequences
%s

## Alternatives Considered
%s
`, date, content.Title, date, contextSection, decisionSection, consequencesSection, alternativesSection)

	// Leaf guard: the probe guard above covered the ancestor chain up to
	// absAdrDir; now guard the exact file (using the pre-EvalSymlinks path to
	// stay in the same namespace as guardRoot) so a symlink leaf pointing
	// outside root is also caught (ML-4B leaf-gap fix).
	absLeaf := filepath.Join(absAdrDirRaw, filepath.Base(filename))
	if guardErr := pathguard.RejectSymlinks(guardRoot, absLeaf); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absLeaf, guardErr)
		return fmt.Errorf("refusing write to %s: %w", absLeaf, guardErr)
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
		return fmt.Errorf("writing ADR: %w", err)
	}

	fmt.Printf("created %s\n", filename)
	return nil
}

// ListADRs lista todos os ADRs encontrados em dir, imprimindo filename e status.
// Retorna nil se o diretório estiver ausente ou sem arquivos .md.
func ListADRs(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return fmt.Errorf("listing ADRs: %w", err)
	}
	if len(matches) == 0 {
		fmt.Printf("No ADRs found in %s\n", dir)
		return nil
	}

	for _, path := range matches {
		filename := filepath.Base(path)
		title, status := parseADRMeta(path)
		if title == "" {
			title = filename
		}
		fmt.Printf("%-60s %s\n", filename, status)
		_ = title
	}
	return nil
}

// parseADRMeta extrai título e status de um arquivo ADR markdown.
func parseADRMeta(path string) (title, status string) {
	f, err := os.Open(path)
	if err != nil {
		return "", "unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	status = "unknown"
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ADR: ") {
			title = strings.TrimPrefix(line, "# ADR: ")
		}
		if strings.Contains(line, "| Status: ") {
			idx := strings.Index(line, "| Status: ")
			if idx >= 0 {
				rest := line[idx+len("| Status: "):]
				rest = strings.TrimRight(rest, " >|")
				status = strings.TrimSpace(rest)
			}
		}
	}
	return title, status
}

// slugNonAlnum corresponde a sequências de caracteres fora de [a-z0-9].
var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// toSlug converte uma string em slug kebab-case portável:
// 1. NFKD normalization — decompõe diacríticos em base + combining mark.
// 2. Remove combining marks (categoria Unicode Mn) — elimina acentos.
// 3. Lowercase.
// 4. Substitui sequências de não-[a-z0-9] por hífen.
// 5. Remove hífens nas extremidades.
// Ex: "Autenticação e Sessão" → "autenticacao-e-sessao"
func toSlug(s string) string {
	// Passo 1+2: NFKD → filtrar combining marks
	normalized := norm.NFKD.String(s)
	var b strings.Builder
	for _, r := range normalized {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	s = b.String()

	// Passo 3: lowercase
	s = strings.ToLower(s)

	// Passo 4: [^a-z0-9]+ → hífen
	s = slugNonAlnum.ReplaceAllString(s, "-")

	// Passo 5: trim hífens nas extremidades
	s = strings.Trim(s, "-")
	return s
}

// slugToTitle converte um slug com hífens em título com title case.
// Ex: "authentication-strategy" → "Authentication Strategy"
func slugToTitle(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// NewADRDraft cria um ADR com Status: Draft a partir de um slug.
// Usado pelo wizard req new para registrar decisões pendentes.
// Retorna o basename do arquivo criado.
// Se o arquivo já existir, não sobrescreve (idempotente) e retorna o basename sem erro.
// O chamador resolve adrDir (via config.Load().ADRDirs[0] para escopo "local" ou via
// GlobalADRDir(home) para escopo "global") — esta função não lê trackfw.yaml, mesmo
// padrão de NewADR.
func NewADRDraft(slug string, adrDir string) (string, error) {
	// Same root-selection logic as NewADR: project scope uses projectRoot(), global scope
	// uses absAdrDir. This ensures consistent containment behavior for both write paths.
	absAdrDirDraft, draftAbsErr := filepath.Abs(adrDir)
	if draftAbsErr != nil {
		return "", fmt.Errorf("resolving adrDir: %w", draftAbsErr)
	}
	// Guard BEFORE EvalSymlinks — same reason as NewADR: resolving the symlink first
	// would defeat the Beneath check by making absAdrDirDraft point outside the tree.
	draftGuardRoot := absAdrDirDraft
	if pr, prErr := projectRoot(); prErr == nil {
		if pathguard.Beneath(pr, absAdrDirDraft) {
			draftGuardRoot = pr
		}
	}
	draftProbe := filepath.Join(absAdrDirDraft, ".trackfw-new-adr-draft")
	if guardErr := pathguard.RejectSymlinks(draftGuardRoot, draftProbe); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absAdrDirDraft, guardErr)
		return "", fmt.Errorf("refusing write to %s: %w", absAdrDirDraft, guardErr)
	}
	// Save pre-EvalSymlinks path for the leaf guard below (same reason as NewADR).
	absAdrDirDraftRaw := absAdrDirDraft
	// EvalSymlinks after guard passes — for canonical path usage.
	if resolved, resolveErr := filepath.EvalSymlinks(absAdrDirDraft); resolveErr == nil {
		absAdrDirDraft = resolved
	}

	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		return "", fmt.Errorf("creating %s: %w", adrDir, err)
	}

	// Verificar idempotência: glob por slug
	pattern := filepath.Join(adrDir, "ADR-*-"+slug+".md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	if len(matches) > 0 {
		basename := filepath.Base(matches[0])
		fmt.Printf("skipped %s (already exists)\n", basename)
		return basename, nil
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("ADR-%s-%s.md", date, slug)
	path := filepath.Join(adrDir, filename)
	title := slugToTitle(slug)

	body := fmt.Sprintf(`---
status: Draft
date: %s
author: ""
---

# ADR: %s

> Date: %s | Status: Draft

## Context
<!-- What is the situation that motivates this decision? -->

## Decision
<!-- What was decided? -->

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->
`, date, title, date)

	// Leaf guard: same pattern as NewADR — guard the exact file using the
	// pre-EvalSymlinks path so a symlink leaf is caught (ML-4B leaf-gap fix).
	absLeafDraft := filepath.Join(absAdrDirDraftRaw, filename)
	if guardErr := pathguard.RejectSymlinks(draftGuardRoot, absLeafDraft); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absLeafDraft, guardErr)
		return "", fmt.Errorf("refusing write to %s: %w", absLeafDraft, guardErr)
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return "", fmt.Errorf("writing ADR draft: %w", err)
	}

	fmt.Printf("created %s\n", filename)
	return filename, nil
}
