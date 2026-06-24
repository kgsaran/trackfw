package validator

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
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
	data, err := os.ReadFile(baselineFileName)
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
	return os.WriteFile(baselineFileName, data, 0644)
}

const staleWIPDays = 7

// contentHasMarker retorna true se content contém algum dos marcadores com valor não-vazio.
func contentHasMarker(content string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(content, marker) && !strings.Contains(content, marker+" \n") {
			return true
		}
	}
	return false
}

// ruleSeverity retorna a severidade configurada para a regra ou "error" como fallback.
func ruleSeverity(name string) string {
	cfg := config.Load()
	if s, ok := cfg.Rules[name]; ok {
		return s
	}
	return "error"
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

// WIPConfig armazena configuração de WIP limit lida do trackfw.yaml.
type WIPConfig struct {
	Limit   int  // default 1
	BySquad bool // default false
}

// readWIPConfig lê wip_limit e wip_by_squad do trackfw.yaml no CWD.
// Retorna {Limit: 1, BySquad: false} se o arquivo não existe ou os campos estão ausentes.
func readWIPConfig() WIPConfig {
	cfg := WIPConfig{Limit: 1, BySquad: false}
	content, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		return cfg
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "wip_limit:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "wip_limit:"))
			fields := strings.Fields(val)
			if len(fields) > 0 {
				var n int
				if _, err := fmt.Sscanf(fields[0], "%d", &n); err == nil && n > 0 {
					cfg.Limit = n
				}
			}
		}
		if strings.HasPrefix(line, "wip_by_squad:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "wip_by_squad:"))
			fields := strings.Fields(val)
			if len(fields) > 0 && fields[0] == "true" {
				cfg.BySquad = true
			}
		}
	}
	return cfg
}

// parseSquadFromFrontmatter lê um arquivo markdown e extrai o valor da linha "squad: <valor>".
// Retorna string vazia se o campo está ausente ou vazio.
func parseSquadFromFrontmatter(path string) string {
	content, err := os.ReadFile(path)
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
		agents := projectCfg.Agents
		if len(agents) == 0 {
			entries, readErr := os.ReadDir(projectCfg.RoadmapDir)
			if readErr == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
		wipCfg := readWIPConfig()
		for _, agent := range agents {
			files, globErr := filepath.Glob(filepath.Join(projectCfg.RoadmapDir, agent, "wip", "*.md"))
			if globErr != nil {
				return nil, nil, globErr
			}
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

	wipCfg := readWIPConfig()

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

// readGovernanceMode lê os campos governance_mode e lenient_until do trackfw.yaml no CWD.
// Retorna GovernanceMode{Mode: "strict"} se o arquivo não existe ou os campos estão ausentes.
func readGovernanceMode() GovernanceMode {
	content, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		return GovernanceMode{Mode: "strict"}
	}
	gm := GovernanceMode{Mode: "strict"}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "governance_mode:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "governance_mode:"))
			// Pegar apenas a primeira palavra (ignorar comentários inline)
			fields := strings.Fields(val)
			if len(fields) > 0 {
				gm.Mode = fields[0]
			}
		}
		if strings.HasPrefix(line, "lenient_until:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "lenient_until:"))
			fields := strings.Fields(val)
			if len(fields) > 0 {
				t, parseErr := time.Parse("2006-01-02", fields[0])
				if parseErr == nil {
					gm.LenientUntil = t
				}
			}
		}
	}
	return gm
}

// IsLenient retorna true se o projeto está em modo lenient e o prazo ainda não expirou.
func IsLenient() bool {
	gm := readGovernanceMode()
	if gm.Mode != "lenient" {
		return false
	}
	if gm.LenientUntil.IsZero() {
		return true
	}
	return time.Now().Before(gm.LenientUntil)
}

// LenientUntilDate retorna a data de expiração do modo lenient formatada em "2006-01-02".
// Retorna string vazia se o modo não for lenient ou a data não estiver definida.
func LenientUntilDate() string {
	gm := readGovernanceMode()
	if gm.Mode != "lenient" || gm.LenientUntil.IsZero() {
		return ""
	}
	return gm.LenientUntil.Format("2006-01-02")
}

// ValidateUnfiltered executa todas as validações sem filtro de baseline nem modo lenient.
// Use para criar snapshots de baseline ou quando você quer o quadro completo.
func ValidateUnfiltered() (violations []string, warnings []string, err error) {
	cfg := config.Load()

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

	frontmatterViolations := validateFrontmatterPresence()
	violations = append(violations, frontmatterViolations...) // sem regra configurável

	refWarnings, e := validateRefTargetsExist()
	if e != nil {
		return nil, nil, e
	}
	applyRule("ref_targets_exist", refWarnings, &violations, &warnings)

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

	// v2.5: verificação bidirecional REQ↔Roadmap via trace_id_field (desativada se campo vazio)
	traceViolations, traceWarnings := validateTraceId(cfg)
	violations = append(violations, traceViolations...)
	warnings = append(warnings, traceWarnings...)

	return violations, warnings, nil
}

func Validate() (violations []string, warnings []string, err error) {
	violations, warnings, err = ValidateUnfiltered()
	if err != nil {
		return
	}

	// Aplicar filtro de baseline (ratchet): falha somente em violations novas
	baseline, bErr := LoadBaseline()
	if bErr != nil {
		return nil, nil, fmt.Errorf("erro ao carregar baseline: %w", bErr)
	}
	if baseline != nil {
		baselineSet := make(map[string]struct{}, len(baseline.Violations))
		for _, v := range baseline.Violations {
			baselineSet[v] = struct{}{}
		}
		var netNew []string
		for _, v := range violations {
			if _, exists := baselineSet[v]; !exists {
				netNew = append(netNew, v)
			}
		}
		violations = netNew

		warnSet := make(map[string]struct{}, len(baseline.Warnings))
		for _, w := range baseline.Warnings {
			warnSet[w] = struct{}{}
		}
		var netNewWarn []string
		for _, w := range warnings {
			if _, exists := warnSet[w]; !exists {
				netNewWarn = append(netNewWarn, w)
			}
		}
		warnings = netNewWarn
	}

	// Modo lenient: mover violations para warnings, exit code 0
	if IsLenient() {
		warnings = append(warnings, violations...)
		violations = nil
	}

	return
}

// validateUnfilteredTagged é a versão interna de ValidateUnfiltered que retorna TaggedMsg.
// Regras sem applyRuleTagged (diretas) ficam com Rule="" — comportamento intencional.
func validateUnfilteredTagged() (violations []TaggedMsg, warnings []TaggedMsg, err error) {
	cfg := config.Load()

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

	frontmatterViolations := validateFrontmatterPresence()
	for _, m := range frontmatterViolations {
		violations = append(violations, TaggedMsg{Rule: "", Msg: m})
	}

	refWarnings, e := validateRefTargetsExist()
	if e != nil {
		return nil, nil, e
	}
	applyRuleTagged("ref_targets_exist", refWarnings, &violations, &warnings)

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

	// v2.5: traceid — applyRuleTagged está no validator_traceid via applyRule; aqui fazemos tagged
	traceViolations, traceWarnings := validateTraceId(cfg)
	for _, m := range traceViolations {
		violations = append(violations, TaggedMsg{Rule: extractRulePrefix(m), Msg: m})
	}
	for _, m := range traceWarnings {
		warnings = append(warnings, TaggedMsg{Rule: extractRulePrefix(m), Msg: m})
	}

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

	// Filtro de baseline: excluir violations/warnings já conhecidos (por mensagem).
	baseline, bErr := LoadBaseline()
	if bErr != nil {
		return nil, nil, fmt.Errorf("erro ao carregar baseline: %w", bErr)
	}
	if baseline != nil {
		baselineSet := make(map[string]struct{}, len(baseline.Violations))
		for _, v := range baseline.Violations {
			baselineSet[v] = struct{}{}
		}
		var netNew []TaggedMsg
		for _, v := range violations {
			if _, exists := baselineSet[v.Msg]; !exists {
				netNew = append(netNew, v)
			}
		}
		violations = netNew

		warnSet := make(map[string]struct{}, len(baseline.Warnings))
		for _, w := range baseline.Warnings {
			warnSet[w] = struct{}{}
		}
		var netNewWarn []TaggedMsg
		for _, w := range warnings {
			if _, exists := warnSet[w.Msg]; !exists {
				netNewWarn = append(netNewWarn, w)
			}
		}
		warnings = netNewWarn
	}

	// Modo lenient: mover violations para warnings, exit code 0.
	if IsLenient() {
		warnings = append(warnings, violations...)
		violations = nil
	}

	return
}

func GetStatus() (string, error) {
	cfg := config.Load()
	var sb strings.Builder
	sb.WriteString("── trackfw status ──────────────────────\n")

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, err := os.ReadDir(cfg.RoadmapDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
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

		wipCfg := readWIPConfig()
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

		// Seção: REQs bloqueadas por ADRs Draft
		blockedByDraft, err := blockedREQs()
		if err == nil && len(blockedByDraft) > 0 {
			sb.WriteString(fmt.Sprintf("\n⏳ REQs blocked by Draft ADRs (%d)\n", len(blockedByDraft)))
			for reqFile, adrs := range blockedByDraft {
				sb.WriteString(fmt.Sprintf("   %s\n", reqFile))
				for _, adr := range adrs {
					sb.WriteString(fmt.Sprintf("     → %s (Draft)\n", adr))
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

// resolveWIPDirs retorna todos os diretórios wip/ conforme o modo de namespacing.
func resolveWIPDirs(cfg config.ProjectConfig) []string {
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, err := os.ReadDir(cfg.RoadmapDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
		var dirs []string
		for _, agent := range agents {
			dirs = append(dirs, cfg.RoadmapDir+"/"+agent+"/wip")
		}
		return dirs
	}
	return []string{cfg.RoadmapDir + "/wip"}
}

// resolveREQFiles retorna paths completos de todos os .md em req_dir,
// consciente de roadmap_namespacing: by_agent percorre req_dir/<agente>/<estado>/.
func resolveREQFiles(cfg config.ProjectConfig) []string {
	reqDir := cfg.REQDir
	if reqDir == "" {
		return nil
	}
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		stateDirs := []string{"backlog", "wip", "blocked", "done", "abandoned"}
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, err := os.ReadDir(reqDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
		var files []string
		for _, agent := range agents {
			for _, state := range stateDirs {
				pattern := filepath.Join(reqDir, agent, state, "*.md")
				matches, err := filepath.Glob(pattern)
				if err == nil {
					files = append(files, matches...)
				}
			}
		}
		return files
	}
	// flat (comportamento anterior)
	matches, err := filepath.Glob(filepath.Join(reqDir, "*.md"))
	if err != nil {
		return nil
	}
	return matches
}

func validateWIPHasREQ() ([]string, error) {
	cfg := config.Load()
	wipDirs := resolveWIPDirs(cfg)

	var violations []string
	for _, wipDir := range wipDirs {
		entries, err := listDir(wipDir)
		if err != nil {
			continue
		}
		for _, name := range entries {
			content, err := os.ReadFile(filepath.Join(wipDir, name))
			if err != nil {
				continue
			}
			if !contentHasMarker(string(content), cfg.LinkFieldsReq) {
				violations = append(violations, fmt.Sprintf("roadmap %q is in wip but has no linked REQ", name))
			}
		}
	}
	return violations, nil
}

func validateREQsHaveADR() ([]string, error) {
	cfg := config.Load()
	files := resolveREQFiles(cfg)

	var violations []string
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !contentHasMarker(string(content), cfg.LinkFieldsADR) {
			violations = append(violations, fmt.Sprintf("req %q has no linked ADR", filepath.Base(path)))
		}
	}
	return violations, nil
}

func validateBlockedHasREQ() ([]string, error) {
	cfg := config.Load()
	entries, err := listDir(cfg.RoadmapDir + "/blocked")
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join(cfg.RoadmapDir+"/blocked", name))
		if err != nil {
			continue
		}
		if !contentHasMarker(string(content), cfg.LinkFieldsReq) {
			violations = append(violations, fmt.Sprintf("roadmap %q is in blocked but has no linked REQ", name))
		}
	}
	return violations, nil
}

func validateREQsHaveRoadmap() ([]string, error) {
	cfg := config.Load()
	files := resolveREQFiles(cfg)

	var violations []string
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !contentHasMarker(string(content), cfg.LinkFieldsRoadmap) {
			violations = append(violations, fmt.Sprintf("req %q has no linked Roadmap", filepath.Base(path)))
		}
	}
	return violations, nil
}

func validateADRsAreReferenced() ([]string, error) {
	cfg := config.Load()
	var adrs []string
	for _, adrDir := range cfg.ADRDirs {
		names := walkADRFiles(adrDir)
		adrs = append(adrs, names...)
	}

	reqPaths := resolveREQFiles(cfg)
	var allREQContent strings.Builder
	for _, p := range reqPaths {
		b, err := os.ReadFile(p)
		if err == nil {
			allREQContent.Write(b)
		}
	}
	combined := allREQContent.String()

	var violations []string
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
		entries, err := listDir(wipDir)
		if err != nil {
			continue
		}
		for _, name := range entries {
			content, err := os.ReadFile(filepath.Join(wipDir, name))
			if err != nil {
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

	var warnings []string
	for _, wipDir := range wipDirs {
		entries, err := filepath.Glob(wipDir + "/*.md")
		if err != nil {
			continue
		}
		for _, path := range entries {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			modTime := info.ModTime()
			if gitTime, ok := gitLastModifiedTime(path); ok {
				modTime = gitTime
			}
			age := time.Since(modTime)
			days := int(age.Hours() / 24)
			if days >= staleWIPDays {
				warnings = append(warnings, fmt.Sprintf(
					"roadmap/wip/%s has been in WIP for %d days (last modified %s)",
					filepath.Base(path), days, modTime.Format("2006-01-02"),
				))
			}
		}
	}
	return warnings, nil
}

// blockedREQs retorna um mapa de REQ-basename → lista de ADR-basenames Draft que a bloqueiam.
// Somente REQs com Status: Open e ADRs com Status: Draft são incluídas.
func blockedREQs() (map[string][]string, error) {
	cfg := config.Load()
	files := resolveREQFiles(cfg)

	result := make(map[string][]string)
	for _, reqPath := range files {
		content, err := os.ReadFile(reqPath)
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
		var draftADRs []string
		for _, adrBasename := range adrNames {
			if adrIsDraft(adrBasename) {
				draftADRs = append(draftADRs, adrBasename)
			}
		}
		if len(draftADRs) > 0 {
			result[filepath.Base(reqPath)] = draftADRs
		}
	}
	return result, nil
}

// validateREQsNotBlockedByDraftADRs verifica se REQs com Status Open têm ADRs Draft vinculados.
// Uma REQ Open com ADR Draft é uma violação: o roadmap não pode ser criado até os ADRs serem aceitos.
func validateREQsNotBlockedByDraftADRs() ([]string, error) {
	cfg := config.Load()
	entries := resolveREQFiles(cfg)

	var violations []string
	for _, path := range entries {
		content, err := os.ReadFile(path)
		if err != nil {
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
			continue
		}
		reqBasename := filepath.Base(path)
		for _, adrBasename := range blockedADRs {
			if adrIsDraft(adrBasename) {
				violations = append(violations, fmt.Sprintf("REQ %s is blocked by Draft ADR: %s", reqBasename, adrBasename))
			}
		}
	}
	return violations, nil
}

// parseBlockedADRs extrai os basenames de ADRs listados na seção "## Blocked by ADRs" de um arquivo REQ.
func parseBlockedADRs(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")

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

// adrIsDraft verifica se o ADR identificado pelo basename contém "Status: Draft".
// Busca recursivamente em todas as ADRDirs configuradas.
func adrIsDraft(adrBasename string) bool {
	cfg := config.Load()
	p := findADRFile(adrBasename, cfg.ADRDirs)
	if p == "" {
		return false
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "Status: Draft")
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
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			if !strings.HasPrefix(string(content), "---") {
				violations = append(violations, fmt.Sprintf("adr %q has no frontmatter block", basename))
			}
		}
	}

	// REQs — usa resolveREQFiles para suportar namespacing by_agent
	reqFiles := resolveREQFiles(cfg)
	for _, f := range reqFiles {
		content, err := os.ReadFile(f)
		if err != nil {
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

// walkADRFiles retorna basenames de todos os arquivos .md encontrados recursivamente em adrDir.
func walkADRFiles(adrDir string) []string {
	var names []string
	_ = filepath.WalkDir(adrDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			names = append(names, filepath.Base(path))
		}
		return nil
	})
	return names
}

// findADRFile busca um arquivo pelo basename recursivamente em todos os adrDirs.
// Retorna o caminho completo ou string vazia se não encontrado.
func findADRFile(adrBasename string, adrDirs []string) string {
	for _, adrDir := range adrDirs {
		var found string
		_ = filepath.WalkDir(adrDir, func(path string, d fs.DirEntry, err error) error {
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
	cmd := exec.Command("git", "log", "-1", "--format=%ct", "--", path)
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
// Retorna string vazia se o campo estiver ausente, vazio ou com valor traço.
func extractRefPath(content, field string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		prefix := field + ":"
		if strings.HasPrefix(trimmed, prefix) {
			val := strings.TrimSpace(trimmed[len(prefix):])
			if val == "" || val == "—" || val == "-" || val == "–" {
				return ""
			}
			fields := strings.Fields(val)
			if len(fields) == 0 {
				return ""
			}
			v := fields[0]
			if strings.HasSuffix(v, ".md") {
				return v
			}
		}
	}
	return ""
}

// validateRefTargetsExist verifica se arquivos referenciados via REQ:, ADR: e Roadmap: existem.
func validateRefTargetsExist() ([]string, error) {
	cfg := config.Load()
	var warnings []string

	wipDirs := resolveWIPDirs(cfg)
	blockedDir := cfg.RoadmapDir + "/blocked"
	for _, dir := range append(wipDirs, blockedDir) {
		entries, _ := listDir(dir)
		for _, name := range entries {
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			if ref := extractRefPath(string(content), "REQ"); ref != "" {
				if !referenceExists(ref, []string{cfg.REQDir}) {
					warnings = append(warnings, fmt.Sprintf("roadmap %q links to REQ %q which does not exist", name, ref))
				}
			}
		}
	}

	reqFiles := resolveREQFiles(cfg)
	for _, reqPath := range reqFiles {
		content, err := os.ReadFile(reqPath)
		if err != nil {
			continue
		}
		s := string(content)
		name := filepath.Base(reqPath)
		if ref := extractRefPath(s, "ADR"); ref != "" {
			if !referenceExists(ref, cfg.ADRDirs) {
				warnings = append(warnings, fmt.Sprintf("req %q links to ADR %q which does not exist", name, ref))
			}
		}
		if ref := extractRefPath(s, "Roadmap"); ref != "" {
			if !referenceExists(ref, []string{cfg.RoadmapDir}) {
				warnings = append(warnings, fmt.Sprintf("req %q links to Roadmap %q which does not exist", name, ref))
			}
		}
	}
	return warnings, nil
}

func referenceExists(ref string, roots []string) bool {
	if _, err := os.Stat(ref); err == nil {
		return true
	}
	base := filepath.Base(ref)
	for _, root := range roots {
		found := false
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && entry.Name() == base {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}
	return false
}

// folderToExpectedStatus mapeia o nome da pasta para os valores de status aceitos.
var folderToExpectedStatus = map[string][]string{
	"wip":       {"WIP", "wip", "In Progress"},
	"backlog":   {"Backlog", "backlog"},
	"blocked":   {"Blocked", "blocked"},
	"done":      {"Done", "done"},
	"abandoned": {"Abandoned", "abandoned"},
}

// validateFolderStatusCoherence verifica se o status declarado no frontmatter é coerente com a pasta.
func validateFolderStatusCoherence() ([]string, error) {
	cfg := config.Load()
	var warnings []string
	states := []string{"wip", "backlog", "blocked", "done", "abandoned"}

	type dirState struct{ path, state string }
	var dirs []dirState

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, _ := os.ReadDir(cfg.RoadmapDir)
			for _, e := range entries {
				if e.IsDir() {
					agents = append(agents, e.Name())
				}
			}
		}
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
		entries, _ := listDir(dir.path)
		for _, name := range entries {
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir.path, name))
			if err != nil {
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
	states := []string{"wip", "backlog", "blocked", "done", "abandoned"}

	seen := map[string][]string{}

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, _ := os.ReadDir(cfg.RoadmapDir)
			for _, e := range entries {
				if e.IsDir() {
					agents = append(agents, e.Name())
				}
			}
		}
		for _, agent := range agents {
			for _, state := range states {
				dir := filepath.Join(cfg.RoadmapDir, agent, state)
				names, _ := listDir(dir)
				for _, name := range names {
					key := agent + "/" + name
					seen[key] = append(seen[key], state)
				}
			}
		}
	} else {
		for _, state := range states {
			dir := filepath.Join(cfg.RoadmapDir, state)
			names, _ := listDir(dir)
			for _, name := range names {
				seen[name] = append(seen[name], state)
			}
		}
	}

	var violations []string
	for name, stateList := range seen {
		if len(stateList) > 1 {
			violations = append(violations, fmt.Sprintf(
				"roadmap %q appears in multiple states: %v", name, stateList,
			))
		}
	}
	return violations, nil
}

// validateBranchHasWIPRoadmap verifica se a branch atual (feat/fix/refactor) tem ao menos um roadmap em wip/.
// Retorna violation se a branch for de implementação mas wip/ estiver vazio — previne trabalho órfão.
func validateBranchHasWIPRoadmap() ([]string, error) {
	branch := firstNonEmpty(os.Getenv("TRACKFW_BRANCH"))
	if branch == "" && isGitWorktree(".") {
		cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
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

	branchSlug := normalizeBranchSlug(strings.SplitN(branch, "/", 2)[1])
	var wipFiles []string
	for _, wipDir := range wipDirs {
		entries, _ := listDir(wipDir)
		for _, name := range entries {
			if strings.HasSuffix(name, ".md") {
				wipFiles = append(wipFiles, name)
				if strings.Contains(normalizeBranchSlug(name), branchSlug) {
					return nil, nil
				}
			}
		}
	}

	if len(wipFiles) == 0 {
		return []string{fmt.Sprintf(
			"branch %q is a feat/fix/refactor branch but no roadmap is in wip/ — create governance artifacts first:\n  trackfw req new \"title\"\n  trackfw roadmap new \"title\"\n  trackfw roadmap move <name> wip",
			branch,
		)}, nil
	}
	return []string{fmt.Sprintf(
		"branch %q has no matching roadmap in wip/ (found: %s) — include the branch slug in the roadmap filename or set TRACKFW_BRANCH explicitly in CI",
		branch, strings.Join(wipFiles, ", "),
	)}, nil
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
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
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
