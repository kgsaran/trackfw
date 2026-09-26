package validator

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/pathguard"
)

// BaselineFile representa o conteúdo de .trackfw-baseline.json
type BaselineFile struct {
	Created    string   `json:"created"`
	Violations []string `json:"violations"`
	Warnings   []string `json:"warnings"`
}

const baselineFileName = ".trackfw-baseline.json"

// LoadBaseline lê .trackfw-baseline.json do CWD. Retorna nil se não existir.
func LoadBaseline() (*BaselineFile, error) {
	data, err := readRegularFile(baselineFileName)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bf BaselineFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("erro ao ler baseline: %w", err)
	}
	return &bf, nil
}

// SaveBaseline salva violations e warnings atuais em .trackfw-baseline.json.
func SaveBaseline(violations, warnings []string) error {
	bf := BaselineFile{
		Created:    time.Now().UTC().Format(time.RFC3339),
		Violations: violations,
		Warnings:   warnings,
	}
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return err
	}
	// Guard before write: baselineFileName is relative to CWD (= project root).
	// Root: EvalSymlinks(Getwd()) — macOS /tmp→/private/tmp invariant.
	// Guard precedes WriteFile so a refused destination creates no stray file.
	// Fail closed: if Getwd() fails we cannot verify containment, so we refuse the write.
	cwd, cwdErr := getwdFn()
	if cwdErr != nil {
		return pathguard.RefuseUnverifiableRoot(baselineFileName, cwdErr)
	}
	baselineRoot := cwd
	if resolved, resolveErr := filepath.EvalSymlinks(cwd); resolveErr == nil {
		baselineRoot = resolved
	}
	absBaseline := filepath.Join(baselineRoot, baselineFileName)
	if guardErr := pathguard.RejectAndReport(baselineRoot, absBaseline); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks above (fail-closed on Getwd error)
	return os.WriteFile(baselineFileName, data, 0644)
}

const staleWIPDays = 7

var staleWIPNow = time.Now

func inspectionDiagnostic(rule, target string, err error) string {
	return fmt.Sprintf("%s: could not inspect %q: %v", rule, target, err)
}

func listDirForRule(rule, dir string, msgs *[]string) []string {
	entries, err := listDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			*msgs = append(*msgs, inspectionDiagnostic(rule, dir, err))
		}
		return nil
	}
	return entries
}

func readFileForRule(rule, path string, msgs *[]string) ([]byte, bool) {
	content, err := readRegularFile(path)
	if err != nil {
		*msgs = append(*msgs, inspectionDiagnostic(rule, path, err))
		return nil, false
	}
	return content, true
}

// contentHasMarker retorna true se content contém algum dos marcadores com valor não-vazio.
// P3: verifica tanto "\n" quanto "\r\n" para detectar campos vazios em arquivos CRLF.
// Um marcador seguido de " \n" ou " \r\n" é tratado como "sem valor" (campo vazio).
//
// Usada apenas para markers de "existência de bloco" (ex: AcceptanceMarkers, um heading de
// seção) — não para markers de link (REQ:/ADR:/Roadmap:), que usam contentHasMarkerValue.
func contentHasMarker(content string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(content, marker) &&
			!strings.Contains(content, marker+" \n") &&
			!strings.Contains(content, marker+" \r\n") {
			return true
		}
	}
	return false
}

// contentHasMarkerValue retorna true se, em alguma LINHA de content, um dos marcadores aparece
// logo após espaços/tabs de indentação (nunca no meio de uma frase) e é seguido de um valor
// substantivo na mesma linha — não vazio e não um comentário HTML disfarçado de valor.
//
// issue #278 (achado A2 da auditoria externa, 2026-09-05): a versão anterior buscava o marcador
// como SUBSTRING em qualquer lugar do conteúdo. Isso produzia dois defeitos medidos no acervo
// mergeado:
//  1. prosa que menciona o marcador no meio de uma frase contava como vínculo real
//     ("veja a secao ADR: mais abaixo" → true, sem nenhum vínculo).
//  2. um comentário HTML usado como placeholder contava como valor
//     ("ADR: <!-- preencher depois -->" → true, campo de fato vazio).
//
// Ambos se resolvem tratando a detecção por LINHA em vez de por substring do documento inteiro:
// o marcador só conta quando é a primeira coisa não-branca da linha (tolera espaços/tabs à
// esquerda, não tolera qualquer outra decoração — "> ", "- ", "# ", "**" não contam como início).
// Medido antes de decidir (não assumido): nas quatro pastas realmente varridas por estas 4
// regras (REQ files para ADR:/Roadmap:; roadmaps de wip/blocked para REQ:), a contagem de linhas
// com decoração diferente de espaço em branco antes do marcador é ZERO — os três geradores
// canônicos sempre emitem o marcador no início da linha. Restringir a espaço/tab não introduz
// falso-positivo novo neste acervo.
//
// NÃO resolve (fora de escopo desta ML): "REQ (**reaberta**):" — decoração ENTRE o nome do
// campo e ":", não ANTES do marcador. A literal "REQ:" não aparece nessa linha nem por
// substring nem por prefixo; ancorar por linha não causa nem cura esse caso. A forma que
// funciona hoje é "REQ: docs/req/x.md (reaberta)" — o marcador intacto, a anotação depois.
//
// "none", "TBD", "N/A", "-" continuam sendo tratados como VALOR (não como placeholder). Nenhum
// dos quatro casos exigidos por esta ML os menciona, e "none" é usado deliberadamente em
// scripts/check-gates-falsify.sh como placeholder inofensivo — para dizer "sem vínculo real,
// de propósito" nos fixtures que testam OUTRA regra — sem ficar vazio nem virar comentário
// HTML. Bloqueá-los moveria a contagem do acervo por um motivo alheio ao achado A2 e quebraria
// cenários de falsificação que dependem desse contrato.
//
// issue #278 (ML-1B): 5 de 7 grafias naturais de campo vazio escapavam da checagem anterior
// por literal — esta função decide por VALOR (TrimSpace do resto da linha), o que já cobre as 7
// grafias medidas na triagem original.
func contentHasMarkerValue(content string, markers []string) bool {
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		leading := strings.TrimLeft(line, " \t")
		for _, marker := range markers {
			if !strings.HasPrefix(leading, marker) {
				continue
			}
			rest := strings.TrimSpace(leading[len(marker):])
			if rest == "" {
				continue
			}
			if isHTMLCommentOnlyValue(rest) {
				continue
			}
			return true
		}
	}
	return false
}

// isHTMLCommentOnlyValue retorna true se value (já sem espaços nas duas pontas) é inteiramente
// um comentário HTML — "<!-- ... -->" do início ao fim, sem nada fora dele. É a forma medida no
// achado A2: "ADR: <!-- preencher depois -->" é sintaticamente um valor, mas semanticamente um
// placeholder de "ainda não preenchido" — mesma intenção de um campo vazio, só que disfarçado.
func isHTMLCommentOnlyValue(value string) bool {
	return strings.HasPrefix(value, "<!--") && strings.HasSuffix(value, "-->")
}

// ruleDefaults mapeia regras cujo default NÃO é "error".
// Regras ausentes deste mapa usam "error" como default.
var ruleDefaults = map[string]string{
	"note_orphan": "warning",
	// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A,
	// ADR-2026-08-12 Emenda 3: the script carries no version marker, so this rule cannot tell
	// legitimate drift (trackfw not updated yet) from tampering — kept a warning, never an error.
	// credential_guard_mode_downgrade is deliberately absent from this map: it falls through to
	// ruleSeverity's "error" default (see validator_credential_guard_integrity.go for why).
	"credential_guard_script_integrity": "warning",
	// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
	// desatualizados, ML-1A: same rationale as credential_guard_script_integrity above —
	// scripts/trackfw-git-branch-guard.sh carries no version marker either, so this rule cannot
	// tell legitimate drift from tampering. git_branch_guard_hook_resolvable is deliberately
	// absent from this map (falls through to "error"), mirroring credential_guard_hook_resolvable.
	"git_branch_guard_script_integrity": "warning",
	// ML-4A (REQ-2026-08-29, achado 1 do parecer hades-tf 2026-08-30): namespace oculto/ambíguo
	// (nome iniciado por ".") é sinal de baixo ruído por natureza — pode ser um namespace legítimo
	// escolhido deliberadamente, não um defeito de configuração como agent_namespace_undeclared
	// (que fica "error"). Nunca "off" por default: silêncio total é exatamente o defeito que esta
	// REQ existe para fechar.
	"agent_namespace_hidden": "warning",
	// ML-1A (AC9, REQ nasce órfã): divergência entre frontmatter `roadmap:` e corpo `Roadmap:`
	// quando ambos estão preenchidos mas apontam para basenames diferentes. Estado diferente
	// (wip→done após roadmap move) não dispara esta regra. Default "warning" porque a divergência
	// pode ser transitória (frontmatter já atualizado, body ainda com path antigo).
	"req_roadmap_sync": "warning",
	// roadmap_wave0_required, roadmap_gate_coverage, roadmap_duplicate_label are absent:
	// they fall through to "error" (ML-4A, REQ #392 — all pre-existing sítios sanitized).
}

// ruleSeverity retorna a severidade configurada para a regra.
// Prioridade (quando anchor disponível): stricter-of(origin/main, disk) > ruleDefaults > "error".
// Prioridade (quando anchor ausente/inaplicável): trackfw.yaml rules: > ruleDefaults > "error".
//
// ADR-2026-09-17 ML-1A: ancoragem generalizada de origin/main para TODAS as regras, via
// currentOriginMain (set at top of each Validate* call by loadOriginMainAnchor). Substitui o
// padrão HEAD-vs-disco anterior (ADR-2026-08-12), que era vácuo em CI (HEAD == disco para edições
// commitadas, medido na Wave 0).
//
// Quatro estados de currentOriginMain — regras de comportamento:
//
//   originAnchorNotSet / originAnchorNoGit / originAnchorNoRemote / originAnchorFileAbsent:
//     disk only — diskRuleSeverity, idêntico ao comportamento pré-ADR-2026-08-12 para todas as
//     ~38 regras. "Not set" é o valor zero, válido fora de Validate* (e.g. testes que chamam
//     ruleSeverity diretamente).
//
//   originAnchorRefUnreadable:
//     FAIL CLOSED — ignora o bloco rules: do disco inteiramente, retorna o default built-in da
//     regra (credentialGuardDefaultSeverity). Previne bypass via `rules: {<regra>: off}` commitado
//     quando o âncora não pode ser verificada. Uma única mensagem de violação é emitida no topo de
//     ValidateUnfiltered / validateUnfilteredTagged — não aqui, para evitar N mensagens.
//
//   originAnchorOK:
//     stricter-wins — stricter of(origin/main severity, disk severity). A mais estrita vence;
//     subir a severidade no disco É respeitado (o critério é "mais estrita vence", não
//     "origin/main sempre vence").
func ruleSeverity(name string) string {
	switch currentOriginMain.state {
	case originAnchorRefUnreadable:
		// ML-1B (Defect 3): return stricter of built-in default and disk (not default-only).
		// Disk can strengthen (raise severity above default), but cannot weaken below default.
		// This allows a repo to escalate a rule from "warning" to "error" even when the anchor
		// is absent, while still preventing bypass via `rules: {<name>: off}` in the same commit.
		return credentialGuardStricterSeverity(credentialGuardDefaultSeverity(name), diskRuleSeverity(name))
	case originAnchorOK:
		diskSev := diskRuleSeverity(name)
		mainSev, ok := currentOriginMain.rules[name]
		if !ok {
			// Rule absent in origin/main's rules: block — use built-in default, which is
			// already the strictest value diskRuleSeverity would fall back to, so disk can
			// only equal or lose this comparison, never win it against the default alone.
			mainSev = credentialGuardDefaultSeverity(name)
		}
		return credentialGuardStricterSeverity(mainSev, diskSev)
	default:
		// originAnchorNotSet, originAnchorNoGit, originAnchorNoRemote, originAnchorFileAbsent:
		// disk only — same as pre-ADR-2026-08-12 for all rules.
		return diskRuleSeverity(name)
	}
}

// diskRuleSeverity is the ordinary, disk-only resolution used by every rule except the 3
// credential-guard rules in credentialGuardAnchoredRules: trackfw.yaml rules: (CWD) > ruleDefaults
// > "error". This is the entire body ruleSeverity had before ADR-2026-08-12 introduced the
// HEAD-anchored branch above — unchanged behavior, only renamed so both callers can share it.
func diskRuleSeverity(name string) string {
	cfg := config.Load()
	if s, ok := cfg.Rules[name]; ok {
		return s
	}
	if d, ok := ruleDefaults[name]; ok {
		return d
	}
	return "error"
}

// scopeRedirectViolations implements the ML-2C scope-redirect guard (AC5 and AC8(c) of
// REQ-2026-09-17-leniencia-sem-prazo). ML-2C replaces the ML-2B discriminant ("disk dir is
// empty") with "coverage loss": artifacts committed in origin/main under the anchor path that
// are no longer visible in any disk governance scope.
//
// Why coverage loss instead of emptiness (ML-2B finding):
// An attacker can create a facade file in the redirected directory (one line, any name) to
// defeat an emptiness check. The correct signal is: did a PR make previously-committed
// governance artifacts invisible to the validation engine? That question can only be answered
// by comparing against the git tree, not the working tree at the anchor path.
//
// Baseline — git tree, not disk at anchor path:
// mdBasenamesInGitTree(ref, anchorDir) reads origin/main's committed files via
// `git ls-tree -r --name-only <ref> -- <dir>`. This is the only source that survives
// "delete-and-redirect": an attacker who removes the original files from disk AND creates a
// facade still loses — origin/main still lists the deleted basenames.
//
// Disk side — union of ALL disk governance scopes:
// A legitimate restructuring (e.g. merging docs/req and docs/adr into docs/governance/) must
// not fire. The test is "visible anywhere in governance", not "visible at the new path of the
// same kind". Union of diskCfg.REQDir ∪ diskCfg.RoadmapDir ∪ diskCfg.ADRDirs provides this.
//
// Known narrow case (declared here, not hidden):
// A PR that legitimately DELETES an artifact AND changes paths in the same commit will fire.
// The violation names the deleted artifact; the remedy is either "don't rename dirs when also
// deleting artifacts" or "accept the named violation via baseline".
//
// State mapping (unchanged from ML-1A/ML-2B):
//   - originAnchorOK: compare git-tree basenames vs disk union; lost basenames → violation.
//   - all other states: no baseline → no violation.
//
// Violations are appended DIRECTLY (not via applyRule/applyRuleTagged) so they cannot be
// silenced by rules: {<name>: off} in the same PR.
func scopeRedirectViolations(diskCfg *config.ProjectConfig) []string {
	if currentOriginMain.state != originAnchorOK || currentOriginMain.dirs == nil {
		return nil
	}
	anchor := currentOriginMain.dirs
	ref := currentOriginMain.ref

	// Item 4 (ML-2C hardening): ref must be non-empty to run git ls-tree. This path is
	// theoretically unreachable (state == originAnchorOK guarantees deriveOriginDefaultBranch
	// returned ok == true), but an explicit guard prevents silent pass-through if invariants
	// ever change — empty ref would silently produce zero anchor basenames, passing every check.
	if ref == "" {
		return nil
	}

	// Collect all anchor governance dirs: used to filter the disk-side union (see below).
	allAnchorDirs := []string{anchor.reqDir, anchor.roadmapDir}
	allAnchorDirs = append(allAnchorDirs, anchor.adrDirs...)

	// Build the disk-side union once: every .md file visible in any disk governance scope.
	// Roots that are strict ancestors of any anchor dir are EXCLUDED from the walk — see
	// mdBasenamesOnDisk for the rationale (defends against roadmap_dir: . and similar).
	diskUnionRoots := make([]string, 0, 2+len(diskCfg.ADRDirs))
	diskUnionRoots = append(diskUnionRoots, diskCfg.REQDir, diskCfg.RoadmapDir)
	diskUnionRoots = append(diskUnionRoots, diskCfg.ADRDirs...)
	diskBasenames := mdBasenamesOnDisk(diskUnionRoots, allAnchorDirs)

	const maxNamed = 5 // max artifact names per violation message; report count for the rest
	var out []string

	// req_dir: check coverage when the configured path changed.
	if filepath.Clean(diskCfg.REQDir) != filepath.Clean(anchor.reqDir) {
		anchorBasenames := mdBasenamesInGitTree(ref, anchor.reqDir)
		if lost := scopeLostArtifacts(anchorBasenames, diskBasenames); len(lost) > 0 {
			named, suffix := formatLostArtifacts(lost, maxNamed)
			out = append(out, fmt.Sprintf(
				"scope redirect: req_dir changed from %q (origin/main) to %q (disk) — "+
					"%d artifact(s) committed in origin/main under the original path are no "+
					"longer visible in any governance directory on disk: %s%s. "+
					"Move the artifacts to the new path before changing req_dir, "+
					"or revert the path change.",
				anchor.reqDir, diskCfg.REQDir, len(lost), named, suffix))
		}
	}

	// roadmap_dir: check coverage when the configured path changed.
	if filepath.Clean(diskCfg.RoadmapDir) != filepath.Clean(anchor.roadmapDir) {
		anchorBasenames := mdBasenamesInGitTree(ref, anchor.roadmapDir)
		if lost := scopeLostArtifacts(anchorBasenames, diskBasenames); len(lost) > 0 {
			named, suffix := formatLostArtifacts(lost, maxNamed)
			out = append(out, fmt.Sprintf(
				"scope redirect: roadmap_dir changed from %q (origin/main) to %q (disk) — "+
					"%d artifact(s) committed in origin/main under the original path are no "+
					"longer visible in any governance directory on disk: %s%s. "+
					"Move the artifacts to the new path before changing roadmap_dir, "+
					"or revert the path change.",
				anchor.roadmapDir, diskCfg.RoadmapDir, len(lost), named, suffix))
		}
	}

	// adr_dirs: check coverage when the set of configured paths changed.
	if !stringSlicesSameSet(anchor.adrDirs, diskCfg.ADRDirs) {
		anchorBasenames := make(map[string]bool)
		for _, d := range anchor.adrDirs {
			for k := range mdBasenamesInGitTree(ref, d) {
				anchorBasenames[k] = true
			}
		}
		if lost := scopeLostArtifacts(anchorBasenames, diskBasenames); len(lost) > 0 {
			named, suffix := formatLostArtifacts(lost, maxNamed)
			out = append(out, fmt.Sprintf(
				"scope redirect: adr_dirs changed from %v (origin/main) to %v (disk) — "+
					"%d artifact(s) committed in origin/main under the original paths are no "+
					"longer visible in any governance directory on disk: %s%s. "+
					"Move the artifacts to the new paths before changing adr_dirs, "+
					"or revert the path change.",
				anchor.adrDirs, diskCfg.ADRDirs, len(lost), named, suffix))
		}
	}

	return out
}

// mdBasenamesInGitTree returns the set of .md file basenames committed in the given git ref
// under the given directory prefix. Uses the same gitCommand wrapper as originMainTrackfwYAML.
// Returns an empty (non-nil) set on error so callers can do set subtraction safely.
func mdBasenamesInGitTree(ref, dirPrefix string) map[string]bool {
	out, err := gitCommand(".", "ls-tree", "-r", "--name-only", ref, "--", dirPrefix).Output()
	set := make(map[string]bool)
	if err != nil {
		return set
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if filepath.Ext(line) == ".md" {
			set[filepath.Base(line)] = true
		}
	}
	return set
}

// mdBasenamesOnDisk returns the union of .md file basenames found by recursively walking the
// provided root directories on the local disk. Missing or unreadable directories are skipped.
//
// Roots are expanded via config.ExpandPath (handles ~/... prefixes in adr_dirs).
//
// Ancestor filter (ML-2C hardening): a disk root that is a STRICT ancestor of any anchor dir
// is excluded from the walk. Without this, an attacker who sets `roadmap_dir: .` causes
// WalkDir(".") to reach the original governance files (docs/req/*, etc.), making all anchor
// basenames appear "visible" in the disk union and defeating the coverage-loss check entirely.
// The filter uses strict ancestry (rel != "." prevents excluding roots equal to anchor dirs).
func mdBasenamesOnDisk(roots []string, anchorDirs []string) map[string]bool {
	set := make(map[string]bool)
	for _, root := range roots {
		expanded := config.ExpandPath(root)
		eClean := filepath.Clean(expanded)
		// Exclude roots that are strict ancestors of any anchor dir.
		isStrictAncestor := false
		for _, aDir := range anchorDirs {
			aClean := filepath.Clean(config.ExpandPath(aDir))
			rel, err := filepath.Rel(eClean, aClean)
			if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				isStrictAncestor = true
				break
			}
		}
		if isStrictAncestor {
			continue
		}
		_ = filepath.WalkDir(expanded, func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if filepath.Ext(d.Name()) == ".md" {
				set[d.Name()] = true
			}
			return nil
		})
	}
	return set
}

// scopeLostArtifacts returns basenames present in anchorSet but absent from diskSet,
// as a sorted slice for deterministic violation messages.
func scopeLostArtifacts(anchorSet, diskSet map[string]bool) []string {
	var lost []string
	for name := range anchorSet {
		if !diskSet[name] {
			lost = append(lost, name)
		}
	}
	sort.Strings(lost)
	return lost
}

// formatLostArtifacts formats the first max names from lost into a comma-separated string,
// appending a " (and N more)" suffix when len(lost) > max.
func formatLostArtifacts(lost []string, max int) (named, suffix string) {
	if len(lost) <= max {
		return strings.Join(lost, ", "), ""
	}
	return strings.Join(lost[:max], ", "), fmt.Sprintf(" (and %d more)", len(lost)-max)
}

// stringSlicesSameSet returns true if a and b contain the same elements after filepath.Clean,
// ignoring order and duplicates. Used by tests to compare adr_dirs sets.
func stringSlicesSameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	setA := make(map[string]bool, len(a))
	for _, s := range a {
		setA[filepath.Clean(s)] = true
	}
	for _, s := range b {
		if !setA[filepath.Clean(s)] {
			return false
		}
	}
	return true
}

// applyRule distribui msgs conforme severidade da regra.
// "off" → silencioso; "warning" → warnings; default ("error") → violations.
func applyRule(ruleName string, msgs []string, violations, warnings *[]string) {
	if len(msgs) == 0 {
		return
	}
	switch ruleSeverity(ruleName) {
	case "off":
		// silencioso
	case "warning":
		*warnings = append(*warnings, msgs...)
	default:
		*violations = append(*violations, msgs...)
	}
}

// applyRuleTagged é idêntico a applyRule mas acumula TaggedMsg (rule+msg) em vez de []string.
// Usado por ValidateTagged para propagar o nome da regra até o BuildResultTagged.
func applyRuleTagged(ruleName string, msgs []string, violations, warnings *[]TaggedMsg) {
	if len(msgs) == 0 {
		return
	}
	tagged := make([]TaggedMsg, len(msgs))
	for i, m := range msgs {
		tagged[i] = TaggedMsg{Rule: ruleName, Msg: m}
	}
	switch ruleSeverity(ruleName) {
	case "off":
		// silencioso
	case "warning":
		*warnings = append(*warnings, tagged...)
	default:
		*violations = append(*violations, tagged...)
	}
}

// lenientCarveoutRules is the named, closed set of rules that lenient mode never silences.
// Criterion: the rule detects a contradiction between live artifacts (not a historical absence).
// These rules signal active inconsistency in the governance chain — suppressing them defeats
// the purpose of governance even during a transition period.
//
// Calibration (2026-09-17): req_roadmap_lifecycle + ref_targets_exist = 8 active violations
// in this repo / 0 historical. All other rules = 0 active / 169 historical.
// A rule is added here only when its violations cannot be historical debt.
var lenientCarveoutRules = map[string]bool{
	"req_roadmap_lifecycle": true,
	"ref_targets_exist":     true,
}

// applyLenientWithCarveout moves non-carve-out violations to warnings (lenient mode) while
// keeping carve-out violations as violations. Rules in lenientCarveoutRules detect contradictions
// between live artifacts and must not be silenced during lenient transitions.
// Violations tagged with Rule "" (e.g. frontmatter_presence) are not in the carve-out and move
// to warnings — this is correct because they represent structural gaps, not live contradictions.
func applyLenientWithCarveout(violations, warnings []TaggedMsg) ([]TaggedMsg, []TaggedMsg) {
	var kept []TaggedMsg
	for _, v := range violations {
		if lenientCarveoutRules[v.Rule] {
			kept = append(kept, v)
		} else {
			warnings = append(warnings, v)
		}
	}
	return kept, warnings
}

// WIPConfig armazena configuração de WIP limit derivada do config.ProjectConfig já carregado.
type WIPConfig struct {
	Limit   int  // default 1
	BySquad bool // default false
}

// wipConfigFrom deriva WIPConfig a partir do ProjectConfig já normalizado por config.Load() —
// nenhuma releitura de trackfw.yaml acontece aqui. cfg.WipLimit já traz o default 1 aplicado
// pelo loader quando o campo está ausente ou é <= 0 (config.parseInt cai no default nesse caso).
func wipConfigFrom(cfg config.ProjectConfig) WIPConfig {
	return WIPConfig{Limit: cfg.WipLimit, BySquad: cfg.WipBySquad}
}

// parseSquadFromFrontmatter lê um arquivo markdown e extrai o valor da linha "squad: <valor>".
// Retorna string vazia se o campo está ausente ou vazio.
func parseSquadFromFrontmatter(path string) string {
	content, err := readRegularFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "squad:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "squad:"))
			return val
		}
	}
	return ""
}

// validateWIPLimit verifica o WIP limit — por agente, por squad ou global — conforme trackfw.yaml.
func validateWIPLimit() (violations []string, warnings []string, err error) {
	projectCfg := config.Load()

	if projectCfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := resolveAgentNamespaces(projectCfg, projectCfg.RoadmapDir)
		wipCfg := wipConfigFrom(projectCfg)
		for _, agent := range agents {
			// ML-4A (achado 2): agent vem do disco (resolveAgentNamespaces), não de config — usa
			// ListMDFiles em vez de filepath.Glob para não interpretar o nome como padrão.
			files := ListMDFiles(filepath.Join(projectCfg.RoadmapDir, agent, "wip"))
			if len(files) > wipCfg.Limit {
				warnings = append(warnings, fmt.Sprintf(
					"%d roadmaps in wip/ for agent %q (limit: %d) — consider focusing",
					len(files), agent, wipCfg.Limit,
				))
			}
		}
		return
	}

	files, globErr := filepath.Glob(filepath.Join(projectCfg.RoadmapDir, "wip", "*.md"))
	if globErr != nil {
		return nil, nil, globErr
	}

	wipCfg := wipConfigFrom(projectCfg)

	if !wipCfg.BySquad {
		if len(files) > wipCfg.Limit {
			warnings = append(warnings, fmt.Sprintf(
				"%d roadmaps in wip/ (limit: %d) — consider focusing",
				len(files), wipCfg.Limit,
			))
		}
		return
	}

	bySquad := map[string][]string{}
	for _, f := range files {
		squad := parseSquadFromFrontmatter(f)
		if squad == "" {
			squad = "(no squad)"
		}
		bySquad[squad] = append(bySquad[squad], filepath.Base(f))
	}
	for squad, items := range bySquad {
		if len(items) > wipCfg.Limit {
			warnings = append(warnings, fmt.Sprintf(
				"squad %q has %d roadmaps in wip/ (limit: %d)",
				squad, len(items), wipCfg.Limit,
			))
		}
	}
	return
}

// GovernanceMode armazena o modo de governança lido do trackfw.yaml.
type GovernanceMode struct {
	Mode         string    // "strict" (default) ou "lenient"
	LenientUntil time.Time // zero se strict ou sem data definida
}

// governanceModeFrom deriva GovernanceMode a partir do ProjectConfig já normalizado por
// config.Load() — nenhuma releitura de trackfw.yaml acontece aqui. cfg.GovernanceMode chega
// como o valor bruto do campo (string vazia se ausente); lenient_until chega como a data literal
// (ex.: "2026-08-02"), convertida aqui para time.Time.
func governanceModeFrom(cfg config.ProjectConfig) GovernanceMode {
	gm := GovernanceMode{Mode: "strict"}
	if cfg.GovernanceMode != "" {
		gm.Mode = cfg.GovernanceMode
	}
	if cfg.LenientUntil != "" {
		if t, err := time.Parse("2006-01-02", cfg.LenientUntil); err == nil {
			gm.LenientUntil = t
		}
	}
	return gm
}

// isLenientFor is the pure, clock-injectable core of the leniency check. It is the single
// place where all three rejection reasons live:
//
//   1. mode is not "lenient"
//   2. lenient_until is absent (zero time) — treats no-deadline as strict (AC2 fix;
//      pre-fix behaviour was to return true here — that was the bug closed by ML-2A)
//   3. lenient_until is in the past (deadline expired)
//   4. lenient_until is more than LenientHorizonDays days from now — treated the same
//      as absent (AC2 ceiling; prevents 9999-12-31 from granting forever-leniency)
//
// IsLenient() is the thin, config-reading wrapper around this function.
// Tests should call isLenientFor() directly to avoid clock flake.
func isLenientFor(mode string, until time.Time, now time.Time) bool {
	if mode != "lenient" {
		return false
	}
	if until.IsZero() {
		// AC2: absent lenient_until → strict. Pre-fix: this path returned true (bug).
		return false
	}
	horizon := now.AddDate(0, 0, config.LenientHorizonDays)
	if !until.Before(horizon) {
		// AC2 ceiling: date is at or beyond the horizon — treated same as absent.
		return false
	}
	return now.Before(until)
}

// IsLenient retorna true se o projeto está em modo lenient e o prazo está dentro do horizonte
// permitido (config.LenientHorizonDays dias a partir de agora) e ainda não expirou.
// Ausência de lenient_until → strict (não é mais leniente para sempre).
func IsLenient() bool {
	gm := governanceModeFrom(config.Load())
	return isLenientFor(gm.Mode, gm.LenientUntil, time.Now())
}

// LenientUntilDate retorna a data de expiração do modo lenient formatada em "2006-01-02".
// Retorna string vazia se o modo não for lenient, a data não estiver definida ou estiver além
// do horizonte permitido (config.LenientHorizonDays dias).
func LenientUntilDate() string {
	gm := governanceModeFrom(config.Load())
	if gm.Mode != "lenient" || gm.LenientUntil.IsZero() {
		return ""
	}
	horizon := time.Now().AddDate(0, 0, config.LenientHorizonDays)
	if !gm.LenientUntil.Before(horizon) {
		return ""
	}
	return gm.LenientUntil.Format("2006-01-02")
}

// ValidateUnfiltered executa todas as validações sem filtro de baseline nem modo lenient.
// Use para criar snapshots de baseline ou quando você quer o quadro completo.
func ValidateUnfiltered() (violations []string, warnings []string, err error) {
	// ML-1A (ROADMAP-2026-09-17): load origin/main severity anchor once per validate call.
	// ruleSeverity() reads currentOriginMain without taking a lock — acceptable because trackfw
	// is a CLI tool (one validate call per process). Must be set BEFORE config.Load() so that any
	// diskRuleSeverity call during the first applyRule already sees the correct anchor state.
	currentOriginMain = loadOriginMainAnchor()
	switch currentOriginMain.state {
	case originAnchorRefUnreadable:
		// ML-1B (Defect 3): changed from unconditional violation to warning.
		// Anchor unavailable is a configuration signal, not a blocking failure for repos that
		// never touch rules:. Bypass is still closed: ruleSeverity() returns stricter(default,disk)
		// for all rules in this state, so disk's `rules: {<name>: off}` cannot lower any rule
		// below its built-in default. A per-rule violation is emitted below for each rule that
		// the disk explicitly weakens below default (guarantees observable even with no findings).
		warnings = append(warnings, originMainRefUnreadableMessage())
	case originAnchorFileAbsent:
		// Informational: trackfw.yaml absent in origin/main — normal for a PR that adds the file.
		// DISTINCT message from originAnchorRefUnreadable (two-states-one-observable prevention).
		warnings = append(warnings, originMainFileAbsentMessage())
	}

	cfg := config.Load()

	// ML-2B: scope-redirect guard — detect req_dir/roadmap_dir/adr_dirs redirected to empty dirs.
	// Direct append (not via applyRule) so rules: {<name>: off} in the same PR cannot silence it.
	for _, msg := range scopeRedirectViolations(&cfg) {
		violations = append(violations, msg)
	}

	// ML-1B (Defect 3): when anchor is unavailable, emit a violation for each rule that the user
	// EXPLICITLY set in trackfw.yaml's rules: block AND whose explicit value is lower than its
	// built-in default. Uses ParseRulesFromContent (not cfg.Rules) to read only explicitly-set
	// rules — cfg.Rules includes config-package defaults (e.g. stale_wip:"warning") which would
	// produce false positives if the user never touched those rules.
	if currentOriginMain.state == originAnchorRefUnreadable {
		if rawContent, readErr := readRegularFile("trackfw.yaml"); readErr == nil {
			for ruleName, diskSev := range config.ParseRulesFromContent(string(rawContent)) {
				defSev := credentialGuardDefaultSeverity(ruleName)
				if credentialGuardSeverityRank(diskSev) < credentialGuardSeverityRank(defSev) {
					violations = append(violations, originMainAnchorWeakeningMessage(ruleName, diskSev, defSev))
				}
			}
		}
	}

	wipViolations, e := validateWIPHasREQ()
	if e != nil {
		return nil, nil, e
	}
	applyRule("wip_has_req", wipViolations, &violations, &warnings)

	reqViolations, e := validateREQsHaveADR()
	if e != nil {
		return nil, nil, e
	}
	applyRule("req_has_adr", reqViolations, &violations, &warnings)

	blockedViolations, e := validateBlockedHasREQ()
	if e != nil {
		return nil, nil, e
	}
	applyRule("blocked_has_req", blockedViolations, &violations, &warnings)

	reqRoadmapViolations, e := validateREQsHaveRoadmap()
	if e != nil {
		return nil, nil, e
	}
	applyRule("req_has_roadmap", reqRoadmapViolations, &violations, &warnings)

	reqRoadmapSyncWarns, e := validateREQRoadmapSync()
	if e != nil {
		return nil, nil, e
	}
	applyRule("req_roadmap_sync", reqRoadmapSyncWarns, &violations, &warnings)

	adrOrphanViolations, e := validateADRsAreReferenced()
	if e != nil {
		return nil, nil, e
	}
	applyRule("adr_orphan", adrOrphanViolations, &violations, &warnings)

	criteriaViolations, e := validateWIPHasAcceptanceCriteria()
	if e != nil {
		return nil, nil, e
	}
	applyRule("wip_acceptance", criteriaViolations, &violations, &warnings)

	wipViolationsLimit, wipWarningsLimit, e := validateWIPLimit()
	if e != nil {
		return nil, nil, e
	}
	applyRule("wip_limit", wipViolationsLimit, &violations, &warnings)
	warnings = append(warnings, wipWarningsLimit...) // warnings de limite não têm severidade configurável

	staleWarnings, e := validateStaleWIP()
	if e != nil {
		return nil, nil, e
	}
	applyRule("stale_wip", staleWarnings, &violations, &warnings)

	draftBlockedViolations, e := validateREQsNotBlockedByDraftADRs()
	if e != nil {
		return nil, nil, e
	}
	applyRule("blocked_by_draft_adr", draftBlockedViolations, &violations, &warnings)

	adrAcceptedViolations, e := validateADRAcceptedWhenREQDone()
	if e != nil {
		return nil, nil, e
	}
	applyRule("adr_accepted_when_req_done", adrAcceptedViolations, &violations, &warnings)

	frontmatterViolations := validateFrontmatterPresence()
	violations = append(violations, frontmatterViolations...) // sem regra configurável

	refWarnings, e := validateRefTargetsExist()
	if e != nil {
		return nil, nil, e
	}
	applyRule("ref_targets_exist", refWarnings, &violations, &warnings)

	reqLifecycleWarnings, e := validateREQRoadmapLifecycle()
	if e != nil {
		return nil, nil, e
	}
	// Ação 1: route through applyRule so that rules: {req_roadmap_lifecycle: error}
	// can promote this to a violation. Pre-fix: direct append to warnings bypassed ruleSeverity.
	applyRule("req_roadmap_lifecycle", reqLifecycleWarnings, &violations, &warnings)

	coherenceWarnings, e := validateFolderStatusCoherence()
	if e != nil {
		return nil, nil, e
	}
	applyRule("folder_status", coherenceWarnings, &violations, &warnings)

	uniquenessViolations, e := validateFilenameUniqueness()
	if e != nil {
		return nil, nil, e
	}
	applyRule("filename_uniqueness", uniquenessViolations, &violations, &warnings)

	branchViolations, e := validateBranchHasWIPRoadmap()
	if e != nil {
		return nil, nil, e
	}
	applyRule("branch_has_wip_roadmap", branchViolations, &violations, &warnings)

	noteOrphanMsgs, e := validateNoteOrphan()
	if e != nil {
		return nil, nil, e
	}
	applyRule("note_orphan", noteOrphanMsgs, &violations, &warnings)

	// v2.5: verificação bidirecional REQ↔Roadmap via trace_id_field (desativada se campo vazio)
	traceViolations, traceWarnings := validateTraceId(cfg)
	violations = append(violations, traceViolations...)
	warnings = append(warnings, traceWarnings...)

	// ML-2A: validação de existência dos diretórios adr_dirs (Warning por padrão, Error se strict_ci_paths)
	adrDirViolations, adrDirWarnings := validateADRDirsExist(cfg)
	violations = append(violations, adrDirViolations...)
	warnings = append(warnings, adrDirWarnings...)

	// ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A: controle positivo —
	// detecta hook de credential-guard registrado cujo script não existe ou não é executável.
	// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
	// desatualizados, ML-1A: soma as mensagens de escopo GLOBAL sob a MESMA regra — ver o
	// comentário sobre os 4 wrappers em validator_git_branch_guard.go.
	credentialGuardHookMsgs, e := validateCredentialGuardHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	credentialGuardGlobalHookMsgs, e := validateCredentialGuardGlobalHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	applyRule("credential_guard_hook_resolvable", append(credentialGuardHookMsgs, credentialGuardGlobalHookMsgs...), &violations, &warnings)

	// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A:
	// detecta adulteração do credential-guard, âncora por alvo (ADR-2026-08-12 Emenda 1).
	credentialGuardScriptMsgs, e := validateCredentialGuardScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	credentialGuardGlobalScriptMsgs, e := validateCredentialGuardGlobalScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	applyRule("credential_guard_script_integrity", append(credentialGuardScriptMsgs, credentialGuardGlobalScriptMsgs...), &violations, &warnings)

	credentialGuardModeMsgs, e := validateCredentialGuardModeDowngrade()
	if e != nil {
		return nil, nil, e
	}
	applyRule("credential_guard_mode_downgrade", credentialGuardModeMsgs, &violations, &warnings)

	// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
	// desatualizados, ML-1A: mesma cobertura acima (existência/executabilidade + integridade,
	// projeto e global), generalizada para trackfw-git-branch-guard.sh.
	gitBranchGuardHookMsgs, e := validateGitBranchGuardHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	gitBranchGuardGlobalHookMsgs, e := validateGitBranchGuardGlobalHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	applyRule("git_branch_guard_hook_resolvable", append(gitBranchGuardHookMsgs, gitBranchGuardGlobalHookMsgs...), &violations, &warnings)

	gitBranchGuardScriptMsgs, e := validateGitBranchGuardScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	gitBranchGuardGlobalScriptMsgs, e := validateGitBranchGuardGlobalScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	applyRule("git_branch_guard_script_integrity", append(gitBranchGuardScriptMsgs, gitBranchGuardGlobalScriptMsgs...), &violations, &warnings)

	// ADR-2026-08-15-gate-de-duas-fases-..., ML-3A (D2): git-anchored detection behind the
	// TRACKFW_ORCHESTRATOR_SESSION guardrail — flags a third-party artifact claim with no
	// matching provenance entry, or a provenance entry whose checksum cannot be reconciled
	// against the installed content via its quarantine record.
	thirdPartyProvenanceMsgs, e := validateThirdPartyArtifactHasProvenance()
	if e != nil {
		return nil, nil, e
	}
	applyRule("thirdparty_artifact_has_provenance", thirdPartyProvenanceMsgs, &violations, &warnings)

	// ML-2A (REQ-2026-08-29): namespace de agente em disco e não declarado em agents: — violação,
	// não aviso (ver comentário em validateAgentNamespaceUndeclared).
	agentNamespaceMsgs := validateAgentNamespaceUndeclared()
	applyRule("agent_namespace_undeclared", agentNamespaceMsgs, &violations, &warnings)

	// ML-4A (achado 1, hades-tf 2026-08-30): contraponto de baixo ruído para nomes ocultos/ambíguos
	// (iniciados por ".") — aviso, nunca silêncio total, nunca erro (rule default abaixo).
	hiddenNamespaceMsgs := hiddenNamespaceWarnings()
	applyRule("agent_namespace_hidden", hiddenNamespaceMsgs, &violations, &warnings)

	// ML-3B/ML-4B (REQ #392): gate coverage (AC7), Wave 0 required (AC7-bis), duplicate labels (AC8).
	// wave0_required + gate_coverage: wip/ only (ML-4B narrowed from wip+blocked — see
	// validator_roadmap_gates.go header for the retroactivity argument).
	// duplicate_label: wip/ + blocked/ (structural error, not convention-age-sensitive).
	// None applies to backlog/analyzing (decision 6-bis) or done/ (decision 7/8).
	wave0Msgs, gateMsgs, dupMsgs := validateRoadmapGatesCoverage()
	applyRule("roadmap_wave0_required", wave0Msgs, &violations, &warnings)
	applyRule("roadmap_gate_coverage", gateMsgs, &violations, &warnings)
	applyRule("roadmap_duplicate_label", dupMsgs, &violations, &warnings)

	return violations, warnings, nil
}

// Validate executes ValidateUnfiltered's tagged twin (validateUnfilteredTagged), applies the
// baseline filter WITH the credential-guard carve-out (see filterBaselineTagged), then strips the
// Rule tags to preserve this function's plain-[]string signature.
//
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-
// e-disco: before this ADR, Validate() filtered the plain []string returned by ValidateUnfiltered
// directly — no rule name attached to each message, so a per-rule baseline carve-out was not
// expressible here at all. Routing through validateUnfilteredTagged (already used by
// ValidateTagged) is what lets both entry points share one baseline-filtering implementation
// instead of the two independently-maintained copies that predated this change — see
// filterBaselineTagged's doc comment for the carve-out itself. validateUnfilteredTagged emits the
// exact same rule checks, in the exact same order, as ValidateUnfiltered (see that function's own
// doc comment) — this refactor does not change Validate()'s output for any message the
// credential-guard carve-out does not apply to.
func Validate() (violations []string, warnings []string, err error) {
	taggedViolations, taggedWarnings, err := validateUnfilteredTagged()
	if err != nil {
		return nil, nil, err
	}

	taggedViolations, taggedWarnings, err = filterBaselineTagged(taggedViolations, taggedWarnings)
	if err != nil {
		return nil, nil, err
	}

	// AC2/AC4: apply lenient carve-out in the tagged domain BEFORE untagging, so that
	// lenientCarveoutRules can be matched by rule name. Non-carve-out violations become warnings;
	// carve-out violations (req_roadmap_lifecycle, ref_targets_exist) remain violations.
	if IsLenient() {
		taggedViolations, taggedWarnings = applyLenientWithCarveout(taggedViolations, taggedWarnings)
	}

	violations = untagMsgs(taggedViolations)
	warnings = untagMsgs(taggedWarnings)

	return
}

// filterBaselineTagged applies the baseline (ratchet) filter shared by Validate() and
// ValidateTagged(), with a carve-out: a violation/warning tagged with one of the 3 rule names in
// credentialGuardAnchoredRules is NEVER tolerated by baseline, regardless of what
// .trackfw-baseline.json contains for it.
//
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-
// e-disco: this is a DIFFERENT mechanism from the HEAD-vs-disk comparison in
// credentialGuardRuleSeverity, deliberately — .trackfw-baseline.json is .gitignore'd on purpose
// (see .gitignore, "baseline local de violations toleradas (nao versionado)"), so there is no HEAD
// copy of it to compare against; "require a commit" simply does not apply to a file the project
// decided never to version. The only closure for this channel is to exclude these 3 rule names
// from ratchet eligibility outright, independent of message content.
func filterBaselineTagged(violations, warnings []TaggedMsg) ([]TaggedMsg, []TaggedMsg, error) {
	baseline, bErr := LoadBaseline()
	if bErr != nil {
		return nil, nil, fmt.Errorf("erro ao carregar baseline: %w", bErr)
	}
	if baseline == nil {
		return violations, warnings, nil
	}

	baselineSet := make(map[string]struct{}, len(baseline.Violations))
	for _, v := range baseline.Violations {
		baselineSet[v] = struct{}{}
	}
	var netNew []TaggedMsg
	for _, v := range violations {
		_, tolerated := baselineSet[v.Msg]
		if tolerated && !credentialGuardAnchoredRules[v.Rule] {
			continue
		}
		netNew = append(netNew, v)
	}

	warnSet := make(map[string]struct{}, len(baseline.Warnings))
	for _, w := range baseline.Warnings {
		warnSet[w] = struct{}{}
	}
	var netNewWarn []TaggedMsg
	for _, w := range warnings {
		_, tolerated := warnSet[w.Msg]
		if tolerated && !credentialGuardAnchoredRules[w.Rule] {
			continue
		}
		netNewWarn = append(netNewWarn, w)
	}

	return netNew, netNewWarn, nil
}

// untagMsgs strips the Rule tag off each TaggedMsg, preserving order. Used by Validate() to
// recover its plain []string signature after routing through the tagged pipeline.
func untagMsgs(tagged []TaggedMsg) []string {
	if len(tagged) == 0 {
		return nil
	}
	out := make([]string, len(tagged))
	for i, t := range tagged {
		out[i] = t.Msg
	}
	return out
}

// validateUnfilteredTagged é a versão interna de ValidateUnfiltered que retorna TaggedMsg.
// Regras sem applyRuleTagged (diretas) ficam com Rule="" — comportamento intencional.
func validateUnfilteredTagged() (violations []TaggedMsg, warnings []TaggedMsg, err error) {
	// ML-1A (ROADMAP-2026-09-17): load origin/main severity anchor once per validate call.
	// Mirrors the same block in ValidateUnfiltered — both are entry points into the validation
	// engine (ValidateUnfiltered is called directly, validateUnfilteredTagged by Validate and
	// ValidateTagged). If they were called in the same process, the second call overwrites
	// currentOriginMain — acceptable because the anchor result is deterministic for a given CWD.
	currentOriginMain = loadOriginMainAnchor()
	const anchorRule = "origin_main_anchor"
	switch currentOriginMain.state {
	case originAnchorRefUnreadable:
		// ML-1B (Defect 3): warning, not violation — mirrors ValidateUnfiltered change.
		warnings = append(warnings, TaggedMsg{Rule: anchorRule, Msg: originMainRefUnreadableMessage()})
	case originAnchorFileAbsent:
		warnings = append(warnings, TaggedMsg{Rule: anchorRule, Msg: originMainFileAbsentMessage()})
	}

	cfg := config.Load()

	// ML-2B: scope-redirect guard — mirrors ValidateUnfiltered. Rule: "" (direct, not via applyRuleTagged).
	for _, msg := range scopeRedirectViolations(&cfg) {
		violations = append(violations, TaggedMsg{Rule: "", Msg: msg})
	}

	// ML-1B (Defect 3): per-rule weakening violations when anchor unavailable — mirrors ValidateUnfiltered.
	if currentOriginMain.state == originAnchorRefUnreadable {
		if rawContent, readErr := readRegularFile("trackfw.yaml"); readErr == nil {
			for ruleName, diskSev := range config.ParseRulesFromContent(string(rawContent)) {
				defSev := credentialGuardDefaultSeverity(ruleName)
				if credentialGuardSeverityRank(diskSev) < credentialGuardSeverityRank(defSev) {
					violations = append(violations, TaggedMsg{Rule: anchorRule, Msg: originMainAnchorWeakeningMessage(ruleName, diskSev, defSev)})
				}
			}
		}
	}

	wipViolations, e := validateWIPHasREQ()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("wip_has_req", wipViolations, &violations, &warnings)

	reqViolations, e := validateREQsHaveADR()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("req_has_adr", reqViolations, &violations, &warnings)

	blockedViolations, e := validateBlockedHasREQ()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("blocked_has_req", blockedViolations, &violations, &warnings)

	reqRoadmapViolations, e := validateREQsHaveRoadmap()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("req_has_roadmap", reqRoadmapViolations, &violations, &warnings)

	reqRoadmapSyncWarnsT, e := validateREQRoadmapSync()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("req_roadmap_sync", reqRoadmapSyncWarnsT, &violations, &warnings)

	adrOrphanViolations, e := validateADRsAreReferenced()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("adr_orphan", adrOrphanViolations, &violations, &warnings)

	criteriaViolations, e := validateWIPHasAcceptanceCriteria()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("wip_acceptance", criteriaViolations, &violations, &warnings)

	wipViolationsLimit, wipWarningsLimit, e := validateWIPLimit()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("wip_limit", wipViolationsLimit, &violations, &warnings)
	for _, m := range wipWarningsLimit {
		warnings = append(warnings, TaggedMsg{Rule: "wip_limit", Msg: m})
	}

	staleWarnings, e := validateStaleWIP()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("stale_wip", staleWarnings, &violations, &warnings)

	draftBlockedViolations, e := validateREQsNotBlockedByDraftADRs()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("blocked_by_draft_adr", draftBlockedViolations, &violations, &warnings)

	adrAcceptedViolations, e := validateADRAcceptedWhenREQDone()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("adr_accepted_when_req_done", adrAcceptedViolations, &violations, &warnings)

	frontmatterViolations := validateFrontmatterPresence()
	for _, m := range frontmatterViolations {
		violations = append(violations, TaggedMsg{Rule: "", Msg: m})
	}

	refWarnings, e := validateRefTargetsExist()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("ref_targets_exist", refWarnings, &violations, &warnings)

	reqLifecycleWarnings, e := validateREQRoadmapLifecycle()
	if e != nil {
		return nil, nil, e
	}
	// Ação 1 (tagged site): route through applyRuleTagged so that rules:
	// {req_roadmap_lifecycle: error} can promote to violation. Pre-fix: manual append to
	// warnings bypassed ruleSeverity — the rule could never become a violation under any config.
	applyRuleTagged("req_roadmap_lifecycle", reqLifecycleWarnings, &violations, &warnings)

	coherenceWarnings, e := validateFolderStatusCoherence()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("folder_status", coherenceWarnings, &violations, &warnings)

	uniquenessViolations, e := validateFilenameUniqueness()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("filename_uniqueness", uniquenessViolations, &violations, &warnings)

	branchViolationsT, e := validateBranchHasWIPRoadmap()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("branch_has_wip_roadmap", branchViolationsT, &violations, &warnings)

	noteOrphanMsgsT, e := validateNoteOrphan()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("note_orphan", noteOrphanMsgsT, &violations, &warnings)

	// v2.5: traceid — applyRuleTagged está no validator_traceid via applyRule; aqui fazemos tagged
	traceViolations, traceWarnings := validateTraceId(cfg)
	for _, m := range traceViolations {
		violations = append(violations, TaggedMsg{Rule: extractRulePrefix(m), Msg: m})
	}
	for _, m := range traceWarnings {
		warnings = append(warnings, TaggedMsg{Rule: extractRulePrefix(m), Msg: m})
	}

	// ML-2A: validação de existência dos diretórios adr_dirs (Warning por padrão, Error se strict_ci_paths)
	adrDirViolations, adrDirWarnings := validateADRDirsExist(cfg)
	for _, m := range adrDirViolations {
		violations = append(violations, TaggedMsg{Rule: "adr_dir_exists", Msg: m})
	}
	for _, m := range adrDirWarnings {
		warnings = append(warnings, TaggedMsg{Rule: "adr_dir_exists", Msg: m})
	}

	// ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A: controle positivo —
	// detecta hook de credential-guard registrado cujo script não existe ou não é executável.
	// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
	// desatualizados, ML-1A: soma as mensagens de escopo GLOBAL sob a MESMA regra — ver o
	// comentário sobre os 4 wrappers em validator_git_branch_guard.go.
	credentialGuardHookMsgsT, e := validateCredentialGuardHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	credentialGuardGlobalHookMsgsT, e := validateCredentialGuardGlobalHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("credential_guard_hook_resolvable", append(credentialGuardHookMsgsT, credentialGuardGlobalHookMsgsT...), &violations, &warnings)

	// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A:
	// detecta adulteração do credential-guard, âncora por alvo (ADR-2026-08-12 Emenda 1).
	credentialGuardScriptMsgsT, e := validateCredentialGuardScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	credentialGuardGlobalScriptMsgsT, e := validateCredentialGuardGlobalScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("credential_guard_script_integrity", append(credentialGuardScriptMsgsT, credentialGuardGlobalScriptMsgsT...), &violations, &warnings)

	credentialGuardModeMsgsT, e := validateCredentialGuardModeDowngrade()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("credential_guard_mode_downgrade", credentialGuardModeMsgsT, &violations, &warnings)

	// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
	// desatualizados, ML-1A: mesma cobertura acima (existência/executabilidade + integridade,
	// projeto e global), generalizada para trackfw-git-branch-guard.sh.
	gitBranchGuardHookMsgsT, e := validateGitBranchGuardHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	gitBranchGuardGlobalHookMsgsT, e := validateGitBranchGuardGlobalHookResolvable()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("git_branch_guard_hook_resolvable", append(gitBranchGuardHookMsgsT, gitBranchGuardGlobalHookMsgsT...), &violations, &warnings)

	gitBranchGuardScriptMsgsT, e := validateGitBranchGuardScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	gitBranchGuardGlobalScriptMsgsT, e := validateGitBranchGuardGlobalScriptIntegrity()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("git_branch_guard_script_integrity", append(gitBranchGuardScriptMsgsT, gitBranchGuardGlobalScriptMsgsT...), &violations, &warnings)

	thirdPartyProvenanceMsgsT, e := validateThirdPartyArtifactHasProvenance()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("thirdparty_artifact_has_provenance", thirdPartyProvenanceMsgsT, &violations, &warnings)

	// ML-2A (REQ-2026-08-29): namespace de agente em disco e não declarado em agents:.
	agentNamespaceMsgsT := validateAgentNamespaceUndeclared()
	applyRuleTagged("agent_namespace_undeclared", agentNamespaceMsgsT, &violations, &warnings)

	// ML-4A (achado 1, hades-tf 2026-08-30): contraponto de baixo ruído para nomes ocultos/ambíguos
	// (iniciados por ".") — aviso, nunca silêncio total, nunca erro (rule default abaixo).
	hiddenNamespaceMsgsT := hiddenNamespaceWarnings()
	applyRuleTagged("agent_namespace_hidden", hiddenNamespaceMsgsT, &violations, &warnings)

	// ML-3B/ML-4B (REQ #392): mirror of ValidateUnfiltered block — same rules, same order.
	// 🔴 Forgetting this Tagged site makes the rules vanish from --json without a compile error.
	wave0MsgsT, gateMsgsT, dupMsgsT := validateRoadmapGatesCoverage()
	applyRuleTagged("roadmap_wave0_required", wave0MsgsT, &violations, &warnings)
	applyRuleTagged("roadmap_gate_coverage", gateMsgsT, &violations, &warnings)
	applyRuleTagged("roadmap_duplicate_label", dupMsgsT, &violations, &warnings)

	return violations, warnings, nil
}

// extractRulePrefix extrai o prefixo "traceid_*" das mensagens de rastreabilidade.
// Retorna a substring antes do primeiro ":" se ela tiver prefixo "traceid_", senão "".
func extractRulePrefix(msg string) string {
	colonIdx := -1
	for i, c := range msg {
		if c == ':' {
			colonIdx = i
			break
		}
	}
	if colonIdx <= 0 {
		return ""
	}
	prefix := msg[:colonIdx]
	if len(prefix) > 8 && prefix[:8] == "traceid_" {
		return prefix
	}
	return ""
}

// ValidateTagged executa toda a validação retornando TaggedMsg com Rule+Msg preenchidos.
// Aplica filtro de baseline e modo lenient igual a Validate().
// Use para --json onde rule e file precisam estar preenchidos.
func ValidateTagged() (violations []TaggedMsg, warnings []TaggedMsg, err error) {
	violations, warnings, err = validateUnfilteredTagged()
	if err != nil {
		return
	}

	// Filtro de baseline (com carve-out de credential-guard) — ver filterBaselineTagged.
	violations, warnings, err = filterBaselineTagged(violations, warnings)
	if err != nil {
		return nil, nil, err
	}

	// AC2/AC4: apply lenient carve-out. Carve-out violations remain violations;
	// non-carve-out violations become warnings; exit code 0 only if no carve-out violations.
	if IsLenient() {
		violations, warnings = applyLenientWithCarveout(violations, warnings)
	}

	return
}

// inventoryBlock monta a seção "📊 Inventory" exibida no topo de `trackfw status`,
// somando o total de ADRs, REQs (discriminadas por status real) e roadmaps (pelos
// 6 estados de pasta, incluindo analyzing — antes ausente de qualquer contagem).
// Roadmaps são somados via resolveStateDirs, que já resolve namespacing flat/by_agent,
// para que a contagem funcione igual nos dois modos.
func inventoryBlock(cfg config.ProjectConfig) string {
	var sb strings.Builder

	adrCount := 0
	for _, adrDir := range cfg.ADRDirs {
		adrCount += len(walkADRFilePaths(adrDir))
	}

	reqFiles, _ := resolveREQFiles(cfg) // inventory: err → 0 counts, safe
	var reqOpen, reqDone, reqClosed, reqOther int
	for _, p := range reqFiles {
		content, err := readRegularFile(p)
		if err != nil {
			continue
		}
		switch strings.ToLower(reqStatusValue(string(content))) {
		case "open":
			reqOpen++
		case "done":
			reqDone++
		case "closed":
			reqClosed++
		default:
			// Toda grafia fora de open/done/closed cai aqui. Sem este bucket o total
			// bate e a quebra some com a diferenca EM SILENCIO: um acervo com
			// approved/backlog/abandoned mostra "53 (8 Open · 36 Done · 0 Closed)" sem
			// indicar que 9 existem e nao estao em lugar nenhum da conta.
			// O Python ja fazia isto (pypi/trackfw/commands/status.py:58,199).
			reqOther++
		}
	}

	states := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
	roadmapCounts := make(map[string]int, len(states))
	roadmapTotal := 0
	for _, state := range states {
		total := 0
		for _, dir := range resolveStateDirs(cfg, state) {
			entries, _ := listDir(dir)
			total += len(entries)
		}
		roadmapCounts[state] = total
		roadmapTotal += total
	}

	sb.WriteString("\n📊 Inventory\n")
	sb.WriteString(fmt.Sprintf("   %-12s%d\n", "ADRs", adrCount))
	reqDetail := fmt.Sprintf("%d Open · %d Done · %d Closed", reqOpen, reqDone, reqClosed)
	if reqOther > 0 {
		reqDetail += fmt.Sprintf(" · %d Other", reqOther)
	}
	sb.WriteString(fmt.Sprintf("   %-12s%d  (%s)\n", "REQs", len(reqFiles), reqDetail))
	sb.WriteString(fmt.Sprintf("   %-12s%d\n", "Roadmaps", roadmapTotal))
	sb.WriteString(fmt.Sprintf("     backlog %d · analyzing %d · wip %d\n", roadmapCounts["backlog"], roadmapCounts["analyzing"], roadmapCounts["wip"]))
	sb.WriteString(fmt.Sprintf("     blocked %d · done %d · abandoned %d\n", roadmapCounts["blocked"], roadmapCounts["done"], roadmapCounts["abandoned"]))

	return sb.String()
}

func GetStatus() (string, error) {
	cfg := config.Load()
	var sb strings.Builder
	sb.WriteString("── trackfw status ──────────────────────\n")

	sb.WriteString(inventoryBlock(cfg))

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := resolveAgentNamespaces(cfg, cfg.RoadmapDir)
		sb.WriteString("\n⚙ WIP by Agent\n")
		for _, agent := range agents {
			wip, _ := listDir(cfg.RoadmapDir + "/" + agent + "/wip")
			if len(wip) > 0 {
				sb.WriteString(fmt.Sprintf("  [%s] WIP (%d)\n", agent, len(wip)))
				for _, f := range wip {
					sb.WriteString(fmt.Sprintf("    %s\n", f))
				}
			}
		}
	} else {
		wip, _ := listDir(cfg.RoadmapDir + "/wip")
		blocked, _ := listDir(cfg.RoadmapDir + "/blocked")
		done, _ := listDir(cfg.RoadmapDir + "/done")

		sb.WriteString(fmt.Sprintf("\n🔄 WIP (%d)\n", len(wip)))
		for _, f := range wip {
			sb.WriteString(fmt.Sprintf("   %s\n", f))
		}

		wipCfg := wipConfigFrom(cfg)
		if wipCfg.BySquad && len(wip) > 0 {
			bySquad := map[string]int{}
			for _, f := range wip {
				squad := parseSquadFromFrontmatter(filepath.Join(cfg.RoadmapDir, "wip", f))
				if squad == "" {
					squad = "(no squad)"
				}
				bySquad[squad]++
			}
			sb.WriteString(fmt.Sprintf("\n⚙ WIP by Squad (limit: %d per squad)\n", wipCfg.Limit))
			for squad, count := range bySquad {
				status := "✓"
				if count > wipCfg.Limit {
					status = "⚠"
				}
				noun := "roadmap"
				if count > 1 {
					noun = "roadmaps"
				}
				sb.WriteString(fmt.Sprintf("   %-20s %d %s  %s\n", squad+":", count, noun, status))
			}
		}

		sb.WriteString(fmt.Sprintf("\n❌ Blocked (%d)\n", len(blocked)))
		for _, f := range blocked {
			sb.WriteString(fmt.Sprintf("   %s\n", f))
		}

		// Seção: stale WIP
		staleWIPs, _ := validateStaleWIP()
		if len(staleWIPs) > 0 {
			sb.WriteString(fmt.Sprintf("\n⚠  Stale WIP (%d)\n", len(staleWIPs)))
			for _, w := range staleWIPs {
				parts := strings.Fields(w)
				if len(parts) > 0 {
					sb.WriteString(fmt.Sprintf("   %s\n", w))
				}
			}
		}

		// Seção: REQs bloqueadas por ADRs não aceitos (Draft ou Proposed). O status exibido
		// por ADR é resolvido via adrStatusForRule (helper canônico) em vez de hardcodar
		// "Draft" — blockedREQs() cobre ambos os status desde que passou a delegar em
		// adrDraftStatusForRule, e um rótulo fixo "(Draft)" mentiria para um ADR Proposed.
		blockedByDraft, err := blockedREQs()
		if err == nil && len(blockedByDraft) > 0 {
			sb.WriteString(fmt.Sprintf("\n⏳ REQs blocked by not-accepted ADRs (%d)\n", len(blockedByDraft)))
			for reqFile, adrs := range blockedByDraft {
				sb.WriteString(fmt.Sprintf("   %s\n", reqFile))
				for _, adr := range adrs {
					status, _ := adrStatusForRule("blocked_by_draft_adr", adr, nil)
					sb.WriteString(fmt.Sprintf("     → %s (%s)\n", adr, status))
				}
			}
		}

		sb.WriteString(fmt.Sprintf("\n✅ Done (last 5)\n"))
		last5 := done
		if len(last5) > 5 {
			last5 = last5[len(last5)-5:]
		}
		for _, f := range last5 {
			sb.WriteString(fmt.Sprintf("   %s\n", f))
		}
	}

	sb.WriteString("\n────────────────────────────────────────\n")
	return sb.String(), nil
}

// resolveAgentNamespaces é o resolvedor canônico de namespaces em modo by_agent — o ÚNICO lugar do
// pacote onde a lista `agents:` do trackfw.yaml é lida ao lado do disco. Devolve a UNIÃO entre
// cfg.Agents (na ordem declarada, deduplicada) e os subdiretórios de primeiro nível encontrados em
// dir (ordenados). Todo outro ponto do pacote que precisar enumerar agentes DEVE chamar esta função
// — nunca reimplementar "if len(agents) == 0 { ler disco }": esse padrão SUBSTITUÍA o disco em vez de
// complementá-lo, deixando invisível qualquer namespace em disco não declarado em agents:
// (REQ-2026-08-29). O padrão `len(agents) == 0` só pode existir aqui dentro.
//
// Segurança — NÃO segue symlink (AC12/AC13, bloqueante): usa os.ReadDir + entry.IsDir(), que reflete
// o tipo da própria entrada do diretório (via Lstat interno), não o alvo do link — um namespace que é
// symlink para fora do projeto nunca é tratado como diretório aqui. NÃO trocar por os.Stat("simplificação"):
// os.Stat segue symlink e reintroduziria o vetor que hoje escreve fora da árvore em Node/Python
// (ver ADR-2026-08-29, decisão 5, e vault/notes/update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28.md).
func resolveAgentNamespaces(cfg config.ProjectConfig, dir string) []string {
	seen := make(map[string]bool, len(cfg.Agents))
	ordered := make([]string, 0, len(cfg.Agents))
	for _, a := range cfg.Agents {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		ordered = append(ordered, a)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ordered
	}
	var fromDisk []string
	for _, e := range entries {
		if !e.IsDir() { // symlinks reportam false aqui — nunca seguidos (AC12/AC13)
			continue
		}
		if isInfraDirName(e.Name()) { // ML-2A: nunca vira namespace, ver comentário na função
			continue
		}
		fromDisk = append(fromDisk, e.Name())
	}
	sort.Strings(fromDisk)
	for _, name := range fromDisk {
		if seen[name] {
			continue
		}
		seen[name] = true
		ordered = append(ordered, name)
	}
	return ordered
}

// isInfraDirName decide, no ponto único de leitura de disco do resolvedor, se uma entrada é
// COMPROVADAMENTE infraestrutura e nunca um namespace de agente — filtrada da união (decisão 1 do
// ADR) e portanto invisível a todo consumidor (validate, status, move, wip limit...).
//
// CORREÇÃO (ML-4A, achado 1 do parecer hades-tf 2026-08-30, REPROVA original): esta lista já incluiu
// "qualquer nome iniciando com '.'". Isso reabria, byte-a-byte, o defeito que a REQ existe para
// fechar (cmdb: artefato de governança invisível, `validate` reportando limpo sobre o que nunca
// enumerou) — só que atrás de um ponto no nome em vez de um `agents:` incompleto. Um namespace real
// batizado ".ghost" desaparecia de union, status, wip limit e `move` sem nenhum sinal, e virava canal
// de ocultação deliberada para quem quisesse esconder trabalho da governança.
//
// A lista fechada agora tem UMA entrada, cada uma justificada:
//   - "node_modules": artefato de tooling JS (npm/yarn/pnpm). Nenhum operador digita isto como nome
//     de agente por acidente ou por design — é ruído inequívoco, sem a ambiguidade de um nome
//     iniciado por ponto (que pode ser um namespace legítimo escolhido deliberadamente). Seguro
//     excluir até da enumeração.
//
// Nomes iniciados por "." (ex.: ".git", ".trackfw", ".ghost") NÃO são mais filtrados aqui — dentro de
// roadmap_dir/req_dir praticamente não existe diretório de infraestrutura real (".git" fica na raiz
// do repositório, não em roadmap_dir); o benefício do filtro é pequeno e o custo — um canal de
// ocultação silenciosa — é alto. Esses nomes continuam ENTRANDO na união (nunca invisíveis), mas são
// tratados como um caso ambíguo, não como violação plena: ver isDotPrefixedName e
// hiddenNamespaceWarnings, que rebaixam o sinal para um aviso de baixo ruído nomeando o diretório —
// nunca zero sinal.
//
// Um diretório cujo nome colide com um nome de estado (ex.: "wip" órfão no topo de roadmap_dir)
// continua entrando na união normalmente; só é excluído da VIOLAÇÃO (não do resolvedor) em
// validateAgentNamespaceUndeclared, porque ali a informação de que é um nome de estado reservado é o
// sinal relevante, não um sinal de "não é diretório real".
func isInfraDirName(name string) bool {
	return name == "node_modules"
}

// isDotPrefixedName reporta se name começa com "." — um sinal ambíguo (pode ser um namespace
// legítimo, ou resto de tooling que nunca deveria ter parado dentro de roadmap_dir/req_dir), não mais
// um filtro de invisibilidade (ver isInfraDirName). Usado só para rebaixar o sinal de
// "não declarado em agents:" de violação para aviso — nunca para remover o nome da união.
func isDotPrefixedName(name string) bool {
	return strings.HasPrefix(name, ".")
}

// agentNamespaceStateNames replica, só para esta regra, os 6 nomes de estado reservados de roadmap/
// REQ — já repetidos como literal em outros pontos deste arquivo (ex.: validateFolderStatusCoherence,
// validateFilenameUniqueness); não existe hoje uma constante compartilhada no pacote validator, e
// introduzir uma só para este ML alargaria o escopo. Um diretório com um desses nomes no topo de
// roadmap_dir/req_dir é, na prática, resto de migração incompleta flat→by_agent (ex.: "wip" órfão) —
// não um agente. A união (decisão 1 do ADR) continua enumerando esses diretórios normalmente — nada
// fica invisível —, mas eles NÃO disparam validateAgentNamespaceUndeclared: pedir para declarar "wip"
// como agente em agents: seria ruído confuso, não uma correção real (ML-0A, seção 3, item 3;
// recomendação adotada sem alteração). Esta exclusão vive só aqui, não no resolvedor — a colisão de
// nome não é "comprovadamente infraestrutura" como isInfraDirName, é uma inferência sobre o
// significado do nome, então só afeta a violação, nunca a união/enumeração.
var agentNamespaceStateNames = map[string]bool{
	"backlog": true, "analyzing": true, "wip": true, "blocked": true, "done": true, "abandoned": true,
}

// undeclaredNamespacesOnDisk devolve, a partir do resolvedor canônico (que já filtra infra e não
// segue symlink), os nomes de namespace presentes em dir e ausentes de agents:, excluindo colisões
// com nome de estado reservado (agentNamespaceStateNames) e nomes iniciados por "." (ML-4A: esses
// continuam na união — resolveAgentNamespaces não os filtra mais — mas não disparam a violação plena;
// ver hiddenNamespaceWarnings para o aviso de baixo ruído que os substitui).
func undeclaredNamespacesOnDisk(cfg config.ProjectConfig, dir string, declared map[string]bool) []string {
	var out []string
	for _, name := range resolveAgentNamespaces(cfg, dir) {
		if declared[name] || agentNamespaceStateNames[name] || isDotPrefixedName(name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// dotPrefixedUndeclaredOnDisk é o espelho de undeclaredNamespacesOnDisk para o caso ambíguo (nome
// iniciado por "."): mesmo resolvedor canônico, mesma exclusão de nomes já declarados, mas mantendo
// (em vez de excluir) exatamente os nomes que undeclaredNamespacesOnDisk descarta por causa do ponto.
func dotPrefixedUndeclaredOnDisk(cfg config.ProjectConfig, dir string, declared map[string]bool) []string {
	var out []string
	for _, name := range resolveAgentNamespaces(cfg, dir) {
		if declared[name] || !isDotPrefixedName(name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// validateAgentNamespaceUndeclared implementa a regra "agent_namespace_undeclared"
// (ADR-2026-08-29, decisão 2 / REQ AC4, AC5, AC9): em modo by_agent, um namespace presente em disco
// (roadmap_dir e/ou req_dir — AC2 estende a união às duas árvores, e esta violação segue) e ausente
// de agents: é VIOLAÇÃO, não aviso — é defeito de configuração que escondeu artefatos de governança
// (ver ADR, "a correção é de uma linha"), e usa o mesmo default "error" de toda regra sem entrada em
// ruleDefaults (diskRuleSeverity).
//
// A união já garante (Wave 1) que o namespace continua sendo ENUMERADO por todo consumidor mesmo com
// esta violação ativa — esta função só ADICIONA o sinal de configuração incompleta, nunca CONDICIONA
// a enumeração a ele (AC5-b).
//
// Deduplicação por namespace, não por árvore: o caso motivador (cmdb, "zeus" ausente de agents: e em
// disco em roadmap_dir E req_dir ao mesmo tempo) produziria duas violações quase-idênticas se o
// laço fosse por árvore — ruído no caso comum, não no caso raro. Uma violação por nome, nomeando
// todas as árvores onde ele foi encontrado.
func validateAgentNamespaceUndeclared() []string {
	cfg := config.Load()
	if cfg.RoadmapNamespacing != config.NamespacingByAgent {
		return nil
	}
	declared := make(map[string]bool, len(cfg.Agents))
	for _, a := range cfg.Agents {
		declared[a] = true
	}

	roadmapNames := undeclaredNamespacesOnDisk(cfg, cfg.RoadmapDir, declared)
	reqNames := undeclaredNamespacesOnDisk(cfg, cfg.REQDir, declared)

	inRoadmap := make(map[string]bool, len(roadmapNames))
	for _, n := range roadmapNames {
		inRoadmap[n] = true
	}
	inReq := make(map[string]bool, len(reqNames))
	for _, n := range reqNames {
		inReq[n] = true
	}

	seen := make(map[string]bool, len(roadmapNames)+len(reqNames))
	var names []string
	for _, n := range roadmapNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, n := range reqNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)

	var msgs []string
	for _, name := range names {
		var trees []string
		if inRoadmap[name] {
			trees = append(trees, "roadmap_dir")
		}
		if inReq[name] {
			trees = append(trees, "req_dir")
		}
		msgs = append(msgs, fmt.Sprintf(
			"agent namespace \"%s\" exists in %s but is not declared in agents: — add it to trackfw.yaml",
			name, strings.Join(trees, ", "),
		))
	}
	return msgs
}

// hiddenNamespaceWarnings implementa a regra "agent_namespace_hidden" — o contraponto de baixo ruído
// de validateAgentNamespaceUndeclared para nomes iniciados por "." (ML-4A, achado 1 do parecer
// hades-tf 2026-08-30). Um diretório oculto/ambíguo em disco (roadmap_dir e/ou req_dir), ausente de
// agents:, NÃO é filtrado da união (resolveAgentNamespaces mantém — nunca fica invisível a nenhum
// consumidor) e NÃO dispara a violação plena (undeclaredNamespacesOnDisk descarta esses nomes) — mas
// também não é silêncio total: esta função emite um aviso nomeando explicitamente o diretório, para
// que quem estiver olhando `validate` perceba o namespace ambíguo e decida — declará-lo em agents: ou
// removê-lo, se for só resto de tooling.
func hiddenNamespaceWarnings() []string {
	cfg := config.Load()
	if cfg.RoadmapNamespacing != config.NamespacingByAgent {
		return nil
	}
	declared := make(map[string]bool, len(cfg.Agents))
	for _, a := range cfg.Agents {
		declared[a] = true
	}

	roadmapNames := dotPrefixedUndeclaredOnDisk(cfg, cfg.RoadmapDir, declared)
	reqNames := dotPrefixedUndeclaredOnDisk(cfg, cfg.REQDir, declared)

	inRoadmap := make(map[string]bool, len(roadmapNames))
	for _, n := range roadmapNames {
		inRoadmap[n] = true
	}
	inReq := make(map[string]bool, len(reqNames))
	for _, n := range reqNames {
		inReq[n] = true
	}

	seen := make(map[string]bool, len(roadmapNames)+len(reqNames))
	var names []string
	for _, n := range roadmapNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, n := range reqNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)

	var msgs []string
	for _, name := range names {
		var trees []string
		if inRoadmap[name] {
			trees = append(trees, "roadmap_dir")
		}
		if inReq[name] {
			trees = append(trees, "req_dir")
		}
		msgs = append(msgs, fmt.Sprintf(
			"dot-prefixed directory %q found in %s is treated as an agent namespace (fully enumerated, not declared in agents:) — declare it in trackfw.yaml if intentional, or remove it if it is leftover tooling",
			name, strings.Join(trees, ", "),
		))
	}
	return msgs
}

// resolveStateDirs retorna todos os diretórios de um estado (ex: "wip", "done") conforme o modo de
// namespacing. É a fonte única de resolução de caminho por estado — resolveWIPDirs e resolveDoneDirs
// são wrappers finos sobre esta função. Duplicar a lógica aqui foi a causa raiz de defeitos
// anteriores (roadmap_dir divergente entre runtimes).
func resolveStateDirs(cfg config.ProjectConfig, state string) []string {
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := resolveAgentNamespaces(cfg, cfg.RoadmapDir)
		var dirs []string
		for _, agent := range agents {
			dirs = append(dirs, cfg.RoadmapDir+"/"+agent+"/"+state)
		}
		return dirs
	}
	return []string{cfg.RoadmapDir + "/" + state}
}

// resolveWIPDirs retorna todos os diretórios wip/ conforme o modo de namespacing.
func resolveWIPDirs(cfg config.ProjectConfig) []string {
	return resolveStateDirs(cfg, "wip")
}

// resolveDoneDirs retorna todos os diretórios done/ conforme o modo de namespacing.
func resolveDoneDirs(cfg config.ProjectConfig) []string {
	return resolveStateDirs(cfg, "done")
}

// ResolveWIPDirs é o wrapper exportado de resolveWIPDirs, usado por consumidores fora do
// pacote validator (ex: comando `trackfw branch new`).
func ResolveWIPDirs(cfg config.ProjectConfig) []string {
	return resolveWIPDirs(cfg)
}

// ResolveDoneDirs é o wrapper exportado de resolveDoneDirs, usado por consumidores fora do
// pacote validator (ex: comando `trackfw branch new`).
func ResolveDoneDirs(cfg config.ProjectConfig) []string {
	return resolveDoneDirs(cfg)
}

// ListMDFiles lista os arquivos .md diretamente dentro de dir (sem subdiretórios, sem glob) —
// substitui filepath.Glob(filepath.Join(dir, "*.md")) em todo ponto onde um COMPONENTE do caminho
// vem de um nome de diretório lido do disco (ex.: um namespace de agente resolvido por
// resolveAgentNamespaces), em vez de vir de config ou de uma constante do código.
//
// CORREÇÃO (ML-4A, achado 2 do parecer hades-tf 2026-08-30, REPROVA original): antes da união
// (Wave 1 desta REQ), `agent` só vinha de string digitada em `agents:` pelo operador. A união faz
// qualquer nome de diretório em disco chegar ao mesmo `filepath.Glob` sem validação de formato —
// diferente de nome de arquivo de REQ/roadmap, que precisa casar TYPE-YYYY-MM-DD-slug.md antes de
// entrar em qualquer regra. Um namespace literalmente chamado "*" fazia o padrão
// "roadmap_dir/*/wip/*.md" (com "*" no lugar do nome do agente) casar com o wip/ de TODOS os
// namespaces, inflando a contagem de WIP daquele agente silenciosamente com arquivos de outros —
// número plausível e errado, não ruído perceptível. Um namespace com um "[" desbalanceado
// (path/filepath.Glob não escapa metacaracteres) derrubava `validate` inteiro com um ErrBadPattern
// cru, inclusive vazando texto puro no canal --json (a chamada de Marshal nunca era alcançada).
//
// os.ReadDir não tem etapa de casamento de padrão para explorar: cada nome retornado é comparado ao
// conteúdo real de dir pelo sistema de arquivos, nunca interpretado como um padrão — não há
// diferença de comportamento entre um nome de diretório comum e um que contenha "*", "?", "[" ou "\".
func ListMDFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files
}

// ResolveAgentNamespaces é o wrapper exportado de resolveAgentNamespaces — o resolvedor canônico
// de namespaces em modo by_agent (união entre agents: e o disco, sem seguir symlink — ver o
// comentário da função não-exportada). Consumido por internal/generators e internal/serve, que não
// podem importar o pacote internal/validator sem cuidado por causa de import cycles pré-existentes
// (generators/context.go já importa validator; validator não pode importar generators de volta).
func ResolveAgentNamespaces(cfg config.ProjectConfig, dir string) []string {
	return resolveAgentNamespaces(cfg, dir)
}

// reqLayoutStates é a lista fechada de nomes de pasta de ESTADO reconhecidos nos layouts LEGADOS de
// REQ. Ela existe apenas para o leitor tolerar árvores antigas: pelo invariante D1 da
// ADR-2026-09-03, REQ NÃO tem dimensão de estado — backlog/analyzing/wip/blocked/done/abandoned são
// conceito de roadmap. Nada aqui deve ser usado para ESCREVER REQ.
var reqLayoutStates = []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}

// ResolveWriteAgent resolve o namespace de agente para escrita em modo by_agent.
//
//   - Em modo flat: retorna ("", nil) — o chamador escreve na raiz, sem subdirectório de agente.
//   - flagAgent não-vazio: retorna-o sem validação contra agents:. O valor fora de agents: cria o
//     namespace e dispara a violação agent_namespace_undeclared da REQ irmã (AC5b).
//   - by_agent + exatamente um namespace não-vazio em agents:: retorna aquele, sem necessidade de flag.
//   - by_agent + zero namespaces não-vazios: retorna ("default", nil) — comportamento de fallback.
//   - by_agent + dois ou mais namespaces não-vazios e sem flag: retorna erro nomeando as opções.
//
// Nomes vazios em agents: não contam — mesma noção de REQWriteDir e agentStateDir (filtrar, não indexar).
func ResolveWriteAgent(cfg config.ProjectConfig, flagAgent string) (string, error) {
	if cfg.RoadmapNamespacing != config.NamespacingByAgent {
		return "", nil
	}
	if flagAgent != "" {
		return flagAgent, nil
	}
	// Filtrar nomes vazios — mesma convenção do lado leitor (resolveAgentNamespaces).
	var nonEmpty []string
	for _, a := range cfg.Agents {
		if a != "" {
			nonEmpty = append(nonEmpty, a)
		}
	}
	switch len(nonEmpty) {
	case 0:
		return "default", nil
	case 1:
		return nonEmpty[0], nil
	default:
		return "", fmt.Errorf(
			"by_agent project has multiple agent namespaces (%s): use --agent to specify one",
			strings.Join(nonEmpty, ", "),
		)
	}
}

// IsMultiAgentByAgent retorna true quando cfg usa namespacing by_agent com 2 ou mais namespaces
// de agente não-vazios — a condição que exige --agent em `req new` / `roadmap new`.
// Retorna (false, nil) para flat ou by_agent com 0–1 agente.
// Segundo valor: lista dos agentes ativos (para mensagens de orientação).
func IsMultiAgentByAgent(cfg config.ProjectConfig) (bool, []string) {
	if cfg.RoadmapNamespacing != config.NamespacingByAgent {
		return false, nil
	}
	var nonEmpty []string
	for _, a := range cfg.Agents {
		if a != "" {
			nonEmpty = append(nonEmpty, a)
		}
	}
	if len(nonEmpty) < 2 {
		return false, nonEmpty
	}
	return true, nonEmpty
}

// ReqNewLine retorna a linha de comando `trackfw req new` correta para o projeto.
// Para projetos flat ou by_agent com 1 agente: forma canônica sem --agent.
// Para projetos by_agent com 2+ agentes: inclui --agent com a lista de agentes disponíveis.
func ReqNewLine(cfg config.ProjectConfig) string {
	multi, agents := IsMultiAgentByAgent(cfg)
	if multi {
		return fmt.Sprintf("trackfw req new --agent <agent> \"title\"  # agents: %s", strings.Join(agents, ", "))
	}
	return "trackfw req new \"title\""
}

// RoadmapNewLine retorna a linha de comando `trackfw roadmap new` correta para o projeto.
func RoadmapNewLine(cfg config.ProjectConfig) string {
	multi, _ := IsMultiAgentByAgent(cfg)
	if multi {
		return "trackfw roadmap new --agent <agent> \"title\""
	}
	return "trackfw roadmap new \"title\""
}

// REQWriteDir é o PONTO ÚNICO que decide ONDE uma REQ nova é gravada (ADR-2026-09-03, D2/D4):
//   - flat      → req_dir/
//   - by_agent  → req_dir/<agente>/   (agente pré-resolvido pelo chamador via ResolveWriteAgent)
//
// 🔴 O par escritor/leitor não pode ter duas noções de layout (D4). Este ponto e ResolveREQFiles
// abaixo são consumidos pelos DOIS lados; a união de leitura contém, por construção, o diretório
// devolvido aqui. Alterar um sem o outro é exatamente o defeito que a REQ-2026-08-30 fecha.
//
// agent deve ser o valor já resolvido por ResolveWriteAgent — não chamar esta função com ""
// em modo by_agent (o fallback interno existe apenas para compatibilidade de chamadores antigos).
func REQWriteDir(cfg config.ProjectConfig, agent string) string {
	reqDir := cfg.REQDir
	if reqDir == "" {
		return ""
	}
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		if agent == "" {
			// Fallback interno: filtrar vazios, mesmo que ResolveWriteAgent já deva ter resolvido.
			// S5 (hades-tf 2026-09-03): FILTRAR os vazios, não testar só o índice 0.
			for _, a := range cfg.Agents {
				if a != "" {
					agent = a
					break
				}
			}
			if agent == "" {
				agent = "default"
			}
		}
		return filepath.Join(reqDir, agent)
	}
	return reqDir
}

// ResolveREQFiles é o PONTO ÚNICO de LEITURA de REQ (ADR-2026-09-03, D3/D4): devolve os paths de
// todos os .md de REQ como UNIÃO dos 4 layouts suportados, nunca como escolha exclusiva entre eles:
//
//	req_dir/*.md                     flat legado
//	req_dir/<estado>/*.md            por-estado legado (apesar de D1)
//	req_dir/<agente>/*.md            CANÔNICO em by_agent
//	req_dir/<agente>/<estado>/*.md   legado
//
// Os dois últimos só se aplicam quando roadmap_namespacing == by_agent — fora dele não há noção de
// namespace de agente e um subdiretório qualquer não pode ser tratado como agente.
//
// 🔴 Layout canônico NÃO significa recusar os demais: a união é o que torna a migração de
// req_dir desnecessária (D3). Nenhum arquivo de ninguém é movido.
//
// 🔴 DEDUPLICAÇÃO É OBRIGATÓRIA, não higiene: resolveAgentNamespaces devolve agents: ∪ disco, então
// um req_dir/backlog/ real entra na lista de agentes e o caso <agente>/*.md emite exatamente os
// mesmos paths do caso <estado>/*.md. Sem o conjunto `seen`, toda REQ em layout por-estado seria
// contada duas vezes e cada violação apareceria em duplicata. Não resolver isso filtrando nomes de
// estado da lista de agentes: um agente legitimamente chamado "done" existiria e sumiria.
func ResolveREQFiles(cfg config.ProjectConfig) ([]string, error) {
	reqDir := cfg.REQDir
	if reqDir == "" {
		return nil, nil
	}
	if err := checkREQDirContained(reqDir); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var files []string
	add := func(paths []string) {
		for _, p := range paths {
			clean := filepath.Clean(p)
			if seen[clean] {
				continue
			}
			seen[clean] = true
			files = append(files, clean)
		}
	}

	// 🔴 §4 (hades-tf 2026-09-03): a dedup por STRING não vê req_dir/Backlog ≡ req_dir/backlog em
	// filesystem case-INSENSITIVE (APFS, NTFS). O nome "Backlog" entra na lista de agentes pelo
	// disco e emite req_dir/Backlog/*.md; o laço de estados emite req_dir/backlog/*.md, hardcoded
	// em minúscula. Mesmo diretório, strings diferentes, `seen` cego: toda REQ contada em DOBRO e
	// cada violação emitida duas vezes (medido em APFS: 2 REQs e 4 violações para 1 arquivo real).
	// Verde no CI Linux, vermelho na máquina do dev.
	//
	// MECANISMO: só enumeramos um candidato de subdiretório se o nome existir VERBATIM na listagem
	// do pai. A grafia do disco é a autoridade — medimos o disco em vez de presumir a propriedade do
	// filesystem. Consequências que decidiram a escolha:
	//   - NÃO troca dupla contagem por SUPRESSÃO: em FS case-SENSITIVE, "Backlog" e "backlog" são
	//     dois diretórios reais e DISTINTOS, e o readdir lista os DOIS — ambos continuam
	//     enumerados. Normalizar por lowercase colapsaria os dois e suprimiria um arquivo real.
	//   - NÃO usa identidade de inode: Go não tem chave hasheável portátil de (dev,ino)
	//     (syscall.Stat_t não existe no Windows, e os.SameFile é par-a-par → O(n²)); e ino == 0 em
	//     alguns FS de rede/Windows colapsaria arquivos distintos — supressão, a direção proibida.
	//   - Cobre também o eixo NFC/NFD, que o case-folding não cobre: a grafia do disco é a
	//     autoridade, então um agents: em outra forma Unicode é filtrado, não duplicado.
	//   - FALLBACK É JOIN CEGO, NUNCA LISTA VAZIA: se o pai não pode ser lido, não filtramos nada e
	//     voltamos ao comportamento anterior (dupla contagem, benigna). Devolver vazio aqui seria
	//     supressão.
	//   - Não filtra por TIPO de entrada: um <estado> que seja symlink continua sendo enumerado
	//     exatamente como antes (fechar essa porta é decisão do §3, fora deste microlote).
	childCache := make(map[string][]string)
	hasChildVerbatim := func(parent, name string) bool {
		children, cached := childCache[parent]
		if !cached {
			entries, err := os.ReadDir(parent)
			if err != nil {
				childCache[parent] = nil // marca "ilegível" → join cego
				return true
			}
			children = make([]string, 0, len(entries))
			for _, e := range entries {
				children = append(children, e.Name())
			}
			childCache[parent] = children
		}
		if children == nil {
			return true
		}
		for _, n := range children {
			if n == name {
				return true
			}
		}
		return false
	}
	addChild := func(parent, name string) {
		if !hasChildVerbatim(parent, name) {
			return
		}
		add(ListMDFiles(filepath.Join(parent, name)))
	}

	// (1) flat legado — req_dir/*.md
	add(ListMDFiles(reqDir))

	// (2) por-estado legado — req_dir/<estado>/*.md
	for _, state := range reqLayoutStates {
		addChild(reqDir, state)
	}

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := resolveAgentNamespaces(cfg, reqDir)
		for _, agent := range agents {
			// ML-4A (achado 2): agent vem do disco (nome de diretório sem validação de formato) —
			// ListMDFiles em vez de filepath.Glob, para que metacaracteres ("*", "[") no nome não
			// sejam interpretados como padrão e corrompam a contagem em silêncio.
			// (3) canônico — req_dir/<agente>/*.md
			//
			// ⚠️ ÂNCORA DE FALSIFICAÇÃO — a chamada logo abaixo é pinada VERBATIM pelo Cenário
			// 183 de scripts/check-gates-falsify.sh, que prova que
			// check-artifact-closed-cycle.sh REPROVA quando este caso canônico sai do resolvedor.
			// O `corrupt_literal` do cenário exige EXATAMENTE 1 ocorrência da chamada e reprova
			// fail-closed ("expected exactly 1 occurrence, got 0") se a grafia mudar. É a 3ª vez
			// que uma âncora literal deste harness quebra por renomeação (Cenários 81 e 179 antes
			// desta): ao renomear o helper, trocar a ordem dos argumentos ou duplicar a chamada,
			// RETARGETAR o cenário junto — senão só o `make quality` de 13 min avisa.
			//
			// 🔴 NÃO reproduzir a grafia exata da chamada em NENHUM comentário deste arquivo: a
			// contagem do `corrupt_literal` é sobre o arquivo inteiro e uma menção morta a levaria
			// a 2, reprovando o cenário do mesmo jeito (medido ao escrever este aviso).
			addChild(reqDir, agent)
			// (4) legado — req_dir/<agente>/<estado>/*.md
			for _, state := range reqLayoutStates {
				addChild(filepath.Join(reqDir, agent), state)
			}
		}
	}

	// Ordem determinística e igual nos 3 CLIs — a ordem de varredura é agent-major e não seria
	// estável entre runtimes (readdir do Node não é ordenado).
	sort.Strings(files)
	return files, nil
}

// resolveREQFiles é o alias interno do ponto único ResolveREQFiles — mantido para os consumidores
// dentro do pacote. NÃO reimplementar a descoberta aqui (D4).
func resolveREQFiles(cfg config.ProjectConfig) ([]string, error) {
	return ResolveREQFiles(cfg)
}

func validateWIPHasREQ() ([]string, error) {
	cfg := config.Load()
	wipDirs := resolveWIPDirs(cfg)

	var violations []string
	for _, wipDir := range wipDirs {
		entries := listDirForRule("wip_has_req", wipDir, &violations)
		for _, name := range entries {
			content, ok := readFileForRule("wip_has_req", filepath.Join(wipDir, name), &violations)
			if !ok {
				continue
			}
			if !contentHasMarkerValue(string(content), cfg.LinkFieldsReq) {
				violations = append(violations, fmt.Sprintf("roadmap %q is in wip but has no linked REQ (marker must start the line with a real, non-placeholder value)", name))
			}
		}
	}
	return violations, nil
}

func validateREQsHaveADR() ([]string, error) {
	cfg := config.Load()
	files, err := resolveREQFiles(cfg)
	if err != nil {
		return []string{err.Error()}, nil
	}

	var violations []string
	for _, path := range files {
		content, ok := readFileForRule("req_has_adr", path, &violations)
		if !ok {
			continue
		}
		if !contentHasMarkerValue(string(content), cfg.LinkFieldsADR) {
			violations = append(violations, fmt.Sprintf("req %q has no linked ADR (marker must start the line with a real, non-placeholder value)", filepath.Base(path)))
		}
	}
	return violations, nil
}

func validateBlockedHasREQ() ([]string, error) {
	cfg := config.Load()

	var violations []string
	for _, blockedDir := range resolveStateDirs(cfg, "blocked") {
		entries := listDirForRule("blocked_has_req", blockedDir, &violations)
		for _, name := range entries {
			content, ok := readFileForRule("blocked_has_req", filepath.Join(blockedDir, name), &violations)
			if !ok {
				continue
			}
			if !contentHasMarkerValue(string(content), cfg.LinkFieldsReq) {
				violations = append(violations, fmt.Sprintf("roadmap %q is in blocked but has no linked REQ (marker must start the line with a real, non-placeholder value)", name))
			}
		}
	}
	return violations, nil
}

// validateREQsHaveRoadmap verifica se cada REQ tem ao menos um campo de vínculo preenchido.
//
// ML-1A (AC9) — fonte de verdade: FRONTMATTER. O campo `roadmap:` do frontmatter é mantido pelo
// `roadmap move` via syncREQReferences; é o que o `serve` e o `validate` usam como autoridade. O
// marcador de corpo `Roadmap:` é legado e pode estar desatualizado após moves. Isso corrige os 21
// casos onde o frontmatter estava preenchido mas a contagem do validate mostrava "órfã" porque
// contentHasMarkerValue (case-sensitive, marcador capital "Roadmap:") não enxergava o campo
// lowercase "roadmap:" do frontmatter.
//
// ML-1D — o que o código faz, exatamente (o comentário anterior afirmava extractRefPath e o código
// usava extractFrontmatterField, que aceita QUALQUER valor não-vazio; medido: `roadmap: none`
// satisfazia a regra, e uma REQ cujo corpo dizia "Roadmap: (a criar quando esta REQ sair do
// backlog…)" contava como vinculada):
//
//   - a detecção é contentHasStructuredRefValue sobre cfg.LinkFieldsRoadmap, que delega a
//     extractRefPath — logo o vínculo tem de ser um caminho terminado em ".md". `none`, `TBD`,
//     `-`, comentário HTML e prosa deixaram de contar;
//   - o casamento da chave é CASE-INSENSITIVE (EqualFold em extractRefPath), então `roadmap:` do
//     frontmatter e `Roadmap:` do corpo são o mesmo campo — e é por isso que a claim de
//     case-insensitividade, falsa enquanto o código usava extractFrontmatterField
//     (HasPrefix(field+":"), sensível a caixa), agora é verdadeira;
//   - a precedência do frontmatter é POSICIONAL, não explícita: extractRefPath varre linha a linha
//     e o frontmatter vem primeiro no arquivo, então ele vence quando carrega um valor real, e o
//     corpo é o fallback quando não carrega. Consequência deliberada e documentada: um `roadmap:`
//     declarado e VERDADEIRAMENTE vazio (nada depois do ":") faz extractRefPath retornar cedo e
//     BLOQUEIA o fallback para o corpo — medido em 2026-09-26 nos 231 REQs deste repositório: zero
//     casos (as 17 REQs com frontmatter vazio gravam `roadmap: ""`, que não é vazio para o
//     extrator e cai no corpo normalmente).
func validateREQsHaveRoadmap() ([]string, error) {
	cfg := config.Load()
	files, err := resolveREQFiles(cfg)
	if err != nil {
		return []string{err.Error()}, nil
	}

	var violations []string
	for _, path := range files {
		content, ok := readFileForRule("req_has_roadmap", path, &violations)
		if !ok {
			continue
		}
		if !contentHasStructuredRefValue(string(content), cfg.LinkFieldsRoadmap) {
			violations = append(violations, fmt.Sprintf("req %q has no linked Roadmap (marker must start the line with a real, non-placeholder value)", filepath.Base(path)))
		}
	}
	return violations, nil
}

// validateREQRoadmapSync detecta REQs onde o campo `roadmap:` do frontmatter e o marcador
// `Roadmap:` do corpo estão AMBOS preenchidos mas apontam para basenames diferentes — sinal
// de edição manual inconsistente. Diferenças apenas de pasta de estado (wip→done) são
// esperadas após `roadmap move` e NÃO disparam esta regra (ML-1A decisão 2: aviso só para
// divergência real de basename).
func validateREQRoadmapSync() ([]string, error) {
	cfg := config.Load()
	files, err := resolveREQFiles(cfg)
	if err != nil {
		return []string{err.Error()}, nil
	}

	var warnings []string
	for _, path := range files {
		content, ok := readFileForRule("req_roadmap_sync", path, &warnings)
		if !ok {
			continue
		}
		// ML-1D: o lado frontmatter passou a usar o MESMO critério de "referência real" do lado corpo
		// (que já ia por extractBodyRefPath → extractRefPath) e do req_has_roadmap. Antes, o
		// frontmatter ia por extractFrontmatterField e aceitava qualquer valor não-vazio: uma REQ com
		// `roadmap: "none"` e um caminho real no corpo era reportada como "divergent roadmap links:
		// frontmatter=\"none\" body=\"x.md\"" — e não há divergência ali, há UM vínculo e UM
		// placeholder. Pior, as duas regras discordariam sobre o que é valor: req_has_roadmap diria
		// "sem vínculo" e esta diria "vínculos conflitantes". Medido nos 231 REQs em 2026-09-26: zero
		// ocorrências (as 17 REQs de frontmatter vazio gravam `roadmap: ""`, que já era ignorado), logo
		// a correção é de mecanismo, não de contagem.
		//
		// A delegação remonta a LINHA do campo em vez de chamar um predicado de valor: o predicado vive
		// dentro de extractRefPath e extraí-lo para um helper próprio exigiria mexer no corpo daquela
		// função, cujo literal o Cenário 28 do check-gates-falsify fixa para sabotar o strip de backtick.
		fmRef := extractRefPath("roadmap: "+extractFrontmatterField(string(content), "roadmap"), "roadmap")
		bodyRef := extractBodyRefPath(string(content), "Roadmap")
		if fmRef == "" || bodyRef == "" {
			continue // divergência requer os dois preenchidos com referência REAL
		}
		fmBase := filepath.Base(strings.Trim(fmRef, `"'` + "`"))
		bodyBase := filepath.Base(strings.Trim(bodyRef, `"'` + "`"))
		if fmBase != bodyBase {
			warnings = append(warnings, fmt.Sprintf("req %q has divergent roadmap links: frontmatter=%q body=%q", filepath.Base(path), fmRef, bodyRef))
		}
	}
	return warnings, nil
}

// extractBodyRefPath extrai o valor de um campo ref (ex: "Roadmap") do CORPO da REQ,
// ignorando o bloco frontmatter. Usa extractRefPath no conteúdo pós-frontmatter.
// Se não houver bloco frontmatter delimitado por "---", retorna "" (sem tentar o arquivo inteiro,
// pois a ausência de frontmatter já é tratada pela função chamadora).
func extractBodyRefPath(content, field string) string {
	lines := strings.Split(content, "\n")
	fmCount := 0
	bodyStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			fmCount++
			if fmCount == 2 {
				bodyStart = i + 1
				break
			}
		}
	}
	if bodyStart < 0 {
		return ""
	}
	bodyContent := strings.Join(lines[bodyStart:], "\n")
	return extractRefPath(bodyContent, field)
}

func validateADRsAreReferenced() ([]string, error) {
	cfg := config.Load()
	var violations []string
	var adrs []string
	for _, adrDir := range cfg.ADRDirs {
		paths := walkADRFilePathsForRule("adr_orphan", adrDir, &violations)
		for _, p := range paths {
			if isOutsideCWD(p) {
				continue
			}
			adrs = append(adrs, filepath.Base(p))
		}
	}

	reqPaths, err := resolveREQFiles(cfg)
	if err != nil {
		violations = append(violations, err.Error())
		return violations, nil
	}
	var allREQContent strings.Builder
	for _, p := range reqPaths {
		b, ok := readFileForRule("adr_orphan", p, &violations)
		if ok {
			allREQContent.Write(b)
		}
	}
	combined := allREQContent.String()

	for _, adr := range adrs {
		if !strings.Contains(combined, adr) {
			violations = append(violations, fmt.Sprintf("adr %q is not referenced by any REQ", adr))
		}
	}
	return violations, nil
}

func validateWIPHasAcceptanceCriteria() ([]string, error) {
	cfg := config.Load()
	wipDirs := resolveWIPDirs(cfg)

	var violations []string
	for _, wipDir := range wipDirs {
		entries := listDirForRule("wip_acceptance", wipDir, &violations)
		for _, name := range entries {
			content, ok := readFileForRule("wip_acceptance", filepath.Join(wipDir, name), &violations)
			if !ok {
				continue
			}
			s := string(content)
			hasBlock := contentHasMarker(s, cfg.AcceptanceMarkers)
			if !hasBlock {
				violations = append(violations, fmt.Sprintf("roadmap %q is in wip but has no acceptance criteria block", name))
			}
		}
	}
	return violations, nil
}

func validateStaleWIP() ([]string, error) {
	cfg := config.Load()
	wipDirs := resolveWIPDirs(cfg)
	thresholdDays := cfg.StaleWIPDays
	if thresholdDays <= 0 {
		thresholdDays = staleWIPDays
	}
	now := staleWIPNow()

	var warnings []string
	for _, wipDir := range wipDirs {
		entries, err := listDir(wipDir)
		if err != nil {
			if !os.IsNotExist(err) {
				warnings = append(warnings, inspectionDiagnostic("stale_wip", wipDir, err))
			}
			continue
		}
		for _, name := range entries {
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			path := filepath.Join(wipDir, name)
			info, err := os.Stat(path)
			if err != nil {
				warnings = append(warnings, inspectionDiagnostic("stale_wip", path, err))
				continue
			}
			refTime := info.ModTime()
			logTime, ok, diagnostics := latestWIPTransitionTime(cfg, path)
			warnings = append(warnings, diagnostics...)
			if ok {
				refTime = logTime
			}
			age := now.Sub(refTime)
			days := int(age.Hours() / 24)
			if days >= thresholdDays {
				warnings = append(warnings, fmt.Sprintf(
					"roadmap/wip/%s has been in WIP for %d days (last modified %s)",
					filepath.Base(path), days, refTime.Format("2006-01-02"),
				))
			}
		}
	}
	return warnings, nil
}

func latestWIPTransitionTime(cfg config.ProjectConfig, roadmapPath string) (time.Time, bool, []string) {
	logPath := filepath.Join(cfg.RoadmapDir, ".trackfw-log")
	data, err := readRegularFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, []string{inspectionDiagnostic("stale_wip", logPath, err)}
	}
	identity := roadmapLogIdentity(cfg, roadmapPath)
	var latest time.Time
	found := false
	var diagnostics []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		timestamp, name, toState, ok := parseTransitionLogLine(line)
		if !ok {
			diagnostics = append(diagnostics, fmt.Sprintf("stale_wip: invalid support line in %q: %q", logPath, line))
			continue
		}
		if name != identity || toState != "wip" {
			continue
		}
		if !found || timestamp.After(latest) {
			latest = timestamp
			found = true
		}
	}
	return latest, found, diagnostics
}

func roadmapLogIdentity(cfg config.ProjectConfig, roadmapPath string) string {
	basename := filepath.Base(roadmapPath)
	if cfg.RoadmapNamespacing != config.NamespacingByAgent {
		return basename
	}
	stateDir := filepath.Dir(roadmapPath)
	agentDir := filepath.Dir(stateDir)
	agent := filepath.Base(agentDir)
	if agent == "." || agent == string(filepath.Separator) || agent == "" {
		return basename
	}
	return filepath.ToSlash(filepath.Join(agent, basename))
}

func parseTransitionLogLine(line string) (time.Time, string, string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return time.Time{}, "", "", false
	}
	timestamp, err := time.ParseInLocation("2006-01-02 15:04", fields[0]+" "+fields[1], time.Local)
	if err != nil {
		return time.Time{}, "", "", false
	}
	arrow := -1
	for i := 3; i < len(fields); i++ {
		if fields[i] == "→" || fields[i] == "->" {
			arrow = i
			break
		}
	}
	if arrow < 0 || arrow+1 >= len(fields) {
		return time.Time{}, "", "", false
	}
	name := fields[2]
	toState := fields[arrow+1]
	return timestamp, name, toState, true
}

// blockedREQs retorna um mapa de REQ-basename → lista de ADR-basenames não aceitos
// (Draft ou Proposed) que a bloqueiam. Somente REQs com Status: Open são incluídas.
func blockedREQs() (map[string][]string, error) {
	cfg := config.Load()
	files, err := resolveREQFiles(cfg)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, reqPath := range files {
		content, err := readRegularFile(reqPath)
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "Status: Open") {
			continue
		}

		adrNames, err := parseBlockedADRs(reqPath)
		if err != nil {
			continue
		}
		var notAcceptedADRs []string
		for _, adrBasename := range adrNames {
			if notAccepted, _ := adrDraftStatusForRule("blocked_by_draft_adr", adrBasename, nil); notAccepted {
				notAcceptedADRs = append(notAcceptedADRs, adrBasename)
			}
		}
		if len(notAcceptedADRs) > 0 {
			result[filepath.Base(reqPath)] = notAcceptedADRs
		}
	}
	return result, nil
}

// validateREQsNotBlockedByDraftADRs verifica se REQs com Status Open têm ADRs não aceitos
// (Draft ou Proposed) vinculados. Uma REQ Open bloqueada por um ADR não aceito é uma
// violação: o roadmap não pode ser criado até o ADR ser aceito. Corrige a cegueira
// anterior a Proposed — ADRs criados por `adr new` (o caminho normal) não eram detectados.
func validateREQsNotBlockedByDraftADRs() ([]string, error) {
	cfg := config.Load()
	entries, err := resolveREQFiles(cfg)
	if err != nil {
		return []string{err.Error()}, nil
	}

	var violations []string
	for _, path := range entries {
		content, ok := readFileForRule("blocked_by_draft_adr", path, &violations)
		if !ok {
			continue
		}
		s := string(content)
		// Verificar se a REQ está com Status: Open (linha de cabeçalho)
		if !strings.Contains(s, "Status: Open") {
			continue
		}
		// Extrair ADRs da seção "## Blocked by ADRs"
		blockedADRs, err := parseBlockedADRs(path)
		if err != nil {
			violations = append(violations, inspectionDiagnostic("blocked_by_draft_adr", path, err))
			continue
		}
		reqBasename := filepath.Base(path)
		for _, adrBasename := range blockedADRs {
			if notAccepted, _ := adrDraftStatusForRule("blocked_by_draft_adr", adrBasename, &violations); notAccepted {
				violations = append(violations, fmt.Sprintf("REQ %s is blocked by not-accepted ADR: %s", reqBasename, adrBasename))
			}
		}
	}
	return violations, nil
}

// parseBlockedADRs extrai os basenames de ADRs listados na seção "## Blocked by ADRs" de um arquivo REQ.
//
// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1H:
// content is folded through integrations.NormalizeCRLF before the exact `line == "## Blocked by
// ADRs"` comparison below. A REQ authored on Windows (or checked out with core.autocrlf=true --
// exactly the scenario ADR-2026-09-04 D1 already decided this project tolerates) leaves a
// trailing "\r" on that line after strings.Split(content, "\n"); nothing downstream trims it, so
// the exact-equality match silently never fires and the whole section is skipped. This was
// measured live in the Python port of this same function (validator.py's _parse_blocked_adrs) on
// Windows ARM64 -- Python's pre-ML-1G read path masked it via an implicit universal-newlines
// translation only Python's own open() does; Go's readRegularFile never had that translation to
// lose, so this exact bug was ALSO present here already, just never exercised by any fixture (Go's
// os.WriteFile never introduces stray CRLF the way Python's text-mode open() can on Windows).
// Closing it here, not just in Python, keeps the 3 CLIs at parity for a real (not test-artifact)
// scenario -- see TestParseBlockedADRsCRLF for the falsification.
func parseBlockedADRs(path string) ([]string, error) {
	content, err := readRegularFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(integrations.NormalizeCRLF(content)), "\n")

	var adrs []string
	inSection := false
	for _, line := range lines {
		if line == "## Blocked by ADRs" {
			inSection = true
			continue
		}
		if inSection {
			// Próxima seção termina a leitura
			if strings.HasPrefix(line, "## ") {
				break
			}
			// Linhas de item: "- ADR-xxx.md (Draft)" ou "- ADR-xxx.md (Accepted)"
			if strings.HasPrefix(line, "- ") {
				item := strings.TrimPrefix(line, "- ")
				// Extrair o basename (primeira palavra antes de espaço ou parêntese)
				parts := strings.Fields(item)
				if len(parts) > 0 && strings.HasSuffix(parts[0], ".md") {
					adrs = append(adrs, parts[0])
				}
			}
		}
	}
	return adrs, nil
}

// adrIsDraft verifica se o ADR identificado pelo basename está não-aceito (Draft ou
// Proposed) via adrStatusIsNotAccepted. Busca recursivamente em todas as ADRDirs
// configuradas. Nome preservado por compatibilidade com os testes existentes.
func adrIsDraft(adrBasename string) bool {
	notAccepted, _ := adrDraftStatusForRule("", adrBasename, nil)
	return notAccepted
}

// adrStatusIsNotAccepted é o helper canônico para decidir se um ADR NÃO está aceito
// (ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito...). "Não aceito" cobre os status
// Draft e Proposed — os dois caminhos de criação de ADR no trackfw (`adr new` gera
// Proposed; `req new` gera Draft via NewADRDraft). Qualquer outro status (Accepted,
// Superseded, Deprecated, Rejected, ou até um status desconhecido/ausente) conta como
// aceito por exclusão — este helper nunca usa allowlist fechada.
//
// Prioridade de leitura: frontmatter `status:` primeiro — é o campo machine-readable
// canônico, o mesmo que os geradores (`adr new`, `NewADRDraft`) escrevem e que a regra
// `folder_status` já usa como fonte de verdade. Cai para a linha de cabeçalho
// "> Date: ... | Status: X" somente se o frontmatter estiver ausente ou sem o campo —
// cobre ADRs legados sem frontmatter. Em um ADR bem formado os dois concordam.
//
// Este fallback é uma extração posicional da linha de cabeçalho (não um
// strings.Contains no corpo inteiro do arquivo): o valor hardcoded anterior
// (`strings.Contains(content, "Status: Draft")`) casava a substring em qualquer lugar
// do documento, inclusive em prosa (ex.: uma seção de Contexto mencionando "estava em
// Status: Draft") — um falso positivo que piora ao somar "Proposed", string bem mais
// comum em texto corrido. A extração por linha de cabeçalho evita essa classe de erro.
// resolveAdrStatus extrai o valor bruto do status de um ADR: frontmatter `status:`
// primeiro, com fallback para a linha de cabeçalho "> Date: ... | Status: X". Retorna
// string vazia se nenhuma das duas fontes tiver o campo.
func resolveAdrStatus(content string) string {
	if status := extractFrontmatterField(content, "status"); status != "" {
		return status
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		idx := strings.Index(trimmed, "| Status: ")
		if idx < 0 {
			continue
		}
		rest := trimmed[idx+len("| Status: "):]
		if pipeIdx := strings.Index(rest, " |"); pipeIdx >= 0 {
			rest = rest[:pipeIdx]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

// statusIsNotAccepted é a única expressão do pacote que conhece o vocabulário
// "Draft"/"Proposed" de status de ADR não aceito. Todo ponto do código que precisa
// dessa checagem deve chamar este helper (ou adrStatusIsNotAccepted, que o aplica
// diretamente sobre o conteúdo de um ADR) em vez de comparar os literais.
func statusIsNotAccepted(status string) bool {
	return strings.EqualFold(status, "Draft") || strings.EqualFold(status, "Proposed")
}

func adrStatusIsNotAccepted(content string) bool {
	return statusIsNotAccepted(resolveAdrStatus(content))
}

// adrStatusForRule resolve o basename do ADR nos adrDirs configurados e retorna o valor
// bruto do status (via resolveAdrStatus). O segundo retorno indica se a resolução foi
// bem-sucedida (ADR não encontrado conta como sucesso, com status vazio).
func adrStatusForRule(rule, adrBasename string, msgs *[]string) (string, bool) {
	cfg := config.Load()
	p := findADRFile(adrBasename, cfg.ADRDirs)
	if p == "" {
		return "", true
	}
	content, err := readRegularFile(p)
	if err != nil {
		if msgs != nil {
			*msgs = append(*msgs, inspectionDiagnostic(rule, p, err))
		}
		return "", false
	}
	return resolveAdrStatus(string(content)), true
}

// adrDraftStatusForRule resolve o basename do ADR nos adrDirs configurados e aplica
// adrStatusIsNotAccepted() ao conteúdo. Apesar do nome (preservado por compatibilidade
// histórica — usado por 3 chamadores, incluindo adrIsDraft), hoje cobre Draft e
// Proposed via o helper canônico, não apenas Draft.
func adrDraftStatusForRule(rule, adrBasename string, msgs *[]string) (bool, bool) {
	status, ok := adrStatusForRule(rule, adrBasename, msgs)
	if !ok {
		return false, false
	}
	return statusIsNotAccepted(status), true
}

// extractFrontmatterField extrai o valor de um campo do bloco frontmatter YAML.
func extractFrontmatterField(content, field string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	block := rest[:end]
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			val := strings.TrimSpace(strings.TrimPrefix(line, field+":"))
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// validateFrontmatterPresence verifica se os artefatos têm frontmatter com status e date.
// Retorna violations para arquivos sem frontmatter válido.
// Esta validação é lenient: só reporta se o frontmatter estiver completamente ausente.
func validateFrontmatterPresence() []string {
	cfg := config.Load()
	var violations []string

	// ADRs — busca recursiva em subpastas
	for _, adrDir := range cfg.ADRDirs {
		basenames := walkADRFiles(adrDir)
		for _, basename := range basenames {
			fullPath := findADRFile(basename, cfg.ADRDirs)
			if fullPath == "" {
				continue
			}
			content, ok := readFileForRule("frontmatter_presence", fullPath, &violations)
			if !ok {
				continue
			}
			if !strings.HasPrefix(string(content), "---") {
				violations = append(violations, fmt.Sprintf("adr %q has no frontmatter block", basename))
			}
		}
	}

	// REQs — usa resolveREQFiles para suportar namespacing by_agent
	reqFiles, err := resolveREQFiles(cfg)
	if err != nil {
		violations = append(violations, err.Error())
		return violations
	}
	for _, f := range reqFiles {
		content, ok := readFileForRule("frontmatter_presence", f, &violations)
		if !ok {
			continue
		}
		if !strings.HasPrefix(string(content), "---") {
			violations = append(violations, fmt.Sprintf("req %q has no frontmatter block", filepath.Base(f)))
		}
	}

	return violations
}

func listDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// validateADRDirsExist verifica se os diretórios em adr_dirs existem no disco.
// Se um diretório não existir:
// - se StrictCIPaths for true: gera violation (Error)
// - se StrictCIPaths for false: gera warning (Warning)
func validateADRDirsExist(cfg config.ProjectConfig) (violations []string, warnings []string) {
	for _, adrDir := range cfg.ADRDirs {
		expandedDir := config.ExpandPath(adrDir)
		if _, err := os.Stat(expandedDir); os.IsNotExist(err) {
			msg := fmt.Sprintf("adr_dir %q does not exist", adrDir)
			if cfg.StrictCIPaths {
				violations = append(violations, msg)
			} else {
				warnings = append(warnings, msg)
			}
		}
	}
	return violations, warnings
}

// isOutsideCWD verifica se o caminho do arquivo/diretório está localizado fora do diretório raiz do projeto local (CWD).
func isOutsideCWD(path string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(config.ExpandPath(path))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absCwd, absPath)
	if err != nil {
		return true
	}
	return strings.HasPrefix(rel, "..") || filepath.IsAbs(rel)
}

// getwdFn is the function used to obtain the current working directory inside
// checkREQDirContained. It defaults to os.Getwd and may be overridden in
// internal tests (package validator) to inject a controlled failure without
// relying on OS-specific filesystem tricks (e.g. removing the CWD, which
// Windows prevents while the directory is in use).
var getwdFn func() (string, error) = os.Getwd

// checkREQDirContained verifica se req_dir está contido dentro do diretório raiz do projeto (CWD).
// Usa EvalSymlinks em ambos os lados para não ser enganado por symlinks — um symlink em
// docs/req apontando para fora da árvore passa numa verificação lexical mas é recusado aqui.
//
// 🔴 NÃO reutiliza isOutsideCWD: aquela função é lexical (sem EvalSymlinks). isOutsideCWD
// continua lexical para o seu uso na regra adr_orphan, que está fora do escopo desta correção
// (AC7 — REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md).
//
// Retorna erro com req_dir VERBATIM (sem expansão) para que o chamador possa imprimir a
// mensagem de diagnóstico sem vazar caminhos absolutos em logs de CI.
//
// Falha fechada: se não for possível estabelecer a raiz do projeto (Getwd falhou ou
// EvalSymlinks do CWD falhou), o erro é retornado com diagnóstico nomeado. Um controle de
// contenção que não pode medir contenção não deve reportar "contido".
func checkREQDirContained(reqDir string) error {
	cwd, err := getwdFn()
	if err != nil {
		return fmt.Errorf("não foi possível resolver o diretório de trabalho — contenção de req_dir não pode ser verificada: %w", err)
	}
	physCWD, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return fmt.Errorf("não foi possível resolver o diretório de trabalho fisicamente (EvalSymlinks falhou) — contenção de req_dir não pode ser verificada: %w", err)
	}

	expanded := config.ExpandPath(reqDir)
	var absReqDir string
	if filepath.IsAbs(expanded) {
		absReqDir = filepath.Clean(expanded)
	} else {
		absReqDir = filepath.Join(physCWD, expanded)
	}

	physReqDir := resolvePhysical(absReqDir)

	rel, err := filepath.Rel(physCWD, physReqDir)
	if err != nil {
		return fmt.Errorf("req_dir %q resolves outside project root — path traversal refused", reqDir)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("req_dir %q resolves outside project root — path traversal refused", reqDir)
	}
	return nil
}

// resolvePhysical resolve um caminho para seu caminho físico real usando filepath.EvalSymlinks.
// Para caminhos que não existem, sobe ao ancestral mais profundo que existe, resolve esse
// ancestral fisicamente e reanexa o sufixo lexicamente.
// Isso permite que req_dir aponte para um diretório ainda não criado sem ser erroneamente
// recusado pela contenção.
func resolvePhysical(p string) string {
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		return resolved
	}
	// O caminho não existe: sobe ao ancestral mais profundo que existe.
	dir := p
	var suffix []string
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return p // chegou na raiz sem sucesso; retorna verbatim
		}
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Join(parts...)
		}
		suffix = append([]string{filepath.Base(dir)}, suffix...)
		dir = parent
	}
}

// walkADRFilePaths retorna os caminhos completos de todos os arquivos .md encontrados recursivamente em adrDir.
func walkADRFilePaths(adrDir string) []string {
	return walkADRFilePathsForRule("", adrDir, nil)
}

func walkADRFilePathsForRule(rule, adrDir string, msgs *[]string) []string {
	adrDir = config.ExpandPath(adrDir)
	var paths []string
	_ = filepath.WalkDir(adrDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if msgs != nil && !os.IsNotExist(err) {
				*msgs = append(*msgs, inspectionDiagnostic(rule, path, err))
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	return paths
}

// walkADRFiles retorna basenames de todos os arquivos .md encontrados recursivamente em adrDir.
func walkADRFiles(adrDir string) []string {
	paths := walkADRFilePaths(adrDir)
	var names []string
	for _, p := range paths {
		names = append(names, filepath.Base(p))
	}
	return names
}

// findADRFile busca um arquivo pelo basename recursivamente em todos os adrDirs.
// Retorna o caminho completo ou string vazia se não encontrado.
func findADRFile(adrBasename string, adrDirs []string) string {
	for _, adrDir := range adrDirs {
		expandedDir := config.ExpandPath(adrDir)
		var found string
		_ = filepath.WalkDir(expandedDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && filepath.Base(path) == adrBasename {
				found = path
				return fs.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

// gitLastModifiedTime retorna o timestamp do último commit que tocou o path via git log.
// Retorna (zero, false) se git não estiver disponível ou o arquivo não tiver histórico.
func gitLastModifiedTime(path string) (time.Time, bool) {
	cmd := gitCommand(".", "log", "-1", "--format=%ct", "--", path)
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return time.Time{}, false
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(ts, 0), true
}

// extractRefPath extrai o valor do campo field: na linha de frontmatter/cabeçalho.
// Retorna string vazia se o campo estiver ausente, vazio ou com valor traço — e TAMBÉM quando o
// primeiro token do valor não termina em ".md" (ML-1D: essa condição é parte do contrato, não um
// detalhe; é ela que faz `none`, `TBD` e prosa NÃO serem referência, e o comentário anterior a
// omitia, o que alimentou a divergência entre a regra req_has_roadmap e este extrator).
// ExtractRefPath é o wrapper exportado de extractRefPath, usado por consumidores fora do pacote
// validator (internal/serve/api_chain.go, ML-3D). Achado durante o ML-3D: o gerador de REQ
// (internal/generators/req.go) grava `adr: ""` e `roadmap: ""` SEMPRE vazios no frontmatter —
// o valor canônico vive no corpo, em "## Linked ADR / ADR: <path>" e "## Linked Roadmap /
// Roadmap: <path>". Um extrator que só lê o bloco YAML de frontmatter (como o antigo
// parseFrontmatter do `serve`) nunca encontra o vínculo real de nenhuma REQ gerada pelo `trackfw
// req new` — não é caso de borda, é o formato canônico. Ponto único: os dois consumidores usam
// esta função em vez de reimplementar a varredura linha a linha.
func ExtractRefPath(content, field string) string {
	return extractRefPath(content, field)
}

func extractRefPath(content, field string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		key, val, ok := strings.Cut(trimmed, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), field) {
			val := strings.TrimSpace(val)
			if val == "" || val == "—" || val == "-" || val == "–" {
				return ""
			}
			fields := strings.Fields(val)
			if len(fields) == 0 {
				return ""
			}
			v := strings.Trim(fields[0], "\"'`")
			if strings.HasSuffix(v, ".md") {
				return v
			}
		}
	}
	return ""
}

// contentHasStructuredRefValue reporta se algum dos markers de vínculo tem, no conteúdo, um valor
// que extractRefPath aceita como REFERÊNCIA REAL — isto é, um caminho terminado em ".md".
//
// ML-1D: é o ponto único de "está vinculada?" para as regras cujo vínculo é lido por extractRefPath,
// e existe para que a regra e o extrator não possam divergir (era o defeito medido: a regra aceitava
// qualquer valor não-vazio do frontmatter, então `roadmap: none` passava).
//
// Alcance, escrito para não virar a próxima claim falsa: quem passa por aqui hoje é `req_has_roadmap`.
// `req_roadmap_sync` e `ref_targets_exist` chamam extractRefPath direto (o MESMO predicado, sem este
// invólucro) porque precisam do VALOR da referência, não do booleano. Já `req_has_adr`, `wip_has_req` e
// `blocked_has_req` continuam em contentHasMarkerValue — decisão de escopo, não descuido: o campo
// `adr:` é o ML-1E desta mesma REQ, e nenhuma dessas três carrega comentário afirmando outra fonte de
// verdade (o defeito que o ML-1D corrige é comentário que mente, não assimetria por si).
//
// O que ele herda de extractRefPath, e que contentHasMarkerValue NÃO faz:
//   - exige que o valor termine em ".md" — recusa `none`, `TBD`, `nenhum`, prosa e qualquer outro
//     texto que não seja um caminho de artefato;
//   - casa a chave por EqualFold, então o `roadmap:` (minúsculo) do frontmatter e o `Roadmap:`
//     (capital) do corpo são o MESMO campo;
//   - remove backtick/aspas do primeiro token, então `` Roadmap: `docs/.../X.md` `` conta.
//
// O que ele preserva de contentHasMarkerValue: a configurabilidade de link_fields (cada marker vira
// o field do extrator, sem o ":" final) e a ancoragem por chave de linha — em extractRefPath a chave
// é TUDO o que vem antes do primeiro ":", então prosa como "veja a secao Roadmap: mais abaixo" não
// casa (a chave seria "veja a secao Roadmap").
func contentHasStructuredRefValue(content string, markers []string) bool {
	for _, marker := range markers {
		field := strings.TrimSuffix(strings.TrimSpace(marker), ":")
		if field == "" {
			continue
		}
		if extractRefPath(content, field) != "" {
			return true
		}
	}
	return false
}

// validateRefTargetsExist verifica se arquivos referenciados via REQ:, ADR: e Roadmap: existem.
func validateRefTargetsExist() ([]string, error) {
	cfg := config.Load()
	var warnings []string

	dirs := append(resolveWIPDirs(cfg), resolveStateDirs(cfg, "blocked")...)
	for _, dir := range dirs {
		entries := listDirForRule("ref_targets_exist", dir, &warnings)
		for _, name := range entries {
			content, ok := readFileForRule("ref_targets_exist", filepath.Join(dir, name), &warnings)
			if !ok {
				continue
			}
			if ref := extractRefPath(string(content), "REQ"); ref != "" {
				if !referenceExists(ref) {
					warnings = append(warnings, fmt.Sprintf("roadmap %q links to REQ %q which does not exist", name, ref))
				}
			}
		}
	}

	reqFiles, err := resolveREQFiles(cfg)
	if err != nil {
		warnings = append(warnings, err.Error())
		return warnings, nil
	}
	for _, reqPath := range reqFiles {
		content, ok := readFileForRule("ref_targets_exist", reqPath, &warnings)
		if !ok {
			continue
		}
		s := string(content)
		name := filepath.Base(reqPath)
		if ref := extractRefPath(s, "ADR"); ref != "" {
			if !referenceExists(ref) {
				warnings = append(warnings, fmt.Sprintf("req %q links to ADR %q which does not exist", name, ref))
			}
		}
		if ref := extractRefPath(s, "Roadmap"); ref != "" {
			resolved, stale := resolveRoadmapRefStatus(cfg, ref)
			switch len(resolved) {
			case 0:
				warnings = append(warnings, fmt.Sprintf("req %q links to Roadmap %q which does not exist", name, ref))
			case 1:
				if stale {
					// literal falhou, resolvido só pelo fallback de basename num outro
					// estado: o vínculo aponta para um caminho de estado velho — o
					// serve (internal/serve/api_chain.go) casa edge.To pelo literal e
					// desenharia aresta órfã para o mesmo vínculo se calássemos aqui.
					actualState := filepath.Base(filepath.Dir(resolved[0]))
					warnings = append(warnings, fmt.Sprintf("req %q links to Roadmap %q but the file is now in %s/ (stale state path)", name, ref, actualState))
				}
				// senão: resolvido pelo caminho literal — nenhum aviso.
			default:
				warnings = append(warnings, fmt.Sprintf("req %q links to Roadmap %q is ambiguous: found in multiple states: %s", name, ref, strings.Join(resolved, ", ")))
			}
		}
	}
	return warnings, nil
}

// normalizeRefSeparator normaliza um valor de referência já extraído de um campo (roadmap:,
// req:, adr:) para o separador portável (/) antes de resolvê-lo no filesystem local. Caminho
// dentro de artefato versionado é dado portável — um valor gravado no Windows antes do fix de
// escrita (ou por qualquer runtime que ainda não normalize) chega aqui com "\" literal, que em
// Linux/macOS não é separador, é caractere de nome de arquivo, e faz os.Stat falhar numa
// referência que na verdade existe (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md).
// NÃO aplicar ao buffer inteiro do arquivo — só ao valor já extraído do campo.
func normalizeRefSeparator(ref string) string {
	return strings.ReplaceAll(ref, "\\", "/")
}

func referenceExists(ref string) bool {
	expandedRef := config.ExpandPath(normalizeRefSeparator(ref))
	if _, err := os.Stat(expandedRef); err == nil {
		return true
	}
	return false
}

// resolveRoadmapRefByBasename procura, nos diretórios de ESTADO do roadmap_dir (backlog/analyzing/
// wip/blocked/done/abandoned, honrando roadmap_namespacing), um arquivo cujo nome (basename) case com
// filepath.Base(ref) — ROADMAP-2026-09-05, ML-3B: um vínculo `Roadmap:` grava o caminho completo
// INCLUINDO a pasta de estado (ex.: "docs/roadmaps/wip/x.md"), e pelo CLAUDE.md a pasta É o estado —
// então todo `trackfw roadmap move` quebra, por construção, o caminho literal gravado antes do move.
//
// Escopo DELIBERADAMENTE restrito ao campo Roadmap: (não REQ:, não ADR:):
//   - REQ não tem dimensão de estado (ADR-2026-09-03, invariante D1, ver reqLayoutStates acima) — um
//     caminho de REQ não fica velho por causa de um `move`, então não há mecanismo a corrigir aqui.
//     Aplicar fallback por basename ao campo REQ: destruiria a garantia estrita da
//     ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto:
//     o Cenário 25 de scripts/check-gates-falsify.sh corrompe deliberadamente um gerador para gravar
//     `filepath.Base(reqPath)` em vez do caminho completo, e esse teste SÓ funciona porque hoje
//     "REQ-flag-source.md" (sem diretório) não resolve por basename em lugar nenhum. Resolver por
//     basename ali tornaria esse teste vácuo — silenciaria exatamente a regressão que ele existe para
//     capturar.
//   - a árvore de ADR é FLAT (sem pastas de estado — confirmado em `ls docs/adr/`), então não há
//     hierarquia de estado equivalente a resolver; inventar uma aqui não tem medição que a sustente.
//
// Chamado só quando o caminho LITERAL já falhou E o segmento imediatamente anterior ao arquivo no
// valor gravado é um nome de estado reconhecido (agentNamespaceStateNames) — isso é o que torna o
// fallback seguro por construção contra o Cenário 25: "REQ-flag-source.md" tem
// filepath.Dir(ref) == "." (não um nome de estado), então nunca entra neste caminho, mesmo que esta
// função fosse chamada para ele.
//
// Retorna a lista de caminhos absolutos encontrados: 0 (sem match em estado algum), 1 (resolvido) ou
// mais de 1 (mesmo basename presente em mais de um estado — ambíguo, decisão do chamador).
func resolveRoadmapRefByBasename(cfg config.ProjectConfig, ref string) []string {
	base := filepath.Base(ref)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return nil
	}
	var found []string
	seen := make(map[string]bool)
	for state := range agentNamespaceStateNames {
		for _, dir := range resolveStateDirs(cfg, state) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || e.Name() != base {
					continue
				}
				full := filepath.Clean(filepath.Join(dir, e.Name()))
				if !seen[full] {
					seen[full] = true
					found = append(found, full)
				}
			}
		}
	}
	sort.Strings(found)
	return found
}

// isStaleRoadmapStateRef reporta se ref parece ser um caminho de roadmap gravado com uma pasta de
// estado que pode ter ficado velha — condição de entrada do fallback por basename
// (resolveRoadmapRefByBasename): o segmento imediatamente antes do nome do arquivo é um dos 6 nomes
// de estado reservados. "REQ-flag-source.md" (Cenário 25) tem dir "." e falha aqui por construção.
func isStaleRoadmapStateRef(ref string) bool {
	dir := filepath.Base(filepath.Dir(filepath.ToSlash(ref)))
	return agentNamespaceStateNames[dir]
}

// resolveRoadmapRef resolve um valor de campo `Roadmap:` já extraído (extractRefPath): caminho
// literal primeiro (idêntico ao referenceExists — preserva o branch de diretório existente hoje,
// ADR-2026-08-02), e só cai para o fallback por basename (resolveRoadmapRefByBasename) quando o
// literal falha e isStaleRoadmapStateRef(ref) indica que vale a pena tentar. Retorna o(s) caminho(s)
// resolvido(s): 0 = vínculo quebrado, 1 = resolvido, >1 = ambíguo.
func resolveRoadmapRef(cfg config.ProjectConfig, ref string) []string {
	resolved, _ := resolveRoadmapRefStatus(cfg, ref)
	return resolved
}

// ResolveRoadmapRef é o wrapper exportado de resolveRoadmapRef, usado por consumidores fora do
// pacote validator (internal/serve/api_chain.go, ML-3D) para casar edge.To de um campo
// `roadmap:` contra o node.ID real quando o caminho literal gravado no frontmatter aponta para
// uma pasta de estado que já ficou velha (mesma causa raiz do ML-3B: `trackfw roadmap move`
// grava a pasta de estado no caminho, e a pasta É o estado). Ponto único de resolução — o
// `serve` NÃO reimplementa a busca por basename, para não recriar em dois lugares o defeito de
// "ponto único por runtime" que este roadmap existe para fechar.
func ResolveRoadmapRef(cfg config.ProjectConfig, ref string) []string {
	return resolveRoadmapRef(cfg, ref)
}

// resolveRoadmapRefStatus é resolveRoadmapRef com um segundo retorno: stale=true quando a
// resolução só teve sucesso via resolveRoadmapRefByBasename (o caminho literal gravado no
// vínculo falhou e o arquivo foi encontrado em outra pasta de estado). O chamador em
// validateRefTargetsExist usa esse flag para decidir entre silêncio (literal bateu) e aviso
// de "stale state path" (fallback bateu) — ver ML-3B: um vínculo resolvido só por fallback
// ainda é uma inconsistência real (o serve, que casa edge.To pelo literal, desenha aresta
// órfã para o mesmo vínculo), então calar aqui faria validate e serve discordarem do mesmo fato.
func resolveRoadmapRefStatus(cfg config.ProjectConfig, ref string) (resolved []string, stale bool) {
	expandedRef := config.ExpandPath(normalizeRefSeparator(ref))
	if _, err := os.Stat(expandedRef); err == nil {
		return []string{expandedRef}, false
	}
	if !isStaleRoadmapStateRef(ref) {
		return nil, false
	}
	found := resolveRoadmapRefByBasename(cfg, ref)
	return found, len(found) > 0
}

func validateREQRoadmapLifecycle() ([]string, error) {
	cfg := config.Load()
	var warnings []string
	reqPaths, err := resolveREQFiles(cfg)
	if err != nil {
		return []string{err.Error()}, nil
	}
	for _, reqPath := range reqPaths {
		content, ok := readFileForRule("req_roadmap_lifecycle", reqPath, &warnings)
		if !ok {
			continue
		}
		s := string(content)
		if !reqStatusIsOpen(s) {
			continue
		}
		ref := extractRefPath(s, "Roadmap")
		if ref == "" {
			continue
		}
		resolved := resolveRoadmapRef(cfg, ref)
		if len(resolved) != 1 {
			// 0 = vínculo quebrado, já reportado por ref_targets_exist; >1 = ambíguo, já
			// reportado por ref_targets_exist. Esta regra só decide estado quando há UM
			// arquivo real de onde derivá-lo — sem isso o "continue" original (fail-open na
			// entrada errada) reaparece disfarçado de ambiguidade.
			continue
		}
		info, err := os.Stat(resolved[0])
		if err != nil || info.IsDir() {
			continue
		}
		if filepath.Base(filepath.Dir(resolved[0])) == "done" {
			warnings = append(warnings, fmt.Sprintf("req %q is Open but linked Roadmap %q is in done/", filepath.Base(reqPath), ref))
		}
	}
	return warnings, nil
}

// reqStatusIsDone verifica se a REQ está com Status: Done, priorizando o frontmatter
// (status: Done) e caindo para a linha de cabeçalho "| Status: Done" como fallback.
// Mesmo padrão de reqStatusIsOpen.
func reqStatusIsDone(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		key, val, ok := strings.Cut(trimmed, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "status") {
			val = strings.Trim(strings.TrimSpace(val), `"'`)
			// Campo de frontmatter presente mas vazio (ex.: `status: ""`) não é
			// resposta — cai para o cabeçalho em vez de decidir "não é Done" aqui.
			if val == "" {
				continue
			}
			return strings.EqualFold(val, "done")
		}
		if idx := strings.Index(trimmed, "| Status: "); idx >= 0 {
			rest := trimmed[idx+len("| Status: "):]
			if pipeIdx := strings.Index(rest, " |"); pipeIdx >= 0 {
				rest = rest[:pipeIdx]
			}
			return strings.EqualFold(strings.TrimSpace(rest), "done")
		}
	}
	return false
}

// validateADRAcceptedWhenREQDone verifica REQs com Status Done cujo ADR vinculado
// ainda não está aceito (Draft ou Proposed). Fecha a lacuna que blocked_by_draft_adr
// não cobre: essa regra só olha REQs Open no momento em que são bloqueadas; uma REQ
// que já foi concluída sem o ADR nunca ter sido aceito passava despercebida
// (ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito...).
func validateADRAcceptedWhenREQDone() ([]string, error) {
	cfg := config.Load()
	files, err := resolveREQFiles(cfg)
	if err != nil {
		return []string{err.Error()}, nil
	}

	var violations []string
	for _, path := range files {
		content, ok := readFileForRule("adr_accepted_when_req_done", path, &violations)
		if !ok {
			continue
		}
		s := string(content)
		if !reqStatusIsDone(s) {
			continue
		}
		adrRef := extractRefPath(s, "ADR")
		if adrRef == "" {
			continue
		}
		adrBasename := filepath.Base(adrRef)
		reqBasename := filepath.Base(path)
		status, ok := adrStatusForRule("adr_accepted_when_req_done", adrBasename, &violations)
		if !ok {
			continue
		}
		if statusIsNotAccepted(status) {
			violations = append(violations, fmt.Sprintf("REQ %q is Done but linked ADR %q is not accepted (status: %s)", reqBasename, adrBasename, status))
		}
	}
	return violations, nil
}

func reqStatusIsOpen(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		key, val, ok := strings.Cut(trimmed, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "status") {
			return strings.EqualFold(strings.Trim(strings.TrimSpace(val), `"'`), "open")
		}
		if idx := strings.Index(trimmed, "| Status: "); idx >= 0 {
			rest := trimmed[idx+len("| Status: "):]
			if pipeIdx := strings.Index(rest, " |"); pipeIdx >= 0 {
				rest = rest[:pipeIdx]
			}
			return strings.EqualFold(strings.TrimSpace(rest), "open")
		}
	}
	return false
}

// reqStatusValue extrai o valor bruto do campo Status de uma REQ, priorizando o
// frontmatter (status: ...) e caindo para o cabeçalho "| Status: ... |" como fallback.
// Usada pelo bloco de Inventory de GetStatus para discriminar Open/Done/Closed —
// mesmo padrão de leitura que reqStatusIsOpen/reqStatusIsDone, mas devolvendo o
// literal em vez de um bool para uma única string específica.
func reqStatusValue(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		key, val, ok := strings.Cut(trimmed, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "status") {
			v := strings.Trim(strings.TrimSpace(val), `"'`)
			if v != "" {
				return v
			}
			continue
		}
		if idx := strings.Index(trimmed, "| Status: "); idx >= 0 {
			rest := trimmed[idx+len("| Status: "):]
			if pipeIdx := strings.Index(rest, " |"); pipeIdx >= 0 {
				rest = rest[:pipeIdx]
			}
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// folderToExpectedStatus mapeia o nome da pasta para os valores de status aceitos.
var folderToExpectedStatus = map[string][]string{
	"wip":       {"WIP", "wip", "In Progress"},
	"backlog":   {"Backlog", "backlog"},
	"analyzing": {"Analyzing", "analyzing"},
	"blocked":   {"Blocked", "blocked"},
	"done":      {"Done", "done"},
	"abandoned": {"Abandoned", "abandoned"},
}

// validateFolderStatusCoherence verifica se o status declarado no frontmatter é coerente com a pasta.
func validateFolderStatusCoherence() ([]string, error) {
	cfg := config.Load()
	var warnings []string
	states := []string{"wip", "backlog", "analyzing", "blocked", "done", "abandoned"}

	type dirState struct{ path, state string }
	var dirs []dirState

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := resolveAgentNamespaces(cfg, cfg.RoadmapDir)
		for _, agent := range agents {
			for _, state := range states {
				dirs = append(dirs, dirState{
					path:  filepath.Join(cfg.RoadmapDir, agent, state),
					state: state,
				})
			}
		}
	} else {
		for _, state := range states {
			dirs = append(dirs, dirState{
				path:  filepath.Join(cfg.RoadmapDir, state),
				state: state,
			})
		}
	}

	for _, dir := range dirs {
		entries, err := listDir(dir.path)
		if err != nil {
			// P2: diretório ausente é esperado (projeto não usa esse estado);
			// qualquer outro erro (ENOTDIR, EPERM…) deve ser reportado.
			if !os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf(
					"folder_status: could not read directory %q: %v", dir.path, err,
				))
			}
			continue
		}
		for _, name := range entries {
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			content, ok := readFileForRule("folder_status", filepath.Join(dir.path, name), &warnings)
			if !ok {
				continue
			}
			declared := extractFrontmatterField(string(content), "status")
			if declared == "" {
				continue
			}
			expected := folderToExpectedStatus[dir.state]
			found := false
			for _, e := range expected {
				if strings.EqualFold(declared, e) {
					found = true
					break
				}
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf(
					"roadmap %q: folder is %q but status declares %q", name, dir.state, declared,
				))
			}
		}
	}
	return warnings, nil
}

// validateFilenameUniqueness detecta o mesmo filename em múltiplos estados.
func validateFilenameUniqueness() ([]string, error) {
	cfg := config.Load()
	states := []string{"wip", "backlog", "analyzing", "blocked", "done", "abandoned"}

	seen := map[string][]string{}

	var listErrors []string
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := resolveAgentNamespaces(cfg, cfg.RoadmapDir)
		for _, agent := range agents {
			for _, state := range states {
				dir := filepath.Join(cfg.RoadmapDir, agent, state)
				names, err := listDir(dir)
				if err != nil {
					// P2: apenas reportar erros que não sejam "diretório ausente".
					if !os.IsNotExist(err) {
						listErrors = append(listErrors, fmt.Sprintf(
							"filename_uniqueness: could not read directory %q: %v", dir, err,
						))
					}
					continue
				}
				for _, name := range names {
					key := agent + "/" + name
					seen[key] = append(seen[key], state)
				}
			}
		}
	} else {
		for _, state := range states {
			dir := filepath.Join(cfg.RoadmapDir, state)
			names, err := listDir(dir)
			if err != nil {
				if !os.IsNotExist(err) {
					listErrors = append(listErrors, fmt.Sprintf(
						"filename_uniqueness: could not read directory %q: %v", dir, err,
					))
				}
				continue
			}
			for _, name := range names {
				seen[name] = append(seen[name], state)
			}
		}
	}

	var violations []string
	violations = append(violations, listErrors...)
	// P3: ordenar a lista de estados em cada mensagem e depois as próprias mensagens
	// para garantir saída determinística independente de ordem de iteração do mapa.
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		stateList := seen[name]
		if len(stateList) > 1 {
			sorted := make([]string, len(stateList))
			copy(sorted, stateList)
			sort.Strings(sorted)
			violations = append(violations, fmt.Sprintf(
				"roadmap %q appears in multiple states: %v", name, sorted,
			))
		}
	}
	return violations, nil
}

// BranchSlugMatchesRoadmap verifica se branchSlug (já normalizado via normalizeBranchSlug) casa com o
// nome de algum roadmap .md encontrado em wipDirs ou doneDirs. Reutilizada por
// validateBranchHasWIPRoadmap e pelo comando `trackfw branch new` — nunca duplicar esta lógica.
//
// matched indica se algum candidato casou com o slug. candidates lista todos os roadmaps .md
// encontrados em wipDirs+doneDirs (para diagnóstico/mensagem de orientação quando matched é false).
func BranchSlugMatchesRoadmap(branchSlug string, wipDirs, doneDirs []string) (matched bool, candidates []string) {
	dirs := append(append([]string{}, wipDirs...), doneDirs...)
	for _, dir := range dirs {
		entries, _ := listDir(dir)
		for _, name := range entries {
			if strings.HasSuffix(name, ".md") {
				candidates = append(candidates, name)
				if strings.Contains(normalizeBranchSlug(name), branchSlug) {
					matched = true
				}
			}
		}
	}
	return matched, candidates
}

// validateBranchHasWIPRoadmap verifica se a branch atual (feat/fix/refactor) tem ao menos um roadmap em wip/.
// Retorna violation se a branch for de implementação mas wip/ estiver vazio — previne trabalho órfão.
func validateBranchHasWIPRoadmap() ([]string, error) {
	branch := firstNonEmpty(os.Getenv("TRACKFW_BRANCH"))
	if branch == "" && isGitWorktree(".") {
		cmd := gitCommand(".", "symbolic-ref", "--short", "HEAD")
		out, err := cmd.Output()
		if err == nil {
			branch = strings.TrimSpace(string(out))
		}
		if branch == "" {
			branch = firstNonEmpty(
				os.Getenv("GITHUB_HEAD_REF"),
				os.Getenv("CI_COMMIT_REF_NAME"),
				os.Getenv("GITHUB_REF_NAME"),
			)
		}
	}
	if !strings.HasPrefix(branch, "feat/") && !strings.HasPrefix(branch, "fix/") && !strings.HasPrefix(branch, "refactor/") {
		return nil, nil // só enforça em branches de implementação
	}

	cfg := config.Load()
	wipDirs := resolveWIPDirs(cfg)
	doneDirs := resolveDoneDirs(cfg)

	branchSlug := normalizeBranchSlug(strings.SplitN(branch, "/", 2)[1])
	matched, candidates := BranchSlugMatchesRoadmap(branchSlug, wipDirs, doneDirs)
	if matched {
		return nil, nil
	}

	if len(candidates) == 0 {
		return []string{BranchGovernanceOrientation(branch, cfg)}, nil
	}
	return []string{BranchNoMatchingRoadmapMessage(branch, candidates)}, nil
}

// BranchGovernanceOrientation is the guidance message printed when a feat/fix/refactor branch
// has no roadmap in wip/ nor done/ at all (candidates is empty). Shared by
// validateBranchHasWIPRoadmap and `trackfw branch new` — never duplicate this string.
// For by_agent projects with 2+ agents, the hint includes --agent so the user does not run
// the command that now requires a flag (AC13, ML-3A).
func BranchGovernanceOrientation(branch string, cfg config.ProjectConfig) string {
	return fmt.Sprintf(
		"branch %q is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ — create governance artifacts first:\n  %s\n  %s\n  trackfw roadmap move <name> wip",
		branch, ReqNewLine(cfg), RoadmapNewLine(cfg),
	)
}

// BranchNoMatchingRoadmapMessage is the guidance message printed when roadmaps exist in wip/ or
// done/ but none of them match the branch's slug. Shared by validateBranchHasWIPRoadmap and
// `trackfw branch new` — never duplicate this string. Does not mutate candidates.
func BranchNoMatchingRoadmapMessage(branch string, candidates []string) string {
	// P3: sort for deterministic output regardless of filesystem ordering.
	sorted := make([]string, len(candidates))
	copy(sorted, candidates)
	sort.Strings(sorted)
	display := sorted
	suffix := ""
	if len(sorted) > 3 {
		display = sorted[:3]
		suffix = fmt.Sprintf(", e mais %d", len(sorted)-3)
	}
	return fmt.Sprintf(
		"branch %q has no matching roadmap in wip/ nor done/ (found: %s%s) — include the branch slug in the roadmap filename or set TRACKFW_BRANCH explicitly in CI",
		branch, strings.Join(display, ", "), suffix,
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isGitWorktree(dir string) bool {
	out, err := gitCommand(dir, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// validateNoteOrphan detecta notas em vault/notes/ não referenciadas pelo index.md.
// Severidade default: warning (ver ruleDefaults). Pode ser elevada a "error" via rules: no trackfw.yaml.
// index.md não conta como nota órfã. Projeto sem vault/ não gera nenhum warning.
func validateNoteOrphan() ([]string, error) {
	vaultNotesDir := "vault/notes"
	indexPath := "vault/notes/index.md"

	// Vault ausente = sem warnings
	if _, err := os.Stat(vaultNotesDir); os.IsNotExist(err) {
		return nil, nil
	}

	matches, err := filepath.Glob(filepath.Join(vaultNotesDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("listando vault/notes: %w", err)
	}

	// Lê conteúdo do index.md (pode não existir ainda — estado legítimo, sem diagnóstico). Erro
	// que não seja "ausente" (permissão negada, FIFO, etc.) é diagnosticado e a regra para aqui —
	// tratar como índice vazio produziria um "not referenced" falso para toda nota existente,
	// escondendo a causa real (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-
	// deixa-de-ser-silencio, ML-1G).
	var msgs []string
	var indexContent string
	data, err := readRegularFile(indexPath)
	switch {
	case err == nil:
		indexContent = string(data)
	case os.IsNotExist(err):
		// ausência legítima — sem diagnóstico, index.md ainda não foi criado.
	default:
		return []string{inspectionDiagnostic("note_orphan", indexPath, err)}, nil
	}
	for _, match := range matches {
		basename := filepath.Base(match)
		if basename == "index.md" {
			continue
		}
		nameWithoutExt := strings.TrimSuffix(basename, ".md")
		// aceita link markdown `[texto](arquivo.md)` ou wikilink `[[nome]]`
		referenced := strings.Contains(indexContent, "("+basename+")") ||
			strings.Contains(indexContent, "[["+nameWithoutExt+"]]") ||
			strings.Contains(indexContent, "[["+basename+"]]")
		if !referenced {
			msgs = append(msgs, fmt.Sprintf("note %q is not referenced in vault/notes/index.md", basename))
		}
	}
	return msgs, nil
}

// NormalizeBranchSlug é o wrapper exportado de normalizeBranchSlug, usado por consumidores fora do
// pacote validator (ex: comando `trackfw branch new`).
func NormalizeBranchSlug(value string) string {
	return normalizeBranchSlug(value)
}

func normalizeBranchSlug(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

// GovernanceViolation holds the messages from a failed CheckShipGovernance call.
type GovernanceViolation struct {
	// Missing contains human-readable violation messages, one per line.
	Missing []string
}

func (e *GovernanceViolation) Error() string {
	return strings.Join(e.Missing, "\n")
}

// CheckShipGovernance is the hard gate called by `trackfw ship` at step 2.
// Unlike Validate(), it bypasses the baseline ratchet, lenient mode and
// per-rule severity configuration — the ship command must always enforce
// governance regardless of project settings.
//
// It checks:
//  1. The current branch has a matching roadmap in wip/ or done/ (branch_has_wip_roadmap)
//  2. All WIP roadmaps have a linked REQ (wip_has_req)
//
// Returns nil when all checks pass. Returns *GovernanceViolation otherwise.
func CheckShipGovernance() *GovernanceViolation {
	var missing []string

	branchViolations, _ := validateBranchHasWIPRoadmap()
	missing = append(missing, branchViolations...)

	wipReqViolations, _ := validateWIPHasREQ()
	missing = append(missing, wipReqViolations...)

	if len(missing) == 0 {
		return nil
	}
	return &GovernanceViolation{Missing: missing}
}
