---
name: id-pelado-e-forma-viva-em-fixtures
description: Exigir caminho .md no campo REQ:/ADR: não quebra artefato governado nenhum, mas quebra as FIXTURES que usam o ID pelado — 6 testes de barrier, 1 de ship e 12 sítios de check-barrier.sh; e o alvo tem de existir no disco
metadata:
  type: feedback
---

Antes de migrar `wip_has_req`/`blocked_has_req`/`req_has_adr` para o leitor estrutural
(`contentHasStructuredRefValue` → `extractRefPath`, que exige valor terminado em `.md`), **grepe pelo
ID pelado nas fixtures**, não só no acervo:

```
grep -rn "REQ: REQ-\|ADR: none" scripts/ internal/ | grep -v docs/
```

**Why:** medido no ML-1E de 2026-09-26. Nos artefatos governados o delta foi **zero** (0 → 0 em `wip/`
e `blocked/`), o que dá a falsa impressão de mudança inócua. Nas fixtures o ID pelado era a forma
padrão: quebrou `TestBarrierContract_*` (6), `TestShip_Integration_GracefulDegradation_RealBinary` e
**12** sítios de `scripts/check-barrier.sh` — todos porque o check `validate` da barrier passou a
bloquear por `wip_has_req` em cenários cuja premissa é "governança verde".

**How to apply:** trocar o ID pelado por caminho real **e materializar o alvo** — `ref_targets_exist`
está em `lenientCarveoutRules`, logo é violação mesmo em lenient, e um alvo inexistente reprova
igual. A REQ materializada precisa ela mesma estar vinculada (`adr:` para um ADR **Accepted**
existente, `roadmap:` para um roadmap em `abandoned/`), senão `req_has_adr`/`req_has_roadmap` acusam a
fixture. Em `check-barrier.sh` o lugar certo é dentro de `common_dirs` (roda antes de todo cenário);
em testes Go, um helper (`writeBarrierREQFixture`) chamado depois de escrever o roadmap.
⚠️ Cuidado ao fazer replace em massa: se o comentário que você escreve CITA o literal antigo, o
`str.replace` reescreve o próprio comentário e o `assert count == N` explode.
Relacionado: [[fixture-de-req-precisa-alvo-em-abandoned]], [[migrar-leitor-de-regra-esvazia-falsify]].
