---
name: enumerate-special-file-types-not-just-errors
description: when auditing "read this config file" code paths for silence/crash, test a FIFO (mkfifo) in place of the file, not just permission/dir-substitution errors — it hangs instead of erroring
metadata:
  type: feedback
---

Self-derived during the ML-1B barrier (2026-09-06, trackfw), confirmed useful (found a real,
live-reproduced 3-runtime hang that a "did we cover ENOENT/EACCES/EISDIR/ELOOP" checklist would
have missed).

**Rule:** when a barrier's job is to enumerate every way a read-a-config-file loop can fail
silently or crash, `open()`/`ReadFile()`/`readFileSync()` returning an *error* is not the only
failure surface — a FIFO (`mkfifo`) placed at the target path makes the read syscall **block
indefinitely** instead of erroring, because it waits for a writer on the other end. No error, no
exception, no exit — the whole process (Go, Node, and Python alike, confirmed live) just never
returns.

**Why:** error-based enumeration (permission denied, is-a-directory, invalid encoding, invalid
JSON) is the natural checklist after reading a diff that fixes error branches — it's easy to stop
there because every prior finding in this exact REQ was error-shaped. A hang is a different failure
shape entirely and won't show up by reading the code's error branches; it only shows up by actually
running the binary against the fixture with a timeout.

**How to apply:** any time the task is "audit this read-path for a residual silent skip / crash",
add one fixture that isn't an error case at all: a named pipe (`mkfifo`) at the target path, run the
real binary with a background job + `sleep N` + `kill -0` check (not `timeout`, which isn't
available in this environment's default shell — see the `command not found: timeout` failure this
session). Treat "still running after N seconds" as a finding on its own, separate from and often
more severe than a crash (crashes at least terminate and often surface — a CI gate that hangs
either burns the runner until an infra-level timeout, or blocks forever with no upstream timeout).

**Caveat learned the same session (advisor caught it):** a project-scope fixture (config file
inside the repo under test) is cheap and safe to build. A global-scope equivalent (e.g. a config
under the real `$HOME`) is NOT something to fabricate live — don't `mkfifo` into the user's actual
home directory just to complete a matrix. State plainly in the report that the global-scope case is
inferred from the identical read primitive, not measured, rather than skipping the caveat or
overclaiming coverage.
