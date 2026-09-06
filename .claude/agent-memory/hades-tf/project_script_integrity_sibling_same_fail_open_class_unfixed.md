---
name: script-integrity-sibling-same-fail-open-class-unfixed
description: credential_guard_script_integrity / git_branch_guard_script_integrity (project+global) have the same read-error fail-open/abort class ML-1B fixed elsewhere — found live 2026-09-06, not fixed, not declared in scope
metadata:
  type: project
---

Found during the ML-1B barrier (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio),
same REQ as [[project_guard_hook_read_ioerror_fail_open_node_python]]. ML-1B fixed read-error
handling in `validateGuardHookResolvable`/`validateGuardGlobalHookResolvable` (the functions that
read the *hook config* files, e.g. `.claude/settings.json`). It did NOT touch the sibling
`*_script_integrity` family, which reads the *generated script* (`scripts/trackfw-credential-guard.sh`)
and has the identical defect class, live and unfixed as of 2026-09-06:

- Go project-scope (`internal/validator/validator_credential_guard_integrity.go:41`,
  `validator_git_branch_guard.go:28`): `return nil, fmt.Errorf(...)` on any non-ENOENT read error,
  propagated via `return nil, nil, e` in `validator.go:490-497` — **aborts the entire `trackfw
  validate` run**, non-JSON stdout. Reproduced live: `chmod 000` on `scripts/` dir → single raw
  error line, no summary.
- Go global-scope (`validator_git_branch_guard.go:319` `validateGuardGlobalScriptIntegrity`):
  opposite bug — `return nil, nil` unconditionally on ANY read error, silent fail-open, no crash.
- Python (`pypi/trackfw/validator.py:3324` `validate_guard_script_integrity`, shared by both
  project and global scope, both rule families): `except OSError: return []` — collapses
  FileNotFoundError with PermissionError/IsADirectoryError, always silent. Same bug class ML-1B
  fixed in `validate_guard_hook_resolvable`, alive here.
- Node: correct — emits a structured "could not inspect ... EACCES" warning. Only runtime that
  behaves as this whole REQ intends.

Severity is NOT uniform: Python's silent branch got compensated in the one scenario measured —
the sibling rule this ML DID fix (`credential_guard_hook_resolvable`) still fired a real violation
(wrong message: "script does not exist" instead of "could not be read", but `exit_code: 1`, user
not left blind). Go global-scope is silent but doesn't break anything beyond itself. Go
project-scope is the real severity concentration: the raw `fmt.Errorf` breaks the documented
`--json` contract entirely — a CI consumer parsing stdout gets unparseable output and loses every
other rule's result, not just this one. All three findings are pre-existing (not introduced by
ML-1B) — confirmed via `git log -- <file>`, last touched by REQ #162/#163, unrelated to this REQ.

**Why:** the REQ's own diagnosis ("o controle reporta saúde sobre o que não conseguiu ler") is not
actually closed by ML-1B — it's endemic to a sibling function family that nobody enumerated. This
is the third time (per [[feedback_execute_all_named_vectors_before_verdict]] and
[[feedback_measure_the_real_code_path_not_an_isolated_proxy]] pattern) that the true blast radius
of a "config the tool can't read" bug turned out bigger than the ML's stated scope, once measured
live instead of trusting the handoff's enumeration.

**How to apply:** if this REQ reopens again, check whether a follow-up ML fixed the
`*_script_integrity` family before re-auditing anything else in this REQ. If not fixed, this is
still open. Verify file paths/line numbers still match before citing — this repo moves fast.
