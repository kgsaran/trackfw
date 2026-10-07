---
name: project_psexecutionpolicypref_bypass_inheritance
description: PowerShell Bypass policy is inherited by child processes via $env:PSExecutionPolicyPreference — tests under Restricted must use cmd.exe as parent process
metadata:
  type: project
---

When PowerShell is started with `-ExecutionPolicy Bypass`, it sets `$env:PSExecutionPolicyPreference=Bypass`. Child PowerShell processes spawned via `System.Diagnostics.ProcessStartInfo` (with `UseShellExecute=$false`) inherit this environment variable and effectively run under Bypass, masking Restricted behavior.

**Why:** Measured on 2026-10-04 during ML-0B. The `measure.ps1` script ran with Bypass, and the S6 test of `trackfw.ps1` reported exit:0 — appearing to succeed under Restricted. The correct test from `cmd.exe` as parent correctly showed exit:1 (blocked).

**How to apply:** When measuring execution policy behavior on Windows VM, always use `cmd.exe` as the parent process (not a PowerShell process that might be running with Bypass). Verify with `cmd /c powershell.exe -NoProfile -Command "Get-ExecutionPolicy"` → should return `Restricted`.
