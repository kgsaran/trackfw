---
name: scaffold-test-wave0block
description: Contract test for scaffold vocabulary derives allowed pending tokens from wave0Block (generator), not a literal list
metadata:
  type: feedback
---

For scaffold vocabulary contract tests, derive the allowed pending first-token set by scanning `wave0Block` (same package, unexported but accessible in `package generators` tests) with `roadmapdoc.StatusLineRe`. This satisfies the ADR "rule, not a list of literals" requirement.

**Why:** `StatusCategory("pending")` and `StatusCategory("⬜ Pendente")` both return `StatusPending` — the package cannot discriminate them. A test that only calls the classifier will fail to detect the `pending` bug. The fix: extract first tokens FROM the generator's own output (wave0Block), then use those as the reference set.

**How to apply:** In any test validating that a scaffold/template teaches canonical vocabulary:
1. Extract markers from generated output using `strings.TrimSpace(line)` + `StatusLineRe` (unmasked — intentional, inverse of MLStatusMarker)
2. Build `generatorPendingTokens` map by scanning `wave0Block` for non-complete status lines
3. Accept marker iff `StatusIsComplete(marker)` OR first token in `generatorPendingTokens`
