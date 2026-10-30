---
name: go-runtime-reopens-closed-fd1-as-devnull
description: Go 1.25 (macOS) reopens closed fd 1 as /dev/null at startup — use 1</dev/null not 1>&- to get real EBADF in tests
metadata:
  type: feedback
---

Use `1</dev/null` (open stdout read-only) to produce a genuine EBADF write error in a subprocess test. Do NOT use `1>&-` (close stdout).

**Why:** On macOS with Go 1.25, when fd 1 is closed before the process starts, the Go runtime reopens it as `/dev/null` (char device, `Dcrw-rw-rw-`). Writes "succeed" silently — `writeErr == nil`. `1>&-` is therefore indistinguishable from `>/dev/null` at the write level. `1</dev/null` leaves fd 1 open but read-only; writes to it return EBADF as expected.

Measured on Go 1.25.2, macOS Darwin 27.2.0: `os.Stdout.Stat()` returns `Dcrw-rw-rw-` after `1>&-`; `fmt.Fprintln(os.Stdout, "x")` returns `writeErr=nil, n=17`. Same `Fprintln` after `1</dev/null` returns `writeErr="write /dev/stdout: bad file descriptor"`.

**How to apply:** whenever writing a test that exercises a write-failure guard on stdout, use shell `1</dev/null` or open stdout read-only in Go, not `1>&-`. Measured on Go 1.25.2/macOS only — behavior on Linux not verified.
