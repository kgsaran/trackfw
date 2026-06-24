package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var codexSkills = map[string]string{
	"trackfw-governance": `---
name: trackfw-governance
description: Inspect or maintain ADR, REQ, ROADMAP, traceability, lifecycle state, and trackfw validation. Use for governance questions and artifact changes; do not use for implementation-only work.
---

1. Run ` + "`trackfw context --format=json`" + ` and read ` + "`docs/agents-working-context.md`" + `.
2. Treat files as state: ADR → REQ → ROADMAP → backlog/analyzing/wip/blocked/done/abandoned.
3. Preserve trace IDs, links, status-folder coherence, and acceptance criteria.
4. Run ` + "`trackfw validate --json`" + ` before reporting completion.
`,
	"trackfw-plan": `---
name: trackfw-plan
description: Plan a trackfw-governed implementation before code changes. Use when creating or refining REQs, ADRs, roadmaps, waves, microbatches, acceptance criteria, and validation commands.
---

1. Inspect existing ADRs, REQs, roadmaps, code, and tests before deciding.
2. Create or update the REQ and required ADRs before the roadmap.
3. Produce decision-complete microbatches with exact outcomes and validation commands.
4. Keep independent microbatches parallel and dependencies explicit.
5. Leave the roadmap in ` + "`analyzing`" + ` until implementation starts.
`,
	"trackfw-implement": `---
name: trackfw-implement
description: Implement work governed by an existing trackfw roadmap. Use for code changes, bug fixes, and features that must follow REQ/ROADMAP lifecycle and validation gates.
---

1. Run ` + "`trackfw context --format=json`" + ` and identify the matching REQ and roadmap.
2. Move the matching roadmap to ` + "`wip`" + ` before editing code.
3. Work one microbatch at a time and update its status in the roadmap.
4. Run project tests plus ` + "`trackfw validate --json`" + `.
5. Do not move to ` + "`done`" + ` until acceptance criteria are satisfied.
`,
	"trackfw-review": `---
name: trackfw-review
description: Review a change for governance traceability, correctness, security, regressions, missing tests, and roadmap acceptance. Use for PR or branch review; stay read-only unless fixes are explicitly requested.
---

1. Compare the branch with its base and identify the linked REQ and roadmap.
2. Prioritize behavioral bugs, security, contract drift, missing tests, and governance bypasses.
3. Verify changed behavior is covered by acceptance criteria and tests.
4. Run ` + "`trackfw validate --json`" + ` and report findings by severity with file references.
`,
	"trackfw-release": `---
name: trackfw-release
description: Prepare and verify a trackfw release. Use for release readiness, versioning, changelog, packaging, parity, and publication gates; do not publish without explicit authorization.
---

1. Confirm all roadmaps included in the release are complete and traceable.
2. Run the repository quality, parity, package, and governance gates.
3. Derive SemVer from commits since the latest tag and draft the changelog.
4. Stop before tag creation or publication unless the user explicitly authorizes it.
`,
}

var codexAgents = map[string]string{
	"trackfw-architect.toml": `name = "trackfw_architect"
description = "Architecture and implementation-planning specialist for governed changes."
sandbox_mode = "read-only"
developer_instructions = """
Map the existing architecture, contracts, constraints, and traceability chain.
Identify material decisions and require ADRs for unresolved architectural choices.
Produce decision-complete plans. Do not edit files unless the parent explicitly changes your assignment.
"""
`,
	"trackfw-backend.toml": `name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope. Preserve public contracts and trackfw traceability.
Run focused tests and report changed files, validation evidence, and remaining risks.
"""
`,
	"trackfw-frontend.toml": `name = "trackfw_frontend"
description = "Frontend implementation specialist focused on UX consistency, accessibility, i18n, and browser behavior."
developer_instructions = """
Implement only the assigned frontend scope. Preserve the design system, accessibility, and localization.
Run focused build and UI tests and report changed files and validation evidence.
"""
`,
	"trackfw-qa.toml": `name = "trackfw_qa"
description = "Read-heavy QA specialist for test strategy, regressions, flaky tests, and acceptance coverage."
sandbox_mode = "read-only"
developer_instructions = """
Trace critical flows and compare implementation against roadmap acceptance criteria.
Prioritize reproducible failures and contract gaps. Do not modify files unless explicitly assigned a fix.
"""
`,
	"trackfw-security.toml": `name = "trackfw_security"
description = "Read-only security reviewer for trust boundaries, secrets, injection, permissions, dependencies, and unsafe defaults."
sandbox_mode = "read-only"
developer_instructions = """
Perform evidence-backed threat analysis on the assigned change.
Report concrete exploit paths and mitigations by severity. Avoid speculative findings.
"""
`,
	"trackfw-reviewer.toml": `name = "trackfw_reviewer"
description = "Owner-level reviewer for correctness, maintainability, governance, and missing tests."
sandbox_mode = "read-only"
developer_instructions = """
Review the branch as an owner. Prioritize bugs, regressions, security, contract drift, and missing tests.
Verify REQ and roadmap linkage. Return findings first, ordered by severity, with file references.
"""
`,
}

func InstallCodex(cwd string) error {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if err := InjectRulesForTool("codex", cwd); err != nil {
		return err
	}
	if err := installCodexConfig(cwd); err != nil {
		return err
	}
	for name, content := range codexSkills {
		path := filepath.Join(cwd, ".agents", "skills", name, "SKILL.md")
		if err := writeManagedFile(path, content); err != nil {
			return err
		}
	}
	for name, content := range codexAgents {
		path := filepath.Join(cwd, ".codex", "agents", name)
		if err := writeManagedFile(path, content); err != nil {
			return err
		}
	}
	if err := injectCodexHooks(cwd); err != nil {
		return err
	}
	fmt.Println("  ✓ Codex: AGENTS.md, skills, custom agents and hooks")
	return nil
}

func installCodexConfig(cwd string) error {
	path := filepath.Join(cwd, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	if strings.Contains(content, "[agents]") {
		return nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n[agents]\nmax_threads = 6\nmax_depth = 1\n"
	return os.WriteFile(path, []byte(content), 0644)
}

func writeManagedFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0644)
}
