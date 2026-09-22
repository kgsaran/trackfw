---
name: verify-tree-did-not-move-during-redteam
description: Before finalizing a red-team report, check git status/mtimes to confirm the working tree did not change mid-session out from under the tests
metadata:
  type: feedback
---

When red-teaming a live branch (not a frozen commit), check `git status --short`, `git diff --stat`
and file mtimes **during** the session, not just at the start. A concurrent agent (implementer, or
the same user in another window) can commit new, undeclared, security-relevant code to the same
branch while you are mid-attack — new files show up with no history in `git log` yet correlate with
mtimes that advance in real time as you keep testing.

**Why:** 2026-09-22, `trackfw-radar` ML-2A (`red-team-multi-empresa.md`). Read the code and started
attacking against commit `057994c` (ML-1H). Around 12:34–12:56 the tree accumulated an entire
undeclared feature (SMTP configuration by superadmin, `internal/segredo`, changes to
`internal/escopo`, `internal/httpapi/{auth,identidade,tokens,vinculos}.go`) while I already had test
users/companies set up and was actively attacking. A `go test ./...` process from another agent was
running in parallel. I noticed only because `git status --short` returned 47 changed files mid-session
where it had returned clean minutes earlier, and `ls -la --time-style=full-iso` on files I'd already
read showed mtimes newer than my server's compile time. Stopped, reported the discrepancy instead of
silently mixing evidence from two code states, and the coordinator then formally expanded scope
(committed as `e66e0aa`, ML-1I) — resolving it cleanly. Had I not checked, the report would have
described code that either didn't exist yet or had already been superseded, undermining every "not
broken" claim in it.

**How to apply:** For any red-team/audit task against a branch (not a tag/frozen SHA), snapshot
`git status --short` + `git log --oneline -1` at the start of live testing and again before writing
the final report. If they diverge, stop and reconcile — either the coordinator explicitly expands
scope to the new commit (cite the new SHA in the report's header), or you finish against the original
frozen state and say so explicitly. Never silently blend two states. See also
[[feedback_verify_by_execution]] — this is the same "verify by execution, not by reading" principle
applied to the stability of what you're executing against, not just to what the code does.
