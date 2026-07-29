package integrations

import (
	"regexp"
	"strings"
	"testing"
)

// TestArchitectIsSoleGitAuthority verifies that trackfw_architect documents
// exclusive Git authority (branch, commit, push) and the barrier protocol,
// while every other agent explicitly disclaims Git operations and defers to
// a self-contained handoff from trackfw_architect.
func TestArchitectIsSoleGitAuthority(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error: %v", err)
	}

	agents := catalog.Items(KindAgents)
	if len(agents) == 0 {
		t.Fatal("catalog reports zero agents — cannot validate Git authority contract")
	}

	gitVerbs := regexp.MustCompile(`(?i)git (add|commit|push|checkout|branch|merge|rebase|stash|reset)`)

	sawArchitect := false
	specialistsChecked := 0

	for _, item := range agents {
		data, err := catalog.ReadAsset(item)
		if err != nil {
			t.Fatalf("ReadAsset(%s) error: %v", item.ID, err)
		}
		content := string(data)

		if item.ID == "architect" {
			sawArchitect = true

			if !strings.Contains(content, "trackfw_architect") {
				t.Errorf("architect.md must reference the public role name trackfw_architect")
			}
			if !strings.Contains(content, "## Git authority") {
				t.Errorf("architect.md must document a Git authority section")
			}
			if !strings.Contains(content, "## Barrier protocol") {
				t.Errorf("architect.md must document a Barrier protocol section")
			}
			if !strings.Contains(content, "code-quality") || !strings.Contains(content, "security") {
				t.Errorf("architect.md barrier protocol must reference invoking code-quality and security")
			}
			if !strings.Contains(strings.ToLower(content), "corrective") {
				t.Errorf("architect.md barrier protocol must describe dispatching a corrective microbatch on failure")
			}
			continue
		}

		specialistsChecked++

		if gitVerbs.MatchString(content) {
			t.Errorf("%s.md must not authorize any Git operation; found a forbidden git verb", item.ID)
		}
		if !strings.Contains(content, "trackfw_architect") {
			t.Errorf("%s.md must reference trackfw_architect as the sole Git authority", item.ID)
		}
		if !strings.Contains(content, "never executes Git operations") {
			t.Errorf("%s.md must explicitly disclaim executing Git operations", item.ID)
		}
		if !strings.Contains(content, "handoff") {
			t.Errorf("%s.md must require acting only on a self-contained handoff from trackfw_architect", item.ID)
		}
	}

	if !sawArchitect {
		t.Fatal("catalog does not include an 'architect' agent item")
	}
	if specialistsChecked != 11 {
		t.Errorf("expected 11 specialist agents besides architect, found %d", specialistsChecked)
	}
}
