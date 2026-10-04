---
name: criterionlinerere-any-char-bypass
description: CriterionLineRe = ^- \[.\] accepts any single char as "met" — [~], [-], [?] all pass without justification
metadata:
  type: project
---

`- [~] x`, `- [-] x`, `- [?] x` all count as met by `CriterionLineRe = regexp.MustCompile('^- \[.\]')` (roadmapdoc.go:46).

**Why:** the regex uses `.` which matches any single char. Measured 2026-10-04 — all three return `result: passed`.

**How to apply:** when evaluating Caducou: security, note that this is a cheaper bypass (no justification text required). Any future AC-counting change should restrict the regex to `^- \[[ xX]\]`. Track as separate issue from REQ-2026-10-04.
