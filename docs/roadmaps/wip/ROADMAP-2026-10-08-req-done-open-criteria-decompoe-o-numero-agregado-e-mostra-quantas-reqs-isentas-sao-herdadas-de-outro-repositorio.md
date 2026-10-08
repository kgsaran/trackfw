---
status: wip
date: 2026-10-08
req: ""
squad: ""
---

# Roadmap: req_done_open_criteria decompoe o numero agregado e mostra quantas REQs isentas sao herdadas de outro repositorio

> Created: 2026-10-08 | Status: wip

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Acceptance Criteria
- [ ] AC1–AC5 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e discriminante
> Dependencies: PR #543 mergeado (ok, 2026-10-08). Blocks all implementation.

### ML-0A — Threat model e escolha do discriminante (AC1)
**Status:** 🔄 Em andamento
**Squad:** hades-tf
**Files affected:** `docs/seguranca/<data>-wave0-req-done-open-criteria-herdadas.md`
**Actions:**
1. Enumeration completeness — todos os sítios que emitem a linha agregada (texto e `--json`), e quem a lê (gates, scripts, `check-validate-rule-pins.sh`, docs).
2. Threat model — quem usa o recorte "herdada" para esconder dívida própria (ex.: REQ local com o mesmo basename de uma do upstream; remote `upstream` apontando para o próprio repo).
3. Falsification targets nas duas direções.
4. Declared residual.
5. Medir custo e semântica do discriminante derivado (`git ls-tree` no ramo padrão de `upstream`, ref ausente, fetch velho, shallow clone) vs declarado (`upstream_origin`).
**Acceptance criteria:**
- [ ] Seções respondidas com evidência e recomendação do discriminante

**Gates da wave:**
```bash
test -n "$(ls docs/seguranca/*wave0-req-done-open-criteria-herdadas.md 2>/dev/null)"
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Recorte na linha agregada (AC2–AC4)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** `internal/validator/validator_req_done_criteria.go`, testes, `docs/cli-parity.md` se documenta a linha
**Acceptance criteria:**
- [ ] Sem remote `upstream`: saída byte-idêntica (teste)
- [ ] Com `upstream`: recorte correto nas duas direções (teste com repo temporário)
- [ ] Falsificação e frase de reconciliação por teste novo
- [ ] `make quality` (arquiteto)

## Wave 2 — Red-team e fechamento
### ML-2A — Red-team do diff
**Status:** ⬜ Pendente
**Squad:** hades-tf
- [ ] Parecer sobre o diff contra o threat model
