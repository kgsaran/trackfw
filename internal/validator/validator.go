package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Validate() ([]string, error) {
	var violations []string

	wipViolations, err := validateWIPHasREQ()
	if err != nil {
		return nil, err
	}
	violations = append(violations, wipViolations...)

	reqViolations, err := validateREQsHaveADR()
	if err != nil {
		return nil, err
	}
	violations = append(violations, reqViolations...)

	blockedViolations, err := validateBlockedHasREQ()
	if err != nil {
		return nil, err
	}
	violations = append(violations, blockedViolations...)

	reqRoadmapViolations, err := validateREQsHaveRoadmap()
	if err != nil {
		return nil, err
	}
	violations = append(violations, reqRoadmapViolations...)

	adrOrphanViolations, err := validateADRsAreReferenced()
	if err != nil {
		return nil, err
	}
	violations = append(violations, adrOrphanViolations...)

	return violations, nil
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
