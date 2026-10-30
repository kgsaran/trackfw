---
name: exit-code-error-t1-t3-gaps
description: ExitCodeError Code=256 bypasses constructor (os.Exit(256)=rc=0); T3 (errors.As) ungated — no subprocess test exercises Execute()
metadata:
  type: project
---

ML-2A barrier 2026-10-30 (REQ-2026-09-07).

**T1 gap — Code=256**: `newExitCodeError(256, ...)` passes the panic check (256 ≠ 0) but `os.Exit(256)` on Unix returns rc=0 (256 mod 256). Constructor only rejects 0; valid range is [1, 255]. Both Code=0 (via struct literal `&ExitCodeError{Code: 0}`) and Code=256 (via constructor) produce fail-open. Fix: check `code < 1 || code > 255` in constructor.

**T3 gap — errors.As ungated**: `TestJSONExit_WrappedExitCodeError_NotSilenced` is in-process; never runs Execute(). Swapping `err.(*ExitCodeError)` for `errors.As(err, &exitErr)` in root.go passes all tests. A subprocess test where RunE returns `fmt.Errorf("%w", newExitCodeError(1, "msg"))` and asserts `stderr != ""` would close this gap.

**Why:** measured via mutation tests — T1 (struct-literal and Code=256 binaries), T3 (Mutation B: errors.As swap, all tests pass). The constructor check alone is insufficient — Execute() must also validate range.

**How to apply:** fix is in Execute() before os.Exit, not only in the constructor: `if exitErr.Code >= 1 && exitErr.Code <= 255 { os.Exit(...) }` else fall through to loud path. For T3, extract Execute()'s decision into a pure `decideExit()` function and table-test it without real commands.
