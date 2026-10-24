---
name: feedback-working-context-rewrite-pattern
description: How to reword agent asset ## Working context sections so TestAgentInstructionsUseWorkingContextDir passes without changes
metadata:
  type: feedback
---

When rewriting the `## Working context` section of non-architect agent assets (ML-1A of REQ-2026-10-23), the new text must preserve the exact substring ``docs/agents-working-context.md`, which is frozen history`` so that `checkWorkingContextDir` in `working_context_dir_test.go` can strip it. This avoids touching `working_context_dir_test.go` (which is outside ML-1A's `Files affected`).

The approved wording is:
> Do not write to the working context — neither in `docs/agents-working-context/` nor in `docs/agents-working-context.md`, which is frozen history. `trackfw_architect` records it. In your final report, state what you started, what changed and what is pending, so the architect can record it. To read the current state, run `trackfw context` or read the 10 most recent entries in `docs/agents-working-context/`.

**Why:** The stripping constant `allowedContextMention` = ``docs/agents-working-context.md`, which is frozen history``. If the new text uses a different phrasing around `.md`, the strip fails and check 2 fires.

**How to apply:** Any future D1-style rewrite of asset sections should verify the `checkWorkingContextDir` logic before finalizing wording.
