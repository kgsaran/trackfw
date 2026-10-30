---
name: project_parselsremotesymref_second_field_vector_gap
description: TestParseLsRemoteSymref missing vector for second field ending with HEAD but not equal to HEAD — allows HasSuffix mutation to survive
metadata:
  type: project
---

In `branch_default_ls_remote_test.go`, the test case named `"second field is refs/heads/HEAD not HEAD"` has misleading naming: the input is `"ref: refs/heads/HEAD\tHEAD"` (second field IS exactly `"HEAD"`), not `"ref: refs/heads/main\trefs/heads/HEAD"` as the name implies.

The missing vector is `{"ref: refs/heads/main\trefs/heads/HEAD", wantOK: false}` which would kill mutation M-2 (`!strings.HasSuffix(second, "HEAD")` instead of `second != "HEAD"`).

**Why:** found 2026-10-30 ML-2A red team. Practical impact is LOW (attacker controlling the remote can serve any branch name anyway), but the test documents wrong intent.

**How to apply:** when reviewing test coverage for `parseLsRemoteSymref`, check that a vector with second field ending in "HEAD" but not equal to "HEAD" exists.
