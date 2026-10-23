package validator

// guard_ac2_lines_ml1c_test.go — AC2 reconciliação (REQ-2026-10-23 / ML-1C)
//
// Asserts byte-for-byte equality between the line-builder functions in
// validator_guard_binary_probe.go and the canonical/legacy constants in
// internal/guardforms.  If either side drifts, these tests fail, making the
// discrepancy visible before it produces a silent false-ok in the validator.
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   Every assertion below corresponds to a contract stated in ML-1C: the
//   validator must recognise the exact strings that guardforms declares as
//   canonical/legacy.  The functions under test are the validator's only
//   source for those strings; if they diverge from the constants, the
//   validator silently mis-classifies hooks.
//
//   These tests PASS on origin/main (the functions already matched the
//   constants for their original subcmds) — they document and pin the
//   contract so that future changes to either side trigger an immediate
//   failure rather than a silent validator divergence.  An AC2 test is
//   not discriminating against origin/main; its value is as a guard against
//   future drift.

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

func TestAC2_GuardExpectedLine_MatchesGuardforms(t *testing.T) {
	// Afirma: guardExpectedLine produz a mesma string que as constantes de
	// guardforms para cada subcomando × família de shell.
	cases := []struct {
		name    string
		fn      func() string
		want    string
	}{
		{
			name: "credential PSPOSIX",
			fn:   func() string { return guardExpectedLine("credential", guardShellFamilyPSPosix) },
			want: guardforms.CredentialCurrentPSPOSIX,
		},
		{
			name: "credential --global PSPOSIX",
			fn:   func() string { return guardExpectedLine("credential --global", guardShellFamilyPSPosix) },
			want: guardforms.CredentialGlobalCurrentPSPOSIX,
		},
		{
			name: "git-branch PSPOSIX",
			fn:   func() string { return guardExpectedLine("git-branch", guardShellFamilyPSPosix) },
			want: guardforms.GitBranchCurrentPSPOSIX,
		},
		{
			name: "credential CmdExe",
			fn:   func() string { return guardExpectedLine("credential", guardShellFamilyCmdExe) },
			want: guardforms.CredentialCurrentCmdExe,
		},
		{
			name: "git-branch CmdExe",
			fn:   func() string { return guardExpectedLine("git-branch", guardShellFamilyCmdExe) },
			want: guardforms.GitBranchCurrentCmdExe,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if got != tc.want {
				t.Errorf("guardExpectedLine(%q):\n  got  %q\n  want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestAC2_GuardD11RevisedLegacyLine_MatchesGuardforms(t *testing.T) {
	// Afirma: guardD11RevisedLegacyLine produz exatamente os valores LegacyD11Rev*.
	cases := []struct {
		name string
		fn   func() string
		want string
	}{
		{
			name: "credential PSPOSIX",
			fn:   func() string { return guardD11RevisedLegacyLine("credential", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD11RevCredentialPSPOSIX,
		},
		{
			name: "credential --global PSPOSIX",
			fn:   func() string { return guardD11RevisedLegacyLine("credential --global", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD11RevCredentialGlobalPSPOSIX,
		},
		{
			name: "git-branch PSPOSIX",
			fn:   func() string { return guardD11RevisedLegacyLine("git-branch", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD11RevGitBranchPSPOSIX,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if got != tc.want {
				t.Errorf("guardD11RevisedLegacyLine(%q):\n  got  %q\n  want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestAC2_GuardD11LegacyLine_MatchesGuardforms(t *testing.T) {
	// Afirma: guardD11LegacyLine produz exatamente os valores LegacyD11*.
	cases := []struct {
		name string
		fn   func() string
		want string
	}{
		{
			name: "credential PSPOSIX",
			fn:   func() string { return guardD11LegacyLine("credential", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD11CredentialPSPOSIX,
		},
		{
			name: "credential --global PSPOSIX",
			fn:   func() string { return guardD11LegacyLine("credential --global", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD11CredentialGlobalPSPOSIX,
		},
		{
			name: "git-branch PSPOSIX",
			fn:   func() string { return guardD11LegacyLine("git-branch", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD11GitBranchPSPOSIX,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if got != tc.want {
				t.Errorf("guardD11LegacyLine(%q):\n  got  %q\n  want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestAC2_GuardD2LegacyLine_MatchesGuardforms(t *testing.T) {
	// Afirma: guardD2LegacyLine produz exatamente os valores LegacyD2*.
	cases := []struct {
		name string
		fn   func() string
		want string
	}{
		{
			name: "credential PSPOSIX",
			fn:   func() string { return guardD2LegacyLine("credential", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD2CredentialPSPOSIX,
		},
		{
			name: "git-branch PSPOSIX",
			fn:   func() string { return guardD2LegacyLine("git-branch", guardShellFamilyPSPosix) },
			want: guardforms.LegacyD2GitBranchPSPOSIX,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if got != tc.want {
				t.Errorf("guardD2LegacyLine(%q):\n  got  %q\n  want %q", tc.name, got, tc.want)
			}
		})
	}
}
