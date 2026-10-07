---
name: restricted-exit-lastexitcode-failopen
description: Under Restricted policy, "; exit $LASTEXITCODE" suffix causes fail-OPEN (exit 0) not fail-closed, when .ps1 shim is blocked by PSSecurityException
metadata:
  type: project
---

Measured 2026-10-04, ML-1D, VM ARM64 Windows 11.

Form: `powershell -NoProfile -Command "probe; exit $LASTEXITCODE"` (cmd parent, Restricted, probe resolves to .ps1)

Result: PSSecurityException shown, then `RESTRICTED_EXIT: 0` (not 1).

**Why:** PSSecurityException is a non-terminating error in the PS command sequence. `exit $LASTEXITCODE` runs after the exception. `$LASTEXITCODE` is only updated by external native processes — a PS exception does NOT update it. So `$LASTEXITCODE` remains 0, and exit 0 → fail-open.

WITHOUT the suffix (`powershell -NoProfile -Command "probe"`): the PSSecurityException terminates PS with exit 1. That's what ML-0B measured.

**How to apply:**
- The `; exit $LASTEXITCODE` suffix makes Restricted WORSE (0 instead of 1).
- For npm channel under Restricted: both forms fail (shim blocked), but suffix produces fail-open instead of fail-closed.
- RemoteSigned resolves this (no MoTW on npm shim → runs as local → exit 2 propagated).
- Kiro/Amazon Q must NOT get the suffix (cmd.exe treats `;` as literal → args passed to binary → cobra help → exit 0).
