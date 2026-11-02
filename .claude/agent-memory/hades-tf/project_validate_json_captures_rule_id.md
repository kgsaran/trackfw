---
name: project-validate-json-captures-rule-id
description: validate --json writes via cmd.OutOrStdout() (SetOut captures it); plain mode uses fmt.Printf (not capturable by SetOut); counter-arms for req_has_adr use --json
metadata:
  type: project
---

Measured 2026-11-02 (feat/primeira-experiencia, binário 11.2.5):

- `validate` (plain mode): violations go to `fmt.Printf` (global os.Stdout). `SetOut(buf)` does NOT capture them. Assert `err == nil` (positive) or `err != nil` (negative).
- `validate --json`: writes JSON through `cmd.OutOrStdout()`. `SetOut(buf)` DOES capture it. JSON shape: `{"violations":[{"rule":"req_has_adr","file":"REQ-…","message":"…"}],…}`.
- No `os.Exit` in `validate.go` — only in the `commands.Execute()` wrapper (root.go:121). Tests use `newRootCmd().Execute()` directly; errors propagate via cobra `RunE` return.

**Why:** validate uses `RunE` which returns `newExitCodeError(1, …)`. The binary's `Execute()` wrapper catches that and calls `os.Exit`. In-process tests skip the wrapper.

**How to apply:** counter-arm for req_has_adr violation: `rootCA.SetArgs([]string{"validate","--json"})` + `rootCA.SetOut(buf)` + `assert.Contains(buf.String(), "req_has_adr")`.

Plain mode violation text (for reference): `✗ req "…" has no linked ADR (marker must start the line with a real, non-placeholder value)` — no rule ID in this string.
