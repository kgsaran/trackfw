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
