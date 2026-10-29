---
name: install_ps1_semicolon_installdir_idempotence
description: INSTALL_DIR semicolon fixed in ML-4C (1257d81b); four LOW residuals (A-1 to A-4) documented in reauditoria 2026-10-29; none block merge
metadata:
  type: project
---

**FIXED in ML-4C (commit 1257d81b):** `scripts/install.ps1` now rejects INSTALL_DIR with ASCII
semicolon at line 131 (`-match ';'`), before any write. C1 (code) and C2 (gate vector + counter-arm
`c1-intact`) both delivered. CI run 40918775429: 39/39 PASS (39 scenarios × 2 PS versions = 78
PASS lines; 3 extra macOS-only scenarios SKIP on Windows runner).

**Reauditoria confirmed (2026-10-29):** all three ASCII `;` vectors (`C:\tf;%TEMP%`, `;C:\tf`,
`C:\tf;`) throw at line 132 with rc=1 when run against the real `install.ps1`.

**Open residual A-1 (LOW) — Unicode semicolon variants:** U+FF1B (FULLWIDTH SEMICOLON), U+FE54
(SMALL SEMICOLON), U+037E (GREEK QUESTION MARK) pass `-match ';'`. None are Windows PATH
separators in Unicode mode (confirmed: `-split ';'` uses ordinal, measured). bestfit1252.txt
(unicode.org/MICSFT/WindowsBestFit) confirms U+FF1B → 0x3B and U+037E → 0x3B; U+FE54 not
listed. Not measured end-to-end. Fix: `-match '[;\u037E\uFE54\uFF1B]'`.

**Open residual A-2 (LOW) — `Join-Path` UNC/PSDrive causes rc≠0:** two sites in install.ps1.
Sítio 1: USERPROFILE block (lines 304–305) — UNC USERPROFILE measured on macOS, throws
terminating error, install exits rc≠0 after binary copy + PATH write (functionally complete).
Sítio 2: "other copies" loop (line 290) — UNC PATH entry causes same abort, before advisory
section. Fix: `[IO.Path]::Combine(...)` at both sites.

**Open residual A-3 (LOW) — cmd.exe forms in cli-parity.md outside gate:** removing `|| exit 2`
from cmd.exe row passes `real_file_green` (measured). Gap declared in code comment lines 279–298
of readme_line_test.go. No regression in HEAD; cell is documentation, not an installed hook.

**Open residual A-4 (LOW) — `.ToLower()` culture-sensitive, tr-TR false-positive warning:**
paths with capital I (INSTALL_DIR lowercase vs USERPROFILE uppercase) produce `ı` ≠ `i` under
tr-TR, spurious warning. Fix: `[string]::Equals(a, b, [StringComparison]::OrdinalIgnoreCase)`.

**Why:** a list separator inside a path value defeats any split-based idempotence check. The
Unicode, UNC, cmd.exe and locale edge cases were found in the reauditoria of ML-4C/4D.
