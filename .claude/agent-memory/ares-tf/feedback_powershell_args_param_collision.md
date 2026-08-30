---
name: feedback-powershell-args-param-collision
description: Never name a PowerShell function parameter $Args — it collides with the automatic $args variable and silently drops the bound value, emptying subprocess argument lists with no error.
metadata:
  type: feedback
---

Never declare a PowerShell function parameter named `$Args` (or `$Env` — same class of risk with the `env:` drive). `$args` is an automatic variable inside every PowerShell function; a same-named parameter can silently fail to bind the caller's value, leaving the parameter's array/hashtable EMPTY inside the function body — with **no error, no warning**. A `[System.Diagnostics.ProcessStartInfo]` built from that empty array launches the target executable with zero arguments (e.g. bare `go` instead of `go run foo.go home`), and the tool just prints its own help text to stderr instead of failing loudly.

**Why this matters:** this was caught only because the agent (ares-tf) actually executed the script locally (via `brew install powershell` + `pwsh -File`) instead of trusting static/syntax review. `[Parser]::ParseFile` reports zero errors for this bug — it's 100% syntactically valid PowerShell. A pure code-review pass, even a careful one, would not catch it. This is the concrete instance of [[feedback... always execute generated scripts, don't just parse/read them]] for PowerShell specifically — see ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md, ML-1A.

**How to apply:** when writing any PowerShell helper function that wraps `Process.Start`/`ProcessStartInfo` (or any function taking an argument-list-shaped parameter), name it `$ArgList`/`$Arguments`/`$CommandArgs` — never `$Args`. Same caution for `$Env` vs `$EnvVars`/`$EnvOverrides` — `$env:` (colon-drive syntax) is unrelated syntactically, but the bare name `$Env` is still an easy collision to introduce by habit. After writing any non-trivial `.ps1` (especially anything with a `Run-Capture`-style subprocess wrapper), actually execute it end-to-end locally (macOS has `brew install powershell`) with representative inputs — don't stop at `[Parser]::ParseFile` syntax validation, which will not catch this class of bug.
