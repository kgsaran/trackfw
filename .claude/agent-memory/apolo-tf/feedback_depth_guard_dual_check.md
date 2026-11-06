---
name: depth-guard-dual-check
description: D4 depth ceiling is enforced by BOTH handleShellWrapper (depth >= 2) AND matchSubcommandPOSIX (depth > 2); counter-arm must patch both
metadata:
  type: feedback
---

The D4 wrapper recursion guard has TWO independent depth checks:

1. `handleShellWrapper` in `internal/guard/gitbranch.go`: `if depth >= 2 { return "wrapper-not-analyzed" }` — this is the **effective** ceiling; it fires before another recursive call.
2. `matchSubcommandPOSIX`: `if depth > 2 { return "wrapper-not-analyzed" }` — this is a belt-and-suspenders guard against direct calls at depth > 2 (which don't happen in practice).

**Why:** A counter-arm test using 3-level nesting and only patching `matchSubcommandPOSIX` to `depth > 3` will STILL return "wrapper-not-analyzed" because `handleShellWrapper` hits `depth >= 2` first at the innermost level. The test appears to pass even with the ceiling raised, giving a false green counter-arm.

**How to apply:** When writing a counter-arm for the depth limit:
- Patch BOTH `matchSubcommandPOSIX` (`depth > 2` → `depth > 3`) AND `handleShellWrapper` (`depth >= 2` → `depth >= 3`) to confirm the test actually fails.
- Measured in ML-1B (REQ-2026-11-05): patching only `matchSubcommandPOSIX` let `TestMatchSubcommand_AC4_Depth3_WrapperNotAnalyzed` pass (false green); patching both made it fail (correct red).
