package generators

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// RoadmapContent contém os dados para criação de um roadmap.
type RoadmapContent struct {
	Title   string
	REQPath string
	Body    string
}

// wave0GateFence is the fixed, literal, non-interpolated gate command emitted inside every
// generated "## Wave 0 — Threat Model" block (AC13, docs/cli-parity.md § "trackfw barrier").
//
// It intentionally FAILS CLOSED: `exit 1` always blocks the "gates" check until the ML-0A
// author replaces this placeholder with a real, project-specific evidence check. This is the
// only mechanical lever `barrier` has against a vacuous Wave 0 — `gates` reports "passed" when a
// wave declares zero gates (parseGates returns an empty, non-nil slice), so an empty Wave 0,
// copied verbatim, or written by the implementer instead of a reviewer, would otherwise pass
// clean every time (docs/seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md, §2.1).
//
// Not interpolated: no REQ title, slug, date or any user-controlled string is substituted into
// this command. runGateCommand (internal/commands/barrier.go) executes gate commands via
// `sh -c` with no sanitization — interpolating a REQ title containing backticks or `$(...)`
// would turn this into arbitrary shell execution inside the harness. The command below is a
// constant string, byte-identical across every project that runs `trackfw update`.
const wave0GateFence = "```bash\n" +
	"# Wave 0 gate — replace this placeholder with a project-specific check before\n" +
	"# marking ML-0A done. Do not remove the gate; replace its command (AC13).\n" +
	"exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md\n" +
	"```\n"

// statusLegendBlock teaches the vocabulary the `barrier` parser accepts for "**Status:**"
// (AC11, ADR decision 5): the canonical form the template now writes (⬜ Pendente) plus the
// other three states. Placed once, right before the first wave, so it is close to the first
// place a "**Status:**" line appears — not repeated per-ML (would clutter) and not left for
// the end (nobody reads that far). Byte-identical across the 3 CLIs
// (gate: scripts/check-artifact-parity.sh).
const statusLegendBlock = "## Status Legend\n" +
	"⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado\n" +
	"\n"

// wave0Block is the "## Wave 0 — Threat Model" section prepended to every generated roadmap,
// before the first implementation wave (AC1, AC12). It is a plain (non-raw) Go string — not a
// backtick raw string literal — because it embeds a fenced ```bash block itself: a raw string
// cannot contain a literal backtick without terminating early. Byte-identical across the "new"
// and "--from-req" generation paths, and across the 3 CLIs (gate: scripts/check-artifact-parity.sh).
//
// The ML is always labeled ML-0A, never ML-1A — NewRoadmapFromREQ labels MLs derived from REQ
// acceptance criteria "ML-1A", "ML-1B", ... starting at the first criterion, so "ML-0A" is the
// only label available to Wave 0 without colliding with a derived ML
// (docs/seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md, §1.2).
const wave0Block = statusLegendBlock +
	"## Wave 0 — Threat Model\n" +
	"> Dependencies: none. Blocks all implementation.\n" +
	"\n" +
	"### ML-0A — Threat model for this roadmap\n" +
	"**Status:** ⬜ Pendente\n" +
	"**Files affected:**\n" +
	"**Actions:**\n" +
	"1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).\n" +
	"2. Threat model — who empties this Wave 0 without breaking any written rule, and how?\n" +
	"3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?\n" +
	"4. Declared residual — what this design accepts not covering.\n" +
	"**Acceptance criteria:**\n" +
	"- [ ] The four sections above answered with evidence, not a one-line assertion\n" +
	"- [ ] No implementation line written for this ML\n" +
	"\n" +
	"**Gates da wave:**\n" +
	wave0GateFence +
	"\n"

var roadmapStateOrder = []string{"analyzing", "wip", "backlog", "blocked", "done", "abandoned"}
var roadmapValidStateNames = map[string]bool{
	"backlog": true, "analyzing": true, "wip": true, "blocked": true, "done": true, "abandoned": true,
}

const roadmapValidStatesMessage = "backlog, analyzing, wip, blocked, done, abandoned"

// stateDir retorna o caminho do diretório para um estado válido no modo flat, ou "", false se inválido.
func stateDir(state string) (string, bool) {
	cfg := config.Load()
	if !roadmapValidStateNames[state] {
		return "", false
	}
	return cfg.RoadmapDir + "/" + state, true
}

// agentStateDir retorna o diretório para um agente+estado em modo by_agent.
// agent="" usa o primeiro agente configurado (ou "default" se lista vazia).
func agentStateDir(agent, state string) (string, bool) {
	cfg := config.Load()
	if !roadmapValidStateNames[state] {
		return "", false
	}
	if agent == "" {
		if len(cfg.Agents) > 0 {
			agent = cfg.Agents[0]
		} else {
			agent = "default"
		}
	}
	return cfg.RoadmapDir + "/" + agent + "/" + state, true
}

// logPath retorna o caminho do arquivo de log de transições.
func logPath() string {
	return config.Load().RoadmapDir + "/.trackfw-log"
}

// NewRoadmap cria um roadmap com template padrão a partir de um título simples.
func NewRoadmap(title string) error {
	return NewRoadmapFromContent(RoadmapContent{Title: title})
}

// NewRoadmapFromContent cria um roadmap a partir de um RoadmapContent.
// Se Body for preenchido, usa diretamente; caso contrário, gera template padrão.
func NewRoadmapFromContent(content RoadmapContent) error {
	// AC1/AC2: o título é dado de uma linha — newline e CR são entrada malformada.
	// A mensagem é contrato de paridade: byte-idêntica nos 3 CLIs (docs/cli-parity.md).
	if strings.ContainsAny(content.Title, "\n\r") {
		return fmt.Errorf("roadmap title must be a single line: newline and carriage return are not allowed")
	}

	cfg := config.Load()

	var backlogDir string
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		dir, ok := agentStateDir("", "backlog")
		if !ok {
			return fmt.Errorf("cannot resolve backlog dir in by_agent mode")
		}
		backlogDir = dir
	} else {
		backlogDir = cfg.RoadmapDir + "/backlog"
	}

	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		return err
	}

	slug := toSlug(content.Title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s/ROADMAP-%s-%s.md", backlogDir, date, slug)

	var body string
	if content.Body != "" {
		body = content.Body
	} else {
		body = fmt.Sprintf(`---
status: backlog
date: %s
req: "%s"
squad: ""
---

# Roadmap: %s

> Created: %s | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: %s

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

`, date, content.REQPath, content.Title, date, content.REQPath) + wave0Block + fmt.Sprintf(`## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — %s
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
`, content.Title)
	}

	if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
		return fmt.Errorf("writing roadmap: %w", err)
	}

	fmt.Printf("✓ created %s\n", filename)
	return nil
}

// NewRoadmapFromREQ cria um roadmap pré-preenchido lendo o conteúdo de uma REQ.
// Extrai título e critérios de aceite; gera MLs rascunho para cada critério.
func NewRoadmapFromREQ(reqPath string) error {
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return fmt.Errorf("reading REQ: %w", err)
	}

	title, criteria, linkedADR := parseREQForRoadmap(string(data))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(reqPath), ".md")
		title = strings.TrimPrefix(title, "REQ-")
	}

	// AC1: o título lido da REQ também pode conter newline forjado — rejeitar cedo,
	// antes de interpolar em fmt.Sprintf abaixo. NewRoadmapFromContent repete a guarda
	// mas a mensagem de erro sai daqui para o caminho --from-req.
	if strings.ContainsAny(title, "\n\r") {
		return fmt.Errorf("roadmap title must be a single line: newline and carriage return are not allowed")
	}

	date := time.Now().Format("2006-01-02")

	// Gerar seção de MLs a partir dos critérios de aceite
	var mlSection strings.Builder
	mlSection.WriteString(wave0Block)
	mlSection.WriteString("## Wave 1 — Implementation (derived from REQ criteria)\n")
	mlSection.WriteString("> Dependencies: none\n")
	for i, criterion := range criteria {
		mlLabel := fmt.Sprintf("ML-1%c", rune('A'+i))
		mlSection.WriteString(fmt.Sprintf("\n### %s — %s\n", mlLabel, criterion))
		mlSection.WriteString("**Status:** ⬜ Pendente\n")
		mlSection.WriteString("**Files affected:**\n")
		mlSection.WriteString("**Actions:**\n")
		mlSection.WriteString("**Acceptance criteria:**\n")
		mlSection.WriteString(fmt.Sprintf("- [ ] %s\n", criterion))
		mlSection.WriteString("- [ ] build passes\n")
		mlSection.WriteString("- [ ] tests green\n")
	}

	adrRef := ""
	if linkedADR != "" {
		adrRef = "\nADR: " + linkedADR
	}

	body := fmt.Sprintf(`---
status: backlog
date: %s
req: "%s"
squad: ""
---

# Roadmap: %s

> Created: %s | Status: backlog

## Context
<!-- Derived from REQ: %s -->
REQ: %s%s

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

%s`, date, reqPath, title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())

	return NewRoadmapFromContent(RoadmapContent{
		Title: title,
		Body:  body,
	})
}

// parseREQForRoadmap extrai título, critérios de aceite e ADR linkada de um arquivo REQ.
func parseREQForRoadmap(content string) (title string, criteria []string, linkedADR string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	inCriteria := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "# REQ: ") {
			title = strings.TrimPrefix(line, "# REQ: ")
			continue
		}
		if strings.HasPrefix(line, "# REQ — ") {
			title = strings.TrimPrefix(line, "# REQ — ")
			continue
		}
		if strings.HasPrefix(line, "# REQ - ") {
			title = strings.TrimPrefix(line, "# REQ - ")
			continue
		}
		if strings.HasPrefix(line, "**ADR:**") {
			linkedADR = strings.TrimSpace(strings.TrimPrefix(line, "**ADR:**"))
			continue
		}

		// Detectar seção de critérios (pt-BR e en-US)
		lower := strings.ToLower(strings.TrimSpace(line))
		if lower == "## critérios de aceite" || lower == "## acceptance criteria" {
			inCriteria = true
			continue
		}
		if inCriteria && strings.HasPrefix(line, "## ") {
			inCriteria = false
			continue
		}
		if inCriteria {
			// Capturar itens de checklist: "- [ ] texto"
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
				item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- [ ]"), "- [x]"), "- [X]"))
				// Remover backticks de código para nome do ML
				item = strings.ReplaceAll(item, "`", "")
				if item != "" {
					criteria = append(criteria, item)
				}
			}
		}
	}
	return title, criteria, linkedADR
}

// rewriteRoadmapStatus rewrites the "status:" field in the frontmatter block and
// the "| Status: <value>" portion of the first matching header line in the body.
//
// Mirrors the semantics of rewriteFrontmatterFields (internal/integrations/render.go):
//   - Scoped strictly to the frontmatter block (between opening "---\n" and closing "\n---").
//   - Every other line is preserved byte-for-byte (order, spacing, quote style).
//   - The key is NOT invented if absent; source is returned unchanged.
//   - If source has no recognizable frontmatter, source is returned unchanged without error.
//
// The body "| Status: " sync is also scoped: only the first occurrence before the
// first "## " heading is updated; any occurrence inside sections or code blocks is left intact.
//
// Returns the (possibly modified) content and a bool indicating whether anything changed.
func rewriteRoadmapStatus(source []byte, state string) ([]byte, bool) {
	s := string(source)
	if !strings.HasPrefix(s, "---\n") {
		return source, false
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return source, false
	}
	frontmatter := s[4 : 4+end]
	rest := s[4+end:] // starts with "\n---", followed by the body

	changed := false
	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "status" {
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		quoted := len(trimmedValue) >= 2 && strings.HasPrefix(trimmedValue, `"`) && strings.HasSuffix(trimmedValue, `"`)
		var newLine string
		if quoted {
			newLine = key + ": \"" + state + "\""
		} else {
			newLine = key + ": " + state
		}
		if lines[i] != newLine {
			lines[i] = newLine
			changed = true
		}
		break // only the first status: in frontmatter
	}

	// Sync "| Status: <value>" in the header line of the body (after the closing ---).
	// Only the first occurrence before the first "## " heading is updated.
	if len(rest) > 4 {
		body := rest[4:] // skip "\n---"
		bodyLines := strings.Split(body, "\n")
		const marker = "| Status: "
		for i, bline := range bodyLines {
			if strings.HasPrefix(strings.TrimSpace(bline), "## ") {
				break
			}
			idx := strings.Index(bline, marker)
			if idx < 0 {
				continue
			}
			prefix := bline[:idx+len(marker)]
			after := bline[idx+len(marker):]
			var suffix string
			if pipeIdx := strings.Index(after, " |"); pipeIdx >= 0 {
				suffix = after[pipeIdx:]
			}
			newLine := prefix + state + suffix
			if bodyLines[i] != newLine {
				bodyLines[i] = newLine
				changed = true
				rest = "\n---" + strings.Join(bodyLines, "\n")
			}
			break // only the first | Status: before ##
		}
	}

	if !changed {
		return source, false
	}
	return []byte("---\n" + strings.Join(lines, "\n") + rest), true
}

func MoveRoadmap(name, state string) error {
	cfg := config.Load()

	// Validar estado antes de buscar o roadmap (melhor UX)
	if !roadmapValidStateNames[state] {
		return fmt.Errorf("invalid state %q — valid states: %s", state, roadmapValidStatesMessage)
	}

	src, err := findRoadmap(name)
	if err != nil {
		return err
	}

	var targetDir string
	var fromState string

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		// em by_agent: src = roadmapDir/agent/state/file → agentDir é a pasta avó
		agentDir := filepath.Dir(filepath.Dir(src))
		agent := filepath.Base(agentDir)
		fromState = filepath.Base(filepath.Dir(src))
		var ok bool
		targetDir, ok = agentStateDir(agent, state)
		if !ok {
			return fmt.Errorf("invalid state %q — valid states: %s", state, roadmapValidStatesMessage)
		}
	} else {
		fromState = filepath.Base(filepath.Dir(src))
		var ok bool
		targetDir, ok = stateDir(state)
		if !ok {
			return fmt.Errorf("invalid state %q — valid states: %s", state, roadmapValidStatesMessage)
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}

	dst := filepath.Join(targetDir, filepath.Base(src))
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving roadmap: %w", err)
	}

	// Synchronize status: in the frontmatter (and header line in body) to match the new state.
	if rawContent, readErr := os.ReadFile(dst); readErr == nil {
		if updated, changed := rewriteRoadmapStatus(rawContent, state); changed {
			_ = os.WriteFile(dst, updated, 0644)
		}
	}

	logBasename := filepath.Base(src)
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agent := filepath.Base(filepath.Dir(filepath.Dir(src)))
		logBasename = agent + "/" + filepath.Base(src)
	}
	appendTransitionLog(logBasename, fromState, state)

	fmt.Printf("✓ moved %s → %s\n", filepath.Base(src), targetDir)

	// Synchronize roadmap: reference in every paired REQ that points at the moved roadmap.
	// Runs after ✓ moved is printed so ✓ synced always follows it in stdout.
	// A sync failure does NOT roll back the move; the error causes non-zero exit.
	if syncErr := syncREQReferences(filepath.Base(src), dst); syncErr != nil {
		return syncErr
	}
	return nil
}

func findRoadmap(name string) (string, error) {
	cfg := config.Load()

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := validator.ResolveAgentNamespaces(cfg, cfg.RoadmapDir)
		for _, agent := range agents {
			for _, state := range roadmapStateOrder {
				dir := cfg.RoadmapDir + "/" + agent + "/" + state
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if containsIgnoreCase(e.Name(), name) {
						return filepath.Join(dir, e.Name()), nil
					}
				}
			}
		}
	} else {
		for _, state := range roadmapStateOrder {
			dir := cfg.RoadmapDir + "/" + state
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if containsIgnoreCase(e.Name(), name) {
					return filepath.Join(dir, e.Name()), nil
				}
			}
		}
	}
	return "", fmt.Errorf("roadmap %q not found in any state directory", name)
}

func containsIgnoreCase(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func appendTransitionLog(basename, fromState, toState string) {
	f, err := os.OpenFile(logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf("%s  %-50s  %s → %s\n",
		time.Now().Format("2006-01-02 15:04"),
		basename,
		fromState,
		toState,
	)
	f.WriteString(line)
}

// ShowRoadmap exibe o conteúdo de um roadmap identificado por nome parcial.
func ShowRoadmap(name string) error {
	cfg := config.Load()

	var pattern string
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		// 3 níveis: roadmapDir/agent/state/file
		pattern = filepath.Join(cfg.RoadmapDir, "*", "*", "*"+name+"*.md")
	} else {
		pattern = filepath.Join(cfg.RoadmapDir, "*", "*"+name+"*.md")
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no roadmap found matching %q", name)
	}
	if len(matches) > 1 {
		fmt.Println("Multiple roadmaps found — be more specific:")
		for _, m := range matches {
			fmt.Printf("  %s\n", m)
		}
		return fmt.Errorf("ambiguous match for %q", name)
	}
	path := matches[0]
	state := filepath.Base(filepath.Dir(path))
	base := filepath.Base(path)
	fmt.Printf("── %s ── [%s] ──────────────────────\n\n", base, strings.ToUpper(state))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	fmt.Printf("Location: %s\n", path)
	return nil
}

// ListRoadmaps imprime todos os roadmaps agrupados por estado (e por agente em modo by_agent).
func ListRoadmaps() error {
	cfg := config.Load()
	found := false

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := validator.ResolveAgentNamespaces(cfg, cfg.RoadmapDir)
		for _, agent := range agents {
			for _, state := range roadmapStateOrder {
				dir := cfg.RoadmapDir + "/" + agent + "/" + state
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				var files []string
				for _, e := range entries {
					if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
						files = append(files, e.Name())
					}
				}
				if len(files) == 0 {
					continue
				}
				found = true
				fmt.Printf("[%s/%s]\n", agent, state)
				for _, f := range files {
					fmt.Printf("  %s\n", f)
				}
			}
		}
	} else {
		for _, state := range roadmapStateOrder {
			dir := cfg.RoadmapDir + "/" + state
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			var files []string
			for _, e := range entries {
				if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
					files = append(files, e.Name())
				}
			}
			if len(files) == 0 {
				continue
			}
			found = true
			fmt.Printf("[%s]\n", state)
			for _, f := range files {
				fmt.Printf("  %s\n", f)
			}
		}
	}

	if !found {
		fmt.Println("Nenhum roadmap encontrado. Crie um com 'trackfw roadmap new'.")
	}
	return nil
}

// ─── REQ synchronization helpers ─────────────────────────────────────────────

// scanREQFiles retorna os caminhos de todos os .md no req_dir,
// espelhando exatamente o comportamento de resolveREQFiles do validador:
//   - flat (padrão)   → req_dir/*.md
//   - by_agent        → req_dir/<agente>/<estado>/*.md
func scanREQFiles(cfg config.ProjectConfig) []string {
	reqDir := cfg.REQDir
	if reqDir == "" {
		return nil
	}
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		stateDirs := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
		agents := validator.ResolveAgentNamespaces(cfg, reqDir)
		var files []string
		for _, agent := range agents {
			// ML-4A (achado 2, hades-tf 2026-08-30): agent vem do disco — validator.ListMDFiles em
			// vez de filepath.Glob (ver comentário de ListMDFiles em internal/validator/validator.go).
			for _, state := range stateDirs {
				files = append(files, validator.ListMDFiles(filepath.Join(reqDir, agent, state))...)
			}
		}
		return files
	}
	matches, _ := filepath.Glob(filepath.Join(reqDir, "*.md"))
	return matches
}

// extractFrontmatterRoadmap extrai o valor do campo roadmap: do bloco frontmatter YAML.
// Retorna string vazia se o campo estiver ausente, vazio ou fora do frontmatter.
// Trima aspas simples e duplas mas NÃO backticks — espelha o comportamento de
// extractRefPath do validador, onde a forma com backtick não termina em ".md"
// e é ignorada pelo validador.
func extractFrontmatterRoadmap(content string) string {
	lines := strings.Split(content, "\n")
	inFM := false
	fmCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			fmCount++
			if fmCount == 1 {
				inFM = true
				continue
			}
			break // fechou o bloco frontmatter
		}
		if !inFM {
			break // sem frontmatter
		}
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), "roadmap") {
			val := strings.TrimSpace(v)
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

// rewriteREQRoadmapRef reescreve o campo roadmap: no frontmatter e a linha Roadmap: no
// corpo da REQ quando o basename do valor atual coincide com roadmapBasename.
// Preserva o estilo de formatação existente (aspas e backticks no corpo).
// Retorna (conteúdo atualizado, true) se houve mudança; (original, false) caso contrário.
//
// Nota: um REQ sem bloco frontmatter (sem par "---") não tem fmClosed=true,
// portanto o corpo não é varrido — situação não esperada pelos templates do projeto.
func rewriteREQRoadmapRef(content []byte, roadmapBasename, newRoadmapPath string) ([]byte, bool) {
	text := string(content)
	lines := strings.Split(text, "\n")

	changed := false
	inFM := false
	fmClosed := false
	fmCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !fmClosed {
			if trimmed == "---" {
				fmCount++
				if fmCount == 1 {
					inFM = true
					continue
				}
				fmClosed = true
				inFM = false
				continue
			}
			if inFM {
				k, v, ok := strings.Cut(line, ":")
				if ok && strings.EqualFold(strings.TrimSpace(k), "roadmap") {
					rawVal := strings.TrimSpace(v)
					plainVal := strings.Trim(rawVal, `"'`)
					if filepath.Base(plainVal) == roadmapBasename {
						// Preservar estilo de aspas do valor original
						var newLine string
						switch {
						case strings.HasPrefix(rawVal, `"`) || strings.HasSuffix(rawVal, `"`):
							newLine = fmt.Sprintf("%s: \"%s\"", strings.TrimSpace(k), newRoadmapPath)
						case strings.HasPrefix(rawVal, `'`) || strings.HasSuffix(rawVal, `'`):
							newLine = fmt.Sprintf("%s: '%s'", strings.TrimSpace(k), newRoadmapPath)
						default:
							newLine = fmt.Sprintf("%s: %s", strings.TrimSpace(k), newRoadmapPath)
						}
						if lines[i] != newLine {
							lines[i] = newLine
							changed = true
						}
					}
				}
				continue
			}
		}

		// Corpo (pós-frontmatter): reescrever linha "Roadmap: <valor>" preservando formato.
		if fmClosed {
			k, v, ok := strings.Cut(line, ":")
			if ok && strings.EqualFold(strings.TrimSpace(k), "Roadmap") {
				rawVal := strings.TrimSpace(v)
				plainVal := strings.Trim(rawVal, "`\"'")
				if filepath.Base(plainVal) == roadmapBasename {
					// Preservar backticks ou aspas do valor original
					var newVal string
					switch {
					case strings.HasPrefix(rawVal, "`") && strings.HasSuffix(rawVal, "`"):
						newVal = "`" + newRoadmapPath + "`"
					case strings.HasPrefix(rawVal, `"`) && strings.HasSuffix(rawVal, `"`):
						newVal = `"` + newRoadmapPath + `"`
					case strings.HasPrefix(rawVal, `'`) && strings.HasSuffix(rawVal, `'`):
						newVal = `'` + newRoadmapPath + `'`
					default:
						newVal = newRoadmapPath
					}
					newLine := fmt.Sprintf("%s: %s", strings.TrimSpace(k), newVal)
					if lines[i] != newLine {
						lines[i] = newLine
						changed = true
					}
				}
			}
		}
	}

	if !changed {
		return content, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// syncREQReferences atualiza o campo roadmap: no frontmatter (e a linha Roadmap: no corpo)
// de todas as REQs em req_dir cujo frontmatter roadmap: aponta para roadmapBasename.
//
// Cardinalidades (contrato pinado em docs/cli-parity.md):
//   - zero REQs apontando → no-op, sem output, exit 0
//   - uma ou mais       → reescreve todas, uma linha em stdout por REQ atualizada
//   - aponta para outro → não toca
//   - já correta        → nenhuma escrita (idempotente byte-a-byte)
//   - falha de escrita  → imprime diagnóstico em stderr, continua nas demais, retorna erro
func syncREQReferences(roadmapBasename, newRoadmapPath string) error {
	cfg := config.Load()
	reqFiles := scanREQFiles(cfg)

	// Ordenação lexicográfica por basename — contrato pinado em docs/cli-parity.md
	// ("Order is pinned, not delegated to the filesystem").
	// Desempate por caminho completo para dois agentes com REQ de mesmo basename.
	sort.Slice(reqFiles, func(i, j int) bool {
		bi, bj := filepath.Base(reqFiles[i]), filepath.Base(reqFiles[j])
		if bi != bj {
			return bi < bj
		}
		return reqFiles[i] < reqFiles[j]
	})

	var firstErr error
	for _, reqPath := range reqFiles {
		reqBase := filepath.Base(reqPath)
		content, err := os.ReadFile(reqPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "trackfw roadmap move: failed to sync %s: %v\n", reqBase, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// Descoberta: frontmatter roadmap: aponta para este roadmap?
		fmVal := extractFrontmatterRoadmap(string(content))
		if fmVal == "" || filepath.Base(fmVal) != roadmapBasename {
			continue // sem referência ou aponta para outro roadmap
		}

		// Idempotência: referência já está correta → nenhuma escrita
		if fmVal == newRoadmapPath {
			continue
		}

		updated, changed := rewriteREQRoadmapRef(content, roadmapBasename, newRoadmapPath)
		if !changed {
			continue
		}

		if err := os.WriteFile(reqPath, updated, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "trackfw roadmap move: failed to sync %s: %v\n", reqBase, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		fmt.Printf("✓ synced %s → %s\n", reqBase, newRoadmapPath)
	}

	return firstErr
}
