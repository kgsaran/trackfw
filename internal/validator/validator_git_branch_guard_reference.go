package validator

// gitBranchGuardScriptReference is a validator-local copy of the
// scripts/trackfw-git-branch-guard.sh template composed in internal/generators/scaffold.go
// (gitBranchGuardScript const). internal/validator cannot import internal/generators directly:
// generators/context.go already imports validator (for `trackfw context`), so importing
// generators from validator would create a Go import cycle -- same constraint documented in
// validator_credential_guard_integrity_reference.go for credentialGuardScriptReference.
//
// ML-2A (ADR-2026-10-04): gitBranchGuardScript is now a thin wrapper that delegates to
// `trackfw guard git-branch`. This reference constant is updated to match.
//
// This single reference constant covers both git_branch_guard_script_integrity (project,
// scripts/trackfw-git-branch-guard.sh) and the global integrity check
// (~/.trackfw/scripts/trackfw-git-branch-guard.sh) -- both scopes use the same thin wrapper.
//
// Drift between this copy and the real generator is caught by
// TestGitBranchGuardScriptReference_MatchesGenerator
// (validator_git_branch_guard_integrity_external_test.go, package validator_test -- an EXTERNAL
// test package, which is allowed to import both internal/generators and internal/validator
// without reintroducing the cycle in production code): it regenerates the script via
// generators.GenerateGitBranchGuardScript into a temp dir and asserts byte-equality against this
// constant.
const gitBranchGuardScriptReference = `#!/usr/bin/env bash
# trackfw-git-branch-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard git-branch` + "`" + `.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
if [ $# -gt 0 ]; then exec trackfw guard git-branch --command "$*"; fi
exec trackfw guard git-branch
`
