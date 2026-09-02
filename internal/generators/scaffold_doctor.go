package generators

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/version"
)

// currentGOOS is set to runtime.GOOS at package init time.
// Tests that need to exercise the Windows platform guard override this variable:
//
//	generators.CurrentGOOS = "windows"
//	defer func() { generators.CurrentGOOS = runtime.GOOS }()
//
// It is exported (capital C) so test files in other packages can reach it;
// production code only reads it through checkMode below.
var CurrentGOOS = runtime.GOOS

// DiscoverGitHubActionsWorkflowPath is the canonical relative path of the second,
// independent CI workflow trackfw writes: the one `trackfw discover --init` (and its
// Node/Python equivalents) generates via InstallGates, distinct from
// GitHubActionsWorkflowPath (trackfw-gate.yml, written by init/update). Both files can
// coexist in the same project — ADR-2026-08-28 names this exact case as the motivation
// for pinning both install mechanisms, not just the install.sh one.
const DiscoverGitHubActionsWorkflowPath = ".github/workflows/trackfw-validate.yml"

// BuildDiscoverGitHubActionsWorkflowContent returns the template content trackfw writes
// to DiscoverGitHubActionsWorkflowPath. It lives in the generators package (not in
// internal/discover) so that internal/discover can import it for writing — package
// internal/discover already imports internal/generators, and internal/generators must
// never import internal/discover (that would be circular).
//
// NOT version-independent (ADR-2026-08-28, REQ-2026-08-28 AC6/AC7): the `go install
// .../cmd/trackfw@vX.Y.Z` step pins the second install mechanism (`go install ...@latest`)
// to internal/version.Version, the version of the binary that generated/updated the
// project — mirroring the install.sh pin already applied to buildGitHubActionsWorkflowContent
// (trackfw-gate.yml) in scaffold.go. Scaffold doctor calls this to compare disk content
// against the current template (AC10/AC11).
//
// Job id is `governance-go-install` (ML-1A, ROADMAP-2026-09-01) — was `governance`,
// colliding with the job id in buildGitHubActionsWorkflowContent (trackfw-gate.yml,
// scaffold.go), which produced two identically-named check-runs on any PR of a project
// with both workflows installed. See the doc comment there for the full rationale; the
// two ids are named after the install mechanism (`go install` here vs install.sh
// there), not the workflow file, since that's what the reader of
// required_status_checks needs to know.
func BuildDiscoverGitHubActionsWorkflowContent() string {
	return `name: trackfw validate
on: [push, pull_request]
jobs:
  governance-go-install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v` + version.Version + `
      - run: trackfw validate
`
}

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

// execBitPresent reports whether the file at path has the owner-execute bit set
// (stat.Mode & 0o100 != 0). Returns false on any stat error.
//
// The check uses the bit mask rather than equality to 0755 so that umask-narrowed
// modes like 0750 or 0700 are also accepted — AC10 of REQ-2026-08-28.
func execBitPresent(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0o100 != 0
}

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
//   - Execute-bit checking (REQ-2026-08-28, AC2–AC5, AC10, AC11):
//     The five scripts that the generator writes with mode 0755 are additionally checked
//     for the owner-execute bit. The check uses mode & 0o100 != 0 (not == 0755) so that
//     umask-narrowed modes (0750, 0700) are accepted. Non-executable artifacts (slash
//     commands, CI workflows) carry execBit=false and are never mode-checked (AC11/AC4).
//     Content divergence takes precedence: if content is wrong, scaffold-divergent is
//     emitted regardless of mode (at most one finding per artifact). Content correct +
//     bit missing → scaffold-wrong-mode (AC3 distinct state). On Windows (CurrentGOOS ==
//     "windows") the execute bit is not representable on NTFS, so the mode check is
//     suppressed entirely — AC5. Python's os.chmod is unconditional and already produces
//     0755 even on existing files; Go and Node added os.Chmod/fs.chmodSync after the
//     WriteFile/writeFileSync call to match Python's behavior on existing files (AC9).
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
	//
	// All five scripts have execBit=true: the generator writes them with mode 0755 and
	// they must remain executable. Non-executable artifacts (slash commands, CI workflows)
	// pass execBit=false and are never accused of a missing execute bit (AC4/AC11).
	if f := checkValidateScriptArtifact(filepath.Join(projectRoot, "scripts/trackfw-validate.sh"), "scripts/trackfw-validate.sh", cfg); f != nil {
		findings = append(findings, *f)
	}
	staticScripts := []struct {
		relPath string
		content []byte
		execBit bool
	}{
		{"scripts/trackfw-attention-signal.sh", []byte(attentionSignalScript), true},
		{"scripts/trackfw-attention-cleanup.sh", []byte(attentionCleanupScript), true},
		{"scripts/trackfw-credential-guard.sh", []byte(credentialGuardScript), true},
		{"scripts/trackfw-git-branch-guard.sh", []byte(gitBranchGuardScript), true},
	}
	for _, s := range staticScripts {
		path := filepath.Join(projectRoot, s.relPath)
		f := checkScaffoldArtifact(path, s.relPath, s.content, true, s.execBit)
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
	//
	// Slash commands are markdown files written with mode 0644 — execBit=false (AC11).
	claudeDir := filepath.Join(projectRoot, ClaudeCommandsDirPath)
	if _, err := os.Stat(claudeDir); err == nil {
		for filename, content := range claudeCommandsContent() {
			relPath := ClaudeCommandsDirPath + "/" + filename
			path := filepath.Join(projectRoot, relPath)
			f := checkScaffoldArtifact(path, relPath, []byte(content), true, false)
			if f != nil {
				findings = append(findings, *f)
			}
		}
	}

	// --- CI workflow (AC13: conditional on ci: in trackfw.yaml) ---
	//
	// CI workflow YAML files are written with mode 0644 — execBit=false (AC11).
	switch cfg.CI {
	case "github-actions":
		relPath := GitHubActionsWorkflowPath
		path := filepath.Join(projectRoot, relPath)
		f := checkScaffoldArtifact(path, relPath, []byte(buildGitHubActionsWorkflowContent(cfg)), true, false)
		if f != nil {
			findings = append(findings, *f)
		}
	case "gitlab-ci":
		relPath := GitLabCIWorkflowPath
		path := filepath.Join(projectRoot, relPath)
		f := checkScaffoldArtifact(path, relPath, []byte(buildGitLabCIWorkflowContent(cfg)), true, false)
		if f != nil {
			findings = append(findings, *f)
		}
	}

	// --- Discover CI workflow (second, independent install mechanism) ---
	//
	// trackfw-validate.yml (written by `trackfw discover --init`, InstallGates) is a
	// separate artifact from trackfw-gate.yml above — both can coexist in the same
	// project (ADR-2026-08-28). Only checked when the file is already present, mirroring
	// the "conditional artifact" treatment of the trackfw-gate.yml case above but using
	// presence-on-disk instead of cfg.CI, because InstallGates decides on its own
	// DiscoveryResult.CISystem signal (github-actions detection), not on trackfw.yaml's
	// `ci:` key — a project can have discover's workflow without cfg.CI ever being set.
	discoverWorkflowPath := filepath.Join(projectRoot, DiscoverGitHubActionsWorkflowPath)
	if _, err := os.Stat(discoverWorkflowPath); err == nil {
		f := checkScaffoldArtifact(discoverWorkflowPath, DiscoverGitHubActionsWorkflowPath, []byte(BuildDiscoverGitHubActionsWorkflowContent()), true, false)
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
//
// Execute-bit check (REQ-2026-08-28 AC2/AC3/AC10): after content membership passes,
// the execute bit is checked (unless CurrentGOOS == "windows" — AC5). Content
// divergence takes precedence over mode: a content-divergent file always produces
// scaffold-divergent, never scaffold-wrong-mode (at most one finding per artifact).
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
	if !bytes.Equal(actual, goNodeForm) && !bytes.Equal(actual, pythonForm) {
		// Content diverges — scaffold-divergent takes precedence over any mode issue.
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldDivergent,
			Destination: relPath,
			Remedy:      scaffoldRemedy("resync", relPath),
		}
		return &f
	}
	// Content is accepted. Now check the execute bit (AC2/AC3).
	// Suppressed on Windows where the bit is not representable (AC5).
	if CurrentGOOS != "windows" && !execBitPresent(path) {
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldWrongMode,
			Destination: relPath,
			Remedy:      scaffoldWrongModeRemedy(relPath),
		}
		return &f
	}
	return nil
}

// checkScaffoldArtifact compares the on-disk content at path against expected.
// Returns a finding if the file is divergent or (when reportMissing=true) absent.
// relPath is used as Destination in the finding (relative to project root, human-readable).
//
// execBit controls whether the owner-execute bit is checked after content passes.
//   - true: the generator writes this artifact with mode 0755; bit absence produces
//     DoctorScaffoldWrongMode (AC2/AC3). Content divergence always takes precedence.
//     Suppressed when CurrentGOOS == "windows" (AC5).
//   - false: artifact is expected 0644 (slash commands, CI workflows); never accused
//     of a missing execute bit (AC4/AC11).
func checkScaffoldArtifact(path, relPath string, expected []byte, reportMissing bool, execBit bool) *integrations.DoctorFinding {
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
	if !bytes.Equal(actual, expected) {
		// Content diverges — takes precedence over any mode issue (at most one finding
		// per artifact; update fixes both content and mode anyway).
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldDivergent,
			Destination: relPath,
			Remedy:      scaffoldRemedy("resync", relPath),
		}
		return &f
	}
	// Content matches. Check the execute bit when required (AC2/AC3).
	// Suppressed on Windows where the bit is not representable (AC5).
	if execBit && CurrentGOOS != "windows" && !execBitPresent(path) {
		f := integrations.DoctorFinding{
			FindingKind: integrations.DoctorScaffoldWrongMode,
			Destination: relPath,
			Remedy:      scaffoldWrongModeRemedy(relPath),
		}
		return &f
	}
	return nil
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

// scaffoldWrongModeRemedy returns a remedy command for the scaffold-wrong-mode finding.
// The message explicitly names the missing execute bit to distinguish it from content
// divergence (AC3 of REQ-2026-08-28). The message is runtime-neutral (no Go/Node/Python
// function names) so the parity check can diff Go, Node, and Python outputs byte-for-byte
// on this kind — same constraint as scaffoldRemedy.
func scaffoldWrongModeRemedy(relPath string) string {
	return fmt.Sprintf(
		"trackfw update   # restore execute bit on %s: content is correct but the owner-execute bit is missing (mode 0755 required); trackfw update now restores the mode unconditionally on existing files",
		relPath,
	)
}
