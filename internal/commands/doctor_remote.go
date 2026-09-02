package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// doctorRemoteDeps holds every dependency runDoctorRemote needs, injectable so the whole
// check runs deterministically offline in tests — see ADR-2026-09-02 and
// docs/roadmaps/wip/ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md
// (ML-3A). Mirrors releaseDeps' execGit/execForgeAPI/availFn seam (release.go) rather than
// inventing a new shape.
type doctorRemoteDeps struct {
	execGit      func(args ...string) (string, error)
	execForgeAPI func(name string, args []string, stdin string) (string, error)
	availFn      func(string) bool
	configForge  string
	repoDir      string
}

func defaultDoctorRemoteDeps(repoDir string) doctorRemoteDeps {
	return doctorRemoteDeps{
		execGit:      defaultGitExec,
		execForgeAPI: defaultExecForgeAPI,
		availFn:      nil,
		configForge:  config.Load().Forge,
		repoDir:      repoDir,
	}
}

// doctorHooksPathNeutralizedValues are the values that discard every git hook invocation on
// each OS git supports as a hooksPath target. Anything else (including an unset value) is left
// alone — a custom husky/lefthook directory is legitimate and must never be flagged (Wave 0's
// "does not break legitimate flow" constraint).
var doctorHooksPathNeutralizedValues = map[string]bool{
	"/dev/null": true,
	"NUL":       true,
}

// runDoctorRemote implements the --remote modality of `trackfw doctor` (ADR-2026-09-02): it
// never runs unless explicitly requested, and every branch either produces a genuine finding
// (evaluated, and wrong) or a DoctorNotEvaluated finding (could not evaluate) — never silence,
// which would read as "ok" to a report that already treats an empty finding list as a clean
// bill of health (printDoctorReport).
func runDoctorRemote(deps doctorRemoteDeps) []integrations.DoctorFinding {
	findings := []integrations.DoctorFinding{}

	// ── Local check: core.hooksPath neutralized (no network needed) ──────────────────────
	if hooksPath, err := deps.execGit("config", "--get", "core.hooksPath"); err == nil {
		if doctorHooksPathNeutralizedValues[strings.TrimSpace(hooksPath)] {
			findings = append(findings, integrations.DoctorFinding{
				FindingKind: integrations.DoctorHooksPathNeutralized,
				Destination: "git:core.hooksPath",
				Remedy:      fmt.Sprintf("git config --unset core.hooksPath   # currently %q discards every hook invocation; unset to restore .git/hooks, or point it at your real hooks directory", strings.TrimSpace(hooksPath)),
			})
		}
	}

	// ── Forge resolution: only GitHub is evaluated; every other forge is not applicable ──
	remoteURL, _ := deps.execGit("remote", "get-url", "origin")
	remoteURL = strings.TrimSpace(remoteURL)
	resolution, resErr := forge.Resolve(forge.Input{
		ConfigForge: deps.configForge,
		RemoteURL:   remoteURL,
		RepoDir:     deps.repoDir,
	})
	if resErr != nil || resolution.Forge != "github" {
		forgeName := resolution.Forge
		if resErr != nil {
			forgeName = "unknown"
		}
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: "branch-protection",
			Remedy:      fmt.Sprintf("not applicable: branch protection is checked only on GitHub, and this repository's forge resolved to %q. Not a failure — no action needed unless this repository is actually hosted on GitHub, in which case set forge: github in trackfw.yaml or add a github.com origin remote.", forgeName),
		})
		return findings
	}

	// ── gh CLI availability ───────────────────────────────────────────────────────────────
	adapter := forge.NewAdapter("github", deps.availFn)
	if !adapter.Available {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: "branch-protection",
			Remedy:      "install the GitHub CLI (gh) to evaluate branch protection remotely: https://cli.github.com, then retry with --remote",
		})
		return findings
	}

	// ── Credential presence: distinct from credential SCOPE below (ADR-2026-09-02) ────────
	if _, err := deps.execForgeAPI("gh", []string{"auth", "status"}, ""); err != nil {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: "branch-protection",
			Remedy:      "GitHub CLI has no credential — authenticate first: gh auth login (or set GITHUB_TOKEN/GH_TOKEN), then retry with --remote",
		})
		return findings
	}

	// ── Repository info: default branch + whether this credential has admin access ────────
	repoInfoResp, err := deps.execForgeAPI("gh", []string{"api", "repos/{owner}/{repo}"}, "")
	if err != nil {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: "branch-protection",
			Remedy:      fmt.Sprintf("could not reach the GitHub API to resolve this repository: %s. Check network connectivity and retry with --remote", err.Error()),
		})
		return findings
	}
	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
		Permissions   struct {
			Admin bool `json:"admin"`
		} `json:"permissions"`
	}
	if jerr := json.Unmarshal([]byte(repoInfoResp), &repoInfo); jerr != nil || repoInfo.DefaultBranch == "" {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: "branch-protection",
			Remedy:      fmt.Sprintf("could not parse the repository response from the GitHub API: %s. Retry with --remote", repoInfoResp),
		})
		return findings
	}

	// Credential SCOPE: reading branch protection requires admin access to the repository.
	// This is a DISTINCT remedy from "no credential" above — one is fixed by authenticating,
	// this one by being granted admin access (or using a token for a repo you administer).
	if !repoInfo.Permissions.Admin {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: "branch-protection",
			Remedy:      "the authenticated GitHub credential lacks admin access to this repository — reading branch protection requires it. Ask a repository admin to grant access, or authenticate as an account that has it, then retry with --remote",
		})
		return findings
	}

	// ── Branch protection itself ────────────────────────────────────────────────────────────
	protectionResp, protErr := deps.execForgeAPI("gh", []string{"api", "repos/{owner}/{repo}/branches/" + repoInfo.DefaultBranch + "/protection"}, "")
	if protErr != nil {
		if strings.Contains(protErr.Error(), "(HTTP 404)") {
			// Evaluated (admin confirmed above): the branch genuinely has no protection at
			// all, which means both checks fail — GitHub does not return the two settings
			// separately when there is no rule to read them from.
			findings = append(findings, integrations.DoctorFinding{
				FindingKind: integrations.DoctorRequiredStatusChecksMissing,
				Destination: fmt.Sprintf("branch-protection:%s:required_status_checks", repoInfo.DefaultBranch),
				Remedy:      fmt.Sprintf("configure required status checks: GitHub repo Settings > Branches > Branch protection rules > %s > Require status checks to pass before merging", repoInfo.DefaultBranch),
			})
			findings = append(findings, integrations.DoctorFinding{
				FindingKind: integrations.DoctorEnforceAdminsDisabled,
				Destination: fmt.Sprintf("branch-protection:%s:enforce_admins", repoInfo.DefaultBranch),
				Remedy:      fmt.Sprintf("gh api repos/{owner}/{repo}/branches/%s/protection/enforce_admins --method POST", repoInfo.DefaultBranch),
			})
			return findings
		}
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: fmt.Sprintf("branch-protection:%s", repoInfo.DefaultBranch),
			Remedy:      fmt.Sprintf("could not read branch protection from the GitHub API: %s. This may be transient (rate limit, network) — retry with --remote", protErr.Error()),
		})
		return findings
	}

	var protection struct {
		RequiredStatusChecks *struct {
			Contexts []string `json:"contexts"`
			Checks   []struct {
				Context string `json:"context"`
			} `json:"checks"`
		} `json:"required_status_checks"`
		EnforceAdmins *struct {
			Enabled bool `json:"enabled"`
		} `json:"enforce_admins"`
	}
	if jerr := json.Unmarshal([]byte(protectionResp), &protection); jerr != nil {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorNotEvaluated,
			Destination: fmt.Sprintf("branch-protection:%s", repoInfo.DefaultBranch),
			Remedy:      fmt.Sprintf("could not parse the branch protection response from the GitHub API: %s. Retry with --remote", protectionResp),
		})
		return findings
	}

	if protection.RequiredStatusChecks == nil ||
		(len(protection.RequiredStatusChecks.Contexts) == 0 && len(protection.RequiredStatusChecks.Checks) == 0) {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorRequiredStatusChecksMissing,
			Destination: fmt.Sprintf("branch-protection:%s:required_status_checks", repoInfo.DefaultBranch),
			Remedy:      fmt.Sprintf("configure required status checks: GitHub repo Settings > Branches > Branch protection rules > %s > Require status checks to pass before merging", repoInfo.DefaultBranch),
		})
	}
	if protection.EnforceAdmins == nil || !protection.EnforceAdmins.Enabled {
		findings = append(findings, integrations.DoctorFinding{
			FindingKind: integrations.DoctorEnforceAdminsDisabled,
			Destination: fmt.Sprintf("branch-protection:%s:enforce_admins", repoInfo.DefaultBranch),
			Remedy:      fmt.Sprintf("gh api repos/{owner}/{repo}/branches/%s/protection/enforce_admins --method POST", repoInfo.DefaultBranch),
		})
	}

	return findings
}
