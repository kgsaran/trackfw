---
name: trackfw-radar-multi-empresa-redteam
description: trackfw-radar ML-2A red team (2026-09-22) — result, sole finding, and where the report lives
metadata:
  type: project
---

`trackfw-radar` (`C:\dev\ferramentas\trackfw-radar`) is a separate repo from `trackfw-main`, same
user, also governed by trackfw (ADR → REQ → ROADMAP → kanban), with its own
`docs/security/threat-model.md` (fundação) and per-feature threat models
(`docs/security/threat-model-<feature>.md`) plus red-team reports
(`docs/security/red-team-<feature>.md`) in the same directory — same pattern as this project's
Wave 0/red-team convention. It has **no** `docs/agents-working-context.md` (checked 2026-09-22) —
don't invent one un-requested.

Ran the Wave 2 red team for `ROADMAP-2026-09-21-cadastro-de-empresas-usuarios-papeis-e-vinculos`
(multi-tenant companies/users/roles/vínculos, ML-2A), against commit `e66e0aa` (ML-1I, adds
superadmin-configurable SMTP under S11). Denominator: 44 STRIDE controls enumerated (S1–S11), 29
attacked live (real HTTP against a real Postgres test DB, including two genuine concurrent-request
races and a full two-sided account-pre-hijack race), 0 broken. One finding, RT-1 (medium, not
high): company name has no control-character validation and reaches the invite email Subject
unsanitized (caught by `email.montar()`'s CRLF check only when real SMTP is used, so it fails
silently in prod); separately, a misconfigured SMTP saved via the new `SalvarConfiguracaoDeEmail`
silently breaks email delivery for the **entire installation** with no error surfaced to callers
(always 202) — reproduced live by accidentally doing exactly this mid-session. Full report:
`docs/security/red-team-multi-empresa.md`. See also
[[feedback_verify_tree_did_not_move_during_redteam]] for the process incident from the same session.
