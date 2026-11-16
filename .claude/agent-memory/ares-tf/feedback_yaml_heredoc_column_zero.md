---
name: yaml-heredoc-column-zero
description: PowerShell here-string @"..."@ requires closing "@ at column 0 — this terminates YAML run: | block scalar
metadata:
  type: feedback
---

Inside a YAML `run: |` literal block scalar, indentation is relative to the block's indent level. PowerShell's here-string `@"..."@` requires the closing `"@` at column 0 (absolute). If any content line inside the here-string also falls at column 0 (e.g., the first `{` of a JSON value), YAML sees it as a new mapping key and throws "while scanning a simple key".

**Why:** Measured in ML-3B: `$pkgJson = @"{"name":"@trackfw-bin/win32-x64"...}"@` caused a YAML parse error because the `{` line was at column 0 inside the YAML block.

**How to apply:** Never use PowerShell here-strings (`@"..."@` or `@'...'@`) inside YAML `run: |` blocks when the content might have lines at column 0. Use single-line string concatenation instead: `$s = '{"key":"' + $var + '"}'`. Same applies to any multi-line construct that requires column-0 content.
