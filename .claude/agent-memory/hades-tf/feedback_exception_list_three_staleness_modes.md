---
name: exception-list-three-staleness-modes
description: Exception list for a gate has three staleness modes, not one; pin counts not offsets; never use calendar expiry
metadata:
  type: feedback
---

When a gate allows declared exceptions, implement three staleness modes — all three must be checked:

(a) File no longer exists on disk → stale exception (e.g., Wave N deleted it)
(b) File exists but NUL/flag count is now 0 → fix was applied; exception unnecessary
(c) File exists but real count diverges from declared → new violation sneaked in, or partial fix

**Do NOT** pin offsets — they shift with every unrelated edit to the file. Pin counts; report offsets only in failure messages.

**Do NOT** use calendar expiry ("expires 2026-12-01"). Calendar dates decouple from the actual condition. Use structural triggers: the three modes above are what "prazo ligado à Wave 3" actually means — when the file disappears, mode (a) fires automatically.

**Why:** An exception list with only mode (a) (file-missing) can become permanent: if someone applies `\0` escape to clean the file, the exception stays forever with count=0. Mode (b) catches that. Mode (c) catches count drift.

**How to apply:** Any gate that has an allowlist/exception mechanism: implement all three modes. Template (bash): check file exists + check count != 0 + check count == declared.

See: `scripts/check-no-literal-nul-in-source.sh` + `scripts/nul-source-exceptions.txt` (ML-1F, 2026-09-13).
