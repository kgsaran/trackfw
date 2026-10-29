---
name: project_mlscope_barrier_ml2a_verdict
description: ML-2A red team of barrier ml_scope (2026-10-29): APROVA COM RESSALVAS; 4 achados, none over-grant
metadata:
  type: project
---

2026-10-29, ML-2A red team of `checkMLScope` + `parseMLScopeFilesAffected` in `barrier.go`.

Verdict: **APROVA COM RESSALVAS** — no regression introduced by ML-1A (`a67354ef`) that over-grants scope or fails open.

**Achados:**

1. **MÉDIO** (pre-existing, `7bddcd346`): evidence string `scope: N file(s) checked, M out of scope` never emitted; code emits `all N touched file(s) are within scope` (`barrier.go:1329`). Contract: `cli-parity.md:3374`.
2. **MÉDIO** (pre-existing, `7bddcd346`): undeclared ML failure prefix `undeclared ML:` never emitted; code emits `<ML-id>: no **Files affected:** declaration (...)` (`barrier.go:1193`).
3. **BAIXO** (ML-1A introduced narrow stop): `barrier.go:890` stop lists only `[ ]`/`[x]`/`[X]` instead of using `CriterionLineRe` (`^- \[.\]`, commit `04afe3f65`). Binary CONFORMS to contract text (which also lists only three). `[~]`/`[-]` in `Files affected` feeds scope. Zero corpus occurrences.
4. **DOC** (ML-1A): `rejected:` format documented in prose `cli-parity.md:3260–3262` but absent from summary table `cli-parity.md:3374`.

**Owners**: Achados 1/2 → implementor (`barrier.go`) or contract maintainer (`cli-parity.md`). Achado 3 → both. Achado 4 → contract maintainer.

**Why:** ML-1A added list-format parsing and req:/adr: prefix validation. Control itself is sound.

**How to apply:** Next barrier ml_scope review: these 4 findings are open; check if fixed before re-classifying.

[[project_req_adr_prefix_grant_live_bypass]]
