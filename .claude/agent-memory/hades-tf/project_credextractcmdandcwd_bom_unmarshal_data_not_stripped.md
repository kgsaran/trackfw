---
name: credextractcmdandcwd-bom-unmarshal-data-not-stripped
description: credExtractCmdAndCwd passes raw data (with BOM) instead of stripped to json.Unmarshal — disables Layer 2b for BOM-prefixed payloads in branch
metadata:
  type: project
---

Found: 2026-10-08, ML-2B red-team of REQ-2026-10-06.

`credExtractCmdAndCwd` in `internal/guard/credential.go` line ~373 computes
`stripped = bytes.TrimPrefix(data, BOM)` but passes `data` (not `stripped`) to
`json.Unmarshal`. Go's stdlib rejects BOM as invalid JSON → `isJSON=false` → Layer 2b
disabled for all BOM-prefixed payloads.

**Why:** PowerShell may emit UTF-8 BOM before JSON stdout (documented as code comment in
same package, commit 782f5767). New function `credExtractCmdAndCwd` introduced by ML-1C
did not replicate the BOM handling that existed in the older `ExtractCommand`.

**How to apply:** Any credential guard red-team involving PowerShell payloads or
CI environments that emit BOM must include at least one BOM-prefixed vector. The fix is
one line: `json.Unmarshal(stripped, &root)` — NOT corrected until ML-2C is merged.
Both `RunCredential` (line 87) and `RunCredentialGlobal` (line 166) share this function.
