---
name: powershell-bom-trap
description: PowerShell Set-Content and WriteAllText default to UTF-8 BOM; serde_json rejects BOM silently unless --dangerously-bypass-hook-trust flag is used
metadata:
  type: feedback
---

PowerShell writes UTF-8 BOM (`EF BB BF`) by default for all file writes (`Set-Content`, `[IO.File]::WriteAllText` without explicit encoding). JSON parsers that don't handle BOM (serde_json, Go's encoding/json) reject files with "expected value at line 1 column 1" or similar.

**Why:** Measured 2026-11-17 on Windows VM. Original 6 Codex hook calls all used hooks.json with BOM → hooks silently disabled → measurement concluded "PreToolUse doesn't fire in exec mode" → conclusion was WRONG. After removing BOM, both SessionStart and PreToolUse hooks fired normally.

**How to apply:** Any time writing JSON to Windows via PowerShell, always use:
```powershell
$utf8NoBOM = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllText($path, $json, $utf8NoBOM)
```
Or write on macOS and transfer via scp. The error is SILENT without `--dangerously-bypass-hook-trust`; with the flag, it appears as an `error` item in the JSONL output.

Also: `exit 0` in a `.cmd` file called via `call` from another batch file kills the outer cmd.exe. Use `exit /b 0` to return to caller. When Codex runs hooks via `cmd.exe /C stubfile.cmd` (spawns a NEW cmd.exe), `exit 0` is fine. But for direct testing with `call`, always use `exit /b 0`.
