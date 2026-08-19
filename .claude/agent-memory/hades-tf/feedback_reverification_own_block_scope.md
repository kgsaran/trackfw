---
name: feedback-reverification-own-block-scope
description: When reverifying my own BLOQUEAR verdict, lift only the specific mechanism named in the handoff's lifting criterion — don't silently let unrelated "Dano real" items ride along as if closed too
metadata:
  type: feedback
---

When asked to reverify a block I issued (trackfw's "quem bloqueou é quem levanta o bloqueio"
pattern, e.g. ML-4A -> ML-4C on `release tag`), the handoff usually names one precise lifting
criterion (e.g. "tag pointing to a commit that isn't the forge's tip"). Fixing that mechanism does
NOT automatically close every "Dano real" item listed in the original block — some damages may be
independent of the mechanism that got fixed.

**Why:** In the `release tag` reverification (2026-08-19), the ADR Emenda 1 fixed the commit-target
selection (forge-sourced instead of local-ref-sourced) — the exact criterion the handoff asked me to
reverify. But two of the three "Dano real" items from my own original block (version-number
squatting via local version files, forged CHANGELOG text under the real tagger identity) never
depended on the commit-target mechanism at all — they read `deps.readFile` locally with zero forge
comparison. My first draft of the reverification report said "Achado novo: nenhum" and implicitly
let the reader conclude the command was now clean. The advisor caught this before I shipped: my own
mid-session note ("mas é apenas o primeiro degrau... a mensagem publicada é inteiramente forjada")
already had the answer, I just didn't carry it into the final verdict framing.

**How to apply:** verdict should be `LEVANTADO COM RESSALVAS`, not a bare `LEVANTADO`, whenever the
original block cited multiple independent damages and the fix only addressed one. Name the
untouched items explicitly in a "Ressalvas" section, state which ML/ADR item (if any) claims to
have addressed them (usually none), and don't let "no new finding" wording imply "fully resolved" —
those are different claims. Also: when closing a gate complaint from the earlier report ("no P4 for
this property"), verify by reading which existing scenario's assertion would flip if the fix were
reverted, rather than leaving the complaint unaddressed in the reverification.

See [[feedback_verify_by_execution]] for the sibling rule about measuring the reproduction itself
with an independent fixture, not just trusting the gate's own scenarios.
