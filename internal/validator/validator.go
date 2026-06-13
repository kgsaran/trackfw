package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

const staleWIPDays = 7

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

func Validate() (violations []string, warnings []string, err error) {
	wipViolations, e := validateWIPHasREQ()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, wipViolations...)

	reqViolations, e := validateREQsHaveADR()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, reqViolations...)

	blockedViolations, e := validateBlockedHasREQ()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, blockedViolations...)

	reqRoadmapViolations, e := validateREQsHaveRoadmap()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, reqRoadmapViolations...)

	adrOrphanViolations, e := validateADRsAreReferenced()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, adrOrphanViolations...)

	criteriaViolations, e := validateWIPHasAcceptanceCriteria()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, criteriaViolations...)

	wipViolationsLimit, wipWarningsLimit, e := validateWIPLimit()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, wipViolationsLimit...)
	warnings = append(warnings, wipWarningsLimit...)

	staleWarnings, e := validateStaleWIP()
	if e != nil {
		return nil, nil, e
	}
	warnings = append(warnings, staleWarnings...)

	draftBlockedViolations, e := validateREQsNotBlockedByDraftADRs()
	if e != nil {
		return nil, nil, e
	}
	violations = append(violations, draftBlockedViolations...)

	frontmatterViolations := validateFrontmatterPresence()
	violations = append(violations, frontmatterViolations...)

	// Modo lenient: mover violations para warnings, exit code 0
	if IsLenient() {
		warnings = append(warnings, violations...)
		violations = nil
	}

	return violations, warnings, nil
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
			if !strings.Contains(string(content), "REQ:") || strings.Contains(string(content), "REQ: \n") {
				violations = append(violations, fmt.Sprintf("roadmap %q is in wip but has no linked REQ", name))
			}
		}
	}
	return violations, nil
}

func validateREQsHaveADR() ([]string, error) {
	cfg := config.Load()
	entries, err := listDir(cfg.REQDir)
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join(cfg.REQDir, name))
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "ADR:") || strings.Contains(string(content), "ADR: \n") {
			violations = append(violations, fmt.Sprintf("req %q has no linked ADR", name))
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
		if !strings.Contains(string(content), "REQ:") || strings.Contains(string(content), "REQ: \n") {
			violations = append(violations, fmt.Sprintf("roadmap %q is in blocked but has no linked REQ", name))
		}
	}
	return violations, nil
}

func validateREQsHaveRoadmap() ([]string, error) {
	cfg := config.Load()
	entries, err := listDir(cfg.REQDir)
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join(cfg.REQDir, name))
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "Roadmap:") || strings.Contains(string(content), "Roadmap: \n") {
			violations = append(violations, fmt.Sprintf("req %q has no linked Roadmap", name))
		}
	}
	return violations, nil
}

func validateADRsAreReferenced() ([]string, error) {
	cfg := config.Load()
	var adrs []string
	for _, adrDir := range cfg.ADRDirs {
		names, err := listDir(adrDir)
		if err != nil {
			continue
		}
		adrs = append(adrs, names...)
	}

	reqs, err := os.ReadDir(cfg.REQDir)
	if err != nil {
		return nil, nil
	}

	var allREQContent strings.Builder
	for _, r := range reqs {
		if r.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(cfg.REQDir, r.Name()))
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
			hasBlock := strings.Contains(s, "## Acceptance Criteria") ||
				strings.Contains(s, "## Critérios de Aceite") ||
				strings.Contains(s, "acceptance criteria") ||
				strings.Contains(s, "Acceptance Criteria:")
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
			age := time.Since(info.ModTime())
			days := int(age.Hours() / 24)
			if days >= staleWIPDays {
				warnings = append(warnings, fmt.Sprintf(
					"roadmap/wip/%s has been in WIP for %d days (last modified %s)",
					filepath.Base(path), days, info.ModTime().Format("2006-01-02"),
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
	entries, err := listDir(cfg.REQDir)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, name := range entries {
		reqPath := filepath.Join(cfg.REQDir, name)
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
			result[name] = draftADRs
		}
	}
	return result, nil
}

// validateREQsNotBlockedByDraftADRs verifica se REQs com Status Open têm ADRs Draft vinculados.
// Uma REQ Open com ADR Draft é uma violação: o roadmap não pode ser criado até os ADRs serem aceitos.
func validateREQsNotBlockedByDraftADRs() ([]string, error) {
	cfg := config.Load()
	entries, err := filepath.Glob(filepath.Join(cfg.REQDir, "*.md"))
	if err != nil {
		return nil, err
	}

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
// Busca em todas as ADRDirs configuradas.
func adrIsDraft(adrBasename string) bool {
	cfg := config.Load()
	for _, adrDir := range cfg.ADRDirs {
		path := filepath.Join(adrDir, adrBasename)
		content, err := os.ReadFile(path)
		if err == nil {
			return strings.Contains(string(content), "Status: Draft")
		}
	}
	return false
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

	// ADRs
	for _, adrDir := range cfg.ADRDirs {
		files, _ := filepath.Glob(filepath.Join(adrDir, "*.md"))
		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			if !strings.HasPrefix(string(content), "---") {
				violations = append(violations, fmt.Sprintf("adr %q has no frontmatter block", filepath.Base(f)))
			}
		}
	}

	// REQs
	reqFiles, _ := filepath.Glob(filepath.Join(cfg.REQDir, "*.md"))
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
