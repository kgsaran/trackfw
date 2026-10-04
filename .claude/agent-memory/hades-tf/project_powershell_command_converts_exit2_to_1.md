---
name: project_powershell_command_converts_exit2_to_1
description: powershell.exe -Command converts native binary exit 2 to exit 1 (documented PS 7, not yet measured live for PS 5.1); Codex/Windsurf/Cursor/Claude Code PS fail-open if confirmed; requires ML-0D
metadata:
  type: project
---

`powershell.exe -Command "native.exe"` where native.exe exits 2: in PS 7 (`pwsh`), the process exits 1.
Mechanism: native exit 2 sets `$LASTEXITCODE=2`, `$?=$false`, and pwsh exits based on `$?` (true/false -> 0/1), not `$LASTEXITCODE`.

**PS 7 status:** documented in `about_Pwsh` (learn.microsoft.com).
**PS 5.1 status:** NOT measured at live with a probe binary. No primary source found confirming that powershell.exe (PS 5.1) maps native binary exit codes via `$?`. Treat as hypothesis until ML-0D.

**Impact on guard Go design (ADR-2026-10-04, found 2026-10-04 ML-0A):**
- CLIs using bare `powershell -Command "trackfw guard git-branch"` without suffix: Codex, Windsurf, Claude Code PS fallback, Cursor
- If conversion confirmed in PS 5.1: these CLIs receive exit 1 from PowerShell -> fail-open (block only on exit 2)
- Gemini is correct for PS: hardcodes `; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }` suffix in hookRunner.ts (PS-only; causes bash syntax error -> deny-all if emitted for bash CLI)
- Kiro (cmd.exe), Amazon Q (cmd.exe): exit code preserved -> correct
- Copilot: fail-closed on any non-zero -> immune

**Cross-shell behavior of `; exit $LASTEXITCODE` suffix (measured 2026-10-04):**
In bash/sh/zsh, `$LASTEXITCODE` is unset -> expands to empty word -> bare `exit` -> exits with last command's status. Measured: `bash -c '(exit 2); exit $LASTEXITCODE'; echo $?` -> 2. Suffix preserves exit code in POSIX shells.
cmd.exe: `;` is NOT a command separator in cmd.exe (`&` is). [hypothesis, not measured on live Windows cmd.exe — pending ML-0D experiment arm 8] `trackfw guard git-branch; exit $LASTEXITCODE` in cmd.exe likely passes extra tokens to the binary (exact tokenization unconfirmed from primary source). Proxy measured on macOS: `trackfw roadmap 'move;' exit '$LASTEXITCODE'` → cobra help → exit 0. Do NOT emit this suffix for cmd.exe CLIs (Kiro, Amazon Q) until ML-0D confirms. Per-CLI emission required.
D2 holds per-CLI hook line, not as a single universal string.

**Fix (if ML-0D confirms):** ML-2A emits `; exit $LASTEXITCODE` suffix for Codex, Windsurf, Claude Code PS, Cursor.

**stdin delivery findings (measured 2026-10-04, bash 5.3.20, same roadmap ML-0A):**
- git-branch guard uses successive 2s read windows (L86-101). Guard denies when a window returns empty (TRUNCATED). NOT "2s after last byte".
  - `(payload; sleep 3) | git-branch-guard` → allow: EOF arrives in second window (t=3s < t=4s window end)
  - `(payload; sleep 6) | git-branch-guard` → deny at ~4s: second window expires at t=4s with empty chunk before EOF at t=6s
  - **FP**: if PS doesn't close the pipe within the current 2s window, benign commands are denied (~4s). ML-1A parity gate: must test BOTH sleep 3 (allow) and sleep 6 (deny ~4s).
- credential guard uses `INPUT=$(cat)` with no timeout (L5). Hangs until EOF.
  - `(payload; sleep 6) | credential-guard` → exit 0 at 6.046s (cred_late:0)
  - **FN conditional**: if the CLI's own timeout fires and allows, the guard didn't block. Copilot documented (fail-closed except on timeout); other CLIs undetermined (ML-0D).
  - **Same delivery vector, opposite directions**: pipe not closed → git-branch FP (denies legit), credential FN (allows if CLI timeout fires).
  - Architectural decision for ML-1B: faithful port (no timeout, declared residual) vs. add timeout as authorized behavior change.

**Why:** ML-0B couldn't detect this: the 9.2.0 binary also exits 1 (unknown subcommand), masking the conversion.

**How to apply:** Check if PS-based CLI runner propagates `$LASTEXITCODE` or uses bare `-Command`. For PS 7, documented. For PS 5.1, verify in ML-0D before writing verdicts. Never emit `; exit $LASTEXITCODE` for cmd.exe CLIs (Kiro, Amazon Q) — hypothesis until ML-0D arm 8 confirms tokenization.
