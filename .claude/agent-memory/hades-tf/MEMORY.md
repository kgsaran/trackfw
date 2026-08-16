# Memory Index — Hades (Security, trackfw)

- [trackfw serve binds all interfaces (Go/Python)](project_serve_binds_all_interfaces.md) — found 2026-08-16, FIXED and verified same day (ML-2A barrier); loopback default + opt-in --host now in all 3 CLIs
- [Verify by execution, not just reading](feedback_verify_by_execution.md) — for serve/CLI security review, spin up the process and curl/lsof it; reading alone missed the bind-address bug
