package validator

// credentialGuardScriptReference is a validator-local copy of the project-scope
// scripts/trackfw-credential-guard.sh template composed in internal/generators/scaffold.go
// (credentialGuardScript). internal/validator cannot import internal/generators directly:
// generators/context.go already imports validator (for `trackfw context`), so importing
// generators from validator would create a Go import cycle.
//
// ML-2A (ADR-2026-10-04): credentialGuardScript is now a thin wrapper that delegates to
// `trackfw guard credential`. This reference constant is updated to match.
//
// Drift between this copy and the real generator is caught by TestCredentialGuardScriptReference_
// MatchesGenerator (validator_credential_guard_integrity_external_test.go, package
// validator_test — an EXTERNAL test package, which is allowed to import both internal/generators
// and internal/validator without reintroducing the cycle in production code): it regenerates the
// script via generators.GenerateCredentialGuardScript into a temp dir and asserts byte-equality
// against this constant.
const credentialGuardScriptReference = `#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard credential` + "`" + `.
# Script is intentionally a no-op when executed outside the project root
set -euo pipefail
[ -f trackfw.yaml ] || exit 0
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential
`
