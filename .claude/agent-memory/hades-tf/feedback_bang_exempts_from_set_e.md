---
name: feedback_bang_exempts_from_set_e
description: "! command" exempts the negated command from set -e; "! cmp -s a b" is NOT a no-op guard — it never aborts even when files are identical
metadata:
  type: feedback
---

`!` in bash negates the exit status AND exempts the command from `set -e`.

Verified 2026-10-30 during ML-0A Wave 0:

```
$ bash -c 'set -euo pipefail; ! cmp -s /etc/hosts /etc/hosts; echo STILL_RUNNING'
STILL_RUNNING
```

When `cmp -s a b` returns 0 (files identical), `! cmp -s a b` returns 1. Under `set -e` a non-zero exit normally aborts, but `!` explicitly disables that — the shell continues.

**Why:** I used `! cmp -s` as a guard to detect falsification no-ops in `check-ci-workflow-pin-parity.sh`. The guard never fires because the shell never aborts.

**How to apply:** Never use `! command` as a guard under `set -e`. Use an explicit `if`/`else`:

```sh
if cmp -s "$FALS/file.yml" "$RUN1/other.yml"; then
  fail "label" "sed was a no-op — falsified file equals original"
fi
```

This pattern is what I prescribed in P1 of REQ-2026-10-30.
