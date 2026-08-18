---
name: project-guard-dedup-hook-structure-gap
description: git-branch-guard/credential-guard global dedup and validate hook_resolvable compare only the "command" string field, never the hook object's structural validity (e.g. "type":"command") — found 2026-08-18, ML-4A barrier, reported not fixed (Hades has no write authority)
metadata:
  type: project
---

Found 2026-08-18 during ML-4A (Wave 4 barrier of
`ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md`):
`globalGitBranchGuardInstalledClaude()`/`hookArrayHasCommand` (dedup, `internal/generators/agentfiles.go`)
and `validateGuardGlobalHookResolvable`/`collectCommandsWithMarker` (validate,
`internal/validator/validator_git_branch_guard.go`) only ever compare the hook's `"command"` string
value — never the presence/correctness of `"type":"command"`, which Claude Code's real hooks schema
requires to actually invoke the hook. Proved by execution (throwaway Go test, not committed): a
`{"command": "<correct path>"}` entry missing `"type"` makes the dedup believe the global guard is
installed (skips project-scope wiring) AND makes `trackfw validate` report zero violations — while
the harness never actually invokes it. Net effect: neither scope protects, validate stays green.

**Why:** exploiting this requires `$HOME` already writable by the agent/attacker — same
precondition `ADR-2026-08-12` already declares out of the defense model — so I approved the roadmap
(no blocking) but named this as an explicit residual debt rather than letting it stay implicit,
since none of the REQ's 8 ACs covered hook structural validity, only command-string presence/path.

**How to apply:** if a future session touches `internal/generators/agentfiles.go`'s
`hookArrayHasCommand`/`simpleArrayHasValue`/`globalXInstalledY` family, or
`internal/validator/validator_git_branch_guard.go`'s `collectCommandsWithMarker`, know this gap
exists before assuming "dedup/validate says installed" implies "harness will actually run it". Full
write-up: `docs/seguranca/2026-08-17-revisao-do-guard-em-escopo-global.md` (section B) and
`vault/notes/global-guard-dedup-and-hook-resolvable-never-validate-hook-structure-2026-08-18.md`.
See also [project_serve_binds_all_interfaces](project_serve_binds_all_interfaces.md) for the sibling
finding pattern (global-scope trackfw artifacts silently not doing what config claims) and
[feedback_verify_by_execution](feedback_verify_by_execution.md) — this finding was only provable by
writing a throwaway test and running it, reading alone would not have surfaced it.
