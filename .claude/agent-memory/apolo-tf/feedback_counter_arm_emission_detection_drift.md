---
name: counter-arm-emission-detection-drift
description: counter-arm that stays green may mean the mutation never reached the emitted artifact — emission and detection must use the same constant
metadata:
  type: feedback
---

When writing a counter-arm that mutates a constant to prove a test catches the wrong value, verify that both detection AND emission use that constant. If emission hardcodes a literal and detection uses the constant, mutating the constant only breaks detection — the emitted file still contains the correct literal and the test under the arm may stay green for the wrong reason.

**Why:** In ML-1B (REQ-2026-11-01), `gitIgnoreAgentMemoryBlock()` hardcoded the pattern as a string literal while `hasGitIgnoreAgentMemoryRule()` compared against the constant. Mutating the constant to `**/.claude/agent-memory/` changed only the detection side: the emitted `.gitignore` still held `*/**/.claude/agent-memory/`, so `Vector_RaizNaoIgnorada` stayed green (not caught by the mutation). `Idempotencia` caught the mutation instead (detection didn't find its own pattern in the file, so a second run re-appended). This masked a real defect: if the constant were ever changed for the wrong pattern, the emitted `.gitignore` would diverge from detection. Fix: `gitIgnoreAgentMemoryBlock()` now embeds `gitIgnoreAgentMemoryEntry` via `fmt.Sprintf`.

**How to apply:** Before running any counter-arm, grep for every site that references the value under test — not just the detection function, also every emission site. If any site hardcodes the value as a literal instead of the constant, that site is a latent bug AND will confound the counter-arm. Fix the drift first, then measure.
