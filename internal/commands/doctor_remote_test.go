package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/integrations"
)

// fakeDoctorRemoteExec records every gh invocation (args joined by " ") so tests can assert
// the call actually happened with the expected endpoint — not just that the resulting finding
// looks right. This is the non-vacuity guard the roadmap asks for: a test that only inspects
// findings would keep passing even if runDoctorRemote stopped calling the network path at all,
// as long as it happened to still produce the right finding by some other route.
type fakeDoctorRemoteExec struct {
	calls     []string
	responses map[string]string // keyed by strings.Join(args, " "), e.g. "auth status", "api repos/{owner}/{repo}"
	errors    map[string]error
}

func (f *fakeDoctorRemoteExec) exec(name string, args []string, stdin string) (string, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err, ok := f.errors[key]; ok {
		return "", err
	}
	if resp, ok := f.responses[key]; ok {
		return resp, nil
	}
	return "{}", nil
}

func baseDoctorRemoteDeps(hooksPath string, hooksPathErr error, exec *fakeDoctorRemoteExec, availGH bool) doctorRemoteDeps {
	return doctorRemoteDeps{
		execGit: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			switch joined {
			case "config --get core.hooksPath":
				if hooksPathErr != nil {
					return "", hooksPathErr
				}
				return hooksPath, nil
			case "remote get-url origin":
				return "https://github.com/kgsaran/trackfw.git", nil
			}
			return "", nil
		},
		execForgeAPI: exec.exec,
		availFn: func(name string) bool {
			if name == "gh" {
				return availGH
			}
			return false
		},
		configForge: "",
		repoDir:     "",
	}
}

// ── Falsification direction (a): repo WITHOUT required_status_checks → finding ────────────

func TestDoctorRemote_NoProtection_ProducesFindings(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":                    "logged in",
			"api repos/{owner}/{repo}":       `{"default_branch":"main","permissions":{"admin":true}}`,
			"api repos/{owner}/{repo}/branches/main/protection": "",
		},
		errors: map[string]error{
			"api repos/{owner}/{repo}/branches/main/protection": errors.New("gh: Branch not protected (HTTP 404)"),
		},
	}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, true)
	findings := runDoctorRemote(deps)

	if !hasKind(findings, integrations.DoctorRequiredStatusChecksMissing) {
		t.Errorf("expected required-status-checks-missing finding, got %+v", findings)
	}
	if !hasKind(findings, integrations.DoctorEnforceAdminsDisabled) {
		t.Errorf("expected enforce-admins-disabled finding, got %+v", findings)
	}
	if hasKind(findings, integrations.DoctorNotEvaluated) {
		t.Errorf("did not expect not-evaluated when the check genuinely ran: %+v", findings)
	}
	assertCalled(t, exec, "api repos/{owner}/{repo}/branches/main/protection")
}

// ── Falsification direction (b), the CONTROL: repo WITH the gate configured → no finding ──

func TestDoctorRemote_ProtectionConfigured_NoFindings(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":              "logged in",
			"api repos/{owner}/{repo}": `{"default_branch":"main","permissions":{"admin":true}}`,
			"api repos/{owner}/{repo}/branches/main/protection": `{"required_status_checks":{"strict":true,"contexts":["governance-go-install"]},"enforce_admins":{"enabled":true}}`,
		},
	}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, true)
	findings := runDoctorRemote(deps)

	if len(findings) != 0 {
		t.Errorf("expected zero findings for a fully configured gate (control case), got %+v", findings)
	}
	assertCalled(t, exec, "api repos/{owner}/{repo}/branches/main/protection")
}

// Same control, but exercised through the "checks" field instead of the legacy "contexts" —
// GitHub's branch protection response carries both; reading only contexts would false-fail this
// control on a repo configured through the newer API shape.
func TestDoctorRemote_ProtectionConfiguredViaChecksField_NoFindings(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":              "logged in",
			"api repos/{owner}/{repo}": `{"default_branch":"main","permissions":{"admin":true}}`,
			"api repos/{owner}/{repo}/branches/main/protection": `{"required_status_checks":{"strict":true,"contexts":[],"checks":[{"context":"governance-go-install"}]},"enforce_admins":{"enabled":true}}`,
		},
	}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, true)
	findings := runDoctorRemote(deps)

	if len(findings) != 0 {
		t.Errorf("expected zero findings when contexts is empty but checks is populated, got %+v", findings)
	}
}

// ── The case that decides the ADR: --remote with no credential → not-evaluated, never ok ──

func TestDoctorRemote_NoCredential_NotEvaluated_NeverOK(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		errors: map[string]error{
			"auth status": errors.New("gh: not logged into any GitHub hosts"),
		},
	}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, true)
	findings := runDoctorRemote(deps)

	if len(findings) != 1 || findings[0].FindingKind != integrations.DoctorNotEvaluated {
		t.Fatalf("expected exactly one not-evaluated finding, got %+v", findings)
	}
	if !strings.Contains(findings[0].Remedy, "authenticate") {
		t.Errorf("remedy should name authentication as the fix, got %q", findings[0].Remedy)
	}
	// Must never reach the branch-protection API call once there is no credential.
	for _, c := range exec.calls {
		if strings.HasPrefix(c, "api repos/{owner}/{repo}/branches/") {
			t.Errorf("should not have queried branch protection without a credential, calls=%v", exec.calls)
		}
	}
}

// ── Token present but insufficient scope (no admin) — DISTINCT message from no-credential ──

func TestDoctorRemote_InsufficientScope_DistinctFromNoCredential(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":              "logged in",
			"api repos/{owner}/{repo}": `{"default_branch":"main","permissions":{"admin":false}}`,
		},
	}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, true)
	findings := runDoctorRemote(deps)

	if len(findings) != 1 || findings[0].FindingKind != integrations.DoctorNotEvaluated {
		t.Fatalf("expected exactly one not-evaluated finding, got %+v", findings)
	}
	if strings.Contains(findings[0].Remedy, "authenticate first") {
		t.Errorf("insufficient-scope remedy must not reuse the no-credential remedy text, got %q", findings[0].Remedy)
	}
	if !strings.Contains(findings[0].Remedy, "admin access") {
		t.Errorf("remedy should name admin access as the fix, got %q", findings[0].Remedy)
	}
	for _, c := range exec.calls {
		if strings.HasPrefix(c, "api repos/{owner}/{repo}/branches/") {
			t.Errorf("should not have queried branch protection without admin access, calls=%v", exec.calls)
		}
	}
}

// ── gh CLI missing entirely ──────────────────────────────────────────────────────────────

func TestDoctorRemote_NoGHCLI_NotEvaluated(t *testing.T) {
	exec := &fakeDoctorRemoteExec{}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, false)
	findings := runDoctorRemote(deps)

	if len(findings) != 1 || findings[0].FindingKind != integrations.DoctorNotEvaluated {
		t.Fatalf("expected exactly one not-evaluated finding, got %+v", findings)
	}
	if len(exec.calls) != 0 {
		t.Errorf("should never shell out to gh when it is not on PATH, calls=%v", exec.calls)
	}
}

// ── Forge other than GitHub → not-evaluated, never a finding ────────────────────────────

func TestDoctorRemote_NonGitHubForge_NotEvaluated(t *testing.T) {
	exec := &fakeDoctorRemoteExec{}
	deps := baseDoctorRemoteDeps("", errors.New("not set"), exec, true)
	deps.execGit = func(args ...string) (string, error) {
		if strings.Join(args, " ") == "remote get-url origin" {
			return "git@gitlab.com:kgsaran/trackfw.git", nil
		}
		return "", errors.New("not set")
	}
	findings := runDoctorRemote(deps)

	if len(findings) != 1 || findings[0].FindingKind != integrations.DoctorNotEvaluated {
		t.Fatalf("expected exactly one not-evaluated finding, got %+v", findings)
	}
	if len(exec.calls) != 0 {
		t.Errorf("should never shell out to gh for a non-GitHub forge, calls=%v", exec.calls)
	}
}

// ── hooksPath: falsification in both directions ──────────────────────────────────────────

func TestDoctorRemote_HooksPathNeutralized_ProducesFinding(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":              "logged in",
			"api repos/{owner}/{repo}": `{"default_branch":"main","permissions":{"admin":true}}`,
			"api repos/{owner}/{repo}/branches/main/protection": `{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}`,
		},
	}
	deps := baseDoctorRemoteDeps("/dev/null", nil, exec, true)
	findings := runDoctorRemote(deps)

	if !hasKind(findings, integrations.DoctorHooksPathNeutralized) {
		t.Errorf("expected hooks-path-neutralized finding for /dev/null, got %+v", findings)
	}
}

func TestDoctorRemote_HooksPathUnset_NoFinding_Control(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":              "logged in",
			"api repos/{owner}/{repo}": `{"default_branch":"main","permissions":{"admin":true}}`,
			"api repos/{owner}/{repo}/branches/main/protection": `{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}`,
		},
	}
	deps := baseDoctorRemoteDeps("", errors.New("core.hooksPath not set"), exec, true)
	findings := runDoctorRemote(deps)

	if hasKind(findings, integrations.DoctorHooksPathNeutralized) {
		t.Errorf("did not expect hooks-path-neutralized when core.hooksPath is unset (control case), got %+v", findings)
	}
}

func TestDoctorRemote_HooksPathCustomHusky_NoFinding(t *testing.T) {
	exec := &fakeDoctorRemoteExec{
		responses: map[string]string{
			"auth status":              "logged in",
			"api repos/{owner}/{repo}": `{"default_branch":"main","permissions":{"admin":true}}`,
			"api repos/{owner}/{repo}/branches/main/protection": `{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}`,
		},
	}
	deps := baseDoctorRemoteDeps(".husky/_", nil, exec, true)
	findings := runDoctorRemote(deps)

	if hasKind(findings, integrations.DoctorHooksPathNeutralized) {
		t.Errorf("did not expect a legitimate husky hooksPath to be flagged, got %+v", findings)
	}
}

func hasKind(findings []integrations.DoctorFinding, kind integrations.DoctorFindingKind) bool {
	for _, f := range findings {
		if f.FindingKind == kind {
			return true
		}
	}
	return false
}

func assertCalled(t *testing.T, exec *fakeDoctorRemoteExec, wantSuffix string) {
	t.Helper()
	for _, c := range exec.calls {
		if c == wantSuffix {
			return
		}
	}
	t.Fatalf("expected a gh call %q, got calls=%v", wantSuffix, exec.calls)
}
