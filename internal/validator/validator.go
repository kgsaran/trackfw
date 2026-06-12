package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const staleWIPDays = 7

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

	wipWarnings, e := validateSingleWIP()
	if e != nil {
		return nil, nil, e
	}
	warnings = append(warnings, wipWarnings...)

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

	return violations, warnings, nil
}

func GetStatus() (string, error) {
	var sb strings.Builder

	wip, _ := listDir("docs/roadmaps/wip")
	blocked, _ := listDir("docs/roadmaps/blocked")
	done, _ := listDir("docs/roadmaps/done")

	sb.WriteString("── trackfw status ──────────────────────\n")

	sb.WriteString(fmt.Sprintf("\n🔄 WIP (%d)\n", len(wip)))
	for _, f := range wip {
		sb.WriteString(fmt.Sprintf("   %s\n", f))
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

	sb.WriteString("\n────────────────────────────────────────\n")
	return sb.String(), nil
}

func validateWIPHasREQ() ([]string, error) {
	entries, err := listDir("docs/roadmaps/wip")
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join("docs/roadmaps/wip", name))
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "REQ:") || strings.Contains(string(content), "REQ: \n") {
			violations = append(violations, fmt.Sprintf("roadmap %q is in wip but has no linked REQ", name))
		}
	}
	return violations, nil
}

func validateREQsHaveADR() ([]string, error) {
	entries, err := listDir("docs/req")
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join("docs/req", name))
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
	entries, err := listDir("docs/roadmaps/blocked")
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join("docs/roadmaps/blocked", name))
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
	entries, err := listDir("docs/req")
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join("docs/req", name))
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
	adrs, err := listDir("docs/adr")
	if err != nil {
		return nil, nil
	}
	reqs, err := os.ReadDir("docs/req")
	if err != nil {
		return nil, nil
	}

	var allREQContent strings.Builder
	for _, r := range reqs {
		if r.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join("docs/req", r.Name()))
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
	entries, err := listDir("docs/roadmaps/wip")
	if err != nil {
		return nil, nil
	}

	var violations []string
	for _, name := range entries {
		content, err := os.ReadFile(filepath.Join("docs/roadmaps/wip", name))
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
	return violations, nil
}

func validateStaleWIP() ([]string, error) {
	entries, err := filepath.Glob("docs/roadmaps/wip/*.md")
	if err != nil {
		return nil, err
	}
	var warnings []string
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
	return warnings, nil
}

func validateSingleWIP() ([]string, error) {
	entries, err := listDir("docs/roadmaps/wip")
	if err != nil {
		return nil, nil
	}
	if len(entries) > 1 {
		return []string{fmt.Sprintf("%d roadmaps in wip/ (recommended: keep only 1 active at a time)", len(entries))}, nil
	}
	return nil, nil
}

// blockedREQs retorna um mapa de REQ-basename → lista de ADR-basenames Draft que a bloqueiam.
// Somente REQs com Status: Open e ADRs com Status: Draft são incluídas.
func blockedREQs() (map[string][]string, error) {
	entries, err := listDir("docs/req")
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, name := range entries {
		reqPath := filepath.Join("docs", "req", name)
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
	entries, err := filepath.Glob(filepath.Join("docs", "req", "*.md"))
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
func adrIsDraft(adrBasename string) bool {
	path := filepath.Join("docs", "adr", adrBasename)
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "Status: Draft")
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
