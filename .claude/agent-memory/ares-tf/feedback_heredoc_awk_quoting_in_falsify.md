---
name: heredoc-awk-quoting-falsify
description: In <<'SABEOF' heredocs, awk single-quote character class must use '"'"' trick; origin/main as old-script fixture breaks post-merge
metadata:
  type: feedback
---

## Rule
When embedding an old bash script as a heredoc fixture in check-gates-falsify.sh to prove non-vacuity, use `<<'SABEOF'` and write awk quoting tricks literally.

**Specific trap:** awk character class `["']` cannot be written as-is inside `<<'SABEOF'`. Write `["'"'"']` instead. The heredoc writes literal bytes; bash later interprets the quoting trick during script execution.

Writing `["']` causes: `unexpected EOF while looking for matching "` when the sabotaged script is run.

**Why:** `<<'SABEOF'` disables all shell interpretation, so the content is written byte-for-byte to the file. When that file is executed, bash applies normal parsing, interpreting `'"'"'` as the standard trick to embed `'` inside a single-quoted string. `["']` writes `["']` literally, and bash sees the `'` as closing the single-quoted awk program prematurely.

**Do NOT use `git show origin/main:scripts/<gate>` as the "old version" fixture.** After the fixing PR merges, `origin/main` holds the FIXED version — the arm that said "old version passes the bug" would now fail permanently on main.

Instead: embed the old version as `<<'SABEOF'` heredoc and use `cmp -s` for liveness (verify current script differs from heredoc).

**How to apply:** Any time a falsify scenario needs to prove "old code has bug X", embed the old code as a heredoc fixture and do `cmp -s current sabotaged` for liveness.
