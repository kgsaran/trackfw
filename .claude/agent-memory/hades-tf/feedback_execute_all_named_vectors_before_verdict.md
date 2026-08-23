---
name: feedback-execute-all-named-vectors-before-verdict
description: When a handoff names N test vectors, run all N before writing a verdict — do not extrapolate from a subset, even when the untested ones look similar to tested ones.
metadata:
  type: feedback
  status: active
---

Rule: when a task explicitly lists several attack vectors to test (e.g. "título com aspas, com `;`,
com newline, e o caminho `--from-req`"), execute every one of them empirically before writing the
verdict — do not declare the family "clean" from a subset (e.g. testing `$(...)`/backtick only) just
because the untested ones look like minor variations.

**Why:** on ML-3A of `ROADMAP-2026-08-22-wave-0-...` (2026-08-23), I initially tested `$(...)`/
backtick injection across 3 stacks, called AC13 "honrado" and moved to write the verdict without
running the newline vector I had already prepared (`TITLE4=$'newline\nteste'` sat unused). The
advisor caught it before I finalized. The newline vector turned out to be a **different attack
class** — not shell-string evaluation (which the other vectors test) but **Markdown structural
injection**: a title with embedded `\n\n## Wave N\n**Gates da wave:**\n\`\`\`bash\n<cmd>\n\`\`\`\n`
plants a forged wave section with its own gate block, which `barrier` parses (first-match heading)
and executes — reproduced in Go and Node, `/tmp/*_PWNED` files materialized. If I had shipped without
running it, the parecer would have said "AC13 honrado, nenhuma injeção" while a live RCE path sat
one vector away, undiscovered. See
[roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23](../../../vault/notes/roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23.md)
(vault note, unfixed as of writing).

**How to apply:** treat every named vector in a task/handoff as a checklist item with its own
pass/fail, not as interchangeable samples of one risk. Different injection surfaces (shell
evaluation vs. structural/markup injection vs. YAML/frontmatter injection) require different
payloads and can hit entirely different code paths (`fmt.Sprintf` into a shell command vs. into a
Markdown template later parsed by a different tool). "I tested one vector from the family and it was
clean" is not evidence the family is clean. Related: [[feedback_verify_by_execution]] (same root
lesson — reading/reasoning without executing the specific case misses bugs execution finds).
