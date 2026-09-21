---
name: feedback-evidence-discipline
description: Measurement rules for Hefesto reviews — specific grep/pipe patterns that lie
metadata:
  type: feedback
---

Never use `grep -c '"keyword"'` with quotes inside the pattern — it searches for the literal quotes
and returns 0 silently.

Never measure RC after a pipe with `cmd | tail; echo $?` — `$?` is the RC of `tail`, not `cmd`.

**Why:** both traps were hit during this project on 2026-09-21 by the architect and produced
false-zero counts that were reported as facts. The vault note on this exists in
`docs/agents-working-context.md` (Zeus entries, 2026-09-21).

**How to apply:** always verify counts with `wc -l` on piped `grep` without `-c`; always capture
RC in a separate step or use `if cmd; then`.
