---
name: cursor-stdin-bom
description: Cursor always sends UTF-8 BOM (EF BB BF) in stdin to hooks, even when the temp file has no BOM
metadata:
  type: project
---

Cursor's hook script: `$OutputEncoding = [System.Text.Encoding]::UTF8; Get-Content -LiteralPath '<tempFilePath>' -Raw | & { $input | <command> }`

`[System.Text.Encoding]::UTF8` in .NET is the UTF-8 WITH BOM variant. When PS serializes the string to the external process pipe, it prepends EF BB BF.

**Measured:** 2026-10-04, ML-1D, VM ARM64 Windows 11. File bytes: `[7B 22 61 22 3A 31 7D]` (no BOM). Probe stdin: `[EF BB BF 7B 22 61 22 3A]` (BOM present). EOF: <1ms.

**Why:** Node.js writes the temp file with UTF-8 no-BOM (`writeFile(h, o, "utf8")`), but PS re-encodes when piping to external process using `$OutputEncoding` which has BOM.

**How to apply:** The Go guard (`internal/guard/`) MUST strip UTF-8 BOM (EF BB BF) before `json.Unmarshal`. Without this, Cursor payloads fail with "invalid character" — fail-closed but rejects all legitimate Cursor commands. This is a required fix for ML-1A.
