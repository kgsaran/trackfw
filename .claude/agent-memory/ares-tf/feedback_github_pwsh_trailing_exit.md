---
name: github-pwsh-trailing-exit
description: GitHub Actions pwsh shell appends exit $LASTEXITCODE — last step must exit 0 explicitly or rc propagates from last native call
metadata:
  type: feedback
---

GitHub's `shell: pwsh` wrapper appends `if ((Test-Path -LiteralPath variable:\LASTEXITCODE)) { exit $LASTEXITCODE }` after every step script. If the last native call in the step returns rc≠0 (e.g., a falsification arm returning rc=2 by design), the step exits with that rc even if you printed "ALL N PASSED".

**Why:** Measured in ML-3B: falsification assertion 6 (write-scope CRLF stdin → rc=2 by design) was the last call; the step would exit 2 and fail CI even though all assertions passed.

**How to apply:** In every `shell: pwsh` step that ends with an intentional non-zero native call, add `exit 0` immediately after the success message. Same applies to `shell: cmd`: add `exit /b 0` at the end. Never let the last native call be one that returns rc≠0 by design.
