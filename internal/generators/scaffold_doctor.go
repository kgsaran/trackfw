package generators

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/version"
)

// pythonValidateScriptForm is the byte-exact content Python's `trackfw init` and
// `trackfw update` (validate-script target) write to scripts/trackfw-validate.sh.
// It is accepted by the set-membership check in checkValidateScriptArtifact so that
// a project initialized by the Python runtime does not produce a false-positive
// scaffold-divergent finding in the Go doctor.
//
// Why a named constant here (not in scaffold.go): this string is only consumed by
// the doctor's membership check, not by any generator; co-locating it with the
// check avoids confusion about which form Go's generator emits.
//
// Scope of the exception: ONLY scripts/trackfw-validate.sh uses set-membership.
// All other scaffold artifacts are compared against the single template the local
// runtime would generate (exact bytes). See docs/cli-parity.md,
// "validate.sh — pertencimento a conjunto (set-membership, escopado)".
const pythonValidateScriptForm = "#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n"

// RunScaffoldDoctor compares scaffold artifacts on disk against the templates the
// currently installed binary would generate (given the project's own trackfw.yaml),
// and returns findings for any artifact that is divergent or missing.
//
// Design decisions (ADR-2026-08-27, REQ-2026-08-27):
//
//   - Property by path, not by manifest (AC3): scaffold artifacts are identified by
//     well-known namespace paths (.claude/commands/trackfw/, scripts/trackfw-*.sh,
//     .github/workflows/trackfw-gate.yml). No manifest entry is written or read.
//
//   - Sibling classifier (AC15): scaffold artifacts are never in the manifest, so
//     routing them through ClassifyDoctor would produce a meaningless Claim and a wrong
//     remedy. The finding kinds DoctorScaffoldDivergent / DoctorScaffoldMissing carry a
//     zero Claim and a trackfw-update remedy, distinct from the catalog-based kinds.
//
//   - Config-rendered templates (AC12): scripts/trackfw-validate.sh content varies with
//     cfg.Backend/cfg.Frontend — the template is rendered from the project's own
//     trackfw.yaml, not from a hardcoded default. Any project with backend: go would be a
//     false positive otherwise.
//
//   - validate.sh set-membership (architect decision 2026-08-27): scripts/trackfw-validate.sh
//     is accepted when it matches ANY known runtime's template (Go/Node form rendered from
//     the project's cfg, OR Python's fixed form). This is the only artifact with this
//     exception — the byte-divergence between runtimes is pre-existing, intentional, and
//     documented. A file that matches NO known form is still accused. See
//     checkValidateScriptArtifact and docs/cli-parity.md.
//
//   - Eligibility for missing (AC14): slash commands are only checked when
//     .claude/commands/trackfw/ already exists. Its absence signals a project initialized
//     via `trackfw discover --init` (which legitimately omits slash commands) — reporting
//     9 missing files would be false positives for that initializer.
//
//   - Conditional artifacts (AC13): CI workflow is only checked when cfg.CI declares it.
//     Absence of an unconfigured artifact is never a finding.
//
//   - Neutral blame message (AC16): no scaffold artifact carries a version stamp, so the
//     binary cannot determine whether it or the project is stale. The remedy and message
//     name the installed binary version but instruct the user to verify the direction.
//
//   - Guards are included (additive coverage): credential-guard and git-branch-guard are
//     covered here in addition to the two `validate` rules that already check them.
//     Neither service is exclusively owned by the other surface — this is complementary.
//
//   - Hook files (husky/lefthook) are excluded per the declared residual in
//     docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md §Residual-3.
//
// AC15 note: the blocker cited in the roadmap ("ClassifyDoctor has no case for
// !Registered && StateModified") is false for all three CLIs. Go received the fix in
// ML-2C (see doctor.go line 118-124); Node.js and Python have equivalent branches. This
// function provides scaffold coverage via a separate path — not by adding a case to
// ClassifyDoctor, which would produce wrong remedies and a meaningless Claim.
func RunScaffoldDoctor(projectRoot string) ([]integrations.DoctorFinding, error) {
	// Eligibility: trackfw.yaml must exist. Without it there is no evidence this is a
	// trackfw project, so return empty findings rather than flooding a non-trackfw repo.
	if _, err := os.Stat(filepath.Join(projectRoot, "trackfw.yaml")); err != nil {
		return []integrations.DoctorFinding{}, nil
	}

	// Load config from the project's trackfw.yaml to render cfg-dependent templates
	// (AC12). config.Load() reads relative to cwd — chdir like Update() does.
	orig, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("scaffold doctor: getwd: %w", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		return nil, fmt.Errorf("scaffold doctor: chdir %s: %w", projectRoot, err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	cfg := loadUpdateConfig()

	findings := []integrations.DoctorFinding{}

	// --- Scripts (always in scope when trackfw.yaml is present) ---
	//
	// All five scripts are written by trackfw init and trackfw update unconditionally.
	// trackfw discover --init also writes them (InstallGates), so their presence is
	// expected in any trackfw-managed project.
	//
	// scripts/trackfw-validate.sh is handled separately via checkValidateScriptArtifact
	// (set-membership against all known runtime forms). The four remaining scripts use
	// single-template equality via checkScaffoldArtifact.
	if f := checkValidateScriptArtifact(filepath.Join(projectRoot, "scripts/trackfw-validate.sh"), "scripts/trackfw-validate.sh", cfg); f != nil {
		findings = append(findings, *f)
	}
	staticScripts := []struct {
		relPath string
		content []byte
	}{
		{"scripts/trackfw-attention-signal.sh", []byte(attentionSignalScript)},
		{"scripts/trackfw-attention-cleanup.sh", []byte(attentionCleanupScript)},
		{"scripts/trackfw-credential-guard.sh", []byte(credentialGuardScript)},
		{"scripts/trackfw-git-branch-guard.sh", []byte(gitBranchGuardScript)},
	}
	for _, s := range staticScripts {
		path := filepath.Join(projectRoot, s.relPath)
		f := checkScaffoldArtifact(path, s.relPath, s.content, true)
		if f != nil {
			findings = append(findings, *f)
		}
	}

	// --- Slash commands (AC14: only when the directory already exists) ---
	//
	// The directory's presence is the eligibility signal: a project initialized via
	// `trackfw discover --init` (which does NOT write slash commands) will not have this
	// directory, so we report no missing commands. A project initialized via `trackfw
	// init` or `trackfw update` will have it, and any absent file inside is a finding.
	claudeDir := filepath.Join(projectRoot, ClaudeCommandsDirPath)
	if _, err := os.Stat(claudeDir); err == nil {
		for filename, content := range claudeCommandsContent() {
			relPath := ClaudeCommandsDirPath + "/" + filename
			path := filepath.Join(projectRoot, relPath)
			f := checkScaffoldArtifact(path, relPath, []byte(content), true)
			if f != nil {
				findings = append(findings, *f)
			}
		}
	}

	// --- CI workflow (AC13: conditional on ci: in trackfw.yaml) ---
	switch cfg.CI {
	case "github-actions":
		relPath := GitHubActionsWorkflowPath
		path := filepath.Join(projectRoot, relPath)
		f := checkScaffoldArtifact(path, relPath, []byte(buildGitHubActionsWorkflowContent(cfg)), true)
		if f != nil {
			findings = append(findings, *f)
		}
	case "gitlab-ci":
		relPath := GitLabCIWorkflowPath
		path := filepath.Join(projectRoot, relPath)
		f := checkScaffoldArtifact(path, relPath, []byte(buildGitLabCIWorkflowContent(cfg)), true)
		if f != nil {
			findings = append(findings, *f)
		}
	}

	// Deterministic output (AC7): sort by destination so the three CLIs are byte-identical
	// when diffed (scripts/check-doctor-parity.sh extension required — ML-2A).
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].Destination < findings[j].Destination
	})

	return findings, nil
}

// checkValidateScriptArtifact checks scripts/trackfw-validate.sh using set-membership:
// the file is accepted if its content matches ANY known runtime's template —
// either the Go/Node form (cfg-rendered from the project's trackfw.yaml) or
// Python's fixed form (pythonValidateScriptForm). All other scaffold artifacts use
// single-template equality via checkScaffoldArtifact.
//
// Why set-membership here: the three runtimes intentionally emit different bytes for
// this file (Go/Node: shebang #!/usr/bin/env sh + cfg-dependent build steps; Python:
// simple #!/usr/bin/env bash form). The divergence is pre-existing and documented.
// A file that matches NONE of the known forms is still accused (AC3 preserved).
func checkValidateScriptArtifact(path, relPath string, cfg Config) *integrations.DoctorFinding {
	actual, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldMissing,
			Destination: relPath,
			Remedy:      scaffoldRemedy("restore", relPath),
		}
		return &f
	}
	if err != nil {
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldDivergent,
			Destination: relPath,
			Remedy:      scaffoldRemedy("resync", relPath),
		}
		return &f
	}
	goNodeForm := []byte(buildValidateScript(cfg))
	pythonForm := []byte(pythonValidateScriptForm)
	if bytes.Equal(actual, goNodeForm) || bytes.Equal(actual, pythonForm) {
		return nil
	}
	f := integrations.DoctorFinding{
		FindingKind: integrations.DoctorScaffoldDivergent,
		Destination: relPath,
		Remedy:      scaffoldRemedy("resync", relPath),
	}
	return &f
}

// checkScaffoldArtifact compares the on-disk content at path against expected.
// Returns a finding if the file is divergent or (when reportMissing=true) absent.
// relPath is used as Destination in the finding (relative to project root, human-readable).
func checkScaffoldArtifact(path, relPath string, expected []byte, reportMissing bool) *integrations.DoctorFinding {
	actual, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if !reportMissing {
			return nil
		}
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldMissing,
			Destination: relPath,
			Remedy:      scaffoldRemedy("restore", relPath),
		}
		return &f
	}
	if err != nil {
		// Unreadable artifact: treat as divergent so the user is informed.
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldDivergent,
			Destination: relPath,
			Remedy:      scaffoldRemedy("resync", relPath),
		}
		return &f
	}
	if bytes.Equal(actual, expected) {
		return nil
	}
	f := integrations.DoctorFinding{
		FindingKind: integrations.DoctorScaffoldDivergent,
		Destination: relPath,
		Remedy:      scaffoldRemedy("resync", relPath),
	}
	return &f
}

// scaffoldRemedy returns a ready-to-copy remedy command for a scaffold finding.
// The message is neutral about blame direction (AC16): the binary version is stated,
// but the user is told to check whether the binary or the project needs updating.
func scaffoldRemedy(action, relPath string) string {
	return fmt.Sprintf(
		"trackfw update   # %s %s: content differs from the template trackfw v%s generates; if this project was initialized with a newer binary, update the binary instead",
		action, relPath, version.Version,
	)
}
