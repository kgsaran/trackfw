---
name: project_npm_ps1_restricted_blocks
description: npm trackfw shim (trackfw.ps1) is blocked by PowerShell Restricted policy when npm is first in PATH; pip-first ordering masks this risk
metadata:
  type: project
---

The npm channel for trackfw generates three shims: `trackfw` (no ext), `trackfw.cmd`, and `trackfw.ps1`. Under `Restricted` ExecutionPolicy (Windows default), PowerShell cannot run `.ps1` files.

**Key behavior (measured 2026-10-04):**
- When npm is FIRST in PATH: PowerShell resolves `trackfw.ps1` (ExternalScript) → BLOCKED, PSSecurityException, exit 1
- When pip is FIRST in PATH: PowerShell resolves `trackfw.exe` (Application) → OK, exit 0
- cmd.exe: always uses `trackfw.exe` from pip (first `.exe`) → OK
- This VM has pip before npm in PATH, which masks the npm channel risk

**Why it matters:** A pure npm install (no pip, no GitHub exe) would leave PowerShell unable to run trackfw under Restricted. This is the ADR-2026-10-04 D5 risk 1, confirmed as real.

**How to apply:** When testing npm-only scenarios on Windows, explicitly set npm PATH first. The Go binary approach (`trackfw guard git-branch`) resolves this for pip/GitHub channels since they deliver `.exe` directly.

**Measured context:** VM `192.168.64.6`, `trackfw 9.2.0`, Windows ARM64, Restricted effective policy.
