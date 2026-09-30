---
name: feedback_fuse_same_line_as_barrier_call
description: T_FUSE safety fuse must be on the same line as the barrier call in gate blocks; ParseGates runs each line as isolated sh -c
metadata:
  type: feedback
---

The T_FUSE shell safety fuse pattern must be on the SAME LINE as the `barrier` call in any gate block inside a roadmap fixture.

**Why:** `ParseGates` (roadmapdoc.go:668) runs each non-empty, non-comment line of a gate block as a separate `sh -c` process. `export T_FUSE=...` on line N does NOT survive to line N+1. If the fuse check is on a separate line from the barrier call, the export happens in a subprocess that dies immediately — the fuse is silently ineffective.

**Measured:** ML-2A 2026-09-30. A multi-line gate fixture had fuse on line 1, barrier call on line 2. The fork bomb ran ~120s uncontained before timeout.

**How to apply:** Any time writing a reentry test in a roadmap fixture gate block (or reviewing one), verify the fuse and the barrier invocation are a single line — as shown in T1/T2 of `barrier_reentry_test.go`.

Also: `pkill -9 -x trackfw` only kills processes named exactly `trackfw`. Test binaries may be named `tf` or `trackfw_bin` — use the correct names.
