package validator

// credentialGuardGlobalScriptReference is a validator-local copy of the GLOBAL-scope
// ~/.trackfw/scripts/trackfw-credential-guard.sh template composed in
// internal/generators/scaffold.go (globalCredentialGuardScript). This is a DIFFERENT template
// than credentialGuardScriptReference (the project-scope variant): the global variant omits the
// project-guard block (`[ -f trackfw.yaml ] || exit 0`) and calls `trackfw guard credential
// --global` instead of `trackfw guard credential`.
//
// ML-2A (ADR-2026-10-04): globalCredentialGuardScript is now a thin wrapper. Updated to match.
//
// internal/validator cannot import internal/generators directly (import cycle — see
// credentialGuardScriptReference's doc comment in
// validator_credential_guard_integrity_reference.go for the full explanation). Drift between this
// copy and the real generator is caught by
// TestCredentialGuardGlobalScriptReference_MatchesGenerator
// (validator_credential_guard_global_integrity_external_test.go, package validator_test).
const credentialGuardGlobalScriptReference = `#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard credential --global` + "`" + `.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential --global
`
