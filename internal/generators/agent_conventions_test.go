package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTrackfwRulesBlock_NoConventions_NoSection confirms the "### Project Conventions" section
// is entirely absent — and the block is byte-identical to the pre-ML-1A output — when no
// agent_conventions text is passed (empty string or whitespace-only).
func TestTrackfwRulesBlock_NoConventions_NoSection(t *testing.T) {
	empty := trackfwRulesBlock("")
	if strings.Contains(empty, "### Project Conventions") {
		t.Errorf("trackfwRulesBlock(\"\") should not contain the Project Conventions section:\n%s", empty)
	}

	whitespace := trackfwRulesBlock("   \n  \n")
	if strings.Contains(whitespace, "### Project Conventions") {
		t.Errorf("trackfwRulesBlock(whitespace-only) should not contain the Project Conventions section:\n%s", whitespace)
	}

	if empty != whitespace {
		t.Errorf("trackfwRulesBlock(\"\") and trackfwRulesBlock(whitespace-only) must be byte-identical:\nempty=%q\nwhitespace=%q", empty, whitespace)
	}
}

// TestTrackfwRulesBlock_WithConventions_SectionPresent confirms the "### Project Conventions"
// section appears verbatim (team text preserved, trimmed) when agent_conventions is non-empty,
// and makes explicit in its own text that this is a team-declared convention, not automatic
// inference — per the REQ's acceptance criterion.
func TestTrackfwRulesBlock_WithConventions_SectionPresent(t *testing.T) {
	conventions := "Use pytest, not unittest.\nAPI REST, no GraphQL."
	block := trackfwRulesBlock(conventions)

	if !strings.Contains(block, "### Project Conventions") {
		t.Fatalf("trackfwRulesBlock(conventions) missing the Project Conventions section:\n%s", block)
	}
	if !strings.Contains(block, "Declared by the team") || !strings.Contains(block, "NOT") || !strings.Contains(block, "inferred automatically") {
		t.Errorf("Project Conventions section must explicitly state it is team-declared, not inferred:\n%s", block)
	}
	if !strings.Contains(block, conventions) {
		t.Errorf("Project Conventions section must preserve the team text verbatim:\n%s", block)
	}
}

// TestInjectOrUpdateRules_NoTrackfwYAML_NoRegression confirms that, for a project without
// trackfw.yaml (or without agent_conventions in it), the injected block is byte-identical to the
// pre-ML-1A behavior — the critical non-regression guarantee for every project already using
// trackfw today.
func TestInjectOrUpdateRules_NoTrackfwYAML_NoRegression(t *testing.T) {
	dir := t.TempDir()
	if err := InjectRulesForTool("claude", dir); err != nil {
		t.Fatalf("InjectRulesForTool failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile CLAUDE.md: %v", err)
	}

	if strings.Contains(string(content), "### Project Conventions") {
		t.Errorf("CLAUDE.md should not contain the Project Conventions section when trackfw.yaml is absent:\n%s", content)
	}

	want := "\n" + trackfwRulesBlock("") + "\n"
	if !strings.HasSuffix(string(content), want) {
		t.Errorf("CLAUDE.md rules block does not match the no-conventions baseline output.\nwant suffix:\n%s\ngot:\n%s", want, content)
	}
}

// TestInjectOrUpdateRules_WithAgentConventions_SectionInjected confirms the real, end-to-end
// injection mechanism (InjectRulesForTool) reads agent_conventions from trackfw.yaml in cwd and
// injects the "### Project Conventions" section into the generated agent file with the exact
// declared text.
func TestInjectOrUpdateRules_WithAgentConventions_SectionInjected(t *testing.T) {
	dir := t.TempDir()
	yaml := "agent_conventions: |\n  Use pytest, not unittest.\n  API REST, no GraphQL.\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}

	if err := InjectRulesForTool("claude", dir); err != nil {
		t.Fatalf("InjectRulesForTool failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile CLAUDE.md: %v", err)
	}
	got := string(content)

	if !strings.Contains(got, "### Project Conventions") {
		t.Fatalf("CLAUDE.md missing the Project Conventions section:\n%s", got)
	}
	if !strings.Contains(got, "Use pytest, not unittest.") || !strings.Contains(got, "API REST, no GraphQL.") {
		t.Errorf("CLAUDE.md does not contain the declared team conventions verbatim:\n%s", got)
	}
}

// TestInjectOrUpdateRules_AgentConventionsAbsentKey_NoRegression confirms a trackfw.yaml that
// exists but does not declare agent_conventions produces the same no-section output as having no
// trackfw.yaml at all.
func TestInjectOrUpdateRules_AgentConventionsAbsentKey_NoRegression(t *testing.T) {
	dir := t.TempDir()
	yaml := "hooks: husky\nci: github\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}

	if err := InjectRulesForTool("claude", dir); err != nil {
		t.Fatalf("InjectRulesForTool failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile CLAUDE.md: %v", err)
	}
	if strings.Contains(string(content), "### Project Conventions") {
		t.Errorf("CLAUDE.md should not contain the Project Conventions section when agent_conventions key is absent:\n%s", content)
	}
}
