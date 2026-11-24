---
name: fifo-test-needs-build-tag
description: FIFO tests (syscall.Mkfifo) must go in a _unix_test.go file with //go:build !windows — runtime.GOOS skip is not enough
metadata:
  type: feedback
---

`syscall.Mkfifo` does not exist in the Windows `syscall` package. A `runtime.GOOS == "windows"` skip inside the test body does **not** prevent a compile error — `go vet` processes every `_test.go` file regardless of which tests will run.

**Why:** In ML-1A (REQ-2026-11-23), the initial FIFO test was placed in `writescope_nested_root_test.go` with only a runtime skip. `GOOS=windows go vet ./internal/guard/` caught the compile error. The fix was to split the FIFO test into `writescope_nested_root_fifo_unix_test.go` with `//go:build !windows`.

**How to apply:** Any test calling `syscall.Mkfifo` must be in a file with `//go:build !windows` at the top (line 1, blank line, rest of file). See the existing pattern in `internal/fileutil/fileutil_fifo_unix_test.go` and `internal/validator/regularfile_fifo_unix_test.go`. Cross-platform tests live in the sibling file without the build tag.
