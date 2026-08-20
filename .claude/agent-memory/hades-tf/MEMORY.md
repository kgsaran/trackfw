# Memory Index — Hades (Security, trackfw)

- [trackfw serve binds all interfaces (Go/Python)](project_serve_binds_all_interfaces.md) — found 2026-08-16, FIXED and verified same day (ML-2A barrier); loopback default + opt-in --host now in all 3 CLIs
- [Verify by execution, not just reading](feedback_verify_by_execution.md) — for serve/CLI security review, spin up the process and curl/lsof it; reading alone missed the bind-address bug
- [Global guard dedup/validate only compares command string, not hook structure](project_guard_dedup_hook_structure_gap.md) — found 2026-08-18, ML-4A barrier, APPROVED with named residual debt, not fixed
- [Reverifying my own block: lift only what the fix actually closed](feedback_reverification_own_block_scope.md) — release tag ML-4A→4C 2026-08-19; commit-target fix didn't close version-squat/forged-message damages from the same original block
