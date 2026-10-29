---
name: install_ps1_semicolon_installdir_idempotence
description: TRACKFW_INSTALL_DIR with semicolon passes absolute-path regex, breaks idempotence check — confirmed in ML-4B red team 2026-10-29
metadata:
  type: project
---

`scripts/install.ps1` line 130 validates INSTALL_DIR with `-cmatch '\A[A-Za-z]:[/\\]'` but does not
reject semicolons. A value like `C:\tf;%TEMP%` passes.

**Why it matters:** the idempotence check (lines 245–261) splits PATH on `;` and compares individual
entries against `$installNorm = $INSTALL_DIR.TrimEnd('\','/').ToLower()`. If INSTALL_DIR contains `;`,
`$installNorm` is never matched by any split entry, so `alreadyPresent = false` on every run —
PATH grows unboundedly. Confirmed by isolated test in pwsh 7 (2026-10-29).

The `%TEMP%` variant (`C:\tf;%TEMP%`) also injects the temp directory into PATH when
the registry value is REG_EXPAND_SZ (preserved by the installer per ADR). End-to-end
Windows reachability NOT measured (macOS environment), but the validation gap is confirmed.

**Wave 0 gap:** §2.4 covers junction/symlink, not semicolons. Gap is triple: Wave 0 + code + gate.

**Corrective C1:** add `if ($INSTALL_DIR -match ';') { throw ... }` after line 132.
**Corrective C2:** add `Assert-ValidationFail 'install-dir-semicolon'` to check-install-ps1.ps1 with
`TRACKFW_INSTALL_DIR = 'C:\tf;C:\evil'`; cite `c1-intact` as counter-arm.

**Why:** a list separator inside a path value defeats any split-based "already present" check.
The regex validator's job is to reject chars that break downstream invariants — `;` breaks PATH
split idempotence even if NTFS allows the character in a filename.
